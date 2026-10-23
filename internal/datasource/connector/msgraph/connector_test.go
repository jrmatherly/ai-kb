package msgraph

import (
	"context"
	"net/http"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func testConnector(f *fakeGraph, kind Kind) *Connector {
	return &Connector{kind: kind, graphBaseURL: f.server.URL, loginBaseURL: f.server.URL}
}

func sharePointConfig(ids ...string) *types.DataSourceConfig {
	return baseConfig(map[string]interface{}{"site_url": "https://contoso.sharepoint.com/sites/hr"}, ids...)
}

func oneDriveConfig(ids ...string) *types.DataSourceConfig {
	return baseConfig(map[string]interface{}{"user": "alice@contoso.com"}, ids...)
}

func serveSite(f *fakeGraph) {
	f.json("GET /sites/contoso.sharepoint.com:/sites/hr", http.StatusOK,
		map[string]any{"id": "site1", "displayName": "HR", "webUrl": "https://contoso.sharepoint.com/sites/hr"})
	f.json("GET /sites/site1/drives", http.StatusOK, map[string]any{"value": []any{
		map[string]any{"id": "b!docs", "name": "Documents", "webUrl": "https://contoso.sharepoint.com/sites/hr/Shared"},
		map[string]any{"id": "b!pol", "name": "Policies", "webUrl": "https://contoso.sharepoint.com/sites/hr/Policies"},
	}})
}

func TestTypes(t *testing.T) {
	require.Equal(t, "onedrive", NewConnector(KindOneDrive).Type())
	require.Equal(t, "sharepoint", NewConnector(KindSharePoint).Type())
	require.Equal(t, graphBaseURL, NewConnector(KindOneDrive).graphBaseURL)
}

func TestValidateCredentialsOnlyMintsAToken(t *testing.T) {
	f := newFakeGraph(t)
	require.NoError(t, testConnector(f, KindSharePoint).Validate(context.Background(), baseConfig(nil)))
	require.Equal(t, int32(1), f.tokens.Load())
	require.Equal(t, 0, f.seen("GET /sites"))
}

func TestValidateResolvesTheRoot(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	require.NoError(t, testConnector(f, KindSharePoint).Validate(context.Background(), sharePointConfig()))
	require.Equal(t, 1, f.seen("GET /sites/contoso.sharepoint.com:/sites/hr"))

	f.json("GET /users/alice@contoso.com/drive", http.StatusOK, map[string]any{"id": "b!alice", "name": "OneDrive"})
	require.NoError(t, testConnector(f, KindOneDrive).Validate(context.Background(), oneDriveConfig()))
	err := testConnector(f, KindOneDrive).Validate(context.Background(),
		baseConfig(map[string]interface{}{"user": "nobody@contoso.com"}))
	require.ErrorIs(t, err, datasource.ErrResourceNotFound)
}

func TestListResourcesSharePointLibrariesThenFolders(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	f.json("GET /drives/b!docs/root/children", http.StatusOK, map[string]any{"value": []any{
		map[string]any{"id": "01F", "name": "Finance", "folder": map[string]any{}, "webUrl": "https://x/Finance"},
		map[string]any{"id": "01X", "name": "readme.md", "file": map[string]any{}},
	}})
	c := testConnector(f, KindSharePoint)
	top, err := c.ListResources(context.Background(), sharePointConfig(), "")
	require.NoError(t, err)
	require.Equal(t, []types.Resource{
		{
			ExternalID:  "b!docs",
			Name:        "Documents",
			Type:        "drive",
			URL:         "https://contoso.sharepoint.com/sites/hr/Shared",
			HasChildren: true,
		},
		{
			ExternalID:  "b!pol",
			Name:        "Policies",
			Type:        "drive",
			URL:         "https://contoso.sharepoint.com/sites/hr/Policies",
			HasChildren: true,
		},
	}, top)
	children, err := c.ListResources(context.Background(), sharePointConfig(), "b!docs")
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.Equal(t, "b!docs:01F", children[0].ExternalID)
	require.Equal(t, "b!docs", children[0].ParentID)
	require.Equal(t, "folder", children[0].Type)
	require.True(t, children[0].HasChildren)
}

func TestListResourcesOneDriveSingleDrive(t *testing.T) {
	f := newFakeGraph(t)
	f.json(
		"GET /users/alice@contoso.com/drive",
		http.StatusOK,
		map[string]any{"id": "b!alice", "name": "OneDrive", "webUrl": "https://x"},
	)
	top, err := testConnector(f, KindOneDrive).ListResources(context.Background(), oneDriveConfig(), "")
	require.NoError(t, err)
	require.Equal(
		t,
		[]types.Resource{{ExternalID: "b!alice", Name: "OneDrive", Type: "drive", URL: "https://x", HasChildren: true}},
		top,
	)
}

func TestListResourcesWithoutPublicSettingIsInvalidConfig(t *testing.T) {
	f := newFakeGraph(t)
	_, err := testConnector(f, KindSharePoint).ListResources(context.Background(), baseConfig(nil), "")
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
}

func TestResolveResourceAncestorsWalksParents(t *testing.T) {
	f := newFakeGraph(t)
	f.json(
		"GET /drives/b!docs/items/01C",
		http.StatusOK,
		map[string]any{
			"id": "01C", "name": "c", "folder": map[string]any{},
			"parentReference": map[string]any{"driveId": "b!docs", "id": "01B"},
		},
	)
	f.json(
		"GET /drives/b!docs/items/01B",
		http.StatusOK,
		map[string]any{
			"id": "01B", "name": "b", "folder": map[string]any{},
			"parentReference": map[string]any{"driveId": "b!docs", "id": "01ROOT"},
		},
	)
	f.json(
		"GET /drives/b!docs/items/01ROOT",
		http.StatusOK,
		map[string]any{
			"id": "01ROOT", "name": "root", "root": map[string]any{},
			"folder": map[string]any{}, "parentReference": map[string]any{"driveId": "b!docs"},
		},
	)
	got, err := testConnector(
		f,
		KindSharePoint,
	).ResolveResourceAncestors(context.Background(), sharePointConfig(), []string{"b!docs:01C", "b!docs"})
	require.NoError(t, err)
	require.Equal(t, []string{"b!docs", "b!docs:01B"}, got)
}

func TestErrorItemAndTombstoneCarryTheItemMetadata(t *testing.T) {
	c := NewConnector(KindSharePoint)
	require.Equal(t, types.FetchedItem{
		ExternalID: "b!docs:01A", Title: "big.pdf",
		Metadata: map[string]string{
			"channel":           "sharepoint",
			"msgraph_drive_id":  "b!docs",
			"msgraph_item_id":   "01A",
			"msgraph_path":      "Finance/big.pdf",
			"error":             "The file exceeds the configured size limit",
			"error_reason_code": "msgraph_file_too_large",
			"error_reason":      "The file exceeds the configured size limit",
		},
	}, c.errorItem("b!docs:01A", "big.pdf", "Finance/big.pdf", reasonFileTooLarge))
	require.Equal(t, types.FetchedItem{
		ExternalID: "b!docs:01A", Title: "big.pdf", IsDeleted: true,
		Metadata: map[string]string{
			"channel":          "sharepoint",
			"msgraph_drive_id": "b!docs",
			"msgraph_item_id":  "01A",
			"msgraph_path":     "Finance/big.pdf",
		},
	}, c.tombstone("b!docs:01A", "big.pdf", "Finance/big.pdf"))
}

func TestMetadataForADriveOnlyID(t *testing.T) {
	m := NewConnector(KindOneDrive).tombstone("b!alice", "OneDrive", "").Metadata
	require.Equal(t, "b!alice", m["msgraph_drive_id"])
	require.Empty(t, m["msgraph_item_id"])
}

func TestResolveResourceAncestorsSkipsAVanishedParent(t *testing.T) {
	f := newFakeGraph(t)
	f.json("GET /drives/b!docs/items/01ORPHAN", http.StatusOK, map[string]any{
		"id": "01ORPHAN", "name": "o", "folder": map[string]any{},
		"parentReference": map[string]any{"driveId": "b!docs", "id": "01GONE"},
	})
	f.json("GET /drives/b!pol/items/01A", http.StatusOK, map[string]any{
		"id": "01A", "name": "a", "folder": map[string]any{},
		"parentReference": map[string]any{"driveId": "b!pol", "id": "01ROOT"},
	})
	f.json("GET /drives/b!pol/items/01ROOT", http.StatusOK, map[string]any{
		"id": "01ROOT", "name": "root", "root": map[string]any{}, "folder": map[string]any{},
		"parentReference": map[string]any{"driveId": "b!pol"},
	})
	got, err := testConnector(f, KindSharePoint).ResolveResourceAncestors(
		context.Background(), sharePointConfig(), []string{"b!docs:01ORPHAN", "b!pol:01A"})
	require.NoError(t, err)
	require.Equal(t, []string{"b!pol"}, got)
}
