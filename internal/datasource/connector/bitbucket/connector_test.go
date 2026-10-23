package bitbucket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/utils"
	"github.com/stretchr/testify/require"
)

const goodToken = "per-source-token"

const diffstatQuery = "?topic=false&renames=false&pagelen=100&fields=values.status,values.old.path,values.new.path,next"

// fakeRepo is the content of the fake's one repository, ws/R (key ws/r).
type fakeRepo struct {
	// commits maps a commit hash to its files, path to body.
	commits map[string]map[string]string
	// attributes and sizes override a listing entry's attributes and size per commit and path.
	attributes map[string]map[string][]string
	sizes      map[string]map[string]int64
	// refs maps a branch name to a commit; "main" defaults to the only commit. tags maps a tag name.
	refs map[string]string
	tags map[string]string
	// timeout answers 555: a commit hash key makes its diffstats and its listings at listingDepth
	// time out; a "<commit>:<dir>" key makes the listing of that directory time out at any depth.
	timeout map[string]bool
	// lfs lists paths whose raw fetch answers a 301 to a media host.
	lfs map[string]bool
	// noMainBranch serves the repository with a null mainbranch.
	noMainBranch bool
	// aliases lists further repository keys served with the same content as ws/r.
	aliases []string
	// status makes every request under a repository key answer that status.
	status map[string]int
	// diffstat is appended to every diffstat answer, for statuses the fake does not derive.
	diffstat []any
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

func (r fakeRepo) repositoryJSON(fullName string) map[string]any {
	var main any
	if !r.noMainBranch {
		main = map[string]string{"name": "main"}
	}
	return map[string]any{
		"uuid": "{u1}", "full_name": fullName, "mainbranch": main,
		"links": map[string]any{"html": map[string]string{"href": "https://bitbucket.org/" + fullName}},
	}
}

type fakeServer struct {
	*httptest.Server
	mu  sync.Mutex
	log []string
}

func (s *fakeServer) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.log...)
}

// rawRequests lists the file fetches, the requests without a query string.
func (s *fakeServer) rawRequests() []string {
	var out []string
	for _, r := range s.requests() {
		if strings.Contains(r, "/src/") && !strings.Contains(r, "?") {
			out = append(out, r)
		}
	}
	return out
}

func (s *fakeServer) requestsMatching(substr string) []string {
	var out []string
	for _, r := range s.requests() {
		if strings.Contains(r, substr) {
			out = append(out, r)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// page writes one page of values, honouring pagelen and page, with a next link
// to the following page.
func page(w http.ResponseWriter, req *http.Request, values []any) {
	q := req.URL.Query()
	size, _ := strconv.Atoi(q.Get("pagelen"))
	if size <= 0 {
		size = 10
	}
	n, _ := strconv.Atoi(q.Get("page"))
	if n <= 0 {
		n = 1
	}
	start := min((n-1)*size, len(values))
	end := min(start+size, len(values))
	body := map[string]any{"values": values[start:end]}
	if end < len(values) {
		q.Set("page", strconv.Itoa(n+1))
		next := *req.URL
		next.Scheme, next.Host, next.RawQuery = "http", req.Host, q.Encode()
		body["next"] = next.String()
	}
	writeJSON(w, body)
}

// fakeBitbucket serves repo at the server root with the endpoints the
// connector uses and records every request path.
func fakeBitbucket(t *testing.T, repo fakeRepo) *fakeServer {
	t.Helper()
	allowLocalServer(t)
	s := &fakeServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.log = append(s.log, r.URL.RequestURI())
		s.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+goodToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		p := r.URL.EscapedPath()
		for key, status := range repo.status {
			if p == "/repositories/"+key || strings.HasPrefix(p, "/repositories/"+key+"/") {
				if status == http.StatusTooManyRequests {
					w.Header().Set("x-ratelimit-reset", "3600")
				}
				w.WriteHeader(status)
				return
			}
		}
		for _, alias := range repo.aliases {
			if p == "/repositories/"+alias || strings.HasPrefix(p, "/repositories/"+alias+"/") {
				p = "/repositories/ws/r" + strings.TrimPrefix(p, "/repositories/"+alias)
			}
		}
		switch {
		case p == "/user":
			writeJSON(w, map[string]string{"uuid": "{me}"})
		case p == "/user/workspaces":
			page(w, r, []any{map[string]string{"slug": "ws"}, map[string]string{"slug": "other"}})
		case p == "/repositories/ws":
			page(w, r, []any{repo.repositoryJSON("ws/R")})
		case p == "/repositories/other":
			page(w, r, []any{repo.repositoryJSON("other/Lib")})
		case p == "/repositories/ws/r":
			writeJSON(w, repo.repositoryJSON("ws/R"))
		case strings.HasPrefix(p, "/repositories/ws/r/refs/branches/"):
			serveRef(w, repo.commitOf, strings.TrimPrefix(p, "/repositories/ws/r/refs/branches/"))
		case strings.HasPrefix(p, "/repositories/ws/r/refs/tags/"):
			serveRef(w, func(name string) (string, bool) { c, ok := repo.tags[name]; return c, ok },
				strings.TrimPrefix(p, "/repositories/ws/r/refs/tags/"))
		case strings.HasPrefix(p, "/repositories/ws/r/commit/"):
			hash := strings.TrimPrefix(p, "/repositories/ws/r/commit/")
			if _, ok := repo.commits[hash]; !ok {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, map[string]string{"hash": hash})
		case strings.HasPrefix(p, "/repositories/ws/r/src/"):
			rest := strings.TrimPrefix(p, "/repositories/ws/r/src/")
			commit, rawPath, _ := strings.Cut(rest, "/")
			filePath, _ := url.PathUnescape(rawPath)
			if rawPath == "" || strings.HasSuffix(rawPath, "/") {
				repo.serveListing(w, r, commit, strings.TrimSuffix(filePath, "/"))
			} else {
				repo.serveRaw(w, r, commit, filePath)
			}
		case strings.HasPrefix(p, "/repositories/ws/r/diffstat/"):
			newHead, oldHead, _ := strings.Cut(strings.TrimPrefix(p, "/repositories/ws/r/diffstat/"), "..")
			repo.serveDiffstat(w, r, newHead, oldHead)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func serveRef(w http.ResponseWriter, lookup func(string) (string, bool), name string) {
	name, _ = url.PathUnescape(name)
	commit, ok := lookup(name)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"target": map[string]string{"hash": commit}})
}

type fakeEntry struct {
	depth int
	value map[string]any
}

func (r fakeRepo) serveListing(w http.ResponseWriter, req *http.Request, commit, dir string) {
	files, ok := r.commits[commit]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	maxDepth, _ := strconv.Atoi(req.URL.Query().Get("max_depth"))
	if r.timeout[commit+":"+dir] || (r.timeout[commit] && maxDepth == listingDepth) {
		w.WriteHeader(statusTimedOut)
		return
	}
	prefix := ""
	if dir != "" {
		prefix = dir + "/"
	}
	seen := map[string]fakeEntry{}
	for file, body := range files {
		if !strings.HasPrefix(file, prefix) {
			continue
		}
		segments := strings.Split(strings.TrimPrefix(file, prefix), "/")
		for i := 1; i <= len(segments) && i <= maxDepth; i++ {
			p := prefix + strings.Join(segments[:i], "/")
			if i < len(segments) {
				seen[p] = fakeEntry{i, map[string]any{"type": "commit_directory", "path": p}}
				continue
			}
			size := int64(len(body))
			if s, ok := r.sizes[commit][file]; ok {
				size = s
			}
			attributes := []string{}
			if a, ok := r.attributes[commit][file]; ok {
				attributes = a
			}
			seen[p] = fakeEntry{i, map[string]any{
				"type": "commit_file", "path": p, "size": size, "attributes": attributes,
			}}
		}
	}
	if len(seen) == 0 && dir != "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		if seen[paths[i]].depth != seen[paths[j]].depth {
			return seen[paths[i]].depth < seen[paths[j]].depth
		}
		return paths[i] < paths[j]
	})
	values := make([]any, 0, len(paths))
	for _, p := range paths {
		values = append(values, seen[p].value)
	}
	page(w, req, values)
}

func (r fakeRepo) serveRaw(w http.ResponseWriter, req *http.Request, commit, file string) {
	body, ok := r.commits[commit][file]
	if !ok {
		http.NotFound(w, req)
		return
	}
	if r.lfs[file] {
		w.Header().Set("Location", "https://media.example/x")
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}
	_, _ = w.Write([]byte(body))
}

func (r fakeRepo) serveDiffstat(w http.ResponseWriter, req *http.Request, newHead, oldHead string) {
	if r.timeout[newHead] || r.timeout[oldHead] {
		w.WriteHeader(statusTimedOut)
		return
	}
	newFiles, okNew := r.commits[newHead]
	oldFiles, okOld := r.commits[oldHead]
	if !okNew || !okOld {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	paths := map[string]bool{}
	for p := range newFiles {
		paths[p] = true
	}
	for p := range oldFiles {
		paths[p] = true
	}
	var values []any
	for _, p := range slices.Sorted(maps.Keys(paths)) {
		newBody, inNew := newFiles[p]
		oldBody, inOld := oldFiles[p]
		ref := map[string]string{"path": p}
		switch {
		case inNew && !inOld:
			values = append(values, map[string]any{"status": "added", "old": nil, "new": ref})
		case inOld && !inNew:
			values = append(values, map[string]any{"status": "removed", "old": ref, "new": nil})
		case newBody != oldBody:
			values = append(values, map[string]any{"status": "modified", "old": ref, "new": ref})
		}
	}
	page(w, req, append(values, r.diffstat...))
}

type streamRecorder struct {
	items       []types.FetchedItem
	checkpoints []*types.SyncCursor
	// failAt makes the n-th Emit fail (1-based); zero never fails.
	failAt int
	// imports stamps each checkpoint, as the service does for every cursor it persists.
	imports utils.ImportCapabilities
}

func (h *streamRecorder) Emit(_ context.Context, item types.FetchedItem) error {
	if h.failAt > 0 && len(h.items)+1 == h.failAt {
		return errors.New("ingest failed")
	}
	h.items = append(h.items, item)
	return nil
}

func (h *streamRecorder) Checkpoint(_ context.Context, cursor *types.SyncCursor) error {
	stamped := *cursor
	stamped.Imports = &h.imports
	h.checkpoints = append(h.checkpoints, &stamped)
	return nil
}

func selection(repository, ref string, paths ...string) map[string]interface{} {
	list := make([]interface{}, 0, len(paths))
	for _, p := range paths {
		list = append(list, p)
	}
	return map[string]interface{}{"repository": repository, "ref": ref, "paths": list}
}

func sourceConfig(token string, repositories ...map[string]interface{}) *types.DataSourceConfig {
	cfg := &types.DataSourceConfig{Credentials: map[string]interface{}{"access_token": token}}
	if len(repositories) > 0 {
		list := make([]interface{}, 0, len(repositories))
		for _, r := range repositories {
			list = append(list, r)
		}
		cfg.Settings = map[string]interface{}{"repositories": list}
	}
	return cfg
}

func withImports(cfg *types.DataSourceConfig, caps utils.ImportCapabilities) *types.DataSourceConfig {
	cfg.Imports = caps
	return cfg
}

func oldCursor(repositories map[string]repositoryCursor) *types.SyncCursor {
	return &types.SyncCursor{
		ConnectorCursor: map[string]interface{}{"repositories": repositories},
		Imports:         &utils.ImportCapabilities{},
	}
}

func repositories(t *testing.T, cursor *types.SyncCursor) map[string]repositoryCursor {
	t.Helper()
	require.NotNil(t, cursor)
	got, ok := cursor.ConnectorCursor["repositories"].(map[string]repositoryCursor)
	require.True(t, ok, "connector_cursor.repositories = %#v", cursor.ConnectorCursor["repositories"])
	return got
}

// summary renders emitted items as "-<id>" for a tombstone, "!<id>=<code>" for
// a failed item and "+<id>=<body>" otherwise, without the "bitbucket:<base>:" prefix.
func summary(base string, items []types.FetchedItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimPrefix(item.ExternalID, "bitbucket:"+base+":")
		switch {
		case item.IsDeleted:
			out = append(out, "-"+id)
		case item.Metadata["error_reason_code"] != "":
			out = append(out, "!"+id+"="+item.Metadata["error_reason_code"])
		default:
			out = append(out, "+"+id+"="+string(item.Content))
		}
	}
	return out
}

func connector(server *fakeServer) *Connector {
	return &Connector{baseURL: server.URL}
}

func stream(
	t *testing.T, server *fakeServer, old *types.SyncCursor, h *streamRecorder, repos ...map[string]interface{},
) (*types.SyncCursor, error) {
	t.Helper()
	return connector(server).FetchStream(context.Background(), sourceConfig(goodToken, repos...), old, h)
}

// numberedDocs returns docs/f001.md..docs/f<n>.md with their index as the body.
func numberedDocs(n int) map[string]string {
	files := map[string]string{}
	for i := 1; i <= n; i++ {
		files[fmt.Sprintf("docs/f%03d.md", i)] = fmt.Sprint(i)
	}
	return files
}

func numberedSummary(n int) []string {
	var all []string
	for i := 1; i <= n; i++ {
		all = append(all, fmt.Sprintf("+ws/r:main:docs/f%03d.md=%d", i, i))
	}
	return all
}

var docs = []string{"docs"}

func TestValidateChecksTheFirstRepositoryOrTheUser(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": {}}})
	ctx := context.Background()

	require.NoError(t, connector(server).Validate(ctx, sourceConfig(goodToken, selection("ws/R", "", "docs"))))
	require.Equal(t, []string{"/repositories/ws/r"}, server.requests())

	require.NoError(t, connector(server).Validate(ctx, sourceConfig(goodToken)))
	require.Equal(t, []string{"/repositories/ws/r", "/user"}, server.requests())

	err := connector(server).Validate(ctx, sourceConfig("wrong", selection("ws/r", "", "docs")))
	require.ErrorIs(t, err, datasource.ErrInvalidCredentials)
}

func TestListResourcesListsWorkspacesRepositoriesThenDirectories(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{
		"c1": {"docs/a.md": "a", "docs/sub/b.md": "b", "src/c.go": "c", "README.md": "r"},
	}})
	ctx := context.Background()
	cfg := sourceConfig(goodToken)

	got, err := connector(server).ListResources(ctx, cfg, "")
	require.NoError(t, err)
	require.Equal(t, []types.Resource{
		{ExternalID: "ws/r", Name: "ws/R", Type: "repository", URL: "https://bitbucket.org/ws/R", HasChildren: true},
		{
			ExternalID: "other/lib", Name: "other/Lib", Type: "repository", URL: "https://bitbucket.org/other/Lib",
			HasChildren: true,
		},
	}, got)

	got, err = connector(server).ListResources(ctx, cfg, "ws/r")
	require.NoError(t, err)
	require.Equal(t, []types.Resource{
		{ExternalID: "ws/r:docs", Name: "docs", Type: "directory", ParentID: "ws/r", HasChildren: true},
		{ExternalID: "ws/r:src", Name: "src", Type: "directory", ParentID: "ws/r", HasChildren: true},
	}, got)
	require.Equal(t, []string{
		"/repositories/ws/r", "/repositories/ws/r/refs/branches/main",
		"/repositories/ws/r/src/c1/?max_depth=1&pagelen=100&fields=values.path,values.type,values.size," +
			"values.attributes,next",
	}, server.requests()[3:])

	got, err = connector(server).ListResources(ctx, cfg, "ws/r:docs")
	require.NoError(t, err)
	require.Equal(t, []types.Resource{
		{ExternalID: "ws/r:docs/sub", Name: "sub", Type: "directory", ParentID: "ws/r:docs", HasChildren: true},
	}, got)
}

func TestListResourcesOfAnEmptyRepositoryListsNothing(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{noMainBranch: true})
	got, err := connector(server).ListResources(context.Background(), sourceConfig(goodToken), "ws/r")
	require.NoError(t, err)
	require.Empty(t, got)
	require.Equal(t, []string{"/repositories/ws/r"}, server.requests())
}

func TestFetchStreamFullSyncEmitsImportableRegularFilesInScope(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{
		commits: map[string]map[string]string{"c1": {
			"docs/a.md": "a", "docs/b.go": "b", "docs/link.md": "x", "docs/big.bin": "y", "vendor/sub": "z",
			"src/c.md": "c", "README.md": "r",
		}},
		attributes: map[string]map[string][]string{"c1": {
			"docs/link.md": {"link"}, "docs/big.bin": {"lfs"}, "vendor/sub": {"subrepository"},
		}},
	})

	h := &streamRecorder{}
	next, err := stream(t, server, nil, h, selection("ws/R", "", "docs"))
	require.NoError(t, err)
	require.Len(t, h.items, 1)
	item := h.items[0]
	require.Equal(t, "bitbucket:"+server.URL+":ws/r:main:docs/a.md", item.ExternalID)
	require.Equal(t, "ws/R/docs/a.md", item.Title)
	require.Equal(t, "R-main/docs/a.md", item.FileName)
	require.Equal(t, "a", string(item.Content))
	require.Equal(t, "text/plain", item.ContentType)
	require.Equal(t, "ws/r", item.SourceResourceID)
	require.True(t, item.UpdatedAt.IsZero())
	require.Equal(t, map[string]string{
		"channel": "bitbucket", "source_type": "bitbucket", "bitbucket_repository": "ws/R",
		"bitbucket_repository_uuid": "{u1}", "bitbucket_ref": "main", "bitbucket_path": "docs/a.md",
		"bitbucket_url": "https://bitbucket.org/ws/R/src/main/docs/a.md",
	}, item.Metadata)
	require.True(t, next.CompleteListing)
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}}, repositories(t, next))
	require.Equal(t, []string{"/repositories/ws/r/src/c1/docs/a.md"}, server.rawRequests())
}

func TestFetchStreamIncrementalUsesDiffstatDirection(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{
		commits: map[string]map[string]string{
			"c1": {"docs/a.md": "v1", "docs/b.md": "b"},
			"c2": {"docs/a.md": "v2", "docs/c.md": "c"},
		},
		refs: map[string]string{"main": "c2"},
	})
	old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}})

	h := &streamRecorder{}
	next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"-ws/r:main:docs/b.md", "+ws/r:main:docs/a.md=v2", "+ws/r:main:docs/c.md=c"},
		summary(server.URL, h.items))
	require.Equal(t, []string{"/repositories/ws/r/diffstat/c2..c1" + diffstatQuery},
		server.requestsMatching("/diffstat/"))
	require.False(t, next.CompleteListing)
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c2", Ref: "main", Paths: docs}}, repositories(t, next))
}

func TestFetchStreamUnchangedHeadEmitsNothing(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": {"docs/a.md": "a"}}})
	old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}})

	h := &streamRecorder{}
	next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Empty(t, h.items)
	require.Len(t, h.checkpoints, 1)
	require.Empty(t, server.requestsMatching("/src/"))
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}}, repositories(t, next))
}

func TestFetchStreamRefChangeTombstonesOldRefAndEmitsNewRoots(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{
		commits: map[string]map[string]string{
			"c1": {"docs/a.md": "a", "docs/b.md": "b"},
			"c2": {"docs/a.md": "a", "docs/c.md": "c"},
		},
		refs: map[string]string{"main": "c1", "release": "c2"},
	})
	old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}})

	h := &streamRecorder{}
	next, err := stream(t, server, old, h, selection("ws/r", "release", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{
		"-ws/r:main:docs/a.md", "-ws/r:main:docs/b.md",
		"+ws/r:release:docs/a.md=a", "+ws/r:release:docs/c.md=c",
	}, summary(server.URL, h.items))
	require.Empty(t, server.requestsMatching("/diffstat/"))
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c2", Ref: "release", Paths: docs}},
		repositories(t, next))
}

func TestFetchStreamPathsChangeTombstonesLostRootAndEmitsGainedRoot(t *testing.T) {
	files := map[string]string{"docs/a.md": "a", "docs/b.md": "b", "guides/g.md": "g", "src/s.md": "s"}
	t.Run("replaced", func(t *testing.T) {
		server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": files}})
		old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}})

		h := &streamRecorder{}
		next, err := stream(t, server, old, h, selection("ws/r", "main", "guides"))
		require.NoError(t, err)
		require.Equal(t, []string{"-ws/r:main:docs/a.md", "-ws/r:main:docs/b.md", "+ws/r:main:guides/g.md=g"},
			summary(server.URL, h.items))
		require.Len(t, server.requestsMatching("/diffstat/"), 1)
		require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: []string{"guides"}}},
			repositories(t, next))
	})
	t.Run("gained", func(t *testing.T) {
		server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": files}})
		old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}})

		h := &streamRecorder{}
		next, err := stream(t, server, old, h, selection("ws/r", "main", "docs", "guides"))
		require.NoError(t, err)
		require.Equal(t, []string{"+ws/r:main:guides/g.md=g"}, summary(server.URL, h.items))
		require.Equal(t, map[string]repositoryCursor{
			"ws/r": {Head: "c1", Ref: "main", Paths: []string{"docs", "guides"}},
		}, repositories(t, next))
	})
}

func TestFetchStreamModifiedFileThatLeftTheListingIsTombstoned(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{
		commits: map[string]map[string]string{
			"c1": {"docs/a.md": "a", "docs/b.md": "b"},
			"c2": {"docs/a.md": "a2", "docs/b.md": "b"},
		},
		attributes: map[string]map[string][]string{"c2": {"docs/a.md": {"lfs"}}},
		refs:       map[string]string{"main": "c2"},
	})
	old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}})

	h := &streamRecorder{}
	next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"-ws/r:main:docs/a.md"}, summary(server.URL, h.items))
	require.Empty(t, server.rawRequests())
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c2", Ref: "main", Paths: docs}}, repositories(t, next))
}

func TestFetchStreamTypeChangedFileThatLeftTheListingIsTombstoned(t *testing.T) {
	ref := map[string]string{"path": "docs/a.md"}
	server := fakeBitbucket(t, fakeRepo{
		commits: map[string]map[string]string{
			"c1": {"docs/a.md": "a", "docs/b.md": "b"},
			"c2": {"docs/a.md": "a", "docs/b.md": "b"},
		},
		attributes: map[string]map[string][]string{"c2": {"docs/a.md": {"link"}}},
		refs:       map[string]string{"main": "c2"},
		diffstat:   []any{map[string]any{"status": "type changed", "old": ref, "new": ref}},
	})
	old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}})

	h := &streamRecorder{}
	next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"-ws/r:main:docs/a.md"}, summary(server.URL, h.items))
	require.Empty(t, server.rawRequests())
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c2", Ref: "main", Paths: docs}}, repositories(t, next))
}

func TestFetchStreamUnreadableCursorEntryIsAFirstSync(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{
		commits: map[string]map[string]string{"c1": {"docs/a.md": "a", "docs/b.md": "b"}},
		aliases: []string{"ws/other"},
	})
	kept := repositoryCursor{Head: "c1", Ref: "main", Paths: docs}
	old := &types.SyncCursor{
		ConnectorCursor: map[string]interface{}{"repositories": map[string]interface{}{
			"ws/r": "c0", "ws/other": kept,
		}},
		Imports: &utils.ImportCapabilities{},
	}

	h := &streamRecorder{}
	next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"), selection("ws/other", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"+ws/r:main:docs/a.md=a", "+ws/r:main:docs/b.md=b"}, summary(server.URL, h.items))
	require.Equal(t, map[string]repositoryCursor{"ws/r": kept, "ws/other": kept}, repositories(t, next))
}

func TestFetchStreamNarrowingAndWideningARoot(t *testing.T) {
	files := map[string]string{"docs/a.md": "a", "docs/sub/b.md": "b"}
	sub := []string{"docs/sub"}
	t.Run("narrowed", func(t *testing.T) {
		server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": files}})
		old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}})

		h := &streamRecorder{}
		next, err := stream(t, server, old, h, selection("ws/r", "main", "docs/sub"))
		require.NoError(t, err)
		require.Equal(t, []string{"-ws/r:main:docs/a.md"}, summary(server.URL, h.items))
		require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: sub}},
			repositories(t, next))
	})
	t.Run("widened", func(t *testing.T) {
		server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": files}})
		old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: sub}})

		h := &streamRecorder{}
		next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
		require.NoError(t, err)
		require.Equal(t, []string{"+ws/r:main:docs/a.md=a"}, summary(server.URL, h.items))
		require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}},
			repositories(t, next))
	})
}

// The service persists ConnectorCursor as JSON and hands it back as generic
// maps; readCursor must recover what bitbucketCursor wrote, a pending with nil
// paths included.
func TestCursorRoundTripsThroughPersistedJSON(t *testing.T) {
	value := cursor{Repositories: map[string]repositoryCursor{
		"ws/r": {Head: "c1", Ref: "main", Pending: &pendingCursor{Head: "c2", Ref: "release", Done: 7, Unknown: true}},
		"ws/x": {Head: "c3", Ref: "main", Paths: docs},
	}}
	raw, err := json.Marshal(bitbucketCursor(value, false).ConnectorCursor)
	require.NoError(t, err)
	var persisted map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &persisted))

	got := readCursor(context.Background(), &types.SyncCursor{ConnectorCursor: persisted})
	require.Equal(t, value.Repositories, got.Repositories)
	require.Contains(t, persisted["raw"],
		`"pending":{"head":"c2","ref":"release","paths":null,"done":7,"unknown":true}`)
}

func TestFetchStreamUnknownChangeSetAfterDiffstatTimeout(t *testing.T) {
	commits := map[string]map[string]string{
		"c1": {"docs/a.md": "a", "docs/old.md": "o"},
		"c2": {"docs/a.md": "a", "docs/new.md": "n"},
	}
	t.Run("timeout", func(t *testing.T) {
		server := fakeBitbucket(t, fakeRepo{
			commits: commits, refs: map[string]string{"main": "c2"}, timeout: map[string]bool{"c1": true},
		})
		old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}})

		h := &streamRecorder{}
		next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
		require.NoError(t, err)
		require.Equal(t, []string{"-ws/r:main:docs/old.md", "+ws/r:main:docs/a.md=a", "+ws/r:main:docs/new.md=n"},
			summary(server.URL, h.items))
		require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c2", Ref: "main", Paths: docs}},
			repositories(t, next))
	})
	t.Run("gone", func(t *testing.T) {
		server := fakeBitbucket(t, fakeRepo{
			commits: map[string]map[string]string{"c2": commits["c2"]}, refs: map[string]string{"main": "c2"},
		})
		old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}})

		h := &streamRecorder{}
		next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
		require.NoError(t, err)
		require.Equal(t, []string{"+ws/r:main:docs/a.md=a", "+ws/r:main:docs/new.md=n"}, summary(server.URL, h.items))
		require.Empty(t, server.requestsMatching("/src/c1/"))
		require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c2", Ref: "main", Paths: docs}},
			repositories(t, next))
	})
}

func TestListingFailsOnDirectoryAtDepthLimit(t *testing.T) {
	deep := "docs/" + strings.TrimSuffix(strings.Repeat("d/", listingDepth), "/") + "/leaf.md"
	server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": {deep: "x", "docs/a.md": "a"}}})

	h := &streamRecorder{}
	next, err := stream(t, server, nil, h, selection("ws/r", "main", "docs"))
	require.ErrorIs(t, err, errDepthExceeded)
	require.Nil(t, next)
	require.Empty(t, h.items)
}

func TestListingFallsBackToPerDirectoryWalkOnTimeout(t *testing.T) {
	files := map[string]string{"docs/a.md": "a", "docs/sub/b.md": "b", "src/c.md": "c"}
	fields := "&pagelen=100&fields=values.path,values.type,values.size,values.attributes,next"
	t.Run("walks", func(t *testing.T) {
		server := fakeBitbucket(t, fakeRepo{
			commits: map[string]map[string]string{"c1": files}, timeout: map[string]bool{"c1": true},
		})

		h := &streamRecorder{}
		next, err := stream(t, server, nil, h, selection("ws/r", "main"))
		require.NoError(t, err)
		require.Equal(t, []string{"+ws/r:main:docs/a.md=a", "+ws/r:main:docs/sub/b.md=b", "+ws/r:main:src/c.md=c"},
			summary(server.URL, h.items))
		require.Equal(t, []string{
			"/repositories/ws/r/src/c1/?max_depth=50" + fields, "/repositories/ws/r/src/c1/?max_depth=10" + fields,
		}, server.requestsMatching("max_depth="))
		require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main"}}, repositories(t, next))
	})
	t.Run("walks a deep root", func(t *testing.T) {
		deep := "docs/" + strings.TrimSuffix(strings.Repeat("d/", fallbackListingDepth), "/") + "/leaf.md"
		server := fakeBitbucket(t, fakeRepo{
			commits: map[string]map[string]string{"c1": {deep: "x", "docs/a.md": "a", "src/c.md": "c"}},
			timeout: map[string]bool{"c1": true},
		})

		h := &streamRecorder{}
		_, err := stream(t, server, nil, h, selection("ws/r", "main", "docs"))
		require.NoError(t, err)
		require.Equal(t, []string{"+ws/r:main:docs/a.md=a", "+ws/r:main:" + deep + "=x"}, summary(server.URL, h.items))
		require.Equal(t, []string{
			"/repositories/ws/r/src/c1/docs/?max_depth=50" + fields,
			"/repositories/ws/r/src/c1/docs/?max_depth=10" + fields,
			"/repositories/ws/r/src/c1/docs/d/d/d/d/d/d/d/d/d/d/?max_depth=10" + fields,
		}, server.requestsMatching("max_depth="))
		require.Empty(t, server.requestsMatching("/src/c1/src/"))
	})
	t.Run("walks a deep directory of the whole repository", func(t *testing.T) {
		deepDir := strings.TrimSuffix(strings.Repeat("d/", fallbackListingDepth), "/")
		server := fakeBitbucket(t, fakeRepo{
			commits: map[string]map[string]string{"c1": {
				"a.md": "a", deepDir + "/leaf.md": "x", "docs/sub/b.md": "b",
			}},
			timeout: map[string]bool{"c1": true},
		})

		h := &streamRecorder{}
		_, err := stream(t, server, nil, h, selection("ws/r", "main"))
		require.NoError(t, err)
		require.Equal(t, []string{
			"+ws/r:main:a.md=a", "+ws/r:main:" + deepDir + "/leaf.md=x", "+ws/r:main:docs/sub/b.md=b",
		}, summary(server.URL, h.items))
		require.Equal(t, []string{
			"/repositories/ws/r/src/c1/?max_depth=50" + fields,
			"/repositories/ws/r/src/c1/?max_depth=10" + fields,
			"/repositories/ws/r/src/c1/" + deepDir + "/?max_depth=10" + fields,
		}, server.requestsMatching("max_depth="))
		require.Empty(t, server.requestsMatching("/src/c1/docs/?"))
	})
	t.Run("a directory timeout fails the sync", func(t *testing.T) {
		deepDir := strings.TrimSuffix(strings.Repeat("d/", fallbackListingDepth), "/")
		server := fakeBitbucket(t, fakeRepo{
			commits: map[string]map[string]string{"c1": {deepDir + "/leaf.md": "x", "docs/a.md": "a"}},
			timeout: map[string]bool{"c1": true, "c1:" + deepDir: true},
		})
		h := &streamRecorder{}
		next, err := stream(t, server, nil, h, selection("ws/r", "main"))
		require.ErrorIs(t, err, errTimedOut)
		require.Nil(t, next)
		require.Empty(t, h.items)
	})
}

func TestMissingRootIsEmptyAndMissingCommitIsGone(t *testing.T) {
	t.Run("missing root", func(t *testing.T) {
		server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": {"src/c.md": "c"}}})

		h := &streamRecorder{}
		next, err := stream(t, server, nil, h, selection("ws/r", "main", "docs"))
		require.NoError(t, err)
		require.Empty(t, h.items)
		require.Contains(t, server.requests(), "/repositories/ws/r/commit/c1")
		require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}},
			repositories(t, next))
	})
	t.Run("gone baseline", func(t *testing.T) {
		server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": {"docs/a.md": "a"}}})
		old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c0", Ref: "release", Paths: docs}})

		h := &streamRecorder{}
		next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
		require.NoError(t, err)
		require.Equal(t, []string{"+ws/r:main:docs/a.md=a"}, summary(server.URL, h.items))
		require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}},
			repositories(t, next))
	})
}

func TestFetchStreamEmptyRepositoryListsAsEmpty(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{noMainBranch: true})

	h := &streamRecorder{}
	next, err := stream(t, server, nil, h, selection("ws/r", "", "docs"))
	require.NoError(t, err)
	require.Empty(t, h.items)
	require.Equal(t, []string{"/repositories/ws/r"}, server.requests())
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Paths: docs}}, repositories(t, next))
}

func TestFetchStreamResumesInsideRepositoryFromPendingCursor(t *testing.T) {
	all := numberedSummary(120)
	server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": numberedDocs(120)}})

	h := &streamRecorder{failAt: 101}
	next, err := stream(t, server, nil, h, selection("ws/r", "main", "docs"))
	require.EqualError(t, err, "ingest failed")
	require.Nil(t, next)
	require.Equal(t, all[:100], summary(server.URL, h.items))
	require.Len(t, h.checkpoints, 2)
	require.Equal(t, map[string]repositoryCursor{
		"ws/r": {Pending: &pendingCursor{Head: "c1", Ref: "main", Paths: docs, Done: 100}},
	}, repositories(t, h.checkpoints[1]))

	resumed := &streamRecorder{}
	next, err = stream(t, server, h.checkpoints[1], resumed, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, all[100:], summary(server.URL, resumed.items))
	require.False(t, next.CompleteListing)
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}}, repositories(t, next))
}

// The pending list is the 50 modified files then docs/x.md and docs/y.md; done 50
// leaves the two additions, which are drained before c3 is diffed against c2.
func TestFetchStreamDrainsPendingBeforeMovedHead(t *testing.T) {
	base := map[string]string{}
	for p := range numberedDocs(50) {
		base[p] = "old"
	}
	c2 := numberedDocs(50)
	c2["docs/x.md"], c2["docs/y.md"] = "x", "y"
	c3 := numberedDocs(50)
	c3["docs/y.md"] = "y"
	server := fakeBitbucket(t, fakeRepo{
		commits: map[string]map[string]string{"c1": base, "c2": c2, "c3": c3},
		refs:    map[string]string{"main": "c3"},
	})
	old := oldCursor(map[string]repositoryCursor{"ws/r": {
		Head: "c1", Ref: "main", Paths: docs,
		Pending: &pendingCursor{Head: "c2", Ref: "main", Paths: docs, Done: 50},
	}})

	h := &streamRecorder{}
	next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"+ws/r:main:docs/x.md=x", "+ws/r:main:docs/y.md=y", "-ws/r:main:docs/x.md"},
		summary(server.URL, h.items))
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c2", Ref: "main", Paths: docs}},
		repositories(t, h.checkpoints[0]))
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c3", Ref: "main", Paths: docs}}, repositories(t, next))
	require.Equal(t, []string{
		"/repositories/ws/r/diffstat/c2..c1" + diffstatQuery,
		"/repositories/ws/r/diffstat/c3..c2" + diffstatQuery,
	}, server.requestsMatching("/diffstat/"))
}

func TestFetchStreamPendingWithGoneBaselineRestartsAtZero(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c2": numberedDocs(60)}})
	old := oldCursor(map[string]repositoryCursor{"ws/r": {
		Head: "c0", Ref: "main", Paths: docs,
		Pending: &pendingCursor{Head: "c2", Ref: "main", Paths: docs, Done: 50},
	}})

	h := &streamRecorder{}
	next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, numberedSummary(60), summary(server.URL, h.items))
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c2", Ref: "main", Paths: docs}}, repositories(t, next))
}

// Run A's diffstat timed out, so its list was every file of the c2 listing and
// the cut fell at 50; the drain must keep that list even though the diffstat
// now answers, or the changed files after position 50 are never fetched.
func TestFetchStreamPendingUnknownListIsKeptWhenDiffstatRecovers(t *testing.T) {
	c1 := numberedDocs(60)
	for i := 41; i <= 60; i++ {
		c1[fmt.Sprintf("docs/f%03d.md", i)] = "old"
	}
	commits := map[string]map[string]string{"c1": c1, "c2": numberedDocs(60)}
	refs := map[string]string{"main": "c2"}
	old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}})

	runA := fakeBitbucket(t, fakeRepo{commits: commits, refs: refs, timeout: map[string]bool{"c1": true}})
	h := &streamRecorder{failAt: 51}
	_, err := stream(t, runA, old, h, selection("ws/r", "main", "docs"))
	require.EqualError(t, err, "ingest failed")
	require.Equal(t, numberedSummary(60)[:50], summary(runA.URL, h.items))
	require.Equal(t, map[string]repositoryCursor{"ws/r": {
		Head: "c1", Ref: "main", Paths: docs,
		Pending: &pendingCursor{Head: "c2", Ref: "main", Paths: docs, Done: 50, Unknown: true},
	}}, repositories(t, h.checkpoints[len(h.checkpoints)-1]))

	runB := fakeBitbucket(t, fakeRepo{commits: commits, refs: refs})
	resumed := &streamRecorder{}
	next, err := stream(t, runB, h.checkpoints[len(h.checkpoints)-1], resumed, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, numberedSummary(60)[50:], summary(runB.URL, resumed.items))
	require.Empty(t, runB.requestsMatching("/diffstat/"))
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c2", Ref: "main", Paths: docs}}, repositories(t, next))
}

// Run A's list was the 60 diffstat entries; a drain whose diffstat times out
// builds the longer unknown list, where offset 50 points elsewhere, so it
// restarts at zero.
func TestFetchStreamPendingDiffstatListRestartsWhenDiffstatTimesOut(t *testing.T) {
	c1 := numberedDocs(120)
	for i := 61; i <= 120; i++ {
		c1[fmt.Sprintf("docs/f%03d.md", i)] = "old"
	}
	commits := map[string]map[string]string{"c1": c1, "c2": numberedDocs(120)}
	refs := map[string]string{"main": "c2"}
	old := oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}})

	runA := fakeBitbucket(t, fakeRepo{commits: commits, refs: refs})
	h := &streamRecorder{failAt: 51}
	_, err := stream(t, runA, old, h, selection("ws/r", "main", "docs"))
	require.EqualError(t, err, "ingest failed")
	require.Equal(t, numberedSummary(120)[60:110], summary(runA.URL, h.items))
	require.Equal(t, map[string]repositoryCursor{"ws/r": {
		Head: "c1", Ref: "main", Paths: docs,
		Pending: &pendingCursor{Head: "c2", Ref: "main", Paths: docs, Done: 50},
	}}, repositories(t, h.checkpoints[len(h.checkpoints)-1]))

	runB := fakeBitbucket(t, fakeRepo{commits: commits, refs: refs, timeout: map[string]bool{"c1": true}})
	resumed := &streamRecorder{}
	next, err := stream(t, runB, h.checkpoints[len(h.checkpoints)-1], resumed, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, numberedSummary(120), summary(runB.URL, resumed.items))
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c2", Ref: "main", Paths: docs}}, repositories(t, next))
}

// The pending list is the 50 modified files then docs/f051.md and docs/f052.md;
// done 50 leaves the two additions, drained under main before the new
// selection is reconciled against c2.
func TestFetchStreamDrainsPendingBeforeChangedSelection(t *testing.T) {
	c1 := map[string]string{}
	for p := range numberedDocs(50) {
		c1[p] = "old"
	}
	c2 := numberedDocs(52)
	c2["guides/h.md"] = "h"
	commits := map[string]map[string]string{"c1": c1, "c2": c2, "c3": {"docs/g.md": "g"}}
	refs := map[string]string{"main": "c2", "release": "c3"}
	old := oldCursor(map[string]repositoryCursor{"ws/r": {
		Head: "c1", Ref: "main", Paths: docs,
		Pending: &pendingCursor{Head: "c2", Ref: "main", Paths: docs, Done: 50},
	}})
	drained := []string{"+ws/r:main:docs/f051.md=51", "+ws/r:main:docs/f052.md=52"}
	var tombstones []string
	for i := 1; i <= 52; i++ {
		tombstones = append(tombstones, fmt.Sprintf("-ws/r:main:docs/f%03d.md", i))
	}
	for _, tc := range []struct {
		name string
		sel  map[string]interface{}
		want []string
		next repositoryCursor
	}{
		{
			name: "ref", sel: selection("ws/r", "release", "docs"),
			want: append(slices.Concat(drained, tombstones), "+ws/r:release:docs/g.md=g"),
			next: repositoryCursor{Head: "c3", Ref: "release", Paths: docs},
		},
		{
			name: "paths", sel: selection("ws/r", "main", "guides"),
			want: append(slices.Concat(drained, tombstones), "+ws/r:main:guides/h.md=h"),
			next: repositoryCursor{Head: "c2", Ref: "main", Paths: []string{"guides"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := fakeBitbucket(t, fakeRepo{commits: commits, refs: refs})

			h := &streamRecorder{}
			next, err := stream(t, server, old, h, tc.sel)
			require.NoError(t, err)
			require.Equal(t, tc.want, summary(server.URL, h.items))
			require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c2", Ref: "main", Paths: docs}},
				repositories(t, h.checkpoints[0]))
			require.Equal(t, map[string]repositoryCursor{"ws/r": tc.next}, repositories(t, next))
			require.Len(t, server.requestsMatching("/src/c2/docs/?"), 1, "the drained listing is the next baseline")
		})
	}
}

func TestFetchStreamGonePendingHeadIsDropped(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{
		commits: map[string]map[string]string{
			"c1": {"docs/a.md": "a", "docs/b.md": "b"},
			"c3": {"docs/a.md": "a2", "docs/c.md": "c"},
		},
		refs: map[string]string{"main": "c3"},
	})
	old := oldCursor(map[string]repositoryCursor{"ws/r": {
		Head: "c1", Ref: "main", Paths: docs,
		Pending: &pendingCursor{Head: "c2", Ref: "main", Paths: docs, Done: 50},
	}})

	h := &streamRecorder{}
	next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"-ws/r:main:docs/b.md", "+ws/r:main:docs/a.md=a2", "+ws/r:main:docs/c.md=c"},
		summary(server.URL, h.items))
	require.Equal(t, []string{"/repositories/ws/r/diffstat/c3..c1" + diffstatQuery},
		server.requestsMatching("/diffstat/"))
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c3", Ref: "main", Paths: docs}}, repositories(t, next))
}

func TestPendingIsDroppedWhenImportCapabilitiesChange(t *testing.T) {
	restarted := []string{"+ws/r:main:docs/b.md=b", "+ws/r:main:docs/c.md=c", "+ws/r:main:docs/d.md=d"}
	for _, tc := range []struct {
		name  string
		stamp *utils.ImportCapabilities
		caps  utils.ImportCapabilities
		want  []string
	}{
		{name: "lost", stamp: &utils.ImportCapabilities{Images: true}, want: restarted},
		{name: "unstamped", want: restarted},
		{
			name: "unstamped with every capability", caps: utils.AllImportCapabilities,
			want: []string{"+ws/r:main:docs/c.md=c", "+ws/r:main:docs/d.md=d"},
		},
		{
			name: "unchanged", stamp: &utils.ImportCapabilities{Images: true},
			caps: utils.ImportCapabilities{Images: true},
			want: []string{"+ws/r:main:docs/c.md=c", "+ws/r:main:docs/d.md=d"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": {
				"docs/a.png": "png", "docs/b.md": "b", "docs/c.md": "c", "docs/d.md": "d",
			}}})
			old := oldCursor(map[string]repositoryCursor{
				"ws/r": {Pending: &pendingCursor{Head: "c1", Ref: "main", Paths: docs, Done: 2}},
			})
			old.Imports = tc.stamp
			h := &streamRecorder{}
			cfg := withImports(sourceConfig(goodToken, selection("ws/r", "main", "docs")), tc.caps)

			next, err := connector(server).FetchStream(context.Background(), cfg, old, h)
			require.NoError(t, err)
			require.Equal(t, tc.want, summary(server.URL, h.items))
			require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}},
				repositories(t, next))
		})
	}
}

func TestRemovedRepositoryIsTombstonedOrDropped(t *testing.T) {
	commits := map[string]map[string]string{
		"c1": {"docs/a.md": "a", "docs/b.md": "b"},
		"c2": {"docs/a.md": "a", "docs/c.md": "c"},
	}
	gone := repositoryCursor{
		Head: "c1", Ref: "main", Paths: docs,
		Pending: &pendingCursor{Head: "c2", Ref: "main", Paths: docs, Done: 1},
	}
	cursor := func() *types.SyncCursor {
		return oldCursor(map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}, "ws/gone": gone})
	}
	main := map[string]string{"main": "c1"}
	kept := map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}}

	t.Run("tombstoned", func(t *testing.T) {
		server := fakeBitbucket(t, fakeRepo{commits: commits, refs: main, aliases: []string{"ws/gone"}})

		h := &streamRecorder{}
		next, err := stream(t, server, cursor(), h, selection("ws/r", "main", "docs"))
		require.NoError(t, err)
		require.Equal(t, []string{"-ws/gone:main:docs/a.md", "-ws/gone:main:docs/b.md", "-ws/gone:main:docs/c.md"},
			summary(server.URL, h.items))
		require.Equal(t, kept, repositories(t, next))
		require.Len(t, h.checkpoints, 2)
		require.Equal(t, kept, repositories(t, h.checkpoints[1]))
	})
	t.Run("tombstoned under another pending ref", func(t *testing.T) {
		server := fakeBitbucket(t, fakeRepo{commits: commits, refs: main, aliases: []string{"ws/gone"}})
		other := gone
		other.Pending = &pendingCursor{Head: "c2", Ref: "release", Paths: docs, Done: 1}
		old := oldCursor(map[string]repositoryCursor{"ws/gone": other})

		h := &streamRecorder{}
		next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
		require.NoError(t, err)
		require.Equal(t, []string{
			"+ws/r:main:docs/a.md=a", "+ws/r:main:docs/b.md=b",
			"-ws/gone:main:docs/a.md", "-ws/gone:main:docs/b.md",
			"-ws/gone:release:docs/a.md", "-ws/gone:release:docs/c.md",
		}, summary(server.URL, h.items))
		require.Equal(t, kept, repositories(t, next))
	})
	for name, status := range map[string]int{"not found": http.StatusNotFound, "forbidden": http.StatusForbidden} {
		t.Run(name, func(t *testing.T) {
			server := fakeBitbucket(t, fakeRepo{
				commits: commits, refs: main, status: map[string]int{"ws/gone": status},
			})

			h := &streamRecorder{}
			next, err := stream(t, server, cursor(), h, selection("ws/r", "main", "docs"))
			require.NoError(t, err)
			require.Empty(t, h.items)
			require.Equal(t, kept, repositories(t, next))
		})
	}
	t.Run("gone recorded commit", func(t *testing.T) {
		server := fakeBitbucket(t, fakeRepo{
			commits: map[string]map[string]string{"c1": commits["c1"]}, aliases: []string{"ws/gone"},
		})
		old := oldCursor(map[string]repositoryCursor{
			"ws/r": {Head: "c1", Ref: "main", Paths: docs}, "ws/gone": {Head: "c0", Ref: "main", Paths: docs},
		})

		h := &streamRecorder{}
		next, err := stream(t, server, old, h, selection("ws/r", "main", "docs"))
		require.NoError(t, err)
		require.Empty(t, h.items)
		require.Equal(t, kept, repositories(t, next))
	})
	transient := map[string]int{
		"server error": http.StatusInternalServerError, "timeout": statusTimedOut,
		"rate limited": http.StatusTooManyRequests,
	}
	for name, status := range transient {
		t.Run(name, func(t *testing.T) {
			server := fakeBitbucket(t, fakeRepo{
				commits: commits, refs: main, status: map[string]int{"ws/gone": status},
			})

			h := &streamRecorder{}
			next, err := stream(t, server, cursor(), h, selection("ws/r", "main", "docs"))
			if status == http.StatusTooManyRequests {
				require.ErrorIs(t, err, errRateLimited)
			} else {
				require.ErrorContains(t, err, "status "+strconv.Itoa(status))
			}
			require.Nil(t, next)
			require.Empty(t, h.items)
			require.Equal(t, gone, repositories(t, h.checkpoints[len(h.checkpoints)-1])["ws/gone"])
		})
	}
}

func TestOversizedAndLFSFilesBecomeErrorItems(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{
		commits: map[string]map[string]string{"c1": {"docs/a.md": "a", "docs/big.md": "big", "docs/lfs.md": "l"}},
		sizes:   map[string]map[string]int64{"c1": {"docs/big.md": 101 << 20}},
		lfs:     map[string]bool{"docs/lfs.md": true},
	})

	h := &streamRecorder{}
	next, err := stream(t, server, nil, h, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{
		"+ws/r:main:docs/a.md=a", "!ws/r:main:docs/big.md=bitbucket_file_too_large",
		"!ws/r:main:docs/lfs.md=bitbucket_lfs_unsupported",
	}, summary(server.URL, h.items))
	require.Equal(t, []string{"/repositories/ws/r/src/c1/docs/a.md", "/repositories/ws/r/src/c1/docs/lfs.md"},
		server.rawRequests())
	big := h.items[1]
	require.Equal(t, "ws/R/docs/big.md", big.Title)
	require.Empty(t, big.Content)
	require.Equal(t, map[string]string{
		"channel": "bitbucket", "source_type": "bitbucket", "bitbucket_path": "docs/big.md",
		"error":             SyncReasonTexts[reasonFileTooLarge],
		"error_reason_code": reasonFileTooLarge, "error_reason": SyncReasonTexts[reasonFileTooLarge],
	}, big.Metadata)
	require.True(t, next.CompleteListing)
}

func TestRawOverrunBecomesTooLargeErrorItem(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": {"docs/nine.md": "123456789"}}})
	c, err := connector(server).configured(sourceConfig(goodToken, selection("ws/r", "main", "docs")))
	require.NoError(t, err)
	c.client.rawLimit = 8
	repo, err := c.client.repository(context.Background(), "ws/r")
	require.NoError(t, err)

	item, err := c.item(context.Background(), repo, "ws/r", "main", "c1", "docs/nine.md")
	require.NoError(t, err)
	require.Equal(t, []string{"!ws/r:main:docs/nine.md=bitbucket_file_too_large"},
		summary(server.URL, []types.FetchedItem{item}))
}

func TestRateLimitEndsTheRunAfterACheckpoint(t *testing.T) {
	inner := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": numberedDocs(60)}})
	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repositories/ws/r/src/c1/docs/f051.md" {
			w.Header().Set("x-ratelimit-reset", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		inner.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(limited.Close)
	server := &fakeServer{Server: limited}

	h := &streamRecorder{}
	next, err := stream(t, server, nil, h, selection("ws/r", "main", "docs"))
	require.ErrorIs(t, err, errRateLimited)
	require.Nil(t, next)
	require.Len(t, h.items, 50)
	require.Equal(t, map[string]repositoryCursor{
		"ws/r": {Pending: &pendingCursor{Head: "c1", Ref: "main", Paths: docs, Done: 50}},
	}, repositories(t, h.checkpoints[len(h.checkpoints)-1]))
}

func TestRateLimitedDiffstatKeepsThePending(t *testing.T) {
	inner := fakeBitbucket(t, fakeRepo{
		commits: map[string]map[string]string{
			"c1": {"docs/a.md": "a"}, "c2": {"docs/a.md": "a2", "docs/b.md": "b"},
		},
		refs:    map[string]string{"main": "c2"},
		aliases: []string{"ws/other"},
	})
	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/repositories/ws/r/diffstat/") {
			w.Header().Set("x-ratelimit-reset", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		inner.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(limited.Close)
	server := &fakeServer{Server: limited}
	pending := repositoryCursor{
		Head: "c1", Ref: "main", Paths: docs,
		Pending: &pendingCursor{Head: "c2", Ref: "main", Paths: docs, Done: 1},
	}
	unchanged := repositoryCursor{Head: "c2", Ref: "main", Paths: docs}
	old := oldCursor(map[string]repositoryCursor{"ws/other": unchanged, "ws/r": pending})

	h := &streamRecorder{}
	next, err := stream(t, server, old, h, selection("ws/other", "main", "docs"), selection("ws/r", "main", "docs"))
	require.ErrorIs(t, err, errRateLimited)
	require.Nil(t, next)
	require.Empty(t, h.items)
	require.NotEmpty(t, h.checkpoints)
	require.Equal(t, map[string]repositoryCursor{"ws/other": unchanged, "ws/r": pending},
		repositories(t, h.checkpoints[len(h.checkpoints)-1]))
}

func TestEmptyFileSyncsItsPath(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": {"docs/empty.md": ""}}})

	h := &streamRecorder{}
	_, err := stream(t, server, nil, h, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"+ws/r:main:docs/empty.md=# ws/R/docs/empty.md\n"}, summary(server.URL, h.items))
}

func TestPathsAreEscapedInSrcURLs(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": {"docs/a b#c.md": "a"}}})

	h := &streamRecorder{}
	_, err := stream(t, server, nil, h, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"+ws/r:main:docs/a b#c.md=a"}, summary(server.URL, h.items))
	require.Equal(t, []string{"/repositories/ws/r/src/c1/docs/a%20b%23c.md"}, server.rawRequests())
	require.Equal(t, "https://bitbucket.org/ws/R/src/main/docs/a b#c.md", h.items[0].Metadata["bitbucket_url"])
}

func TestRawRedirectBecomesLFSErrorItem(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{
		commits: map[string]map[string]string{"c1": {"docs/a.md": "a", "docs/z.md": "z"}},
		lfs:     map[string]bool{"docs/a.md": true},
	})

	h := &streamRecorder{}
	next, err := stream(t, server, nil, h, selection("ws/r", "main", "docs"))
	require.NoError(t, err)
	require.Equal(t, []string{"!ws/r:main:docs/a.md=bitbucket_lfs_unsupported", "+ws/r:main:docs/z.md=z"},
		summary(server.URL, h.items))
	require.Equal(t, []string{"/repositories/ws/r/src/c1/docs/a.md", "/repositories/ws/r/src/c1/docs/z.md"},
		server.rawRequests())
	require.True(t, next.CompleteListing)
}

func TestFetchStreamMissingRepositoryIsResourceNotFound(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": {}}})
	_, err := stream(t, server, nil, &streamRecorder{}, selection("ws/nope", "", "docs"))
	require.ErrorIs(t, err, datasource.ErrResourceNotFound)
	require.Equal(t, []string{"/repositories/ws/nope"}, server.requests())
}

func TestFetchAllAndFetchIncrementalCollectTheStream(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{"c1": {"docs/a.md": "a"}}})
	ctx := context.Background()
	cfg := sourceConfig(goodToken, selection("ws/r", "main", "docs"))

	items, err := connector(server).FetchAll(ctx, cfg, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"+ws/r:main:docs/a.md=a"}, summary(server.URL, items))

	items, next, err := connector(server).FetchIncremental(ctx, cfg, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"+ws/r:main:docs/a.md=a"}, summary(server.URL, items))
	require.Equal(t, map[string]repositoryCursor{"ws/r": {Head: "c1", Ref: "main", Paths: docs}}, repositories(t, next))

	ancestors, err := connector(server).ResolveResourceAncestors(ctx, cfg, []string{"ws/r"})
	require.NoError(t, err)
	require.Equal(t, []string{}, ancestors)
	require.Equal(t, types.ConnectorTypeBitbucket, NewConnector().Type())
}
