package neo4j

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDelGraphCypherUsesCallInTransactions(t *testing.T) {
	query := delGraphCypher("Kb:Doc")
	assert.Contains(t, query, "MATCH (n:Kb:Doc {kg: $knowledge_id})")
	assert.Contains(t, query, "IN TRANSACTIONS OF 1000 ROWS")
	assert.NotContains(t, query, "apoc")
}
