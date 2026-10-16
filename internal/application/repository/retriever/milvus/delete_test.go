package milvus

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/milvus-io/milvus-proto/go-api/v3/commonpb"
	"github.com/milvus-io/milvus-proto/go-api/v3/milvuspb"
	"github.com/milvus-io/milvus-proto/go-api/v3/schemapb"
	"github.com/stretchr/testify/require"
)

const deleteTestCollection = "aikb_embeddings_1024"

// fakeMilvusDeleteService answers the RPCs the delete path makes. HasCollection
// is DescribeCollection under the hood, so a missing collection is a Describe
// answered with code 100, the same as a Delete against it.
type fakeMilvusDeleteService struct {
	milvuspb.UnimplementedMilvusServiceServer
	mu          sync.Mutex
	collections map[string]bool
	describeErr *commonpb.Status
	deleteErr   *commonpb.Status
	describes   []string
	// events records loads and deletes in arrival order.
	events []string
	calls  int
}

func missingCollectionStatus(name string) *commonpb.Status {
	return &commonpb.Status{
		Code:   100,
		Reason: fmt.Sprintf("can't find collection[database=default][collection=%s]", name),
	}
}

func (s *fakeMilvusDeleteService) DescribeCollection(
	_ context.Context, request *milvuspb.DescribeCollectionRequest,
) (*milvuspb.DescribeCollectionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	name := request.GetCollectionName()
	s.describes = append(s.describes, name)
	if s.describeErr != nil {
		return &milvuspb.DescribeCollectionResponse{Status: s.describeErr}, nil
	}
	if !s.collections[name] {
		return &milvuspb.DescribeCollectionResponse{Status: missingCollectionStatus(name)}, nil
	}
	return &milvuspb.DescribeCollectionResponse{
		Status:       &commonpb.Status{},
		CollectionID: 1,
		Schema: &schemapb.CollectionSchema{
			Name: name,
			Fields: []*schemapb.FieldSchema{
				{FieldID: 100, Name: fieldID, DataType: schemapb.DataType_VarChar, IsPrimaryKey: true},
				{FieldID: 101, Name: fieldContent, DataType: schemapb.DataType_VarChar},
			},
		},
	}, nil
}

func (s *fakeMilvusDeleteService) LoadCollection(
	_ context.Context, request *milvuspb.LoadCollectionRequest,
) (*commonpb.Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.events = append(s.events, "load "+request.GetCollectionName())
	return &commonpb.Status{}, nil
}

func (s *fakeMilvusDeleteService) GetLoadingProgress(
	_ context.Context, _ *milvuspb.GetLoadingProgressRequest,
) (*milvuspb.GetLoadingProgressResponse, error) {
	return &milvuspb.GetLoadingProgressResponse{Status: &commonpb.Status{}, Progress: 100, RefreshProgress: 100}, nil
}

func (s *fakeMilvusDeleteService) Delete(
	_ context.Context, request *milvuspb.DeleteRequest,
) (*milvuspb.MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	name := request.GetCollectionName()
	s.events = append(s.events, "delete "+name)
	if s.deleteErr != nil {
		return &milvuspb.MutationResult{Status: s.deleteErr}, nil
	}
	if !s.collections[name] {
		return &milvuspb.MutationResult{Status: missingCollectionStatus(name)}, nil
	}
	return &milvuspb.MutationResult{Status: &commonpb.Status{}}, nil
}

func (s *fakeMilvusDeleteService) deletes() []string {
	var out []string
	for _, event := range s.events {
		if name, ok := strings.CutPrefix(event, "delete "); ok {
			out = append(out, name)
		}
	}
	return out
}

func newDeleteMilvusHarness(t *testing.T, collections ...string) (*milvusRepository, *fakeMilvusDeleteService) {
	t.Helper()
	fake := &fakeMilvusDeleteService{collections: map[string]bool{}}
	for _, name := range collections {
		fake.collections[name] = true
	}

	c := newFakeMilvusClient(t, fake)

	return &milvusRepository{client: c, collectionBaseName: "aikb_embeddings"}, fake
}

func deleteOperations() []struct {
	name string
	run  func(context.Context, *milvusRepository) error
} {
	return []struct {
		name string
		run  func(context.Context, *milvusRepository) error
	}{
		{"chunk ids", func(ctx context.Context, repo *milvusRepository) error {
			return repo.DeleteByChunkIDList(ctx, []string{"chunk-1"}, 1024, types.KnowledgeBaseTypeDocument)
		}},
		{"knowledge ids", func(ctx context.Context, repo *milvusRepository) error {
			return repo.DeleteByKnowledgeIDList(ctx, []string{"knowledge-1"}, 1024, types.KnowledgeBaseTypeDocument)
		}},
		{"source ids", func(ctx context.Context, repo *milvusRepository) error {
			return repo.DeleteBySourceIDList(ctx, []string{"source-1"}, 1024, types.KnowledgeBaseTypeDocument)
		}},
	}
}

func TestMilvusDeletesSkipMissingCollection(t *testing.T) {
	for _, operation := range deleteOperations() {
		t.Run(operation.name, func(t *testing.T) {
			repo, fake := newDeleteMilvusHarness(t)

			require.NoError(t, operation.run(context.Background(), repo))
			require.Equal(t, []string{deleteTestCollection}, fake.describes)
			require.Empty(t, fake.events, "a missing collection must get no load or delete")
		})
	}
}

// A filter delete on a released collection fails with "collection not loaded",
// so an uncached collection is loaded before the delete reaches it.
func TestMilvusDeletesLoadThenIssueWhenCollectionExists(t *testing.T) {
	for _, operation := range deleteOperations() {
		t.Run(operation.name, func(t *testing.T) {
			repo, fake := newDeleteMilvusHarness(t, deleteTestCollection)

			require.NoError(t, operation.run(context.Background(), repo))
			require.Equal(t, []string{"load " + deleteTestCollection, "delete " + deleteTestCollection}, fake.events)
			_, ok := repo.initializedCollections.Load(1024)
			require.True(t, ok, "the loaded collection must be cached")
		})
	}
}

func TestMilvusDeletePropagatesCollectionCheckFailure(t *testing.T) {
	for _, operation := range deleteOperations() {
		t.Run(operation.name, func(t *testing.T) {
			repo, fake := newDeleteMilvusHarness(t, deleteTestCollection)
			fake.describeErr = &commonpb.Status{Code: 5, Reason: "milvus unavailable"}

			err := operation.run(context.Background(), repo)
			require.ErrorContains(t, err, "failed to check collection existence")
			require.ErrorContains(t, err, "milvus unavailable")
			require.Empty(t, fake.deletes())
		})
	}
}

func TestMilvusDeleteSkipsProbeForInitializedDimension(t *testing.T) {
	repo, fake := newDeleteMilvusHarness(t, deleteTestCollection)
	repo.initializedCollections.Store(1024, true)

	require.NoError(t, repo.DeleteByKnowledgeIDList(
		context.Background(), []string{"knowledge-1"}, 1024, types.KnowledgeBaseTypeDocument,
	))
	require.Empty(t, fake.describes)
	require.Equal(t, []string{"delete " + deleteTestCollection}, fake.events)
}

func TestMilvusDeleteWithEmptyIDListSkipsAllRPCs(t *testing.T) {
	repo, fake := newDeleteMilvusHarness(t, deleteTestCollection)

	ctx := context.Background()
	require.NoError(t, repo.DeleteByChunkIDList(ctx, nil, 1024, types.KnowledgeBaseTypeDocument))
	require.NoError(t, repo.DeleteByKnowledgeIDList(ctx, nil, 1024, types.KnowledgeBaseTypeDocument))
	require.NoError(t, repo.DeleteBySourceIDList(ctx, nil, 1024, types.KnowledgeBaseTypeDocument))
	require.Equal(t, 0, fake.calls)
}

// A collection dropped behind this process leaves the cache stale, so the
// delete skips the probe and Milvus answers code 100. That is a no-op, and the
// caches must go so the following write recreates the collection.
func TestMilvusDeleteTreatsStaleCacheMissingCollectionAsNoOp(t *testing.T) {
	for _, operation := range deleteOperations() {
		t.Run(operation.name, func(t *testing.T) {
			repo, fake := newDeleteMilvusHarness(t)
			repo.initializedCollections.Store(1024, true)
			repo.collectionAnalyzerModes.Store(deleteTestCollection, collectionAnalyzerMulti)

			require.NoError(t, operation.run(context.Background(), repo))
			require.Empty(t, fake.describes)
			require.Equal(t, []string{deleteTestCollection}, fake.deletes())
			_, ok := repo.initializedCollections.Load(1024)
			require.False(t, ok, "the initialized cache must be dropped")
			_, ok = repo.collectionAnalyzerModes.Load(deleteTestCollection)
			require.False(t, ok, "the analyzer mode cache must be dropped")
		})
	}
}

// Any other delete failure, such as a collection that is not loaded, is not a
// missing collection and must fail with the caches left alone.
func TestMilvusDeletePropagatesOtherDeleteFailures(t *testing.T) {
	repo, fake := newDeleteMilvusHarness(t, deleteTestCollection)
	repo.initializedCollections.Store(1024, true)
	fake.deleteErr = &commonpb.Status{Code: 101, Reason: "collection not loaded"}

	err := repo.DeleteBySourceIDList(context.Background(), []string{"source-1"}, 1024, types.KnowledgeBaseTypeDocument)
	require.ErrorContains(t, err, "failed to delete by source IDs")
	require.ErrorContains(t, err, "collection not loaded")
	_, ok := repo.initializedCollections.Load(1024)
	require.True(t, ok)
}
