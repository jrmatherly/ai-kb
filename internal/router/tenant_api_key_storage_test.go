package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/handler"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type apiKeyTestTenantService struct{ interfaces.TenantService }

func (apiKeyTestTenantService) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	return &types.Tenant{ID: id}, nil
}

type apiKeyTestUserService struct{ interfaces.UserService }

func (apiKeyTestUserService) GetUserByTenantID(context.Context, uint64) (*types.User, error) {
	return nil, fmt.Errorf("no user")
}

// TestTenantAPIKeyStoredAsHashAndHint creates a key through the tenant routes, authenticates
// with the returned token, lists the masked value, and checks that no column holds the token.
func TestTenantAPIKeyStoredAsHashAndHint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.TenantAPIKey{}))
	keys := service.NewTenantAPIKeyService(repository.NewTenantAPIKeyRepository(db))

	enabled := true
	cfg := &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}
	auth := middleware.Auth(apiKeyTestTenantService{}, apiKeyTestUserService{}, nil, keys, cfg)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if c.GetHeader("X-API-Key") != "" {
			auth(c)
			return
		}
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleOwner)
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Next()
	})
	v1 := r.Group("/api/v1")
	tenantHandler := handler.NewTenantHandler(nil, keys, nil, nil, nil, cfg, nil)
	RegisterTenantRoutes(v1, tenantHandler, nil, nil, nil, &rbacGuards{cfg: cfg})
	v1.GET("/whoami", func(c *gin.Context) {
		scope, _ := types.TenantAPIKeyScopeFromContext(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"key_id": scope.KeyID})
	})

	serve := func(method, path, body, apiKey string) (int, map[string]any) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("X-API-Key", apiKey)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
		return w.Code, resp
	}

	code, created := serve(http.MethodPost, "/api/v1/tenants/7/api-keys", `{"name":"ci","full_access":true}`, "")
	require.Equal(t, http.StatusCreated, code, created)
	data := created["data"].(map[string]any)
	token := data["token"].(string)
	require.True(t, strings.HasPrefix(token, "sk-"), token)
	mask := token[:7] + "..." + token[len(token)-4:]

	code, who := serve(http.MethodGet, "/api/v1/whoami", "", token)
	require.Equal(t, http.StatusOK, code, who)
	require.Equal(t, data["id"], who["key_id"])

	code, _ = serve(http.MethodGet, "/api/v1/whoami", "", token+"x")
	require.Equal(t, http.StatusUnauthorized, code)

	code, listed := serve(http.MethodGet, "/api/v1/tenants/7/api-keys", "", "")
	require.Equal(t, http.StatusOK, code, listed)
	items := listed["data"].([]any)
	require.Len(t, items, 1)
	require.Equal(t, mask, items[0].(map[string]any)["api_key"])

	var rows []map[string]any
	require.NoError(t, db.Raw("SELECT * FROM tenant_api_keys").Scan(&rows).Error)
	require.Len(t, rows, 1)
	for column, value := range rows[0] {
		require.NotContains(t, fmt.Sprint(value), token[3:], "column %s holds the token", column)
	}
}
