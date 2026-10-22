package service

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/utils"
)

// The mass-deletion guard holds back a run that would delete at least
// deletionGuardMaxCount items, or at least deletionGuardMinCount items that are
// also deletionGuardPercent of what the data source owns. 10% is the Algolia
// Crawler's default lost-records limit and 500 is Entra Connect's default
// deletion threshold; the floor lets a small source's routine deletions through.
const (
	deletionGuardMinCount = 10
	deletionGuardPercent  = 10
	deletionGuardMaxCount = 500
)

func deletionGuardTrips(stale, owned int) bool {
	return stale >= deletionGuardMaxCount ||
		(stale >= deletionGuardMinCount && stale*100 >= owned*deletionGuardPercent)
}

// deletionFailureNote is the sync-log suffix for failed deletions.
func deletionFailureNote(failed int) string {
	if failed == 0 {
		return ""
	}
	return fmt.Sprintf("; %d deletion(s) failed, the next sync retries them", failed)
}

// runDeletions collects one sync run's deletions so settleDeletions can guard
// and apply them as one batch after the listing ends.
type runDeletions struct {
	// pending maps each external ID to delete to whether this run emitted its
	// tombstone (already counted in SyncResult.Total) or an earlier run left it
	// pending. nil when deletion sync is off, so tombstones reach
	// applyFetchedItem, which ignores them.
	pending map[string]bool
	// seen holds the live external IDs the run emitted. A pending deletion of
	// one of them, or of a sub-item of one, is cancelled.
	seen map[string]struct{}
	// reconcile is set for a full sync whose connector can report a complete
	// listing: every owned item it does not see is stale.
	reconcile bool
	// held mirrors types.SyncCursor.DeletionsHeld.
	held bool
	// imports is stamped on every persisted cursor so the next run can tell
	// whether the knowledge base gained a capability since.
	imports utils.ImportCapabilities
}

// newRunDeletions starts from the deletions stored left pending.
func newRunDeletions(
	ds *types.DataSource, stored *types.SyncCursor, reconcile bool, imports utils.ImportCapabilities,
) *runDeletions {
	d := &runDeletions{imports: imports}
	if !ds.SyncDeletions {
		return d
	}
	d.pending = make(map[string]bool)
	if stored != nil {
		for _, id := range stored.PendingDeletions {
			d.pending[id] = false
		}
		d.held = stored.DeletionsHeld
	}
	d.seen = make(map[string]struct{})
	d.reconcile = reconcile
	return d
}

// take reports whether item was deferred: a tombstone waits for
// settleDeletions, and a live item cancels a pending deletion of its ID.
func (d *runDeletions) take(item *types.FetchedItem) bool {
	if d.pending == nil || item.ExternalID == "" {
		return false
	}
	if item.IsDeleted {
		d.pending[item.ExternalID] = true
		return true
	}
	delete(d.pending, item.ExternalID)
	d.seen[item.ExternalID] = struct{}{}
	return false
}

// withPending returns a copy of c, or an empty cursor, carrying the pending
// deletions. Persisting them in the same write as the connector's cursor means
// a crash can never keep a cursor past a tombstone while losing the tombstone.
func (d *runDeletions) withPending(c *types.SyncCursor) *types.SyncCursor {
	var out types.SyncCursor
	if c != nil {
		out = *c
	}
	out.PendingDeletions = slices.Sorted(maps.Keys(d.pending))
	out.DeletionsHeld = d.held && len(d.pending) > 0
	out.Imports = &d.imports
	return &out
}

// saveCursor sets ds.LastSyncCursor to next, or when the connector returned
// none to the cursor already stored, carrying the pending deletions.
func (d *runDeletions) saveCursor(ds *types.DataSource, next *types.SyncCursor) {
	if next == nil {
		next, _ = ds.ParseSyncCursor()
	}
	if next == nil && len(d.pending) == 0 {
		return
	}
	if cursorJSON, err := d.withPending(next).ToJSON(); err == nil {
		ds.LastSyncCursor = cursorJSON
	}
}

// isSeen reports whether externalID, or the parent of a sub-item built with
// types.SubtreeChildID, was emitted this run. A seen parent's sub-items are
// left to its subtree sweep.
func isSeen(seen map[string]struct{}, externalID string) bool {
	if _, ok := seen[externalID]; ok {
		return true
	}
	parent, ok := types.SubtreeParentID(externalID)
	if !ok {
		return false
	}
	_, ok = seen[parent]
	return ok
}

// settleDeletions applies, as one batch, every deletion the run collected:
// connector tombstones, deletions earlier runs left pending, and on a
// reconciling full sync whose listing was complete every owned item the run did
// not emit. Unless allowMass is set, an empty reconcile listing, a batch
// deletionGuardTrips flags, or any batch while an earlier one is held deletes
// nothing and stays pending, reported in result.Reconcile. Failed deletions stay pending too, so the next run retries
// them.
func (s *DataSourceService) settleDeletions(
	ctx context.Context, ds *types.DataSource, selection []string, next *types.SyncCursor,
	d *runDeletions, allowMass bool, result *types.SyncResult,
) {
	if d.pending == nil {
		return
	}
	reconcile := d.reconcile
	var incomplete *types.SyncItemError
	if reconcile && (next == nil || !next.CompleteListing) {
		reconcile = false
		incomplete = &types.SyncItemError{
			Code:    types.SyncCodeReconcileListingIncomplete,
			Message: "Deletions were not reconciled: the connector could not confirm a complete listing",
		}
	}
	if !reconcile && len(d.pending) == 0 {
		if incomplete != nil {
			result.Reconcile = &types.SyncReconcile{Held: incomplete}
		}
		return
	}
	// Owned is counted before anything is deleted, so the guard measures the
	// batch against the whole source. Only a reconcile reads every owned item;
	// other runs read just their pending ones.
	repo := s.knowledgeService.GetRepository()
	var owned []types.DataSourceItemRef
	var ownedCount int
	var err error
	if reconcile {
		owned, err = repo.ListDataSourceItems(ctx, ds.TenantID, ds.KnowledgeBaseID, ds.ID)
		ownedCount = len(owned)
	} else if ownedCount, err = repo.CountDataSourceItems(ctx, ds.TenantID, ds.KnowledgeBaseID, ds.ID); err == nil {
		owned, err = repo.ListDataSourceItems(ctx, ds.TenantID, ds.KnowledgeBaseID, ds.ID,
			slices.Collect(maps.Keys(d.pending))...)
	}
	if err != nil {
		logger.Errorf(ctx, "deletions for ds=%s left pending: list owned items: %v", ds.ID, err)
		return
	}
	report := &types.SyncReconcile{Held: incomplete}
	result.Reconcile = report
	tracksSource := datasource.HasCapability(ds.Type, datasource.CapabilitySelectionTracksSource)
	selected := make(map[string]bool, len(selection))
	for _, id := range selection {
		selected[id] = true
	}
	var doomed []types.DataSourceItemRef
	held := make(map[string]bool)
	for _, item := range owned {
		// This run's tombstone wins; a deletion left pending or a reconcile
		// candidate yields to a live emit of the item or of its parent.
		emitted, wasPending := d.pending[item.ExternalID]
		if !emitted && (isSeen(d.seen, item.ExternalID) || !wasPending && !reconcile) {
			continue
		}
		doomed = append(doomed, item)
		held[item.ExternalID] = emitted
		if tracksSource && item.SourceResourceID != "" && !selected[item.SourceResourceID] {
			report.Deselected++
		} else {
			report.Missing++
		}
	}
	// A tombstone for an item this data source does not own deletes nothing.
	for id, emitted := range d.pending {
		if _, ok := held[id]; !ok && emitted {
			result.Skipped++
		}
	}
	d.pending = held

	switch {
	case allowMass:
	case reconcile && len(d.seen) == 0 && ownedCount > 0:
		// An empty listing more likely means lost access than an emptied source.
		report.Held = &types.SyncItemError{
			Code:   types.SyncCodeReconcileListingEmpty,
			Params: map[string]string{"owned": strconv.Itoa(ownedCount)},
			Message: fmt.Sprintf("Deletions held back: the listing returned nothing while the data source "+
				"owns %d item(s)", ownedCount),
		}
	case len(doomed) > 0 && (d.held || deletionGuardTrips(len(doomed), ownedCount)):
		percent := len(doomed) * 100 / ownedCount
		report.Held = &types.SyncItemError{
			Code: types.SyncCodeDeletionGuardTripped,
			Params: map[string]string{
				"count": strconv.Itoa(len(doomed)), "owned": strconv.Itoa(ownedCount),
				"percent": strconv.Itoa(percent), "deselected": strconv.Itoa(report.Deselected),
			},
			Message: fmt.Sprintf("Deletions held back: %d item(s), %d%% of the %d this data source owns "+
				"(%d from resources removed from the selection); run Sync and apply deletions if this is expected",
				len(doomed), percent, ownedCount, report.Deselected),
		}
	}
	if report.Held != nil && report.Held != incomplete {
		d.held = true
		report.Overridable = true
		for _, item := range doomed[:min(len(doomed), maxSyncResultErrors)] {
			report.Pending = append(report.Pending, cmp.Or(item.Title, item.ExternalID))
		}
		logger.Warnf(ctx, "deletion guard held back %d of %d owned item(s) for ds=%s",
			len(doomed), ownedCount, ds.ID)
		return
	}
	d.held = false
	for _, item := range doomed {
		if err := ctx.Err(); err != nil {
			logger.Warnf(ctx, "deletions for ds=%s stopped, the rest stay pending: %v", ds.ID, err)
			return
		}
		if !d.pending[item.ExternalID] {
			result.Total++
		}
		failedBefore := result.DeletionFailed
		s.applyFetchedItem(withKBActivitySuppressed(ctx), ds, d.imports,
			&types.FetchedItem{ExternalID: item.ExternalID, Title: item.Title, IsDeleted: true}, nil, result)
		if result.DeletionFailed == failedBefore {
			delete(d.pending, item.ExternalID)
		}
	}
	logger.Infof(ctx, "deletions for ds=%s: %d owned item(s), %d deselected, %d missing",
		ds.ID, ownedCount, report.Deselected, report.Missing)
}

// runStatus folds a run's per-item failures and held deletions into its
// sync-log status and message.
func runStatus(status, msg string, result *types.SyncResult) (string, string) {
	add := func(part string) {
		status = types.SyncLogStatusPartial
		if msg != "" {
			msg += "; "
		}
		msg += part
	}
	if result.Failed > 0 {
		add(fmt.Sprintf("%d document(s) failed to sync", result.Failed) + deletionFailureNote(result.DeletionFailed))
	}
	if r := result.Reconcile; r != nil && r.Overridable {
		add(r.Held.Message)
	}
	return status, msg
}
