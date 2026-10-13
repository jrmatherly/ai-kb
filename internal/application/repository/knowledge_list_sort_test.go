package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type knowledgeSortSeed struct {
	id        string
	fileName  string
	title     string
	createdAt time.Time
	updatedAt time.Time
}

func insertKnowledgeSortSeed(t *testing.T, db *gorm.DB, seed knowledgeSortSeed) {
	t.Helper()
	require.NoError(t, db.Exec(`
		INSERT INTO knowledges
			(id, tenant_id, knowledge_base_id, type, title, source, file_name, parse_status, created_at, updated_at)
		VALUES (?, 1, 'kb-sort', 'file', ?, 'manual', ?, 'completed', ?, ?)
	`, seed.id, seed.title, seed.fileName, seed.createdAt, seed.updatedAt).Error)
}

func knowledgeIDs(rows []*types.Knowledge) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func TestListPagedKnowledgeSortModes(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	repo := NewKnowledgeRepository(db).(*knowledgeRepository)
	ctx := context.Background()

	jan := time.Date(2024, time.January, 1, 8, 0, 0, 0, time.UTC)
	feb := time.Date(2024, time.February, 1, 8, 0, 0, 0, time.UTC)
	mar := time.Date(2024, time.March, 1, 8, 0, 0, 0, time.UTC)
	insertKnowledgeSortSeed(t, db, knowledgeSortSeed{
		id: "doc-a", fileName: "Zulu.txt", title: "Zulu", createdAt: jan, updatedAt: mar,
	})
	insertKnowledgeSortSeed(t, db, knowledgeSortSeed{
		id: "doc-b", fileName: "alpha.txt", title: "Alpha", createdAt: feb, updatedAt: jan,
	})
	// With file_name empty, name sorting must use title, as the frontend displays.
	insertKnowledgeSortSeed(t, db, knowledgeSortSeed{
		id: "doc-c", fileName: "", title: "Bravo note", createdAt: mar, updatedAt: feb,
	})

	tests := []struct {
		name   string
		filter types.KnowledgeListFilter
		want   []string
	}{
		{
			name: "updated time, newest first",
			filter: types.KnowledgeListFilter{
				SortBy: types.KnowledgeListSortByUpdatedAt, SortOrder: types.KnowledgeListSortDescending,
			},
			want: []string{"doc-a", "doc-c", "doc-b"},
		},
		{
			name: "updated time, oldest first",
			filter: types.KnowledgeListFilter{
				SortBy: types.KnowledgeListSortByUpdatedAt, SortOrder: types.KnowledgeListSortAscending,
			},
			want: []string{"doc-b", "doc-c", "doc-a"},
		},
		{
			name: "created time, newest first",
			filter: types.KnowledgeListFilter{
				SortBy: types.KnowledgeListSortByCreatedAt, SortOrder: types.KnowledgeListSortDescending,
			},
			want: []string{"doc-c", "doc-b", "doc-a"},
		},
		{
			name: "created time, oldest first",
			filter: types.KnowledgeListFilter{
				SortBy: types.KnowledgeListSortByCreatedAt, SortOrder: types.KnowledgeListSortAscending,
			},
			want: []string{"doc-a", "doc-b", "doc-c"},
		},
		{
			name: "file name A to Z, case-insensitive",
			filter: types.KnowledgeListFilter{
				SortBy: types.KnowledgeListSortByFileName, SortOrder: types.KnowledgeListSortAscending,
			},
			want: []string{"doc-b", "doc-c", "doc-a"},
		},
		{
			name: "file name Z to A, case-insensitive",
			filter: types.KnowledgeListFilter{
				SortBy: types.KnowledgeListSortByFileName, SortOrder: types.KnowledgeListSortDescending,
			},
			want: []string{"doc-a", "doc-c", "doc-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, total, err := repo.ListPagedKnowledgeByKnowledgeBaseID(
				ctx,
				1,
				"kb-sort",
				&types.Pagination{Page: 1, PageSize: 100},
				tt.filter,
			)
			require.NoError(t, err)
			assert.Equal(t, int64(3), total)
			assert.Equal(t, tt.want, knowledgeIDs(rows))
		})
	}
}

func TestListPagedKnowledgeSortUsesStableIDTieBreaker(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	repo := NewKnowledgeRepository(db).(*knowledgeRepository)
	sameTime := time.Date(2024, time.April, 1, 8, 0, 0, 0, time.UTC)

	insertKnowledgeSortSeed(t, db, knowledgeSortSeed{
		id: "doc-b", fileName: "same.txt", title: "Same", createdAt: sameTime, updatedAt: sameTime,
	})
	insertKnowledgeSortSeed(t, db, knowledgeSortSeed{
		id: "doc-a", fileName: "same.txt", title: "Same", createdAt: sameTime, updatedAt: sameTime,
	})

	rows, _, err := repo.ListPagedKnowledgeByKnowledgeBaseID(
		context.Background(),
		1,
		"kb-sort",
		&types.Pagination{Page: 1, PageSize: 100},
		types.KnowledgeListFilter{
			SortBy: types.KnowledgeListSortByFileName, SortOrder: types.KnowledgeListSortAscending,
		},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"doc-a", "doc-b"}, knowledgeIDs(rows))

	// One row per page, to confirm documents with equal sort values are neither repeated nor skipped across pages.
	for page, wantID := range []string{"doc-a", "doc-b"} {
		rows, total, err := repo.ListPagedKnowledgeByKnowledgeBaseID(
			context.Background(), 1, "kb-sort",
			&types.Pagination{Page: page + 1, PageSize: 1},
			types.KnowledgeListFilter{
				SortBy: types.KnowledgeListSortByFileName, SortOrder: types.KnowledgeListSortAscending,
			},
		)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Equal(t, []string{wantID}, knowledgeIDs(rows))
	}
}
