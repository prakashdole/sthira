package offlinedelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
)

func setupTestServer(t *testing.T, src PublicationSource, cfg Config) (*httptest.Server, *Handler) {
	t.Helper()
	h := NewHandler(cfg, src, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Request-ID", "test-req-123")
			next(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, h
}

// 1. Unchanged ETag returns 304 Not Modified
func TestManifestUnchangedETag_304(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	// First request: get ETag
	resp, err := http.Get(srv.URL + "/api/v3/regions/KL/manifest")
	if err != nil {
		t.Fatalf("GET /manifest failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatalf("expected non-empty ETag header")
	}

	// Second request: with If-None-Match
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v3/regions/KL/manifest", nil)
	req.Header.Set("If-None-Match", etag)
	client := &http.Client{}
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatalf("conditional GET failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("expected 304 Not Modified, got %d", resp2.StatusCode)
	}
	body, _ := io.ReadAll(resp2.Body)
	if len(body) != 0 {
		t.Errorf("expected empty body for 304, got %d bytes", len(body))
	}
	if resp2.Header.Get("ETag") != etag {
		t.Errorf("expected ETag %s in 304 response, got %s", etag, resp2.Header.Get("ETag"))
	}
}

// 2. Changed ETag returns 200 OK with fresh body
func TestManifestChangedETag_200(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v3/regions/KL/manifest", nil)
	req.Header.Set("If-None-Match", `"sha256-old-stale-checksum"`)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Errorf("expected non-empty body for changed ETag")
	}
}

// 3. Full card download with immutable headers
func TestCardFullDownload_200(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	resp, err := http.Get(srv.URL + "/api/v3/packages/PKG-WAYANAD-01/versions/1")
	if err != nil {
		t.Fatalf("GET card failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("expected immutable Cache-Control, got %s", cc)
	}
	if resp.Header.Get("Accept-Ranges") != "bytes" {
		t.Errorf("expected Accept-Ranges: bytes, got %s", resp.Header.Get("Accept-Ranges"))
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("Chooralmala")) {
		t.Errorf("expected card body to contain Chooralmala")
	}
}

// 4. Partial card download via Range header (206 Partial Content)
func TestCardPartialDownload_206(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	// First get full card to compare
	fullResp, _ := http.Get(srv.URL + "/api/v3/packages/PKG-WAYANAD-01/versions/1")
	fullBody, _ := io.ReadAll(fullResp.Body)
	fullResp.Body.Close()
	totalLen := int64(len(fullBody))

	// Request bytes=0-49 (first 50 bytes)
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v3/packages/PKG-WAYANAD-01/versions/1", nil)
	req.Header.Set("Range", "bytes=0-49")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Range request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content, got %d", resp.StatusCode)
	}
	expectedCR := fmt.Sprintf("bytes 0-49/%d", totalLen)
	if cr := resp.Header.Get("Content-Range"); cr != expectedCR {
		t.Errorf("expected Content-Range %s, got %s", expectedCR, cr)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "50" {
		t.Errorf("expected Content-Length: 50, got %s", cl)
	}
	partBody, _ := io.ReadAll(resp.Body)
	if len(partBody) != 50 {
		t.Fatalf("expected 50 bytes, got %d", len(partBody))
	}
	if !bytes.Equal(partBody, fullBody[0:50]) {
		t.Errorf("partial body does not match expected slice")
	}
}

// 5. Card suffix range (bytes=-50)
func TestCardSuffixRange_206(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	fullResp, _ := http.Get(srv.URL + "/api/v3/packages/PKG-WAYANAD-01/versions/1")
	fullBody, _ := io.ReadAll(fullResp.Body)
	fullResp.Body.Close()
	totalLen := int64(len(fullBody))

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v3/packages/PKG-WAYANAD-01/versions/1", nil)
	req.Header.Set("Range", "bytes=-50")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Suffix range request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content, got %d", resp.StatusCode)
	}
	expectedCR := fmt.Sprintf("bytes %d-%d/%d", totalLen-50, totalLen-1, totalLen)
	if cr := resp.Header.Get("Content-Range"); cr != expectedCR {
		t.Errorf("expected Content-Range %s, got %s", expectedCR, cr)
	}
	partBody, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(partBody, fullBody[totalLen-50:]) {
		t.Errorf("suffix range body mismatch")
	}
}

// 6. Card open-ended range (bytes=100-)
func TestCardOpenEndedRange_206(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	fullResp, _ := http.Get(srv.URL + "/api/v3/packages/PKG-WAYANAD-01/versions/1")
	fullBody, _ := io.ReadAll(fullResp.Body)
	fullResp.Body.Close()
	totalLen := int64(len(fullBody))

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v3/packages/PKG-WAYANAD-01/versions/1", nil)
	req.Header.Set("Range", "bytes=100-")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Open-ended range request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content, got %d", resp.StatusCode)
	}
	expectedCR := fmt.Sprintf("bytes 100-%d/%d", totalLen-1, totalLen)
	if cr := resp.Header.Get("Content-Range"); cr != expectedCR {
		t.Errorf("expected Content-Range %s, got %s", expectedCR, cr)
	}
	partBody, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(partBody, fullBody[100:]) {
		t.Errorf("open-ended range body mismatch")
	}
}

// 7. Invalid/unsatisfiable range returns 416 Range Not Satisfiable
func TestCardInvalidRange_416(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v3/packages/PKG-WAYANAD-01/versions/1", nil)
	req.Header.Set("Range", "bytes=9999999-")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("expected 416 Range Not Satisfiable, got %d", resp.StatusCode)
	}
	cr := resp.Header.Get("Content-Range")
	if !strings.HasPrefix(cr, "bytes */") {
		t.Errorf("expected Content-Range bytes */total, got %s", cr)
	}
}

// 8. If-Range matching ETag returns 206 Partial Content
func TestCardIfRangeMatch_206(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	// Get entity tag first
	headResp, _ := http.Head(srv.URL + "/api/v3/packages/PKG-WAYANAD-01/versions/1")
	etag := headResp.Header.Get("ETag")

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v3/packages/PKG-WAYANAD-01/versions/1", nil)
	req.Header.Set("Range", "bytes=0-10")
	req.Header.Set("If-Range", etag)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content on matching If-Range, got %d", resp.StatusCode)
	}
}

// 9. If-Range mismatch ignores Range and returns 200 OK with full representation
func TestCardIfRangeMismatch_200(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v3/packages/PKG-WAYANAD-01/versions/1", nil)
	req.Header.Set("Range", "bytes=0-10")
	req.Header.Set("If-Range", `"sha256-wrong-or-drifted-etag"`)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on mismatched If-Range, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) <= 11 {
		t.Errorf("expected full entity body, got %d bytes", len(body))
	}
}

// 10. Resumable download of content-addressed binary resource
func TestResourceResumableDownload(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	// Full fetch to compare
	fullResp, err := http.Get(srv.URL + "/api/v3/resources/RES-MAP-KL-WAYANAD-v1")
	if err != nil {
		t.Fatalf("GET resource failed: %v", err)
	}
	fullBytes, _ := io.ReadAll(fullResp.Body)
	fullResp.Body.Close()

	if fullResp.Header.Get("Content-Type") != "application/vnd.mapbox-vector-tile" {
		t.Errorf("expected vector tile mime type, got %s", fullResp.Header.Get("Content-Type"))
	}
	if fullResp.Header.Get("Accept-Ranges") != "bytes" {
		t.Errorf("expected Accept-Ranges: bytes")
	}

	totalSize := int64(len(fullBytes))

	// Simulate first partial chunk: 0 to 1023 (1 KiB)
	req1, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v3/resources/RES-MAP-KL-WAYANAD-v1", nil)
	req1.Header.Set("Range", "bytes=0-1023")
	client := &http.Client{}
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("chunk 1 failed: %v", err)
	}
	chunk1, _ := io.ReadAll(resp1.Body)
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 for chunk 1, got %d", resp1.StatusCode)
	}
	if len(chunk1) != 1024 {
		t.Fatalf("expected 1024 bytes, got %d", len(chunk1))
	}

	// Simulate resumption from byte 1024 to end
	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v3/resources/RES-MAP-KL-WAYANAD-v1", nil)
	req2.Header.Set("Range", "bytes=1024-")
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("chunk 2 failed: %v", err)
	}
	chunk2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 for chunk 2, got %d", resp2.StatusCode)
	}

	// Reassemble and verify integrity
	reassembled := append(chunk1, chunk2...)
	if int64(len(reassembled)) != totalSize {
		t.Fatalf("reassembled size %d != totalSize %d", len(reassembled), totalSize)
	}
	if !bytes.Equal(reassembled, fullBytes) {
		t.Errorf("reassembled stream corrupted")
	}
}

// 11. Path traversal attempts rejected with 400 Bad Request
func TestPathTraversalRejection(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	traversalTargets := []string{
		"/api/v3/regions/../manifest",
		"/api/v3/regions/KL%2f..%2fetc/manifest",
		"/api/v3/packages/../../versions/1",
		"/api/v3/resources/..%2e%2fpasswd",
	}

	client := &http.Client{}
	for _, target := range traversalTargets {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+target, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request %s failed: %v", target, err)
		}
		resp.Body.Close()
		// Cleaned paths may 404 or our handler rejects with 400 Bad Request
		if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
			t.Errorf("target %s expected 400 or 404, got %d", target, resp.StatusCode)
		}
	}
}

// 12. Invalid identifiers rejected with 400 Bad Request
func TestInvalidIdentifiers(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	// Non-numeric version
	resp, err := http.Get(srv.URL + "/api/v3/packages/PKG-1/versions/abc")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for non-integer version, got %d", resp.StatusCode)
	}

	// Negative/zero version
	resp2, err := http.Get(srv.URL + "/api/v3/packages/PKG-1/versions/0")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for zero version, got %d", resp2.StatusCode)
	}
}

// 13. Not found returns standard contracts.Envelope error
func TestNotFoundResponses(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	resp, err := http.Get(srv.URL + "/api/v3/regions/UNKNOWN/manifest")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", resp.StatusCode)
	}

	var env contracts.Envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode error envelope: %v", err)
	}
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrNotFound {
		t.Errorf("expected error code %s, got %+v", contracts.ErrNotFound, env.Errors)
	}
}

// 14. Method not allowed returns 405 with Allow header
func TestMethodNotAllowed(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	resp, err := http.Post(srv.URL+"/api/v3/regions/KL/manifest", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 Method Not Allowed, got %d", resp.StatusCode)
	}
	if allow := resp.Header.Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("expected Allow header to contain GET, got %s", allow)
	}
}

// 15. HEAD requests return headers with zero body bytes
func TestHeadRequests(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	resp, err := http.Head(srv.URL + "/api/v3/regions/KL/manifest")
	if err != nil {
		t.Fatalf("HEAD failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for HEAD, got %d", resp.StatusCode)
	}
	if resp.Header.Get("ETag") == "" {
		t.Errorf("expected ETag header on HEAD")
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Errorf("expected 0 body bytes on HEAD, got %d", len(body))
	}
}

// 16. Private data exclusion: headers and tokens are never leaked or echoed
func TestPrivateDataExclusion(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v3/regions/KL/manifest", nil)
	// Client mistakenly sends private session auth
	req.Header.Set("Authorization", "Bearer secret-citizen-session-token-999")
	req.Header.Set("Cookie", "session=private-citizen-cookie")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Response must NOT echo private data
	if strings.Contains(string(body), "secret-citizen-session-token-999") {
		t.Errorf("private session token leaked in body!")
	}
	if strings.Contains(string(body), "private-citizen-cookie") {
		t.Errorf("private cookie leaked in body!")
	}
	if resp.Header.Get("Set-Cookie") != "" {
		t.Errorf("unexpected Set-Cookie on public delivery endpoint")
	}
	// Cache headers must remain public
	if !strings.Contains(resp.Header.Get("Cache-Control"), "public") {
		t.Errorf("expected public Cache-Control, got %s", resp.Header.Get("Cache-Control"))
	}
}

// 17. Cache headers distinguish mutable manifest from immutable card/resources
func TestCacheHeaders_MutableVsImmutable(t *testing.T) {
	src := newMockPublicationSource()
	srv, _ := setupTestServer(t, src, DefaultConfig())

	// Manifest: mutable with must-revalidate
	respM, _ := http.Get(srv.URL + "/api/v3/regions/KL/manifest")
	respM.Body.Close()
	ccM := respM.Header.Get("Cache-Control")
	if !strings.Contains(ccM, "max-age=60") || !strings.Contains(ccM, "must-revalidate") {
		t.Errorf("expected mutable cache policy for manifest, got %s", ccM)
	}

	// Card: immutable
	respC, _ := http.Get(srv.URL + "/api/v3/packages/PKG-WAYANAD-01/versions/1")
	respC.Body.Close()
	ccC := respC.Header.Get("Cache-Control")
	if !strings.Contains(ccC, "immutable") || !strings.Contains(ccC, "max-age=31536000") {
		t.Errorf("expected immutable cache policy for card, got %s", ccC)
	}

	// Resource: immutable
	respR, _ := http.Get(srv.URL + "/api/v3/resources/RES-MAP-KL-WAYANAD-v1")
	respR.Body.Close()
	ccR := respR.Header.Get("Cache-Control")
	if !strings.Contains(ccR, "immutable") || !strings.Contains(ccR, "max-age=31536000") {
		t.Errorf("expected immutable cache policy for resource, got %s", ccR)
	}
}

// 18. Demonstrate repeated client downloads do not cause proportional upstream calls
func TestUpstreamCallEfficiency(t *testing.T) {
	src := newMockPublicationSource()
	cfg := DefaultConfig()
	cfg.CardCacheTTL = 5 * time.Minute
	cfg.ManifestCacheTTL = 5 * time.Minute
	srv, _ := setupTestServer(t, src, cfg)

	client := &http.Client{}
	const requestsCount = 50

	// 50 concurrent card requests
	var wg sync.WaitGroup
	for i := 0; i < requestsCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(srv.URL + "/api/v3/packages/PKG-WAYANAD-01/versions/1")
			if err != nil {
				t.Errorf("card fetch error: %v", err)
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()
	}
	wg.Wait()

	// Upstream card query should have been called only once (or very small count under initial race)
	cardCalls := src.cardCalls.Load()
	if cardCalls > 3 {
		t.Errorf("expected upstream card calls to be amortized (<=3), got %d for %d requests", cardCalls, requestsCount)
	}

	// 50 sequential manifest requests
	for i := 0; i < requestsCount; i++ {
		resp, err := client.Get(srv.URL + "/api/v3/regions/KL/manifest")
		if err != nil {
			t.Fatalf("manifest fetch error: %v", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	manifestCalls := src.manifestCalls.Load()
	if manifestCalls != 1 {
		t.Errorf("expected exactly 1 upstream manifest call for cached manifest, got %d", manifestCalls)
	}
}

// 19. Size limits rejection (prevent memory exhaustion)
func TestPayloadSizeLimits(t *testing.T) {
	src := newMockPublicationSource()
	// Set artificial tight limits (100 bytes)
	cfg := DefaultConfig()
	cfg.MaxCardBytes = 100
	cfg.MaxManifestBytes = 100
	srv, _ := setupTestServer(t, src, cfg)

	resp, err := http.Get(srv.URL + "/api/v3/packages/PKG-WAYANAD-01/versions/1")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 Internal Server Error when card exceeds limit, got %d", resp.StatusCode)
	}
}

// 20. Client cancellation terminates stream cleanly
func TestClientCancellation(t *testing.T) {
	src := newMockPublicationSource()
	// Large resource
	largeData := make([]byte, 1024*1024) // 1 MiB
	slow := newSlowReader(largeData)
	src.setResource("RES-SLOW-STREAM", largeData, "application/octet-stream")

	// Override with slow reader
	src.mu.Lock()
	src.resources["RES-SLOW-STREAM"] = largeData
	src.mu.Unlock()

	srv, h := setupTestServer(t, src, DefaultConfig())
	_ = h

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v3/resources/RES-SLOW-STREAM", nil)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read small portion then cancel context
	buf := make([]byte, 512)
	_, _ = resp.Body.Read(buf)
	cancel() // Cancel request mid-stream

	// Reader should terminate without hanging
	doneCh := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		close(doneCh)
	}()

	select {
	case <-doneCh:
		// Succeeded
	case <-time.After(2 * time.Second):
		t.Errorf("stream did not terminate promptly after context cancellation")
	}
	slow.Close()
}
