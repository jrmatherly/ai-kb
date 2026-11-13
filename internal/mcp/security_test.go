package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

func TestValidateServiceOutboundURLsRejectsOAuthMetadataSSRF(t *testing.T) {
	serviceURL := "https://example.com/mcp"
	err := ValidateServiceOutboundURLs(&types.MCPService{
		URL: &serviceURL,
		AuthConfig: &types.MCPAuthConfig{
			AuthServerMetadataURL: "http://169.254.169.254/latest/meta-data",
		},
	})
	if err == nil {
		t.Fatal("expected OAuth metadata URL to be rejected")
	}
}

func TestNewMCPClientRejectsStoredUnsafeURL(t *testing.T) {
	serviceURL := "http://127.0.0.1:8080/mcp"
	_, err := NewMCPClient(&ClientConfig{Service: &types.MCPService{
		URL:           &serviceURL,
		TransportType: types.MCPTransportHTTPStreamable,
	}})
	if err == nil {
		t.Fatal("expected stored unsafe MCP URL to be rejected at client construction")
	}
}

// GO-2026-6911 (scripts/govulncheck-allowlist.txt): mcp-go forwards headers to a
// redirect target unless the caller supplies the HTTP client. This fails if
// NewMCPClient stops supplying the SSRF-safe one.
func TestMCPClientDoesNotFollowCrossOriginRedirect(t *testing.T) {
	t.Setenv("SSRF_WHITELIST", "127.0.0.1")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)

	var reached atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { reached.Store(true) }))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusFound)
	}))
	defer server.Close()

	serviceURL := server.URL
	client, err := NewMCPClient(&ClientConfig{Service: &types.MCPService{
		ID:            "svc-redirect",
		TransportType: types.MCPTransportHTTPStreamable,
		URL:           &serviceURL,
		Headers:       map[string]string{"X-Api-Key": "synthetic"},
	}})
	if err != nil {
		t.Fatalf("NewMCPClient: %v", err)
	}
	ctx := context.Background()
	if err := client.Connect(ctx); err == nil {
		defer func() { _ = client.Disconnect() }()
		if _, err := client.Initialize(ctx); err == nil {
			t.Fatal("Initialize followed a cross-origin redirect")
		}
	}
	if reached.Load() {
		t.Fatal("cross-origin redirect target was reached")
	}
}
