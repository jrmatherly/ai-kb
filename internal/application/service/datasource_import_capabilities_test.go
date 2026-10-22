package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	fullImports = utils.ImportCapabilities{Images: true, Audio: true}
	noImports   = utils.ImportCapabilities{}
)

func pngItem() *types.FetchedItem {
	return &types.FetchedItem{
		ExternalID: "file:1", Title: "shot", FileName: "shot.png",
		Content: []byte("\x89PNG\r\n\x1a\nxxxx"), ContentType: "image/png",
	}
}

func TestIngestItemSkipsUnimportableBeforeDeleting(t *testing.T) {
	ds := &types.DataSource{ID: "ds-1", TenantID: 1, KnowledgeBaseID: "kb-1"}
	repo := &deletionLookupKnowledgeRepo{knowledge: &types.Knowledge{ID: "old"}}
	ks := &sweepFakeKS{repo: repo}
	svc := &DataSourceService{knowledgeService: ks}

	isUpdate, err := svc.ingestItem(context.Background(), ds, noImports, pngItem(), nil)
	assert.ErrorIs(t, err, errUnimportable)
	assert.False(t, isUpdate)
	assert.Empty(t, ks.events, "an unimportable item must leave the existing row alone")

	isUpdate, err = svc.ingestItem(context.Background(), ds, fullImports, pngItem(), nil)
	require.NoError(t, err)
	assert.True(t, isUpdate)
	assert.Equal(t, []string{"delete:old", "create:shot.png"}, ks.events)
}

func TestIngestItemPrecheckIgnoresURLOnlyItems(t *testing.T) {
	ds := &types.DataSource{ID: "ds-1", TenantID: 1, KnowledgeBaseID: "kb-1"}
	repo := &deletionLookupKnowledgeRepo{}
	ks := &sweepFakeKS{repo: repo, createURLKnowledge: &types.Knowledge{ID: "url-knowledge-1"}}
	svc := &DataSourceService{knowledgeService: ks}

	isUpdate, err := svc.ingestItem(context.Background(), ds, noImports, &types.FetchedItem{
		ExternalID: "url:1", URL: "https://example.com/photo",
	}, nil)
	require.NoError(t, err)
	assert.False(t, isUpdate)
	require.Len(t, repo.metadataUpdates, 1)
	assert.Equal(t, "url:1", repo.metadataUpdates[0]["external_id"])
}

func TestApplyFetchedItemCountsUnimportableAsSkipped(t *testing.T) {
	ds := &types.DataSource{ID: "ds-1", TenantID: 1, KnowledgeBaseID: "kb-1"}
	ks := &sweepFakeKS{repo: &sweepFakeRepo{}}
	svc := &DataSourceService{knowledgeService: ks}

	result := &types.SyncResult{}
	svc.applyFetchedItem(context.Background(), ds, noImports, pngItem(), nil, result)
	assert.Equal(t, &types.SyncResult{Skipped: 1}, result)
	assert.Empty(t, ks.events)

	result = &types.SyncResult{}
	svc.applyFetchedItem(context.Background(), ds, fullImports, pngItem(), nil, result)
	assert.Equal(t, &types.SyncResult{Created: 1}, result)
}

func TestStartCursorForForcesFullSyncOnlyAgainstARecordedValue(t *testing.T) {
	images := utils.ImportCapabilities{Images: true}
	unstamped := &types.SyncCursor{PendingDeletions: []string{"a"}}
	stampedNone := &types.SyncCursor{PendingDeletions: []string{"a"}, Imports: &noImports}
	stampedImages := &types.SyncCursor{PendingDeletions: []string{"a"}, Imports: &images}

	assert.Nil(t, startCursorFor(nil, images))
	assert.Same(t, unstamped, startCursorFor(unstamped, images))
	assert.Nil(t, startCursorFor(stampedNone, images))
	assert.Same(t, stampedImages, startCursorFor(stampedImages, noImports))
	assert.Same(t, stampedImages, startCursorFor(stampedImages, images))
}

func TestWithPendingStampsImports(t *testing.T) {
	images := utils.ImportCapabilities{Images: true}
	ds := &types.DataSource{SyncDeletions: true}
	d := newRunDeletions(ds, &types.SyncCursor{PendingDeletions: []string{"gone"}}, false, images)

	assert.Equal(t, &types.SyncCursor{PendingDeletions: []string{"gone"}, Imports: &images}, d.withPending(nil))

	connector := &types.SyncCursor{ConnectorCursor: map[string]interface{}{"page": "2"}}
	assert.Equal(t, &types.SyncCursor{
		ConnectorCursor:  map[string]interface{}{"page": "2"},
		PendingDeletions: []string{"gone"},
		Imports:          &images,
	}, d.withPending(connector))
	assert.Nil(t, connector.Imports, "the connector's own cursor is copied, not mutated")
}

type recordingListConnector struct {
	datasource.Connector
	got []utils.ImportCapabilities
}

func (recordingListConnector) Type() string { return "recording-list" }

func (c *recordingListConnector) ListResources(
	_ context.Context, config *types.DataSourceConfig, _ string,
) ([]types.Resource, error) {
	c.got = append(c.got, config.Imports)
	return nil, nil
}

func TestListResourcesForConfigLoadsTheKnowledgeBase(t *testing.T) {
	connector := &recordingListConnector{}
	registry := datasource.NewConnectorRegistry()
	require.NoError(t, registry.Register(connector))
	svc := &DataSourceService{
		connectorRegistry: registry,
		kbService:         &processSyncKBService{kb: &types.KnowledgeBase{ID: "kb-1"}},
	}

	_, err := svc.ListResourcesForConfig(context.Background(),
		&types.DataSourceConfig{Type: "recording-list"}, "kb-1", "")
	require.NoError(t, err)
	_, err = svc.ListResourcesForConfig(context.Background(),
		&types.DataSourceConfig{Type: "recording-list"}, "", "")
	require.NoError(t, err)
	assert.Equal(t, []utils.ImportCapabilities{noImports, fullImports}, connector.got)

	svc.kbService = &processSyncKBService{getErr: errors.New("gone")}
	_, err = svc.ListResourcesForConfig(context.Background(),
		&types.DataSourceConfig{Type: "recording-list"}, "kb-1", "")
	require.NoError(t, err)
	assert.Equal(t, fullImports, connector.got[2], "an unloadable knowledge base must not hide files")
}
