package types

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yanyiwu/gojieba"
)

func TestEmbeddedJiebaDictsSegmentWithoutTheModuleCache(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("HOME", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("LocalAppData", cache)

	dir, err := writeJiebaDicts()
	if err != nil {
		t.Fatal(err)
	}
	if rel, err := filepath.Rel(cache, dir); err != nil || rel == "" || rel[0] == '.' {
		t.Fatalf("dictionaries written to %s, outside the user cache %s", dir, cache)
	}

	paths := make([]string, len(jiebaDictFiles))
	for i, f := range jiebaDictFiles {
		paths[i] = filepath.Join(dir, f)
	}
	j := gojieba.NewJieba(paths...)
	defer j.Free()
	// "I came to Tsinghua University in Beijing", gojieba's README example.
	got := j.Cut("\u6211\u6765\u5230\u5317\u4eac\u6e05\u534e\u5927\u5b66", true)
	want := []string{"\u6211", "\u6765\u5230", "\u5317\u4eac", "\u6e05\u534e\u5927\u5b66"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Cut = %q, want %q", got, want)
	}

	before, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	again, err := writeJiebaDicts()
	if err != nil || again != dir {
		t.Fatalf("second write: dir %s, err %v; want %s", again, err, dir)
	}
	after, _ := os.Stat(paths[0])
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("second write rewrote a dictionary that was already in place")
	}
}
