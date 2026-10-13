package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateKnowledgeFromURLDuplicateReturnsExistingEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":1005,"message":"URL already exists",` +
			`"details":{"code":"duplicate_url","data":{"id":"k-old","knowledge_base_id":"kb-1"}}}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("sk-test"))
	existing, err := c.CreateKnowledgeFromURL(context.Background(), "kb-1",
		CreateKnowledgeFromURLRequest{URL: "https://example.com"})
	if !errors.Is(err, ErrDuplicateURL) {
		t.Fatalf("err = %v, want ErrDuplicateURL", err)
	}
	if existing == nil || existing.ID != "k-old" {
		t.Fatalf("existing = %+v, want ID k-old", existing)
	}
}
