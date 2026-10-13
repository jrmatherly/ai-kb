package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindByDataSourceExternalID(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	repo := NewKnowledgeRepository(db).(*knowledgeRepository)
	ctx := context.Background()

	const tenantID uint64 = 50
	kbID := uuid.New().String()
	dsA := uuid.New().String()
	dsB := uuid.New().String()

	insertRow := func(dsID, extID string) string {
		id := uuid.New().String()
		metadata := fmt.Sprintf(`{"datasource_id":%q,"external_id":%q}`, dsID, extID)
		require.NoError(t, db.Exec(`
			INSERT INTO knowledges
			  (id, tenant_id, knowledge_base_id, type, title, source, parse_status, metadata)
			VALUES (?, ?, ?, 'document', ?, 'feishu', 'completed', ?)
		`, id, tenantID, kbID, extID, metadata).Error)
		return id
	}

	idA := insertRow(dsA, "file:shared")
	_ = insertRow(dsB, "file:shared")

	found, err := repo.FindByDataSourceExternalID(ctx, tenantID, kbID, dsA, "file:shared")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, idA, found.ID)
}

func TestFindByDataSourceExternalID_DeletedRowsExcluded(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	repo := NewKnowledgeRepository(db).(*knowledgeRepository)
	ctx := context.Background()

	const tenantID uint64 = 51
	kbID := uuid.New().String()
	dsID := uuid.New().String()

	id := uuid.New().String()
	metadata := fmt.Sprintf(`{"datasource_id":%q,"external_id":%q}`, dsID, "file:gone")
	require.NoError(t, db.Exec(`
		INSERT INTO knowledges
		  (id, tenant_id, knowledge_base_id, type, title, source, parse_status, metadata, deleted_at)
		VALUES (?, ?, ?, 'document', 'gone', 'feishu', 'completed', ?, '2026-01-01 00:00:00')
	`, id, tenantID, kbID, metadata).Error)

	found, err := repo.FindByDataSourceExternalID(ctx, tenantID, kbID, dsID, "file:gone")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestListDataSourceItems(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	repo := NewKnowledgeRepository(db).(*knowledgeRepository)
	ctx := context.Background()

	const tenantID uint64 = 53
	kbID := uuid.New().String()
	insert := func(kb, metadata, deletedAt string) {
		var deleted any
		if deletedAt != "" {
			deleted = deletedAt
		}
		require.NoError(t, db.Exec(`
			INSERT INTO knowledges
			  (id, tenant_id, knowledge_base_id, type, title, source, parse_status, metadata, deleted_at)
			VALUES (?, ?, ?, 'document', 't', 'feishu', 'completed', ?, ?)
		`, uuid.New().String(), tenantID, kb, metadata, deleted).Error)
	}
	insert(kbID, `{"datasource_id":"ds-a","external_id":"doc:1","source_resource_id":"space-1"}`, "")
	insert(kbID, `{"datasource_id":"ds-a","external_id":"doc:1","source_resource_id":"space-1"}`, "")
	insert(kbID, `{"datasource_id":"ds-a","external_id":"doc:2"}`, "")
	insert(kbID, `{"datasource_id":"ds-a","external_id":"doc:gone"}`, "2026-01-01 00:00:00")
	insert(kbID, `{"datasource_id":"ds-a"}`, "")
	insert(kbID, `{"datasource_id":"ds-b","external_id":"doc:3"}`, "")
	insert(uuid.New().String(), `{"datasource_id":"ds-a","external_id":"doc:4"}`, "")

	items, err := repo.ListDataSourceItems(ctx, tenantID, kbID, "ds-a")
	require.NoError(t, err)
	assert.ElementsMatch(t, []types.DataSourceItemRef{
		{ExternalID: "doc:1", Title: "t", SourceResourceID: "space-1"},
		{ExternalID: "doc:2", Title: "t"},
	}, items)

	items, err = repo.ListDataSourceItems(ctx, tenantID, kbID, "ds-a", "doc:1", "doc:3", "doc:gone", "doc:none")
	require.NoError(t, err)
	assert.Equal(t, []types.DataSourceItemRef{{ExternalID: "doc:1", Title: "t", SourceResourceID: "space-1"}}, items)

	count, err := repo.CountDataSourceItems(ctx, tenantID, kbID, "ds-a")
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestHardDeleteKnowledge(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	repo := NewKnowledgeRepository(db).(*knowledgeRepository)
	ctx := context.Background()

	const tenantID uint64 = 52
	kbID := uuid.New().String()
	id := uuid.New().String()
	metadata := `{"datasource_id":"ds-1","external_id":"file:1"}`
	require.NoError(t, db.Exec(`
		INSERT INTO knowledges
		  (id, tenant_id, knowledge_base_id, type, title, source, parse_status, metadata, deleted_at)
		VALUES (?, ?, ?, 'document', 'file:1', 'feishu', 'completed', ?, '2026-01-01 00:00:00')
	`, id, tenantID, kbID, metadata).Error)

	require.NoError(t, repo.HardDeleteKnowledge(ctx, tenantID, id))

	var count int64
	require.NoError(t, db.Unscoped().Model(&struct{}{}).Table("knowledges").Where("id = ?", id).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}
