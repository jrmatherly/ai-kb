package envlist

import (
	"maps"
	"slices"
	"testing"
)

func TestBool(t *testing.T) {
	const name = "AIKB_ENVLIST_TEST_BOOL"
	for value, want := range map[string]bool{
		"": false, "true": true, " TRUE\n": true, "1": true, "false": false, "0": false, "yes": false,
	} {
		t.Setenv(name, value)
		if got := Bool(name); got != want {
			t.Errorf("Bool with %q = %v, want %v", value, got, want)
		}
	}
}

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
		for _, name := range tc.wantEnabled {
			if !enabled[name] {
				t.Errorf("Parse(%q)[%q] = false, want true", tc.value, name)
			}
		}
		if !slices.Equal(unknown, tc.wantUnknown) {
			t.Errorf("Parse(%q) unknown %v, want %v", tc.value, unknown, tc.wantUnknown)
		}
	}
}
