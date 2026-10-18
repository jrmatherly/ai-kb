package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/types"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

const listedReturnTo = "https://abc.chromiumapp.org/oidc"

func oidcReturnToConfig(urls ...string) *config.Config {
	return &config.Config{OIDCAuth: &config.OIDCAuthConfig{Enable: true, ExtensionReturnURLs: urls}}
}

func startWithReturnTo(
	t *testing.T, cfg *config.Config, returnTo string,
) (*httptest.ResponseRecorder, *stubOIDCStartUserService) {
	t.Helper()
	us := &stubOIDCStartUserService{}
	r := newOIDCStartTestRouter(NewAuthHandler(cfg, us, nil, nil, nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/auth/oidc/start?return_to="+urlQueryEscape(returnTo), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, us
}

func TestOIDCStart_RejectsUnlistedReturnTo(t *testing.T) {
	cases := []struct {
		name     string
		listed   []string
		returnTo string
	}{
		{"unlisted", []string{listedReturnTo}, "https://nope.example/x"},
		{"trailing slash", []string{listedReturnTo}, listedReturnTo + "/"},
		{"query", []string{listedReturnTo}, listedReturnTo + "?x=1"},
		{"http variant", []string{listedReturnTo}, strings.Replace(listedReturnTo, "https:", "http:", 1)},
		{"empty list", nil, listedReturnTo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, us := startWithReturnTo(t, oidcReturnToConfig(tc.listed...), tc.returnTo)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", w.Code, w.Body.String())
			}
			if loc := w.Header().Get("Location"); loc != "" {
				t.Errorf("Location = %q, want none", loc)
			}
			if hasNonceCookie(w) {
				t.Errorf("nonce cookie set on a rejected start")
			}
			if us.calls != 0 {
				t.Errorf("GetOIDCAuthorizationURL called %d times, want 0", us.calls)
			}
		})
	}
}

func TestOIDCStart_AcceptsListedReturnToAndSignsIt(t *testing.T) {
	w, us := startWithReturnTo(t, oidcReturnToConfig(listedReturnTo), listedReturnTo)
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302 (body %s)", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "http://idp/authorize" {
		t.Errorf("Location = %q", loc)
	}
	if us.lastReturnTo != listedReturnTo {
		t.Errorf("returnTo passed to the service = %q, want %q", us.lastReturnTo, listedReturnTo)
	}
	if !hasNonceCookie(w) {
		t.Errorf("nonce cookie missing")
	}
}

func TestOIDCStart_WithOIDCDisabledLeavesTheServiceToAnswer(t *testing.T) {
	w, _ := startWithReturnTo(t, &config.Config{OIDCAuth: &config.OIDCAuthConfig{Enable: false}}, listedReturnTo)
	if w.Code == http.StatusBadRequest {
		t.Fatalf("status = 400; with OIDC disabled the allow-list must not be the reported cause")
	}
}

func signedStateAt(t *testing.T, returnTo string, issuedAt int64) string {
	t.Helper()
	state, err := secutils.SignOIDCState(&secutils.OIDCStatePayload{
		Nonce:       "nonce456",
		RedirectURI: "http://ai-kb.example.com/api/v1/auth/oidc/callback",
		ReturnTo:    returnTo,
		IssuedAt:    issuedAt,
	})
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}
	return state
}

var okLogin = func(context.Context, string, string) (*types.OIDCCallbackResponse, error) {
	return &types.OIDCCallbackResponse{
		Success: true, Token: "a1", RefreshToken: "r1", Memberships: []types.Membership{},
	}, nil
}

var failedLogin = func(context.Context, string, string) (*types.OIDCCallbackResponse, error) {
	return &types.OIDCCallbackResponse{Success: false, Message: "nope"}, nil
}

func TestOIDCRedirectCallback_RedirectsToReturnTo(t *testing.T) {
	listed := oidcReturnToConfig(listedReturnTo)
	state := signedStateAt(t, listedReturnTo, 0)
	tampered := state[:len(state)-2] + "zz"
	expired := signedStateAt(t, listedReturnTo, time.Now().Add(-11*time.Minute).Unix())
	cases := []struct {
		name       string
		cfg        *config.Config
		query      string
		cookie     bool
		login      func(context.Context, string, string) (*types.OIDCCallbackResponse, error)
		wantPrefix string
	}{
		{"success", listed, "code=c1&state=" + state, true, okLogin, listedReturnTo + "#oidc_result="},
		{"idp error without state", listed, "error=access_denied", false, okLogin, "/#oidc_error=access_denied"},
		{
			"idp error with state", listed, "error=access_denied&state=" + state, false, okLogin,
			listedReturnTo + "#oidc_error=access_denied",
		},
		{"missing code", listed, "state=" + state, true, okLogin, listedReturnTo + "#oidc_error=missing_code"},
		{
			"tampered state still lands on the return URL", listed, "code=c1&state=" + tampered, true, okLogin,
			listedReturnTo + "#oidc_error=invalid_state",
		},
		{
			"expired state still lands on the return URL", listed, "code=c1&state=" + expired, true, okLogin,
			listedReturnTo + "#oidc_error=invalid_state",
		},
		{
			"login failed", listed, "code=c1&state=" + state, true, failedLogin,
			listedReturnTo + "#oidc_error=login_failed",
		},
		{
			"no return_to in the state", listed, "code=c1&state=" + signedStateAt(t, "", 0), true, okLogin,
			"/#oidc_result=",
		},
		{"return_to no longer listed", oidcReturnToConfig(), "code=c1&state=" + state, true, okLogin, "/#oidc_result="},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			us := &stubOIDCStartUserService{loginWithOIDC: tc.login}
			r := newOIDCStartTestRouter(NewAuthHandler(tc.cfg, us, nil, nil, nil, nil))
			req := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?"+tc.query, nil)
			if tc.cookie {
				req.AddCookie(&http.Cookie{Name: oidcNonceCookieName, Value: "nonce456"})
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302 (body %s)", w.Code, w.Body.String())
			}
			if loc := w.Header().Get("Location"); !strings.HasPrefix(loc, tc.wantPrefix) {
				t.Errorf("Location = %q, want prefix %q", loc, tc.wantPrefix)
			}
		})
	}
}
