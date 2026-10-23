package datasource

import (
	"slices"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

// The truth table of the behavior-changing capabilities. A new connector type
// needs a row here. GitHub and Bitbucket reconcile but do not track their
// selection: they keep it in settings.repositories, not in the resource IDs
// the deselection count reads; Seafile stores the library root while the
// selection may name folders inside it.
func TestCapabilityTableMatchesRegistry(t *testing.T) {
	type row struct{ reconcile, tracksSource, sourceIdentityDedup bool }
	want := map[string]row{
		types.ConnectorTypeFeishu:      {reconcile: true, tracksSource: true},
		types.ConnectorTypeLark:        {reconcile: true, tracksSource: true},
		types.ConnectorTypeFeishuDrive: {reconcile: true, tracksSource: true},
		types.ConnectorTypeLarkDrive:   {reconcile: true, tracksSource: true},
		types.ConnectorTypeDingTalk:    {reconcile: true, tracksSource: true},
		types.ConnectorTypeConfluence:  {reconcile: true, tracksSource: true, sourceIdentityDedup: true},
		types.ConnectorTypeSeafile:     {reconcile: true, sourceIdentityDedup: true},
		types.ConnectorTypeGitHub:      {reconcile: true, sourceIdentityDedup: true},
		types.ConnectorTypeBitbucket:   {reconcile: true, sourceIdentityDedup: true},
		types.ConnectorTypeGitLab:      {sourceIdentityDedup: true},
		types.ConnectorTypeNotion:      {},
		types.ConnectorTypeYuque:       {},
		types.ConnectorTypeIMA:         {},
		types.ConnectorTypeGoogleDrive: {},
		types.ConnectorTypeOneDrive:    {},
		types.ConnectorTypeWebCrawler:  {},
		types.ConnectorTypeSlack:       {},
		types.ConnectorTypeIMAP:        {},
		types.ConnectorTypeRSS:         {},
	}
	for connectorType := range ConnectorMetadataRegistry {
		r, ok := want[connectorType]
		if !ok {
			t.Errorf("%q has no row in the capability table", connectorType)
			continue
		}
		got := row{
			reconcile:           HasCapability(connectorType, CapabilityFullSyncReconcile),
			tracksSource:        HasCapability(connectorType, CapabilitySelectionTracksSource),
			sourceIdentityDedup: HasCapability(connectorType, CapabilitySourceIdentityDedup),
		}
		if got != r {
			t.Errorf("%q capabilities = %+v, want %+v", connectorType, got, r)
		}
	}
	for connectorType := range want {
		if _, ok := ConnectorMetadataRegistry[connectorType]; !ok {
			t.Errorf("table row %q is not a registered connector type", connectorType)
		}
	}
}

func TestSelectionTracksSourceImpliesFullSyncReconcile(t *testing.T) {
	for connectorType := range ConnectorMetadataRegistry {
		if HasCapability(connectorType, CapabilitySelectionTracksSource) &&
			!HasCapability(connectorType, CapabilityFullSyncReconcile) {
			t.Errorf("%q tracks its selection but cannot reconcile a full sync", connectorType)
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
	if got := enabled(); !slices.Equal(got, []string{"notion", "confluence", "github", "bitbucket"}) {
		t.Fatalf("blank allow-list enabled %v, want [notion confluence github bitbucket]", got)
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
