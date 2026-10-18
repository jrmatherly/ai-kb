package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

func TestOIDCCallbackEmitsLoginAuditRows(t *testing.T) {
	listed := oidcReturnToConfig(listedReturnTo)
	state := signedStateAt(t, listedReturnTo, 0)
	cases := []struct {
		name    string
		login   func(context.Context, string, string) (*types.OIDCCallbackResponse, error)
		action  types.AuditAction
		outcome types.AuditOutcome
		actor   string
	}{
		{"success", func(context.Context, string, string) (*types.OIDCCallbackResponse, error) {
			return &types.OIDCCallbackResponse{
				Success: true, Token: "a1", RefreshToken: "r1", Memberships: []types.Membership{},
				User: &types.User{ID: "u-oidc"}, Tenant: &types.Tenant{ID: 7},
			}, nil
		}, types.AuditActionAuthLoginSucceeded, types.AuditOutcomeSuccess, "u-oidc"},
		{"failed", failedLogin, types.AuditActionAuthLoginFailed, types.AuditOutcomeDenied, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			audit := &recordingAudit{}
			us := &stubOIDCStartUserService{loginWithOIDC: tc.login}
			r := newOIDCStartTestRouter(NewAuthHandler(listed, us, nil, nil, nil, audit))
			req := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=c1&state="+state, nil)
			req.AddCookie(&http.Cookie{Name: oidcNonceCookieName, Value: "nonce456"})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusFound, w.Code)
			row := audit.only(t)
			require.Equal(t, tc.action, row.Action)
			require.Equal(t, tc.outcome, row.Outcome)
			require.Equal(t, tc.actor, row.ActorUserID)
			require.Contains(t, string(row.Details), `"oidc"`)
			if tc.actor != "" {
				require.Equal(t, uint64(7), row.TenantID)
			}
		})
	}
}

type erroringLoginUserService struct{ interfaces.UserService }

func (erroringLoginUserService) Login(context.Context, *types.LoginRequest) (*types.LoginResponse, error) {
	return nil, context.DeadlineExceeded
}

func TestLoginServiceErrorIsNotLabelledAsBadCredentials(t *testing.T) {
	audit := &recordingAudit{}
	h := NewAuthHandler(oidcReturnToConfig(), erroringLoginUserService{}, nil, nil, nil, audit)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.POST("/auth/login", h.Login)
	rec := serve(r, http.MethodPost, "/auth/login", `{"email":"a@example.com","password":"password123"}`)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	row := audit.only(t)
	require.Equal(t, types.AuditActionAuthLoginFailed, row.Action)
	require.Contains(t, string(row.Details), `"service_error"`)
	require.NotContains(t, string(row.Details), "invalid_credentials")
}

type modelCredentialsService struct{ interfaces.ModelService }

func (modelCredentialsService) ClearModelCredential(context.Context, string, string) error {
	return nil
}

func TestClearModelCredentialEmitsChangedRow(t *testing.T) {
	audit := &recordingAudit{}
	h := NewModelCredentialsHandler(modelCredentialsService{}, audit)
	rec := serve(auditTestRouter(func(c *gin.Context) {
		c.Params = gin.Params{{Key: "id", Value: "m-1"}, {Key: "field", Value: "api_key"}}
		h.DeleteField(c)
	}), http.MethodDelete, "/models/m-1/credentials/api_key", "")
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	row := audit.only(t)
	require.Equal(t, types.AuditActionModelChanged, row.Action)
	require.Equal(t, "m-1", row.TargetID)
	require.Contains(t, string(row.Details), `"credentials_cleared"`)
	require.Contains(t, string(row.Details), `"api_key"`)
}

type mcpCredentialsService struct{ interfaces.MCPServiceService }

func (mcpCredentialsService) ClearMCPCredential(context.Context, uint64, string, string) error {
	return nil
}

func TestClearMCPCredentialEmitsChangedRow(t *testing.T) {
	audit := &recordingAudit{}
	h := NewMCPCredentialsHandler(mcpCredentialsService{}, audit)
	rec := serve(auditTestRouter(func(c *gin.Context) {
		c.Params = gin.Params{{Key: "id", Value: "mcp-1"}, {Key: "field", Value: "token"}}
		h.DeleteField(c)
	}), http.MethodDelete, "/mcp-services/mcp-1/credentials/token", "")
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	row := audit.only(t)
	require.Equal(t, types.AuditActionMCPServiceChanged, row.Action)
	require.Equal(t, "mcp-1", row.TargetID)
	require.Contains(t, string(row.Details), `"credentials_cleared"`)
}

func TestCopyModelEmitsCopiedRow(t *testing.T) {
	audit := &recordingAudit{}
	stub := &stubCopyModelService{model: &types.Model{ID: "copy-1", Name: "gpt", Type: types.ModelTypeKnowledgeQA}}
	h := NewModelHandler(stub, audit)
	rec := serve(auditTestRouter(func(c *gin.Context) {
		c.Params = gin.Params{{Key: "id", Value: "src"}}
		h.CopyModel(c)
	}), http.MethodPost, "/models/src/copy", `{"display_name":"copy"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	row := audit.only(t)
	require.Equal(t, types.AuditActionModelChanged, row.Action)
	require.Equal(t, "copy-1", row.TargetID)
	require.Contains(t, string(row.Details), `"copied"`)
}
