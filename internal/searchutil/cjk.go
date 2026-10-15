package searchutil

import (
	"strings"
	"unicode"
)

// IsCJK reports whether r is Han, Hiragana, Katakana or Hangul, the scripts
// Lucene's CJKBigramFilter bigrams. The Katakana long vowel marks (U+30FC and
// halfwidth U+FF70) are script Common but sit inside Katakana words.
func IsCJK(r rune) bool {
	return r == '\u30fc' || r == '\uff70' ||
		unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// CJKBigrams splits each run of CJK characters into overlapping bigrams (a
// lone character stays a unigram) and keeps other words whole, so a
// whitespace tokenizer can match CJK text without a dictionary. The result is
// space-separated; delimiters are spaces, punctuation and symbols. Changing
// it changes the SQLite FTS5 and Weaviate content_bigram indexes, which then
// need a rebuild.
func CJKBigrams(text string) string {
	return cjkTokens(text, false)
}

// CJKBigramsAndUnigrams is CJKBigrams plus every CJK character on its own. An
// index built with it also matches one-character queries, which an engine
// without prefix search cannot otherwise find inside a longer run.
func CJKBigramsAndUnigrams(text string) string {
	return cjkTokens(text, true)
}

func cjkTokens(text string, unigrams bool) string {
	var out strings.Builder
	out.Grow(len(text) * 2)
	token := func(runes ...rune) {
		if out.Len() > 0 {
			out.WriteByte(' ')
		}
		for _, r := range runes {
			out.WriteRune(r)
		}
	}
	var run []rune
	wordStart := -1
	flushRun := func() {
		switch {
		case len(run) == 1:
			token(run[0])
		case len(run) > 1:
			for i := 0; i < len(run)-1; i++ {
				token(run[i], run[i+1])
			}
			if unigrams {
				for _, r := range run {
					token(r)
				}
			}
		}
		run = run[:0]
	}
	flushWord := func(end int) {
		if wordStart >= 0 {
			if out.Len() > 0 {
				out.WriteByte(' ')
			}
			out.WriteString(text[wordStart:end])
			wordStart = -1
		}
	}

	for i, r := range text {
		switch {
		case IsCJK(r):
			flushWord(i)
			run = append(run, r)
		case unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r):
			flushRun()
			flushWord(i)
		default:
			flushRun()
			if wordStart < 0 {
				wordStart = i
			}
		}
	}
	flushRun()
	flushWord(len(text))
	return out.String()
}
