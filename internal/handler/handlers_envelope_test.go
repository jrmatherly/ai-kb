package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type rejectingLoginUserService struct{ interfaces.UserService }

func (rejectingLoginUserService) Login(context.Context, *types.LoginRequest) (*types.LoginResponse, error) {
	return &types.LoginResponse{Success: false, Message: "Invalid email or password"}, nil
}

func TestHandlerFailuresReturnErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	withTenant := func(c *gin.Context) { c.Set(types.TenantIDContextKey.String(), uint64(1)) }

	vectorStores := &VectorStoreHandler{}
	r.GET("/vector-stores", vectorStores.ListStores)
	r.PUT("/tenant/vector-stores/:id", withTenant, vectorStores.UpdateStore)
	r.GET("/web-search-providers", (&WebSearchProviderHandler{}).ListProviders)
	r.POST("/system/sandbox-check", (&SystemHandler{}).CheckSandboxConfig)
	r.POST("/system/docreader/reconnect", (&SystemHandler{}).ReconnectDocReader)
	r.POST("/auth/login", (&AuthHandler{userService: rejectingLoginUserService{}}).Login)
	r.POST("/knowledge/file", func(c *gin.Context) {
		existing := &types.Knowledge{ID: "k-old"}
		(&KnowledgeHandler{}).handleDuplicateKnowledgeError(c, types.NewDuplicateFileError(existing), existing, "file")
	})
	r.PUT("/sandbox-configs/:id", func(c *gin.Context) {
		respondSandboxConfigRefusal(c, repository.ErrSandboxConfigCordoned)
	})

	cases := []struct {
		method, path, body string
		wantStatus         int
		wantBody           string
	}{
		{
			http.MethodGet, "/vector-stores", "", http.StatusUnauthorized,
			`{"success":false,"error":{"code":1001,"message":"unauthorized: workspace context missing"}}`,
		},
		{
			http.MethodPut, "/tenant/vector-stores/" + types.EnvStoreIDPrefix + "x", `{"name":"n"}`,
			http.StatusBadRequest,
			`{"success":false,"error":{"code":1000,` +
				`"message":"environment-configured vector stores cannot be modified via API"}}`,
		},
		{
			http.MethodGet, "/web-search-providers", "", http.StatusUnauthorized,
			`{"success":false,"error":{"code":1001,"message":"unauthorized: workspace context missing"}}`,
		},
		{
			http.MethodPost, "/system/sandbox-check", `{`, http.StatusBadRequest,
			`{"success":false,"error":{"code":1000,"message":"Malformed request body"}}`,
		},
		{
			http.MethodPost, "/system/docreader/reconnect", `{"addr":"  "}`, http.StatusBadRequest,
			`{"success":false,"error":{"code":1000,"message":"addr must not be empty"}}`,
		},
		{
			http.MethodPost, "/auth/login", `{"email":"a@example.com","password":"secret1"}`,
			http.StatusUnauthorized,
			`{"success":false,"error":{"code":1001,"message":"Invalid email or password"}}`,
		},
		{
			http.MethodPost, "/knowledge/file", "", http.StatusConflict,
			`{"success":false,"error":{"code":1005,"message":"File already exists: ",` +
				`"details":{"code":"duplicate_file","data":` + knowledgeJSON(t, &types.Knowledge{ID: "k-old"}) + `}}}`,
		},
		{
			http.MethodPut, "/sandbox-configs/cfg-a", "", http.StatusLocked,
			`{"success":false,"error":{"code":1005,"message":"Someone else is modifying this config; retry later",` +
				`"details":{"code":"sandbox_config_cordoned"}}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.wantStatus, w.Code)
			require.JSONEq(t, tc.wantBody, w.Body.String())
		})
	}
}

func knowledgeJSON(t *testing.T, k *types.Knowledge) string {
	t.Helper()
	b, err := json.Marshal(k)
	require.NoError(t, err)
	return string(b)
}
