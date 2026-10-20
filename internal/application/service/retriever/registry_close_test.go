package retriever

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

type closerEngine struct {
	interfaces.RetrieveEngineService
	closed int
}

func (c *closerEngine) Close() error { c.closed++; return nil }

func TestRegistryClosesEnginesItStopsServing(t *testing.T) {
	r := NewRetrieveEngineRegistry(nil, nil).(*RetrieveEngineRegistry)
	first, second := &closerEngine{}, &closerEngine{}
	r.RegisterWithStoreID("store", first)
	r.RegisterWithStoreID("store", second)
	require.Equal(t, 1, first.closed, "the replaced engine is closed")
	require.Equal(t, 0, second.closed)
	r.UnregisterByStoreID("store")
	require.Equal(t, 1, second.closed, "the removed engine is closed")
	r.UnregisterByStoreID("store")
	require.Equal(t, 1, second.closed, "a second unregister closes nothing")
}
