package msgraph

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func sampleCursor() *cursor {
	c, _ := decodeCursor(nil)
	c.Drives["b!abc"] = "Documents"
	c.Roots["b!abc:01ROOT"] = &rootState{
		Delta: "https://graph.microsoft.com/v1.0/drives/b!abc/items/01ROOT/delta?token=t1",
	}
	c.Folders["b!abc:01ROOT"] = folderState{Parent: "b!abc:top", Name: "Policies", Root: "b!abc:01ROOT"}
	c.Folders["b!abc:01SUB"] = folderState{Parent: "b!abc:01ROOT", Name: "2026", Root: "b!abc:01ROOT"}
	c.Items["b!abc:01A"] = itemState{
		FP:     "qx:a|10|b!abc:01SUB|a.docx",
		Parent: "b!abc:01SUB",
		Name:   "a.docx",
		Root:   "b!abc:01ROOT",
	}
	c.Items["b!abc:01B"] = itemState{
		FP:     "qx:b|20|b!abc:01ROOT|b:c.pdf",
		Parent: "b!abc:01ROOT",
		Name:   "b:c.pdf",
		Root:   "b!abc:01ROOT",
	}
	return c
}

func TestCursorRoundTrip(t *testing.T) {
	c := sampleCursor()
	sc := c.toSyncCursor()
	require.Equal(t, float64(1), sc.ConnectorCursor["schema_version"])
	back, err := decodeCursor(sc)
	require.NoError(t, err)
	require.Equal(t, c.Items, back.Items)
	require.Equal(t, c.Roots["b!abc:01ROOT"].Delta, back.Roots["b!abc:01ROOT"].Delta)
}

func TestCursorRejectsUnknownSchema(t *testing.T) {
	_, err := decodeCursor(&types.SyncCursor{ConnectorCursor: map[string]interface{}{"schema_version": 7}})
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
}

func TestCursorPathWalksFoldersAndSanitizes(t *testing.T) {
	c := sampleCursor()
	require.Equal(t, "Documents/Policies/2026/a.docx", c.path("b!abc:01A"))
	require.Equal(t, "Documents/Policies/b_c.pdf", c.path("b!abc:01B"))
	require.Equal(t, "", c.path("b!abc:missing"))
}

func TestCursorDropSubtreeForgetsNestedFoldersWhateverTheIterationOrder(t *testing.T) {
	for i := 0; i < 20; i++ {
		c, _ := decodeCursor(nil)
		c.Folders["d:F"] = folderState{Parent: "d:top", Name: "F", Root: "d"}
		c.Folders["d:G"] = folderState{Parent: "d:F", Name: "G", Root: "d"}
		c.Folders["d:H"] = folderState{Parent: "d:G", Name: "H", Root: "d"}
		c.Items["d:x"] = itemState{FP: "x", Parent: "d:H", Name: "x.pdf", Root: "d"}
		c.dropSubtree("d:F")
		require.Empty(t, c.Folders, "run %d", i)
		require.Empty(t, c.Items, "run %d", i)
	}
}

func TestCursorPathSkipsTheDriveRootItem(t *testing.T) {
	c, _ := decodeCursor(nil)
	c.Drives["d"] = "Documents"
	c.Folders["d:root"] = folderState{Parent: "", Name: "", Root: "d"}
	c.Folders["d:P"] = folderState{Parent: "d:root", Name: "Policies", Root: "d"}
	c.Items["d:a"] = itemState{FP: "a", Parent: "d:P", Name: "a.docx", Root: "d"}
	require.Equal(t, "Documents/Policies/a.docx", c.path("d:a"))
}

func TestCursorDescendantsAndDrop(t *testing.T) {
	c := sampleCursor()
	require.Equal(t, []string{"b!abc:01A"}, c.descendants("b!abc:01SUB"))
	require.Equal(t, []string{"b!abc:01A", "b!abc:01B"}, c.descendants("b!abc:01ROOT"))
	require.Equal(t, []string{"b!abc:01A", "b!abc:01B"}, c.itemsUnderRoot("b!abc:01ROOT"))
	c.dropSubtree("b!abc:01SUB")
	require.NotContains(t, c.Items, "b!abc:01A")
	require.NotContains(t, c.Folders, "b!abc:01SUB")
	require.Contains(t, c.Items, "b!abc:01B")
}

func TestFingerprintPrefersHashesAndIncludesNameAndParent(t *testing.T) {
	it := driveItem{ID: "01A", Name: "a.docx", Size: 10, ETag: `"e1"`}
	it.ParentReference = &struct {
		DriveID string `json:"driveId"`
		ID      string `json:"id"`
	}{DriveID: "b!abc", ID: "01SUB"}
	require.Equal(t, `etag:"e1"|10|b!abc:01SUB|a.docx`, fingerprint(it))
	it.File = &struct {
		MimeType string `json:"mimeType"`
		Hashes   *struct {
			QuickXorHash string `json:"quickXorHash"`
			Sha1Hash     string `json:"sha1Hash"`
			Sha256Hash   string `json:"sha256Hash"`
		} `json:"hashes"`
	}{}
	it.File.Hashes = &struct {
		QuickXorHash string `json:"quickXorHash"`
		Sha1Hash     string `json:"sha1Hash"`
		Sha256Hash   string `json:"sha256Hash"`
	}{QuickXorHash: "QX"}
	require.Equal(t, "qx:QX|10|b!abc:01SUB|a.docx", fingerprint(it))
	renamed := it
	renamed.Name = "renamed.docx"
	require.NotEqual(t, fingerprint(it), fingerprint(renamed))
}

func TestKnowledgeFileName(t *testing.T) {
	require.Equal(
		t,
		"Documents/Policies/hand_book.docx",
		knowledgeFileName([]string{"Documents", "Policies", "hand?book.docx"}),
	)
	require.Equal(t, "untitled", knowledgeFileName([]string{""}))
}

func TestCursorUnderRootCoversDriveOrFolderSubtree(t *testing.T) {
	c := sampleCursor()
	drive := root{DriveID: "b!abc"}
	require.True(t, c.underRoot("b!abc:01A", "b!abc:01SUB", drive))
	require.False(t, c.underRoot("b!other:01A", "b!other:01SUB", drive))
	folder := root{DriveID: "b!abc", ItemID: "01ROOT"}
	require.True(t, c.underRoot("b!abc:01A", "b!abc:01SUB", folder))
	require.False(t, c.underRoot("b!abc:01Z", "b!abc:top", folder))
}
