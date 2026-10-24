package msgraph

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

func allowLocalServer(t *testing.T) {
	t.Helper()
	t.Setenv("SSRF_WHITELIST", "127.0.0.1,::1,localhost")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
}

// fakeGraph serves the token endpoint, the Graph paths and the download host
// from one server. Handlers are keyed by "METHOD path" without the query string.
type fakeGraph struct {
	t        *testing.T
	server   *httptest.Server
	mu       sync.Mutex
	handlers map[string]http.HandlerFunc
	tokens   atomic.Int32
	requests []string
	// token is what the token endpoint hands out; tests change it to check re-minting.
	token string
	// tokenStatus lets a test fail the token endpoint.
	tokenStatus int
	// expiresIn is the token lifetime the endpoint reports; tests lower it to check caching.
	expiresIn int64
}

func newFakeGraph(t *testing.T) *fakeGraph {
	t.Helper()
	allowLocalServer(t)
	f := &fakeGraph{
		t: t, handlers: map[string]http.HandlerFunc{}, token: "tok-1", tokenStatus: http.StatusOK, expiresIn: 3599,
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGraph) handle(key string, h http.HandlerFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers[key] = h
}

func (f *fakeGraph) json(key string, status int, body any) {
	f.handle(key, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	})
}

func (f *fakeGraph) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	// Recorded decoded, so a drive id such as b!abc matches the handler key and f.seen alike.
	f.requests = append(f.requests, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
	h, ok := f.handlers[r.Method+" "+r.URL.Path]
	f.mu.Unlock()
	if strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/token") {
		f.tokens.Add(1)
		if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "client_credentials" ||
			r.Form.Get("scope") != "https://graph.microsoft.com/.default" || r.Form.Get("client_secret") == "" {
			http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
			return
		}
		if f.tokenStatus != http.StatusOK {
			w.WriteHeader(f.tokenStatus)
			_, _ = w.Write(
				[]byte(`{"error":"invalid_client","error_description":"AADSTS7000215: Invalid client secret"}`),
			)
			return
		}
		_ = json.NewEncoder(w).
			Encode(map[string]any{"token_type": "Bearer", "expires_in": f.expiresIn, "access_token": f.token})
		return
	}
	if !ok {
		http.Error(w, `{"error":{"code":"itemNotFound"}}`, http.StatusNotFound)
		return
	}
	h(w, r)
}

// requireBearer fails the request unless it carries the current token.
func (f *fakeGraph) requireBearer(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.token {
			http.Error(w, `{"error":{"code":"InvalidAuthenticationToken"}}`, http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

func (f *fakeGraph) seen(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

func (f *fakeGraph) client(t *testing.T) *client {
	t.Helper()
	cfg := &config{TenantID: "contoso.onmicrosoft.com", ClientID: "app", ClientSecret: "s3cret"}
	return newClient(f.server.URL, f.server.URL, cfg)
}
