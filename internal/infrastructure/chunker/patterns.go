// Package chunker - patterns.go is the source of truth for multilingual
// regex patterns used by the heading-aware and heuristic splitters.
//
// Patterns are grouped by purpose (chapter markers, numbering, separators)
// and tagged with a priority that the heuristic splitter uses to rank
// candidate chunk boundaries.
package chunker

import "regexp"

// BoundaryPriority levels for heuristic chunk boundaries. Higher = stronger.
const (
	PrioFormFeed       = 100
	PrioNumberedHead   = 90
	PrioChapterMarker  = 85
	PrioAllCapsHeading = 70
	PrioVisualSep      = 60
	PrioPageFooter     = 50
	PrioBlankBlock     = 40
)

// MarkdownHeadingPattern matches an ATX-style Markdown heading at line start.
// Capture groups: (1) hashes, (2) heading text.
var MarkdownHeadingPattern = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+?)\s*#*\s*$`)

// FormFeedPattern matches the form-feed control character used by some PDF
// converters as a page break marker.
var FormFeedPattern = regexp.MustCompile(`\f`)

// NumberedSectionPattern matches lines starting with numeric or roman numbering
// followed by a non-empty title, e.g. "1. Intro", "2.3 Methods", "IV. Results",
// "2.2.1 Users and Permissions". The trailing dot after a multi-level numeral is optional
// because many technical documents write "1.1 Foo" without a closing dot.
var NumberedSectionPattern = regexp.MustCompile(`(?m)^[ \t]*(?:\d+(?:\.\d+){1,3}\.?|(?:\d+|[IVX]{1,5})\.)[ \t]+\S.{0,200}$`)

// AllCapsHeadingPattern matches short all-caps lines (likely section titles
// rendered without Markdown headings). It requires at least 4 letters and
// up to ~10 words. Trailing colons are tolerated.
var AllCapsHeadingPattern = regexp.MustCompile(`(?m)^[ \t]*([A-ZÄÖÜ][A-ZÄÖÜ \-]{3,80}):?\s*$`)

// VisualSeparatorPattern matches horizontal rules / divider lines used as
// section separators in plain text or pre-Markdown documents.
var VisualSeparatorPattern = regexp.MustCompile(`(?m)^[ \t]*(?:-{3,}|={3,}|\*{3,}|_{3,})[ \t]*$`)

// ExcessiveBlanksPattern matches three or more consecutive newlines, which
// usually denote a hard section break.
var ExcessiveBlanksPattern = regexp.MustCompile(`\n{3,}`)

// PageFooterPattern matches typical "Seite X von Y" / "Page X of Y" lines, and the
// Chinese "page" / "page number" footer (U+9875, U+9875 U+7801).
var PageFooterPattern = regexp.MustCompile("(?mi)^[ \\t]*(?:Seite|Page|\u9875\u7801?)\\s+\\d+(?:\\s*(?:von|of|/)\\s*" +
	"\\d+)?[ \\t]*$")

// GermanChapterPattern matches German chapter / section markers.
var GermanChapterPattern = regexp.MustCompile(`(?m)^[ \t]*(?:Kapitel|Abschnitt|Teil)\s+(?:[0-9]+|[IVX]{1,5})[\.: ].{0,200}$`)

// EnglishChapterPattern matches English chapter / section markers.
var EnglishChapterPattern = regexp.MustCompile(`(?m)^[ \t]*(?:Chapter|Section|Part)\s+(?:[0-9]+|[IVX]{1,5})[\.: ].{0,200}$`)

// ChineseChapterPattern matches Chinese chapter / section markers: the ordinal
// prefix U+7B2C, a Chinese or Arabic numeral, then a unit such as chapter (U+7AE0)
// or section (U+8282). Whitespace between the three parts is tolerated.
var ChineseChapterPattern = regexp.MustCompile("(?m)^[ \\t]*\u7b2c[ \\t]*[\u4e00\u4e8c\u4e09\u56db\u4e94\u516d\u4e03" +
	"\u516b\u4e5d\u5341\u767e\u5343\u96f6\u30070-9]+[ \\t]*(?:\u7ae0|\u8282|\u7bc0|\u90e8\u5206|\u7bc7)[ " +
	"\\t]?.{0,200}$")

// SentenceSeparators returns sentence-level separators tuned for the language.
// The escapes are the full-width period, exclamation mark, question mark and semicolon.
// Used for fine-grained sub-splitting when a section is still too large.
func SentenceSeparators(lang string) []string {
	switch lang {
	case LangChinese:
		return []string{"\u3002", "\uff01", "\uff1f", "\uff1b", "\n"}
	case LangGerman, LangEnglish:
		return []string{". ", "! ", "? ", "; ", "\n"}
	default:
		return []string{"\u3002", "\uff01", "\uff1f", "\uff1b", ". ", "! ", "? ", "; ", "\n"}
	}
}

// ChapterPatternsForLangs returns the chapter-marker regexes that apply for
// the given language hints. An empty / unknown list returns all of them so
// that auto-detected documents still match.
func ChapterPatternsForLangs(langs []string) []*regexp.Regexp {
	if len(langs) == 0 {
		return []*regexp.Regexp{GermanChapterPattern, EnglishChapterPattern, ChineseChapterPattern}
	}
	var out []*regexp.Regexp
	for _, l := range langs {
		switch l {
		case LangGerman:
			out = append(out, GermanChapterPattern)
		case LangEnglish:
			out = append(out, EnglishChapterPattern)
		case LangChinese:
			out = append(out, ChineseChapterPattern)
		}
	}
	if len(out) == 0 {
		out = []*regexp.Regexp{GermanChapterPattern, EnglishChapterPattern, ChineseChapterPattern}
	}
	return out
}
