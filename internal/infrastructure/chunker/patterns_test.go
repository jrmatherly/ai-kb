package chunker

import "testing"

func TestMarkdownHeadingPattern_BasicLevels(t *testing.T) {
	cases := []struct {
		in    string
		match bool
	}{
		{"# Heading 1", true},
		{"## Heading 2", true},
		{"###### Heading 6", true},
		{"####### Too many", false},
		{"#NoSpace", false},
		{"  # Indented", false},
		{"plain text", false},
	}
	for _, c := range cases {
		got := MarkdownHeadingPattern.MatchString(c.in)
		if got != c.match {
			t.Errorf("MarkdownHeadingPattern(%q): got %v want %v", c.in, got, c.match)
		}
	}
}

func TestNumberedSectionPattern(t *testing.T) {
	cases := []struct {
		in    string
		match bool
	}{
		{"1. Introduction", true},
		{"2.3 Methodology", true}, // multi-level numerals no longer require trailing dot
		{"2.3. Methodology", true},
		{"2.2.1 \u7528\u6237\u4e0e\u6743\u9650", true}, // three-level numbering, Chinese title, no trailing dot
		{"3.2.1 \u5355\u673a Docker Compose", true},
		{"IV. Results", true},
		{"1.Introduction", false},     // requires whitespace
		{"1.", false},                 // requires title
		{"1.1", false},                // requires title after numeral
		{"1 NoDotSingleLevel", false}, // single-level numerals still need trailing dot
		{"1.2.3.4.5 TooDeep", false},  // more than 3 sub-levels not accepted
		{"plain text", false},
	}
	for _, c := range cases {
		got := NumberedSectionPattern.MatchString(c.in)
		if got != c.match {
			t.Errorf("NumberedSectionPattern(%q): got %v want %v", c.in, got, c.match)
		}
	}
}

func TestGermanChapterPattern(t *testing.T) {
	cases := []struct {
		in    string
		match bool
	}{
		{"Kapitel 1: Einführung", true},
		{"Abschnitt 2.3 Methodik", true},
		{"Abschnitt 3 Methodik", true},
		{"Teil II Ergebnisse", true},
		{"chapter 1", false},
	}
	for _, c := range cases {
		got := GermanChapterPattern.MatchString(c.in)
		if got != c.match {
			t.Errorf("GermanChapterPattern(%q): got %v want %v", c.in, got, c.match)
		}
	}
}

func TestEnglishChapterPattern(t *testing.T) {
	cases := []struct {
		in    string
		match bool
	}{
		{"Chapter 1: Intro", true},
		{"Section 5 Methods", true},
		{"Part IV Results", true},
		{"Kapitel 1", false},
	}
	for _, c := range cases {
		got := EnglishChapterPattern.MatchString(c.in)
		if got != c.match {
			t.Errorf("EnglishChapterPattern(%q): got %v want %v", c.in, got, c.match)
		}
	}
}

func TestChineseChapterPattern(t *testing.T) {
	cases := []struct {
		in    string
		match bool
	}{
		{"\u7b2c\u4e00\u7ae0 \u5f15\u8a00", true},
		{"\u7b2c3\u8282 \u65b9\u6cd5\u8bba", true},
		{"\u7b2c\u4e8c\u90e8\u5206 \u7ed3\u679c", true},
		{"\u7b2c 1 \u7ae0 \u5f15\u8a00", true}, // space between ordinal prefix / numeral / unit
		{"\u7b2c \u4e00 \u7ae0 \u5f15\u8a00", true},
		{"\u7b2c1\u7ae0 \u5f15\u8a00", true},
		{"Chapter 1", false},
		{"\u7b2c \u7ae0 \u7a7a\u6570\u5b57", false}, // missing numeral
	}
	for _, c := range cases {
		got := ChineseChapterPattern.MatchString(c.in)
		if got != c.match {
			t.Errorf("ChineseChapterPattern(%q): got %v want %v", c.in, got, c.match)
		}
	}
}

func TestVisualSeparatorPattern(t *testing.T) {
	cases := []struct {
		in    string
		match bool
	}{
		{"---", true},
		{"========", true},
		{"***", true},
		{"____", true},
		{"--", false}, // needs at least 3
		{"-- text", false},
	}
	for _, c := range cases {
		got := VisualSeparatorPattern.MatchString(c.in)
		if got != c.match {
			t.Errorf("VisualSeparatorPattern(%q): got %v want %v", c.in, got, c.match)
		}
	}
}

func TestPageFooterPattern(t *testing.T) {
	cases := []struct {
		in    string
		match bool
	}{
		{"Seite 3 von 24", true},
		{"Page 5 of 12", true},
		{"page 7", true},
		{"Seite 9", true},
		{"\u9875 3", true},
		{"\u9875\u7801 3 / 12", true},
		{"Some text", false},
	}
	for _, c := range cases {
		got := PageFooterPattern.MatchString(c.in)
		if got != c.match {
			t.Errorf("PageFooterPattern(%q): got %v want %v", c.in, got, c.match)
		}
	}
}

func TestAllCapsHeadingPattern(t *testing.T) {
	cases := []struct {
		in    string
		match bool
	}{
		{"INTRODUCTION", true},
		{"METHODS AND MATERIALS", true},
		{"Mixed Case Heading", false},
		{"ABC", false}, // <4 chars
	}
	for _, c := range cases {
		got := AllCapsHeadingPattern.MatchString(c.in)
		if got != c.match {
			t.Errorf("AllCapsHeadingPattern(%q): got %v want %v", c.in, got, c.match)
		}
	}
}

func TestSentenceSeparators(t *testing.T) {
	if got := SentenceSeparators(LangChinese); got[0] != "\u3002" {
		t.Errorf("Chinese should start with \u3002, got %v", got)
	}
	if got := SentenceSeparators(LangEnglish); got[0] != ". " {
		t.Errorf("English should start with '. ', got %v", got)
	}
	if got := SentenceSeparators("xx"); len(got) < 5 {
		t.Errorf("Unknown lang should return mixed (>=5 separators), got %v", got)
	}
}

func TestChapterPatternsForLangs(t *testing.T) {
	if got := ChapterPatternsForLangs(nil); len(got) != 3 {
		t.Errorf("nil langs should return all 3, got %d", len(got))
	}
	if got := ChapterPatternsForLangs([]string{LangGerman}); len(got) != 1 {
		t.Errorf("only DE requested should return 1, got %d", len(got))
	}
	if got := ChapterPatternsForLangs([]string{LangGerman, LangChinese}); len(got) != 2 {
		t.Errorf("DE+ZH requested should return 2, got %d", len(got))
	}
	if got := ChapterPatternsForLangs([]string{"xx"}); len(got) != 3 {
		t.Errorf("unknown lang should fall back to all 3, got %d", len(got))
	}
}
