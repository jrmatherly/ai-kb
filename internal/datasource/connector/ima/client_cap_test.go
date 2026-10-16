package ima

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
)

func TestCallAPI_RejectsOversizedResponse(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write(bytes.Repeat([]byte("a"), 1<<20+1))
	}))
	defer server.Close()

	c := newClient(&Config{ClientID: "cid", APIKey: "cak", BaseURL: server.URL})
	c.jsonLimit = 1 << 20
	err := c.callAPI(context.Background(), "get_knowledge_list", map[string]any{}, &map[string]any{})
	if err == nil {
		t.Fatal("callAPI accepted a response larger than the cap, want error")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Fatalf("error = %v, want an explicit over-limit error", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("server hit %d times, want 1 (oversized responses must not be retried)", got)
	}
}

func TestCallAPIMapsAuthFailures(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		sentinel error
	}{
		{"401 is a bad key", http.StatusUnauthorized, `{"msg":"unauthorized"}`, datasource.ErrInvalidCredentials},
		{"403 is a missing permission", http.StatusForbidden, `{"msg":"forbidden"}`, datasource.ErrPermissionDenied},
		{
			"code 110030 is a credential problem", http.StatusOK, `{"code":110030,"msg":"no permission"}`,
			datasource.ErrInvalidCredentials,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			c := newClient(&Config{ClientID: "cid", APIKey: "cak", BaseURL: server.URL})
			err := c.callAPI(context.Background(), "get_knowledge_list", map[string]any{}, &map[string]any{})
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("error = %v, want %v", err, tc.sentinel)
			}
			for _, other := range []error{datasource.ErrInvalidCredentials, datasource.ErrPermissionDenied} {
				if other != tc.sentinel && errors.Is(err, other) {
					t.Fatalf("error = %v, must not be %v", err, other)
				}
			}
		})
	}
}
