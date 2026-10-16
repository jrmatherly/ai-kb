package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/datasource"
)

// retryTestServer builds a server that always answers the auth-token call and
// routes the given target path to h, so tests can drive DoRequest's retry loop.
func retryTestServer(t *testing.T, target string, h http.HandlerFunc) (*httptest.Server, *Config) {
	t.Helper()
	resetFeishuRequestGates(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, TokenResponse{
			ApiResponse:       ApiResponse{Code: 0},
			TenantAccessToken: "fake-token",
			Expire:            7200,
		})
	})
	mux.HandleFunc(target, h)
	ts := httptest.NewServer(mux)
	return ts, &Config{AppID: "a", AppSecret: "b", BaseURL: ts.URL}
}

func TestDoRequest_RetriesOn429ThenSucceeds(t *testing.T) {
	var attempts atomic.Int32
	ts, cfg := retryTestServer(t, "/target", func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		if attempts.Load() == 1 {
			// "0" is coerced to a short delay inside the client so the test stays fast.
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"code":99991400,"msg":"rate limited"}`)
			return
		}
		writeJSON(w, ApiResponse{Code: 0})
	})
	defer ts.Close()

	c := NewClient(cfg)
	var resp ApiResponse
	if err := c.DoRequest(context.Background(), http.MethodGet, "/target", nil, &resp); err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if attempts.Load() < 2 {
		t.Errorf("attempts = %d, want >= 2 (should retry after 429)", attempts.Load())
	}
}

func TestDoRequest_429ExhaustsRetries(t *testing.T) {
	var attempts atomic.Int32
	ts, cfg := retryTestServer(t, "/target", func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"code":99991400,"msg":"rate limited"}`)
	})
	defer ts.Close()

	c := NewClient(cfg)
	err := c.DoRequest(context.Background(), http.MethodGet, "/target", nil, nil)
	if err == nil {
		t.Fatal("expected error when 429s exceed the retry budget")
	}
	if attempts.Load() != 4 { // initial + 3 retries
		t.Errorf("attempts = %d, want 4 (1 + 3 retries)", attempts.Load())
	}
}

func TestDoRequest_5xxRetriesOnce(t *testing.T) {
	var attempts atomic.Int32
	ts, cfg := retryTestServer(t, "/target", func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"code":1,"msg":"internal error"}`)
	})
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := NewClient(cfg)
	if err := c.DoRequest(ctx, http.MethodGet, "/target", nil, nil); err == nil {
		t.Fatal("expected error after 5xx exhaustion")
	}
	if attempts.Load() != 2 { // initial + 1 retry
		t.Errorf("attempts = %d, want 2 (5xx retries exactly once)", attempts.Load())
	}
}

func TestDoRequest_4xxNotRetried(t *testing.T) {
	var attempts atomic.Int32
	ts, cfg := retryTestServer(t, "/target", func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"code":1,"msg":"bad request"}`)
	})
	defer ts.Close()

	c := NewClient(cfg)
	if err := c.DoRequest(context.Background(), http.MethodGet, "/target", nil, nil); err == nil {
		t.Fatal("expected error on 400")
	}
	if attempts.Load() != 1 {
		t.Errorf("attempts = %d, want 1 (non-429/5xx 4xx must not retry)", attempts.Load())
	}
}

func TestDoRequest_4xxReturnsAPIErrorWithBodyCode(t *testing.T) {
	body := `{"code":1061004,"msg":"forbidden"}`
	ts, cfg := retryTestServer(t, "/target", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, body)
	})
	defer ts.Close()

	err := NewClient(cfg).DoRequest(context.Background(), http.MethodGet, "/target", nil, nil)
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("error = %#v, want *APIError", err)
	}
	if apiErr.Status != http.StatusBadRequest || apiErr.Code != 1061004 {
		t.Errorf("status, code = %d, %d, want 400, 1061004", apiErr.Status, apiErr.Code)
	}
	if !errors.Is(err, datasource.ErrPermissionDenied) {
		t.Error("code 1061004 does not match datasource.ErrPermissionDenied")
	}
	if want := "feishu api error: status=400 body=" + body; err.Error() != want {
		t.Errorf("error text = %q, want %q", err.Error(), want)
	}
}

func TestGetTenantAccessToken_FailureIsInvalidCredentials(t *testing.T) {
	resetFeishuRequestGates(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, TokenResponse{ApiResponse: ApiResponse{Code: 10014, Msg: "app secret invalid"}})
	}))
	defer ts.Close()

	_, err := NewClient(&Config{AppID: "a", AppSecret: "b", BaseURL: ts.URL}).GetTenantAccessToken(context.Background())
	if !errors.Is(err, datasource.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want datasource.ErrInvalidCredentials", err)
	}
	if code, _, _ := feishuFailure(err); code != "feishu_auth_or_permission" {
		t.Errorf("feishuFailure code = %q, want feishu_auth_or_permission", code)
	}
}

func TestDownloadRawBytes_RetriesOn429ThenSucceeds(t *testing.T) {
	var attempts atomic.Int32
	ts, cfg := retryTestServer(t, "/dl", func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		if attempts.Load() == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("payload-bytes"))
	})
	defer ts.Close()

	c := NewClient(cfg)
	data, err := c.downloadRawBytes(context.Background(), "/dl")
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if string(data) != "payload-bytes" {
		t.Errorf("data = %q, want %q", string(data), "payload-bytes")
	}
	if attempts.Load() < 2 {
		t.Errorf("attempts = %d, want >= 2 (should retry download after 429)", attempts.Load())
	}
}

func TestDownloadRawBytes_4xxNotRetried(t *testing.T) {
	var attempts atomic.Int32
	ts, cfg := retryTestServer(t, "/dl", func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusForbidden)
	})
	defer ts.Close()

	c := NewClient(cfg)
	if _, err := c.downloadRawBytes(context.Background(), "/dl"); err == nil {
		t.Fatal("expected error on 403")
	}
	if attempts.Load() != 1 {
		t.Errorf("attempts = %d, want 1 (403 must not retry)", attempts.Load())
	}
}

func TestFeishuRateLimitWait(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	tests := []struct {
		reset string
		retry string
		want  time.Duration
	}{
		{"", "", 10 * time.Second},
		{"", "0", 100 * time.Millisecond},
		{"", "-1", 100 * time.Millisecond},
		{"3", "", 3 * time.Second},
		{"3", "5", 5 * time.Second},
		{"5", "3", 5 * time.Second},
		{"abc", "2", 2 * time.Second},
		{"", now.Add(4 * time.Second).UTC().Format(http.TimeFormat), 4 * time.Second},
		{"NaN", "Inf", 10 * time.Second},
		{"1e30", "abc", 10 * time.Second},
	}
	t.Setenv("FEISHU_RATE_LIMIT_RETRY_WAIT", "")
	for _, tt := range tests {
		headers := http.Header{}
		headers.Set("x-ogw-ratelimit-reset", tt.reset)
		headers.Set("Retry-After", tt.retry)
		if got := feishuRateLimitWait(headers, now); got != tt.want {
			t.Errorf("wait(reset=%q, retry=%q) = %v, want %v", tt.reset, tt.retry, got, tt.want)
		}
	}
}

func TestDoRequest_RejectsOversizedResponse(t *testing.T) {
	var hits atomic.Int32
	ts, cfg := retryTestServer(t, "/open-apis/docx/v1/documents", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write(bytes.Repeat([]byte("a"), 1<<20+1))
	})
	defer ts.Close()

	c := NewClient(cfg)
	c.jsonLimit = 1 << 20
	err := c.DoRequest(context.Background(), http.MethodGet, "/open-apis/docx/v1/documents", nil, &map[string]any{})
	if err == nil {
		t.Fatal("DoRequest accepted a response larger than the cap, want error")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Fatalf("error = %v, want an explicit over-limit error", err)
	}
	// The same document is just as large on every attempt; retrying only burns
	// the rate-limit budget and re-reads the full cap each time.
	if got := hits.Load(); got != 1 {
		t.Fatalf("server hit %d times, want 1 (oversized responses must not be retried)", got)
	}
}

// The token response is decoded under tokenMu on every refresh and Ping, so it
// must honour the same cap as every other API response.
func TestGetTenantAccessToken_RejectsOversizedResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, _ *http.Request) {
		// Valid JSON, so only the size cap can reject it.
		_, _ = io.WriteString(w, `{"code":0,"tenant_access_token":"fake-token","expire":7200,"pad":"`)
		_, _ = w.Write(bytes.Repeat([]byte("a"), 1<<20))
		_, _ = io.WriteString(w, `"}`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	c := NewClient(&Config{AppID: "a", AppSecret: "b", BaseURL: ts.URL})
	c.jsonLimit = 1 << 20
	_, err := c.GetTenantAccessToken(context.Background())
	if err == nil {
		t.Fatal("GetTenantAccessToken accepted a response larger than the cap, want error")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Fatalf("error = %v, want an explicit over-limit error", err)
	}
}

// countingBody records how much of a response body the client consumed.
type countingBody struct {
	io.ReadCloser
	n *atomic.Int64
}

func (b countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.n.Add(int64(n))
	return n, err
}

type countingTransport struct {
	base http.RoundTripper
	n    *atomic.Int64
}

func (t countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err == nil && strings.Contains(req.URL.Path, "/download") {
		resp.Body = countingBody{ReadCloser: resp.Body, n: t.n}
	}
	return resp, err
}

// Download error bodies are only a diagnostic preview: they must be read with a
// small bound and never copied whole into the returned error.
func TestDownloadRawBytes_BoundsErrorBodies(t *testing.T) {
	const bodySize = 1 << 20
	for _, status := range []int{
		http.StatusTooManyRequests, http.StatusBadRequest, http.StatusInternalServerError, http.StatusForbidden,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			handler := func(w http.ResponseWriter, _ *http.Request) {
				if status == http.StatusTooManyRequests {
					w.Header().Set("Retry-After", "0")
				}
				w.WriteHeader(status)
				_, _ = w.Write(bytes.Repeat([]byte("a"), bodySize))
			}
			ts, cfg := retryTestServer(t, "/open-apis/drive/v1/files/f/download", handler)
			defer ts.Close()

			c := NewClient(cfg)
			var read atomic.Int64
			hc := *c.httpClient
			base := hc.Transport
			if base == nil {
				base = http.DefaultTransport
			}
			hc.Transport = countingTransport{base: base, n: &read}
			c.httpClient = &hc

			_, err := c.DownloadDriveFile(context.Background(), "f")
			if err == nil {
				t.Fatalf("status %d: want error", status)
			}
			if len(err.Error()) > 1024 {
				t.Fatalf("error carries %d bytes of response body, want a short preview", len(err.Error()))
			}
			if got := read.Load(); got >= bodySize {
				t.Fatalf("read %d bytes of error bodies, want a bounded preview per response", got)
			}
		})
	}
}

func TestAPIErrorMatchesDatasourceSentinels(t *testing.T) {
	for _, tc := range []struct {
		err  *APIError
		want error
	}{
		{&APIError{Status: http.StatusForbidden}, datasource.ErrPermissionDenied},
		{&APIError{Code: 1061004}, datasource.ErrPermissionDenied},
		{&APIError{Status: http.StatusUnauthorized}, datasource.ErrInvalidCredentials},
		{&APIError{Code: 1061005}, datasource.ErrInvalidCredentials},
		{&APIError{Code: 99991672}, datasource.ErrPermissionDenied},
		{&APIError{Code: 1061003}, datasource.ErrResourceNotFound},
	} {
		if !errors.Is(fmt.Errorf("wrapped: %w", tc.err), tc.want) {
			t.Errorf("%v does not match %v", tc.err, tc.want)
		}
		partial := &PartialDriveFileListError{Failures: []DriveFileListFailure{{Err: tc.err}}}
		if !errors.Is(partial, tc.want) {
			t.Errorf("partial listing with %v does not match %v", tc.err, tc.want)
		}
	}
	unmapped := []*APIError{{Status: http.StatusBadRequest, Code: 1}, {Status: http.StatusNotFound}, {Code: 99991663}}
	for _, other := range unmapped {
		for _, sentinel := range []error{
			datasource.ErrPermissionDenied, datasource.ErrInvalidCredentials, datasource.ErrResourceNotFound,
		} {
			if errors.Is(other, sentinel) {
				t.Errorf("%v matches %v", other, sentinel)
			}
		}
	}
}

func TestGetTenantAccessToken_TransientFailureIsNotInvalidCredentials(t *testing.T) {
	for _, tc := range []struct {
		status int
		reason string
	}{
		{http.StatusTooManyRequests, "feishu_rate_limited"},
		{http.StatusBadGateway, "feishu_server_unavailable"},
	} {
		resetFeishuRequestGates(t)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			writeJSON(w, TokenResponse{ApiResponse: ApiResponse{Code: 99991400, Msg: "try later"}})
		}))
		c := NewClient(&Config{AppID: "a", AppSecret: "b", BaseURL: ts.URL})
		_, err := c.GetTenantAccessToken(context.Background())
		ts.Close()
		if err == nil || errors.Is(err, datasource.ErrInvalidCredentials) {
			t.Fatalf("status %d: error = %v, want a non-credentials failure", tc.status, err)
		}
		if reason, _, _ := feishuFailure(err); reason != tc.reason {
			t.Errorf("status %d: feishuFailure reason = %q, want %q", tc.status, reason, tc.reason)
		}
	}
}
