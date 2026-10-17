package handler

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/jrmatherly/ai-kb/internal/application/service"
	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

// errScrubUserService fails every call with err, the way the user service
// fails for an authored reason (an AppError) or a repository error.
type errScrubUserService struct {
	interfaces.UserService
	err error
}

func (s errScrubUserService) GetCurrentUser(context.Context) (*types.User, error) {
	return &types.User{ID: "u-1"}, nil
}

func (s errScrubUserService) Login(context.Context, *types.LoginRequest) (*types.LoginResponse, error) {
	return nil, s.err
}

func (s errScrubUserService) GetOIDCAuthorizationURL(
	context.Context, string, string,
) (*types.OIDCAuthURLResponse, error) {
	return nil, s.err
}

func (s errScrubUserService) RefreshToken(context.Context, string) (string, string, error) {
	return "", "", s.err
}

func (s errScrubUserService) UpdateUserPreferences(
	context.Context, string, types.UserPreferences,
) (types.UserPreferences, error) {
	return types.UserPreferences{}, s.err
}

func (s errScrubUserService) ChangePassword(context.Context, string, string, string) error {
	return s.err
}

func (s errScrubUserService) SwitchTenant(context.Context, *types.User, uint64, string) (*types.LoginResponse, error) {
	return nil, s.err
}

func (s errScrubUserService) ValidateToken(context.Context, string) (*types.User, uint64, error) {
	return nil, 0, s.err
}

func TestAuthErrorsForwardOnlyAuthoredCauses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repoErr := fmt.Errorf("look up token: %w", stderrors.New(`pq: relation "auth_tokens" does not exist`))
	internal := `{"success":false,"error":{"code":1007,"message":"Internal server error"}}`
	for _, tc := range []struct {
		name, method, path, body string
		err                      error
		status                   int
		want                     string
	}{
		{
			"login", http.MethodPost, "/auth/login", `{"email":"a@b.co","password":"secret1"}`, repoErr, 401,
			`{"success":false,"error":{"code":1001,"message":"Login failed"}}`,
		},
		{
			"oidc authored", http.MethodGet, "/auth/oidc/url?redirect_uri=https://x/cb", "",
			apperrors.NewForbiddenError("OIDC login is disabled"), 403,
			`{"success":false,"error":{"code":1002,"message":"OIDC authorization unavailable",` +
				`"details":"OIDC login is disabled"}}`,
		},
		{
			"oidc discovery failure", http.MethodGet, "/auth/oidc/url?redirect_uri=https://x/cb", "",
			stderrors.New("OIDC discovery request failed: status=500 body=internal idp detail"), 403,
			`{"success":false,"error":{"code":1002,"message":"OIDC authorization unavailable"}}`,
		},
		{
			"refresh authored", http.MethodPost, "/auth/refresh", `{"refreshToken":"t"}`,
			apperrors.NewUnauthorizedError("refresh token is revoked"), 401,
			`{"success":false,"error":{"code":1001,"message":"Token refresh failed",` +
				`"details":"refresh token is revoked"}}`,
		},
		{"refresh repository", http.MethodPost, "/auth/refresh", `{"refreshToken":"t"}`, repoErr, 500, internal},
		{
			"preferences authored", http.MethodPut, "/auth/me/preferences", `{}`,
			apperrors.NewBadRequestError("browser search instructions must not exceed 2000 characters"), 400,
			`{"success":false,"error":{"code":1000,"message":"Failed to update preferences",` +
				`"details":"browser search instructions must not exceed 2000 characters"}}`,
		},
		{"preferences repository", http.MethodPut, "/auth/me/preferences", `{}`, repoErr, 500, internal},
		{
			"password repository", http.MethodPost, "/auth/change-password",
			`{"old_password":"a","new_password":"b"}`, repoErr, 500, internal,
		},
		{
			"switch without membership", http.MethodPost, "/auth/switch-tenant", `{"tenant_id":9}`,
			fmt.Errorf("lookup membership: %w", service.ErrMembershipNotFound), 403,
			`{"success":false,"error":{"code":1002,"message":"workspace switch failed",` +
				`"details":"tenant membership not found"}}`,
		},
		{"switch repository", http.MethodPost, "/auth/switch-tenant", `{"tenant_id":9}`, repoErr, 500, internal},
		{
			"validate authored", http.MethodGet, "/auth/validate", "",
			apperrors.NewUnauthorizedError("token is revoked"), 401,
			`{"success":false,"error":{"code":1001,"message":"Token validation failed",` +
				`"details":"token is revoked"}}`,
		},
		{"validate repository", http.MethodGet, "/auth/validate", "", repoErr, 500, internal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &AuthHandler{userService: errScrubUserService{err: tc.err}}
			r := gin.New()
			r.Use(middleware.ErrorHandler())
			r.POST("/auth/login", h.Login)
			r.GET("/auth/oidc/url", h.GetOIDCAuthorizationURL)
			r.POST("/auth/refresh", h.RefreshToken)
			r.PUT("/auth/me/preferences", h.UpdateMyPreferences)
			r.POST("/auth/change-password", h.ChangePassword)
			r.POST("/auth/switch-tenant", h.SwitchTenant)
			r.GET("/auth/validate", h.ValidateToken)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer t")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.JSONEq(t, tc.want, w.Body.String())
		})
	}
}

func TestUserServiceTokenRejectionsAreAuthored401s(t *testing.T) {
	svc := service.NewUserService(nil, nil, nil, nil, nil, nil)
	_, _, err := svc.ValidateToken(context.Background(), "not-a-jwt")
	appErr, ok := apperrors.IsAppError(err)
	require.True(t, ok, "%v", err)
	require.Equal(t, http.StatusUnauthorized, appErr.HTTPCode)
	require.Equal(t, "invalid token", appErr.Message)

	_, _, err = svc.RefreshToken(context.Background(), "not-a-jwt")
	appErr, ok = apperrors.IsAppError(err)
	require.True(t, ok, "%v", err)
	require.Equal(t, "invalid refresh token", appErr.Message)
}
