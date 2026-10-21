package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/datasource/connector/repofile"
)

// A snapshot is the set of syncable files at one commit, keyed by repository
// path. GitHub has no straight compare between two commits (compare is a
// merge-base comparison capped at 300 files), so an incremental sync takes the
// snapshot at the recorded head and at the new head and diffs them locally.
// A force-push or revert then emits exactly the files whose content differs
// between the two heads, and a rename is a deletion plus an addition. Only
// regular blobs (mode 100644 or 100755) in scope with a supported extension are
// kept; symlinks and submodules are skipped.
type snapshot map[string]blob

type blob struct {
	SHA  string
	Size int64
}

type change struct {
	Path    string
	Deleted bool
}

var errMissingCommit = errors.New("github commit tree is gone")

// snapshot reads the recursive tree of commit, or walks its subtrees one call
// at a time when the API truncates the listing. A 404 or 422 is reported as
// errMissingCommit: the recorded head was garbage-collected after a force-push.
func (c *Connector) snapshot(ctx context.Context, key, commit string, roots []string) (snapshot, error) {
	t, err := c.client.tree(ctx, key, commit, true)
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) &&
			(apiErr.status == http.StatusNotFound || apiErr.status == http.StatusUnprocessableEntity) {
			return nil, fmt.Errorf("%w: %w", errMissingCommit, err)
		}
		return nil, err
	}
	if t.Truncated {
		return c.walkSubtrees(ctx, key, t.SHA, roots)
	}
	snap := snapshot{}
	for _, e := range t.Entries {
		snap.add(e.Path, e, roots)
	}
	return snap, nil
}

func (s snapshot) add(path string, e treeEntry, roots []string) {
	if e.Type == "blob" && (e.Mode == "100644" || e.Mode == "100755") &&
		repofile.InScope(path, roots) && repofile.IsSupported(path) {
		s[path] = blob{SHA: e.SHA, Size: e.Size}
	}
}

// walkSubtrees descends from the root tree through the non-recursive tree
// endpoint, entering only subtrees that lead to or lie inside a root.
func (c *Connector) walkSubtrees(ctx context.Context, key, rootTreeSHA string, roots []string) (snapshot, error) {
	snap := snapshot{}
	var walk func(dir, sha string) error
	walk = func(dir, sha string) error {
		t, err := c.client.tree(ctx, key, sha, false)
		if err != nil {
			return err
		}
		for _, e := range t.Entries {
			p := e.Path
			if dir != "" {
				p = dir + "/" + e.Path
			}
			if e.Type == "tree" {
				if leadsToRoot(p, roots) {
					if err := walk(p, e.SHA); err != nil {
						return err
					}
				}
				continue
			}
			snap.add(p, e, roots)
		}
		return nil
	}
	return snap, walk("", rootTreeSHA)
}

// leadsToRoot reports whether dir is inside a root or a root is inside dir.
func leadsToRoot(dir string, roots []string) bool {
	if repofile.InScope(dir, roots) {
		return true
	}
	for _, r := range roots {
		if strings.HasPrefix(r, dir+"/") {
			return true
		}
	}
	return false
}

// diff lists what changed from prev to next: deletions sorted by path, then
// changed or added files sorted by path. With sameRef false every file of prev
// is deleted and every file of next is added, since the ref is part of the
// item identity.
func diff(prev, next snapshot, sameRef bool) []change {
	var deleted, changed []string
	for p := range prev {
		if _, ok := next[p]; !sameRef || !ok {
			deleted = append(deleted, p)
		}
	}
	for p, b := range next {
		if o, ok := prev[p]; !sameRef || !ok || o.SHA != b.SHA {
			changed = append(changed, p)
		}
	}
	sort.Strings(deleted)
	sort.Strings(changed)
	out := make([]change, 0, len(deleted)+len(changed))
	for _, p := range deleted {
		out = append(out, change{Path: p, Deleted: true})
	}
	for _, p := range changed {
		out = append(out, change{Path: p})
	}
	return out
}
