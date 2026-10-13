package service

import (
	"context"
	"encoding/json"
	"errors"
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

// reconcileRepo holds live knowledge per data source and external ID.
type reconcileRepo struct {
	interfaces.KnowledgeRepository
	live map[string]map[string]*types.Knowledge
}

func (r *reconcileRepo) ListDataSourceExternalIDs(_ context.Context, _ uint64, _, dsID string) ([]string, error) {
	ids := make([]string, 0, len(r.live[dsID]))
	for id := range r.live[dsID] {
		ids = append(ids, id)
	}
	return ids, nil
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

type reconcileCase struct {
	syncDeletions bool
	forceFull     bool
	complete      bool
	emitNothing   bool
	deleteErr     error
}

// runReconcile syncs data source ds-a, whose connector emits doc:keep, against
// a knowledge base that also holds items of ds-b. It returns the sync log and
// the IDs of the knowledge items deleted.
func runReconcile(t *testing.T, c reconcileCase) (*types.SyncLog, []string) {
	t.Helper()
	configJSON, err := (&types.DataSourceConfig{Type: reconcileConnectorType}).ToJSON()
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
			"doc:gone":         {ID: "k-gone"},
			"doc:gone#file#2":  {ID: "k-gone-file"},
		},
		"ds-b": {
			"doc:gone":  {ID: "k-b-gone"},
			"doc:other": {ID: "k-b-other"},
		},
	}}
	ks := &sweepFakeKS{repo: repo, deleteErr: c.deleteErr}
	syncLogRepo := &processSyncSyncLogRepo{logs: map[string]*types.SyncLog{syncLog.ID: syncLog}}
	registry := datasource.NewConnectorRegistry()
	items := []types.FetchedItem{{ExternalID: "doc:keep", Title: "keep"}}
	if c.emitNothing {
		items = nil
	}
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
	})
	require.NoError(t, err)
	require.NoError(t, svc.ProcessSync(context.Background(), asynq.NewTask(types.TypeDataSourceSync, payload)))

	deleted := append([]string(nil), ks.deleted...)
	sort.Strings(deleted)
	return syncLogRepo.logs[syncLog.ID], deleted
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
