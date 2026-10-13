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

// reconcileConnector is a batch connector whose full sync returns fixed items
// and reports the given listing completeness.
type reconcileConnector struct {
	items    []types.FetchedItem
	complete bool
}

func (reconcileConnector) Type() string { return reconcileConnectorType }

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
	// longer selected.
	kept, missing, deselected int
}

// runReconcile syncs data source ds-a, whose connector emits doc:keep, against
// a knowledge base that also holds items of ds-b. Besides the case's extra
// items, ds-a owns four: doc:keep and its sub-item, and doc:gone and its
// sub-item, both stale with no source resource. It returns the sync log and the
// IDs of the knowledge items deleted.
func runReconcile(t *testing.T, c reconcileCase) (*types.SyncLog, []string) {
	t.Helper()
	configJSON, err := (&types.DataSourceConfig{
		Type: reconcileConnectorType, ResourceIDs: []string{reconcileSelection},
	}).ToJSON()
	require.NoError(t, err)
	syncMode := types.SyncModeIncremental
	ds := &types.DataSource{
		ID: "ds-a", TenantID: 1, KnowledgeBaseID: "kb-1", Name: "Reconcile",
		Type: reconcileConnectorType, Config: configJSON, SyncMode: syncMode,
		Status: types.DataSourceStatusActive, SyncDeletions: c.syncDeletions,
	}
	syncLog := &types.SyncLog{
		ID: "log-reconcile", DataSourceID: ds.ID, TenantID: ds.TenantID,
		Status: types.SyncLogStatusRunning, StartedAt: time.Now().UTC(),
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
	own := func(prefix, source string, n int, emit bool) {
		for i := range n {
			ext := fmt.Sprintf("doc:%s-%04d", prefix, i)
			repo.live["ds-a"][ext] = &types.Knowledge{ID: "k-" + ext, Title: fmt.Sprintf("%s %d", prefix, i)}
			repo.sources[ext] = source
			if emit {
				items = append(items, types.FetchedItem{ExternalID: ext})
			}
		}
	}
	own("kept", reconcileSelection, c.kept, true)
	own("missing", reconcileSelection, c.missing, false)
	own("deselected", "res-old", c.deselected, false)
	if c.emitNothing {
		items = nil
	}
	ks := &sweepFakeKS{repo: repo, deleteErr: c.deleteErr}
	syncLogRepo := &processSyncSyncLogRepo{logs: map[string]*types.SyncLog{syncLog.ID: syncLog}}
	registry := datasource.NewConnectorRegistry()
	require.NoError(t, registry.Register(reconcileConnector{items: items, complete: c.complete}))
	svc := &DataSourceService{
		dsRepo:           newKBDeleteDSRepo(ds.KnowledgeBaseID, ds),
		syncLogRepo:      syncLogRepo,
		knowledgeService: ks,
		kbService: &processSyncKBService{
			kb: &types.KnowledgeBase{ID: ds.KnowledgeBaseID, TenantID: ds.TenantID},
		},
		connectorRegistry: registry,
		tenantRepo:        &processSyncTenantRepo{tenant: &types.Tenant{ID: ds.TenantID}},
		tagService:        &processSyncTagService{},
	}
	payload, err := json.Marshal(types.DataSourceSyncPayload{
		DataSourceID: ds.ID, TenantID: ds.TenantID, SyncLogID: syncLog.ID, ForceFull: c.forceFull,
		AllowMassDeletion: c.allowMass,
	})
	require.NoError(t, err)
	require.NoError(t, svc.ProcessSync(context.Background(), asynq.NewTask(types.TypeDataSourceSync, payload)))

	deleted := append([]string(nil), ks.deleted...)
	sort.Strings(deleted)
	return syncLogRepo.logs[syncLog.ID], deleted
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
	assert.Equal(t, "2 document(s) failed to sync; 2 deletion(s) failed, "+
		"the next full sync retries them when its listing is complete", log.ErrorMessage)
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
