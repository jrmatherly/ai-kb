// Package msgraph is the Microsoft OneDrive and SharePoint data source
// connector over the Microsoft Graph API; one package serves both types.
package msgraph

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
)

// Kind selects the connector type a Connector instance registers as.
type Kind string

// The two kinds, one per registered connector type.
const (
	KindOneDrive   Kind = "onedrive"
	KindSharePoint Kind = "sharepoint"
)

func (k Kind) connectorType() string {
	if k == KindSharePoint {
		return types.ConnectorTypeSharePoint
	}
	return types.ConnectorTypeOneDrive
}

// root is one selected drive (ItemID empty) or folder.
type root struct {
	DriveID string
	ItemID  string
}

func (r root) id() string {
	if r.ItemID == "" {
		return r.DriveID
	}
	return r.DriveID + ":" + r.ItemID
}

type config struct {
	TenantID, ClientID, ClientSecret string
	// User is the OneDrive owner (principal name or object id).
	User string
	// SiteHost and SitePath address the SharePoint site: /sites/{host}:/{path}.
	SiteHost, SitePath string
	Roots              []root
}

const (
	settingUser    = "user"
	settingSiteURL = "site_url"
)

// publicSetting reads a non-secret field from settings, or from the
// credentials map where the editor sends it during a connection test.
func publicSetting(ds *types.DataSourceConfig, key string) string {
	if v, ok := ds.Settings[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	v, _ := ds.Credentials[key].(string)
	return strings.TrimSpace(v)
}

func credential(ds *types.DataSourceConfig, key string) (string, error) {
	v, _ := ds.Credentials[key].(string)
	if strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("%w: %s is required", datasource.ErrInvalidConfig, key)
	}
	return strings.TrimSpace(v), nil
}

func parseConfig(kind Kind, ds *types.DataSourceConfig) (*config, error) {
	if ds == nil {
		return nil, datasource.ErrInvalidConfig
	}
	cfg := &config{}
	var err error
	if cfg.TenantID, err = credential(ds, "tenant_id"); err != nil {
		return nil, err
	}
	if cfg.ClientID, err = credential(ds, "client_id"); err != nil {
		return nil, err
	}
	if cfg.ClientSecret, err = credential(ds, "client_secret"); err != nil {
		return nil, err
	}
	switch kind {
	case KindSharePoint:
		if raw := publicSetting(ds, settingSiteURL); raw != "" {
			if cfg.SiteHost, cfg.SitePath, err = parseSiteURL(raw); err != nil {
				return nil, err
			}
		}
	default:
		cfg.User = publicSetting(ds, settingUser)
	}
	seen := map[string]bool{}
	for _, id := range ds.ResourceIDs {
		r, err := parseResourceID(id)
		if err != nil {
			return nil, err
		}
		if seen[r.id()] {
			continue
		}
		seen[r.id()] = true
		cfg.Roots = append(cfg.Roots, r)
	}
	return cfg, nil
}

// parseSiteURL splits https://host/path into the Graph site address parts.
func parseSiteURL(raw string) (host, sitePath string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return "", "", fmt.Errorf("%w: site_url must be an https URL such as https://contoso.sharepoint.com/sites/hr",
			datasource.ErrInvalidConfig)
	}
	return u.Hostname(), strings.Trim(u.Path, "/"), nil
}

// parseResourceID accepts "<driveId>" or "<driveId>:<itemId>"; drive ids
// carry no colon, so the split is at the first one.
func parseResourceID(id string) (root, error) {
	driveID, itemID, _ := strings.Cut(strings.TrimSpace(id), ":")
	if driveID == "" || strings.ContainsAny(driveID+itemID, "/ ") {
		return root{}, fmt.Errorf("%w: invalid resource id %q", datasource.ErrInvalidConfig, id)
	}
	return root{DriveID: driveID, ItemID: itemID}, nil
}

// Error reason codes on error items; the frontend localises them as
// datasource.syncError.<code> and SyncReasonTexts holds the English fallbacks.
const (
	reasonFileTooLarge     = "msgraph_file_too_large"
	reasonEmptyFile        = "msgraph_empty_file"
	reasonSourceChanged    = "msgraph_source_changed"
	reasonFetchFailed      = "msgraph_fetch_failed"
	reasonPermissionDenied = "msgraph_permission_denied"
	reasonNotFound         = "msgraph_not_found"
	reasonMalware          = "msgraph_malware_detected"
	reasonInvalidResponse  = "msgraph_invalid_response"
)

// SyncReasonTexts maps every reason code to its English fallback.
var SyncReasonTexts = map[string]string{
	reasonFileTooLarge:     "The file exceeds the configured size limit",
	reasonEmptyFile:        "The file is empty",
	reasonSourceChanged:    "The file changed while it was being fetched",
	reasonFetchFailed:      "Failed to fetch the file from Microsoft Graph",
	reasonPermissionDenied: "Permission to read the file was denied",
	reasonNotFound:         "The file was not found",
	reasonMalware:          "Microsoft flagged the file as malware; it was not imported",
	reasonInvalidResponse:  "Microsoft Graph returned an unexpected response",
}
