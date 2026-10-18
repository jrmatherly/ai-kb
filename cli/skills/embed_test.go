package skills

import "testing"

// The ingest skill is the page's promise of upload, URL import and notes; a
// skill directory is only shipped when the embed directive names it.
func TestBundledSkillsIncludeIngest(t *testing.T) {
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(list))
	for _, s := range list {
		names = append(names, s.Name)
	}
	want := []string{"ai-kb-ingest", "ai-kb-rag-search", "ai-kb-shared"}
	if len(names) != len(want) {
		t.Fatalf("bundled skills = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("bundled skills = %v, want %v", names, want)
		}
	}
}
