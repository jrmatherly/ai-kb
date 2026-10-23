package msgraph

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
)

var _ datasource.FullStreamingConnector = (*Connector)(nil)

// Connector syncs files from Microsoft OneDrive or SharePoint; kind decides
// which type it registers as and where the resource tree starts.
type Connector struct {
	kind Kind
	// The hosts are fixed in production; tests point them at a fake.
	graphBaseURL, loginBaseURL string
}

// NewConnector creates a stateless connector for one kind. Each data source
// carries its own app registration in its encrypted credentials.
func NewConnector(kind Kind) *Connector {
	return &Connector{kind: kind, graphBaseURL: graphBaseURL, loginBaseURL: loginBaseURL}
}

// Type returns the connector type.
func (c *Connector) Type() string { return c.kind.connectorType() }

func (c *Connector) newClient(cfg *config) *client {
	return newClient(c.graphBaseURL, c.loginBaseURL, cfg)
}

func (c *Connector) requireTarget(cfg *config) error {
	switch c.kind {
	case KindSharePoint:
		if cfg.SiteHost == "" {
			return fmt.Errorf("%w: site_url is required", datasource.ErrInvalidConfig)
		}
	default:
		if cfg.User == "" {
			return fmt.Errorf("%w: user is required", datasource.ErrInvalidConfig)
		}
	}
	return nil
}

// rootDrives lists the drives the tree starts from: the site's document
// libraries, or the user's single drive.
func (c *Connector) rootDrives(ctx context.Context, cl *client, cfg *config) ([]driveInfo, error) {
	if c.kind == KindSharePoint {
		site, err := cl.site(ctx, cfg.SiteHost, cfg.SitePath)
		if err != nil {
			return nil, err
		}
		return cl.siteDrives(ctx, site.ID)
	}
	d, err := cl.userDrive(ctx, cfg.User)
	if err != nil {
		return nil, err
	}
	return []driveInfo{*d}, nil
}

// Validate mints a token and, once the user or site URL is set, resolves the
// drive or site too, so a connection test also catches a wrong target.
func (c *Connector) Validate(ctx context.Context, ds *types.DataSourceConfig) error {
	cfg, err := parseConfig(c.kind, ds)
	if err != nil {
		return err
	}
	cl := c.newClient(cfg)
	if c.requireTarget(cfg) != nil {
		_, err := cl.accessToken(ctx)
		return err
	}
	_, err = c.rootDrives(ctx, cl, cfg)
	return err
}

// ListResources returns the root drives for an empty parent and one level of
// child folders otherwise. Files are not pickable: a sync root is a folder.
func (c *Connector) ListResources(
	ctx context.Context, ds *types.DataSourceConfig, parentID string,
) ([]types.Resource, error) {
	cfg, err := parseConfig(c.kind, ds)
	if err != nil {
		return nil, err
	}
	if err := c.requireTarget(cfg); err != nil {
		return nil, err
	}
	cl := c.newClient(cfg)
	out := []types.Resource{}
	if parentID == "" {
		drives, err := c.rootDrives(ctx, cl, cfg)
		if err != nil {
			return nil, err
		}
		for _, d := range drives {
			out = append(
				out,
				types.Resource{ExternalID: d.ID, Name: d.Name, Type: "drive", URL: d.WebURL, HasChildren: true},
			)
		}
		return out, nil
	}
	parent, err := parseResourceID(parentID)
	if err != nil {
		return nil, err
	}
	folders, err := cl.childFolders(ctx, parent.DriveID, parent.ItemID)
	if err != nil {
		return nil, err
	}
	for _, f := range folders {
		out = append(out, types.Resource{
			ExternalID: externalID(parent.DriveID, f.ID), Name: f.Name, Type: "folder", URL: f.WebURL,
			ParentID: parentID, HasChildren: true, ModifiedAt: f.LastModified,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ResolveResourceAncestors walks parentReference ids up to the drive root so
// the lazy picker can reveal a saved folder selection: one request per hop,
// the drive root reported as the drive resource itself. A saved folder that
// no longer exists contributes nothing rather than failing the picker.
func (c *Connector) ResolveResourceAncestors(
	ctx context.Context, ds *types.DataSourceConfig, ids []string,
) ([]string, error) {
	cfg, err := parseConfig(c.kind, ds)
	if err != nil {
		return nil, err
	}
	cl := c.newClient(cfg)
	seen := map[string]bool{}
	out := []string{}
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, id := range ids {
		r, err := parseResourceID(id)
		if err != nil || r.ItemID == "" {
			continue
		}
		it, err := cl.item(ctx, r.DriveID, r.ItemID)
		if errors.Is(err, datasource.ErrResourceNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for hops := 0; hops < maxPaginationHops && it.ParentReference != nil && it.ParentReference.ID != ""; hops++ {
			parent, err := cl.item(ctx, r.DriveID, it.ParentReference.ID)
			if errors.Is(err, datasource.ErrResourceNotFound) {
				break
			}
			if err != nil {
				return nil, err
			}
			if parent.Root != nil || parent.ParentReference == nil || parent.ParentReference.ID == "" {
				add(r.DriveID)
				break
			}
			add(externalID(r.DriveID, parent.ID))
			it = parent
		}
	}
	sort.Strings(out)
	return out, nil
}

// collector gathers a stream for the compatibility methods required by the
// base Connector interface.
type collector struct{ items []types.FetchedItem }

func (h *collector) Emit(_ context.Context, item types.FetchedItem) error {
	h.items = append(h.items, item)
	return nil
}

func (*collector) Checkpoint(context.Context, *types.SyncCursor) error { return nil }

// FetchAll collects what a full FetchStream would emit.
func (c *Connector) FetchAll(ctx context.Context, ds *types.DataSourceConfig, _ []string) ([]types.FetchedItem, error) {
	h := &collector{}
	_, err := c.FetchStream(ctx, ds, nil, h)
	return h.items, err
}

// FetchIncremental collects what FetchStream would emit from old.
func (c *Connector) FetchIncremental(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	h := &collector{}
	next, err := c.FetchStream(ctx, ds, old, h)
	if err != nil {
		return nil, nil, err
	}
	return h.items, next, nil
}

func (c *Connector) metadata(id, p string) map[string]string {
	driveID, itemID, _ := strings.Cut(id, ":")
	return map[string]string{
		"channel":          c.Type(),
		"msgraph_drive_id": driveID,
		"msgraph_item_id":  itemID,
		"msgraph_path":     p,
	}
}

// errorItem follows the service contract for failed fetches: no content, a
// stable reason code and English fallback text.
func (c *Connector) errorItem(id, title, p, code string) types.FetchedItem {
	m := c.metadata(id, p)
	m["error"] = SyncReasonTexts[code]
	m["error_reason_code"] = code
	m["error_reason"] = SyncReasonTexts[code]
	return types.FetchedItem{ExternalID: id, Title: title, Metadata: m}
}

func (c *Connector) tombstone(id, title, p string) types.FetchedItem {
	return types.FetchedItem{ExternalID: id, Title: title, IsDeleted: true, Metadata: c.metadata(id, p)}
}
