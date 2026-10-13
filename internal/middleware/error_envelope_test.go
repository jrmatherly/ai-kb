package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

type fakeAPIKeyService struct {
	interfaces.TenantAPIKeyService
	key *types.TenantAPIKey
}

func (f *fakeAPIKeyService) AuthenticateAPIKey(context.Context, string) (*types.TenantAPIKey, error) {
	return f.key, nil
}

// Each rejecting middleware runs behind Recovery and ErrorHandler in the order router.go mounts them,
// and the client must receive the AppError envelope with the old status and message.
func TestMiddlewareRejectionsWriteEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	platformKey := &fakeAPIKeyService{key: &types.TenantAPIKey{ID: 9, ScopeType: types.APIKeyScopePlatform}}

	cases := []struct {
		name     string
		handlers []gin.HandlerFunc
		header   map[string]string
		route    string
		path     string
		status   int
		body     string
	}{
		{
			name:     "auth missing credentials",
			handlers: []gin.HandlerFunc{Auth(nil, nil, nil, nil, cfg)},
			status:   http.StatusUnauthorized,
			body:     `{"success":false,"error":{"code":1001,"message":"Unauthorized: missing authentication"}}`,
		},
		{
			name:     "platform key without workspace",
			handlers: []gin.HandlerFunc{Auth(nil, nil, nil, platformKey, cfg)},
			header:   map[string]string{"X-API-Key": "sk-platform"},
			status:   http.StatusConflict,
			body: `{"success":false,"error":{"code":1005,` +
				`"message":"Workspace required: platform API keys must send X-Tenant-ID",` +
				`"details":{"reason":"TENANT_REQUIRED"}}}`,
		},
		{
			name: "tenantless JWT user",
			handlers: []gin.HandlerFunc{func(c *gin.Context) {
				if AttachAuthenticatedUser(c, nil, newFakeMemberService(), cfg, &types.User{ID: "u-1"}, 0) {
					c.Status(http.StatusNoContent)
				}
			}},
			status: http.StatusConflict,
			body: `{"success":false,"error":{"code":1005,"message":"Workspace required",` +
				`"details":{"reason":"TENANT_REQUIRED"}}}`,
		},
		{
			name: "websocket ticket without user",
			handlers: []gin.HandlerFunc{func(c *gin.Context) {
				AttachAuthenticatedUser(c, nil, nil, cfg, nil, 0)
			}},
			status: http.StatusUnauthorized,
			body:   `{"success":false,"error":{"code":1001,"message":"Unauthorized: invalid or expired token"}}`,
		},
		{
			name:     "system admin required",
			handlers: []gin.HandlerFunc{RequireSystemAdmin(cfg)},
			status:   http.StatusForbidden,
			body:     `{"success":false,"error":{"code":1002,"message":"Forbidden: system administrator required"}}`,
		},
		{
			name: "api key denied",
			handlers: []gin.HandlerFunc{
				func(c *gin.Context) {
					scope := types.TenantAPIKeyScope{FullAccess: true}
					c.Request = c.Request.WithContext(types.WithTenantAPIKeyScope(c.Request.Context(), scope))
				},
				DenyAPIKeyPrincipal(),
			},
			status: http.StatusForbidden,
			body: `{"success":false,"error":{"code":1002,` +
				`"message":"Forbidden: API keys cannot access this endpoint"}}`,
		},
		{
			name: "mcp endpoint without token",
			handlers: []gin.HandlerFunc{
				MCPEndpointAuth(&fakeMCPEndpointService{}, &fakeTenantService{}),
			},
			route:  "/mcp/:endpoint_id",
			path:   "/mcp/ep-1",
			status: http.StatusUnauthorized,
			body:   `{"success":false,"error":{"code":1001,"message":"unauthorized: missing bearer token"}}`,
		},
		{
			name:     "panic",
			handlers: []gin.HandlerFunc{func(*gin.Context) { panic("db password is hunter2") }},
			status:   http.StatusInternalServerError,
			body:     `{"success":false,"error":{"code":1007,"message":"Internal Server Error"}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(Recovery(), ErrorHandler())
			route, path := tc.route, tc.path
			if route == "" {
				route, path = "/api/v1/knowledge-bases", "/api/v1/knowledge-bases"
			}
			r.GET(route, append(tc.handlers, func(c *gin.Context) { c.Status(http.StatusNoContent) })...)

			req := httptest.NewRequest(http.MethodGet, path, nil)
			for k, v := range tc.header {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d (body=%s)", w.Code, tc.status, w.Body.String())
			}
			if got := w.Body.String(); got != tc.body {
				t.Fatalf("body = %s\nwant   %s", got, tc.body)
			}
		})
	}
}
