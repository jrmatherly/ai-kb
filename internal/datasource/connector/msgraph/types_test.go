package msgraph

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func baseConfig(settings map[string]interface{}, ids ...string) *types.DataSourceConfig {
	return &types.DataSourceConfig{
		Credentials: map[string]interface{}{
			"tenant_id":     "contoso.onmicrosoft.com",
			"client_id":     "app",
			"client_secret": "s3cret",
		},
		Settings:    settings,
		ResourceIDs: ids,
	}
}

func TestParseConfigOneDrive(t *testing.T) {
	cfg, err := parseConfig(
		KindOneDrive,
		baseConfig(map[string]interface{}{"user": "alice@contoso.com"}, "b!abc", "b!abc:01F"),
	)
	require.NoError(t, err)
	require.Equal(t, "alice@contoso.com", cfg.User)
	require.Equal(t, []root{{DriveID: "b!abc"}, {DriveID: "b!abc", ItemID: "01F"}}, cfg.Roots)
}

func TestParseConfigSharePointSiteURL(t *testing.T) {
	cfg, err := parseConfig(
		KindSharePoint,
		baseConfig(map[string]interface{}{"site_url": "https://contoso.sharepoint.com/sites/hr/"}, "b!abc"),
	)
	require.NoError(t, err)
	require.Equal(t, "contoso.sharepoint.com", cfg.SiteHost)
	require.Equal(t, "sites/hr", cfg.SitePath)

	cfg, err = parseConfig(
		KindSharePoint,
		baseConfig(map[string]interface{}{"site_url": "https://contoso.sharepoint.com"}, "b!abc"),
	)
	require.NoError(t, err)
	require.Equal(t, "", cfg.SitePath)
}

func TestParseConfigPublicFieldFallsBackToCredentials(t *testing.T) {
	ds := baseConfig(nil)
	ds.Credentials["site_url"] = "https://contoso.sharepoint.com/sites/hr"
	cfg, err := parseConfig(KindSharePoint, ds)
	require.NoError(t, err)
	require.Equal(t, "sites/hr", cfg.SitePath)
}

func TestParseConfigRejects(t *testing.T) {
	cases := map[string]*types.DataSourceConfig{
		"missing secret": {Credentials: map[string]interface{}{"tenant_id": "t", "client_id": "c"}},
		"blank tenant": {
			Credentials: map[string]interface{}{"tenant_id": " ", "client_id": "c", "client_secret": "s"},
		},
		"bad site url":  baseConfig(map[string]interface{}{"site_url": "contoso.sharepoint.com/sites/hr"}),
		"http site url": baseConfig(map[string]interface{}{"site_url": "http://contoso.sharepoint.com/sites/hr"}),
		"bad root":      baseConfig(map[string]interface{}{"site_url": "https://c.sharepoint.com/sites/hr"}, ":01F"),
	}
	for name, ds := range cases {
		_, err := parseConfig(KindSharePoint, ds)
		require.ErrorIs(t, err, datasource.ErrInvalidConfig, name)
	}
}

func TestParseConfigDropsDuplicateRoots(t *testing.T) {
	cfg, err := parseConfig(KindSharePoint, sharePointConfigIDs("b!abc", "b!abc:01F", "b!abc"))
	require.NoError(t, err)
	require.Equal(t, []root{{DriveID: "b!abc"}, {DriveID: "b!abc", ItemID: "01F"}}, cfg.Roots)
}

func sharePointConfigIDs(ids ...string) *types.DataSourceConfig {
	return baseConfig(map[string]interface{}{"site_url": "https://c.sharepoint.com/sites/hr"}, ids...)
}

func TestParseConfigWithoutSelectionIsAllowed(t *testing.T) {
	// Validate runs before anything is selected; the sync requires roots itself.
	cfg, err := parseConfig(KindOneDrive, baseConfig(map[string]interface{}{"user": "alice@contoso.com"}))
	require.NoError(t, err)
	require.Empty(t, cfg.Roots)
}

func TestKindConnectorTypes(t *testing.T) {
	require.Equal(t, types.ConnectorTypeOneDrive, KindOneDrive.connectorType())
	require.Equal(t, types.ConnectorTypeSharePoint, KindSharePoint.connectorType())
}

func TestRootID(t *testing.T) {
	require.Equal(t, "b!abc", root{DriveID: "b!abc"}.id())
	require.Equal(t, "b!abc:01F", root{DriveID: "b!abc", ItemID: "01F"}.id())
}

func TestSyncReasonTextsCoverEveryCode(t *testing.T) {
	for _, code := range []string{
		reasonFileTooLarge, reasonEmptyFile, reasonSourceChanged, reasonFetchFailed,
		reasonPermissionDenied, reasonNotFound, reasonMalware, reasonInvalidResponse,
	} {
		require.NotEmpty(t, SyncReasonTexts[code], code)
	}
}
