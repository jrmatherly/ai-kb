package msgraph

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
)

const cursorSchemaVersion = 1

// rootState is one selected drive or folder's delta position. Next is the
// page link being processed (resume point); Delta is the link for the next
// incremental run; Resync marks an enumeration restarted after a 410. Gen
// counts restarts: every item the restart lists is stamped with it, so a
// restart interrupted by a checkpoint still knows what it has seen, and its
// end tombstones every known item under the root with an older stamp.
type rootState struct {
	Delta  string `json:"delta,omitempty"`
	Next   string `json:"next,omitempty"`
	Resync bool   `json:"resync,omitempty"`
	Gen    int    `json:"gen,omitempty"`
}

type folderState struct {
	Parent string `json:"parent"`
	Name   string `json:"name"`
	Root   string `json:"root"`
	Gen    int    `json:"gen,omitempty"`
}

// itemState is a known file. An empty FP with no Failed code marks a file
// whose fetch failed for a transient reason and is retried before the next
// run's delta walk; Failed records a deterministic failure (too large, empty,
// malware) that is reported once and re-checked only when the file changes.
type itemState struct {
	FP     string `json:"fp"`
	Failed string `json:"failed,omitempty"`
	Parent string `json:"parent"`
	Name   string `json:"name"`
	Root   string `json:"root"`
	Gen    int    `json:"gen,omitempty"`
}

type cursor struct {
	SchemaVersion int                    `json:"schema_version"`
	Roots         map[string]*rootState  `json:"roots"`
	Folders       map[string]folderState `json:"folders"`
	Items         map[string]itemState   `json:"items"`
	Drives        map[string]string      `json:"drives"`
}

func decodeCursor(old *types.SyncCursor) (*cursor, error) {
	c := &cursor{SchemaVersion: cursorSchemaVersion}
	if old != nil && len(old.ConnectorCursor) > 0 {
		c.SchemaVersion = 0
		raw, err := json.Marshal(old.ConnectorCursor)
		if err == nil {
			err = json.Unmarshal(raw, c)
		}
		if err != nil {
			return nil, fmt.Errorf("%w: invalid Microsoft Graph cursor: %w", datasource.ErrInvalidConfig, err)
		}
		if c.SchemaVersion != cursorSchemaVersion {
			return nil, fmt.Errorf("%w: unsupported Microsoft Graph cursor schema %d",
				datasource.ErrInvalidConfig, c.SchemaVersion)
		}
	}
	if c.Roots == nil {
		c.Roots = map[string]*rootState{}
	}
	if c.Folders == nil {
		c.Folders = map[string]folderState{}
	}
	if c.Items == nil {
		c.Items = map[string]itemState{}
	}
	if c.Drives == nil {
		c.Drives = map[string]string{}
	}
	return c, nil
}

// toSyncCursor snapshots the maps through a JSON round trip so the service can
// serialize it after the run moves on.
func (c *cursor) toSyncCursor() *types.SyncCursor {
	raw, _ := json.Marshal(c)
	fields := map[string]interface{}{}
	_ = json.Unmarshal(raw, &fields)
	return &types.SyncCursor{LastSyncTime: time.Now().UTC(), ConnectorCursor: fields}
}

func externalID(driveID, itemID string) string { return driveID + ":" + itemID }

func driveOf(id string) string {
	for i := 0; i < len(id); i++ {
		if id[i] == ':' {
			return id[:i]
		}
	}
	return id
}

// path builds "<drive name>/<folders>/<file>" for a known item by walking
// parent ids; it stops at the first parent the maps do not know. The drive
// root item is recorded with an empty name and contributes no segment. An
// unknown item yields "".
func (c *cursor) path(id string) string {
	it, ok := c.Items[id]
	if !ok {
		return ""
	}
	segments := []string{it.Name}
	for parent := it.Parent; parent != ""; {
		f, ok := c.Folders[parent]
		if !ok {
			break
		}
		if f.Name != "" {
			segments = append([]string{f.Name}, segments...)
		}
		parent = f.Parent
	}
	if name := c.Drives[driveOf(id)]; name != "" {
		segments = append([]string{name}, segments...)
	}
	return knowledgeFileName(segments)
}

// isUnder reports whether the chain of parents starting at parent reaches folderID.
func (c *cursor) isUnder(parent, folderID string) bool {
	for seen := 0; parent != "" && seen < len(c.Folders)+1; seen++ {
		if parent == folderID {
			return true
		}
		f, ok := c.Folders[parent]
		if !ok {
			return false
		}
		parent = f.Parent
	}
	return false
}

// underRoot reports whether an item or folder with the given parent lies in
// the selected root r: a whole drive covers everything in that drive, a
// folder covers its subtree.
func (c *cursor) underRoot(id, parent string, r root) bool {
	if r.ItemID == "" {
		return driveOf(id) == r.DriveID
	}
	return c.isUnder(parent, r.id())
}

// descendants lists the known items inside folderID, sorted for deterministic emission.
func (c *cursor) descendants(folderID string) []string {
	var out []string
	for id, it := range c.Items {
		if c.isUnder(it.Parent, folderID) {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func (c *cursor) itemsUnderRoot(rootID string) []string {
	var out []string
	for id, it := range c.Items {
		if it.Root == rootID {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// dropSubtree forgets a folder, its subfolders and their items. The folder
// ids are collected before anything is deleted: isUnder walks parent chains
// through the same map, so deleting while iterating would orphan subfolders
// whose parent was removed first.
func (c *cursor) dropSubtree(folderID string) {
	for _, id := range c.descendants(folderID) {
		delete(c.Items, id)
	}
	var folders []string
	for id, f := range c.Folders {
		if id == folderID || c.isUnder(f.Parent, folderID) {
			folders = append(folders, id)
		}
	}
	for _, id := range folders {
		delete(c.Folders, id)
	}
}

// fingerprint identifies a file version plus its place and name: a content
// change, a rename or a move re-emits, a sharing or metadata change does not.
func fingerprint(it driveItem) string {
	content := "etag:" + it.ETag
	if it.File != nil && it.File.Hashes != nil {
		switch h := it.File.Hashes; {
		case h.QuickXorHash != "":
			content = "qx:" + h.QuickXorHash
		case h.Sha1Hash != "":
			content = "sha1:" + h.Sha1Hash
		case h.Sha256Hash != "":
			content = "sha256:" + h.Sha256Hash
		}
	}
	parent := ""
	if it.ParentReference != nil {
		parent = externalID(it.ParentReference.DriveID, it.ParentReference.ID)
	}
	return fmt.Sprintf("%s|%d|%s|%s", content, it.Size, parent, it.Name)
}
