package browserskill

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadExtensionUsesBuiltArchiveName(t *testing.T) {
	p := filepath.Join(t.TempDir(), "browser-skill-ai-kb-0.3.1.zip")
	if err := os.WriteFile(p, []byte("PK"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BROWSERSKILL_EXTENSION_PATH", p)
	m := &Manager{binary: "bsk"}
	rec := httptest.NewRecorder()
	m.DownloadExtension(rec, httptest.NewRequest(http.MethodGet, "/api/v1/me/browser/extension", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Disposition"); got != "attachment; filename=browser-skill-ai-kb-0.3.1.zip" {
		t.Fatalf("Content-Disposition = %q", got)
	}
}
