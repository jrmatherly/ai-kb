package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

// stubOIDCStartUserService implements GetOIDCAuthorizationURL and
// LoginWithOIDC; every other UserService call panics via the nil interface
// embedding. Keeps the tests focused on the two redirect handlers.
type stubOIDCStartUserService struct {
	interfaces.UserService
	getOIDCAuthorizationURL func(ctx context.Context, redirectURI string) (*types.OIDCAuthURLResponse, error)
	loginWithOIDC           func(ctx context.Context, code, redirectURI string) (*types.OIDCCallbackResponse, error)
	calls                   int
	lastReturnTo            string
}

func (s *stubOIDCStartUserService) GetOIDCAuthorizationURL(
	ctx context.Context, redirectURI, returnTo string,
) (*types.OIDCAuthURLResponse, error) {
	s.calls++
	s.lastReturnTo = returnTo
	if s.getOIDCAuthorizationURL == nil {
		return &types.OIDCAuthURLResponse{
			Success: true, AuthorizationURL: "http://idp/authorize", State: "s", Nonce: "nonce456",
		}, nil
	}
	return s.getOIDCAuthorizationURL(ctx, redirectURI)
}

func (s *stubOIDCStartUserService) LoginWithOIDC(
	ctx context.Context, code, redirectURI string, _ types.TenantProvisioningMode,
) (*types.OIDCCallbackResponse, error) {
	return s.loginWithOIDC(ctx, code, redirectURI)
}

func newOIDCStartTestRouter(h *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorCapture())
	r.GET("/auth/oidc/start", h.OIDCStart)
	r.GET("/auth/oidc/callback", h.OIDCRedirectCallback)
	return r
}

func hasNonceCookie(w *httptest.ResponseRecorder) bool {
	for _, ck := range w.Result().Cookies() {
		if ck.Name == oidcNonceCookieName && ck.Value != "" {
			return true
		}
	}
	return false
}

// TestOIDCStart_RedirectsToAuthProvider: on success the handler must 302
// to the IdP authorization URL and bind the nonce via cookie (CSRF/replay
// defence), exactly like /auth/oidc/url.
func TestOIDCStart_RedirectsToAuthProvider(t *testing.T) {
	const authURL = "http://idp.example.com/authorize?client_id=aikb"
	us := &stubOIDCStartUserService{
		getOIDCAuthorizationURL: func(context.Context, string) (*types.OIDCAuthURLResponse, error) {
			return &types.OIDCAuthURLResponse{
				Success:          true,
				AuthorizationURL: authURL,
				State:            "state123",
				Nonce:            "nonce456",
			}, nil
		},
	}
	h := NewAuthHandler(&config.Config{}, us, nil, nil, nil, nil)
	r := newOIDCStartTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/auth/oidc/start", nil)
	req.Host = "ai-kb.example.com"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != authURL {
		t.Errorf("Location = %q, want %q", loc, authURL)
	}
	if !hasNonceCookie(w) {
		t.Errorf("expected %s cookie to be set with the nonce", oidcNonceCookieName)
	}
	if us.lastReturnTo != "" {
		t.Errorf("returnTo = %q, want empty without a return_to parameter", us.lastReturnTo)
	}
}

// TestOIDCStart_BuildsCallbackURLFromRequestOrigin: redirect_uri handed to
// the IdP is derived from the request's own origin (no caller-supplied
// value needed); X-Forwarded-Proto upgrades the scheme to https.
func TestOIDCStart_BuildsCallbackURLFromRequestOrigin(t *testing.T) {
	var captured string
	us := &stubOIDCStartUserService{
		getOIDCAuthorizationURL: func(_ context.Context, redirectURI string) (*types.OIDCAuthURLResponse, error) {
			captured = redirectURI
			return &types.OIDCAuthURLResponse{Success: true, AuthorizationURL: "http://idp", Nonce: "n"}, nil
		},
	}
	h := NewAuthHandler(&config.Config{}, us, nil, nil, nil, nil)
	r := newOIDCStartTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/auth/oidc/start", nil)
	req.Host = "ai-kb.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	const want = "https://ai-kb.example.com/api/v1/auth/oidc/callback"
	if captured != want {
		t.Errorf("callback URL passed to IdP = %q, want %q", captured, want)
	}
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
}

// TestOIDCStart_ServiceErrorReturnsNonRedirect: if the IdP URL cannot be
// built, never 302 - surface the error the same way /auth/oidc/url does.
func TestOIDCStart_ServiceErrorReturnsNonRedirect(t *testing.T) {
	us := &stubOIDCStartUserService{
		getOIDCAuthorizationURL: func(context.Context, string) (*types.OIDCAuthURLResponse, error) {
			return nil, fmt.Errorf("idp unavailable")
		},
	}
	h := NewAuthHandler(&config.Config{}, us, nil, nil, nil, nil)
	r := newOIDCStartTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/auth/oidc/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusFound {
		t.Fatalf("status = 302, want non-redirect on service error")
	}
}
