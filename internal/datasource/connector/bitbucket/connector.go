package bitbucket

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/datasource/connector/repofile"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/utils"
)

var _ datasource.StreamingConnector = (*Connector)(nil)

const checkpointEvery = 50

const (
	reasonFileTooLarge   = "bitbucket_file_too_large"
	reasonLFSUnsupported = "bitbucket_lfs_unsupported"
)

// SyncReasonTexts maps every reason code to its English fallback.
var SyncReasonTexts = map[string]string{
	reasonFileTooLarge:   "The Bitbucket file exceeds the 100 MB import cap",
	reasonLFSUnsupported: "The Bitbucket file is stored in Git LFS, which is not imported",
}

// Connector syncs document files from selected Bitbucket Cloud repositories.
type Connector struct {
	// baseURL is the API host; the production value is fixed and tests point it at a fake.
	baseURL    string
	client     *client
	imports    utils.ImportCapabilities
	importable func(string) bool
}

// NewConnector creates a stateless connector. Each data source provides its
// own access token in its encrypted credentials.
func NewConnector() *Connector { return &Connector{baseURL: apiBaseURL} }

// Type returns the connector type.
func (c *Connector) Type() string { return types.ConnectorTypeBitbucket }

func (c *Connector) configured(ds *types.DataSourceConfig) (*Connector, error) {
	if ds == nil {
		return nil, datasource.ErrInvalidConfig
	}
	username, _ := ds.Credentials["username"].(string)
	token, _ := ds.Credentials["access_token"].(string)
	client, err := newClient(c.baseURL, username, token)
	if err != nil {
		return nil, err
	}
	return &Connector{baseURL: c.baseURL, client: client, imports: ds.Imports, importable: ds.Importable}, nil
}

// Validate checks the credentials against the first selected repository, the
// call every sync opens with, or against the user when nothing is selected yet.
func (c *Connector) Validate(ctx context.Context, ds *types.DataSourceConfig) error {
	configured, err := c.configured(ds)
	if err != nil {
		return err
	}
	if _, ok := ds.Settings["repositories"]; !ok {
		return configured.client.ping(ctx)
	}
	cfg, err := parseConfig(ds)
	if err != nil {
		return err
	}
	_, err = configured.client.repository(ctx, cfg.Repositories[0].Repository)
	return err
}

// ListResources lists every repository of every workspace the token can see,
// or the subdirectories of parent ("workspace/slug[:dir]") on the main branch.
func (c *Connector) ListResources(
	ctx context.Context, ds *types.DataSourceConfig, parent string,
) ([]types.Resource, error) {
	var err error
	if c, err = c.configured(ds); err != nil {
		return nil, err
	}
	out := []types.Resource{}
	if parent == "" {
		workspaces, err := c.client.workspaces(ctx)
		if err != nil {
			return nil, err
		}
		for _, workspace := range workspaces {
			repos, err := c.client.repositories(ctx, workspace)
			if err != nil {
				return nil, err
			}
			for _, r := range repos {
				out = append(out, types.Resource{
					ExternalID: strings.ToLower(r.FullName), Name: r.FullName, Type: "repository",
					URL: r.Links.HTML.Href, HasChildren: true,
				})
			}
		}
		return out, nil
	}
	key, dir, _ := strings.Cut(parent, ":")
	repo, err := c.client.repository(ctx, key)
	if err != nil {
		return nil, err
	}
	if repo.MainBranch == nil {
		return out, nil
	}
	// A branch name may contain "/", which the src endpoint would read as a path.
	hash, err := c.client.headCommit(ctx, key, repo.MainBranch.Name)
	if err != nil {
		return nil, err
	}
	entries, err := c.client.listing(ctx, key, hash, dir, 1)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Type != "commit_directory" {
			continue
		}
		out = append(out, types.Resource{
			ExternalID: key + ":" + e.Path, Name: path.Base(e.Path), Type: "directory", ParentID: parent,
			HasChildren: true,
		})
	}
	return out, nil
}

// ResolveResourceAncestors is a no-op: the selection is explicit paths.
func (c *Connector) ResolveResourceAncestors(context.Context, *types.DataSourceConfig, []string) ([]string, error) {
	return []string{}, nil
}

// collector gathers a stream for the compatibility methods required by the
// base Connector interface.
type collector struct {
	items []types.FetchedItem
}

func (h *collector) Emit(_ context.Context, item types.FetchedItem) error {
	h.items = append(h.items, item)
	return nil
}

func (h *collector) Checkpoint(context.Context, *types.SyncCursor) error { return nil }

// FetchAll collects what a full FetchStream would emit.
func (c *Connector) FetchAll(ctx context.Context, ds *types.DataSourceConfig, _ []string) ([]types.FetchedItem, error) {
	h := &collector{}
	_, err := c.FetchStream(ctx, ds, nil, h)
	return h.items, err
}

// FetchIncremental collects what FetchStream would emit from old.
func (c *Connector) FetchIncremental(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	h := &collector{}
	next, err := c.FetchStream(ctx, ds, old, h)
	if err != nil {
		return nil, nil, err
	}
	return h.items, next, nil
}

// A repositoryCursor records the selection its head was synced with, not the
// head alone: the recorded ref and paths say which items the index holds, so a
// changed ref tombstones every old-ref item and a changed directory list emits
// exactly the files that entered or left the selection.
type repositoryCursor struct {
	Head    string         `json:"head"`
	Ref     string         `json:"ref"`
	Paths   []string       `json:"paths"`
	Pending *pendingCursor `json:"pending,omitempty"`
}

// A pendingCursor records the head, ref and paths of an interrupted run and
// how many entries of its change list were emitted. The list is computed
// against the completed baseline (Head, Ref and Paths of the repositoryCursor),
// and the baseline is a commit that still exists, so a retry recomputes the
// same list and continues at Done. GitHub needs a chain of pendings because its
// baseline is a snapshot overlaid with each interrupted prefix; here the
// pending run is always finished first, whatever changed in the meantime, and
// recorded as the new baseline, so one slot is enough and no path an
// interrupted run emitted is left without its tombstone. If the baseline
// commit is gone when the run resumes, the list's definition changed, so the
// resume restarts at zero: re-emitting is idempotent, skipping is not.
// Unknown records that the list was built without a diffstat (it timed out or
// its commits were gone), because a diffstat timeout is transient: the drain
// must build the list the same way or Done indexes a different list.
type pendingCursor struct {
	Head    string   `json:"head"`
	Ref     string   `json:"ref"`
	Paths   []string `json:"paths"`
	Done    int      `json:"done"`
	Unknown bool     `json:"unknown,omitempty"`
}

type cursor struct {
	Repositories map[string]repositoryCursor `json:"repositories"`
}

// readCursor treats an unreadable repository entry as a first sync rather than failing the run.
func readCursor(ctx context.Context, old *types.SyncCursor) cursor {
	out := cursor{Repositories: map[string]repositoryCursor{}}
	if old == nil {
		return out
	}
	b, _ := json.Marshal(old.ConnectorCursor["repositories"])
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	for key, v := range raw {
		var rc repositoryCursor
		if err := json.Unmarshal(v, &rc); err != nil {
			logger.Warnf(ctx, "[Bitbucket] %s: cursor entry is unreadable (%v), treating it as a first sync", key, err)
			continue
		}
		out.Repositories[key] = rc
	}
	return out
}

func bitbucketCursor(value cursor, complete bool) *types.SyncCursor {
	repos := maps.Clone(value.Repositories)
	raw, _ := json.Marshal(cursor{Repositories: repos})
	return &types.SyncCursor{
		LastSyncTime:    time.Now().UTC(),
		ConnectorCursor: map[string]interface{}{"repositories": repos, "raw": string(raw)},
		CompleteListing: complete,
	}
}

// FetchStream is the production sync path. It emits each file as soon as it is
// read, so large repositories do not accumulate their contents in memory. A
// run that started from a nil cursor returns a complete listing, which lets
// the service remove items the run did not emit.
func (c *Connector) FetchStream(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor, h datasource.StreamHandler,
) (*types.SyncCursor, error) {
	var err error
	if c, err = c.configured(ds); err != nil {
		return nil, err
	}
	cfg, err := parseConfig(ds)
	if err != nil {
		return nil, err
	}
	complete := old == nil
	next := readCursor(ctx, old)
	// Pending.Done indexes a change list computed under the cursor's recorded
	// capabilities; under another filter it points at the wrong entries.
	if old != nil && old.RecordedImports() != c.imports {
		for key, rc := range next.Repositories {
			if rc.Pending != nil {
				logger.Infof(ctx,
					"[Bitbucket] %s: import capabilities changed since the interrupted run, restarting its sync", key)
			}
			rc.Pending = nil
			next.Repositories[key] = rc
		}
	}
	for _, selection := range cfg.Repositories {
		if err := c.streamRepository(ctx, selection, next, h); err != nil {
			return nil, err
		}
	}
	for _, key := range slices.Sorted(maps.Keys(next.Repositories)) {
		if slices.ContainsFunc(cfg.Repositories, func(s repositorySelection) bool { return s.Repository == key }) {
			continue
		}
		if err := c.removeRepository(ctx, key, next, h); err != nil {
			return nil, err
		}
	}
	return bitbucketCursor(next, complete), nil
}

func (c *Connector) streamRepository(
	ctx context.Context, selection repositorySelection, next cursor, h datasource.StreamHandler,
) error {
	key, roots := selection.Repository, selection.Paths
	repo, err := c.client.repository(ctx, key)
	if err != nil {
		return err
	}
	ref, head := selection.Ref, ""
	if ref == "" && repo.MainBranch != nil {
		ref = repo.MainBranch.Name
	}
	// An empty repository has no main branch and lists as empty.
	if ref != "" {
		if head, err = c.client.headCommit(ctx, key, ref); err != nil {
			return err
		}
	}
	rc := next.Repositories[key]
	checkpoint := func(pending *pendingCursor) error {
		rc.Pending = pending
		next.Repositories[key] = rc
		return h.Checkpoint(ctx, bitbucketCursor(next, false))
	}
	// A drained pending run's listing is the baseline of the run after it.
	var drained files
	if p := rc.Pending; p != nil {
		pendingFiles, err := c.files(ctx, key, p.Head, p.Paths)
		switch {
		case errors.Is(err, errMissingCommit):
			logger.Warnf(ctx, "[Bitbucket] %s: pending head %s is gone, dropping the interrupted run; "+
				"files it added that the current head lacks stay until a full-mode sync with deletion sync on",
				key, p.Head)
			rc.Pending = nil
		case err != nil:
			return err
		default:
			if err := c.run(ctx, repo, key, &rc, p.Head, p.Ref, p.Paths, pendingFiles, nil, p.Done, p.Unknown,
				h, checkpoint); err != nil {
				return err
			}
			drained = pendingFiles
		}
	}
	if rc.Head == head && rc.Ref == ref && slices.Equal(rc.Paths, roots) {
		return checkpoint(nil)
	}
	nextFiles := files{}
	if head != "" {
		if nextFiles, err = c.files(ctx, key, head, roots); err != nil {
			return err
		}
	}
	return c.run(ctx, repo, key, &rc, head, ref, roots, nextFiles, drained, 0, false, h, checkpoint)
}

// run emits the change list from the baseline rc to head, ref and paths from
// start on, then records them as the new baseline. recorded, when non-nil, is
// the listing of rc, already fetched. A gone baseline, or a diffstat that a
// resumed run built its list with but that timed out this time, restarts at 0.
func (c *Connector) run(
	ctx context.Context, repo *repository, key string, rc *repositoryCursor, head, ref string, paths []string,
	next, recorded files, start int, forceUnknown bool, h datasource.StreamHandler,
	checkpoint func(*pendingCursor) error,
) error {
	list, gone, unknown, err := c.changeList(ctx, key, *rc, head, ref, paths, next, recorded, forceUnknown)
	if err != nil {
		return err
	}
	switch {
	case gone:
		start = 0
	case unknown && !forceUnknown && start > 0:
		logger.Infof(ctx, "[Bitbucket] %s: the diffstat of the interrupted run timed out this time, "+
			"restarting it over the whole listing", key)
		start = 0
	}
	err = c.emit(ctx, repo, key, ref, head, rc.Ref, list, next, start, h, func(n int) error {
		return checkpoint(&pendingCursor{Head: head, Ref: ref, Paths: paths, Done: n, Unknown: unknown})
	})
	if err != nil {
		return err
	}
	*rc = repositoryCursor{Head: head, Ref: ref, Paths: paths}
	return checkpoint(nil)
}

// emit sends list[start:]. A tombstone goes under prevRef, the ref its item was indexed under.
func (c *Connector) emit(
	ctx context.Context, repo *repository, key, ref, head, prevRef string, list []change, sizes files,
	start int, h datasource.StreamHandler, checkpoint func(done int) error,
) error {
	for i := start; i < len(list); i++ {
		ch := list[i]
		var item types.FetchedItem
		switch {
		case ch.Deleted:
			item = c.deleted(key, prevRef, ch.Path)
		case sizes[ch.Path] > c.client.rawLimit:
			item = c.errorItem(repo, key, ref, ch.Path, reasonFileTooLarge)
		default:
			var err error
			if item, err = c.item(ctx, repo, key, ref, head, ch.Path); err != nil {
				return err
			}
		}
		if err := h.Emit(ctx, item); err != nil {
			return err
		}
		if n := i + 1; n%checkpointEvery == 0 {
			if err := checkpoint(n); err != nil {
				return err
			}
		}
	}
	return nil
}

// removeRepository tombstones everything the index holds for a repository that
// left the selection: the completed baseline under its ref, then what an
// interrupted run added under the pending ref, and drops its cursor entry.
func (c *Connector) removeRepository(
	ctx context.Context, key string, next cursor, h datasource.StreamHandler,
) error {
	rc := next.Repositories[key]
	drop := func() error {
		delete(next.Repositories, key)
		return h.Checkpoint(ctx, bitbucketCursor(next, false))
	}
	// Deletion and renaming are the ordinary reasons a repository leaves the
	// selection; keeping the entry would fail every later sync with no
	// operator fix. Any other error is returned, so a transient failure does
	// not orphan the repository's items.
	unreadable := func(err error) error {
		if !errors.Is(err, datasource.ErrPermissionDenied) && !errors.Is(err, datasource.ErrResourceNotFound) &&
			!errors.Is(err, errMissingCommit) {
			return err
		}
		logger.Warnf(ctx, "[Bitbucket] %s left the selection but its recorded listing is unreadable (%v); "+
			"its remaining items stay until a full-mode sync with deletion sync on reconciles them", key, err)
		return drop()
	}
	emitted := map[string]bool{}
	if rc.Head != "" {
		prev, err := c.files(ctx, key, rc.Head, rc.Paths)
		if err != nil {
			return unreadable(err)
		}
		for _, p := range slices.Sorted(maps.Keys(prev)) {
			if err := h.Emit(ctx, c.deleted(key, rc.Ref, p)); err != nil {
				return err
			}
			emitted[p] = true
		}
	}
	if p := rc.Pending; p != nil {
		pending, err := c.files(ctx, key, p.Head, p.Paths)
		if err != nil {
			return unreadable(err)
		}
		for _, file := range slices.Sorted(maps.Keys(pending)) {
			if p.Ref == rc.Ref && emitted[file] {
				continue
			}
			if err := h.Emit(ctx, c.deleted(key, p.Ref, file)); err != nil {
				return err
			}
		}
	}
	return drop()
}

func (c *Connector) externalID(key, ref, file string) string {
	return "bitbucket:" + c.client.baseURL + ":" + key + ":" + ref + ":" + file
}

func (c *Connector) item(
	ctx context.Context, repo *repository, key, ref, head, file string,
) (types.FetchedItem, error) {
	body, err := c.client.raw(ctx, key, head, file)
	switch {
	case errors.Is(err, errLFSRedirect):
		return c.errorItem(repo, key, ref, file, reasonLFSUnsupported), nil
	case errors.Is(err, errTooLarge):
		return c.errorItem(repo, key, ref, file, reasonFileTooLarge), nil
	case err != nil:
		return types.FetchedItem{}, err
	}
	title := repo.FullName + "/" + file
	// A file emptied by a commit still exists at this path, and ingestion reads
	// an item with no content and no URL as "nothing to sync" (see
	// datasource_service applyFetchedItem): the sync is acknowledged as
	// successful while the body indexed from an earlier commit stays fully
	// searchable. Nothing revisits it afterwards either: the next diffstat
	// reports no change for it.
	//
	// Emitting the file's path as its content replaces the stale body instead,
	// as the Notion, Confluence, IMA, GitLab and GitHub connectors do for the
	// same case. The check is on length alone, not on TrimSpace: a file that
	// holds only whitespace still carries content the repository has, and only
	// a zero-byte file has none.
	if len(body) == 0 {
		logger.Infof(ctx, "[Bitbucket] %s is empty, syncing its path only", title)
		body = []byte("# " + title + "\n")
	}
	// UpdatedAt is intentionally left unset: the file's last commit time is not
	// fetched here, and a fetch timestamp would be a fabricated source time.
	return types.FetchedItem{
		ExternalID:       c.externalID(key, ref, file),
		Title:            title,
		FileName:         repofile.KnowledgeRelativePath(repo.slug(), ref, file),
		Content:          body,
		ContentType:      "text/plain",
		SourceResourceID: key,
		Metadata: map[string]string{
			"channel":                   types.ConnectorTypeBitbucket,
			"source_type":               "bitbucket",
			"bitbucket_repository":      repo.FullName,
			"bitbucket_repository_uuid": repo.UUID,
			"bitbucket_ref":             ref,
			"bitbucket_path":            file,
			"bitbucket_url":             repo.Links.HTML.Href + "/src/" + ref + "/" + file,
		},
	}, nil
}

func (c *Connector) deleted(key, ref, file string) types.FetchedItem {
	return types.FetchedItem{
		ExternalID: c.externalID(key, ref, file),
		IsDeleted:  true,
		Metadata:   map[string]string{"channel": types.ConnectorTypeBitbucket, "bitbucket_path": file},
	}
}

// errorItem follows the service contract for failed fetches: no content, a
// stable reason code and English fallback text.
func (c *Connector) errorItem(repo *repository, key, ref, file, code string) types.FetchedItem {
	return types.FetchedItem{
		ExternalID: c.externalID(key, ref, file),
		Title:      repo.FullName + "/" + file,
		Metadata: map[string]string{
			"channel":           types.ConnectorTypeBitbucket,
			"source_type":       "bitbucket",
			"bitbucket_path":    file,
			"error":             SyncReasonTexts[code],
			"error_reason_code": code,
			"error_reason":      SyncReasonTexts[code],
		},
	}
}
