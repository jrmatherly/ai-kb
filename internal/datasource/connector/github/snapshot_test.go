package github

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiffOrdersDeletionsThenChanges(t *testing.T) {
	got := diff(snapshot{"a": {SHA: "1"}, "b": {SHA: "2"}}, snapshot{"b": {SHA: "3"}, "c": {SHA: "4"}}, true)
	require.Equal(t, []change{{"a", true}, {"b", false}, {"c", false}}, got)
	got = diff(snapshot{"a": {SHA: "1"}}, snapshot{"a": {SHA: "1"}}, false)
	require.Equal(t, []change{{"a", true}, {"a", false}}, got, "a ref change re-identifies every file")
}

func TestSnapshotFallsBackToSubtreeWalkWhenTruncated(t *testing.T) {
	repo := fakeRepo{
		commits: map[string]fakeCommit{"c1": {files: map[string]string{
			"docs/a.md": "aaaa", "docs/sub/b.md": "bb", "src/c.md": "c",
		}}},
		truncate: true,
	}
	server := fakeGitHub(t, repo)
	cfg := sourceConfig(server.base(), goodToken, selection("o/r", "main", "docs"))
	connector, err := NewConnector().configured(cfg)
	require.NoError(t, err)

	snap, err := connector.snapshot(context.Background(), "o/r", "c1", []string{"docs"})
	require.NoError(t, err)
	require.Equal(t, snapshot{
		"docs/a.md":     {SHA: blobSHA("aaaa"), Size: 4},
		"docs/sub/b.md": {SHA: blobSHA("bb"), Size: 2},
	}, snap)

	trees := "/api/v3/repos/o/r/git/trees/"
	requests := server.requests()
	require.Contains(t, requests, trees+"c1?recursive=1")
	require.Contains(t, requests, trees+repo.treeSHA("c1", ""))
	require.Contains(t, requests, trees+repo.treeSHA("c1", "docs"))
	require.Contains(t, requests, trees+repo.treeSHA("c1", "docs/sub"))
	require.NotContains(t, requests, trees+repo.treeSHA("c1", "src"), "src is outside every root")
}
