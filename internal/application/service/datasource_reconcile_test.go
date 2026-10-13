package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reconcileConnectorType borrows a type that declares
// datasource.CapabilityFullSyncReconcile.
const reconcileConnectorType = types.ConnectorTypeDingTalk

// reconcileConnector is a batch connector whose syncs return fixed items and
// report the given listing completeness. incompleteConfig makes it, like
// Confluence Cloud, unable to report a complete listing under its config.
type reconcileConnector struct {
	items            []types.FetchedItem
	complete         bool
	incompleteConfig bool
}

func (reconcileConnector) Type() string { return reconcileConnectorType }

func (c reconcileConnector) ReportsCompleteListing(*types.DataSourceConfig) bool {
	return !c.incompleteConfig
}

func (reconcileConnector) Validate(context.Context, *types.DataSourceConfig) error { return nil }

func (reconcileConnector) ListResources(context.Context, *types.DataSourceConfig, string) ([]types.Resource, error) {
	return nil, nil
}

func (reconcileConnector) ResolveResourceAncestors(
	context.Context, *types.DataSourceConfig, []string,
) ([]string, error) {
	return nil, nil
}

func (c reconcileConnector) FetchAll(context.Context, *types.DataSourceConfig, []string) ([]types.FetchedItem, error) {
	return c.items, nil
}

func (c reconcileConnector) FetchAllFromCursor(
	context.Context, *types.DataSourceConfig, []string, *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	return c.items, &types.SyncCursor{CompleteListing: c.complete}, nil
}

func (c reconcileConnector) FetchIncremental(
	context.Context, *types.DataSourceConfig, *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	return c.items, &types.SyncCursor{CompleteListing: c.complete}, nil
}

// reconcileStreamConnector streams the same items, checkpointing after each
// tombstone the way Confluence drops it from its cursor; fail ends the stream
// with that error after every item was emitted.
type reconcileStreamConnector struct {
	reconcileConnector
	fail error
}

func (c reconcileStreamConnector) FetchStream(
	ctx context.Context, _ *types.DataSourceConfig, _ *types.SyncCursor, h datasource.StreamHandler,
) (*types.SyncCursor, error) {
	for i, item := range c.items {
		if err := h.Emit(ctx, item); err != nil {
			return nil, err
		}
		if item.IsDeleted {
			checkpoint := &types.SyncCursor{ConnectorCursor: map[string]any{"emitted": i + 1}}
			if err := h.Checkpoint(ctx, checkpoint); err != nil {
				return nil, err
			}
		}
	}
	if c.fail != nil {
		return nil, c.fail
	}
	return &types.SyncCursor{ConnectorCursor: map[string]any{"emitted": len(c.items)}, CompleteListing: c.complete}, nil
}

func streamOf(items []types.FetchedItem, fail error) reconcileStreamConnector {
	return reconcileStreamConnector{reconcileConnector: reconcileConnector{items: items}, fail: fail}
}

// reconcileRepo holds live knowledge per data source and external ID. sources
// maps a ds-a external ID to the source_resource_id ListDataSourceItems reports.
type reconcileRepo struct {
	interfaces.KnowledgeRepository
	live    map[string]map[string]*types.Knowledge
	sources map[string]string
}

func (r *reconcileRepo) ListDataSourceItems(
	_ context.Context, _ uint64, _, dsID string,
) ([]types.DataSourceItemRef, error) {
	items := make([]types.DataSourceItemRef, 0, len(r.live[dsID]))
	for id, k := range r.live[dsID] {
		items = append(items, types.DataSourceItemRef{ExternalID: id, Title: k.Title, SourceResourceID: r.sources[id]})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ExternalID < items[j].ExternalID })
	return items, nil
}

func (r *reconcileRepo) FindByDataSourceExternalID(
	_ context.Context, _ uint64, _, dsID, externalID string,
) (*types.Knowledge, error) {
	return r.live[dsID][externalID], nil
}

func (r *reconcileRepo) HardDeleteKnowledge(_ context.Context, _ uint64, id string) error {
	for _, items := range r.live {
		for ext, k := range items {
			if k.ID == id {
				delete(items, ext)
			}
		}
	}
	return nil
}

// reconcileSelection is ds-a's selected resource. Items stored under any other
// resource ID were deselected.
const reconcileSelection = "res-1"

type reconcileCase struct {
	syncDeletions bool
	forceFull     bool
	complete      bool
	emitNothing   bool
	deleteErr     error
	allowMass     bool
	// kept adds owned items the connector still emits; missing adds stale items
	// of the selected resource; deselected adds stale items of a resource no
	// longer selected; tombstones adds items of the selected resource the
	// connector reports deleted.
	kept, missing, deselected, tombstones int
}

// reconcileHarness is data source ds-a in a knowledge base that also holds
// items of ds-b. Its store persists across syncs, as does ds-a's cursor.
type reconcileHarness struct {
	ds   *types.DataSource
	repo *reconcileRepo
	ks   *sweepFakeKS
	logs *processSyncSyncLogRepo
	svc  *DataSourceService
	// items is what the case's connector emits: doc:keep, the kept items and
	// the tombstones.
	items []types.FetchedItem
}

// newReconcileHarness builds ds-a for c. Besides the case's extra items, ds-a
// owns four: doc:keep and its sub-item, and doc:gone and its sub-item, both
// stale with no source resource.
func newReconcileHarness(t *testing.T, c reconcileCase) *reconcileHarness {
	t.Helper()
	configJSON, err := (&types.DataSourceConfig{
		Type: reconcileConnectorType, ResourceIDs: []string{reconcileSelection},
	}).ToJSON()
	require.NoError(t, err)
	ds := &types.DataSource{
		ID: "ds-a", TenantID: 1, KnowledgeBaseID: "kb-1", Name: "Reconcile",
		Type: reconcileConnectorType, Config: configJSON, SyncMode: types.SyncModeIncremental,
		Status: types.DataSourceStatusActive, SyncDeletions: c.syncDeletions,
	}
	repo := &reconcileRepo{live: map[string]map[string]*types.Knowledge{
		"ds-a": {
			"doc:keep":         {ID: "k-keep"},
			"doc:keep#image#1": {ID: "k-keep-image"},
			"doc:gone":         {ID: "k-gone", Title: "Gone"},
			"doc:gone#file#2":  {ID: "k-gone-file"},
		},
		"ds-b": {
			"doc:gone":  {ID: "k-b-gone"},
			"doc:other": {ID: "k-b-other"},
		},
	}, sources: map[string]string{}}
	items := []types.FetchedItem{{ExternalID: "doc:keep", Title: "keep"}}
	var tombstones []types.FetchedItem
	own := func(prefix, source string, n int) []string {
		ids := make([]string, n)
		for i := range n {
			ids[i] = fmt.Sprintf("doc:%s-%04d", prefix, i)
			repo.live["ds-a"][ids[i]] = &types.Knowledge{ID: "k-" + ids[i], Title: fmt.Sprintf("%s %d", prefix, i)}
			repo.sources[ids[i]] = source
		}
		return ids
	}
	for _, id := range own("kept", reconcileSelection, c.kept) {
		items = append(items, types.FetchedItem{ExternalID: id})
	}
	own("missing", reconcileSelection, c.missing)
	own("deselected", "res-old", c.deselected)
	for _, id := range own("tomb", reconcileSelection, c.tombstones) {
		tombstones = append(tombstones, types.FetchedItem{ExternalID: id, IsDeleted: true})
	}
	if c.emitNothing {
		items = nil
	}
	ks := &sweepFakeKS{repo: repo, deleteErr: c.deleteErr}
	logs := &processSyncSyncLogRepo{logs: map[string]*types.SyncLog{}}
	return &reconcileHarness{
		ds: ds, repo: repo, ks: ks, logs: logs, items: append(items, tombstones...),
		svc: &DataSourceService{
			dsRepo:           newKBDeleteDSRepo(ds.KnowledgeBaseID, ds),
			syncLogRepo:      logs,
			knowledgeService: ks,
			kbService: &processSyncKBService{
				kb: &types.KnowledgeBase{ID: ds.KnowledgeBaseID, TenantID: ds.TenantID},
			},
			tenantRepo: &processSyncTenantRepo{tenant: &types.Tenant{ID: ds.TenantID}},
			tagService: &processSyncTagService{},
		},
	}
}

// sync runs one ProcessSync through conn and returns its sync log and error.
func (h *reconcileHarness) sync(
	t *testing.T, conn datasource.Connector, forceFull, allowMass bool,
) (*types.SyncLog, error) {
	t.Helper()
	logID := fmt.Sprintf("log-%d", len(h.logs.logs))
	h.logs.logs[logID] = &types.SyncLog{
		ID: logID, DataSourceID: h.ds.ID, TenantID: h.ds.TenantID,
		Status: types.SyncLogStatusRunning, StartedAt: time.Now().UTC(),
	}
	registry := datasource.NewConnectorRegistry()
	require.NoError(t, registry.Register(conn))
	h.svc.connectorRegistry = registry
	payload, err := json.Marshal(types.DataSourceSyncPayload{
		DataSourceID: h.ds.ID, TenantID: h.ds.TenantID, SyncLogID: logID, ForceFull: forceFull,
		AllowMassDeletion: allowMass,
	})
	require.NoError(t, err)
	err = h.svc.ProcessSync(context.Background(), asynq.NewTask(types.TypeDataSourceSync, payload))
	return h.logs.logs[logID], err
}

// deleted returns the sorted IDs of the knowledge items deleted so far.
func (h *reconcileHarness) deleted() []string {
	deleted := append([]string(nil), h.ks.deleted...)
	sort.Strings(deleted)
	return deleted
}

// pending returns the deletions ds-a's stored cursor carries to the next run.
func (h *reconcileHarness) pending(t *testing.T) []string {
	t.Helper()
	cursor, err := h.ds.ParseSyncCursor()
	require.NoError(t, err)
	if cursor == nil {
		return nil
	}
	return cursor.PendingDeletions
}

// runReconcile runs one batch sync of c and returns the sync log and the IDs
// of the knowledge items deleted.
func runReconcile(t *testing.T, c reconcileCase) (*types.SyncLog, []string) {
	t.Helper()
	h := newReconcileHarness(t, c)
	log, err := h.sync(t, reconcileConnector{items: h.items, complete: c.complete}, c.forceFull, c.allowMass)
	require.NoError(t, err)
	return log, h.deleted()
}

// reconcileReport decodes the reconcile section of a sync log's result.
func reconcileReport(t *testing.T, log *types.SyncLog) *types.SyncReconcile {
	t.Helper()
	var result types.SyncResult
	require.NoError(t, json.Unmarshal(log.Result, &result))
	return result.Reconcile
}

func TestProcessSync_FullSyncReconcileDeletesItemsMissingFromCompleteListing(t *testing.T) {
	log, deleted := runReconcile(t, reconcileCase{syncDeletions: true, forceFull: true, complete: true})

	// doc:gone and the sub-item of the unseen parent go; the seen parent's
	// sub-item stays for its subtree sweep, and ds-b is never touched.
	assert.Equal(t, []string{"k-gone", "k-gone-file"}, deleted)
	assert.Equal(t, 2, log.ItemsDeleted)
	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
}

func TestProcessSync_FullSyncReconcileSkipsPartialListing(t *testing.T) {
	log, deleted := runReconcile(t, reconcileCase{syncDeletions: true, forceFull: true, complete: false})

	assert.Empty(t, deleted)
	assert.Equal(t, 0, log.ItemsDeleted)
}

func TestProcessSync_FullSyncReconcileSkipsWhenDeletionSyncOff(t *testing.T) {
	log, deleted := runReconcile(t, reconcileCase{syncDeletions: false, forceFull: true, complete: true})

	assert.Empty(t, deleted)
	assert.Equal(t, 0, log.ItemsDeleted)
}

func TestProcessSync_IncrementalSyncNeverReconciles(t *testing.T) {
	log, deleted := runReconcile(t, reconcileCase{syncDeletions: true, forceFull: false, complete: true})

	assert.Empty(t, deleted)
	assert.Equal(t, 0, log.ItemsDeleted)
}

func TestProcessSync_FullSyncReconcileCountsFailedDeletions(t *testing.T) {
	log, _ := runReconcile(t, reconcileCase{
		syncDeletions: true, forceFull: true, complete: true, deleteErr: errors.New("delete failed"),
	})

	assert.Equal(t, 0, log.ItemsDeleted)
	assert.Equal(t, 2, log.ItemsFailed)
	assert.Equal(t, 2, deletionFailedCount(t, log))
	assert.Equal(t, types.SyncLogStatusPartial, log.Status)
	assert.Equal(t, "2 document(s) failed to sync; 2 deletion(s) failed, the next sync retries them",
		log.ErrorMessage)
}

func TestProcessSync_FullSyncReconcileSkipsEmptyListing(t *testing.T) {
	log, deleted := runReconcile(t, reconcileCase{
		syncDeletions: true, forceFull: true, complete: true, emitNothing: true,
	})

	assert.Empty(t, deleted)
	assert.Equal(t, 0, log.ItemsDeleted)
}

func TestDeletionGuardTrips(t *testing.T) {
	cases := []struct {
		stale, owned int
		want         bool
	}{
		{stale: 9, owned: 9, want: false},
		{stale: 10, owned: 100, want: true},
		{stale: 10, owned: 101, want: false},
		{stale: 19, owned: 200, want: false},
		{stale: 20, owned: 200, want: true},
		{stale: 499, owned: 5000, want: false},
		{stale: 500, owned: 5001, want: true},
		{stale: 500, owned: 100000, want: true},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, deletionGuardTrips(c.stale, c.owned), "stale=%d owned=%d", c.stale, c.owned)
	}
}

func TestProcessSync_FullSyncReconcileCountsDeselectedAndMissing(t *testing.T) {
	// 7 stale of 106 owned stays under the guard.
	log, deleted := runReconcile(t, reconcileCase{
		syncDeletions: true, forceFull: true, complete: true, kept: 97, missing: 2, deselected: 3,
	})

	assert.Len(t, deleted, 7)
	assert.Equal(t, 7, log.ItemsDeleted)
	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
	assert.Equal(t, &types.SyncReconcile{Deselected: 3, Missing: 4}, reconcileReport(t, log))
}

func TestProcessSync_DeletionGuardHoldsBackTenPercent(t *testing.T) {
	// 10 stale (4 missing, 2 base, 4 deselected) of 100 owned is exactly 10%.
	log, deleted := runReconcile(t, reconcileCase{
		syncDeletions: true, forceFull: true, complete: true, kept: 88, missing: 4, deselected: 4,
	})

	assert.Empty(t, deleted)
	assert.Equal(t, 0, log.ItemsDeleted)
	assert.Equal(t, types.SyncLogStatusPartial, log.Status)
	assert.Equal(t, "Deletions held back: 10 item(s), 10% of the 100 this data source owns "+
		"(4 from resources removed from the selection); run Sync and apply deletions if this is expected",
		log.ErrorMessage)
	report := reconcileReport(t, log)
	assert.Equal(t, 4, report.Deselected)
	assert.Equal(t, 6, report.Missing)
	assert.Equal(t, "deletion_guard_tripped", report.Held.Code)
	assert.Equal(t, map[string]string{"count": "10", "owned": "100", "percent": "10", "deselected": "4"},
		report.Held.Params)
	assert.Equal(t, []string{
		"deselected 0", "deselected 1", "deselected 2", "deselected 3", "Gone", "doc:gone#file#2",
		"missing 0", "missing 1", "missing 2", "missing 3",
	}, report.Pending)
}

func TestProcessSync_DeletionGuardOverrideDeletes(t *testing.T) {
	log, deleted := runReconcile(t, reconcileCase{
		syncDeletions: true, forceFull: true, complete: true, kept: 88, missing: 4, deselected: 4, allowMass: true,
	})

	assert.Len(t, deleted, 10)
	assert.Equal(t, 10, log.ItemsDeleted)
	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
	assert.Equal(t, &types.SyncReconcile{Deselected: 4, Missing: 6}, reconcileReport(t, log))
}

func TestProcessSync_DeletionGuardHoldsBackFiveHundred(t *testing.T) {
	// 500 stale of 5506 owned is under 10%, so only the absolute limit trips.
	log, deleted := runReconcile(t, reconcileCase{
		syncDeletions: true, forceFull: true, complete: true, kept: 5004, missing: 498,
	})

	assert.Empty(t, deleted)
	assert.Equal(t, types.SyncLogStatusPartial, log.Status)
	report := reconcileReport(t, log)
	assert.Equal(t, "deletion_guard_tripped", report.Held.Code)
	assert.Equal(t, map[string]string{"count": "500", "owned": "5506", "percent": "9", "deselected": "0"},
		report.Held.Params)
	assert.Len(t, report.Pending, 100)
}

func TestProcessSync_DeletionGuardLetsFourNinetyNineThrough(t *testing.T) {
	log, deleted := runReconcile(t, reconcileCase{
		syncDeletions: true, forceFull: true, complete: true, kept: 5004, missing: 497,
	})

	assert.Len(t, deleted, 499)
	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
	assert.Nil(t, reconcileReport(t, log).Held)
}

func TestProcessSync_EmptyListingHoldBackIsReported(t *testing.T) {
	log, deleted := runReconcile(t, reconcileCase{
		syncDeletions: true, forceFull: true, complete: true, emitNothing: true,
	})

	assert.Empty(t, deleted)
	assert.Equal(t, types.SyncLogStatusPartial, log.Status)
	assert.Equal(t, "Deletions held back: the listing returned nothing while the data source owns 4 item(s)",
		log.ErrorMessage)
	assert.Equal(t, &types.SyncItemError{
		Code: "reconcile_listing_empty", Params: map[string]string{"owned": "4"},
		Message: "Deletions held back: the listing returned nothing while the data source owns 4 item(s)",
	}, reconcileReport(t, log).Held)
}

func TestProcessSync_EmptyListingOverrideDeletes(t *testing.T) {
	log, deleted := runReconcile(t, reconcileCase{
		syncDeletions: true, forceFull: true, complete: true, emitNothing: true, allowMass: true,
	})

	assert.Equal(t, []string{"k-gone", "k-gone-file", "k-keep", "k-keep-image"}, deleted)
	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
}

func TestProcessSync_IncompleteListingIsReported(t *testing.T) {
	log, _ := runReconcile(t, reconcileCase{syncDeletions: true, forceFull: true, complete: false})

	// The next complete full sync reconciles, so the run itself stays a success.
	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
	assert.Equal(t, "", log.ErrorMessage)
	assert.Equal(t, "reconcile_listing_incomplete", reconcileReport(t, log).Held.Code)
}

func TestManualSync_CarriesAllowMassDeletionInTheRunPayload(t *testing.T) {
	for _, allow := range []bool{false, true} {
		ds := &types.DataSource{
			ID: "ds-a", TenantID: 1, KnowledgeBaseID: "kb-1", Status: types.DataSourceStatusActive,
		}
		queue := &autoTagRecordingQueue{}
		svc := &DataSourceService{
			dsRepo:       newKBDeleteDSRepo(ds.KnowledgeBaseID, ds),
			syncLogRepo:  &processSyncSyncLogRepo{logs: map[string]*types.SyncLog{}},
			taskEnqueuer: queue,
		}
		_, err := svc.ManualSync(context.Background(), ds.ID, allow)
		require.NoError(t, err)

		var payload types.DataSourceSyncPayload
		require.NoError(t, json.Unmarshal(queue.task.Payload(), &payload))
		assert.Equal(t, allow, payload.AllowMassDeletion)
		assert.False(t, payload.ForceFull)
	}
}

// tombIDs returns the external IDs of the first n tombstoned items.
func tombIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("doc:tomb-%04d", i)
	}
	return ids
}

// The reported DingTalk failure: 1000 documents, an incremental run lists 100
// from a scope not marked failed, and the connector tombstones the other 900.
func TestProcessSync_ConnectorTombstonesPastTheGuardAreHeld(t *testing.T) {
	h := newReconcileHarness(t, reconcileCase{syncDeletions: true, kept: 99, tombstones: 900})
	log, err := h.sync(t, reconcileConnector{items: h.items}, false, false)
	require.NoError(t, err)

	assert.Empty(t, h.deleted())
	assert.Equal(t, 0, log.ItemsDeleted)
	assert.Equal(t, 1000, log.ItemsTotal)
	assert.Equal(t, types.SyncLogStatusPartial, log.Status)
	assert.Equal(t, "Deletions held back: 900 item(s), 89% of the 1003 this data source owns "+
		"(0 from resources removed from the selection); run Sync and apply deletions if this is expected",
		log.ErrorMessage)
	report := reconcileReport(t, log)
	assert.Equal(t, "deletion_guard_tripped", report.Held.Code)
	assert.Equal(t, map[string]string{"count": "900", "owned": "1003", "percent": "89", "deselected": "0"},
		report.Held.Params)
	assert.True(t, report.Overridable)
	assert.Equal(t, 900, report.Missing)
	assert.Len(t, report.Pending, 100)
	assert.Equal(t, "tomb 0", report.Pending[0])
	assert.Equal(t, tombIDs(900), h.pending(t))

	// The connector has dropped the tombstones from its cursor; the held set
	// keeps them, and a later unguarded run holds them again.
	log, err = h.sync(t, reconcileConnector{items: h.items[:1]}, false, false)
	require.NoError(t, err)
	assert.Empty(t, h.deleted())
	assert.Equal(t, types.SyncLogStatusPartial, log.Status)
	assert.Equal(t, "900", reconcileReport(t, log).Held.Params["count"])
	assert.Equal(t, tombIDs(900), h.pending(t))
}

func TestProcessSync_OverrideAppliesHeldTombstonesItsListingNoLongerEmits(t *testing.T) {
	h := newReconcileHarness(t, reconcileCase{syncDeletions: true, kept: 99, tombstones: 900})
	_, err := h.sync(t, reconcileConnector{items: h.items}, false, false)
	require.NoError(t, err)
	require.Len(t, h.pending(t), 900)

	log, err := h.sync(t, reconcileConnector{items: h.items[:1]}, false, true)
	require.NoError(t, err)

	assert.Len(t, h.deleted(), 900)
	assert.Equal(t, "k-doc:tomb-0000", h.deleted()[0])
	assert.Equal(t, 900, log.ItemsDeleted)
	assert.Equal(t, 901, log.ItemsTotal)
	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
	assert.Equal(t, &types.SyncReconcile{Missing: 900}, reconcileReport(t, log))
	assert.Nil(t, h.pending(t))
}

func TestProcessSync_TombstonesUnderTheGuardApplyInAnIncrementalRun(t *testing.T) {
	h := newReconcileHarness(t, reconcileCase{syncDeletions: true, kept: 97, tombstones: 3})
	log, err := h.sync(t, reconcileConnector{items: h.items}, false, false)
	require.NoError(t, err)

	assert.Equal(t, []string{"k-doc:tomb-0000", "k-doc:tomb-0001", "k-doc:tomb-0002"}, h.deleted())
	assert.Equal(t, 3, log.ItemsDeleted)
	assert.Equal(t, 101, log.ItemsTotal)
	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
	assert.Equal(t, &types.SyncReconcile{Missing: 3}, reconcileReport(t, log))
	assert.Nil(t, h.pending(t))
}

// Tombstones are not part of the listing, so a full sync that emits nothing
// else listed nothing.
func TestProcessSync_ListingOfOnlyTombstonesIsHeldAsEmpty(t *testing.T) {
	h := newReconcileHarness(t, reconcileCase{
		syncDeletions: true, forceFull: true, complete: true, emitNothing: true, tombstones: 2,
	})
	log, err := h.sync(t, reconcileConnector{items: h.items, complete: true}, true, false)
	require.NoError(t, err)

	assert.Empty(t, h.deleted())
	assert.Equal(t, types.SyncLogStatusPartial, log.Status)
	assert.Equal(t, &types.SyncItemError{
		Code: "reconcile_listing_empty", Params: map[string]string{"owned": "6"},
		Message: "Deletions held back: the listing returned nothing while the data source owns 6 item(s)",
	}, reconcileReport(t, log).Held)
	assert.Equal(t, []string{
		"doc:gone", "doc:gone#file#2", "doc:keep", "doc:keep#image#1", "doc:tomb-0000", "doc:tomb-0001",
	}, h.pending(t))
}

func TestProcessSync_StreamingTombstonesPastTheGuardAreHeld(t *testing.T) {
	h := newReconcileHarness(t, reconcileCase{syncDeletions: true, kept: 99, tombstones: 900})
	log, err := h.sync(t, streamOf(h.items, nil), false, false)
	require.NoError(t, err)

	assert.Empty(t, h.deleted())
	assert.Equal(t, types.SyncLogStatusPartial, log.Status)
	assert.Equal(t, "deletion_guard_tripped", reconcileReport(t, log).Held.Code)
	assert.Equal(t, tombIDs(900), h.pending(t))
}

// A streaming run that fails after checkpointing past a tombstone keeps the
// tombstone in the same cursor write, so the next run applies it.
func TestProcessSync_StreamingCheckpointKeepsDeferredTombstones(t *testing.T) {
	h := newReconcileHarness(t, reconcileCase{syncDeletions: true, tombstones: 1})
	log, err := h.sync(t, streamOf(h.items, errors.New("reset")), false, false)
	require.Error(t, err)

	assert.Equal(t, types.SyncLogStatusFailed, log.Status)
	assert.Empty(t, h.deleted())
	cursor, err := h.ds.ParseSyncCursor()
	require.NoError(t, err)
	assert.Equal(t, float64(2), cursor.ConnectorCursor["emitted"])
	assert.Equal(t, []string{"doc:tomb-0000"}, cursor.PendingDeletions)

	log, err = h.sync(t, streamOf(h.items[:1], nil), false, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"k-doc:tomb-0000"}, h.deleted())
	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
	assert.Nil(t, h.pending(t))
}

// Confluence Cloud's case: the connector type reconciles, this configuration
// cannot, so a full sync neither reconciles nor reports an incomplete listing.
func TestProcessSync_ConfigWithoutCompleteListingsDoesNotReconcile(t *testing.T) {
	h := newReconcileHarness(t, reconcileCase{syncDeletions: true})
	log, err := h.sync(t, reconcileConnector{items: h.items, complete: true, incompleteConfig: true}, true, false)
	require.NoError(t, err)

	assert.Empty(t, h.deleted())
	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
	assert.Nil(t, reconcileReport(t, log))
}
