package datasource

import (
	"slices"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

// The connectors that set types.SyncCursor.CompleteListing, and only they,
// declare CapabilityFullSyncReconcile; all but Seafile, GitHub and Bitbucket
// record the selected resource an item was listed under. GitHub and Bitbucket
// keep their selection in settings.repositories, not in the resource IDs the
// deselection count reads.
func TestReconcileCapabilitiesMatchConnectors(t *testing.T) {
	reconciles := map[string]bool{
		types.ConnectorTypeFeishu: true, types.ConnectorTypeLark: true,
		types.ConnectorTypeFeishuDrive: true, types.ConnectorTypeLarkDrive: true,
		types.ConnectorTypeConfluence: true, types.ConnectorTypeSeafile: true,
		types.ConnectorTypeDingTalk: true, types.ConnectorTypeGitHub: true,
		types.ConnectorTypeBitbucket: true,
	}
	for connectorType := range ConnectorMetadataRegistry {
		if got := HasCapability(connectorType, CapabilityFullSyncReconcile); got != reconciles[connectorType] {
			t.Errorf("full_sync_reconcile(%q) = %v, want %v", connectorType, got, reconciles[connectorType])
		}
		tracks := reconciles[connectorType] &&
			connectorType != types.ConnectorTypeSeafile && connectorType != types.ConnectorTypeGitHub &&
			connectorType != types.ConnectorTypeBitbucket
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

func TestListAvailableConnectorsMarksAllowList(t *testing.T) {
	enabled := func() []string {
		var got []string
		listed := ListAvailableConnectors()
		if len(listed) != len(ConnectorMetadataRegistry) {
			t.Fatalf("listed %d connectors, want the whole catalog of %d", len(listed), len(ConnectorMetadataRegistry))
		}
		for i, meta := range listed {
			if i > 0 && listed[i-1].Priority > meta.Priority {
				t.Fatalf("%s (priority %d) listed after %s (priority %d)",
					meta.Type, meta.Priority, listed[i-1].Type, listed[i-1].Priority)
			}
			if meta.Enabled {
				got = append(got, meta.Type)
			}
		}
		return got
	}
	t.Setenv(ConnectorTypesEnv, "")
	if got := enabled(); !slices.Equal(got, []string{"notion", "confluence", "bitbucket", "github"}) {
		t.Fatalf("blank allow-list enabled %v, want [notion confluence bitbucket github]", got)
	}
	t.Setenv(ConnectorTypesEnv, " ALL ")
	if got := len(enabled()); got != len(ConnectorMetadataRegistry) {
		t.Fatalf("allow-list all enables %d connectors, want all %d", got, len(ConnectorMetadataRegistry))
	}
	// Mixed separators and case; priority order puts Feishu before RSS.
	t.Setenv(ConnectorTypesEnv, " RSS; feishu|no-such-type ")
	if got := enabled(); !slices.Equal(got, []string{"feishu", "rss"}) {
		t.Fatalf("enabled %v, want [feishu rss]", got)
	}
	t.Setenv(ConnectorTypesEnv, "no-such-type")
	if got := enabled(); len(got) != 0 {
		t.Fatalf("an allow-list of unknown names enabled %v, want none", got)
	}
}

func TestConnectorTypeEnabled(t *testing.T) {
	for _, value := range []string{"", "  "} {
		t.Setenv(ConnectorTypesEnv, value)
		if !ConnectorTypeEnabled(types.ConnectorTypeNotion) || !ConnectorTypeEnabled(types.ConnectorTypeConfluence) ||
			!ConnectorTypeEnabled(types.ConnectorTypeGitHub) || !ConnectorTypeEnabled(types.ConnectorTypeBitbucket) ||
			ConnectorTypeEnabled(types.ConnectorTypeFeishu) || ConnectorTypeEnabled("fake") {
			t.Fatalf("allow-list %q must admit only notion, confluence, github and bitbucket", value)
		}
	}
	for _, value := range []string{"all", " All "} {
		t.Setenv(ConnectorTypesEnv, value)
		if !ConnectorTypeEnabled("fake") || !ConnectorTypeEnabled(types.ConnectorTypeFeishu) {
			t.Fatalf("allow-list %q rejects a type", value)
		}
	}
	t.Setenv(ConnectorTypesEnv, "rss")
	if !ConnectorTypeEnabled(types.ConnectorTypeRSS) || ConnectorTypeEnabled(types.ConnectorTypeNotion) ||
		ConnectorTypeEnabled("fake") {
		t.Fatal(`allow-list "rss" must admit rss and nothing else`)
	}
}
