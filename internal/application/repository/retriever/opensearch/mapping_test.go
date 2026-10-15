package opensearch

import (
	"encoding/json"
	"testing"

	osapi "github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

// Decodes through the SDK type, since opensearch-go v4.7 moved the index map
// behind GetIndices and its own UnmarshalJSON.
func TestExtractMappingFingerprintFromResp(t *testing.T) {
	body := `{"kb_a":{"mappings":{"properties":{"embedding":{"type":"knn_vector","dimension":768,
		"method":{"name":"hnsw","engine":"faiss","space_type":"innerproduct",
		"parameters":{"m":16,"ef_construction":128}}}}}}}`
	var resp osapi.MappingGetResp
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}

	got, err := extractMappingFingerprintFromResp(&resp, "kb_a")
	if err != nil {
		t.Fatal(err)
	}
	want := embFingerprint{Dimension: 768, M: 16, EFConstruction: 128, Engine: "faiss", SpaceType: "innerproduct"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if _, err := extractMappingFingerprintFromResp(&resp, "kb_missing"); err == nil {
		t.Fatal("missing index: want error")
	}
}
