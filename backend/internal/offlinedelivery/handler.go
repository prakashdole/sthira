package offlinedelivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"sthira/backend/internal/contracts"
)

var (
	safeJurisdictionRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{2,32}$`)
	safePackageIDRegex    = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,64}$`)
	safeResourceIDRegex   = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,128}$`)
)

// Handler serves public offline delivery endpoints conforming to plan/p5-contract.md.
type Handler struct {
	cfg    Config
	src    PublicationSource
	logger *slog.Logger
	now    func() time.Time
}

// NewHandler constructs an offlinedelivery Handler with the given source and configuration.
func NewHandler(cfg Config, src PublicationSource, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.MaxCardBytes <= 0 {
		cfg.MaxCardBytes = DefaultConfig().MaxCardBytes
	}
	if cfg.MaxManifestBytes <= 0 {
		cfg.MaxManifestBytes = DefaultConfig().MaxManifestBytes
	}
	if cfg.MaxResourceBytes <= 0 {
		cfg.MaxResourceBytes = DefaultConfig().MaxResourceBytes
	}
	if cfg.StreamChunkSize <= 0 {
		cfg.StreamChunkSize = DefaultConfig().StreamChunkSize
	}
	return &Handler{
		cfg:    cfg,
		src:    NewCachedSource(src, cfg, time.Now),
		logger: logger,
		now:    time.Now,
	}
}

// RegisterRoutes mounts the P5 public offline delivery endpoints on the provided ServeMux.
// An optional wrapMiddleware function can be provided to apply request correlation or logging.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, wrap func(http.HandlerFunc) http.HandlerFunc) {
	apply := func(fn http.HandlerFunc) http.HandlerFunc {
		if wrap != nil {
			return wrap(fn)
		}
		return fn
	}

	mux.HandleFunc("/api/v3/regions/{id}/manifest", apply(h.HandleGetManifest))
	mux.HandleFunc("/api/v3/packages/{id}/versions/{version}", apply(h.HandleGetCard))
	mux.HandleFunc("/api/v3/packages/{id}/versions/{version}/card", apply(h.HandleGetCard))
	mux.HandleFunc("/api/v3/resources/{id}", apply(h.HandleGetResource))
}

// HandleGetManifest handles GET/HEAD /api/v3/regions/{id}/manifest.
// Returns the active regional manifest with strong ETag and short must-revalidate caching.
func (h *Handler) HandleGetManifest(w http.ResponseWriter, r *http.Request) {
	if !h.requireMethod(w, r, http.MethodGet, http.MethodHead) {
		return
	}

	jurisdiction := r.PathValue("id")
	if !validateJurisdiction(jurisdiction) {
		h.writeError(w, r, http.StatusBadRequest, contracts.ErrInvalidValue, "invalid jurisdiction identifier", "id")
		return
	}

	rec, err := h.src.GetManifest(r.Context(), jurisdiction)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.writeError(w, r, http.StatusNotFound, contracts.ErrNotFound, fmt.Sprintf("no active manifest for region %q", jurisdiction), "id")
			return
		}
		if errors.Is(err, ErrQuarantined) {
			h.writeError(w, r, http.StatusGone, contracts.ErrDataUnavailable, "regional manifest is quarantined", "id")
			return
		}
		h.logger.Error("failed to get manifest", "jurisdiction", jurisdiction, "error", err)
		h.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable, "manifest unavailable", "")
		return
	}

	if int64(len(rec.RawJSON)) > h.cfg.MaxManifestBytes {
		h.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "manifest exceeds maximum size limit", "")
		return
	}

	etag := FormatETag(rec.ChecksumSHA256)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=60, must-revalidate")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if CheckIfNoneMatch(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(rec.RawJSON)))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return
	}

	_, _ = w.Write(rec.RawJSON) // #nosec G705
}

// HandleGetCard handles GET/HEAD /api/v3/packages/{id}/versions/{version}.
// Returns the immutable public incident card projection with strong ETag and long immutable caching.
// Supports standard RFC 9110 Range and If-Range requests for resumable retrieval.
func (h *Handler) HandleGetCard(w http.ResponseWriter, r *http.Request) {
	if !h.requireMethod(w, r, http.MethodGet, http.MethodHead) {
		return
	}

	packageID := r.PathValue("id")
	if !validatePackageID(packageID) {
		h.writeError(w, r, http.StatusBadRequest, contracts.ErrInvalidValue, "invalid package identifier", "id")
		return
	}

	versionStr := r.PathValue("version")
	version, err := strconv.Atoi(versionStr)
	if err != nil || version <= 0 {
		h.writeError(w, r, http.StatusBadRequest, contracts.ErrInvalidValue, "version must be a positive integer", "version")
		return
	}

	rec, err := h.src.GetCard(r.Context(), packageID, version)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.writeError(w, r, http.StatusNotFound, contracts.ErrNotFound, fmt.Sprintf("package %q version %d not found", packageID, version), "id")
			return
		}
		if errors.Is(err, ErrQuarantined) {
			h.writeError(w, r, http.StatusGone, contracts.ErrDataUnavailable, "package version is quarantined", "id")
			return
		}
		h.logger.Error("failed to get card", "package_id", packageID, "version", version, "error", err)
		h.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable, "card unavailable", "")
		return
	}

	totalSize := int64(len(rec.RawJSON))
	if totalSize > h.cfg.MaxCardBytes {
		h.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "card exceeds maximum size limit", "")
		return
	}

	etag := FormatETag(rec.ChecksumSHA256)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if CheckIfNoneMatch(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Check Range and If-Range
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" && CheckIfRange(r, etag) {
		byteRange, hasRange, rangeErr := ParseRange(rangeHeader, totalSize)
		if errors.Is(rangeErr, errRangeUnsatisfiable) {
			w.Header().Set("Content-Range", FormatUnsatisfiableRange(totalSize))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if hasRange && rangeErr == nil {
			w.Header().Set("Content-Range", byteRange.FormatContentRange(totalSize))
			w.Header().Set("Content-Length", strconv.FormatInt(byteRange.Length, 10))
			w.WriteHeader(http.StatusPartialContent)

			if r.Method == http.MethodHead {
				return
			}
			_, _ = w.Write(rec.RawJSON[byteRange.Start : byteRange.End+1]) // #nosec G705
			return
		}
	}

	// Full content (200 OK)
	w.Header().Set("Content-Length", strconv.FormatInt(totalSize, 10))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(rec.RawJSON) // #nosec G705
}

// HandleGetResource handles GET/HEAD /api/v3/resources/{id}.
// Streams content-addressed immutable assets (map vectors, styles, fonts, audio) with resumable Range support.
func (h *Handler) HandleGetResource(w http.ResponseWriter, r *http.Request) {
	if !h.requireMethod(w, r, http.MethodGet, http.MethodHead) {
		return
	}

	resourceID := r.PathValue("id")
	if !validateResourceID(resourceID) {
		h.writeError(w, r, http.StatusBadRequest, contracts.ErrInvalidValue, "invalid resource identifier", "id")
		return
	}

	res, err := h.src.GetResource(r.Context(), resourceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.writeError(w, r, http.StatusNotFound, contracts.ErrNotFound, fmt.Sprintf("resource %q not found", resourceID), "id")
			return
		}
		if errors.Is(err, ErrQuarantined) {
			h.writeError(w, r, http.StatusGone, contracts.ErrDataUnavailable, "resource is quarantined", "id")
			return
		}
		h.logger.Error("failed to get resource", "resource_id", resourceID, "error", err)
		h.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable, "resource unavailable", "")
		return
	}
	defer res.Reader.Close()

	if res.ContentLength > h.cfg.MaxResourceBytes {
		h.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "resource exceeds maximum size limit", "")
		return
	}

	etag := res.ETag
	if etag == "" && res.ChecksumSHA256 != "" {
		etag = FormatETag(res.ChecksumSHA256)
	}

	contentType := res.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if CheckIfNoneMatch(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Check Range and If-Range
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" && CheckIfRange(r, etag) {
		byteRange, hasRange, rangeErr := ParseRange(rangeHeader, res.ContentLength)
		if errors.Is(rangeErr, errRangeUnsatisfiable) {
			w.Header().Set("Content-Range", FormatUnsatisfiableRange(res.ContentLength))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if hasRange && rangeErr == nil {
			if _, err := res.Reader.Seek(byteRange.Start, io.SeekStart); err != nil {
				h.logger.Error("seek failed on resource reader", "resource_id", resourceID, "error", err)
				h.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "failed to seek resource", "")
				return
			}

			w.Header().Set("Content-Range", byteRange.FormatContentRange(res.ContentLength))
			w.Header().Set("Content-Length", strconv.FormatInt(byteRange.Length, 10))
			w.WriteHeader(http.StatusPartialContent)

			if r.Method == http.MethodHead {
				return
			}

			h.streamWithContext(r.Context(), w, io.LimitReader(res.Reader, byteRange.Length))
			return
		}
	}

	// Full content (200 OK)
	w.Header().Set("Content-Length", strconv.FormatInt(res.ContentLength, 10))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return
	}

	h.streamWithContext(r.Context(), w, res.Reader)
}

// streamWithContext streams from src to dst in bounded chunks while monitoring request cancellation.
func (h *Handler) streamWithContext(ctx context.Context, dst io.Writer, src io.Reader) {
	buf := make([]byte, h.cfg.StreamChunkSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := src.Read(buf)
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return
			}
			if flusher, ok := dst.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

func (h *Handler) requireMethod(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	for _, m := range allowed {
		if r.Method == m {
			return true
		}
	}
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	h.writeError(w, r, http.StatusMethodNotAllowed, contracts.ErrMethodNotAllowed, fmt.Sprintf("method %s not allowed", r.Method), "")
	return false
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, status int, code, message, field string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)

	env := contracts.Envelope{
		RequestID:     r.Header.Get("X-Request-ID"),
		SchemaVersion: contracts.SchemaVersionV3,
		GeneratedAt:   h.now().UTC().Format(time.RFC3339),
		DataVersion:   "none",
		SourceStatus:  contracts.FreshnessUnknown,
		Errors: []contracts.APIError{{
			Code:          code,
			Message:       message,
			Field:         field,
			CorrelationID: r.Header.Get("X-Request-ID"),
			Retryable:     status >= 500,
		}},
	}
	_ = json.NewEncoder(w).Encode(env)
}

func validateJurisdiction(id string) bool {
	if !safeJurisdictionRegex.MatchString(id) {
		return false
	}
	return !containsTraversal(id)
}

func validatePackageID(id string) bool {
	if !safePackageIDRegex.MatchString(id) {
		return false
	}
	return !containsTraversal(id)
}

func validateResourceID(id string) bool {
	if !safeResourceIDRegex.MatchString(id) {
		return false
	}
	return !containsTraversal(id)
}

func containsTraversal(id string) bool {
	return strings.Contains(id, "..") ||
		strings.Contains(id, "/") ||
		strings.Contains(id, "\\") ||
		strings.Contains(id, "%2e") ||
		strings.Contains(id, "%2f") ||
		strings.Contains(id, "\x00")
}
