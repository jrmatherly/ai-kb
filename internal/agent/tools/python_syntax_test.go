package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstBrokenPythonQuoteCatchesNestedASCIIQuotes(t *testing.T) {
	src := "slides = [\n" +
		"    (\"\u8fd9\u4e0d\u662f\u4e00\u4e2a\"\u5927\u5e72\u5feb\u4e0a\"\u7684\u591c\u665a\uff0c\u800c\u662f\u4e00" +
		"\u4e2a\u9002\u5408\u6574\u7406\u3001\u6c89\u6dc0\u3001\", False),\n" +
		"]\n"
	line, ok := firstBrokenPythonQuote(src)
	require.True(t, ok)
	assert.Equal(t, 2, line)
}

func TestFirstBrokenPythonQuoteAllowsKeywordAfterString(t *testing.T) {
	_, ok := firstBrokenPythonQuote(`x = "hello" if cond else "bye"`)
	assert.False(t, ok)
}

func TestFirstBrokenPythonQuoteAllowsImplicitConcat(t *testing.T) {
	_, ok := firstBrokenPythonQuote("x = \"hello\" \"world\"\n")
	assert.False(t, ok)
}

func TestFirstBrokenPythonQuoteAllowsSingleQuotedChinese(t *testing.T) {
	_, ok := firstBrokenPythonQuote("x = '\u8fd9\u4e0d\u662f\u4e00\u4e2a\"\u5927\u5e72\u5feb\u4e0a\"\u7684\u591c" +
		"\u665a'\n")
	assert.False(t, ok)
}

func TestFirstBrokenPythonQuoteAllowsCornerQuotes(t *testing.T) {
	_, ok := firstBrokenPythonQuote("x = \"\u8fd9\u4e0d\u662f\u4e00\u4e2a\u300c\u5927\u5e72\u5feb\u4e0a\u300d\u7684" +
		"\u591c\u665a\"\n")
	assert.False(t, ok)
}

func TestFirstBrokenPythonQuoteAllowsFString(t *testing.T) {
	_, ok := firstBrokenPythonQuote("x = f\"hello {name}\"\n")
	assert.False(t, ok)
}

func TestFirstBrokenPythonQuoteReportsUnclosed(t *testing.T) {
	line, ok := firstBrokenPythonQuote("x = \"hello\n")
	require.True(t, ok)
	assert.Equal(t, 1, line)
}

func TestPythonScriptSyntaxHintOnlyForPy(t *testing.T) {
	src := "(\"\u8fd9\u4e0d\u662f\u4e00\u4e2a\"\u5927\u5e72\u5feb\u4e0a\"\u7684\u591c\u665a\")\n"
	assert.Contains(t, pythonScriptSyntaxHint("/workspace/output/x.py", src, ToolEditSandboxFile),
		ToolEditSandboxFile, "the hint must name the tool that can repair this file")
	assert.Contains(t, pythonScriptSyntaxHint(testSkillDir+"/run.py", src, ToolEditSkillFile),
		ToolEditSkillFile, "a skill-tree file is not repairable with edit_sandbox_file")
	assert.Empty(t, pythonScriptSyntaxHint("/workspace/output/x.md", src, ToolEditSandboxFile))
}

func TestPythonSyntaxErrorHint(t *testing.T) {
	assert.Empty(t, pythonSyntaxErrorHint("ModuleNotFoundError: docx"))
	assert.Contains(t, pythonSyntaxErrorHint("SyntaxError: invalid syntax"), "edit_sandbox_file")
}
