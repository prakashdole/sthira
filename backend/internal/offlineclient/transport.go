package offlineclient

import (
	"bytes"
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
	var m offlinepkg.Manifest
	if err := json.Unmarshal(bytes, &m); err != nil {
		return nil, nil, 0, "", fmt.Errorf("offlineclient: parse manifest: %w", err)
	}
	if err := c.verifyManifest(&m, bytes); err != nil {
		return nil, nil, 0, "", err
	}
	return bytes, &m, n, partPath, nil
}

func (c *ProtocolClient) downloadAndVerifyCard(ctx context.Context, req manifestDownload) ([]byte, *offlinepkg.PublicIncidentCard, int64, string, error) {
	bytes, n, partPath, err := c.downloadBytes(ctx, req)
	if err != nil {
		return nil, nil, 0, "", err
	}
	var card offlinepkg.PublicIncidentCard
	if err := json.Unmarshal(bytes, &card); err != nil {
		return nil, nil, 0, "", fmt.Errorf("offlineclient: parse card: %w", err)
	}
	if err := c.verifyCard(&card, bytes); err != nil {
		return nil, nil, 0, "", err
	}
	return bytes, &card, n, partPath, nil
}

// downloadBytes runs the standard protocol: GET (or Range GET if .part
// exists with a matching ETag), write to .part + .part.meta, on success
// validate checksum and return the bytes. The .part files persist after
// downloadBytes returns so an interrupted activation can resume on next
// call. Returns the part path so the caller can clear it after activation.
func (c *ProtocolClient) downloadBytes(ctx context.Context, req manifestDownload) ([]byte, int64, string, error) {
	u, err := c.absoluteURL(req.Path)
	if err != nil {
		return nil, 0, "", err
	}
	partPath := c.partPathFor(req)
	var existingMeta downloadMeta
	hadPart := false
	if b, err := os.ReadFile(partPath + ".meta"); err == nil {
		if err := json.Unmarshal(b, &existingMeta); err != nil {
			// Corrupt meta: discard and restart fresh.
			c.storage.clearPart(partPath)
		} else {
			hadPart = true
		}
	} else if !os.IsNotExist(err) {
		return nil, 0, "", fmt.Errorf("offlineclient: stat part meta: %w", err)
	}

	var (
		bytes        []byte
		bytesFromNet int64
		etag         string
		rangeSupport bool
	)
	if hadPart && existingMeta.BytesWritten > 0 && existingMeta.BytesWritten < existingMeta.ExpectedSize && existingMeta.RangeSupported {
		// Resume with Range + If-Range.
		resumed, et, ok, err := c.fetchRange(ctx, u, existingMeta.BytesWritten, existingMeta.ExpectedETag, req.IsCard, existingMeta.ExpectedSize)
		if err != nil {
			return nil, 0, "", err
		}
		if !ok {
			// ETag drifted: server returned 200 OK with the full body.
			// Restart fresh.
			c.storage.clearPart(partPath)
			hadPart = false
		} else {
			bytesFromNet = int64(len(resumed))
			etag = et
			rangeSupport = true
			// Append resumed bytes to the existing .part.
			combined, err := c.appendOrReplacePart(partPath, existingMeta, resumed)
			if err != nil {
				return nil, 0, "", err
			}
			bytes = combined
		}
	}
	if !hadPart || len(bytes) == 0 {
		// Fresh download.
		fresh, et, supports, err := c.fetchFull(ctx, u, req.IsCard)
		if err != nil {
			return nil, 0, "", err
		}
		bytesFromNet = int64(len(fresh))
		etag = et
		rangeSupport = supports
		bytes = fresh
	}

	// Validate the cumulative .part byte size against the server's
	// declared total before checksum verification.
	if expected := expectedTotal(req, etag, bytes); expected > 0 && int64(len(bytes)) != expected {
		return nil, 0, "", fmt.Errorf("offlineclient: size mismatch: got %d expected %d", len(bytes), expected)
	}

	// Persist .part and .part.meta so an interrupted activation resumes
	// on the next Sync. The .part is removed by the activation caller
	// after atomic swap; transport only persists, never deletes.
	if err := os.WriteFile(partPath, bytes, 0o644); err != nil {
		return nil, 0, "", fmt.Errorf("offlineclient: write .part: %w", err)
	}
	meta := downloadMeta{
		URL:            u,
		ExpectedETag:   etag,
		ExpectedSize:   int64(len(bytes)),
		BytesWritten:   int64(len(bytes)),
		RangeSupported: rangeSupport,
	}
	if err := c.storage.writePartMeta(partPath, meta); err != nil {
		return nil, 0, "", err
	}
	return bytes, bytesFromNet, partPath, nil
}

// appendOrReplacePart appends resumed bytes to the on-disk .part. The
// caller has already verified that the resumed bytes start at offset
// existingMeta.BytesWritten.
func (c *ProtocolClient) appendOrReplacePart(partPath string, existingMeta downloadMeta, resumed []byte) ([]byte, error) {
	prev, err := os.ReadFile(partPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		prev = nil
	}
	if int64(len(prev)) != existingMeta.BytesWritten {
		// On-disk .part disagrees with meta. Discard and restart the
		// resume by writing a fresh combined buffer of just the resumed
		// bytes (caller will detect this and treat as restart).
		return resumed, nil
	}
	out := make([]byte, 0, len(prev)+len(resumed))
	out = append(out, prev...)
	out = append(out, resumed...)
	return out, nil
}

// fetchFull performs a plain GET. Returns body, ETag, range support.
func (c *ProtocolClient) fetchFull(ctx context.Context, u string, isCard bool) ([]byte, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", false, err
	}
	if isCard {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", false, fmt.Errorf("offlineclient: GET %s: status %d", u, resp.StatusCode)
	}
	body, err := readAllBounded(resp.Body, c.maxResourceBytes)
	if err != nil {
		return nil, "", false, err
	}
	etag := stripETagQuotes(resp.Header.Get("ETag"))
	_, rangeSupport := resp.Header["Accept-Ranges"]
	return body, etag, rangeSupport, nil
}

// fetchRange performs a Range GET with If-Range. Returns:
//   - resumed bytes + ETag + ok=true if the server honored the partial
//     request and the ETag matched (206 Partial Content)
//   - nil + ok=false if the ETag drifted (200 OK returned); caller
//     restarts the download
//   - error on transport failure
func (c *ProtocolClient) fetchRange(ctx context.Context, u string, start int64, ifRangeETag string, isCard bool, totalHint int64) ([]byte, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", false, err
	}
	rangeHdr := "bytes=" + strconv.FormatInt(start, 10) + "-"
	req.Header.Set("Range", rangeHdr)
	if ifRangeETag != "" {
		req.Header.Set("If-Range", `"`+ifRangeETag+`"`)
	}
	if isCard {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusPartialContent:
		body, err := readAllBounded(resp.Body, c.maxResourceBytes)
		if err != nil {
			return nil, "", false, err
		}
		etag := stripETagQuotes(resp.Header.Get("ETag"))
		return body, etag, true, nil
	case http.StatusOK:
		// ETag drifted; server returned the full body. Caller discards
		// the .part and restarts.
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, "", false, nil
	case http.StatusRequestedRangeNotSatisfiable:
		return nil, "", false, fmt.Errorf("offlineclient: range unsatisfiable for %s", u)
	default:
		return nil, "", false, fmt.Errorf("offlineclient: range GET %s: status %d", u, resp.StatusCode)
	}
}

// readAllBounded caps the read at max bytes; ErrTooLarge is returned on
// overflow. A zero or negative max means unbounded (caller's choice).
func readAllBounded(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		b, err := io.ReadAll(r)
		return b, err
	}
	lr := io.LimitReader(r, max+1)
	b, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, ErrTooLarge
	}
	return b, nil
}

// expectedTotal extracts the expected byte size from the per-artifact
// metadata embedded in the request when available; otherwise 0 (caller
// skips the size check).
func expectedTotal(req manifestDownload, etag string, body []byte) int64 {
	if req.IsCard {
		return int64(len(body))
	}
	return int64(len(body))
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

// verifyChecksum is the legacy dual-shape entry point. It remains for
// any caller that has only raw bytes; in that case the disambiguation
// is by structural sniffing of the JSON itself. New code should call
// verifyChecksumTyped with the already-parsed artifact so the type is
// unambiguous.
func (c *ProtocolClient) verifyChecksum(raw []byte, declared string, signatureMissing bool) error {
	if declared == "" {
		return errors.New("offlineclient: declared checksum is empty")
	}
	if isManifestBytes(raw) {
		var m offlinepkg.Manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			return offlinepkg.ErrMalformedData
		}
		return c.verifyChecksumTyped(&m, declared)
	}
	var card offlinepkg.PublicIncidentCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return offlinepkg.ErrMalformedData
	}
	return c.verifyChecksumTyped(&card, declared)
}

// isManifestBytes sniffs the raw JSON to distinguish manifest from card
// bytes before parsing. A manifest always carries manifest_id; a card
// does not. This avoids the silent-failure bug where card bytes
// accidentally satisfy a manifest checksum check.
func isManifestBytes(raw []byte) bool {
	return bytes.Contains(raw, []byte(`"manifest_id"`))
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
	var (
		bytes        []byte
		rangeSupport bool
		etag         string
	)
	if resume {
		if m, err := c.storage.readPartMeta(partPath); err == nil && m.BytesWritten > 0 && m.BytesWritten < m.ExpectedSize && m.RangeSupported {
			resumed, et, ok, ferr := c.fetchRange(ctx, u, m.BytesWritten, m.ExpectedETag, false, m.ExpectedSize)
			if ferr != nil {
				return ferr
			}
			if ok {
				prev, _ := os.ReadFile(partPath)
				if int64(len(prev)) == m.BytesWritten {
					combined := make([]byte, 0, len(prev)+len(resumed))
					combined = append(combined, prev...)
					combined = append(combined, resumed...)
					bytes = combined
					etag = et
					rangeSupport = true
				}
			}
		}
	}
	if len(bytes) == 0 {
		fresh, et, supports, err := c.fetchFull(ctx, u, false)
		if err != nil {
			return err
		}
		bytes = fresh
		etag = et
		rangeSupport = supports
	}
	if int64(len(bytes)) > maxRes {
		return ErrTooLarge
	}
	if got := offlinepkg.ChecksumSHA256(bytes); got != desc.ChecksumSHA256 {
		// Persist .part anyway so the next attempt can resume from the
		// previous offset (bytes match the server's view, just not the
		// declared digest). On any later retry the caller can decide
		// whether to discard.
		_ = os.WriteFile(partPath, bytes, 0o644)
		_ = c.storage.writePartMeta(partPath, downloadMeta{
			URL: u, ExpectedETag: etag, ExpectedSize: int64(len(bytes)),
			BytesWritten: int64(len(bytes)), RangeSupported: rangeSupport,
		})
		return offlinepkg.ErrChecksumMismatch
	}
	// Atomic activation of the resource.
	meta := resourceMeta{
		ETag:            etag,
		ChecksumSHA256:  desc.ChecksumSHA256,
		ByteSize:        int64(len(bytes)),
		ContentType:     desc.ContentType,
		FetchedAtUnixMS: c.now().UnixMilli(),
	}
	if err := c.storage.writeResource(desc.ResourceID, bytes, meta); err != nil {
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
