package service

import (
	"context"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
	"github.com/stretchr/testify/require"
)

type gitLabKnowledgeRepo struct {
	interfaces.KnowledgeRepository
}

func (r *gitLabKnowledgeRepo) GetKnowledgeTags(context.Context, []string) (map[string][]*types.KnowledgeTag, error) {
	return map[string][]*types.KnowledgeTag{}, nil
}

func TestCreateKnowledgeFromFileGitLabPreservesSourcePaths(t *testing.T) {
	db := setupKnowledgeSharedAccessDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	repo := &gitLabKnowledgeRepo{KnowledgeRepository: repository.NewKnowledgeRepository(db)}
	svc := &knowledgeService{
		repo:      repo,
		kbService: &createKnowledgeFileKBServiceStub{kb: &types.KnowledgeBase{ID: "kb-1"}},
		fileSvc:   &createKnowledgeFileServiceStub{},
		task:      &createKnowledgeTaskEnqueuerStub{},
	}
	ctx := newCreateKnowledgeFileContext()
	create := func(channel, source, file string) (*types.Knowledge, error) {
		t.Helper()
		metadata := map[string]string{}
		if source != "" {
			metadata["datasource_id"] = source
			metadata["external_id"] = "gitlab:instance:1:main:" + file
		}
		return svc.CreateKnowledgeFromFile(ctx, "kb-1", newMultipartFileHeader(t, "README.md", "# same content"),
			metadata, nil, "docs-main/"+file, nil, channel, nil)
	}

	root, err := create(types.ConnectorTypeGitLab, "ds-1", "README.md")
	require.NoError(t, err)
	ds := &types.DataSource{ID: "ds-1", TenantID: 1, KnowledgeBaseID: "kb-1", Type: types.ConnectorTypeGitLab}
	item := &types.FetchedItem{
		ExternalID: "gitlab:instance:1:main:a/b/c/d/e/README.md",
		FileName:   "docs-main/a/b/c/d/e/README.md",
		Content:    []byte("# same content"),
	}
	result := &types.SyncResult{}
	(&DataSourceService{knowledgeService: svc}).applyFetchedItem(
		ctx, ds, secutils.AllImportCapabilities, item, nil, result)
	require.Equal(t, 1, result.Created, "identical content at a different repository path must still be imported")
	require.Zero(t, result.Skipped)
	require.Zero(t, result.Failed)
	nested, err := repo.FindByDataSourceExternalID(ctx, ds.TenantID, ds.KnowledgeBaseID, ds.ID, item.ExternalID)
	require.NoError(t, err)
	require.NotNil(t, nested)
	require.NotEqual(t, root.ID, nested.ID)
	require.Equal(t, "docs-main/a/b/c/d/e", nested.FolderPath)

	otherSource, err := create(types.ConnectorTypeGitLab, "ds-2", "README.md")
	require.NoError(t, err, "another data source must retain its own file")
	require.NotEqual(t, root.ID, otherSource.ID)

	duplicate, err := create(types.ConnectorTypeGitLab, "ds-1", "a/b/c/d/e/README.md")
	var dupErr *types.DuplicateKnowledgeError
	require.ErrorAs(t, err, &dupErr)
	require.Equal(t, nested.ID, duplicate.ID, "a retry must match this path's own knowledge")

	for _, channel := range []string{"", types.ConnectorTypeFeishu} {
		_, err := create(channel, "ds-3", "another/README.md")
		require.ErrorAs(t, err, &dupErr, "other import channels retain content deduplication")
	}
	_, err = create(types.ConnectorTypeGitLab, "", "missing-identity/README.md")
	require.ErrorAs(t, err, &dupErr, "incomplete source identity retains content deduplication")
}

func TestCreateKnowledgeFromFileGitHubUsesSourceIdentity(t *testing.T) {
	db := setupKnowledgeSharedAccessDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	svc := &knowledgeService{
		repo:      &gitLabKnowledgeRepo{KnowledgeRepository: repository.NewKnowledgeRepository(db)},
		kbService: &createKnowledgeFileKBServiceStub{kb: &types.KnowledgeBase{ID: "kb-1"}},
		fileSvc:   &createKnowledgeFileServiceStub{},
		task:      &createKnowledgeTaskEnqueuerStub{},
	}
	ctx := newCreateKnowledgeFileContext()
	create := func(source, file string) (*types.Knowledge, error) {
		t.Helper()
		return svc.CreateKnowledgeFromFile(ctx, "kb-1", newMultipartFileHeader(t, "README.md", "# same content"),
			map[string]string{"datasource_id": source, "external_id": "github:https://api.github.com:o/r:main:" + file},
			nil, "r-main/"+file, nil, types.ConnectorTypeGitHub, nil)
	}

	first, err := create("ds-1", "README.md")
	require.NoError(t, err)
	nested, err := create("ds-1", "docs/README.md")
	require.NoError(t, err, "identical content at a different repository path must still be imported")
	require.NotEqual(t, first.ID, nested.ID)
	otherSource, err := create("ds-2", "README.md")
	require.NoError(t, err, "another data source must retain its own file")
	require.NotEqual(t, first.ID, otherSource.ID)

	duplicate, err := create("ds-1", "docs/README.md")
	var dupErr *types.DuplicateKnowledgeError
	require.ErrorAs(t, err, &dupErr)
	require.Equal(t, nested.ID, duplicate.ID, "a retry must match this path's own knowledge")
}

func TestCreateKnowledgeFromFileConfluencePreservesCopiedPages(t *testing.T) {
	db := setupKnowledgeSharedAccessDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	repo := &gitLabKnowledgeRepo{KnowledgeRepository: repository.NewKnowledgeRepository(db)}
	svc := &knowledgeService{
		repo:      repo,
		kbService: &createKnowledgeFileKBServiceStub{kb: &types.KnowledgeBase{ID: "kb-1"}},
		fileSvc:   &createKnowledgeFileServiceStub{},
		task:      &createKnowledgeTaskEnqueuerStub{},
	}
	ctx := newCreateKnowledgeFileContext()
	create := func(source, pageID string) (*types.Knowledge, error) {
		t.Helper()
		return svc.CreateKnowledgeFromFile(ctx, "kb-1", newMultipartFileHeader(t, "Overview.md", "# same body"),
			map[string]string{"datasource_id": source, "external_id": pageID},
			nil, "Overview-"+pageID+".md", nil, types.ChannelConfluence, nil)
	}

	first, err := create("ds-1", "p1")
	require.NoError(t, err)
	second, err := create("ds-1", "p2")
	require.NoError(t, err, "copied Confluence pages with identical bodies must still be imported")
	require.NotEqual(t, first.ID, second.ID)

	duplicate, err := create("ds-1", "p2")
	var dupErr *types.DuplicateKnowledgeError
	require.ErrorAs(t, err, &dupErr)
	require.Equal(t, second.ID, duplicate.ID)
}

func TestCreateKnowledgeDuplicateKeepsExistingCreatedAt(t *testing.T) {
	db := setupKnowledgeSharedAccessDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	svc := &knowledgeService{
		repo:      &gitLabKnowledgeRepo{KnowledgeRepository: repository.NewKnowledgeRepository(db)},
		kbService: &createKnowledgeFileKBServiceStub{kb: &types.KnowledgeBase{ID: "kb-1"}},
		fileSvc:   &createKnowledgeFileServiceStub{},
		task:      &createKnowledgeTaskEnqueuerStub{},
	}
	ctx := newCreateKnowledgeFileContext()
	created := time.Date(2026, 10, 14, 17, 17, 56, 0, time.UTC)
	createdAt := func(id string) time.Time {
		t.Helper()
		var k types.Knowledge
		require.NoError(t, db.First(&k, "id = ?", id).Error)
		return k.CreatedAt.UTC()
	}
	var dupErr *types.DuplicateKnowledgeError

	createRSS := func(source string) (*types.Knowledge, error) {
		return svc.CreateKnowledgeFromFile(ctx, "kb-1", newMultipartFileHeader(t, "b11612.md", "# release notes"),
			map[string]string{"datasource_id": source, "external_id": "b11612"}, nil, "b11612.md", nil,
			types.ChannelRSS, nil)
	}
	owned, err := createRSS("ds-a")
	require.NoError(t, err)
	require.NoError(t, db.Model(&types.Knowledge{}).Where("id = ?", owned.ID).Update("created_at", created).Error)
	_, err = createRSS("ds-b")
	require.ErrorAs(t, err, &dupErr)
	require.Equal(t, created, createdAt(owned.ID), "a duplicate from another source must not rewrite created_at")

	const page = "https://93.184.216.34/post"
	t.Setenv("SSRF_WHITELIST", "93.184.216.34")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
	first, err := svc.CreateKnowledgeFromURL(ctx, "kb-1", page, "", "", nil, "", nil, "", nil)
	require.NoError(t, err)
	require.NoError(t, db.Model(&types.Knowledge{}).Where("id = ?", first.ID).Update("created_at", created).Error)
	_, err = svc.CreateKnowledgeFromURL(ctx, "kb-1", page, "", "", nil, "", nil, "", nil)
	require.ErrorAs(t, err, &dupErr)
	require.Equal(t, created, createdAt(first.ID), "re-adding a URL must not rewrite created_at")
}
