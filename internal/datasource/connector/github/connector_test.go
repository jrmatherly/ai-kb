package github

import (
	"context"
	"crypto/sha1" // git object ids are SHA-1 by definition
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

const goodToken = "per-source-token"

const repositoryJSON = `{"id":7,"name":"R","full_name":"o/R",` +
	`"html_url":"https://github.com/o/R","default_branch":"main"}`

// fakeCommit is the content of one commit: path to body, with optional mode and
// reported-size overrides per path.
type fakeCommit struct {
	files map[string]string
	modes map[string]string
	sizes map[string]int64
}

type fakeRepo struct {
	commits map[string]fakeCommit
	// refs maps a ref name to a commit; "main" defaults to the only commit.
	refs map[string]string
	// truncate makes the recursive tree endpoint answer truncated with no entries.
	truncate bool
	// missing lists commits whose tree request answers 404.
	missing map[string]bool
	// aliases lists further repository keys served with the same content as o/r.
	aliases []string
}

func blobSHA(content string) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(content), content)))
	return hex.EncodeToString(sum[:])
}

// index lists every directory of a commit: dir to its name-relative entries,
// and dir to its tree id (the SHA-1 of its sorted entry list, so equal
// directories share an id as on GitHub).
func (c fakeCommit) index() (entries map[string][]treeEntry, ids map[string]string) {
	children := map[string]map[string]bool{}
	for file := range c.files {
		for p := file; p != "."; p = path.Dir(p) {
			dir := path.Dir(p)
			if dir == "." {
				dir = ""
			}
			if children[dir] == nil {
				children[dir] = map[string]bool{}
			}
			children[dir][path.Base(p)] = true
		}
	}
	entries, ids = map[string][]treeEntry{}, map[string]string{}
	var build func(dir string) string
	build = func(dir string) string {
		names := make([]string, 0, len(children[dir]))
		for name := range children[dir] {
			names = append(names, name)
		}
		sort.Strings(names)
		var list []treeEntry
		var lines string
		for _, name := range names {
			full := path.Join(dir, name)
			var e treeEntry
			if body, ok := c.files[full]; ok {
				e = treeEntry{Path: name, Mode: "100644", Type: "blob", SHA: blobSHA(body), Size: int64(len(body))}
				if mode, ok := c.modes[full]; ok {
					e.Mode = mode
				}
				if size, ok := c.sizes[full]; ok {
					e.Size = size
				}
			} else {
				e = treeEntry{Path: name, Mode: "040000", Type: "tree", SHA: build(full)}
			}
			list = append(list, e)
			lines += e.Mode + " " + e.Type + " " + e.SHA + " " + name + "\n"
		}
		sum := sha1.Sum([]byte(lines))
		entries[dir], ids[dir] = list, hex.EncodeToString(sum[:])
		return ids[dir]
	}
	build("")
	return entries, ids
}

func (r fakeRepo) treeSHA(commit, dir string) string {
	_, ids := r.commits[commit].index()
	return ids[dir]
}

func (r fakeRepo) commitOf(ref string) (string, bool) {
	if c, ok := r.refs[ref]; ok {
		return c, true
	}
	if ref == "main" && len(r.commits) == 1 {
		for sha := range r.commits {
			return sha, true
		}
	}
	return "", false
}

type fakeServer struct {
	*httptest.Server
	mu  sync.Mutex
	log []string
}

func (s *fakeServer) base() string { return s.URL + "/api/v3" }

func (s *fakeServer) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.log...)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// fakeGitHub serves repo under /api/v3 with the endpoints the connector uses
// and records every request path.
func fakeGitHub(t *testing.T, repo fakeRepo) *fakeServer {
	t.Helper()
	allowLocalGitHubServer(t)
	s := &fakeServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.log = append(s.log, r.URL.RequestURI())
		s.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+goodToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/api/v3")
		for _, alias := range repo.aliases {
			if p == "/repos/"+alias || strings.HasPrefix(p, "/repos/"+alias+"/") {
				p = "/repos/o/r" + strings.TrimPrefix(p, "/repos/"+alias)
			}
		}
		switch {
		case p == "/user":
			_, _ = w.Write([]byte(`{"id":1}`))
		case p == "/user/repos":
			_, _ = w.Write([]byte("[" + repositoryJSON + "]"))
		case p == "/repos/o/r":
			_, _ = w.Write([]byte(repositoryJSON))
		case strings.HasPrefix(p, "/repos/o/r/commits/"):
			sha, ok := repo.commitOf(strings.TrimPrefix(p, "/repos/o/r/commits/"))
			if !ok {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, map[string]string{"sha": sha})
		case strings.HasPrefix(p, "/repos/o/r/git/trees/"):
			repo.serveTree(w, r, strings.TrimPrefix(p, "/repos/o/r/git/trees/"))
		case strings.HasPrefix(p, "/repos/o/r/git/blobs/"):
			sha := strings.TrimPrefix(p, "/repos/o/r/git/blobs/")
			for _, c := range repo.commits {
				for _, body := range c.files {
					if blobSHA(body) == sha {
						_, _ = w.Write([]byte(body))
						return
					}
				}
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

// serveTree resolves id as a ref name, a commit or a tree id, then answers the
// recursive listing of the whole commit or the direct entries of that tree.
func (r fakeRepo) serveTree(w http.ResponseWriter, req *http.Request, id string) {
	if r.missing[id] {
		http.NotFound(w, req)
		return
	}
	if c, ok := r.commitOf(id); ok {
		id = c
	}
	if c, ok := r.commits[id]; ok {
		entries, ids := c.index()
		if req.URL.Query().Get("recursive") != "1" {
			writeJSON(w, tree{SHA: ids[""], Entries: entries[""]})
			return
		}
		if r.truncate {
			writeJSON(w, tree{SHA: ids[""], Truncated: true, Entries: []treeEntry{}})
			return
		}
		var all []treeEntry
		for dir, list := range entries {
			for _, e := range list {
				e.Path = path.Join(dir, e.Path)
				all = append(all, e)
			}
		}
		writeJSON(w, tree{SHA: ids[""], Entries: all})
		return
	}
	for _, c := range r.commits {
		entries, ids := c.index()
		for dir, sha := range ids {
			if sha == id {
				writeJSON(w, tree{SHA: sha, Entries: entries[dir]})
				return
			}
		}
	}
	http.NotFound(w, req)
}

type gitHubStreamRecorder struct {
	items       []types.FetchedItem
	checkpoints []*types.SyncCursor
	// failAt makes the n-th Emit fail (1-based); zero never fails.
	failAt int
}

func (h *gitHubStreamRecorder) Emit(_ context.Context, item types.FetchedItem) error {
	if h.failAt > 0 && len(h.items)+1 == h.failAt {
		return errors.New("ingest failed")
	}
	h.items = append(h.items, item)
	return nil
}

func (h *gitHubStreamRecorder) Checkpoint(_ context.Context, cursor *types.SyncCursor) error {
	h.checkpoints = append(h.checkpoints, cursor)
	return nil
}

func selection(repository, ref string, paths ...string) map[string]interface{} {
	list := make([]interface{}, 0, len(paths))
	for _, p := range paths {
		list = append(list, p)
	}
	return map[string]interface{}{"repository": repository, "ref": ref, "paths": list}
}

func sourceConfig(base, token string, repositories ...map[string]interface{}) *types.DataSourceConfig {
	list := make([]interface{}, 0, len(repositories))
	for _, r := range repositories {
		list = append(list, r)
	}
	return &types.DataSourceConfig{
		Credentials: map[string]interface{}{"base_url": base, "access_token": token},
		Settings:    map[string]interface{}{"repositories": list},
	}
}

func oldCursor(repositories map[string]repositoryCursor) *types.SyncCursor {
	return &types.SyncCursor{ConnectorCursor: map[string]interface{}{"repositories": repositories}}
}

func repositories(t *testing.T, cursor *types.SyncCursor) map[string]repositoryCursor {
	t.Helper()
	require.NotNil(t, cursor)
	got, ok := cursor.ConnectorCursor["repositories"].(map[string]repositoryCursor)
	require.True(t, ok, "connector_cursor.repositories = %#v", cursor.ConnectorCursor["repositories"])
	return got
}

// summary renders emitted items as "-<id>" for a tombstone and "+<id>=<body>"
// otherwise, with the "github:<base>:" prefix removed from the external ID.
func summary(base string, items []types.FetchedItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimPrefix(item.ExternalID, "github:"+base+":")
		if item.IsDeleted {
			out = append(out, "-"+id)
		} else {
			out = append(out, "+"+id+"="+string(item.Content))
		}
	}
	return out
}

func stream(
	t *testing.T, server *fakeServer, old *types.SyncCursor, h *gitHubStreamRecorder, repos ...map[string]interface{},
) (*types.SyncCursor, error) {
	t.Helper()
	return NewConnector().FetchStream(context.Background(), sourceConfig(server.base(), goodToken, repos...), old, h)
}

func TestValidateUsesDataSourceCredentials(t *testing.T) {
	server := fakeGitHub(t, fakeRepo{commits: map[string]fakeCommit{"c1": {}}})
	ctx := context.Background()
	cfg := sourceConfig(server.base(), goodToken, selection("o/r", "", "docs"))
	require.NoError(t, NewConnector().Validate(ctx, cfg))
	err := NewConnector().Validate(ctx, sourceConfig(server.base(), "wrong", selection("o/r", "", "docs")))
	require.ErrorIs(t, err, datasource.ErrInvalidCredentials)
}

func TestFetchStreamMissingRepositoryIsResourceNotFound(t *testing.T) {
	server := fakeGitHub(t, fakeRepo{commits: map[string]fakeCommit{"c1": {}}})
	_, err := stream(t, server, nil, &gitHubStreamRecorder{}, selection("o/gone", "", "docs"))
	require.ErrorIs(t, err, datasource.ErrResourceNotFound)
	require.Equal(t, []string{"/api/v3/repos/o/gone"}, server.requests())
}

func TestListResourcesListsRepositoriesThenDirectories(t *testing.T) {
	server := fakeGitHub(t, fakeRepo{commits: map[string]fakeCommit{"c1": {files: map[string]string{
		"docs/a.md": "a", "docs/sub/b.md": "b", "src/c.md": "c", "README.md": "r",
	}}}})
	cfg := sourceConfig(server.base(), goodToken, selection("o/r", "", "docs"))
	ctx := context.Background()

	got, err := NewConnector().ListResources(ctx, cfg, "")
	require.NoError(t, err)
	require.Equal(t, []types.Resource{
		{ExternalID: "o/r", Name: "o/R", Type: "repository", URL: "https://github.com/o/R", HasChildren: true},
	}, got)

	got, err = NewConnector().ListResources(ctx, cfg, "o/r")
	require.NoError(t, err)
	require.Equal(t, []types.Resource{
		{ExternalID: "o/r:docs", Name: "docs", Type: "directory", ParentID: "o/r", HasChildren: true},
		{ExternalID: "o/r:src", Name: "src", Type: "directory", ParentID: "o/r", HasChildren: true},
	}, got)

	got, err = NewConnector().ListResources(ctx, cfg, "o/r:docs")
	require.NoError(t, err)
	require.Equal(t, []types.Resource{
		{ExternalID: "o/r:docs/sub", Name: "sub", Type: "directory", ParentID: "o/r:docs", HasChildren: true},
	}, got)
}

func TestFetchStreamFullSyncEmitsSupportedRegularBlobsInScope(t *testing.T) {
	server := fakeGitHub(t, fakeRepo{commits: map[string]fakeCommit{"c1": {
		files: map[string]string{
			"docs/a.md": "hello", "docs/b.go": "package b", "docs/link.md": "../a.md",
			"src/c.md": "c", "README.md": "r",
		},
		modes: map[string]string{"docs/link.md": "120000"},
	}}})
	h := &gitHubStreamRecorder{}

	next, err := stream(t, server, nil, h, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	base := server.base()
	require.Equal(t, []types.FetchedItem{{
		ExternalID:       "github:" + base + ":o/r:main:docs/a.md",
		Title:            "o/R/docs/a.md",
		FileName:         "R-main/docs/a.md",
		Content:          []byte("hello"),
		ContentType:      "text/plain",
		SourceResourceID: "o/r",
		Metadata: map[string]string{
			"channel":              "github",
			"source_type":          "github",
			"github_repository":    "o/R",
			"github_repository_id": "7",
			"github_ref":           "main",
			"github_path":          "docs/a.md",
			"github_url":           "https://github.com/o/R/blob/main/docs/a.md",
		},
	}}, h.items)
	require.True(t, next.CompleteListing)
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "main", Paths: []string{"docs"}}},
		repositories(t, next))
	require.Len(t, h.checkpoints, 1)
}

func TestFetchStreamIncrementalEmitsOnlyChangedAndDeleted(t *testing.T) {
	server := fakeGitHub(t, fakeRepo{
		commits: map[string]fakeCommit{
			"c1": {files: map[string]string{"docs/a.md": "v1", "docs/b.md": "x"}},
			"c2": {files: map[string]string{"docs/a.md": "v2", "docs/c.md": "x"}},
		},
		refs: map[string]string{"main": "c2"},
	})
	h := &gitHubStreamRecorder{}
	old := oldCursor(map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "main", Paths: []string{"docs"}}})

	next, err := stream(t, server, old, h, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"-o/r:main:docs/b.md", "+o/r:main:docs/a.md=v2", "+o/r:main:docs/c.md=x"},
		summary(server.base(), h.items))
	require.False(t, next.CompleteListing)
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c2", Ref: "main", Paths: []string{"docs"}}},
		repositories(t, next))
}

func TestFetchStreamForcePushEmitsSnapshotChanges(t *testing.T) {
	server := fakeGitHub(t, fakeRepo{
		commits: map[string]fakeCommit{
			"c1": {files: map[string]string{"docs/a.md": "v1", "docs/only-old.md": "o", "docs/same.md": "s"}},
			"c2": {files: map[string]string{"docs/a.md": "v0", "docs/same.md": "s"}},
		},
		refs: map[string]string{"main": "c2"},
	})
	h := &gitHubStreamRecorder{}
	old := oldCursor(map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "main", Paths: []string{"docs"}}})

	_, err := stream(t, server, old, h, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"-o/r:main:docs/only-old.md", "+o/r:main:docs/a.md=v0"}, summary(server.base(), h.items))
}

func TestFetchStreamMissingPreviousHeadFallsBackToFullEmit(t *testing.T) {
	server := fakeGitHub(t, fakeRepo{
		commits: map[string]fakeCommit{"c2": {files: map[string]string{"docs/a.md": "a", "docs/b.md": "b"}}},
		refs:    map[string]string{"main": "c2"},
		missing: map[string]bool{"gone": true},
	})
	h := &gitHubStreamRecorder{}
	old := oldCursor(map[string]repositoryCursor{"o/r": {Head: "gone", Ref: "main", Paths: []string{"docs"}}})

	next, err := stream(t, server, old, h, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"+o/r:main:docs/a.md=a", "+o/r:main:docs/b.md=b"}, summary(server.base(), h.items))
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c2", Ref: "main", Paths: []string{"docs"}}},
		repositories(t, next))
}

func TestFetchStreamUnchangedHeadEmitsNothing(t *testing.T) {
	server := fakeGitHub(t, fakeRepo{commits: map[string]fakeCommit{
		"c1": {files: map[string]string{"docs/a.md": "a"}},
	}})
	h := &gitHubStreamRecorder{}
	old := oldCursor(map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "main", Paths: []string{"docs"}}})

	next, err := stream(t, server, old, h, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	require.Empty(t, h.items)
	require.Len(t, h.checkpoints, 1)
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "main", Paths: []string{"docs"}}},
		repositories(t, next))
}

func TestFetchStreamSelectionChangeTombstonesOldRefAndEmitsNewRoots(t *testing.T) {
	repo := fakeRepo{
		commits: map[string]fakeCommit{"c1": {files: map[string]string{
			"docs/a.md": "a", "docs/b.md": "b", "guides/g.md": "g",
		}}},
		refs: map[string]string{"main": "c1", "release": "c1"},
	}
	old := oldCursor(map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "main", Paths: []string{"docs"}}})

	server := fakeGitHub(t, repo)
	h := &gitHubStreamRecorder{}
	next, err := stream(t, server, old, h, selection("o/r", "release", "docs", "guides"))
	require.NoError(t, err)
	require.Equal(t, []string{
		"-o/r:main:docs/a.md", "-o/r:main:docs/b.md",
		"+o/r:release:docs/a.md=a", "+o/r:release:docs/b.md=b", "+o/r:release:guides/g.md=g",
	}, summary(server.base(), h.items))
	require.Equal(t,
		map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "release", Paths: []string{"docs", "guides"}}},
		repositories(t, next))

	server = fakeGitHub(t, repo)
	h = &gitHubStreamRecorder{}
	_, err = stream(t, server, old, h, selection("o/r", "main", "docs", "guides"))
	require.NoError(t, err)
	require.Equal(t, []string{"+o/r:main:guides/g.md=g"}, summary(server.base(), h.items))
}

func TestFetchStreamResumesInsideRepositoryFromPendingCursor(t *testing.T) {
	var all []string
	for i := 1; i <= 120; i++ {
		all = append(all, fmt.Sprintf("+o/r:main:docs/f%03d.md=%d", i, i))
	}
	server := fakeGitHub(t, fakeRepo{commits: map[string]fakeCommit{"c1": {files: numberedDocs(120)}}})

	h := &gitHubStreamRecorder{failAt: 101}
	next, err := stream(t, server, nil, h, selection("o/r", "main", "docs"))
	require.EqualError(t, err, "ingest failed")
	require.Nil(t, next)
	require.Equal(t, all[:100], summary(server.base(), h.items))
	require.Len(t, h.checkpoints, 2)
	require.Equal(t, map[string]repositoryCursor{
		"o/r": {Pending: &pendingCursor{Head: "c1", Ref: "main", Paths: []string{"docs"}, Done: 100}},
	}, repositories(t, h.checkpoints[1]))

	resumed := &gitHubStreamRecorder{}
	next, err = stream(t, server, h.checkpoints[1], resumed, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, all[100:], summary(server.base(), resumed.items))
	require.False(t, next.CompleteListing)
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "main", Paths: []string{"docs"}}},
		repositories(t, next))
}

// numberedDocs returns docs/f001.md..docs/f<n>.md with their index as the body.
func numberedDocs(n int) map[string]string {
	files := map[string]string{}
	for i := 1; i <= n; i++ {
		files[fmt.Sprintf("docs/f%03d.md", i)] = fmt.Sprint(i)
	}
	return files
}

// interruptedStream runs a sync from old that fails at the failAt-th emit and
// returns the recorder and the last checkpoint it took.
func interruptedStream(
	t *testing.T, server *fakeServer, old *types.SyncCursor, failAt int, repos ...map[string]interface{},
) (*gitHubStreamRecorder, *types.SyncCursor) {
	t.Helper()
	h := &gitHubStreamRecorder{failAt: failAt}
	next, err := stream(t, server, old, h, repos...)
	require.EqualError(t, err, "ingest failed")
	require.Nil(t, next)
	require.NotEmpty(t, h.checkpoints)
	return h, h.checkpoints[len(h.checkpoints)-1]
}

// emittedBodies maps each path to the bodies emitted for it, in order.
func emittedBodies(items ...[]types.FetchedItem) map[string][]string {
	out := map[string][]string{}
	for _, list := range items {
		for _, item := range list {
			out[item.Metadata["github_path"]] = append(out[item.Metadata["github_path"]], string(item.Content))
		}
	}
	return out
}

// Three runs under one ref: c0 is the completed baseline, c1 changes every
// guide and is cut off at 100, c2 changes g001..g050 again, reverts g060 to its
// c0 body and adds g121, and the run against it is cut off at 50. Its diff is
// against the c0 baseline overlaid with the c1 prefix (g001..g050 then g060,
// g101..g121), so the retry must skip 50 entries of that same list, not of the
// c0-to-c2 diff, which does not contain the revert of g060 at all.
func TestFetchStreamResumeAfterOverlayRunContinuesTheOverlayDiff(t *testing.T) {
	c0, c1, c2 := map[string]string{}, map[string]string{}, map[string]string{}
	for _, files := range []map[string]string{c0, c1, c2} {
		files["docs/a.md"], files["docs/b.md"] = "a", "b"
	}
	for i := 1; i <= 120; i++ {
		name := fmt.Sprintf("docs/guides/g%03d.md", i)
		c0[name], c1[name], c2[name] = fmt.Sprint(i), fmt.Sprintf("%d.1", i), fmt.Sprintf("%d.1", i)
		if i <= 50 {
			c2[name] = fmt.Sprintf("%d.2", i)
		}
	}
	c2["docs/guides/g060.md"] = c0["docs/guides/g060.md"]
	c2["docs/guides/g121.md"] = "121.2"
	repo := fakeRepo{
		commits: map[string]fakeCommit{"c0": {files: c0}, "c1": {files: c1}, "c2": {files: c2}},
		refs:    map[string]string{"main": "c1"},
	}
	server := fakeGitHub(t, repo)
	old := oldCursor(map[string]repositoryCursor{"o/r": {Head: "c0", Ref: "main", Paths: []string{"docs"}}})
	docs := []string{"docs"}

	first, pending := interruptedStream(t, server, old, 101, selection("o/r", "main", "docs"))
	var want []string
	for i := 1; i <= 100; i++ {
		want = append(want, fmt.Sprintf("+o/r:main:docs/guides/g%03d.md=%d.1", i, i))
	}
	require.Equal(t, want, summary(server.base(), first.items))
	c1Pending := &pendingCursor{Head: "c1", Ref: "main", Paths: docs, Done: 100}
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c0", Ref: "main", Paths: docs, Pending: c1Pending}},
		repositories(t, pending))

	repo.refs["main"] = "c2"
	second, pending := interruptedStream(t, server, pending, 51, selection("o/r", "main", "docs"))
	want = nil
	for i := 1; i <= 50; i++ {
		want = append(want, fmt.Sprintf("+o/r:main:docs/guides/g%03d.md=%d.2", i, i))
	}
	require.Equal(t, want, summary(server.base(), second.items))
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c0", Ref: "main", Paths: docs, Pending: &pendingCursor{
		Head: "c2", Ref: "main", Paths: docs, Done: 50, Base: c1Pending,
	}}}, repositories(t, pending))

	third := &gitHubStreamRecorder{}
	next, err := stream(t, server, pending, third, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	want = []string{"+o/r:main:docs/guides/g060.md=60"}
	for i := 101; i <= 120; i++ {
		want = append(want, fmt.Sprintf("+o/r:main:docs/guides/g%03d.md=%d.1", i, i))
	}
	want = append(want, "+o/r:main:docs/guides/g121.md=121.2")
	require.Equal(t, want, summary(server.base(), third.items))
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c2", Ref: "main", Paths: docs}}, repositories(t, next))

	// Across the three runs every guide was emitted and the last body emitted
	// for each is its c2 body. The paths emitted twice are exactly those whose
	// body changed after the first run indexed them: g001..g050 (c1 body, then
	// c2 body) and g060 (c1 body, then the c0 body c2 reverted to).
	bodies := emittedBodies(first.items, second.items, third.items)
	require.Len(t, bodies, 121)
	for i := 1; i <= 121; i++ {
		name := fmt.Sprintf("docs/guides/g%03d.md", i)
		got := bodies[name]
		require.Equal(t, c2[name], got[len(got)-1], name)
		switch {
		case i <= 50:
			require.Equal(t, []string{fmt.Sprintf("%d.1", i), fmt.Sprintf("%d.2", i)}, got, name)
		case i == 60:
			require.Equal(t, []string{"60.1", "60"}, got, name)
		default:
			require.Len(t, got, 1, name)
		}
	}
}

// A first sync of 120 files cut off at 100, then the head moves (f001..f050
// changed, f121 added) and the run against it is cut off at 50. Its diff was
// against the c1 prefix (f001..f050 then f101..f121), so the retry emits the
// 21 entries after the first 50 and no tombstones.
func TestFetchStreamFreshRepositoryResumeAfterOverlayRun(t *testing.T) {
	c2 := numberedDocs(121)
	for i := 1; i <= 50; i++ {
		c2[fmt.Sprintf("docs/f%03d.md", i)] = fmt.Sprintf("%d.2", i)
	}
	repo := fakeRepo{
		commits: map[string]fakeCommit{"c1": {files: numberedDocs(120)}, "c2": {files: c2}},
		refs:    map[string]string{"main": "c1"},
	}
	server := fakeGitHub(t, repo)
	docs := []string{"docs"}

	_, pending := interruptedStream(t, server, nil, 101, selection("o/r", "main", "docs"))
	c1Pending := &pendingCursor{Head: "c1", Ref: "main", Paths: docs, Done: 100}
	require.Equal(t, map[string]repositoryCursor{"o/r": {Pending: c1Pending}}, repositories(t, pending))

	repo.refs["main"] = "c2"
	second, pending := interruptedStream(t, server, pending, 51, selection("o/r", "main", "docs"))
	var want []string
	for i := 1; i <= 50; i++ {
		want = append(want, fmt.Sprintf("+o/r:main:docs/f%03d.md=%d.2", i, i))
	}
	require.Equal(t, want, summary(server.base(), second.items))
	require.Equal(t, map[string]repositoryCursor{"o/r": {Pending: &pendingCursor{
		Head: "c2", Ref: "main", Paths: docs, Done: 50, Base: c1Pending,
	}}}, repositories(t, pending))

	resumed := &gitHubStreamRecorder{}
	next, err := stream(t, server, pending, resumed, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	want = nil
	for i := 101; i <= 120; i++ {
		want = append(want, fmt.Sprintf("+o/r:main:docs/f%03d.md=%d", i, i))
	}
	want = append(want, "+o/r:main:docs/f121.md=121")
	require.Equal(t, want, summary(server.base(), resumed.items))
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c2", Ref: "main", Paths: docs}}, repositories(t, next))
}

func TestFetchStreamHeadMovedAfterPendingCheckpointContinuesFromPendingPrefix(t *testing.T) {
	c2 := numberedDocs(121)
	c2["docs/f050.md"] = "changed"
	repo := fakeRepo{
		commits: map[string]fakeCommit{"c1": {files: numberedDocs(120)}, "c2": {files: c2}},
		refs:    map[string]string{"main": "c1"},
	}
	server := fakeGitHub(t, repo)
	h := &gitHubStreamRecorder{failAt: 101}
	_, err := stream(t, server, nil, h, selection("o/r", "main", "docs"))
	require.EqualError(t, err, "ingest failed")
	require.Equal(t, map[string]repositoryCursor{
		"o/r": {Pending: &pendingCursor{Head: "c1", Ref: "main", Paths: []string{"docs"}, Done: 100}},
	}, repositories(t, h.checkpoints[1]))

	repo.refs["main"] = "c2"
	resumed := &gitHubStreamRecorder{}
	next, err := stream(t, server, h.checkpoints[1], resumed, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	want := []string{"+o/r:main:docs/f050.md=changed"}
	for i := 101; i <= 121; i++ {
		want = append(want, fmt.Sprintf("+o/r:main:docs/f%03d.md=%d", i, i))
	}
	require.Equal(t, want, summary(server.base(), resumed.items))
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c2", Ref: "main", Paths: []string{"docs"}}},
		repositories(t, next))
}

func TestFetchStreamRevertAfterPendingCheckpointReplacesThePendingBody(t *testing.T) {
	c0 := map[string]string{"docs/a.md": "old", "docs/b.md": "old"}
	c1 := map[string]string{"docs/a.md": "new", "docs/b.md": "new", "docs/c.md": "c"}
	c2 := map[string]string{"docs/a.md": "old", "docs/b.md": "new"}
	server := fakeGitHub(t, fakeRepo{
		commits: map[string]fakeCommit{"c0": {files: c0}, "c1": {files: c1}, "c2": {files: c2}},
		refs:    map[string]string{"main": "c2"},
	})
	pending := oldCursor(map[string]repositoryCursor{"o/r": {
		Head: "c0", Ref: "main", Paths: []string{"docs"},
		Pending: &pendingCursor{Head: "c1", Ref: "main", Paths: []string{"docs"}, Done: 2},
	}})

	h := &gitHubStreamRecorder{}
	next, err := stream(t, server, pending, h, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"+o/r:main:docs/a.md=old"}, summary(server.base(), h.items))
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c2", Ref: "main", Paths: []string{"docs"}}},
		repositories(t, next))
}

// interruptedGuidesSync syncs a cursor of {c1, main, [docs]} under ref with
// [docs, guides] (2 docs files, 120 guides files) and fails at the 101st emit,
// returning the server, the pending checkpoint and the guides file names.
func interruptedGuidesSync(t *testing.T, ref string) (*fakeServer, *types.SyncCursor, []string) {
	t.Helper()
	files := map[string]string{"docs/a.md": "a", "docs/b.md": "b"}
	var guides []string
	for i := 1; i <= 120; i++ {
		name := fmt.Sprintf("guides/g%03d.md", i)
		files[name] = fmt.Sprint(i)
		guides = append(guides, name)
	}
	server := fakeGitHub(t, fakeRepo{
		commits: map[string]fakeCommit{"c1": {files: files}},
		refs:    map[string]string{"main": "c1", "release": "c1"},
	})
	old := oldCursor(map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "main", Paths: []string{"docs"}}})

	h := &gitHubStreamRecorder{failAt: 101}
	_, err := stream(t, server, old, h, selection("o/r", ref, "docs", "guides"))
	require.EqualError(t, err, "ingest failed")
	require.Equal(t, map[string]repositoryCursor{"o/r": {
		Head: "c1", Ref: "main", Paths: []string{"docs"},
		Pending: &pendingCursor{Head: "c1", Ref: ref, Paths: []string{"docs", "guides"}, Done: 100},
	}}, repositories(t, h.checkpoints[1]))
	return server, h.checkpoints[1], guides
}

// A ref change interrupted after 100 entries (2 main deletions, then docs/a,
// docs/b and 96 guides under release) and retried with fewer paths: the
// pending guides are tombstoned, the docs files are not, since the new diff
// re-emits them under release.
func TestFetchStreamPathsShrinkAfterPendingRefChangeKeepsReAddedItems(t *testing.T) {
	server, pending, guides := interruptedGuidesSync(t, "release")

	resumed := &gitHubStreamRecorder{}
	next, err := stream(t, server, pending, resumed, selection("o/r", "release", "docs"))
	require.NoError(t, err)
	var want []string
	for _, name := range guides[:96] {
		want = append(want, "-o/r:release:"+name)
	}
	want = append(want,
		"-o/r:main:docs/a.md", "-o/r:main:docs/b.md", "+o/r:release:docs/a.md=a", "+o/r:release:docs/b.md=b")
	require.Equal(t, want, summary(server.base(), resumed.items))
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "release", Paths: []string{"docs"}}},
		repositories(t, next))
}

func TestFetchStreamSelectionChangeAfterPendingCheckpointRestartsTheDiff(t *testing.T) {
	server, pending, guides := interruptedGuidesSync(t, "main")

	resumed := &gitHubStreamRecorder{}
	next, err := stream(t, server, pending, resumed, selection("o/r", "release", "docs", "guides"))
	require.NoError(t, err)
	var want []string
	for _, name := range guides[:100] {
		want = append(want, "-o/r:main:"+name)
	}
	want = append(want,
		"-o/r:main:docs/a.md", "-o/r:main:docs/b.md", "+o/r:release:docs/a.md=a", "+o/r:release:docs/b.md=b")
	for i, name := range guides {
		want = append(want, fmt.Sprintf("+o/r:release:%s=%d", name, i+1))
	}
	require.Equal(t, want, summary(server.base(), resumed.items))
	require.Equal(t,
		map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "release", Paths: []string{"docs", "guides"}}},
		repositories(t, next))
}

func TestFetchStreamPathsShrinkAfterPendingCheckpointTombstonesPendingItems(t *testing.T) {
	server, pending, guides := interruptedGuidesSync(t, "main")

	resumed := &gitHubStreamRecorder{}
	next, err := stream(t, server, pending, resumed, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	var want []string
	for _, name := range guides[:100] {
		want = append(want, "-o/r:main:"+name)
	}
	require.Equal(t, want, summary(server.base(), resumed.items))
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "main", Paths: []string{"docs"}}},
		repositories(t, next))
}

// o/gone is in the cursor but no longer selected: its indexed files are
// tombstoned under the ref they were synced with, its cursor entry is dropped,
// and o/r, whose head is unchanged, emits nothing.
func TestFetchStreamRemovedRepositoryIsTombstoned(t *testing.T) {
	server := fakeGitHub(t, fakeRepo{
		commits: map[string]fakeCommit{"c1": {files: map[string]string{"docs/a.md": "a", "docs/b.md": "b"}}},
		aliases: []string{"o/gone"},
	})
	docs := []string{"docs"}
	old := oldCursor(map[string]repositoryCursor{
		"o/r":    {Head: "c1", Ref: "main", Paths: docs},
		"o/gone": {Head: "c1", Ref: "main", Paths: docs},
	})

	h := &gitHubStreamRecorder{}
	next, err := stream(t, server, old, h, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"-o/gone:main:docs/a.md", "-o/gone:main:docs/b.md"}, summary(server.base(), h.items))
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "main", Paths: docs}}, repositories(t, next))
	require.Len(t, h.checkpoints, 2)
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "main", Paths: docs}},
		repositories(t, h.checkpoints[1]))
}

// The removed repository also has a pending checkpoint under another ref whose
// first four entries were the two main deletions and docs/a.md and docs/b.md
// under release: both groups are tombstoned, the baseline under main and the
// pending prefix under release.
func TestFetchStreamRemovedRepositoryWithPendingIsTombstonedUnderBothRefs(t *testing.T) {
	server := fakeGitHub(t, fakeRepo{
		commits: map[string]fakeCommit{"c1": {files: map[string]string{
			"docs/a.md": "a", "docs/b.md": "b", "guides/g.md": "g",
		}}},
		aliases: []string{"o/gone"},
	})
	docs := []string{"docs"}
	old := oldCursor(map[string]repositoryCursor{
		"o/r": {Head: "c1", Ref: "main", Paths: docs},
		"o/gone": {
			Head: "c1", Ref: "main", Paths: docs,
			Pending: &pendingCursor{Head: "c1", Ref: "release", Paths: []string{"docs", "guides"}, Done: 4},
		},
	})

	h := &gitHubStreamRecorder{}
	next, err := stream(t, server, old, h, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{
		"-o/gone:main:docs/a.md", "-o/gone:main:docs/b.md",
		"-o/gone:release:docs/a.md", "-o/gone:release:docs/b.md",
	}, summary(server.base(), h.items))
	require.Equal(t, map[string]repositoryCursor{"o/r": {Head: "c1", Ref: "main", Paths: docs}}, repositories(t, next))
}

func TestOversizedBlobBecomesErrorItem(t *testing.T) {
	server := fakeGitHub(t, fakeRepo{commits: map[string]fakeCommit{"c1": {
		files: map[string]string{"docs/big.md": "x"},
		sizes: map[string]int64{"docs/big.md": 101 << 20},
	}}})
	h := &gitHubStreamRecorder{}

	_, err := stream(t, server, nil, h, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []types.FetchedItem{{
		ExternalID: "github:" + server.base() + ":o/r:main:docs/big.md",
		Title:      "o/R/docs/big.md",
		Metadata: map[string]string{
			"channel":           "github",
			"source_type":       "github",
			"github_path":       "docs/big.md",
			"error":             "The GitHub file exceeds the 100 MB API limit",
			"error_reason_code": "github_file_too_large",
			"error_reason":      "The GitHub file exceeds the 100 MB API limit",
		},
	}}, h.items)
	for _, r := range server.requests() {
		require.NotContains(t, r, "/git/blobs/")
	}
}

func TestEmptyBlobSyncsItsPath(t *testing.T) {
	server := fakeGitHub(t, fakeRepo{commits: map[string]fakeCommit{
		"c1": {files: map[string]string{"docs/empty.md": ""}},
	}})
	h := &gitHubStreamRecorder{}

	_, err := stream(t, server, nil, h, selection("o/r", "main", "docs"))
	require.NoError(t, err)
	require.Len(t, h.items, 1)
	require.Equal(t, "# o/R/docs/empty.md\n", string(h.items[0].Content))
}
