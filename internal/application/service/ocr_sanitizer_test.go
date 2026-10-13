package service

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestSanitizeOCRText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "   \n\t  ",
			want:  "",
		},
		{
			name:  "pure HTML skeleton with no text",
			input: `<html><body><div class="image"><img/></div></body></html>`,
			want:  "",
		},
		{
			name:  "HTML with only whitespace text",
			input: "<html><body>  \n  </body></html>",
			want:  "",
		},
		{
			name: "valid markdown passes through",
			input: "# \u6807\u9898\n\n\u8fd9\u662f\u4e00\u6bb5\u6b63\u6587\uff0c\u5305\u542b\u4e00\u4e9b\u5185\u5bb9" +
				"\u3002\n\n| \u52171 | \u52172 |\n| --- | --- |\n| \u6570\u636e1 | \u6570\u636e2 |",
			want: "# \u6807\u9898\n\n\u8fd9\u662f\u4e00\u6bb5\u6b63\u6587\uff0c\u5305\u542b\u4e00\u4e9b\u5185\u5bb9" +
				"\u3002\n\n| \u52171 | \u52172 |\n| --- | --- |\n| \u6570\u636e1 | \u6570\u636e2 |",
		},
		{
			name:  "code block wrapper stripped",
			input: "```markdown\n# \u6587\u6863\u6807\u9898\n\n\u6b63\u6587\u5185\u5bb9\u5728\u8fd9\u91cc\u3002\n```",
			want:  "# \u6587\u6863\u6807\u9898\n\n\u6b63\u6587\u5185\u5bb9\u5728\u8fd9\u91cc\u3002",
		},
		{
			name:  "html code block wrapper stripped",
			input: "```html\n<p>\u8fd9\u662f\u4e00\u6bb5\u5185\u5bb9</p>\n```",
			want:  "\u8fd9\u662f\u4e00\u6bb5\u5185\u5bb9",
		},
		{
			name: "HTML document converted to markdown",
			input: "<html><body><h1>\u6807\u9898</h1><p>\u8fd9\u662f\u4e00\u6bb5\u5f88\u957f\u7684\u6b63\u6587\u5185" +
				"\u5bb9\uff0c\u7528\u6765\u6d4b\u8bd5 HTML \u5230 Markdown \u7684\u8f6c\u6362\u3002</p></body></html>",
			want: "# \u6807\u9898\n\n\u8fd9\u662f\u4e00\u6bb5\u5f88\u957f\u7684\u6b63\u6587\u5185\u5bb9\uff0c\u7528" +
				"\u6765\u6d4b\u8bd5 HTML \u5230 Markdown \u7684\u8f6c\u6362\u3002",
		},
		{
			name:  "known empty reply - Chinese",
			input: "\u65e0\u6587\u5b57\u5185\u5bb9",
			want:  "",
		},
		{
			name:  "known empty reply - no text",
			input: "No text",
			want:  "",
		},
		{
			name:  "known empty reply - \u56fe\u7247\u4e2d\u6ca1\u6709\u6587\u5b57",
			input: "\u56fe\u7247\u4e2d\u6ca1\u6709\u6587\u5b57",
			want:  "",
		},
		{
			name:  "plain text with minimal HTML not converted",
			input: "\u8fd9\u662f\u4e00\u6bb5\u6b63\u5e38\u6587\u672c\uff0c\u4ef7\u683c <100 \u5143\u3002",
			want:  "\u8fd9\u662f\u4e00\u6bb5\u6b63\u5e38\u6587\u672c\uff0c\u4ef7\u683c <100 \u5143\u3002",
		},
		{
			name:  "multiple blank lines collapsed",
			input: "\u6bb5\u843d\u4e00\n\n\n\n\n\u6bb5\u843d\u4e8c",
			want:  "\u6bb5\u843d\u4e00\n\n\u6bb5\u843d\u4e8c",
		},
		{
			name: "HTML with substantial text content is converted",
			input: "<div><h2>\u62a5\u544a\u6458\u8981</h2><p>\u672c\u5b63\u5ea6\u8425\u6536\u540c\u6bd4\u589e\u957f " +
				"15%\uff0c\u51c0\u5229\u6da6\u8fbe\u5230 2.3 \u4ebf\u5143\u3002</p><table><tr><th>\u6307\u6807</th><t" +
				"h>\u6570\u503c</th></tr><tr><td>\u8425\u6536</td><td>10\u4ebf</td></tr></table></div>",
			want: "", // placeholder; will be checked for non-empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeOCRText(tt.input)

			if tt.name == "HTML with substantial text content is converted" {
				if got == "" {
					t.Errorf("sanitizeOCRText() returned empty for substantial HTML content")
				}
				if got == tt.input {
					t.Errorf("sanitizeOCRText() did not convert HTML, got original")
				}
				return
			}

			if got != tt.want {
				t.Errorf("sanitizeOCRText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripMarkdownCodeBlock(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no code block",
			input: "just normal text",
			want:  "just normal text",
		},
		{
			name:  "markdown code block",
			input: "```markdown\n# Title\nContent here\n```",
			want:  "# Title\nContent here",
		},
		{
			name:  "html code block",
			input: "```html\n<p>hello</p>\n```",
			want:  "<p>hello</p>",
		},
		{
			name:  "plain code block",
			input: "```\nsome text\n```",
			want:  "some text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdownCodeBlock(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdownCodeBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLooksLikeHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "HTML document",
			input: "<html><body><p>text</p></body></html>",
			want:  true,
		},
		{
			name:  "DOCTYPE",
			input: "<!DOCTYPE html><html><body></body></html>",
			want:  true,
		},
		{
			name:  "body tag",
			input: "<body><p>content</p></body>",
			want:  true,
		},
		{
			name:  "plain markdown",
			input: "# Title\n\nSome paragraph text",
			want:  false,
		},
		{
			name:  "text with minor HTML",
			input: "This is mostly text with a <b>bold</b> word.",
			want:  false,
		},
		{
			name:  "heavy HTML tags",
			input: "<div><p><span>x</span></p></div><div><p><span>y</span></p></div>",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeHTML(tt.input)
			if got != tt.want {
				t.Errorf("looksLikeHTML() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSanitizeOCRText_ConvertsInlineHTMLTable guards against a regression where
// an HTML <table> embedded in a markdown body survived sanitization untouched:
// the mixed content does not satisfy looksLikeHTML, so the old HTML-to-markdown
// path never ran and the raw <table> markup reached the chunker.
func TestSanitizeOCRText_ConvertsInlineHTMLTable(t *testing.T) {
	para := "\u672c\u5b63\u5ea6\u516c\u53f8\u6574\u4f53\u7ecf\u8425\u60c5\u51b5\u4fdd\u6301\u7a33\u5b9a\uff0c\u8425" +
		"\u6536\u4e0e\u5229\u6da6\u4e24\u9879\u6838\u5fc3\u6307\u6807\u5747\u5b9e\u73b0\u540c\u6bd4\u589e\u957f\u3002" +
		"\u4e3a\u4e86\u4fbf\u4e8e\u7ba1\u7406\u5c42\u5feb\u901f\u4e86\u89e3\u7ecf\u8425\u6210\u679c\uff0c\u4e0b\u8868" +
		"\u6c47\u603b\u4e86\u62a5\u544a\u671f\u5185\u7684\u4e3b\u8981\u8d22\u52a1\u6570\u636e\uff0c" +
		"\u4f9b\u540e\u7eed\u7ecf\u8425\u5206\u6790\u3001\u9884\u7b97\u7f16\u5236\u4ee5\u53ca\u5e74\u5ea6\u8003\u6838" +
		"\u7b49\u5de5\u4f5c\u53c2\u8003\u4f7f\u7528\uff0c" +
		"\u8bf7\u7ed3\u5408\u5b9e\u9645\u4e1a\u52a1\u60c5\u51b5\u7efc\u5408\u5224\u65ad\uff0c\u5207\u52ff\u8131\u79bb" +
		"\u4e1a\u52a1\u80cc\u666f\u5355\u72ec\u89e3\u8bfb\u5176\u4e2d\u4efb\u4f55\u4e00\u9879\u6570\u5b57\u3002"
	tail := "\u4ee5\u4e0a\u6570\u636e\u5747\u6765\u81ea\u8d22\u52a1\u90e8\u95e8\u5ba1\u6838\u540e\u7684\u6b63\u5f0f" +
		"\u62a5\u8868\uff0c\u7edf\u8ba1\u53e3\u5f84\u4e0e\u4e0a\u4e00\u62a5\u544a\u671f\u4fdd\u6301\u4e00\u81f4\uff0c" +
		"\u672a\u53d1\u751f\u4f1a\u8ba1\u653f\u7b56\u53d8\u66f4\u6216\u8ffd\u6eaf\u8c03\u6574\u3002\u5982\u6709\u7591" +
		"\u95ee\u8bf7\u4e0e\u8d22\u52a1\u90e8\u95e8\u8054\u7cfb\u786e\u8ba4\uff0c" +
		"\u6700\u7ec8\u89e3\u91ca\u6743\u5f52\u516c\u53f8\u8d22\u52a1\u90e8\u95e8\u6240\u6709\u3002"
	input := "# \u62a5\u544a\n\n" + para + "\n\n" +
		"<table><tr><th>\u6307\u6807</th><th>\u6570\u503c</th></tr>" +
		"<tr><td>\u8425\u6536</td><td>10\u4ebf</td></tr>" +
		"<tr><td>\u5229\u6da6</td><td>2.3\u4ebf</td></tr></table>" +
		"\n\n" + tail + "\n\n"

	if looksLikeHTML(input) {
		t.Fatalf("test input unexpectedly looks like HTML; the regression test must exercise the mixed-content path")
	}

	got := sanitizeOCRText(input)

	if strings.Contains(got, "<table") {
		t.Fatalf("expected inline HTML table to be converted, got:\n%s", got)
	}
	if !strings.Contains(got, "# \u62a5\u544a") || !strings.Contains(got,
		"\u6700\u7ec8\u89e3\u91ca\u6743\u5f52\u516c\u53f8\u8d22\u52a1\u90e8\u95e8\u6240\u6709\u3002") {
		t.Fatalf("expected surrounding markdown to be preserved, got:\n%s", got)
	}
	for _, want := range []string{
		"\u6307\u6807", "\u6570\u503c", "\u8425\u6536", "10\u4ebf", "\u5229\u6da6",
		"2.3\u4ebf",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected converted table to retain %q, got:\n%s", want, got)
		}
	}
	if !regexp.MustCompile(`\|[-:]{3,}\|`).MatchString(got) {
		t.Fatalf("expected a GFM table separator row, got:\n%s", got)
	}
}

func TestIsKnownEmptyReply(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"\u65e0\u6587\u5b57\u5185\u5bb9", true},
		{"\u65e0\u6cd5\u8bc6\u522b", true},
		{"no text", true},
		{"No Text", true},
		{"NO CONTENT", true},
		{"empty", true},
		{"\u8fd9\u662f\u6b63\u5e38\u5185\u5bb9", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isKnownEmptyReply(tt.input)
			if got != tt.want {
				t.Errorf("isKnownEmptyReply(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateOCRTextRejectsDegenerateOutput(t *testing.T) {
	for _, tt := range []struct {
		name, input, reason string
	}{
		{"recorded empty cells", strings.Repeat("|  ", 2500), "no_readable_content"},
		{"empty response", " \n\t", "empty_content"},
		{"markdown skeleton", "| --- | --- |\n| | |", "no_readable_content"},
		{"repeated phrase", strings.Repeat("\u4f20\u7403\u6309\u94ae ", 200), "repetitive_content"},
		{"repeated suffix after real text", "\u4f20\u7403\n" + strings.Repeat("|  ", 2500), "repetitive_content"},
		{
			"split delimiter loops", strings.Repeat("|  ", 1250) + "\u4f20\u7403\n" + strings.Repeat("|  ", 1250),
			"repetitive_content",
		},
		{
			"different repeated periods", strings.Repeat("| ",
				600) + "\u4f20\u7403\n" + strings.Repeat("\u53d6\u6d88\u65bd\u6cd5 ", 240),
			"repetitive_content",
		},
		{
			"repeated wrapped lines", strings.Repeat("\u5f00\u59cb\n\u9501\u5b9a\u76ee\u6807\n", 100),
			"repetitive_content",
		},
		{"empty HTML skeleton", "<html><body><div><img/></div></body></html>", "empty_content"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			text, err := validateOCRText(tt.input)
			invalid, ok := err.(*ocrValidationError)
			if text != "" || !ok || invalid.reason != tt.reason {
				t.Fatalf("text=%q error=%v; want rejection %s", text, err, tt.reason)
			}
			if got := sanitizeOCRText(tt.input); got != "" {
				t.Fatal("shared sanitizer must also discard rejected OCR")
			}
		})
	}
}

func TestValidateOCRTextPreservesRealContent(t *testing.T) {
	var table strings.Builder
	table.WriteString("| \u9879\u76ee | \u6570\u503c |\n| --- | --- |\n")
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&table, "| \u9879\u76ee%d | %d |\n", i, i)
	}
	for _, input := range []string{
		"A", "3", "\u4f20\u7403", "A\nB\nL1\nL2\n\u4f20\u7403\n\u4f20\u7403", "0:0\n58.29\n\u53d6\u6d88\u65bd\u6cd5",
		"±", "$x^2 + y^2 = z^2$",
		table.String(), strings.Repeat("\u540c\u4e00\u4e2a\u6309\u94ae\n", 5),
		// A run matches many periods; its overlapping ranges must count once.
		table.String() + strings.Repeat("X", 600),
	} {
		got, err := validateOCRText(input)
		if err != nil || got != strings.TrimSpace(input) {
			t.Fatalf("valid OCR rejected or changed: input=%q got=%q err=%v", input, got, err)
		}
	}
	for _, input := range []string{
		"No text content.", "\u65e0\u6587\u5b57\u5185\u5bb9", "```\nNo text content.\n```",
		"<p>No text content.</p>", "<html><body>No text content.</body></html>",
		"<p><strong>No text content.</strong></p>",
		"```html\n<p>No text content.</p>\n```", "<p>\u65e0\u6587\u5b57\u5185\u5bb9</p>",
		"<p>empty</p>", "<p>no text</p>", "<p>No text content&#46;</p>",
	} {
		if got, err := validateOCRText(input); err != nil || got != "" {
			t.Fatalf("explicit no-text answer should succeed without text: %q %v", got, err)
		}
	}
}
