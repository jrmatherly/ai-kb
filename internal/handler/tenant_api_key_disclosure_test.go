package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

const disclosureTestToken = "sk-Ab12cdefghijklmnopqrstuvwxyz0123456789ABwxyz"

type stubTenantAPIKeyService struct {
	interfaces.TenantAPIKeyService
}

func (stubTenantAPIKeyService) storedKey() *types.TenantAPIKey {
	tenantID := uint64(7)
	return &types.TenantAPIKey{
		ID: 3, TenantID: &tenantID, ScopeType: types.APIKeyScopeTenant,
		Name: "ci", KeyHint: types.TenantAPIKeyHint(disclosureTestToken), FullAccess: true,
	}
}

func (s stubTenantAPIKeyService) CreateAPIKey(
	context.Context, interfaces.TenantAPIKeyCreateRequest,
) (*interfaces.TenantAPIKeyCreateResult, error) {
	return &interfaces.TenantAPIKeyCreateResult{APIKey: s.storedKey(), Token: disclosureTestToken}, nil
}

func (s stubTenantAPIKeyService) ListAPIKeys(context.Context, uint64) ([]*types.TenantAPIKey, error) {
	return []*types.TenantAPIKey{s.storedKey()}, nil
}

func (s stubTenantAPIKeyService) UpdateAPIKey(
	context.Context, interfaces.TenantAPIKeyUpdateRequest,
) (*types.TenantAPIKey, error) {
	return s.storedKey(), nil
}

func newTenantAPIKeyTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &TenantHandler{apiKeyService: stubTenantAPIKeyService{}}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.GET("/api/v1/tenants/:id/api-keys", h.ListAPIKeys)
	r.POST("/api/v1/tenants/:id/api-keys", h.CreateAPIKey)
	r.PUT("/api/v1/tenants/:id/api-keys/:key_id", h.UpdateAPIKey)
	return r
}

func serveTenantAPIKeyRequest(t *testing.T, method, path, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newTenantAPIKeyTestEngine().ServeHTTP(w, req)
	require.Less(t, w.Code, 300, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func TestTenantAPIKeyPlaintextOnlyAtCreation(t *testing.T) {
	created := serveTenantAPIKeyRequest(t, http.MethodPost, "/api/v1/tenants/7/api-keys",
		`{"name":"ci","full_access":true}`)["data"].(map[string]any)
	assert.Equal(t, disclosureTestToken, created["token"])
	assert.Equal(t, disclosureTestToken, created["api_key"])

	listed := serveTenantAPIKeyRequest(t, http.MethodGet, "/api/v1/tenants/7/api-keys", "")["data"].([]any)
	require.Len(t, listed, 1)
	assert.Equal(t, "sk-Ab12...wxyz", listed[0].(map[string]any)["api_key"])

	updated := serveTenantAPIKeyRequest(t, http.MethodPut, "/api/v1/tenants/7/api-keys/3",
		`{"name":"ci","full_access":true}`)["data"].(map[string]any)
	assert.Equal(t, "sk-Ab12...wxyz", updated["api_key"])
	assert.NotContains(t, updated, "token")
}
