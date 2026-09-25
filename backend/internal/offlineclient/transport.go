package offlineclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sthira/backend/internal/offlinepkg"
)

// downloadAndVerifyArtifact performs the full download → verify → return
// pipeline for one manifest or card. It is the single chokepoint for
// Range/If-Range resumption, checksum verification, signature verification
// under the TrustStore, and rollback detection. The transport layer is
// shared between manifest and card so they exercise the same code path.
//
// Returns: canonical bytes, parsed manifest/card, bytes newly downloaded
// from the network (excluding .part resume bytes), staging .part path,
// error. The caller is responsible for clearing the .part after the
// activated bytes are durably committed.
func (c *ProtocolClient) downloadAndVerifyArtifact(ctx context.Context, req manifestDownload) ([]byte, *offlinepkg.Manifest, int64, string, error) {
	if req.IsCard {
		bytes, card, n, partPath, err := c.downloadAndVerifyCard(ctx, req)
		if err != nil {
			return nil, nil, 0, "", err
		}
		_ = card
		return bytes, nil, n, partPath, nil
	}
	return c.downloadAndVerifyManifest(ctx, req)
}

func (c *ProtocolClient) downloadAndVerifyManifest(ctx context.Context, req manifestDownload) ([]byte, *offlinepkg.Manifest, int64, string, error) {
	bytes, n, partPath, err := c.downloadBytes(ctx, req)
	if err != nil {
		return nil, nil, 0, "", err
	}
	m, err := offlinepkg.ParseManifest(bytes, offlinepkg.Limits{MaxBytes: 256 * 1024, MaxDepth: 32})
	if err != nil {
		return nil, nil, 0, "", fmt.Errorf("offlineclient: parse manifest: %w", err)
	}
	if err := c.verifyManifest(m, bytes); err != nil {
		return nil, nil, 0, "", err
	}
	return bytes, m, n, partPath, nil
}

func (c *ProtocolClient) downloadAndVerifyCard(ctx context.Context, req manifestDownload) ([]byte, *offlinepkg.PublicIncidentCard, int64, string, error) {
	bytes, n, partPath, err := c.downloadBytes(ctx, req)
	if err != nil {
		return nil, nil, 0, "", err
	}
	card, err := offlinepkg.ParseCard(bytes, offlinepkg.Limits{MaxBytes: 64 * 1024, MaxDepth: 32})
	if err != nil {
		return nil, nil, 0, "", fmt.Errorf("offlineclient: parse card: %w", err)
	}
	if err := c.verifyCard(card, bytes); err != nil {
		return nil, nil, 0, "", err
	}
	if req.ExpectedCardDesc != nil {
		desc := req.ExpectedCardDesc
		if card.PackageID != desc.PackageID {
			return nil, nil, 0, "", fmt.Errorf("offlineclient: card package_id mismatch: got %q expected %q", card.PackageID, desc.PackageID)
		}
		if card.Version != desc.Version {
			return nil, nil, 0, "", fmt.Errorf("offlineclient: card version mismatch: got %d expected %d", card.Version, desc.Version)
		}
		if req.Jurisdiction != "" && card.Jurisdiction != req.Jurisdiction {
			return nil, nil, 0, "", fmt.Errorf("offlineclient: card jurisdiction mismatch: got %q expected %q", card.Jurisdiction, req.Jurisdiction)
		}
		if desc.ChecksumSHA256 != "" && card.ChecksumSHA256 != desc.ChecksumSHA256 {
			return nil, nil, 0, "", fmt.Errorf("%w: card checksum %q does not match manifest reference %q", offlinepkg.ErrChecksumMismatch, card.ChecksumSHA256, desc.ChecksumSHA256)
		}
		if desc.UncompressedBytes > 0 && int64(len(bytes)) > desc.UncompressedBytes {
			return nil, nil, 0, "", fmt.Errorf("offlineclient: card byte size %d exceeds declared manifest budget %d", len(bytes), desc.UncompressedBytes)
		}
		if int64(len(bytes)) > 65536 {
			return nil, nil, 0, "", fmt.Errorf("offlineclient: card byte size %d exceeds 64 KiB ceiling", len(bytes))
		}
	}
	return bytes, card, n, partPath, nil
}

// parseContentRange parses "bytes <start>-<end>/<total>" or "bytes <start>-<end>/*".
func parseContentRange(hdr string) (start, end, total int64, err error) {
	hdr = strings.TrimSpace(hdr)
	if !strings.HasPrefix(hdr, "bytes ") {
		return 0, 0, 0, fmt.Errorf("invalid Content-Range header: %q", hdr)
	}
	spec := strings.TrimPrefix(hdr, "bytes ")
	slashIdx := strings.IndexByte(spec, '/')
	if slashIdx == -1 {
		return 0, 0, 0, fmt.Errorf("missing slash in Content-Range: %q", hdr)
	}
	rangePart := spec[:slashIdx]
	totalPart := spec[slashIdx+1:]

	hyphenIdx := strings.IndexByte(rangePart, '-')
	if hyphenIdx == -1 {
		return 0, 0, 0, fmt.Errorf("missing hyphen in Content-Range: %q", hdr)
	}
	start, err = strconv.ParseInt(rangePart[:hyphenIdx], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid start in Content-Range: %w", err)
	}
	end, err = strconv.ParseInt(rangePart[hyphenIdx+1:], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid end in Content-Range: %w", err)
	}
	if start < 0 || end < start {
		return 0, 0, 0, fmt.Errorf("invalid range span in Content-Range: %q", hdr)
	}
	if totalPart != "*" {
		total, err = strconv.ParseInt(totalPart, 10, 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid total in Content-Range: %w", err)
		}
		if total >= 0 && end >= total {
			return 0, 0, 0, fmt.Errorf("end exceeds total in Content-Range: %q", hdr)
		}
	} else {
		total = -1
	}
	return start, end, total, nil
}

type streamResult struct {
	Bytes        []byte
	BytesFromNet int64
	ETag         string
	PartPath     string
	RangeSupport bool
	ExpectedSize int64
}

// downloadStreaming downloads an artifact chunk-by-chunk directly into a .part file
// on disk. On network interruption, partial bytes remain on disk and metadata is
// flushed. On resumption, Range / If-Range is sent. ETag drift or 416 resets and retries.
func (c *ProtocolClient) downloadStreaming(ctx context.Context, u, partPath string, resume, isCard bool, maxBytes int64) (*streamResult, error) {
	if err := os.MkdirAll(filepath.Dir(partPath), 0o750); err != nil {
		return nil, fmt.Errorf("offlineclient: mkdir for .part: %w", err)
	}

	var existingMeta downloadMeta
	hadPart := false
	if resume {
		if b, err := os.ReadFile(filepath.Clean(partPath + ".meta")); err == nil { // #nosec G304
			if err := json.Unmarshal(b, &existingMeta); err == nil {
				if fi, err := os.Stat(partPath); err == nil && existingMeta.URL == u && fi.Size() == existingMeta.BytesWritten && existingMeta.BytesWritten > 0 {
					hadPart = true
				} else {
					c.storage.clearPart(partPath)
				}
			} else {
				c.storage.clearPart(partPath)
			}
		}
	}

	var resp *http.Response
	var isResume bool

	if hadPart && existingMeta.BytesWritten > 0 && (existingMeta.ExpectedSize == 0 || existingMeta.BytesWritten < existingMeta.ExpectedSize) && existingMeta.RangeSupported {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		if isCard {
			req.Header.Set("Accept", "application/json")
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingMeta.BytesWritten))
		if existingMeta.ExpectedETag != "" {
			req.Header.Set("If-Range", `"`+stripETagQuotes(existingMeta.ExpectedETag)+`"`)
		}
		r, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		if r.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			_ = r.Body.Close() // #nosec G104
			c.storage.clearPart(partPath)
			return c.downloadStreaming(ctx, u, partPath, false, isCard, maxBytes)
		} else if r.StatusCode == http.StatusPartialContent {
			cr := r.Header.Get("Content-Range")
			start, _, total, perr := parseContentRange(cr)
			if perr != nil || start != existingMeta.BytesWritten {
				_ = r.Body.Close() // #nosec G104
				c.storage.clearPart(partPath)
				return c.downloadStreaming(ctx, u, partPath, false, isCard, maxBytes)
			}
			if total > 0 {
				existingMeta.ExpectedSize = total
			}
			if old, got := existingMeta.ExpectedETag, stripETagQuotes(r.Header.Get("ETag")); old != "" && got != "" && old != got {
				_ = r.Body.Close() // #nosec G104
				c.storage.clearPart(partPath)
				return c.downloadStreaming(ctx, u, partPath, false, isCard, maxBytes)
			}
			resp = r
			isResume = true
		} else if r.StatusCode == http.StatusOK {
			resp = r
			isResume = false
		} else {
			_ = r.Body.Close() // #nosec G104
			return nil, fmt.Errorf("offlineclient: GET %s: status %d", u, r.StatusCode)
		}
	}

	if resp == nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		if isCard {
			req.Header.Set("Accept", "application/json")
		}
		r, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		if r.StatusCode != http.StatusOK {
			_ = r.Body.Close() // #nosec G104
			return nil, fmt.Errorf("offlineclient: GET %s: status %d", u, r.StatusCode)
		}
		resp = r
		isResume = false
	}
	defer resp.Body.Close()

	etag := stripETagQuotes(resp.Header.Get("ETag"))
	_, rangeSupport := resp.Header["Accept-Ranges"]

	var meta downloadMeta
	var flags int
	if isResume {
		meta = existingMeta
		if etag != "" {
			meta.ExpectedETag = etag
		}
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	} else {
		var expectedSize int64
		if cl := resp.Header.Get("Content-Length"); cl != "" {
			expectedSize, _ = strconv.ParseInt(cl, 10, 64)
		}
		meta = downloadMeta{
			URL:            u,
			ExpectedETag:   etag,
			ExpectedSize:   expectedSize,
			BytesWritten:   0,
			RangeSupported: rangeSupport,
		}
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	f, err := os.OpenFile(filepath.Clean(partPath), flags, 0o600) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("offlineclient: open .part file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	var bytesFromNet int64

	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if maxBytes > 0 && meta.BytesWritten+int64(n) > maxBytes {
				_ = f.Sync()
				_ = c.storage.writePartMeta(partPath, meta)
				return nil, ErrTooLarge
			}
			if _, werr := f.Write(buf[:n]); werr != nil {
				_ = f.Sync()
				_ = c.storage.writePartMeta(partPath, meta)
				return nil, fmt.Errorf("offlineclient: write .part: %w", werr)
			}
			bytesFromNet += int64(n)
			meta.BytesWritten += int64(n)
			_ = f.Sync()
			_ = c.storage.writePartMeta(partPath, meta)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			_ = f.Sync()
			_ = c.storage.writePartMeta(partPath, meta)
			return nil, rerr
		}
	}

	if err := f.Sync(); err != nil {
		return nil, err
	}
	if err := c.storage.writePartMeta(partPath, meta); err != nil {
		return nil, err
	}

	finalBytes, err := os.ReadFile(filepath.Clean(partPath)) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("offlineclient: read completed .part: %w", err)
	}

	return &streamResult{
		Bytes:        finalBytes,
		BytesFromNet: bytesFromNet,
		ETag:         meta.ExpectedETag,
		PartPath:     partPath,
		RangeSupport: meta.RangeSupported,
		ExpectedSize: meta.ExpectedSize,
	}, nil
}

// downloadBytes runs the standard protocol: GET (or Range GET if .part
// exists with a matching ETag), writes directly into .part + .part.meta via streaming,
// and returns canonical bytes.
func (c *ProtocolClient) downloadBytes(ctx context.Context, req manifestDownload) ([]byte, int64, string, error) {
	u, err := c.absoluteURL(req.Path)
	if err != nil {
		return nil, 0, "", err
	}
	partPath := c.partPathFor(req)
	maxBytes := int64(256 * 1024) // manifest ceiling; resources use downloadResource.
	if req.IsCard {
		maxBytes = 65536
	}
	res, err := c.downloadStreaming(ctx, u, partPath, true, req.IsCard, maxBytes)
	if err != nil {
		return nil, 0, "", err
	}

	if res.ExpectedSize > 0 && int64(len(res.Bytes)) != res.ExpectedSize {
		return nil, 0, "", fmt.Errorf("offlineclient: size mismatch: got %d expected %d", len(res.Bytes), res.ExpectedSize)
	}

	return res.Bytes, res.BytesFromNet, res.PartPath, nil
}

func stripETagQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// partPathFor returns the .part path used to stage the artifact during
// download. We always use the package ID + version (or jurisdiction) so
// concurrent syncs for different jurisdictions do not collide.
func (c *ProtocolClient) partPathFor(req manifestDownload) string {
	if req.IsCard {
		return filepath.Join(c.StorageDir(), "downloads", fmt.Sprintf("card-%s-%d", req.PackageID, req.Version))
	}
	return filepath.Join(c.StorageDir(), "downloads", "manifest-"+req.Jurisdiction)
}

// absoluteURL joins the configured BaseURL with path. The result is
// normalized so the transport always sends a fully-qualified URL.
func (c *ProtocolClient) absoluteURL(path string) (string, error) {
	base := strings.TrimRight(c.baseURL, "/")
	if path == "" {
		return base, nil
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u, err := url.Parse(base + path)
	if err != nil {
		return "", fmt.Errorf("offlineclient: join URL %q + %q: %w", base, path, err)
	}
	return u.String(), nil
}

// --- Verification ---

// verifyManifest checks the checksum and signature under the TrustStore,
// then structural validity. It does NOT check rollback; that is the
// caller's job because rollback semantics involve persisted state.
func (c *ProtocolClient) verifyManifest(m *offlinepkg.Manifest, bytes []byte) error {
	if err := offlinepkg.ValidateManifestStructure(m); err != nil {
		return err
	}
	if err := c.verifyChecksumTyped(m, m.ChecksumSHA256); err != nil {
		return err
	}
	if m.Signature == nil {
		return errors.New("offlineclient: manifest missing signature")
	}
	canonical, jurisdiction := strippedManifest{M: m}.bytes()
	if err := c.verifySignature(canonical, jurisdiction, m.Signature.KeyID, m.Signature.Value); err != nil {
		return err
	}
	return nil
}

func (c *ProtocolClient) verifyCard(card *offlinepkg.PublicIncidentCard, bytes []byte) error {
	if err := offlinepkg.ValidateCardStructure(card); err != nil {
		return err
	}
	if err := c.verifyChecksumTyped(card, card.ChecksumSHA256); err != nil {
		return err
	}
	if card.Signature == nil {
		return errors.New("offlineclient: card missing signature")
	}
	canonical, jurisdiction := strippedCard{C: card}.bytes()
	if err := c.verifySignature(canonical, jurisdiction, card.Signature.KeyID, card.Signature.Value); err != nil {
		return err
	}
	return nil
}

// verifyChecksum recomputes the canonical-bytes digest and compares it
// against the declared value. A mismatch is integrity failure, not
// signature failure: it proves the bytes were corrupted or substituted
// at some point in the chain. The caller MUST pass the appropriate
// typed struct so the function can disambiguate manifest vs card
// bytes (both have a checksum_sha256 field, so the bytes parse as
// either — taking the wrong path silently fails verification).
func (c *ProtocolClient) verifyChecksumTyped(parsed any, declared string) error {
	if declared == "" {
		return errors.New("offlineclient: declared checksum is empty")
	}
	stripped := stripChecksumSignature(parsed)
	canonical, err := offlinepkg.CanonicalBytes(stripped)
	if err != nil {
		return err
	}
	got := offlinepkg.ChecksumSHA256(canonical)
	if got != declared {
		return offlinepkg.ErrChecksumMismatch
	}
	return nil
}

// stripChecksumSignature returns a copy of the parsed artifact with
// ChecksumSHA256 and Signature zeroed, so the checksum can be computed
// over the unsigned body. Used by verifyChecksumTyped for both
// manifest and card shapes.
func stripChecksumSignature(parsed any) any {
	switch v := parsed.(type) {
	case *offlinepkg.Manifest:
		cp := *v
		cp.ChecksumSHA256 = ""
		cp.Signature = nil
		return &cp
	case offlinepkg.Manifest:
		cp := v
		cp.ChecksumSHA256 = ""
		cp.Signature = nil
		return cp
	case *offlinepkg.PublicIncidentCard:
		cp := *v
		cp.ChecksumSHA256 = ""
		cp.Signature = nil
		return &cp
	case offlinepkg.PublicIncidentCard:
		cp := v
		cp.ChecksumSHA256 = ""
		cp.Signature = nil
		return cp
	}
	return parsed
}

// strippedManifest / strippedCard are tiny helpers to access the
// canonical bytes for a parsed artifact with signature + checksum
// zeroed. They live next to verifyChecksum because that is the only
// caller.
type strippedManifest struct{ M *offlinepkg.Manifest }

func (s strippedManifest) bytes() ([]byte, string) {
	cp := *s.M
	cp.ChecksumSHA256 = ""
	cp.Signature = nil
	canonical, err := offlinepkg.CanonicalBytes(cp)
	if err != nil {
		return nil, ""
	}
	return canonical, s.M.Jurisdiction
}

type strippedCard struct {
	C *offlinepkg.PublicIncidentCard
}

func (s strippedCard) bytes() ([]byte, string) {
	cp := *s.C
	cp.ChecksumSHA256 = ""
	cp.Signature = nil
	canonical, err := offlinepkg.CanonicalBytes(cp)
	if err != nil {
		return nil, ""
	}
	return canonical, s.C.Jurisdiction
}

// verifySignature binds the canonical bytes, the key ID, and the
// jurisdiction together through the TrustStore. Authorization and
// signature validity are checked together because splitting them lets a
// caller forget one.
func (c *ProtocolClient) verifySignature(canonical []byte, jurisdiction, keyID, sigBase64 string) error {
	if keyID == "" {
		return errors.New("offlineclient: signature missing key_id")
	}
	if sigBase64 == "" {
		return errors.New("offlineclient: signature missing value")
	}
	if canonical == nil {
		return errors.New("offlineclient: canonical bytes unavailable")
	}
	if err := c.trust.VerifySignature(keyID, jurisdiction, canonical, sigBase64); err != nil {
		return err
	}
	return nil
}

// --- Resource download ---

func (c *ProtocolClient) downloadResource(ctx context.Context, desc offlinepkg.ResourceDescriptor, resume bool) error {
	if desc.ResourceID == "" {
		return errors.New("offlineclient: ResourceID required")
	}
	if desc.ChecksumSHA256 == "" {
		return errors.New("offlineclient: resource ChecksumSHA256 required")
	}
	if desc.ByteSize > 0 && desc.ByteSize > c.maxResourceBytes {
		return ErrTooLarge
	}
	u, err := c.absoluteURL(desc.URI)
	if err != nil {
		return err
	}
	partPath := c.partPathForResource(desc.ResourceID)
	maxRes := c.maxResourceBytes
	if desc.ByteSize > 0 && desc.ByteSize < maxRes {
		maxRes = desc.ByteSize
	}

	res, err := c.downloadStreaming(ctx, u, partPath, resume, false, maxRes)
	if err != nil {
		return err
	}

	if int64(len(res.Bytes)) > maxRes {
		return ErrTooLarge
	}
	if desc.ByteSize > 0 && int64(len(res.Bytes)) != desc.ByteSize {
		return fmt.Errorf("offlineclient: resource size mismatch: got %d expected %d", len(res.Bytes), desc.ByteSize)
	}

	if got := offlinepkg.ChecksumSHA256(res.Bytes); got != desc.ChecksumSHA256 {
		return offlinepkg.ErrChecksumMismatch
	}

	// Atomic activation of the resource.
	meta := resourceMeta{
		ETag:            res.ETag,
		ChecksumSHA256:  desc.ChecksumSHA256,
		ByteSize:        int64(len(res.Bytes)),
		ContentType:     desc.ContentType,
		FetchedAtUnixMS: c.now().UnixMilli(),
	}
	if err := c.storage.writeResource(desc.ResourceID, res.Bytes, meta); err != nil {
		return fmt.Errorf("offlineclient: activate resource: %w", err)
	}
	c.storage.clearPart(partPath)
	return nil
}

func (c *ProtocolClient) partPathForResource(id string) string {
	return filepath.Join(c.StorageDir(), "downloads", "resource-"+id)
}

// _ keeps the json import live while only used inside helper closures
// above. It also documents the only externally-meaningful use of json in
// this file (canonicalization happens in offlinepkg).
var _ = json.Marshal
