package datasource

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

// The connectors that set types.SyncCursor.CompleteListing, and only they,
// declare CapabilityFullSyncReconcile: it picks the sync-log retry note and
// whether a full sync collects emitted IDs.
func TestReconcilesOnFullSyncMatchesCompleteListingConnectors(t *testing.T) {
	want := map[string]bool{
		types.ConnectorTypeFeishu: true, types.ConnectorTypeLark: true,
		types.ConnectorTypeFeishuDrive: true, types.ConnectorTypeLarkDrive: true,
		types.ConnectorTypeConfluence: true, types.ConnectorTypeSeafile: true,
		types.ConnectorTypeDingTalk: true,
	}
	for connectorType := range ConnectorMetadataRegistry {
		if got := ReconcilesOnFullSync(connectorType); got != want[connectorType] {
			t.Errorf("ReconcilesOnFullSync(%q) = %v, want %v", connectorType, got, want[connectorType])
		}
	}
}

func TestFeishuMetadataDoesNotAdvertiseWebhook(t *testing.T) {
	meta := ConnectorMetadataRegistry[types.ConnectorTypeFeishu]

	for _, capability := range meta.Capabilities {
		if capability == "webhook" {
			t.Fatalf("Feishu connector should not advertise webhook until webhook sync is implemented")
		}
	}
}
