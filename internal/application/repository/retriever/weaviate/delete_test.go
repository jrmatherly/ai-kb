package weaviate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
	sdk "github.com/weaviate/weaviate-go-client/v5/weaviate"
)

const deleteTestClass = "Aikb_embeddings_1024"

// deleteTestServer stands in for the schema and batch delete endpoints.
// probeStatus and deleteStatus override the answer for every request when set.
type deleteTestServer struct {
	mu           sync.Mutex
	classes      map[string]bool
	probeStatus  int
	deleteStatus int
	deleteBody   string
	probes       []string
	deletes      []string
	requests     int
}

func newDeleteTestServer(t *testing.T, classes ...string) (*weaviateRepository, *deleteTestServer) {
	t.Helper()
	s := &deleteTestServer{classes: map[string]bool{}}
	for _, name := range classes {
		s.classes[name] = true
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case r.URL.Path == "/v1/meta":
			_, _ = w.Write([]byte(`{"version":"1.40.1"}`))
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/schema/"):
			s.requests++
			name := strings.TrimPrefix(r.URL.Path, "/v1/schema/")
			s.probes = append(s.probes, name)
			switch {
			case s.probeStatus != 0:
				w.WriteHeader(s.probeStatus)
				_, _ = w.Write([]byte(weaviateErrorBody("weaviate unavailable")))
			case !s.classes[name]:
				w.WriteHeader(http.StatusNotFound)
			default:
				_, _ = w.Write([]byte(`{"class":"` + name + `"}`))
			}
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/batch/objects":
			s.requests++
			var body struct {
				Match struct {
					Class string `json:"class"`
				} `json:"match"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			name := body.Match.Class
			s.deletes = append(s.deletes, name)
			switch {
			case s.deleteStatus != 0:
				w.WriteHeader(s.deleteStatus)
				_, _ = w.Write([]byte(weaviateErrorBody(s.deleteBody)))
			case !s.classes[name]:
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(weaviateErrorBody("validate: failed to get class: " + name)))
			default:
				_, _ = w.Write([]byte(`{"results":{"matches":1,"successful":1}}`))
			}
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client, err := sdk.NewClient(sdk.Config{Scheme: "http", Host: strings.TrimPrefix(server.URL, "http://")})
	require.NoError(t, err)
	return &weaviateRepository{client: client, collectionBaseName: "Aikb_embeddings"}, s
}

func deleteOperations() []struct {
	name string
	run  func(context.Context, *weaviateRepository) error
} {
	return []struct {
		name string
		run  func(context.Context, *weaviateRepository) error
	}{
		{"chunk ids", func(ctx context.Context, repo *weaviateRepository) error {
			return repo.DeleteByChunkIDList(ctx, []string{"chunk-1"}, 1024, types.KnowledgeBaseTypeDocument)
		}},
		{"knowledge ids", func(ctx context.Context, repo *weaviateRepository) error {
			return repo.DeleteByKnowledgeIDList(ctx, []string{"knowledge-1"}, 1024, types.KnowledgeBaseTypeDocument)
		}},
		{"source ids", func(ctx context.Context, repo *weaviateRepository) error {
			return repo.DeleteBySourceIDList(ctx, []string{"source-1"}, 1024, types.KnowledgeBaseTypeDocument)
		}},
	}
}

func TestWeaviateDeletesSkipMissingCollection(t *testing.T) {
	for _, operation := range deleteOperations() {
		t.Run(operation.name, func(t *testing.T) {
			repo, server := newDeleteTestServer(t)

			require.NoError(t, operation.run(context.Background(), repo))
			require.Equal(t, []string{deleteTestClass}, server.probes)
			require.Empty(t, server.deletes)
		})
	}
}

func TestWeaviateDeletesIssueWhenCollectionExists(t *testing.T) {
	for _, operation := range deleteOperations() {
		t.Run(operation.name, func(t *testing.T) {
			repo, server := newDeleteTestServer(t, deleteTestClass)

			require.NoError(t, operation.run(context.Background(), repo))
			require.Equal(t, []string{deleteTestClass}, server.deletes)
		})
	}
}

func TestWeaviateDeletePropagatesCollectionCheckFailure(t *testing.T) {
	for _, operation := range deleteOperations() {
		t.Run(operation.name, func(t *testing.T) {
			repo, server := newDeleteTestServer(t, deleteTestClass)
			server.probeStatus = http.StatusInternalServerError

			require.ErrorContains(t, operation.run(context.Background(), repo), "failed to check collection existence")
			require.Empty(t, server.deletes)
		})
	}
}

func TestWeaviateDeleteSkipsProbeForInitializedDimension(t *testing.T) {
	repo, server := newDeleteTestServer(t, deleteTestClass)
	repo.initializedCollections.Store(1024, true)

	require.NoError(t, repo.DeleteByKnowledgeIDList(
		context.Background(), []string{"knowledge-1"}, 1024, types.KnowledgeBaseTypeDocument,
	))
	require.Empty(t, server.probes)
	require.Equal(t, []string{deleteTestClass}, server.deletes)
}

func TestWeaviateDeleteWithEmptyIDListSkipsAllRequests(t *testing.T) {
	repo, server := newDeleteTestServer(t, deleteTestClass)

	ctx := context.Background()
	require.NoError(t, repo.DeleteByChunkIDList(ctx, nil, 1024, types.KnowledgeBaseTypeDocument))
	require.NoError(t, repo.DeleteByKnowledgeIDList(ctx, nil, 1024, types.KnowledgeBaseTypeDocument))
	require.NoError(t, repo.DeleteBySourceIDList(ctx, nil, 1024, types.KnowledgeBaseTypeDocument))
	require.Equal(t, 0, server.requests)
}

// A class dropped behind this process leaves the cache stale, so the delete
// skips the probe and Weaviate answers 422. The re-probe finds the class gone,
// so the delete is a no-op and the cache goes so the following write recreates
// the class.
func TestWeaviateDeleteTreatsStaleCacheMissingCollectionAsNoOp(t *testing.T) {
	for _, operation := range deleteOperations() {
		t.Run(operation.name, func(t *testing.T) {
			repo, server := newDeleteTestServer(t)
			repo.initializedCollections.Store(1024, true)

			require.NoError(t, operation.run(context.Background(), repo))
			require.Equal(t, []string{deleteTestClass}, server.deletes)
			require.Equal(t, []string{deleteTestClass}, server.probes)
			_, ok := repo.initializedCollections.Load(1024)
			require.False(t, ok, "the initialized cache must be dropped")
		})
	}
}

// Weaviate also answers 422 for a malformed match, so a 422 while the class
// still exists is a real failure.
func TestWeaviateDeleteReturns422WhenClassStillExists(t *testing.T) {
	repo, server := newDeleteTestServer(t, deleteTestClass)
	repo.initializedCollections.Store(1024, true)
	server.deleteStatus = http.StatusUnprocessableEntity
	server.deleteBody = "validate: empty where filter"

	err := repo.DeleteByChunkIDList(context.Background(), []string{"chunk-1"}, 1024, types.KnowledgeBaseTypeDocument)
	require.ErrorContains(t, err, "failed to delete by chunk IDs")
	require.ErrorContains(t, err, "empty where filter")
	require.Equal(t, []string{deleteTestClass}, server.probes)
	_, ok := repo.initializedCollections.Load(1024)
	require.True(t, ok)
}

// A failed re-probe must not hide the delete failure behind it.
func TestWeaviateDeleteReturns422WhenReprobeFails(t *testing.T) {
	repo, server := newDeleteTestServer(t)
	repo.initializedCollections.Store(1024, true)
	server.probeStatus = http.StatusInternalServerError

	err := repo.DeleteByChunkIDList(context.Background(), []string{"chunk-1"}, 1024, types.KnowledgeBaseTypeDocument)
	require.ErrorContains(t, err, "failed to delete by chunk IDs")
	require.ErrorContains(t, err, "failed to get class: "+deleteTestClass)
	_, ok := repo.initializedCollections.Load(1024)
	require.True(t, ok)
}

func TestWeaviateDeleteDoesNotReprobeOtherFailures(t *testing.T) {
	repo, server := newDeleteTestServer(t, deleteTestClass)
	repo.initializedCollections.Store(1024, true)
	server.deleteStatus = http.StatusInternalServerError
	server.deleteBody = "disk full"

	err := repo.DeleteBySourceIDList(context.Background(), []string{"source-1"}, 1024, types.KnowledgeBaseTypeDocument)
	require.ErrorContains(t, err, "disk full")
	require.Empty(t, server.probes)
	_, ok := repo.initializedCollections.Load(1024)
	require.True(t, ok)
}
