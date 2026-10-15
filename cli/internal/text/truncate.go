package text

import "github.com/mattn/go-runewidth"

const ellipsis = "…"

// Truncate cuts s to at most maxWidth display columns (CJK / emoji occupy
// 2 columns; ASCII 1), appending "…" if truncated. It cuts between grapheme
// clusters, so an emoji ZWJ sequence or a flag is never split.
//
//	maxWidth <= 0   -> ""
//	s already fits  -> s returned unchanged
func Truncate(maxWidth int, s string) string {
	if maxWidth <= 0 {
		return ""
	}
	return runewidth.Truncate(s, maxWidth, ellipsis)
}
