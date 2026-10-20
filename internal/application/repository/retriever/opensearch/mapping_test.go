package opensearch

import (
	"strings"
	"testing"
)

func TestExtractMappingFingerprintFromBody(t *testing.T) {
	body := `{"kb_a":{"mappings":{"properties":{"embedding":{"type":"knn_vector","dimension":768,
		"method":{"name":"hnsw","engine":"faiss","space_type":"innerproduct",
		"parameters":{"m":16,"ef_construction":128}}}}}}}`

	got, err := extractMappingFingerprintFromBody(strings.NewReader(body), "kb_a")
	if err != nil {
		t.Fatal(err)
	}
	want := embFingerprint{Dimension: 768, M: 16, EFConstruction: 128, Engine: "faiss", SpaceType: "innerproduct"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if _, err := extractMappingFingerprintFromBody(strings.NewReader(body), "kb_missing"); err == nil {
		t.Fatal("missing index: want error")
	}
	if _, err := extractMappingFingerprintFromBody(nil, "kb_a"); err == nil {
		t.Fatal("empty body: want error")
	}
}
