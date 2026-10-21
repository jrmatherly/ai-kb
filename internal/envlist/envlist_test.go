package envlist

import (
	"maps"
	"slices"
	"testing"
)

func TestParse(t *testing.T) {
	known := []string{"rss", "notion", "feishu"}
	for _, tc := range []struct {
		value       string
		wantEnabled []string
		wantUnknown []string
	}{
		{"", []string{"feishu", "notion", "rss"}, nil},
		{"  \t ", []string{"feishu", "notion", "rss"}, nil},
		{" RSS; feishu|no-such\nNotion,,", []string{"feishu", "notion", "rss"}, []string{"no-such"}},
		{"rss", []string{"rss"}, nil},
		{"nope,other", []string{}, []string{"nope", "other"}},
	} {
		enabled, unknown := Parse(tc.value, known)
		if got := slices.Sorted(maps.Keys(enabled)); !slices.Equal(got, tc.wantEnabled) {
			t.Errorf("Parse(%q) enabled %v, want %v", tc.value, got, tc.wantEnabled)
		}
		if !slices.Equal(unknown, tc.wantUnknown) {
			t.Errorf("Parse(%q) unknown %v, want %v", tc.value, unknown, tc.wantUnknown)
		}
	}
}
