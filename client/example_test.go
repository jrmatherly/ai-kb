package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// The server sends references on their own ResponseTypeReferences frame
// (internal/handler/session/helpers.go); the done frame carries none.
func TestExampleUsagePrintsReferences(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/knowledge-bases":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"kb-1","name":"Test Knowledge Base"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/sessions":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"s-1"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/knowledge-chat/s-1":
			w.Header().Set("Content-Type", "text/event-stream")
			knowledgeSSEEvent(t, w, StreamResponse{
				ResponseType:        ResponseTypeReferences,
				KnowledgeReferences: []*SearchResult{{ID: "c-1", Content: "AI chunk content"}},
			}, "")
			knowledgeSSEEvent(t, w, StreamResponse{ResponseType: ResponseTypeAnswer, Content: "An answer."}, "")
			knowledgeSSEEvent(t, w, StreamResponse{ResponseType: ResponseTypeAnswer, Done: true}, "")
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false}`))
		}
	}))
	defer srv.Close()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	exampleUsage(srv.URL)
	os.Stdout = stdout
	_ = w.Close()
	out := <-done

	if want := "An answer.\nReferences:\n1. AI chunk content...\n"; !strings.Contains(out, want) {
		t.Errorf("output does not contain %q:\n%s", want, out)
	}
}
