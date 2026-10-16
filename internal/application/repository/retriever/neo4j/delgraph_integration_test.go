package neo4j

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Run against a disposable Neo4j with APOC and no auth:
// docker run --rm -d -p 17687:7687 -e NEO4J_AUTH=none -e 'NEO4J_PLUGINS=["apoc"]' neo4j:2026.09.0
// AIKB_NEO4J_TEST_URI=bolt://127.0.0.1:17687 go test ./internal/application/repository/retriever/neo4j -run DelGraph
func liveDriver(t *testing.T) (context.Context, neo4j.Driver) {
	t.Helper()
	uri := os.Getenv("AIKB_NEO4J_TEST_URI")
	if uri == "" {
		t.Skip("set AIKB_NEO4J_TEST_URI to run the live Neo4j regression")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	driver, err := neo4j.NewDriver(uri, neo4j.NoAuth())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, driver.Close(context.Background())) })
	require.NoError(t, driver.VerifyConnectivity(ctx))
	return ctx, driver
}

// gqlStatuses returns the GQL status codes a statement's summary carries.
func gqlStatuses(
	ctx context.Context, t *testing.T, driver neo4j.Driver, cypher string, params map[string]any,
) []string {
	t.Helper()
	session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer func() { _ = session.Close(ctx) }()
	res, err := session.Run(ctx, cypher, params)
	require.NoError(t, err)
	summary, err := res.Consume(ctx)
	require.NoError(t, err)
	var codes []string
	for _, status := range summary.GqlStatusObjects() {
		codes = append(codes, status.GqlStatus())
	}
	return codes
}

// The query DelGraph used before 2026-10-16, verbatim: apoc.periodic.iterate
// is deprecated and Neo4j reports it with GQL status 01N01.
func TestDelGraphApocDeprecated(t *testing.T) {
	ctx, driver := liveDriver(t)
	apocDelete := `
		CALL apoc.periodic.iterate(
			"MATCH (n:DelGraphDeprecationProbe {kg: $knowledge_id}) RETURN n",
			"DELETE n",
			{batchSize: 1000, parallel: true, params: {knowledge_id: $knowledge_id}}
		) YIELD batches, total
		RETURN total
	`
	assert.Contains(t, gqlStatuses(ctx, t, driver, apocDelete, map[string]any{"knowledge_id": "none"}), "01N01")
}

// DelGraph removes one knowledge's nodes and relationships, leaves the other
// knowledge's graph alone, and its query carries no deprecation status.
func TestDelGraphIntegration(t *testing.T) {
	ctx, driver := liveDriver(t)
	repo := NewNeo4jRepository(driver).(*Neo4jRepository)
	namespace := types.NameSpace{KnowledgeBase: fmt.Sprintf("delgraph_regression_%d", time.Now().UnixNano())}
	label := repo.Label(namespace)
	// AddGraph labels every node with the knowledge base and the knowledge, and
	// DelGraph matches on both.
	doc1 := repo.Label(types.NameSpace{KnowledgeBase: namespace.KnowledgeBase, Knowledge: "doc1"})
	doc2 := repo.Label(types.NameSpace{KnowledgeBase: namespace.KnowledgeBase, Knowledge: "doc2"})
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, cleanupErr := neo4j.ExecuteQuery(cleanupCtx, driver, "MATCH (n:"+label+") DETACH DELETE n",
			nil, neo4j.EagerResultTransformer)
		assert.NoError(t, cleanupErr)
	})
	_, err := neo4j.ExecuteQuery(ctx, driver, `
		CREATE (a1:`+doc1+` {name: 'Acme', kg: 'doc1'}), (b1:`+doc1+` {name: 'Shanghai', kg: 'doc1'}),
		       (a2:`+doc2+` {name: 'Acme', kg: 'doc2'}), (b2:`+doc2+` {name: 'Shanghai', kg: 'doc2'}),
		       (a1)-[:HEADQUARTERED_IN]->(b1), (a2)-[:HAS_BRANCH_IN]->(b2), (a1)-[:SELF]->(a1)
	`, nil, neo4j.EagerResultTransformer)
	require.NoError(t, err)

	require.NoError(t, repo.DelGraph(ctx, []types.NameSpace{
		{KnowledgeBase: namespace.KnowledgeBase, Knowledge: "doc1"},
	}))

	count := func(kg string) (nodes, rels int64) {
		result, queryErr := neo4j.ExecuteQuery(ctx, driver,
			"MATCH (n:"+label+" {kg: $kg}) OPTIONAL MATCH (n)-[r]-() RETURN count(DISTINCT n), count(DISTINCT r)",
			map[string]any{"kg": kg}, neo4j.EagerResultTransformer)
		require.NoError(t, queryErr)
		require.Len(t, result.Records, 1)
		return result.Records[0].Values[0].(int64), result.Records[0].Values[1].(int64)
	}
	nodes, rels := count("doc1")
	assert.Zero(t, nodes, "doc1 nodes survive")
	assert.Zero(t, rels, "doc1 relationships survive")
	nodes, rels = count("doc2")
	assert.EqualValues(t, 2, nodes, "doc2 nodes deleted")
	assert.EqualValues(t, 1, rels, "doc2 relationship deleted")

	statuses := gqlStatuses(ctx, t, driver, delGraphCypher(label), map[string]any{"knowledge_id": "doc2"})
	assert.NotContains(t, statuses, "01N01")
}
