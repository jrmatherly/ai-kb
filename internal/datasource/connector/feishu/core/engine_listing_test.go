package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listingOps lists one node "n1" per resource; a resource named in partial
// reports a partial listing.
type listingOps struct {
	partial map[string]bool
}

func (o listingOps) List(_ context.Context, _ *Client, resourceID string) ([]string, error, error) {
	if o.partial[resourceID] {
		return []string{"n1"}, errors.New("sub-tree listing failed"), nil
	}
	return []string{"n1"}, nil, nil
}

func (listingOps) Token(n string) string         { return n }
func (listingOps) Title(n string) string         { return n }
func (listingOps) ObjType(string) string         { return "docx" }
func (listingOps) EditTime(string) string        { return "1" }
func (listingOps) ResourceNoun() string          { return "nodes" }
func (listingOps) EmptyResourceIDsError() string { return "no resources" }
func (listingOps) LogTag() string                { return "[test]" }

func (listingOps) Fetch(
	_ context.Context, _ *Client, n string, resourceID string, _ bool,
) ([]*types.FetchedItem, error) {
	return []*types.FetchedItem{{ExternalID: n, SourceResourceID: resourceID, Content: []byte("x")}}, nil
}

func (listingOps) ListFailureItems(resourceID string, _ error) []types.FetchedItem {
	return []types.FetchedItem{{ExternalID: resourceID, Metadata: map[string]string{"error": "listing"}}}
}

func (listingOps) DecodeCursorTimes(m map[string]interface{}) map[string]map[string]string {
	out := map[string]map[string]string{}
	for res, v := range m {
		times := map[string]string{}
		for tok, et := range v.(map[string]interface{}) {
			times[tok] = et.(string)
		}
		out[res] = times
	}
	return out
}

func (listingOps) EncodeCursor(times map[string]map[string]string, lastSync time.Time) *types.SyncCursor {
	m := map[string]interface{}{}
	for res, t := range times {
		inner := map[string]interface{}{}
		for tok, et := range t {
			inner[tok] = et
		}
		m[res] = inner
	}
	return &types.SyncCursor{LastSyncTime: lastSync, ConnectorCursor: m}
}

func TestRunSyncReportsCompleteListing(t *testing.T) {
	resumed := &types.SyncCursor{ConnectorCursor: map[string]interface{}{
		"r1": map[string]interface{}{"n1": "1"},
	}}
	for _, tc := range []struct {
		name      string
		cursor    *types.SyncCursor
		partial   map[string]bool
		wantClaim bool
	}{
		{name: "full listing from no cursor", wantClaim: true},
		{name: "one resource listed partially", partial: map[string]bool{"r2": true}},
		{name: "resumed from a checkpoint that skips synced nodes", cursor: resumed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := &types.DataSourceConfig{ResourceIDs: []string{"r1", "r2"}}
			next, err := runSync(context.Background(), nil, config, config.ResourceIDs,
				tc.cursor, &CollectHandler{}, listingOps{partial: tc.partial})
			require.NoError(t, err)
			assert.Equal(t, tc.wantClaim, next.CompleteListing)
		})
	}
}
