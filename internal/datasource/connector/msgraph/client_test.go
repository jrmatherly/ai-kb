package msgraph

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/stretchr/testify/require"
)

func TestAccessTokenIsCachedAndRemintedOn401(t *testing.T) {
	f := newFakeGraph(t)
	f.handle("GET /drives/b!abc", f.requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"b!abc","name":"Documents","webUrl":"https://x"}`))
	}))
	c := f.client(t)
	_, err := c.drive(context.Background(), "b!abc")
	require.NoError(t, err)
	_, err = c.drive(context.Background(), "b!abc")
	require.NoError(t, err)
	require.Equal(t, int32(1), f.tokens.Load(), "second call reuses the cached token")
	// The server rotates the accepted token: the cached one is rejected once, re-minted, and the call succeeds.
	f.token = "tok-2"
	_, err = c.drive(context.Background(), "b!abc")
	require.NoError(t, err)
	require.Equal(t, int32(2), f.tokens.Load(), "a 401 re-mints once")
	require.Equal(t, 4, f.seen("GET /drives/b!abc"), "two cached calls, one rejected, one retried")
}

func TestAccessTokenBadSecretIsInvalidCredentials(t *testing.T) {
	f := newFakeGraph(t)
	f.tokenStatus = http.StatusUnauthorized
	_, err := f.client(t).accessToken(context.Background())
	require.ErrorIs(t, err, datasource.ErrInvalidCredentials)
}

func TestSiteAddressesByHostAndPath(t *testing.T) {
	f := newFakeGraph(t)
	f.json(
		"GET /sites/contoso.sharepoint.com:/sites/hr",
		http.StatusOK,
		map[string]any{
			"id":          "contoso.sharepoint.com,1,2",
			"displayName": "HR",
			"webUrl":      "https://contoso.sharepoint.com/sites/hr",
		},
	)
	f.json("GET /sites/contoso.sharepoint.com", http.StatusOK, map[string]any{"id": "root-site", "displayName": "Root"})
	c := f.client(t)
	site, err := c.site(context.Background(), "contoso.sharepoint.com", "sites/hr")
	require.NoError(t, err)
	require.Equal(t, "contoso.sharepoint.com,1,2", site.ID)
	site, err = c.site(context.Background(), "contoso.sharepoint.com", "")
	require.NoError(t, err)
	require.Equal(t, "root-site", site.ID)
}

func TestStatusMapping(t *testing.T) {
	f := newFakeGraph(t)
	f.json(
		"GET /drives/forbidden",
		http.StatusForbidden,
		map[string]any{"error": map[string]any{"code": "accessDenied"}},
	)
	c := f.client(t)
	_, err := c.drive(context.Background(), "forbidden")
	require.ErrorIs(t, err, datasource.ErrPermissionDenied)
	_, err = c.drive(context.Background(), "missing")
	require.ErrorIs(t, err, datasource.ErrResourceNotFound)
}

func TestChildFoldersFollowsNextLinkAndKeepsFoldersOnly(t *testing.T) {
	f := newFakeGraph(t)
	f.json("GET /drives/b!abc/items/01F/children", http.StatusOK, map[string]any{
		"value": []any{
			map[string]any{"id": "01A", "name": "Policies", "folder": map[string]any{}},
			map[string]any{"id": "01B", "name": "note.txt", "file": map[string]any{}},
		},
		"@odata.nextLink": f.server.URL + "/drives/b!abc/items/01F/children?$skiptoken=2",
	})
	f.handle("GET /drives/b!abc/items/01F/children", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("$skiptoken") == "2" {
			_, _ = w.Write([]byte(`{"value":[{"id":"01C","name":"Archive","folder":{}}]}`))
			return
		}
		require.Contains(t, r.URL.Query().Get("$select"), "remoteItem", "the shortcut filter needs its facet")
		require.Contains(t, r.URL.Query().Get("$select"), "deleted")
		_, _ = w.Write(
			[]byte(`{"value":[{"id":"01A","name":"Policies","folder":{}},{"id":"01B","name":"note.txt","file":{}},` +
				`{"id":"01D","name":"Shared","folder":{},"remoteItem":{}},` +
				`{"id":"01E","name":"Gone","folder":{},"deleted":{}}],` +
				`"@odata.nextLink":"` + f.server.URL + `/drives/b!abc/items/01F/children?$skiptoken=2"}`),
		)
	})
	folders, err := f.client(t).childFolders(context.Background(), "b!abc", "01F")
	require.NoError(t, err)
	require.Len(t, folders, 2)
	require.Equal(t, "Policies", folders[0].Name)
	require.Equal(t, "Archive", folders[1].Name)
}

func TestPagesRejectOffHostLink(t *testing.T) {
	f := newFakeGraph(t)
	f.handle("GET /drives/b!abc/root/children", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"value":[],"@odata.nextLink":"https://evil.example/next"}`))
	})
	_, err := f.client(t).childFolders(context.Background(), "b!abc", "")
	require.ErrorContains(t, err, "outside the Graph host")
}

func TestDeltaPageReturnsResyncOn410(t *testing.T) {
	f := newFakeGraph(t)
	f.handle("GET /drives/b!abc/root/delta", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", f.server.URL+"/drives/b!abc/root/delta?token=fresh")
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write(
			[]byte(`{"error":{"code":"resyncRequired","innerError":{"code":"resyncChangesApplyDifferences"}}}`),
		)
	})
	c := f.client(t)
	_, err := c.deltaPage(context.Background(), c.deltaStart("b!abc", ""))
	var resync *resyncError
	require.ErrorAs(t, err, &resync)
	require.Equal(t, f.server.URL+"/drives/b!abc/root/delta?token=fresh", resync.Location)
}

func TestRateLimitWaitsOnceThenSurfaces(t *testing.T) {
	f := newFakeGraph(t)
	hits := 0
	f.handle("GET /drives/b!abc", func(w http.ResponseWriter, _ *http.Request) {
		hits++
		if hits <= 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"id":"b!abc","name":"Documents"}`))
	})
	c := f.client(t)
	start := time.Now()
	_, err := c.drive(context.Background(), "b!abc")
	require.ErrorIs(t, err, errRateLimited, "the second 429 in a run is not waited for")
	require.GreaterOrEqual(t, time.Since(start), time.Second, "the first 429 was waited for")
	require.Equal(t, 2, hits)
}

func TestDownloadFollowsRedirectWithoutToken(t *testing.T) {
	f := newFakeGraph(t)
	body := []byte("%PDF-1.4 hello")
	f.handle("GET /drives/b!abc/items/01B/content", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok-1", r.Header.Get("Authorization"))
		http.Redirect(w, r, f.server.URL+"/download/01B?tempauth=x", http.StatusFound)
	})
	f.handle("GET /download/01B", func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Authorization"), "the pre-authenticated URL gets no bearer token")
		_, _ = w.Write(body)
	})
	got, err := f.client(t).download(context.Background(), "b!abc", "01B", int64(len(body)), 1<<20)
	require.NoError(t, err)
	require.Equal(t, body, got)
}

func TestDownloadRejectsInsecureLocation(t *testing.T) {
	f := newFakeGraph(t)
	f.handle("GET /drives/b!abc/items/01B/content", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://downloads.example/01B", http.StatusFound)
	})
	_, err := f.client(t).download(context.Background(), "b!abc", "01B", 10, 1<<20)
	require.ErrorIs(t, err, errInsecureDownload)
}

func TestDownloadSizeMismatchAndEmptyAndTooLarge(t *testing.T) {
	f := newFakeGraph(t)
	f.handle("GET /drives/b!abc/items/01B/content", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, f.server.URL+"/download/01B", http.StatusFound)
	})
	f.handle("GET /download/01B", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("abc")) })
	c := f.client(t)
	_, err := c.download(context.Background(), "b!abc", "01B", 5, 1<<20)
	require.ErrorIs(t, err, errSourceChanged)
	_, err = c.download(context.Background(), "b!abc", "01B", 3, 2)
	require.ErrorIs(t, err, errTooLarge)
	f.handle("GET /download/01B", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	_, err = c.download(context.Background(), "b!abc", "01B", 0, 1<<20)
	require.ErrorIs(t, err, errEmptyFile)
}

func TestNewClientRejectsPrivateHostWithoutWhitelist(t *testing.T) {
	cfg := &config{TenantID: "t", ClientID: "c", ClientSecret: "s"}
	c := newClient("http://127.0.0.1:1", "http://127.0.0.1:1", cfg)
	_, err := c.accessToken(context.Background())
	require.ErrorContains(t, err, "SSRF", "the SSRF dial guard refuses a loopback host unless whitelisted")
	require.False(t, errors.Is(err, datasource.ErrInvalidCredentials))
}

func TestThrottleWithoutRetryAfterBacksOffThreeTimes(t *testing.T) {
	f := newFakeGraph(t)
	hits := 0
	f.handle("GET /drives/b!abc", func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	c := f.client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start := time.Now()
	_, err := c.drive(ctx, "b!abc")
	require.ErrorIs(t, err, errRateLimited)
	require.Equal(t, 4, hits, "the first answer plus three backed-off retries")
	require.GreaterOrEqual(t, time.Since(start), 14*time.Second, "2 + 4 + 8 seconds of backoff")
}

func TestErrorsCarryNoQueryStringOrLink(t *testing.T) {
	f := newFakeGraph(t)
	f.json(
		"GET /drives/b!abc/root/delta",
		http.StatusInternalServerError,
		map[string]any{"error": map[string]any{"code": "x"}},
	)
	c := f.client(t)
	_, err := c.deltaPage(context.Background(), c.deltaStart("b!abc", "")+"&token=SECRET")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "SECRET")
	require.NotContains(t, err.Error(), "$select")
	require.NotContains(t, (&resyncError{Location: "https://x/delta?token=SECRET"}).Error(), "SECRET")

	f.handle("GET /drives/b!abc/root/delta", func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		require.NoError(t, err)
		_ = conn.Close()
	})
	_, err = c.deltaPage(context.Background(), c.deltaStart("b!abc", "")+"&token=SECRET")
	require.Error(t, err)
	require.Contains(t, err.Error(), "/drives/b!abc/root/delta", "a transport error names the path")
	require.NotContains(t, err.Error(), "SECRET")
	require.NotContains(t, err.Error(), "$select")
	require.NotContains(t, err.Error(), f.server.URL)
}

func TestSiteDrivesUserDriveAndItem(t *testing.T) {
	f := newFakeGraph(t)
	f.json("GET /sites/contoso.sharepoint.com,1,2/drives", http.StatusOK, map[string]any{
		"value": []any{
			map[string]any{"id": "b!1", "name": "Documents"},
			map[string]any{"id": "b!2", "name": "Policies"},
		},
	})
	f.json("GET /users/alice@contoso.com/drive", http.StatusOK, map[string]any{"id": "b!me", "name": "OneDrive"})
	f.json("GET /drives/b!abc/items/01B", http.StatusOK, map[string]any{
		"id": "01B", "name": "a.pdf", "size": 3, "file": map[string]any{"mimeType": "application/pdf"},
		"parentReference": map[string]any{"driveId": "b!abc", "id": "01F"},
	})
	c := f.client(t)
	drives, err := c.siteDrives(context.Background(), "contoso.sharepoint.com,1,2")
	require.NoError(t, err)
	require.Equal(t, []driveInfo{{ID: "b!1", Name: "Documents"}, {ID: "b!2", Name: "Policies"}}, drives)
	d, err := c.userDrive(context.Background(), "alice@contoso.com")
	require.NoError(t, err)
	require.Equal(t, "b!me", d.ID)
	it, err := c.item(context.Background(), "b!abc", "01B")
	require.NoError(t, err)
	require.True(t, it.isFile())
	require.Equal(t, "01F", it.ParentReference.ID)
	it.Package = &struct{}{}
	require.False(t, it.isFile(), "a package such as a OneNote notebook is not a file to import")
}

func TestDownloadTransportErrorKeepsItsCause(t *testing.T) {
	f := newFakeGraph(t)
	f.handle("GET /drives/b!abc/items/01B/content", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, f.server.URL+"/download/01B?tempauth=SECRET", http.StatusFound)
	})
	f.handle("GET /download/01B", func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		require.NoError(t, err)
		_ = conn.Close()
	})
	c := f.client(t)
	_, err := c.download(context.Background(), "b!abc", "01B", 3, 1<<20)
	require.ErrorIs(t, err, datasource.ErrFetchFailed)
	require.Contains(t, err.Error(), "EOF", "the transport cause survives")
	require.NotContains(t, err.Error(), "SECRET", "the pre-authenticated URL does not")

	ctx, cancel := context.WithCancel(context.Background())
	f.handle("GET /download/01B", func(w http.ResponseWriter, _ *http.Request) {
		cancel()
		<-time.After(200 * time.Millisecond)
		_, _ = w.Write([]byte("abc"))
	})
	_, err = c.download(ctx, "b!abc", "01B", 3, 1<<20)
	require.ErrorIs(t, err, context.Canceled, "a cancelled download ends the run instead of failing one file")
}

func TestGraphErrorBodyCodeAndMessageSurvive(t *testing.T) {
	f := newFakeGraph(t)
	f.json("GET /drives/b!abc", http.StatusBadRequest, map[string]any{
		"error": map[string]any{"code": "invalidRequest", "message": "Invalid hostname for this tenancy"},
	})
	_, err := f.client(t).drive(context.Background(), "b!abc")
	require.ErrorContains(t, err, "status 400")
	require.ErrorContains(t, err, "invalidRequest")
	require.ErrorContains(t, err, "Invalid hostname for this tenancy")
}
