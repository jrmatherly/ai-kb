package repofile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePathRejectsTraversal(t *testing.T) {
	if _, err := NormalizePath("docs/../secrets"); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestKnowledgeRelativePathPreservesRepositoryTreeBelowProjectAndBranch(t *testing.T) {
	got := KnowledgeRelativePath("knowledge", "feature/login", "docs/guide/install.md")
	if got != "knowledge-feature-login/docs/guide/install.md" {
		t.Fatalf("knowledge relative path = %q", got)
	}
}

func TestCollapsePathsDropsNestedAndDuplicateRoots(t *testing.T) {
	require.Equal(t, []string{"docs", "src"}, CollapsePaths([]string{"src", "docs/api", "docs", "src"}))
	require.Nil(t, CollapsePaths([]string{"docs", ""}), "an empty root means the whole repository")
}

func TestInScopeMatchesRootOrDescendant(t *testing.T) {
	require.True(t, InScope("docs/a.md", []string{"docs"}))
	require.False(t, InScope("docs-old/a.md", []string{"docs"}))
	require.True(t, InScope("any/file.md", nil))
}

func TestIsSupportedFollowsTheSharedImportSet(t *testing.T) {
	require.True(t, IsSupported("README.MD"))
	require.True(t, IsSupported("notes/plan.xmind"), "every upload-importable extension syncs")
	require.True(t, IsSupported("docs/guide.mdx"))
	require.False(t, IsSupported("main.go"))
}
