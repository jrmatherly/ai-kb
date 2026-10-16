package confluence

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
)

func TestPaginateRejectsLoop(t *testing.T) {
	client := &client{cfg: config{baseURL: "https://confluence.test"}}
	err := client.paginate(context.Background(), "/rest/api/space?limit=100", func(string) (string, error) {
		return "/rest/api/space?limit=100", nil
	})
	if err == nil || !strings.Contains(err.Error(), "looped") {
		t.Fatalf("paginate() error = %v", err)
	}
}

func TestGetIncludesClientErrorBody(t *testing.T) {
	client := &client{
		cfg: config{baseURL: "https://confluence.test", username: "reader", secret: "secret"},
		http: &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString("invalid CQL expression")),
			}, nil
		})},
	}

	err := client.get(context.Background(), "/rest/api/content/search", nil)
	if err == nil || !strings.Contains(err.Error(), "invalid CQL expression") {
		t.Fatalf("get() error = %v", err)
	}
}

func TestPingUsesEditionEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  config
		want string
	}{
		{name: "server", cfg: config{baseURL: "https://confluence.test"}, want: "/rest/api/space"},
		{
			name: "cloud",
			cfg:  config{baseURL: "https://team.atlassian.net/wiki", edition: editionCloud},
			want: "/wiki/api/v2/spaces",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var path string
			transport := roundTripper(func(req *http.Request) (*http.Response, error) {
				path = req.URL.Path
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("{}")),
				}, nil
			})
			client := &client{cfg: tc.cfg, http: &http.Client{Transport: transport}}

			if err := client.ping(context.Background()); err != nil {
				t.Fatal(err)
			}
			if path != tc.want {
				t.Fatalf("ping path = %q, want %q", path, tc.want)
			}
		})
	}
}

func TestGetMapsAuthStatusesToSentinels(t *testing.T) {
	for _, tc := range []struct {
		status     int
		credential bool
		permission bool
	}{
		{http.StatusUnauthorized, true, false},
		{http.StatusForbidden, false, true},
		{http.StatusNotFound, false, false},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			client := &client{
				cfg: config{baseURL: "https://confluence.test", username: "reader", secret: "secret"},
				http: &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: tc.status, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString("")),
					}, nil
				})},
			}
			err := client.get(context.Background(), "/rest/api/space?limit=1", nil)
			if err == nil {
				t.Fatal("get() error = nil")
			}
			if got := errors.Is(err, datasource.ErrInvalidCredentials); got != tc.credential {
				t.Errorf("errors.Is(err, ErrInvalidCredentials) = %v, want %v (%v)", got, tc.credential, err)
			}
			if got := errors.Is(err, datasource.ErrPermissionDenied); got != tc.permission {
				t.Errorf("errors.Is(err, ErrPermissionDenied) = %v, want %v (%v)", got, tc.permission, err)
			}
		})
	}
}
