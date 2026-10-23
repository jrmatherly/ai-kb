package msgraph

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/utils"
)

const checkpointEvery = 50

// run is one FetchStream invocation's working state.
type run struct {
	c        *Connector
	cl       *client
	cfg      *config
	ds       *types.DataSourceConfig
	cur      *cursor
	h        datasource.StreamHandler
	roots    map[string]root // the selected roots by id
	rootIDs  []string        // the selected root ids, sorted, so ties resolve the same way every run
	limit    int64
	emitted  int
	fresh    bool // started from a nil cursor
	resumed  bool // some root had a page link in progress
	resynced bool // some root restarted after a 410
}

// FetchStream walks every selected root's delta stream, emitting changed
// files, tombstones and per-file failures, and checkpoints after every page
// and every checkpointEvery emits.
func (c *Connector) FetchStream(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor, h datasource.StreamHandler,
) (*types.SyncCursor, error) {
	return c.stream(ctx, ds, old, h, false)
}

// FetchFullStream re-fetches every file through the resync path: each root
// restarts its enumeration with a new generation, unchanged fingerprints no
// longer skip the download, and the end of the enumeration tombstones what
// it did not list. A checkpoint taken inside the full run resumes it.
func (c *Connector) FetchFullStream(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor, h datasource.StreamHandler,
) (*types.SyncCursor, error) {
	return c.stream(ctx, ds, old, h, true)
}

func (c *Connector) stream(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor, h datasource.StreamHandler, full bool,
) (*types.SyncCursor, error) {
	cfg, err := parseConfig(c.kind, ds)
	if err != nil {
		return nil, err
	}
	if len(cfg.Roots) == 0 {
		return nil, fmt.Errorf("%w: select at least one library or folder", datasource.ErrInvalidConfig)
	}
	cur, err := decodeCursor(old)
	if err != nil {
		return nil, err
	}
	r := &run{
		c:     c,
		cl:    c.newClient(cfg),
		cfg:   cfg,
		ds:    ds,
		cur:   cur,
		h:     h,
		roots: map[string]root{},
		limit: utils.GetMaxFileSize(),
		fresh: old == nil,
	}
	for _, rt := range cfg.Roots {
		r.roots[rt.id()] = rt
		r.rootIDs = append(r.rootIDs, rt.id())
	}
	sort.Strings(r.rootIDs)
	if err := r.dropDeselected(ctx); err != nil {
		return nil, err
	}
	// A resumed full attempt leaves every root alone: the finished ones keep
	// their cleared Full, the cut one keeps its Next and Gen. Resetting the
	// finished roots again would re-download them on every retry.
	fullFromScratch := full && !cur.fullRun()
	// Every selected root has a state before the retry pass, which may fetch
	// under a root that is new this run (an item re-homed by dropDeselected).
	for _, id := range r.rootIDs {
		st := r.cur.Roots[id]
		if st == nil {
			st = &rootState{}
			r.cur.Roots[id] = st
		}
		if fullFromScratch {
			st.Delta, st.Next, st.Resync, st.Full = "", "", true, true
			st.Gen++
		}
	}
	// Drive names for the knowledge path.
	for _, rt := range cfg.Roots {
		if _, ok := cur.Drives[rt.DriveID]; ok {
			continue
		}
		d, err := r.cl.drive(ctx, rt.DriveID)
		if err != nil {
			return nil, err
		}
		cur.Drives[rt.DriveID] = d.Name
	}
	// A from-scratch full run re-fetches every file anyway; a resumed one walks
	// only the cut root, so failures elsewhere still need the pass.
	if !fullFromScratch {
		if err := r.retryFailed(ctx); err != nil {
			return nil, err
		}
	}
	for _, id := range r.rootIDs {
		if err := r.walkRoot(ctx, r.roots[id]); err != nil {
			return nil, err
		}
	}
	out := cur.toSyncCursor()
	// Only an uninterrupted first run or from-scratch full run emitted every live
	// file, so only then may the service delete what it did not receive.
	out.CompleteListing = !r.resumed && (fullFromScratch || (r.fresh && !r.resynced))
	return out, nil
}

// dropDeselected tombstones what the roots that left the selection held,
// except items another selected root still covers, and forgets those roots.
func (r *run) dropDeselected(ctx context.Context) error {
	for id := range r.cur.Roots {
		if _, ok := r.roots[id]; ok {
			continue
		}
		for _, itemID := range r.cur.itemsUnderRoot(id) {
			if other := r.coveringRoot(itemID, r.cur.Items[itemID].Parent); other != "" {
				st := r.cur.Items[itemID]
				st.Root = other
				r.cur.Items[itemID] = st
				continue
			}
			if err := r.emitTombstone(ctx, itemID); err != nil {
				return err
			}
		}
		for fid, f := range r.cur.Folders {
			if f.Root != id {
				continue
			}
			if other := r.coveringRoot(fid, f.Parent); other != "" {
				f.Root = other
				r.cur.Folders[fid] = f
			} else {
				delete(r.cur.Folders, fid)
			}
		}
		delete(r.cur.Roots, id)
		if err := r.checkpoint(ctx); err != nil {
			return err
		}
	}
	return nil
}

// coveringRoot returns the first selected root, in id order, that still
// covers the entry, or "".
func (r *run) coveringRoot(id, parent string) string {
	for _, rootID := range r.rootIDs {
		if r.cur.underRoot(id, parent, r.roots[rootID]) {
			return rootID
		}
	}
	return ""
}

func (r *run) checkpoint(ctx context.Context) error {
	r.emitted = 0
	return r.h.Checkpoint(ctx, r.cur.toSyncCursor())
}

// emit hands one item to the service and checkpoints every 50 emits. The
// caller has already recorded the item's state, so the checkpoint is a
// faithful snapshot.
func (r *run) emit(ctx context.Context, item types.FetchedItem) error {
	if err := r.h.Emit(ctx, item); err != nil {
		return err
	}
	r.emitted++
	if r.emitted >= checkpointEvery {
		return r.checkpoint(ctx)
	}
	return nil
}

func (r *run) emitTombstone(ctx context.Context, id string) error {
	it, ok := r.cur.Items[id]
	if !ok {
		return nil
	}
	p := r.cur.path(id)
	delete(r.cur.Items, id)
	return r.emit(ctx, r.c.tombstone(id, it.Name, p))
}

// retryFailed re-fetches items whose last download failed for a transient
// reason; delta will not re-send an unchanged item, so this pass is the only
// retry they get. A deterministic failure (Failed set) waits for a change,
// and a 403 on the item itself becomes one; any other per-item error leaves
// the entry transient for the next run.
func (r *run) retryFailed(ctx context.Context) error {
	var failed []string
	for id, it := range r.cur.Items {
		if it.FP == "" && it.Failed == "" {
			failed = append(failed, id)
		}
	}
	sort.Strings(failed)
	for _, id := range failed {
		st := r.cur.Items[id]
		driveID, itemID, _ := strings.Cut(id, ":")
		item, err := r.cl.item(ctx, driveID, itemID)
		switch {
		case errors.Is(err, datasource.ErrResourceNotFound):
			if err := r.emitTombstone(ctx, id); err != nil {
				return err
			}
			continue
		case err != nil && !r.perItem(err):
			return err
		case err != nil:
			if errors.Is(err, datasource.ErrPermissionDenied) {
				st.Failed = reasonPermissionDenied
				r.cur.Items[id] = st
			}
			if err := r.emit(ctx, r.c.errorItem(id, st.Name, r.cur.path(id), downloadReason(err))); err != nil {
				return err
			}
			continue
		}
		if !item.isFile() || !r.ds.Importable(item.Name) {
			// An earlier version may be in the knowledge base; the service skips
			// a tombstone for an item it does not own.
			if err := r.emitTombstone(ctx, id); err != nil {
				return err
			}
			continue
		}
		if err := r.fetchFile(ctx, *item, st.Root); err != nil {
			return err
		}
	}
	if len(failed) > 0 {
		return r.checkpoint(ctx)
	}
	return nil
}

// perItem reports whether a fetch error is confined to one file. Credential,
// rate-limit, cancellation and transport failures affect the whole run and
// are not hidden in error items.
func (r *run) perItem(err error) bool {
	var limited *rateLimitError
	var api *apiError
	switch {
	case errors.As(err, &limited), errors.Is(err, datasource.ErrInvalidCredentials), errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return false
	case errors.As(err, &api):
		return api.status != http.StatusUnauthorized
	}
	return errors.Is(err, errEmptyFile) || errors.Is(err, errSourceChanged) || errors.Is(err, errTooLarge) ||
		errors.Is(
			err,
			errInsecureDownload,
		) || errors.Is(err, errNoRedirect) || errors.Is(err, datasource.ErrFetchFailed)
}

// walkRoot runs one root's delta stream from its stored position.
func (r *run) walkRoot(ctx context.Context, rt root) error {
	id := rt.id()
	st := r.cur.Roots[id]
	link := st.Next
	switch {
	case link != "":
		r.resumed = true
	case st.Delta != "":
		link = st.Delta
	default:
		link = r.cl.deltaStart(rt.DriveID, rt.ItemID)
	}
	visited := map[string]struct{}{}
	for link != "" {
		if _, dup := visited[link]; dup {
			return fmt.Errorf("microsoft graph delta for %s repeated a page link", id)
		}
		if len(visited) >= maxPaginationHops {
			return fmt.Errorf("microsoft graph delta for %s exceeded %d pages", id, maxPaginationHops)
		}
		visited[link] = struct{}{}
		st.Next = link
		page, err := r.cl.deltaPage(ctx, link)
		var resync *resyncError
		if errors.As(err, &resync) {
			logger.Warnf(ctx, "[MSGraph] delta token for %s expired; re-enumerating", id)
			st.Delta, st.Resync, r.resynced = "", true, true
			st.Gen++
			link = resync.Location
			continue
		}
		if err != nil {
			return err
		}
		if page.Next == "" && page.Delta == "" {
			return fmt.Errorf("%w: delta page for %s carried neither a next link nor a delta link",
				datasource.ErrFetchFailed, id)
		}
		if err := r.applyPage(ctx, rt, st, page.Items); err != nil {
			// No checkpoint here: item state is recorded before each emit, so a
			// snapshot now could claim an item the handler never received. The
			// last checkpoint's Next still points at this page.
			return err
		}
		if page.Delta != "" {
			if st.Resync {
				if err := r.tombstoneUnseen(ctx, id, st.Gen); err != nil {
					return err
				}
				st.Resync, st.Full = false, false
			}
			st.Delta, st.Next = page.Delta, ""
			link = ""
		} else {
			link = page.Next
			st.Next = link
		}
		if err := r.checkpoint(ctx); err != nil {
			return err
		}
	}
	return nil
}

// applyPage records the page's folders first (a folder may follow its files
// in the same page), then handles each item with the last occurrence of a
// repeated id winning.
func (r *run) applyPage(ctx context.Context, rt root, st *rootState, items []driveItem) error {
	rootID := rt.id()
	last := map[string]int{}
	for i, it := range items {
		last[it.ID] = i
	}
	for i, it := range items {
		if last[it.ID] != i || it.Folder == nil || it.Deleted != nil || it.RemoteItem != nil {
			continue
		}
		r.recordFolder(rt, st, it)
	}
	for i, it := range items {
		if last[it.ID] != i {
			continue
		}
		id := externalID(rt.DriveID, it.ID)
		switch {
		case it.Deleted != nil:
			if f, isFolder := r.cur.Folders[id]; isFolder {
				if f.Root != rootID {
					continue // another selected root owns it now (a move between roots)
				}
				for _, child := range r.cur.descendants(id) {
					if err := r.emitTombstone(ctx, child); err != nil {
						return err
					}
				}
				r.cur.dropSubtree(id)
				continue
			}
			if known, ok := r.cur.Items[id]; ok && known.Root != rootID {
				continue
			}
			if err := r.emitTombstone(ctx, id); err != nil {
				return err
			}
		case it.Folder != nil, it.Package != nil, it.RemoteItem != nil, it.File == nil:
			continue
		case !r.ds.Importable(it.Name):
			// A file renamed to an extension the knowledge base cannot import
			// leaves the index; one that was never imported leaves no trace.
			if err := r.emitTombstone(ctx, id); err != nil {
				return err
			}
		default:
			// A full sync re-fetches an unchanged file unless this run's
			// enumeration already did (a resumed attempt).
			known, ok := r.cur.Items[id]
			if ok && known.FP != "" && known.FP == fingerprint(it) && (!st.Full || known.Gen == st.Gen) {
				known.Gen, known.Root = st.Gen, rootID
				r.cur.Items[id] = known
				continue
			}
			if err := r.ensureAncestors(ctx, rt, st, it); err != nil {
				return err
			}
			if err := r.fetchFile(ctx, it, rootID); err != nil {
				return err
			}
		}
	}
	return nil
}

// recordFolder stores a folder for path building. The drive root item
// contributes no path segment, so it is stored with an empty name.
func (r *run) recordFolder(rt root, st *rootState, it driveItem) {
	parent, name := "", it.Name
	if it.ParentReference != nil && it.ParentReference.ID != "" {
		parent = externalID(rt.DriveID, it.ParentReference.ID)
	}
	if it.Root != nil || parent == "" {
		name = ""
	}
	r.cur.Folders[externalID(rt.DriveID, it.ID)] = folderState{Parent: parent, Name: name, Root: rt.id(), Gen: st.Gen}
}

// ensureAncestors walks from a file up to its root through the folder map,
// fetching every folder no delta page has delivered yet (a parent, or a
// grandparent, can arrive on a later page), so the knowledge path is complete
// when the file is emitted. The walk ends at the selected folder, whose own
// name belongs in the path, or at the drive root.
func (r *run) ensureAncestors(ctx context.Context, rt root, st *rootState, it driveItem) error {
	if it.ParentReference == nil {
		return nil
	}
	parentItem := it.ParentReference.ID
	for hops := 0; parentItem != "" && hops < maxPaginationHops; hops++ {
		parentID := externalID(rt.DriveID, parentItem)
		f, known := r.cur.Folders[parentID]
		if !known {
			folder, err := r.cl.item(ctx, rt.DriveID, parentItem)
			if err != nil {
				return err
			}
			r.recordFolder(rt, st, *folder)
			f = r.cur.Folders[parentID]
		}
		if parentItem == rt.ItemID || f.Parent == "" {
			return nil
		}
		_, parentItem, _ = strings.Cut(f.Parent, ":")
	}
	return nil
}

// tombstoneUnseen ends a resync: every known item under the root that the
// fresh enumeration did not stamp with the current generation is gone.
func (r *run) tombstoneUnseen(ctx context.Context, rootID string, gen int) error {
	for _, id := range r.cur.itemsUnderRoot(rootID) {
		if r.cur.Items[id].Gen == gen {
			continue
		}
		if err := r.emitTombstone(ctx, id); err != nil {
			return err
		}
	}
	for id, f := range r.cur.Folders {
		if f.Root == rootID && f.Gen != gen {
			delete(r.cur.Folders, id)
		}
	}
	return nil
}

// fetchFile downloads one file and emits it, or emits an error item. A
// deterministic failure (malware, too large, empty) keeps the fingerprint and
// waits for the file to change; a transient one leaves FP empty for the retry pass.
func (r *run) fetchFile(ctx context.Context, it driveItem, rootID string) error {
	id := externalID(driveOf(rootID), it.ID)
	parent := ""
	if it.ParentReference != nil && it.ParentReference.ID != "" {
		parent = externalID(driveOf(rootID), it.ParentReference.ID)
	}
	gen := r.cur.Roots[rootID].Gen
	state := itemState{Parent: parent, Name: it.Name, Root: rootID, Gen: gen}
	r.cur.Items[id] = state
	p := r.cur.path(id)
	fail := func(code string, permanent bool) error {
		if permanent {
			state.FP, state.Failed = fingerprint(it), code
			r.cur.Items[id] = state
		}
		return r.emit(ctx, r.c.errorItem(id, it.Name, p, code))
	}
	if it.Malware != nil {
		return fail(reasonMalware, true)
	}
	if it.Size > r.limit {
		return fail(reasonFileTooLarge, true)
	}
	body, err := r.cl.download(ctx, driveOf(rootID), it.ID, it.Size, r.limit)
	if err != nil {
		if !r.perItem(err) {
			return err
		}
		return fail(downloadReason(err), errors.Is(err, errEmptyFile) || errors.Is(err, errTooLarge))
	}
	m := r.c.metadata(id, p)
	m["msgraph_web_url"] = it.WebURL
	state.FP = fingerprint(it)
	r.cur.Items[id] = state
	item := types.FetchedItem{
		ExternalID: id, Title: it.Name, FileName: p, Content: body, ContentType: http.DetectContentType(body),
		URL: it.WebURL, UpdatedAt: it.LastModified, SourceResourceID: rootID, Metadata: m,
	}
	return r.emit(ctx, item)
}

func downloadReason(err error) string {
	switch {
	case errors.Is(err, errEmptyFile):
		return reasonEmptyFile
	case errors.Is(err, errSourceChanged):
		return reasonSourceChanged
	case errors.Is(err, errTooLarge):
		return reasonFileTooLarge
	case errors.Is(err, datasource.ErrPermissionDenied):
		return reasonPermissionDenied
	case errors.Is(err, datasource.ErrResourceNotFound):
		return reasonNotFound
	case errors.Is(err, errInsecureDownload), errors.Is(err, errNoRedirect):
		return reasonInvalidResponse
	}
	return reasonFetchFailed
}
