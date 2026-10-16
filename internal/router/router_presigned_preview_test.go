package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

func servePresignedPreviewForTest(t *testing.T, catalog interfaces.ResourceCatalog, filePath string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("LOCAL_STORAGE_BASE_DIR", t.TempDir())
	engine := newFilesTestEngine()
	servePresignedPreview(engine, nil, nil, catalog)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/files/presigned-preview?file_path="+url.QueryEscape(filePath), nil)
	ctx := context.WithValue(req.Context(), types.TenantInfoContextKey, &types.Tenant{ID: 42})
	ctx = types.WithCaller(ctx, types.Caller{TenantID: 42, UserID: "admin", Role: types.TenantRoleAdmin})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req.WithContext(ctx))
	return recorder.Code
}

// A known handle of another tenant's resource must not be exchangeable for an
// anonymous /r/ grant (the stub catalog panics if a grant is minted).
func TestPresignedPreviewRejectsCrossTenantResource(t *testing.T) {
	catalog := &stubResourceCatalog{resource: &types.StoredResource{
		TenantID: 7, PhysicalPath: "local://7/exports/a.png",
	}}
	code := servePresignedPreviewForTest(t, catalog, "resource://AbCdEfGhIjKlMnOpQrStUv")
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", code, http.StatusForbidden)
	}
}

func TestPresignedPreviewRejectsCrossTenantPath(t *testing.T) {
	for _, filePath := range []string{"local://7/knowledge/secret.pdf", "local://docs/example.txt"} {
		if code := servePresignedPreviewForTest(t, &stubResourceCatalog{}, filePath); code != http.StatusForbidden {
			t.Fatalf("%s: status = %d, want %d", filePath, code, http.StatusForbidden)
		}
	}
}

// Without a catalog a resource reference cannot be attributed to a tenant, so
// it is refused rather than signed.
func TestPresignedPreviewRejectsUnresolvableResource(t *testing.T) {
	if code := servePresignedPreviewForTest(t, nil, "resource://AbCdEfGhIjKlMnOpQrStUv"); code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", code, http.StatusForbidden)
	}
}

// An unconfigured provider is the operator's mistake: the envelope's details
// name the provider and say what to fix.
func TestPresignedPreviewUnconfiguredProviderReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("LOCAL_STORAGE_BASE_DIR", t.TempDir())
	engine := newFilesTestEngine()
	servePresignedPreview(engine, nil, nil, &stubResourceCatalog{})

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/files/presigned-preview?file_path="+url.QueryEscape("minio://42/exports/a.png"), nil)
	ctx := context.WithValue(req.Context(), types.TenantInfoContextKey, &types.Tenant{ID: 42})
	ctx = types.WithCaller(ctx, types.Caller{TenantID: 42, UserID: "admin", Role: types.TenantRoleAdmin})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req.WithContext(ctx))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	want := `{"success":false,"error":{"code":1000,"message":"storage backend is unavailable","details":` +
		`{"hint":"workspace storage config is missing or incomplete for this provider","provider":"minio"}}}`
	if got := strings.TrimSpace(recorder.Body.String()); got != want {
		t.Fatalf("body:\n got %s\nwant %s", got, want)
	}
}

func TestPresignedPreviewAllowsOwnTenantPath(t *testing.T) {
	code := servePresignedPreviewForTest(t, &stubResourceCatalog{}, "local://42/exports/a.png")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
}
