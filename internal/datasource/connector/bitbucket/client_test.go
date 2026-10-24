package bitbucket

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
	"github.com/stretchr/testify/require"
)

func allowLocalServer(t *testing.T) {
	t.Helper()
	t.Setenv("SSRF_WHITELIST", "127.0.0.1,::1,localhost")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
}

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	allowLocalServer(t)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func newTestClient(t *testing.T, server *httptest.Server, username string) *client {
	t.Helper()
	c, err := newClient(server.URL, username, "per-source-token")
	require.NoError(t, err)
	return c
}

type recorder struct {
	mu   sync.Mutex
	uris []string
}

func (r *recorder) add(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.uris = append(r.uris, req.URL.RequestURI())
}

func (r *recorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.uris...)
}

func TestNewClientRejectsEmptyToken(t *testing.T) {
	_, err := newClient(apiBaseURL, "", "")
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
}

func TestAuthHeaderIsBasicWithUsernameAndBearerWithout(t *testing.T) {
	var auth, accept []string
	var mu sync.Mutex
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auth = append(auth, r.Header.Get("Authorization"))
		accept = append(accept, r.Header.Get("Accept"))
		mu.Unlock()
		_, _ = w.Write([]byte(`{"uuid":"{u}"}`))
	})

	require.NoError(t, newTestClient(t, server, " me@example.com ").ping(context.Background()))
	require.NoError(t, newTestClient(t, server, "").ping(context.Background()))

	basic := "Basic " + base64.StdEncoding.EncodeToString([]byte("me@example.com:per-source-token"))
	require.Equal(t, []string{basic, "Bearer per-source-token"}, auth)
	require.Equal(t, []string{"application/json", "application/json"}, accept)
}

func TestAPIErrorMapsToSentinels(t *testing.T) {
	sentinels := []error{
		datasource.ErrInvalidCredentials, datasource.ErrPermissionDenied,
		datasource.ErrResourceNotFound, errTimedOut,
	}
	for status, want := range map[int]error{
		http.StatusUnauthorized:        datasource.ErrInvalidCredentials,
		http.StatusForbidden:           datasource.ErrPermissionDenied,
		http.StatusNotFound:            datasource.ErrResourceNotFound,
		statusTimedOut:                 errTimedOut,
		http.StatusInternalServerError: nil,
	} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			})
			err := newTestClient(t, server, "").ping(context.Background())
			var apiErr *apiError
			require.ErrorAs(t, err, &apiErr)
			require.Equal(t, status, apiErr.status)
			require.Contains(t, err.Error(), "/user")
			for _, s := range sentinels {
				require.Equal(t, s == want, errors.Is(err, s), "status %d vs %v", status, s)
			}
		})
	}
}

func TestRateLimitWaitsOnceForAShortReset(t *testing.T) {
	var requests int64
	server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt64(&requests, 1) == 2 {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.Header().Set("x-ratelimit-reset", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	c := newTestClient(t, server, "")

	start := time.Now()
	require.NoError(t, c.ping(context.Background()))
	require.GreaterOrEqual(t, time.Since(start), time.Second)
	require.Equal(t, int64(2), atomic.LoadInt64(&requests))
	require.True(t, c.waited)

	err := c.ping(context.Background())
	require.ErrorIs(t, err, errRateLimited)
	require.NotErrorIs(t, err, datasource.ErrPermissionDenied)
	require.Equal(t, int64(3), atomic.LoadInt64(&requests))
}

func TestRateLimitBeyondTheWaitEndsTheRun(t *testing.T) {
	for reset, want := range map[string]string{"3600": "resets in 1h0m0s", "": "resets at an unknown time"} {
		t.Run(want, func(t *testing.T) {
			var requests int64
			server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt64(&requests, 1)
				if reset != "" {
					w.Header().Set("x-ratelimit-reset", reset)
				}
				w.WriteHeader(http.StatusTooManyRequests)
			})
			c := newTestClient(t, server, "")

			err := c.ping(context.Background())

			require.ErrorIs(t, err, errRateLimited)
			require.Contains(t, err.Error(), want)
			require.Equal(t, int64(1), atomic.LoadInt64(&requests))
			require.False(t, c.waited)
		})
	}
}

func TestRateLimitWaitStopsWhenTheContextIsCancelled(t *testing.T) {
	var requests int64
	server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&requests, 1)
		w.Header().Set("x-ratelimit-reset", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	c := newTestClient(t, server, "")
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	err := c.ping(ctx)

	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(start), 5*time.Second)
	require.Equal(t, int64(1), atomic.LoadInt64(&requests))
}

func TestPagesFollowNextOnly(t *testing.T) {
	var rec recorder
	var server *httptest.Server
	server = newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		next := ""
		if page < 3 {
			next = fmt.Sprintf("%s/items?page=%d", server.URL, page+1)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"values": []int{page}, "next": next, "previous": server.URL + "/items?page=0",
		})
	})
	c := newTestClient(t, server, "")

	var got []int
	err := c.pages(context.Background(), server.URL+"/items?page=1", func(values json.RawMessage) error {
		var page []int
		if err := json.Unmarshal(values, &page); err != nil {
			return err
		}
		got = append(got, page...)
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, got)
	require.Equal(t, []string{"/items?page=1", "/items?page=2", "/items?page=3"}, rec.all())
}

func TestPagesRejectRepeatedOffHostAndEndlessNext(t *testing.T) {
	ignore := func(json.RawMessage) error { return nil }

	t.Run("repeated", func(t *testing.T) {
		var requests int64
		server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt64(&requests, 1) > 3 {
				http.Error(w, "repeated-page guard missing", http.StatusTeapot)
				return
			}
			self := "http://" + r.Host + r.URL.RequestURI()
			_ = json.NewEncoder(w).Encode(map[string]any{"values": []int{}, "next": self})
		})
		c := newTestClient(t, server, "")

		err := c.pages(context.Background(), server.URL+"/items?page=1", ignore)

		require.ErrorContains(t, err, "repeated")
		require.Equal(t, int64(1), atomic.LoadInt64(&requests))
	})

	t.Run("off host", func(t *testing.T) {
		var requests int64
		server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt64(&requests, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"values": []int{}, "next": "https://evil.example/items?page=2",
			})
		})
		c := newTestClient(t, server, "")

		err := c.pages(context.Background(), server.URL+"/items?page=1", ignore)

		require.ErrorContains(t, err, "outside the API base URL")
		require.Equal(t, int64(1), atomic.LoadInt64(&requests))
	})

	t.Run("endless", func(t *testing.T) {
		var requests int64
		var server *httptest.Server
		server = newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt64(&requests, 1) > datasource.MaxPaginationHops+10 {
				http.Error(w, "hop cap missing", http.StatusTeapot)
				return
			}
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"values": []int{}, "next": fmt.Sprintf("%s/items?page=%d", server.URL, page+1),
			})
		})
		c := newTestClient(t, server, "")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := c.pages(ctx, server.URL+"/items?page=1", ignore)

		require.ErrorContains(t, err, "exceeded "+strconv.Itoa(datasource.MaxPaginationHops)+" pages")
		require.Equal(t, int64(datasource.MaxPaginationHops), atomic.LoadInt64(&requests))
	})
}

func TestHeadCommitResolvesBranchThenTagThenHash(t *testing.T) {
	hash := strings.Repeat("ab", 20)
	var rec recorder
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		switch r.URL.EscapedPath() {
		case "/repositories/ws/repo/refs/branches/release%2F1.0":
			_, _ = w.Write([]byte(`{"target":{"hash":"b1"}}`))
		case "/repositories/ws/repo/refs/branches/empty":
			_, _ = w.Write([]byte(`{"target":{}}`))
		case "/repositories/ws/repo/refs/tags/v1":
			_, _ = w.Write([]byte(`{"target":{"hash":"t1"}}`))
		case "/repositories/ws/repo/commit/" + hash:
			_, _ = w.Write([]byte(`{"hash":"` + hash + `"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	c := newTestClient(t, server, "")
	ctx := context.Background()

	for ref, want := range map[string]string{"release/1.0": "b1", "v1": "t1", hash: hash} {
		got, err := c.headCommit(ctx, "ws/repo", ref)
		require.NoError(t, err, ref)
		require.Equal(t, want, got, ref)
	}

	_, err := c.headCommit(ctx, "ws/repo", "empty")
	require.ErrorContains(t, err, "no commit hash")

	rec = recorder{}
	_, err = c.headCommit(ctx, "ws/repo", "nope")
	require.ErrorIs(t, err, datasource.ErrResourceNotFound)
	require.Equal(t, []string{
		"/repositories/ws/repo/refs/branches/nope", "/repositories/ws/repo/refs/tags/nope",
	}, rec.all())
}

func TestWorkspacesRepositoriesAndCommitExists(t *testing.T) {
	hash := strings.Repeat("cd", 20)
	var rec recorder
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		switch r.URL.EscapedPath() {
		case "/user/workspaces":
			_, _ = w.Write([]byte(`{"values":[{"slug":"beta"},{"slug":"alpha"}]}`))
		case "/repositories/my%20ws":
			_, _ = w.Write([]byte(`{"values":[{"uuid":"{r1}","full_name":"my ws/docs",` +
				`"mainbranch":{"name":"main"},"links":{"html":{"href":"https://bitbucket.org/ws/docs"}}}]}`))
		case "/repositories/ws/docs":
			_, _ = w.Write([]byte(`{"uuid":"{r1}","full_name":"ws/docs"}`))
		case "/repositories/ws/docs/commit/" + hash:
			_, _ = w.Write([]byte(`{"hash":"` + hash + `"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	c := newTestClient(t, server, "")
	ctx := context.Background()

	slugs, err := c.workspaces(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"beta", "alpha"}, slugs)

	repos, err := c.repositories(ctx, "my ws")
	require.NoError(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, "{r1}", repos[0].UUID)
	require.Equal(t, "docs", repos[0].slug())
	require.Equal(t, "main", repos[0].MainBranch.Name)
	require.Equal(t, "https://bitbucket.org/ws/docs", repos[0].Links.HTML.Href)

	repo, err := c.repository(ctx, "ws/docs")
	require.NoError(t, err)
	require.Nil(t, repo.MainBranch)

	exists, err := c.commitExists(ctx, "ws/docs", hash)
	require.NoError(t, err)
	require.True(t, exists)
	exists, err = c.commitExists(ctx, "ws/docs", "gone")
	require.NoError(t, err)
	require.False(t, exists)

	require.Equal(t, []string{
		"/user/workspaces?pagelen=100&fields=values.slug,next",
		"/repositories/my%20ws?role=member&pagelen=100&fields=values.full_name,values.uuid," +
			"values.mainbranch.name,values.links.html.href,next",
		"/repositories/ws/docs",
		"/repositories/ws/docs/commit/" + hash,
		"/repositories/ws/docs/commit/gone",
	}, rec.all())
}

func TestListingSendsDepthFieldsAndEscapesSegments(t *testing.T) {
	var rec recorder
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		_, _ = w.Write([]byte(`{"values":[{"type":"commit_file","path":"docs/a b/x.md",` +
			`"size":3,"attributes":["lfs"]}]}`))
	})
	c := newTestClient(t, server, "")

	entries, err := c.listing(context.Background(), "ws/repo", "c1", "docs/a b", listingDepth)
	require.NoError(t, err)
	want := srcEntry{Type: "commit_file", Path: "docs/a b/x.md", Size: 3, Attributes: []string{"lfs"}}
	require.Equal(t, []srcEntry{want}, entries)
	require.True(t, entries[0].has("lfs"))
	require.False(t, entries[0].has("binary"))

	_, err = c.listing(context.Background(), "ws/repo", "c1", "", fallbackListingDepth)
	require.NoError(t, err)

	fields := "&pagelen=100&fields=values.path,values.type,values.size,values.attributes,next"
	require.Equal(t, []string{
		"/repositories/ws/repo/src/c1/docs/a%20b/?max_depth=50" + fields,
		"/repositories/ws/repo/src/c1/?max_depth=10" + fields,
	}, rec.all())
}

func TestDiffstatSendsNewThenOld(t *testing.T) {
	var rec recorder
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		_, _ = w.Write([]byte(`{"values":[{"status":"added","new":{"path":"a.md"}},` +
			`{"status":"modified","old":{"path":"b.md"},"new":{"path":"b.md"}}]}`))
	})
	c := newTestClient(t, server, "")

	entries, err := c.diffstat(context.Background(), "ws/repo", "new", "old")

	require.NoError(t, err)
	require.Equal(t, []string{
		"/repositories/ws/repo/diffstat/new..old" + diffstatQuery,
	}, rec.all())
	require.Len(t, entries, 2)
	require.Equal(t, "added", entries[0].Status)
	require.Nil(t, entries[0].Old)
	require.Equal(t, "a.md", entries[0].New.Path)
	require.Equal(t, "modified", entries[1].Status)
	require.Equal(t, "b.md", entries[1].Old.Path)
}

func TestRawDoesNotFollowRedirects(t *testing.T) {
	var requests int64
	var headers http.Header
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requests, 1)
		headers = r.Header.Clone()
		if r.URL.Path == "/repositories/ws/repo/src/c1/big.bin" {
			w.Header().Set("Location", "https://media.example/x")
			w.WriteHeader(http.StatusMovedPermanently)
			return
		}
		_, _ = w.Write([]byte("123456789"))
	})
	c := newTestClient(t, server, "")

	_, err := c.raw(context.Background(), "ws/repo", "c1", "big.bin")
	require.ErrorIs(t, err, errLFSRedirect)
	require.Equal(t, int64(1), atomic.LoadInt64(&requests))

	c.rawLimit = 8
	_, err = c.raw(context.Background(), "ws/repo", "c1", "docs/nine.txt")
	require.ErrorContains(t, err, "exceeds maximum size")
	require.Empty(t, headers.Get("Accept"))
	require.Equal(t, "Bearer per-source-token", headers.Get("Authorization"))

	c.rawLimit = 9
	body, err := c.raw(context.Background(), "ws/repo", "c1", "docs/nine.txt")
	require.NoError(t, err)
	require.Equal(t, "123456789", string(body))
}
