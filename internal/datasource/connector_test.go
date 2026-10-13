package datasource

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

// The connectors that set types.SyncCursor.CompleteListing, and only they,
// declare CapabilityFullSyncReconcile; all but Seafile record the selected
// resource an item was listed under.
func TestReconcileCapabilitiesMatchConnectors(t *testing.T) {
	reconciles := map[string]bool{
		types.ConnectorTypeFeishu: true, types.ConnectorTypeLark: true,
		types.ConnectorTypeFeishuDrive: true, types.ConnectorTypeLarkDrive: true,
		types.ConnectorTypeConfluence: true, types.ConnectorTypeSeafile: true,
		types.ConnectorTypeDingTalk: true,
	}
	for connectorType := range ConnectorMetadataRegistry {
		if got := HasCapability(connectorType, CapabilityFullSyncReconcile); got != reconciles[connectorType] {
			t.Errorf("full_sync_reconcile(%q) = %v, want %v", connectorType, got, reconciles[connectorType])
		}
		tracks := reconciles[connectorType] && connectorType != types.ConnectorTypeSeafile
		if got := HasCapability(connectorType, CapabilitySelectionTracksSource); got != tracks {
			t.Errorf("selection_tracks_source(%q) = %v, want %v", connectorType, got, tracks)
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
