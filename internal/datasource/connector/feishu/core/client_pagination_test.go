package core

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// truncatedPage answers like a Feishu list API that says more pages follow but
// gives no token to fetch them.
func truncatedPage(items any) map[string]any {
	return map[string]any{"code": 0, "data": map[string]any{
		"items": items, "files": items, "has_more": true, "page_token": "", "next_page_token": "",
	}}
}

func TestListWikiNodesRecursiveFrom_TruncatedChildPageIsPartial(t *testing.T) {
	ts, cfg := retryTestServer(t, "/open-apis/wiki/v2/spaces/sp/nodes", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("parent_node_token") == "" {
			writeJSON(w, map[string]any{"code": 0, "data": map[string]any{
				"items":    []map[string]any{{"node_token": "n1", "title": "Parent", "has_child": true}},
				"has_more": false,
			}})
			return
		}
		writeJSON(w, truncatedPage([]map[string]any{{"node_token": "n2", "title": "Child"}}))
	})
	defer ts.Close()

	nodes, err := NewClient(cfg).ListWikiNodesRecursiveFrom(context.Background(), "sp", "")

	var partial *PartialWikiNodeListError
	require.ErrorAs(t, err, &partial)
	require.Len(t, partial.Failures, 1)
	assert.Equal(t, "n1", partial.Failures[0].Node.NodeToken)
	assert.ErrorIs(t, partial.Failures[0].Err, errTruncatedPage)
	require.Len(t, nodes, 1)
	assert.Equal(t, "n1", nodes[0].NodeToken)
}

func TestListWikiNodes_TruncatedTopLevelPageFails(t *testing.T) {
	ts, cfg := retryTestServer(t, "/open-apis/wiki/v2/spaces/sp/nodes", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, truncatedPage([]map[string]any{{"node_token": "n1"}}))
	})
	defer ts.Close()

	nodes, err := NewClient(cfg).ListWikiNodes(context.Background(), "sp", "")

	assert.True(t, errors.Is(err, errTruncatedPage), "err = %v", err)
	assert.Nil(t, nodes)
}

func TestListDriveFilesRecursiveFrom_TruncatedSubfolderPageIsPartial(t *testing.T) {
	ts, cfg := retryTestServer(t, "/open-apis/drive/v1/files", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("folder_token") == "root" {
			writeJSON(w, map[string]any{"code": 0, "data": map[string]any{
				"files": []map[string]any{
					{"token": "doc1", "name": "Doc", "type": "docx"},
					{"token": "sub", "name": "Sub", "type": "folder"},
				},
				"has_more": false,
			}})
			return
		}
		writeJSON(w, truncatedPage([]map[string]any{{"token": "doc2", "name": "Doc 2", "type": "docx"}}))
	})
	defer ts.Close()

	files, err := NewClient(cfg).ListDriveFilesRecursiveFrom(context.Background(), "root")

	var partial *PartialDriveFileListError
	require.ErrorAs(t, err, &partial)
	require.Len(t, partial.Failures, 1)
	assert.Equal(t, "sub", partial.Failures[0].FolderToken)
	assert.ErrorIs(t, partial.Failures[0].Err, errTruncatedPage)
	require.Len(t, files, 1)
	assert.Equal(t, "doc1", files[0].Token)
}
