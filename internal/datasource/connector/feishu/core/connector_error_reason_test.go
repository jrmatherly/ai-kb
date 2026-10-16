package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

// feishuFailure classifies a raw connector/API error into a stable i18n code
// (+ optional numeric feishu code) and an English fallback message. The frontend
// localises by code; the raw status/body/log_id stays in the server logs and
// must never appear in the code, codeValue or fallback.
func TestFeishuFailure(t *testing.T) {
	cases := []struct {
		name          string
		err           error
		wantCode      string
		wantCodeValue string
		fallbackHas   []string
		noLeak        []string // must appear in neither code nor fallback
	}{
		{
			name:        "rate limited",
			err:         newAPIError("feishu rate limited", http.StatusTooManyRequests, []byte(`{"code":99991400}`)),
			wantCode:    "feishu_rate_limited",
			fallbackHas: []string{"retry", "next sync"},
			noLeak:      []string{"body=", "{", "99991400", "automatically"},
		},
		{
			name:     "server 5xx",
			err:      newAPIError("download server error", http.StatusInternalServerError, []byte(`{"code":1663}`)),
			wantCode: "feishu_server_unavailable",
			noLeak:   []string{"body=", "{"},
		},
		{
			name:     "rate limit code over HTTP 400",
			err:      newAPIError("feishu rate limited", http.StatusBadRequest, []byte(`{"code":99991400}`)),
			wantCode: "feishu_rate_limited",
		},
		{
			name:     "export polls exhausted",
			err:      fmt.Errorf("export report (docx): %w after 60 polls (ticket=abc)", errExportPollsExhausted),
			wantCode: "feishu_timeout",
			noLeak:   []string{"ticket=", "abc"},
		},
		{
			name:     "request deadline",
			err:      fmt.Errorf("execute request: %w", context.DeadlineExceeded),
			wantCode: "feishu_timeout",
		},
		{
			name:     "network timeout",
			err:      fmt.Errorf("download request: %w", os.ErrDeadlineExceeded),
			wantCode: "feishu_timeout",
		},
		{
			name: "api error carries the feishu code as a param",
			err: fmt.Errorf("list children of n1: %w", &APIError{
				Op: "list wiki nodes error", Code: 1663, Msg: "internal error log_id=20260",
			}),
			wantCode:      "feishu_api_error",
			wantCodeValue: "1663",
			noLeak:        []string{"log_id", "internal error"},
		},
		{
			name:     "api error without a code is generic",
			err:      &APIError{Op: "download failed", Status: http.StatusNotFound, Body: "not found"},
			wantCode: "feishu_api_error_generic",
		},
		{
			name:     "failed export task is generic",
			err:      fmt.Errorf("export report (docx): %w", &exportJobError{Status: 3, Msg: "internal"}),
			wantCode: "feishu_api_error_generic",
		},
		{
			name:     "export job without permission",
			err:      &exportJobError{Status: 110, Msg: "no permission"},
			wantCode: "feishu_auth_or_permission",
		},
		{
			name:     "export job timed out",
			err:      &exportJobError{Status: 108},
			wantCode: "feishu_timeout",
		},
		{
			name:     "invalid tenant token is actionable, not a retry",
			err:      &APIError{Op: "get wiki node error", Code: 99991663, Msg: "Invalid access token"},
			wantCode: "feishu_auth_or_permission",
			noLeak:   []string{"Invalid access token", "99991663"},
		},
		{
			name:     "wiki permission denied",
			err:      &APIError{Op: "list wiki nodes error", Code: 131006, Msg: "permission denied"},
			wantCode: "feishu_auth_or_permission",
		},
		{
			name:     "download forbidden",
			err:      &APIError{Op: "download failed", Status: http.StatusForbidden},
			wantCode: "feishu_auth_or_permission",
		},
		{
			name:     "unknown error falls back to sync_failed",
			err:      errors.New("some totally unexpected failure"),
			wantCode: "sync_failed",
			noLeak:   []string{"totally unexpected"},
		},
		{
			// Only typed errors classify; keywords in plain text do not.
			name:     "keyword in plain text is not a classification",
			err:      errors.New("failed to decode response: permission timeout api error"),
			wantCode: "sync_failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, codeValue, fallback := feishuFailure(tc.err)
			if code != tc.wantCode {
				t.Errorf("code = %q, want %q", code, tc.wantCode)
			}
			if codeValue != tc.wantCodeValue {
				t.Errorf("codeValue = %q, want %q", codeValue, tc.wantCodeValue)
			}
			if strings.TrimSpace(fallback) == "" {
				t.Errorf("fallback message must not be empty")
			}
			for _, s := range tc.fallbackHas {
				if !strings.Contains(strings.ToLower(fallback), s) {
					t.Errorf("fallback %q missing %q", fallback, s)
				}
			}
			for _, s := range tc.noLeak {
				if strings.Contains(code, s) || strings.Contains(fallback, s) {
					t.Errorf("raw detail %q leaked into code/fallback (code=%q fallback=%q)", s, code, fallback)
				}
			}
		})
	}
}

// The client's real failures reach feishuFailure through its typed errors.
func TestFeishuFailureFromClientErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		download bool
		want     string
	}{
		{"wiki 131006 over HTTP 400", http.StatusBadRequest, `{"code":131006}`, false, "feishu_auth_or_permission"},
		{"api 400 with a code", http.StatusBadRequest, `{"code":1254000,"msg":"bad"}`, false, "feishu_api_error"},
		{"download 403", http.StatusForbidden, "", true, "feishu_auth_or_permission"},
		{"download 404 with a code", http.StatusNotFound, `{"code":1061003}`, true, "feishu_api_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "/target"
			if tc.download {
				path = "/open-apis/drive/v1/files/f/download"
			}
			ts, cfg := retryTestServer(t, path, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			defer ts.Close()

			c := NewClient(cfg)
			var err error
			if tc.download {
				_, err = c.DownloadDriveFile(context.Background(), "f")
			} else {
				err = c.DoRequest(context.Background(), http.MethodGet, path, nil, nil)
			}
			if code, _, _ := feishuFailure(err); code != tc.want {
				t.Errorf("feishuFailure(%v) = %q, want %q", err, code, tc.want)
			}
		})
	}
}
