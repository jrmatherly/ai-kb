package bitbucket

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChangeListOrdersTombstonesThenItems(t *testing.T) {
	server := fakeBitbucket(t, fakeRepo{commits: map[string]map[string]string{
		"c1": {"docs/z.md": "z", "docs/b.md": "b", "docs/m.md": "m"},
		"c2": {"docs/a.md": "a", "docs/b.md": "b2", "docs/m.md": "m", "docs/y.md": "y"},
	}})
	c, err := connector(server).configured(sourceConfig(goodToken, selection("ws/r", "main", "docs")))
	require.NoError(t, err)
	ctx := context.Background()

	next, err := c.files(ctx, "ws/r", "c2", docs)
	require.NoError(t, err)
	require.Equal(t, files{"docs/a.md": 1, "docs/b.md": 2, "docs/m.md": 1, "docs/y.md": 1}, next)

	base := repositoryCursor{Head: "c1", Ref: "main", Paths: docs}
	list, gone, unknown, err := c.changeList(ctx, "ws/r", base, "c2", "main", docs, next, nil, false)
	require.NoError(t, err)
	require.False(t, gone)
	require.False(t, unknown)
	require.Equal(t, []change{
		{Path: "docs/z.md", Deleted: true}, {Path: "docs/a.md"}, {Path: "docs/b.md"}, {Path: "docs/y.md"},
	}, list)
	require.Equal(t, []string{"/repositories/ws/r/diffstat/c2..c1" + diffstatQuery},
		server.requestsMatching("/diffstat/"))
	require.Empty(t, server.requestsMatching("/src/c1/"))

	list, gone, unknown, err = c.changeList(ctx, "ws/r", repositoryCursor{}, "c2", "main", docs, next, nil, false)
	require.NoError(t, err)
	require.False(t, gone)
	require.False(t, unknown)
	require.Equal(t, []change{{Path: "docs/a.md"}, {Path: "docs/b.md"}, {Path: "docs/m.md"}, {Path: "docs/y.md"}}, list)
}

func TestLeadsToRoot(t *testing.T) {
	roots := []string{"docs/guides", "src"}
	for dir, want := range map[string]bool{
		"docs": true, "docs/guides": true, "docs/guides/sub": true, "src": true, "docs/api": false, "vendor": false,
		"doc": false,
	} {
		require.Equal(t, want, leadsToRoot(dir, roots), dir)
	}
	require.True(t, leadsToRoot("anything", nil))
}
