package syncreason

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource/connector/bitbucket"
	"github.com/jrmatherly/ai-kb/internal/datasource/connector/confluence"
	"github.com/jrmatherly/ai-kb/internal/datasource/connector/dingtalk"
	feishu "github.com/jrmatherly/ai-kb/internal/datasource/connector/feishu/core"
	"github.com/jrmatherly/ai-kb/internal/datasource/connector/github"
	"github.com/jrmatherly/ai-kb/internal/datasource/connector/seafile"
	"github.com/jrmatherly/ai-kb/internal/types"
)

// Every sync item reason code the backend emits is rendered by the frontend as
// datasource.syncError.<code>, so each emitter's table and the en-US bundle
// must list the same codes. This test-only package exists so the check depends
// on the emitters and nothing depends on it.
func TestSyncReasonCodesMatchFrontendKeys(t *testing.T) {
	var backend []string
	for _, texts := range []map[string]string{
		feishu.SyncReasonTexts, confluence.SyncReasonTexts, dingtalk.SyncReasonTexts, seafile.SyncReasonTexts,
		github.SyncReasonTexts, bitbucket.SyncReasonTexts,
	} {
		for code := range texts {
			backend = append(backend, code)
		}
	}
	backend = append(backend, types.SyncReasonCodes...)
	slices.Sort(backend)

	frontend := frontendSyncErrorKeys(t)
	for _, code := range backend {
		if !slices.Contains(frontend, code) {
			t.Errorf("backend emits %q but en-US.ts has no datasource.syncError.%s", code, code)
		}
	}
	for _, key := range frontend {
		if !slices.Contains(backend, key) {
			t.Errorf("datasource.syncError.%s has no backend emitter", key)
		}
	}
}

func frontendSyncErrorKeys(t *testing.T) []string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "frontend", "src", "i18n", "locales", "en-US.ts")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)\n    syncError: \{\n(.*?)\n    \}`).FindSubmatch(content)
	if block == nil {
		t.Fatalf("no datasource.syncError block in %s", path)
	}
	var keys []string
	for _, m := range regexp.MustCompile(`(?m)^      ([a-z_]+): `).FindAllSubmatch(block[1], -1) {
		keys = append(keys, string(m[1]))
	}
	if len(keys) == 0 {
		t.Fatalf("no keys parsed from the datasource.syncError block in %s", path)
	}
	return keys
}
