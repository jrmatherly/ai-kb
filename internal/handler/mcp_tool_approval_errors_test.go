package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

// failingApprovalRepo fails the way a GORM repository does, with text that
// contains "not found".
type failingApprovalRepo struct {
	interfaces.MCPToolApprovalRepository
}

func (failingApprovalRepo) ListByService(context.Context, uint64, string) ([]*types.MCPToolApproval, error) {
	return nil, fmt.Errorf("list mcp tool approvals: %w", gorm.ErrRecordNotFound)
}

func (failingApprovalRepo) UpsertPolicy(context.Context, uint64, string, string, types.MCPToolPolicyPatch) error {
	return fmt.Errorf("upsert mcp tool policy: %w", gorm.ErrRecordNotFound)
}

func TestMCPToolApprovalErrorsSplitMissingServiceFromRepositoryFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	services := &mcpServiceRepoStub{services: map[string]*types.MCPService{"known": {ID: "known", TenantID: 7}}}
	svc := service.NewMCPToolApprovalService(failingApprovalRepo{}, services)
	h := &MCPServiceHandler{mcpToolApprovalService: svc}
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Next()
	})
	r.GET("/mcp-services/:id/tool-approvals", h.ListMCPToolApprovals)
	r.PUT("/mcp-services/:id/tool-approvals/:tool_name", h.SetMCPToolApproval)

	notFound := `{"success":false,"error":{"code":1003,"message":"MCP service not found"}}`
	internal := `{"success":false,"error":{"code":1007,"message":"Internal server error"}}`
	for _, tc := range []struct {
		name, method, path string
		status             int
		want               string
	}{
		{"list unknown service", http.MethodGet, "/mcp-services/missing/tool-approvals", 404, notFound},
		{"list repository failure", http.MethodGet, "/mcp-services/known/tool-approvals", 500, internal},
		{"set unknown service", http.MethodPut, "/mcp-services/missing/tool-approvals/t", 404, notFound},
		{"set repository failure", http.MethodPut, "/mcp-services/known/tool-approvals/t", 500, internal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"enabled":true}`)))
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.JSONEq(t, tc.want, w.Body.String())
		})
	}
}
