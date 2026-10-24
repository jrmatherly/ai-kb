package github

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/logger"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
	"github.com/stretchr/testify/require"
)

func allowLocalGitHubServer(t *testing.T) {
	t.Helper()
	t.Setenv("SSRF_WHITELIST", "127.0.0.1,::1,localhost")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
}

// newTestServer whitelists loopback and serves handler; 127.0.0.1 is treated as
// an Enterprise Server host, so handlers see paths under /api/v3.
func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	allowLocalGitHubServer(t)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func newTestClient(t *testing.T, server *httptest.Server) *client {
	t.Helper()
	c, err := newClient(server.URL, "per-source-token")
	require.NoError(t, err)
	return c
}

func TestNormalizeBaseURL(t *testing.T) {
	for in, want := range map[string]string{
		"":                                  "https://api.github.com",
		"github.com":                        "https://api.github.com",
		"https://www.github.com/":           "https://api.github.com",
		"https://api.github.com":            "https://api.github.com",
		"octocorp.ghe.com":                  "https://api.octocorp.ghe.com",
		"https://api.octocorp.ghe.com/":     "https://api.octocorp.ghe.com",
		"ghe.example.com":                   "https://ghe.example.com/api/v3",
		"GHE.Example.com":                   "https://ghe.example.com/api/v3",
		"https://ghe.example.com/api/v3/":   "https://ghe.example.com/api/v3",
		"http://127.0.0.1:8080":             "http://127.0.0.1:8080/api/v3",
		"API.Octocorp.GHE.com":              "https://api.octocorp.ghe.com",
		"https://ghe.example.com/git":       "https://ghe.example.com/git/api/v3",
		"https://ghe.example.com/fooapi/v3": "https://ghe.example.com/fooapi/v3/api/v3",
	} {
		require.Equal(t, want, normalizeBaseURL(in), in)
	}
}

func TestNewClientRejectsEmptyTokenAndBlockedHost(t *testing.T) {
	_, err := newClient("", "")
	require.ErrorIs(t, err, datasource.ErrInvalidConfig, "an empty token is a configuration error")
	_, err = newClient("http://127.0.0.1:1", "token") // without the whitelist helper
	require.Error(t, err, "the SSRF policy runs on a non-empty base_url")
}

func TestRequestsCarryGitHubHeaders(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/user" {
			http.NotFound(w, r)
			return
		}
		require.Equal(t, "Bearer per-source-token", r.Header.Get("Authorization"))
		require.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		require.Equal(t, "2022-11-28", r.Header.Get("X-GitHub-Api-Version"))
		_, _ = w.Write([]byte(`{"id":42}`))
	})
	require.NoError(t, newTestClient(t, server).ping(context.Background()))
}

func TestAPIErrorMapsToSentinels(t *testing.T) {
	for _, tc := range []struct {
		status                       int
		invalid, forbidden, notFound bool
	}{
		{http.StatusUnauthorized, true, false, false},
		{http.StatusForbidden, false, true, false},
		{http.StatusNotFound, false, false, true},
		{http.StatusInternalServerError, false, false, false},
	} {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			})
			err := newTestClient(t, server).ping(context.Background())
			require.Error(t, err)
			require.Equal(t, tc.invalid, errors.Is(err, datasource.ErrInvalidCredentials))
			require.Equal(t, tc.forbidden, errors.Is(err, datasource.ErrPermissionDenied))
			require.Equal(t, tc.notFound, errors.Is(err, datasource.ErrResourceNotFound))
		})
	}
}

const secondaryLimitBody = `{"message":"You have exceeded a secondary rate limit. ` +
	`Please wait a few minutes before you try again.","documentation_url":"https://docs.github.com/rest"}`

func TestRateLimitIsNotPermissionDenied(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		hdr    http.Header
		body   string
		reset  string
	}{
		"primary 403": {
			status: http.StatusForbidden,
			hdr:    http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"1800000000"}},
			reset:  "2027-01-15T08:00:00Z",
		},
		"secondary 403": {
			status: http.StatusForbidden,
			hdr:    http.Header{"Retry-After": {"120"}, "X-Ratelimit-Remaining": {"4000"}},
			reset:  "unknown",
		},
		"header-less secondary 403": {
			status: http.StatusForbidden,
			hdr:    http.Header{},
			body:   secondaryLimitBody,
			reset:  "unknown",
		},
		"429": {status: http.StatusTooManyRequests, hdr: http.Header{}, reset: "unknown"},
	} {
		t.Run(name, func(t *testing.T) {
			server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				for k, v := range tc.hdr {
					w.Header()[k] = v
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			err := newTestClient(t, server).ping(context.Background())
			require.ErrorIs(t, err, errRateLimited)
			require.NotErrorIs(t, err, datasource.ErrPermissionDenied)
			require.EqualError(t, err, "github API rate limit exhausted until "+tc.reset)
		})
	}
}

func TestShortRetryAfterIsRetriedOnce(t *testing.T) {
	for name, tc := range map[string]struct {
		retryAfter   string
		limitedTimes int64
		wantErr      error
		wantRequests int64
		minElapsed   time.Duration
	}{
		"retried then served": {retryAfter: "1", limitedTimes: 1, wantRequests: 2, minElapsed: time.Second},
		"over the cap":        {retryAfter: "120", limitedTimes: 1, wantErr: errRateLimited, wantRequests: 1},
		"limited twice":       {retryAfter: "1", limitedTimes: 2, wantErr: errRateLimited, wantRequests: 2},
	} {
		t.Run(name, func(t *testing.T) {
			var requests int64
			server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				if atomic.AddInt64(&requests, 1) <= tc.limitedTimes {
					w.Header().Set("Retry-After", tc.retryAfter)
					w.Header().Set("X-RateLimit-Remaining", "4000")
					w.WriteHeader(http.StatusForbidden)
					return
				}
				_, _ = w.Write([]byte(`{"id":42}`))
			})
			start := time.Now()
			err := newTestClient(t, server).ping(context.Background())
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantRequests, atomic.LoadInt64(&requests))
			require.GreaterOrEqual(t, time.Since(start), tc.minElapsed)
		})
	}
}

func TestRepositoriesFollowLinkHeader(t *testing.T) {
	var (
		mu    sync.Mutex
		pages []string
	)
	var server *httptest.Server
	server = newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/user/repos", r.URL.Path)
		q := r.URL.Query()
		page := q.Get("page")
		if page == "" {
			page = "1"
			require.Equal(t, "100", q.Get("per_page"))
			require.Equal(t, "full_name", q.Get("sort"))
			require.Equal(t, "owner,collaborator,organization_member", q.Get("affiliation"))
		} else {
			require.Equal(t, "owner,collaborator", q.Get("affiliation"), "the next link is requested as advertised")
		}
		mu.Lock()
		pages = append(pages, page)
		mu.Unlock()
		n, _ := strconv.Atoi(page)
		count := 100
		if n == 3 {
			count = 1
		} else {
			// The literal commas in the query must not split the Link header.
			repos := server.URL + "/api/v3/user/repos?per_page=100&affiliation=owner,collaborator"
			w.Header().Set("Link", fmt.Sprintf(`<%s&page=%d>; rel="next", <%s&page=3>; rel="last"`, repos, n+1, repos))
		}
		items := make([]string, count)
		for i := range items {
			items[i] = fmt.Sprintf(`{"id":%d,"name":"r%d","full_name":"o/r%d",`+
				`"html_url":"https://github.com/o/r%d","default_branch":"main"}`, n*1000+i, i, i, i)
		}
		_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
	})

	repos, err := newTestClient(t, server).repositories(context.Background())
	require.NoError(t, err)
	require.Len(t, repos, 201)
	require.Equal(t, []string{"1", "2", "3"}, pages)
	require.Equal(t, repository{
		ID: 3000, Name: "r0", FullName: "o/r0", HTMLURL: "https://github.com/o/r0", DefaultBranch: "main",
	}, repos[200])
}

func TestRepositoriesStopAtPaginationGuard(t *testing.T) {
	var requests int64
	var server *httptest.Server
	server = newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt64(&requests, 1)
		if n > datasource.MaxPaginationHops+10 {
			http.Error(w, "hop cap missing", http.StatusBadRequest)
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s/api/v3/user/repos?page=%d>; rel="next"`, server.URL, n+1))
		_, _ = w.Write([]byte(`[]`))
	})
	var logs bytes.Buffer
	logger.SetOutput(&logs)
	t.Cleanup(logger.ConfigureFromEnv)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := newTestClient(t, server).repositories(ctx)
	require.EqualError(t, err, "github repository pagination exceeded 10000 pages")
	require.Equal(t, int64(datasource.MaxPaginationHops), atomic.LoadInt64(&requests))
	require.Contains(t, logs.String(), "pagination exceeded 10000 pages")
}

func TestRepositoriesRejectNextLinkOffBaseURL(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Link", `<https://elsewhere.example/api/v3/user/repos?page=2>; rel="next"`)
		_, _ = w.Write([]byte(`[]`))
	})
	_, err := newTestClient(t, server).repositories(context.Background())
	require.ErrorContains(t, err, "outside the API base URL")
}

func TestRepositoryAndCommitSHAPaths(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/api/v3/repos/o/r":
			_, _ = w.Write([]byte(`{"id":7,"name":"r","full_name":"O/r",` +
				`"html_url":"https://github.com/O/r","default_branch":"trunk"}`))
		case "/api/v3/repos/o/r/commits/feature%2Fx":
			_, _ = w.Write([]byte(`{"sha":"deadbeef"}`))
		default:
			http.NotFound(w, r)
		}
	})
	c := newTestClient(t, server)

	repo, err := c.repository(context.Background(), "o/r")
	require.NoError(t, err)
	require.Equal(t, &repository{
		ID: 7, Name: "r", FullName: "O/r", HTMLURL: "https://github.com/O/r", DefaultBranch: "trunk",
	}, repo)

	sha, err := c.commitSHA(context.Background(), "o/r", "feature/x")
	require.NoError(t, err)
	require.Equal(t, "deadbeef", sha)
}

func TestTreeReportsTruncationAndUsesTreeCap(t *testing.T) {
	body := `{"sha":"t1","truncated":true,` +
		`"tree":[{"path":"docs/a.md","mode":"100644","type":"blob","sha":"b1","size":12}]}`
	var gotQuery []string
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/repos/o/r/git/trees/abc", r.URL.Path)
		gotQuery = append(gotQuery, r.URL.RawQuery)
		_, _ = w.Write([]byte(body))
	})
	c := newTestClient(t, server)

	got, err := c.tree(context.Background(), "o/r", "abc", true)
	require.NoError(t, err)
	require.Equal(t, &tree{SHA: "t1", Truncated: true, Entries: []treeEntry{
		{Path: "docs/a.md", Mode: "100644", Type: "blob", SHA: "b1", Size: 12},
	}}, got)

	_, err = c.tree(context.Background(), "o/r", "abc", false)
	require.NoError(t, err)
	require.Equal(t, []string{"recursive=1", ""}, gotQuery)

	c.treeLimit = 16
	_, err = c.tree(context.Background(), "o/r", "abc", true)
	require.ErrorContains(t, err, "exceeds maximum size")
}

func TestBlobUsesRawMediaTypeAndCap(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/repos/o/r/git/blobs/b1", r.URL.Path)
		require.Equal(t, "application/vnd.github.raw+json", r.Header.Get("Accept"))
		require.Equal(t, "Bearer per-source-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte("123456789"))
	})
	c := newTestClient(t, server)

	data, err := c.blob(context.Background(), "o/r", "b1")
	require.NoError(t, err)
	require.Equal(t, "123456789", string(data))

	c.rawLimit = 8
	_, err = c.blob(context.Background(), "o/r", "b1")
	require.ErrorContains(t, err, "exceeds maximum size")
}
