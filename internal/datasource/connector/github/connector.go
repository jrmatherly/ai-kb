package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/datasource/connector/repofile"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
)

var _ datasource.StreamingConnector = (*Connector)(nil)

const checkpointEvery = 50

const reasonFileTooLarge = "github_file_too_large"

// SyncReasonTexts maps every reason code to its English fallback.
var SyncReasonTexts = map[string]string{
	reasonFileTooLarge: "The GitHub file exceeds the 100 MB API limit",
}

// Connector syncs document files from selected GitHub repositories.
type Connector struct {
	client *client
}

// NewConnector creates a stateless connector. Each data source provides its
// own GitHub host and access token in its encrypted credentials.
func NewConnector() *Connector { return &Connector{} }

// Type returns the connector type.
func (c *Connector) Type() string { return types.ConnectorTypeGitHub }

func (c *Connector) configured(ds *types.DataSourceConfig) (*Connector, error) {
	if ds == nil {
		return nil, datasource.ErrInvalidConfig
	}
	baseURL, _ := ds.Credentials["base_url"].(string)
	token, _ := ds.Credentials["access_token"].(string)
	client, err := newClient(baseURL, token)
	if err != nil {
		return nil, err
	}
	return &Connector{client: client}, nil
}

// Validate checks the credentials against the API and the selection syntax.
func (c *Connector) Validate(ctx context.Context, ds *types.DataSourceConfig) error {
	configured, err := c.configured(ds)
	if err != nil {
		return err
	}
	if err := configured.client.ping(ctx); err != nil {
		return err
	}
	if _, ok := ds.Settings["repositories"]; ok {
		if _, err := parseConfig(ds); err != nil {
			return err
		}
	}
	return nil
}

// ListResources lists the repositories the token can see, or the
// subdirectories of parent ("owner/repo[:dir]") on the default branch.
func (c *Connector) ListResources(
	ctx context.Context, ds *types.DataSourceConfig, parent string,
) ([]types.Resource, error) {
	var err error
	if c, err = c.configured(ds); err != nil {
		return nil, err
	}
	if parent == "" {
		repos, err := c.client.repositories(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]types.Resource, 0, len(repos))
		for _, r := range repos {
			out = append(out, types.Resource{
				ExternalID: strings.ToLower(r.FullName), Name: r.FullName, Type: "repository", URL: r.HTMLURL,
				HasChildren: true,
			})
		}
		return out, nil
	}
	key, dir, _ := strings.Cut(parent, ":")
	repo, err := c.client.repository(ctx, key)
	if err != nil {
		return nil, err
	}
	t, err := c.client.tree(ctx, key, repo.DefaultBranch, false)
	if err != nil {
		return nil, err
	}
	if dir != "" {
		for _, segment := range strings.Split(dir, "/") {
			i := slices.IndexFunc(t.Entries, func(e treeEntry) bool { return e.Type == "tree" && e.Path == segment })
			if i < 0 {
				return nil, fmt.Errorf("%w: directory %s in %s", datasource.ErrResourceNotFound, dir, key)
			}
			if t, err = c.client.tree(ctx, key, t.Entries[i].SHA, false); err != nil {
				return nil, err
			}
		}
	}
	out := make([]types.Resource, 0, len(t.Entries))
	for _, e := range t.Entries {
		if e.Type != "tree" {
			continue
		}
		p := e.Path
		if dir != "" {
			p = dir + "/" + e.Path
		}
		out = append(out, types.Resource{
			ExternalID: key + ":" + p, Name: e.Path, Type: "directory", ParentID: parent, HasChildren: true,
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
// head alone. The previous snapshot is rebuilt from the recorded ref and paths,
// so a changed ref tombstones every old-ref item and a changed directory list
// emits exactly the added and removed files. Pending marks a repository whose
// ordered diff was partly emitted, so a run cut off by the hourly rate limit
// continues where it stopped instead of restarting.
type repositoryCursor struct {
	Head    string         `json:"head"`
	Ref     string         `json:"ref"`
	Paths   []string       `json:"paths"`
	Pending *pendingCursor `json:"pending,omitempty"`
}

// A pendingCursor records the head, ref and paths the interrupted diff was
// computed against, because the diff's order (and so what Done counts) depends
// on all three. A resumed sync with the same selection recomputes the same
// diff and skips the first Done entries. A selection changed in between is
// reconciled first: the items the interrupted run emitted under the pending
// selection are the only record of them (Head, Ref and Paths still describe
// the last completed selection), so the resume rebuilds that prefix and
// tombstones what the new selection does not own before emitting its own diff
// from the start.
type pendingCursor struct {
	Head  string   `json:"head"`
	Ref   string   `json:"ref"`
	Paths []string `json:"paths"`
	Done  int      `json:"done"`
}

type cursor struct {
	Repositories map[string]repositoryCursor `json:"repositories"`
}

// readCursor decodes the previous cursor. A repository value that is not an
// object (an older cursor shape) counts as no previous head.
func readCursor(old *types.SyncCursor) cursor {
	out := cursor{Repositories: map[string]repositoryCursor{}}
	if old == nil {
		return out
	}
	b, _ := json.Marshal(old.ConnectorCursor["repositories"])
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	for key, v := range raw {
		var rc repositoryCursor
		if json.Unmarshal(v, &rc) == nil {
			out.Repositories[key] = rc
		}
	}
	return out
}

func gitHubCursor(value cursor, complete bool) *types.SyncCursor {
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
	next := readCursor(old)
	for _, selection := range cfg.Repositories {
		if err := c.streamRepository(ctx, selection, next, h); err != nil {
			return nil, err
		}
	}
	return gitHubCursor(next, complete), nil
}

func (c *Connector) streamRepository(
	ctx context.Context, selection repositorySelection, next cursor, h datasource.StreamHandler,
) error {
	key := selection.Repository
	repo, err := c.client.repository(ctx, key)
	if err != nil {
		return err
	}
	ref := selection.Ref
	if ref == "" {
		ref = repo.DefaultBranch
	}
	head, err := c.client.commitSHA(ctx, key, ref)
	if err != nil {
		return err
	}
	rc := next.Repositories[key]
	done := repositoryCursor{Head: head, Ref: ref, Paths: selection.Paths}
	if rc.Head == head && rc.Ref == ref && slices.Equal(rc.Paths, selection.Paths) && rc.Pending == nil {
		next.Repositories[key] = done
		return h.Checkpoint(ctx, gitHubCursor(next, false))
	}
	snap, err := c.snapshot(ctx, key, head, selection.Paths)
	if err != nil {
		return err
	}
	prev, err := c.baseline(ctx, key, rc)
	if err != nil {
		return err
	}
	changes := diff(prev, snap, rc.Ref == ref)
	start := 0
	if p := rc.Pending; p != nil {
		if p.Head == head && p.Ref == ref && slices.Equal(p.Paths, selection.Paths) {
			start = p.Done
		} else {
			tombstones, err := c.supersededPendingTombstones(ctx, key, rc, prev, ref, snap)
			if err != nil {
				return err
			}
			for _, item := range tombstones {
				if err := h.Emit(ctx, item); err != nil {
					return err
				}
			}
		}
	}
	for i := start; i < len(changes); i++ {
		ch := changes[i]
		var item types.FetchedItem
		switch {
		case ch.Deleted:
			item = c.deleted(key, rc.Ref, ch.Path)
		case snap[ch.Path].Size > c.client.rawLimit:
			item = c.errorItem(repo, key, ref, ch.Path, reasonFileTooLarge)
		default:
			if item, err = c.item(ctx, repo, key, ref, ch.Path, snap[ch.Path].SHA); err != nil {
				return err
			}
		}
		if err := h.Emit(ctx, item); err != nil {
			return err
		}
		if n := i + 1; n%checkpointEvery == 0 {
			rc.Pending = &pendingCursor{Head: head, Ref: ref, Paths: selection.Paths, Done: n}
			next.Repositories[key] = rc
			if err := h.Checkpoint(ctx, gitHubCursor(next, false)); err != nil {
				return err
			}
		}
	}
	next.Repositories[key] = done
	return h.Checkpoint(ctx, gitHubCursor(next, false))
}

// baseline is the snapshot at the recorded head, or nil when there is none or
// its tree is gone (a force-push whose old commit was garbage-collected), in
// which case the whole new snapshot is emitted with no deletions.
func (c *Connector) baseline(ctx context.Context, key string, rc repositoryCursor) (snapshot, error) {
	if rc.Head == "" {
		return nil, nil
	}
	prev, err := c.snapshot(ctx, key, rc.Head, rc.Paths)
	if errors.Is(err, errMissingCommit) {
		logger.Infof(ctx, "[GitHub] %s: previous head %s is gone, emitting the whole snapshot", key, rc.Head)
		return nil, nil
	}
	return prev, err
}

// supersededPendingTombstones reconstructs the items an interrupted run emitted
// under rc.Pending's selection and tombstones those the current run (ref, snap)
// will not own: an item is kept only when it is at the same ref and still in
// the new snapshot. A path the baseline already holds under the pending ref is
// left to the new diff, which deletes it under that same ref.
func (c *Connector) supersededPendingTombstones(
	ctx context.Context, key string, rc repositoryCursor, prev snapshot, ref string, snap snapshot,
) ([]types.FetchedItem, error) {
	p := rc.Pending
	pendingSnap, err := c.snapshot(ctx, key, p.Head, p.Paths)
	if errors.Is(err, errMissingCommit) {
		logger.Infof(ctx, "[GitHub] %s: pending head %s is gone, nothing to reconcile", key, p.Head)
		pendingSnap = nil
	} else if err != nil {
		return nil, err
	}
	emitted := diff(prev, pendingSnap, rc.Ref == p.Ref)
	emitted = emitted[:min(p.Done, len(emitted))]
	var out []types.FetchedItem
	for _, ch := range emitted {
		if ch.Deleted {
			continue
		}
		if _, inNew := snap[ch.Path]; inNew && p.Ref == ref {
			continue
		}
		if _, inBaseline := prev[ch.Path]; inBaseline && rc.Ref == p.Ref {
			continue
		}
		out = append(out, c.deleted(key, p.Ref, ch.Path))
	}
	return out, nil
}

func (c *Connector) externalID(key, ref, file string) string {
	return fmt.Sprintf("github:%s:%s:%s:%s", c.client.baseURL, key, ref, file)
}

func (c *Connector) item(
	ctx context.Context, repo *repository, key, ref, file, sha string,
) (types.FetchedItem, error) {
	body, err := c.client.blob(ctx, key, sha)
	if err != nil {
		return types.FetchedItem{}, err
	}
	title := repo.FullName + "/" + file
	// A file emptied by a commit still exists at this path, and ingestion reads
	// an item with no content and no URL as "nothing to sync" (see
	// datasource_service applyFetchedItem): the sync is acknowledged as
	// successful while the body indexed from an earlier commit stays fully
	// searchable. Nothing revisits it afterwards either: the next snapshot diff
	// sees the same empty blob at both heads.
	//
	// Emitting the file's path as its content replaces the stale body instead,
	// as the Notion, Confluence, IMA and GitLab connectors do for the same
	// case. The check is on length alone, not on TrimSpace: a blob that holds
	// only whitespace still carries content the repository has, and only a
	// zero-byte blob has none.
	if len(body) == 0 {
		logger.Infof(ctx, "[GitHub] %s is empty, syncing its path only", title)
		body = []byte("# " + title + "\n")
	}
	// UpdatedAt is intentionally left unset: the file's last commit time is not
	// fetched here, and a fetch timestamp would be a fabricated source time.
	return types.FetchedItem{
		ExternalID:       c.externalID(key, ref, file),
		Title:            title,
		FileName:         repofile.KnowledgeRelativePath(repo.Name, ref, file),
		Content:          body,
		ContentType:      "text/plain",
		SourceResourceID: key,
		Metadata: map[string]string{
			"channel":              types.ConnectorTypeGitHub,
			"source_type":          "github",
			"github_repository":    repo.FullName,
			"github_repository_id": strconv.FormatInt(repo.ID, 10),
			"github_ref":           ref,
			"github_path":          file,
			"github_url":           repo.HTMLURL + "/blob/" + ref + "/" + file,
		},
	}, nil
}

func (c *Connector) deleted(key, ref, file string) types.FetchedItem {
	return types.FetchedItem{
		ExternalID: c.externalID(key, ref, file),
		IsDeleted:  true,
		Metadata:   map[string]string{"channel": types.ConnectorTypeGitHub, "github_path": file},
	}
}

// errorItem follows the service contract for failed fetches: no content, a
// stable reason code and English fallback text.
func (c *Connector) errorItem(repo *repository, key, ref, file, code string) types.FetchedItem {
	return types.FetchedItem{
		ExternalID: c.externalID(key, ref, file),
		Title:      repo.FullName + "/" + file,
		Metadata: map[string]string{
			"channel":           types.ConnectorTypeGitHub,
			"source_type":       "github",
			"github_path":       file,
			"error":             SyncReasonTexts[code],
			"error_reason_code": code,
			"error_reason":      SyncReasonTexts[code],
		},
	}
}
