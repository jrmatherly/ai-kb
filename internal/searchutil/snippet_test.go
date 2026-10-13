package searchutil

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractSnippet_AroundHit(t *testing.T) {
	content := strings.Repeat("\u524d", 80) + "\u90e8\u7f72\u624b\u518c" + strings.Repeat("\u540e", 80)
	got := ExtractSnippet(content, "\u90e8\u7f72")
	assert.Contains(t, got, "\u90e8\u7f72")
	assert.True(t, strings.HasPrefix(got, "... "))
	assert.True(t, strings.HasSuffix(got, " ..."))
	assert.Less(t, len([]rune(got)), 250)
}

func TestExtractSnippet_InvalidRegex(t *testing.T) {
	assert.Empty(t, ExtractSnippet("hello world", "("))
}

func TestExtractSnippet_EmptyInputs(t *testing.T) {
	assert.Empty(t, ExtractSnippet("", "q"))
	assert.Empty(t, ExtractSnippet("body", ""))
}
