package handler

import (
	"context"
	stderrors "errors"
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
		"blocked-url": {
			ID: "blocked-url", TenantID: 7, TransportType: types.MCPTransportSSE,
			URL: func() *string { u := "gopher://127.0.0.1/"; return &u }(),
		},
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
	router.POST("/mcp-services/:id/test", h.TestMCPService)
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
		{"update with blocked stored url", http.MethodPut, "/mcp-services/blocked-url", `{"enabled":false}`, 400},
		{"update unknown", http.MethodPut, "/mcp-services/missing", `{"name":"n"}`, 404},
		{"update builtin", http.MethodPut, "/mcp-services/builtin", `{"name":"n"}`, 403},
		{"delete unknown", http.MethodDelete, "/mcp-services/missing", "", 404},
		{"delete builtin", http.MethodDelete, "/mcp-services/builtin", "", 403},
		{"test unknown", http.MethodPost, "/mcp-services/missing/test", "", 404},
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

type mcpServiceRepoFailing struct {
	interfaces.MCPServiceRepository
}

func (mcpServiceRepoFailing) GetByID(context.Context, uint64, string) (*types.MCPService, error) {
	return nil, stderrors.New("pq: connection refused secret-host:5432")
}

func TestMCPServiceRepositoryFailuresReturn500WithoutDriverText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewMCPServiceService(mcpServiceRepoFailing{}, nil, nil)
	h := &MCPServiceHandler{mcpServiceService: svc}
	creds := NewMCPCredentialsHandler(svc)
	oauth := NewMCPOAuthHandler(nil, nil, svc, nil)

	router := gin.New()
	router.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		principal := types.Principal{Type: types.PrincipalWebUser, ID: "u-1"}
		c.Request = c.Request.WithContext(types.WithPrincipal(c.Request.Context(), principal))
		c.Next()
	})
	router.GET("/mcp-services/:id", h.GetMCPService)
	router.PUT("/mcp-services/:id", h.UpdateMCPService)
	router.DELETE("/mcp-services/:id", h.DeleteMCPService)
	router.POST("/mcp-services/:id/test", h.TestMCPService)
	router.GET("/mcp-services/:id/tools", h.GetMCPServiceTools)
	router.GET("/mcp-services/:id/resources", h.GetMCPServiceResources)
	router.PUT("/mcp-services/:id/credentials", creds.Put)
	router.DELETE("/mcp-services/:id/credentials/:field", creds.DeleteField)
	router.POST("/mcp-services/:id/metadata/refresh", h.RefreshMCPMetadata)
	router.POST("/mcp-services/:id/usage-instructions/generate", h.GenerateMCPUsageInstructions)
	router.POST("/mcp-services/:id/oauth/authorize-url", oauth.AuthorizeURL)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"get", http.MethodGet, "/mcp-services/s1", ""},
		{"update", http.MethodPut, "/mcp-services/s1", `{"name":"n"}`},
		{"delete", http.MethodDelete, "/mcp-services/s1", ""},
		{"test", http.MethodPost, "/mcp-services/s1/test", ""},
		{"tools", http.MethodGet, "/mcp-services/s1/tools", ""},
		{"resources", http.MethodGet, "/mcp-services/s1/resources", ""},
		{"credentials no-op", http.MethodPut, "/mcp-services/s1/credentials", `{}`},
		{"credentials write", http.MethodPut, "/mcp-services/s1/credentials", `{"token":"t"}`},
		{"clear credential", http.MethodDelete, "/mcp-services/s1/credentials/token", ""},
		{"metadata refresh", http.MethodPost, "/mcp-services/s1/metadata/refresh", ""},
		{"usage instructions", http.MethodPost, "/mcp-services/s1/usage-instructions/generate", `{}`},
		{"oauth authorize", http.MethodPost, "/mcp-services/s1/oauth/authorize-url", `{"redirect_uri":"https://h/cb"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, request)
			require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
			require.NotContains(t, w.Body.String(), "secret-host")
			require.NotContains(t, w.Body.String(), "pq:")
		})
	}
}
