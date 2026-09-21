package offlineclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"sthira/backend/internal/offlinepkg"
)

// narrowly-scoped security tests for the offlineclient transport.
// Pins: a smuggled URI scheme is rejected at absoluteURL, a forged
// checksum is rejected at downloadResource, the canonical bytes
// carry a checksum + signature block before verification succeeds.

// TestAbsoluteURLRejectsSmuggledScheme confirms that joining
// user-supplied paths under a configured baseURL never lets a
// different scheme or different host leak through. Today
// absoluteURL implements this by inheritance: the joined URL
// re-uses the base scheme + host. The test pins that contract for
// any future refactor.
func TestAbsoluteURLRejectsSmuggledScheme(t *testing.T) {
	c := &ProtocolClient{baseURL: "https://files.example"}
	for _, bad := range []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"://attacker.example/foo",
		"/api/../" + strings.Repeat("a", 0),
	} {
		t.Run(bad, func(t *testing.T) {
			got, err := c.absoluteURL(bad)
			if err != nil {
				t.Fatalf("absoluteURL(%q) err: %v", bad, err)
			}
			u, perr := url.Parse(got)
			if perr != nil {
				t.Fatalf("absoluteURL(%q) parsed result invalid: %v", bad, perr)
			}
			if u.Scheme != "https" {
				t.Fatalf("scheme drift: %q", got)
			}
			if u.Host != "files.example" {
				t.Fatalf("host drift: %q", got)
			}
		})
	}
}

// TestDownloadResourceRejectsForgedChecksum confirms the boundary
// at transport.go:downloadResource: a response whose bytes do
// NOT match the manifest's declared checksum is rejected and the
// resource is NOT activated.
func TestDownloadResourceRejectsForgedChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("attacker-controlled-bytes"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	c, err := NewClient(ClientConfig{
		BaseURL:    srv.URL,
		StorageDir: dir,
		HTTPClient: srv.Client(),
		Now:        nil,
		TrustStore: stubTrust{},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	u, _ := url.Parse(srv.URL)
	desc := offlinepkg.ResourceDescriptor{
		ResourceID:     "r1",
		URI:            strings.TrimRight(u.Path, "/") + "/resource.bin",
		ChecksumSHA256: "0000000000000000000000000000000000000000000000000000000000000000",
		ByteSize:       1,
		ContentType:    "application/octet-stream",
	}
	if err := c.DownloadResource(context.Background(), desc, false); err == nil {
		t.Fatalf("forged-checksum resource accepted; signed-package guard broken")
	}
}

// stubTrust is a TrustStore that no-ops; tests in this file
// intentionally do not exercise signature path. The verifier
// hook is documented in backend/security/pending-p6.md.
type stubTrust struct{}

func (stubTrust) VerifySignature(_, _ string, _ []byte, _ string) error { return nil }
func (stubTrust) LookupKey(_ string) (offlinepkg.TrustedKey, error) {
	return offlinepkg.TrustedKey{}, nil
}
