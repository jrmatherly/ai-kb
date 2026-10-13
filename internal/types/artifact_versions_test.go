package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArtifactVersionsPreserveBothFiles(t *testing.T) {
	old := MessageArtifact{
		URL: "resource://dHZ_fFslfs0GgJGaJZGjGA", FileName: "contest-info.pptx",
		SourcePath: "/workspace/output/contest-info.pptx",
	}
	next := old
	next.URL = "resource://4N1nAo-FZZoDEExDQz2yoA"
	body := "New version generated. ![PPT](" + old.URL + ")"
	got := ClarifyArtifactVersions(body, MessageArtifacts{next}, MessageArtifacts{old}, "zh-CN")
	require.True(t, strings.HasPrefix(got, body), "must not silently redirect an intentional historical reference")
	require.Contains(t, got, "Referenced previous version")
	require.Contains(t, got, "File generated this turn: ![contest-info.pptx]("+next.URL+")")
	require.Equal(t, got, ClarifyArtifactVersions(got, MessageArtifacts{next}, MessageArtifacts{old}, "zh-CN"))
	require.Equal(t, body, ClarifyArtifactVersions(body, MessageArtifacts{old}, MessageArtifacts{old}, "zh-CN"))
	// An explicit comparison already contains the new file; leave it alone.
	comparison := body + "\n![new](" + next.URL + ")"
	require.Equal(t, comparison, ClarifyArtifactVersions(comparison, MessageArtifacts{next}, MessageArtifacts{old}, "zh-CN"))
	next.SourcePath = "/workspace/output/other/contest-info.pptx"
	require.Equal(t, body, ClarifyArtifactVersions(body, MessageArtifacts{next}, MessageArtifacts{old}, "zh-CN"), "matching file names alone cannot establish a version relationship")
}

func TestArtifactVersionsEscapeMarkdownDelimitersInFileName(t *testing.T) {
	old := MessageArtifact{
		URL: "resource://dHZ_fFslfs0GgJGaJZGjGA", FileName: "report(final).pptx",
		SourcePath: "/workspace/output/report(final).pptx",
	}
	next := old
	next.URL = "resource://4N1nAo-FZZoDEExDQz2yoA"
	got := ClarifyArtifactVersions("![old]("+old.URL+")", MessageArtifacts{next}, MessageArtifacts{old}, "zh-CN")
	require.Contains(t, got, "![report\\(final\\).pptx]("+old.URL+")")
	require.Contains(t, got, "![report\\(final\\).pptx]("+next.URL+")")
	require.True(t, strings.HasSuffix(got, "]("+next.URL+")"))
}
