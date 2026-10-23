package bitbucket

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/datasource/connector/repofile"
	"github.com/jrmatherly/ai-kb/internal/logger"
)

// files is the set of syncable files at one commit, path to size: the in-scope
// commit_file entries the knowledge base can import that are not a symlink, a
// submodule or an LFS pointer.
//
// Bitbucket exposes no blob id, so GitHub's snapshot diff is impossible, and a
// diffstat carries no sizes or attributes, so it cannot be the only source
// either. Every run that has work to do lists each selected root at the new
// head; the listing is the only source of candidates, so size and attribute
// checks happen before any fetch in every mode, and the diffstat only decides
// which of those candidates changed.
type files map[string]int64

type change struct {
	Path    string
	Deleted bool
}

var (
	errMissingCommit = errors.New("bitbucket commit is gone")
	errDepthExceeded = errors.New("bitbucket listing reached its depth limit")
)

func (f files) add(e srcEntry, roots []string, importable func(string) bool) {
	if e.Type == "commit_file" && repofile.InScope(e.Path, roots) && importable(e.Path) &&
		!e.has("link") && !e.has("subrepository") && !e.has("lfs") {
		f[e.Path] = e.Size
	}
}

// depthBelow is the number of path segments of p below dir.
func depthBelow(p, dir string) int {
	if dir != "" {
		p = strings.TrimPrefix(p, dir+"/")
	}
	return strings.Count(p, "/") + 1
}

// files lists every root of commit. A root that is 404 at an existing commit
// is empty; a 404 on the whole tree, or on a root when the commit itself is
// gone, is errMissingCommit. A listing that times out is rebuilt one directory
// at a time. A directory at a listing's depth limit is errDepthExceeded: the
// listing silently stops there, and a listing missing files would tombstone them.
func (c *Connector) files(ctx context.Context, key, commit string, roots []string) (files, error) {
	out := files{}
	dirs := roots
	if len(dirs) == 0 {
		dirs = []string{""}
	}
	for _, dir := range dirs {
		entries, err := c.client.listing(ctx, key, commit, dir, listingDepth)
		switch {
		case errors.Is(err, errTimedOut):
			walked, err := c.walk(ctx, key, commit, dir, roots)
			if err != nil {
				return nil, err
			}
			maps.Copy(out, walked)
			continue
		case errors.Is(err, datasource.ErrResourceNotFound):
			if dir == "" {
				return nil, fmt.Errorf("%w: %w", errMissingCommit, err)
			}
			exists, err := c.client.commitExists(ctx, key, commit)
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, fmt.Errorf("%w: %s at %s", errMissingCommit, key, commit)
			}
			continue
		case err != nil:
			return nil, err
		}
		for _, e := range entries {
			if e.Type == "commit_directory" && depthBelow(e.Path, dir) >= listingDepth {
				return nil, fmt.Errorf("%w: %s at %s, directory %q", errDepthExceeded, key, commit, e.Path)
			}
			out.add(e, roots, c.importable)
		}
	}
	return out, nil
}

// walk lists dir with the shallower depth and descends into each directory at
// that depth limit that leads to or lies inside a root. A timeout here is
// returned: a single directory that cannot be listed fails the sync.
func (c *Connector) walk(ctx context.Context, key, commit, dir string, roots []string) (files, error) {
	entries, err := c.client.listing(ctx, key, commit, dir, fallbackListingDepth)
	if err != nil {
		return nil, err
	}
	out := files{}
	for _, e := range entries {
		if e.Type == "commit_directory" {
			if depthBelow(e.Path, dir) >= fallbackListingDepth && leadsToRoot(e.Path, roots) {
				sub, err := c.walk(ctx, key, commit, e.Path, roots)
				if err != nil {
					return nil, err
				}
				maps.Copy(out, sub)
			}
			continue
		}
		out.add(e, roots, c.importable)
	}
	return out, nil
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

// changeList is what a sync from base to head with roots must emit: tombstones
// sorted by path, then items sorted by path. The order is what makes a pending
// offset valid across runs.
//
// Under the same ref the diffstat head..base (Bitbucket's new..old direction)
// decides: its removed entries in the recorded scope that the new listing lacks
// are tombstones, its added and modified entries present in the new listing are
// items, a root that left the selection tombstones its recorded listing and a
// root that entered it emits its new listing. A first sync, a changed ref and a
// gone recorded head are the degenerate cases with an empty intersection: every
// recorded path (under the recorded ref) is a tombstone and every new path an
// item. A diffstat that times out or whose commits are gone makes the change set
// unknown: every new path is an item and the recorded listing, where readable,
// supplies the tombstones. baselineGone reports a recorded head that no longer
// exists, so a resumed pending run restarts at zero.
func (c *Connector) changeList(
	ctx context.Context, key string, base repositoryCursor, head, ref string, roots []string, next files,
) (list []change, baselineGone bool, err error) {
	tombstones, items := map[string]bool{}, map[string]bool{}
	for p := range next {
		items[p] = true
	}
	baseline := func() (files, error) {
		prev, err := c.files(ctx, key, base.Head, base.Paths)
		if errors.Is(err, errMissingCommit) {
			logger.Infof(ctx, "[Bitbucket] %s: previous head %s is gone, emitting the whole listing", key, base.Head)
			return nil, nil
		}
		return prev, err
	}
	switch {
	case base.Head == "":
	case base.Ref != ref:
		prev, err := baseline()
		if err != nil {
			return nil, false, err
		}
		baselineGone = prev == nil
		for p := range prev {
			tombstones[p] = true
		}
	default:
		entries, err := c.client.diffstat(ctx, key, head, base.Head)
		unknown := errors.Is(err, errTimedOut) || errors.Is(err, datasource.ErrResourceNotFound)
		if err != nil && !unknown {
			return nil, false, err
		}
		baselineGone = errors.Is(err, datasource.ErrResourceNotFound)
		var prev files
		if (unknown && !baselineGone) || !slices.Equal(base.Paths, roots) {
			if prev, err = baseline(); err != nil {
				return nil, false, err
			}
			if prev == nil {
				baselineGone, unknown = true, true
			}
		}
		if unknown {
			for p := range prev {
				if _, ok := next[p]; !ok {
					tombstones[p] = true
				}
			}
			break
		}
		clear(items)
		for _, e := range entries {
			if e.New != nil && (e.Status == "added" || e.Status == "modified") {
				if _, ok := next[e.New.Path]; ok {
					items[e.New.Path] = true
					continue
				}
			}
			// A removed file, or a modified one the new listing no longer
			// offers (it gained the lfs or link attribute, or its extension
			// stopped being importable), leaves the index.
			if e.Old == nil || (e.Status != "removed" && e.Status != "modified") {
				continue
			}
			p := e.Old.Path
			if _, ok := next[p]; !ok && repofile.InScope(p, base.Paths) && c.importable(p) {
				tombstones[p] = true
			}
		}
		for p := range prev {
			if !repofile.InScope(p, roots) {
				tombstones[p] = true
			}
		}
		for p := range next {
			if !repofile.InScope(p, base.Paths) {
				items[p] = true
			}
		}
	}
	list = make([]change, 0, len(tombstones)+len(items))
	for _, p := range slices.Sorted(maps.Keys(tombstones)) {
		list = append(list, change{Path: p, Deleted: true})
	}
	for _, p := range slices.Sorted(maps.Keys(items)) {
		list = append(list, change{Path: p})
	}
	return list, baselineGone, nil
}
