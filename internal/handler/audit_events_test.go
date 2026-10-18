package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// recordingAudit keeps every entry the handler under test emits.
type recordingAudit struct {
	interfaces.AuditLogService
	entries []*types.AuditLog
}

func (r *recordingAudit) Log(_ context.Context, entry *types.AuditLog) error {
	r.entries = append(r.entries, entry)
	return nil
}

func (r *recordingAudit) only(t *testing.T) *types.AuditLog {
	t.Helper()
	require.Len(t, r.entries, 1, "expected exactly one audit row")
	return r.entries[0]
}

func auditTestRouter(handle gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Any("/*path", func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u-actor")
		c.Request = c.Request.WithContext(ctx)
		handle(c)
	})
	return r
}

func serve(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

type loginUserService struct {
	interfaces.UserService
	resp *types.LoginResponse
}

func (s loginUserService) Login(context.Context, *types.LoginRequest) (*types.LoginResponse, error) {
	return s.resp, nil
}

func TestLoginEmitsSucceededAndFailedAuditRows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	audit := &recordingAudit{}
	ok := &types.LoginResponse{
		Success: true, User: &types.User{ID: "u1"}, ActiveTenant: &types.Tenant{ID: 7},
		Memberships: []types.Membership{
			{TenantID: 3, Role: types.TenantRoleViewer},
			{TenantID: 7, Role: types.TenantRoleOwner},
		},
	}
	h := NewAuthHandler(&config.Config{Auth: &config.AuthConfig{}}, loginUserService{resp: ok}, nil, nil, nil, audit)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.POST("/auth/login", h.Login)
	rec := serve(r, http.MethodPost, "/auth/login", `{"email":"a@example.com","password":"password123"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	row := audit.only(t)
	require.Equal(t, types.AuditActionAuthLoginSucceeded, row.Action)
	require.Equal(t, "u1", row.ActorUserID)
	require.Equal(t, "u1", row.TargetUserID)
	require.Equal(t, uint64(7), row.TenantID)
	require.Equal(t, string(types.TenantRoleOwner), row.ActorRole)
	require.Equal(t, types.AuditOutcomeSuccess, row.Outcome)

	audit.entries = nil
	failed := &types.LoginResponse{Success: false, Message: "bad password"}
	h = NewAuthHandler(&config.Config{Auth: &config.AuthConfig{}}, loginUserService{resp: failed}, nil, nil, nil, audit)
	r = gin.New()
	r.Use(middleware.ErrorHandler())
	r.POST("/auth/login", h.Login)
	rec = serve(r, http.MethodPost, "/auth/login", `{"email":"a@example.com","password":"password123"}`)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	row = audit.only(t)
	require.Equal(t, types.AuditActionAuthLoginFailed, row.Action)
	require.Equal(t, types.AuditOutcomeDenied, row.Outcome)
	require.Equal(t, "", row.ActorUserID)
	require.NotContains(t, string(row.Details), "a@example.com")
}

type revokeAPIKeyService struct {
	interfaces.TenantAPIKeyService
}

func (revokeAPIKeyService) RevokeAPIKey(context.Context, uint64, uint64) error { return nil }

func TestDeleteAPIKeyEmitsAuditRow(t *testing.T) {
	audit := &recordingAudit{}
	h := NewTenantHandler(nil, revokeAPIKeyService{}, nil, nil, nil, &config.Config{}, nil, audit)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.DELETE("/tenants/:id/api-keys/:key_id", func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.UserIDContextKey, "u-actor"))
		h.DeleteAPIKey(c)
	})
	rec := serve(r, http.MethodDelete, "/tenants/7/api-keys/42", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	row := audit.only(t)
	require.Equal(t, types.AuditActionTenantAPIKeyDeleted, row.Action)
	require.Equal(t, uint64(7), row.TenantID)
	require.Equal(t, "u-actor", row.ActorUserID)
	require.Equal(t, "tenant_api_key", row.TargetType)
	require.Equal(t, "42", row.TargetID)
}

type deleteModelService struct{ interfaces.ModelService }

func (deleteModelService) DeleteModel(context.Context, string) error { return nil }

func TestDeleteModelEmitsChangedRow(t *testing.T) {
	audit := &recordingAudit{}
	h := NewModelHandler(deleteModelService{}, audit)
	rec := serve(auditTestRouter(func(c *gin.Context) {
		c.Params = gin.Params{{Key: "id", Value: "m-1"}}
		h.DeleteModel(c)
	}), http.MethodDelete, "/models/m-1", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	row := audit.only(t)
	require.Equal(t, types.AuditActionModelChanged, row.Action)
	require.Equal(t, uint64(7), row.TenantID)
	require.Equal(t, "u-actor", row.ActorUserID)
	require.Equal(t, "model", row.TargetType)
	require.Equal(t, "m-1", row.TargetID)
	require.Contains(t, string(row.Details), `"op":"deleted"`)
}

type deleteMCPService struct{ interfaces.MCPServiceService }

func (deleteMCPService) DeleteMCPService(context.Context, uint64, string) error { return nil }

func TestDeleteMCPServiceEmitsChangedRow(t *testing.T) {
	audit := &recordingAudit{}
	h := NewMCPServiceHandler(deleteMCPService{}, nil, nil, nil, nil, audit)
	rec := serve(auditTestRouter(func(c *gin.Context) {
		c.Params = gin.Params{{Key: "id", Value: "mcp-1"}}
		h.DeleteMCPService(c)
	}), http.MethodDelete, "/mcp-services/mcp-1", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	row := audit.only(t)
	require.Equal(t, types.AuditActionMCPServiceChanged, row.Action)
	require.Equal(t, "mcp_service", row.TargetType)
	require.Equal(t, "mcp-1", row.TargetID)
	require.Contains(t, string(row.Details), `"op":"deleted"`)
}

type auditWebSearchRepo struct {
	interfaces.WebSearchProviderRepository
}

func (auditWebSearchRepo) GetByID(
	_ context.Context, tenantID uint64, id string,
) (*types.WebSearchProviderEntity, error) {
	return &types.WebSearchProviderEntity{ID: id, TenantID: tenantID}, nil
}

type webSearchServiceStub struct {
	interfaces.WebSearchProviderService
}

func (webSearchServiceStub) DeleteProvider(context.Context, uint64, string) error { return nil }

func TestDeleteWebSearchProviderEmitsChangedRow(t *testing.T) {
	audit := &recordingAudit{}
	h := NewWebSearchProviderHandler(auditWebSearchRepo{}, webSearchServiceStub{}, nil, audit)
	rec := serve(auditTestRouter(func(c *gin.Context) {
		c.Params = gin.Params{{Key: "id", Value: "ws-1"}}
		h.DeleteProvider(c)
	}), http.MethodDelete, "/web-search/providers/ws-1", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	row := audit.only(t)
	require.Equal(t, types.AuditActionWebSearchProviderChanged, row.Action)
	require.Equal(t, "web_search_provider", row.TargetType)
	require.Equal(t, "ws-1", row.TargetID)
}

type storageServiceStub struct {
	interfaces.StorageBackendService
}

func (storageServiceStub) Delete(context.Context, uint64, string) error { return nil }

func TestDeleteStorageBackendEmitsChangedRow(t *testing.T) {
	audit := &recordingAudit{}
	h := NewStorageBackendHandler(nil, storageServiceStub{}, audit)
	rec := serve(auditTestRouter(func(c *gin.Context) {
		c.Params = gin.Params{{Key: "id", Value: "sb-1"}}
		h.Delete(c)
	}), http.MethodDelete, "/storage-backends/sb-1", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	row := audit.only(t)
	require.Equal(t, types.AuditActionStorageBackendChanged, row.Action)
	require.Equal(t, "storage_backend", row.TargetType)
	require.Equal(t, "sb-1", row.TargetID)
}

type createAgentService struct{ interfaces.CustomAgentService }

func (createAgentService) CreateAgent(_ context.Context, agent *types.CustomAgent) (*types.CustomAgent, error) {
	agent.ID = "agent-1"
	return agent, nil
}

func TestCreateAgentEmitsChangedRow(t *testing.T) {
	audit := &recordingAudit{}
	h := NewCustomAgentHandler(createAgentService{}, nil, nil, nil, nil, service.HostSandboxManager{}, audit)
	rec := serve(auditTestRouter(h.CreateAgent), http.MethodPost, "/agents", `{"name":"Helper"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	row := audit.only(t)
	require.Equal(t, types.AuditActionAgentChanged, row.Action)
	require.Equal(t, "agent", row.TargetType)
	require.Equal(t, "agent-1", row.TargetID)
	require.Contains(t, string(row.Details), `"op":"created"`)
}

func TestTenantAPIKeyDetailsRecordsFullAccess(t *testing.T) {
	expires := int64(1760000000)
	got := string(tenantAPIKeyDetails("created", tenantAPIKeyUpdateRequest{FullAccess: true, ExpiresAt: &expires}))
	require.JSONEq(t,
		`{"op":"created","fields":["name","capabilities","knowledge_base_ids","expires_at"],"full_access":true}`, got)
	got = string(tenantAPIKeyDetails("updated", tenantAPIKeyUpdateRequest{}))
	require.JSONEq(t, `{"op":"updated","fields":["name","capabilities","knowledge_base_ids"],"full_access":false}`, got)
}
