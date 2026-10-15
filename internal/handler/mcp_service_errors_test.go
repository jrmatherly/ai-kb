package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type mcpServiceRepoStub struct {
	interfaces.MCPServiceRepository
	services map[string]*types.MCPService
}

func (r *mcpServiceRepoStub) GetByID(_ context.Context, _ uint64, id string) (*types.MCPService, error) {
	return r.services[id], nil
}

func TestMCPServiceCallerErrorsReturn4xx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &mcpServiceRepoStub{services: map[string]*types.MCPService{
		"custom":  {ID: "custom", TenantID: 7, TransportType: types.MCPTransportSSE},
		"builtin": {ID: "builtin", IsBuiltin: true, TransportType: types.MCPTransportSSE},
	}}
	svc := service.NewMCPServiceService(repo, nil, nil)
	h := &MCPServiceHandler{mcpServiceService: svc}
	creds := NewMCPCredentialsHandler(svc)

	router := gin.New()
	router.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Next()
	})
	router.POST("/mcp-services", h.CreateMCPService)
	router.PUT("/mcp-services/:id", h.UpdateMCPService)
	router.DELETE("/mcp-services/:id", h.DeleteMCPService)
	router.GET("/mcp-services/:id/tools", h.GetMCPServiceTools)
	router.GET("/mcp-services/:id/resources", h.GetMCPServiceResources)
	router.PUT("/mcp-services/:id/credentials", creds.Put)
	router.DELETE("/mcp-services/:id/credentials/:field", creds.DeleteField)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"create stdio", http.MethodPost, "/mcp-services", `{"name":"s","transport_type":"stdio"}`, 400},
		{"update to stdio", http.MethodPut, "/mcp-services/custom", `{"transport_type":"stdio"}`, 400},
		{"update unknown", http.MethodPut, "/mcp-services/missing", `{"name":"n"}`, 404},
		{"update builtin", http.MethodPut, "/mcp-services/builtin", `{"name":"n"}`, 403},
		{"delete unknown", http.MethodDelete, "/mcp-services/missing", "", 404},
		{"delete builtin", http.MethodDelete, "/mcp-services/builtin", "", 403},
		{"tools unknown", http.MethodGet, "/mcp-services/missing/tools", "", 404},
		{"resources unknown", http.MethodGet, "/mcp-services/missing/resources", "", 404},
		{"credentials unknown", http.MethodPut, "/mcp-services/missing/credentials", `{"token":"t"}`, 404},
		{"credentials builtin", http.MethodPut, "/mcp-services/builtin/credentials", `{"token":"t"}`, 403},
		{"clear credential unknown", http.MethodDelete, "/mcp-services/missing/credentials/token", "", 404},
		{"clear credential builtin", http.MethodDelete, "/mcp-services/builtin/credentials/token", "", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, request)
			require.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}
