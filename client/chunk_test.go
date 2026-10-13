package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The route is registered in internal/router/routes_knowledge.go as
// DELETE /chunks/by-id/:id/questions.
func TestDeleteGeneratedQuestionRoute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/chunks/by-id/c-1/questions" {
			t.Errorf("request = %s %s, want DELETE /api/v1/chunks/by-id/c-1/questions", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["question_id"] != "q-1" {
			t.Errorf("question_id = %q, want q-1", body["question_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"Generated question deleted"}`))
	}))
	defer srv.Close()

	if err := NewClient(srv.URL).DeleteGeneratedQuestion(context.Background(), "c-1", "q-1"); err != nil {
		t.Fatal(err)
	}
}
