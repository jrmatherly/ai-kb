package text_test

import (
	"testing"

	"github.com/jrmatherly/ai-kb/cli/internal/text"
)

const family = "\U0001F468\u200d\U0001F469\u200d\U0001F467" // man, woman, girl joined by ZWJ

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		maxWidth int
		s        string
		want     string
	}{
		{"no truncate ASCII", 20, "hello", "hello"},
		{"truncate ASCII", 5, "hello world", "hell…"},
		{"no truncate CJK", 8, "\u4e2d\u6587\u6d4b\u8bd5", "\u4e2d\u6587\u6d4b\u8bd5"},
		{"truncate CJK", 5, "\u4e2d\u6587\u6d4b\u8bd5", "\u4e2d\u6587…"},
		{"empty", 5, "", ""},
		{"single char fits", 1, "a", "a"},
		{"truncate to width 1", 1, "abc", "…"},
		{"maxWidth zero", 0, "x", ""},
		{"keeps ZWJ emoji whole", 3, family + " family", family + "…"},
		{"drops ZWJ emoji that does not fit", 2, family + " family", "…"},
		{"keeps flags whole", 4, "\U0001F1FA\U0001F1F8\U0001F1EF\U0001F1F5 flags", "\U0001F1FA\U0001F1F8…"},
	}
	for _, tt := range tests {
		got := text.Truncate(tt.maxWidth, tt.s)
		if got != tt.want {
			t.Errorf("Truncate(%d, %q) = %q, want %q", tt.maxWidth, tt.s, got, tt.want)
		}
	}
}
