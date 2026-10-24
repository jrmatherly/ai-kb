package msgraph

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/utils"
	"github.com/stretchr/testify/require"
)

func jsonEncode(w io.Writer, body any) error { return json.NewEncoder(w).Encode(body) }

func secutilsAll() utils.ImportCapabilities { return utils.AllImportCapabilities }

// deltaScript serves the pages of one root in order: the first request gets
// pages[0], the next link "?page=1" gets pages[1], and so on. The last page
// carries the deltaLink "?token=done-<n>" where n counts enumerations.
func deltaScript(f *fakeGraph, path string, pages ...[]map[string]any) {
	runs := 0
	f.handle("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		idx := 0
		if p := r.URL.Query().Get("page"); p != "" {
			idx = int(p[0] - '0')
		}
		if r.URL.Query().Get("token") != "" && idx == 0 {
			// an incremental call: answer the change page registered under "changes"
			f.mu.Lock()
			h, ok := f.handlers["GET "+path+"#changes"]
			f.mu.Unlock()
			if ok {
				h(w, r)
				return
			}
			_, _ = w.Write(
				[]byte(
					`{"value":[],"@odata.deltaLink":"` + f.server.URL + path + `?token=` + r.URL.Query().
						Get("token") +
						`"}`,
				),
			)
			return
		}
		body := map[string]any{"value": pages[idx]}
		if idx+1 < len(pages) {
			body["@odata.nextLink"] = f.server.URL + path + "?page=" + string(rune('0'+idx+1))
		} else {
			runs++
			body["@odata.deltaLink"] = f.server.URL + path + "?token=done-" + string(rune('0'+runs))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = jsonEncode(w, body)
	})
}

func serveFile(f *fakeGraph, driveID, itemID string, body string) {
	f.handle("GET /drives/"+driveID+"/items/"+itemID+"/content", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, f.server.URL+"/download/"+itemID, http.StatusFound)
	})
	f.handle("GET /download/"+itemID, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
}

func file(id, name, parent string, body string, hash string) map[string]any {
	return map[string]any{
		"id":                   id,
		"name":                 name,
		"size":                 len(body),
		"eTag":                 `"e-` + id + `"`,
		"webUrl":               "https://x/" + name,
		"lastModifiedDateTime": "2026-10-23T01:00:00Z",
		"file": map[string]any{
			"mimeType": "application/octet-stream",
			"hashes":   map[string]any{"quickXorHash": hash},
		},
		"parentReference": map[string]any{"driveId": "b!docs", "id": parent},
	}
}

func rootItem(id string) map[string]any {
	return map[string]any{
		"id": id, "name": "root", "folder": map[string]any{}, "root": map[string]any{},
		"parentReference": map[string]any{"driveId": "b!docs"},
	}
}

func folder(id, name, parent string) map[string]any {
	return map[string]any{
		"id": id, "name": name, "folder": map[string]any{},
		"parentReference": map[string]any{"driveId": "b!docs", "id": parent},
	}
}

func deleted(id string) map[string]any {
	return map[string]any{"id": id, "deleted": map[string]any{}, "parentReference": map[string]any{"driveId": "b!docs"}}
}

type recorder struct {
	items       []types.FetchedItem
	checkpoints []*types.SyncCursor
}

func (r *recorder) Emit(_ context.Context, item types.FetchedItem) error {
	r.items = append(r.items, item)
	return nil
}

func (r *recorder) Checkpoint(_ context.Context, c *types.SyncCursor) error {
	r.checkpoints = append(r.checkpoints, c)
	return nil
}

func (r *recorder) names() []string {
	var out []string
	for _, it := range r.items {
		switch {
		case it.IsDeleted:
			out = append(out, "-"+it.ExternalID)
		case it.Metadata["error_reason_code"] != "":
			out = append(out, "!"+it.Metadata["error_reason_code"]+":"+it.FileName+it.Title)
		default:
			out = append(out, it.FileName)
		}
	}
	return out
}

// failingRecorder refuses the item titled failOn, the way a handler whose
// ingestion fails mid-page does.
type failingRecorder struct {
	recorder
	failOn string
}

func (r *failingRecorder) Emit(ctx context.Context, item types.FetchedItem) error {
	if item.Title == r.failOn {
		return errors.New("handler refused " + item.Title)
	}
	return r.recorder.Emit(ctx, item)
}

func importsAll() *types.DataSourceConfig {
	ds := sharePointConfig("b!docs")
	ds.Imports = secutilsAll()
	return ds
}

func TestFetchStreamInitialEnumerationEmitsFilesAndCompleteListing(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			folder("01P", "Policies", "01ROOT"),
			file("01A", "a.docx", "01P", "AAAA", "qa"),
		},
		[]map[string]any{file("01B", "b.pdf", "01ROOT", "BBBBBB", "qb"), file("01N", "notes.xyz", "01ROOT", "N", "qn")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BBBBBB")
	rec := &recorder{}
	next, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, rec)
	require.NoError(t, err)
	require.Equal(
		t,
		[]string{"Documents/Policies/a.docx", "Documents/b.pdf"},
		rec.names(),
		"notes.xyz is not importable",
	)
	require.True(t, next.CompleteListing)
	require.Equal(t, "sharepoint", rec.items[0].Metadata["channel"])
	require.Equal(t, "b!docs:01A", rec.items[0].ExternalID)
	require.Equal(t, "b!docs", rec.items[0].SourceResourceID)
	require.Len(t, rec.checkpoints, 2, "one checkpoint per page")
	cur, err := decodeCursor(next)
	require.NoError(t, err)
	require.Contains(t, cur.Roots["b!docs"].Delta, "token=done-1")
	require.Empty(t, cur.Roots["b!docs"].Next)
	require.NotContains(t, cur.Items, "b!docs:01N", "unimportable files are not tracked")
}

func TestFetchStreamFolderAfterChildrenKeepsPath(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			file("01A", "a.docx", "01P", "AAAA", "qa"),
			folder("01P", "Policies", "01ROOT"),
			rootItem("01ROOT"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	rec := &recorder{}
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/Policies/a.docx"}, rec.names())
}

func TestFetchStreamRenameReemitsAndMetadataChangeDoesNot(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			file("01A", "a.docx", "01ROOT", "AAAA", "qa"),
			file("01B", "b.pdf", "01ROOT", "BB", "qb"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	first := &recorder{}
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, first)
	require.NoError(t, err)
	// The incremental page: a.docx renamed (same hash), b.pdf re-sent unchanged.
	f.handle("GET /drives/b!docs/root/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(
			w,
			map[string]any{
				"value": []any{
					file("01A", "renamed.docx", "01ROOT", "AAAA", "qa"),
					file("01B", "b.pdf", "01ROOT", "BB", "qb"),
				},
				"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-2",
			},
		)
	})
	second := &recorder{}
	next, err := c.FetchStream(context.Background(), importsAll(), cur, second)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/renamed.docx"}, second.names())
	require.False(t, next.CompleteListing)
}

func TestFetchStreamDeletedFolderTombstonesDescendants(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			folder("01P", "Policies", "01ROOT"),
			file("01A", "a.docx", "01P", "AAAA", "qa"),
			file("01B", "b.pdf", "01ROOT", "BB", "qb"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	f.handle("GET /drives/b!docs/root/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{deleted("01P")},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-2",
		})
	})
	rec := &recorder{}
	next, err := c.FetchStream(context.Background(), importsAll(), cur, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"-b!docs:01A"}, rec.names())
	require.Equal(t, "Documents/Policies/a.docx", rec.items[0].Metadata["msgraph_path"])
	after, _ := decodeCursor(next)
	require.NotContains(t, after.Items, "b!docs:01A")
	require.NotContains(t, after.Folders, "b!docs:01P")
	require.Contains(t, after.Items, "b!docs:01B")
}

func TestFetchStreamDeselectedRootTombstonesItsItems(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	deltaScript(
		f,
		"/drives/b!pol/root/delta",
		[]map[string]any{rootItem("02ROOT"), file("02A", "p.pdf", "02ROOT", "PP", "qp")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.json("GET /drives/b!pol", http.StatusOK, map[string]any{"id": "b!pol", "name": "Policies"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!pol", "02A", "PP")
	c := testConnector(f, KindSharePoint)
	both := sharePointConfig("b!docs", "b!pol")
	both.Imports = secutilsAll()
	cur, err := c.FetchStream(context.Background(), both, nil, &recorder{})
	require.NoError(t, err)
	rec := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), cur, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"-b!pol:02A"}, rec.names())
}

func TestFetchStreamResumesFromPageLink(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(f, "/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
		[]map[string]any{file("01B", "b.pdf", "01ROOT", "BB", "qb")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	f.handle(
		"GET /download/01B",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
	)
	first := &recorder{}
	_, err := c.FetchStream(context.Background(), importsAll(), nil, first)
	require.NoError(t, err, "a download failure is a per-item error item, not a run failure")
	require.Equal(t, []string{"Documents/a.docx", "!msgraph_fetch_failed:b.pdf"}, first.names())
	mid, err := decodeCursor(first.checkpoints[0])
	require.NoError(t, err)
	require.Contains(t, mid.Roots["b!docs"].Next, "page=1")
	serveFile(f, "b!docs", "01B", "BB")
	second := &recorder{}
	next, err := c.FetchStream(context.Background(), importsAll(), first.checkpoints[0], second)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/b.pdf"}, second.names())
	require.False(t, next.CompleteListing, "a resumed run is not a complete listing")
}

func TestFetchStreamRetriesFailedDownloadsBeforeDelta(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.handle(
		"GET /drives/b!docs/items/01A/content",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
	)
	c := testConnector(f, KindSharePoint)
	first := &recorder{}
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, first)
	require.NoError(t, err)
	require.Equal(t, []string{"!msgraph_fetch_failed:a.docx"}, first.names())
	// A 502, not a 503: this client treats a 503 as a rate limit that ends the run.
	f.json("GET /drives/b!docs/items/01A", http.StatusBadGateway, map[string]any{"error": "upstream"})
	second := &recorder{}
	cur, err = c.FetchStream(context.Background(), importsAll(), cur, second)
	require.NoError(t, err)
	require.Equal(t, []string{"!msgraph_fetch_failed:a.docx"}, second.names())
	mid, _ := decodeCursor(cur)
	require.Equal(t, itemState{}, itemState{FP: mid.Items["b!docs:01A"].FP, Failed: mid.Items["b!docs:01A"].Failed})
	f.json("GET /drives/b!docs/items/01A", http.StatusOK, file("01A", "a.docx", "01ROOT", "AAAA", "qa"))
	serveFile(f, "b!docs", "01A", "AAAA")
	third := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), cur, third)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/a.docx"}, third.names())
}

func TestFetchStreamResyncTombstonesUnseen(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			file("01A", "a.docx", "01ROOT", "AAAA", "qa"),
			file("01B", "b.pdf", "01ROOT", "BB", "qb"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	// The stored deltaLink is gone; the fresh enumeration no longer lists b.pdf.
	f.handle("GET /drives/b!docs/root/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", f.server.URL+"/drives/b!docs/root/delta?fresh=1")
		w.WriteHeader(http.StatusGone)
	})
	f.handle("GET /drives/b!docs/root/delta", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fresh") == "1" {
			_ = jsonEncode(
				w,
				map[string]any{
					"value": []any{
						rootItem("01ROOT"),
						file("01A", "a.docx", "01ROOT", "AAAA", "qa"),
					},
					"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-9",
				},
			)
			return
		}
		w.Header().Set("Location", f.server.URL+"/drives/b!docs/root/delta?fresh=1")
		w.WriteHeader(http.StatusGone)
	})
	rec := &recorder{}
	next, err := c.FetchStream(context.Background(), importsAll(), cur, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"-b!docs:01B"}, rec.names(), "a.docx is unchanged, b.pdf vanished during the gap")
	require.False(t, next.CompleteListing, "a resync is not a fresh enumeration from a nil cursor")
	after, _ := decodeCursor(next)
	require.False(t, after.Roots["b!docs"].Resync)
	require.Contains(t, after.Roots["b!docs"].Delta, "done-9")
}

func TestFetchStreamFileTooLargeAndMalwareAreErrorItemsWithoutDownload(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	big := file("01A", "big.pdf", "01ROOT", "", "qa")
	big["size"] = 1 << 40
	bad := file("01M", "bad.docx", "01ROOT", "MM", "qm")
	bad["malware"] = map[string]any{"description": "eicar"}
	deltaScript(f, "/drives/b!docs/root/delta", []map[string]any{rootItem("01ROOT"), big, bad})
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	rec := &recorder{}
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"!msgraph_file_too_large:big.pdf", "!msgraph_malware_detected:bad.docx"}, rec.names())
	require.Equal(t, 0, f.seen("GET /drives/b!docs/items/01A/content"))
	require.Equal(t, 0, f.seen("GET /drives/b!docs/items/01M/content"))
}

func TestFetchStreamNoRootsIsInvalidConfig(t *testing.T) {
	f := newFakeGraph(t)
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), sharePointConfig(), nil, &recorder{})
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
}

func TestFetchStreamResumedResyncKeepsItemsListedBeforeTheCut(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			file("01A", "a.docx", "01ROOT", "AAAA", "qa"),
			file("01B", "b.pdf", "01ROOT", "BB", "qb"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	// The stored deltaLink is gone. The restart has two pages: a.docx on page 1, b.pdf on page 2. The first
	// attempt is cancelled when it asks for page 2, so the second attempt resumes there with Resync still set.
	f.handle("GET /drives/b!docs/root/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", f.server.URL+"/drives/b!docs/root/delta?fresh=1")
		w.WriteHeader(http.StatusGone)
	})
	first := &recorder{}
	ctx, cancel := context.WithCancel(context.Background())
	pageTwo := false
	f.handle("GET /drives/b!docs/root/delta", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("fresh") {
		case "1":
			_ = jsonEncode(
				w,
				map[string]any{
					"value":           []any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
					"@odata.nextLink": f.server.URL + "/drives/b!docs/root/delta?fresh=2",
				},
			)
		case "2":
			if !pageTwo {
				pageTwo = true
				cancel() // the first attempt is cut when it asks for page 2
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = jsonEncode(w, map[string]any{
				"value":            []any{file("01B", "b.pdf", "01ROOT", "BB", "qb")},
				"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-9",
			})
		default:
			w.Header().Set("Location", f.server.URL+"/drives/b!docs/root/delta?fresh=1")
			w.WriteHeader(http.StatusGone)
		}
	})
	_, err = c.FetchStream(ctx, importsAll(), cur, first)
	require.Error(t, err)
	require.NotEmpty(t, first.checkpoints)
	mid, _ := decodeCursor(first.checkpoints[len(first.checkpoints)-1])
	require.True(t, mid.Roots["b!docs"].Resync, "the checkpoint sits inside the resync")
	require.Contains(t, mid.Roots["b!docs"].Next, "fresh=2")
	second := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), first.checkpoints[len(first.checkpoints)-1], second)
	require.NoError(t, err)
	require.Empty(t, second.names(), "nothing changed, nothing vanished: no tombstones for a.docx or b.pdf")
}

func TestFetchStreamParentOnLaterPageIsFetchedForThePath(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(f, "/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01P", "AAAA", "qa")},
		[]map[string]any{folder("01P", "Policies", "01ROOT")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.json("GET /drives/b!docs/items/01P", http.StatusOK, folder("01P", "Policies", "01ROOT"))
	serveFile(f, "b!docs", "01A", "AAAA")
	rec := &recorder{}
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/Policies/a.docx"}, rec.names())
}

func TestFetchStreamPermanentFailuresAreReportedOnceAndNotRefetched(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	big := file("01A", "big.pdf", "01ROOT", "", "qa")
	big["size"] = 1 << 40
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), big, file("01B", "b.pdf", "01ROOT", "BB", "qb")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	first := &recorder{}
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, first)
	require.NoError(t, err)
	require.Equal(t, []string{"!msgraph_file_too_large:big.pdf", "Documents/b.pdf"}, first.names())
	second := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), cur, second)
	require.NoError(t, err)
	require.Empty(t, second.names(), "a quiet run emits nothing, so the service does not count it as all-failed")
	require.Equal(t, 0, f.seen("GET /drives/b!docs/items/01A?"), "the oversized file is not re-fetched by id")
}

func TestFetchStreamRenameToUnimportableTombstones(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	f.handle("GET /drives/b!docs/root/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{file("01A", "a.xyz", "01ROOT", "AAAA", "qa")},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-2",
		})
	})
	rec := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), cur, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"-b!docs:01A"}, rec.names())
}

func TestFetchStreamMoveBetweenRootsKeepsTheFile(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	// Two folder roots in one drive; a.docx moves from A to B. Root A's stream reports the deletion,
	// root B's stream reports the arrival; whichever order the streams run in, the file stays.
	deltaScript(
		f,
		"/drives/b!docs/items/01A/delta",
		[]map[string]any{folder("01A", "A", "01ROOT"), file("01X", "a.docx", "01A", "AAAA", "qa")},
	)
	deltaScript(f, "/drives/b!docs/items/01B/delta", []map[string]any{folder("01B", "B", "01ROOT")})
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01X", "AAAA")
	c := testConnector(f, KindSharePoint)
	both := sharePointConfig("b!docs:01A", "b!docs:01B")
	both.Imports = secutilsAll()
	cur, err := c.FetchStream(context.Background(), both, nil, &recorder{})
	require.NoError(t, err)
	f.handle("GET /drives/b!docs/items/01A/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{deleted("01X")},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/items/01A/delta?token=done-2",
		})
	})
	f.handle("GET /drives/b!docs/items/01B/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{file("01X", "a.docx", "01B", "AAAA", "qa")},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/items/01B/delta?token=done-2",
		})
	})
	rec := &recorder{}
	next, err := c.FetchStream(context.Background(), both, cur, rec)
	require.NoError(t, err)
	// Roots run in id order: A (tombstone) then B (live). The service keeps the file because live follows
	// the tombstone, and the cursor records it under B.
	require.Equal(t, []string{"-b!docs:01X", "Documents/B/a.docx"}, rec.names())
	after, _ := decodeCursor(next)
	require.Equal(t, "b!docs:01B", after.Items["b!docs:01X"].Root)
	// A later deletion entry from A's stream for an item B now owns is ignored.
	f.handle("GET /drives/b!docs/items/01A/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{deleted("01X")},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/items/01A/delta?token=done-3",
		})
	})
	f.handle("GET /drives/b!docs/items/01B/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/items/01B/delta?token=done-3",
		})
	})
	third := &recorder{}
	_, err = c.FetchStream(context.Background(), both, next, third)
	require.NoError(t, err)
	require.Empty(t, third.names())
}

func TestFetchStreamRepeatedPageLinkIsAnError(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	f.handle("GET /drives/b!docs/root/delta", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(
			w,
			map[string]any{"value": []any{}, "@odata.nextLink": f.server.URL + "/drives/b!docs/root/delta?page=loop"},
		)
	})
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.ErrorContains(t, err, "repeated a page link")
}

func TestFetchStreamRetryPassTurnsAForbiddenItemIntoAnErrorItem(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.handle(
		"GET /drives/b!docs/items/01A/content",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) },
	)
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	f.json(
		"GET /drives/b!docs/items/01A",
		http.StatusForbidden,
		map[string]any{"error": map[string]any{"code": "accessDenied"}},
	)
	rec := &recorder{}
	next, err := c.FetchStream(context.Background(), importsAll(), cur, rec)
	require.NoError(t, err, "one unreadable file does not stop the delta walk")
	require.Equal(t, []string{"!msgraph_permission_denied:a.docx"}, rec.names())
	recorded, _ := decodeCursor(next)
	require.Equal(t, reasonPermissionDenied, recorded.Items["b!docs:01A"].Failed)
	third := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), next, third)
	require.NoError(t, err)
	require.Empty(t, third.names())
	require.Equal(t, 1, f.seen("GET /drives/b!docs/items/01A?"))
}

func TestFetchStreamRotatedSecretFailsTheRunNotTheFile(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.handle("GET /drives/b!docs/items/01A/content", func(w http.ResponseWriter, _ *http.Request) {
		f.tokenStatus = http.StatusUnauthorized // the secret is revoked while the run is in flight
		http.Error(w, `{"error":{"code":"InvalidAuthenticationToken"}}`, http.StatusUnauthorized)
	})
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.ErrorIs(t, err, datasource.ErrInvalidCredentials)
}

func TestFetchStreamReHomedTransientItemIsRetriedUnderTheNewRoot(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/items/01P/delta",
		[]map[string]any{folder("01P", "Policies", "01ROOT"), file("01A", "a.docx", "01P", "AAAA", "qa")},
	)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			folder("01P", "Policies", "01ROOT"),
			file("01A", "a.docx", "01P", "AAAA", "qa"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.handle(
		"GET /drives/b!docs/items/01A/content",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
	)
	c := testConnector(f, KindSharePoint)
	folderOnly := sharePointConfig("b!docs:01P")
	folderOnly.Imports = secutilsAll()
	first := &recorder{}
	cur, err := c.FetchStream(context.Background(), folderOnly, nil, first)
	require.NoError(t, err)
	require.Equal(t, []string{"!msgraph_fetch_failed:a.docx"}, first.names())
	// The folder leaves the selection and its drive enters it: the failed item moves to the drive root,
	// which has no state yet when the retry pass fetches it.
	f.json("GET /drives/b!docs/items/01A", http.StatusOK, file("01A", "a.docx", "01P", "AAAA", "qa"))
	serveFile(f, "b!docs", "01A", "AAAA")
	second := &recorder{}
	next, err := c.FetchStream(context.Background(), importsAll(), cur, second)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/Policies/a.docx"}, second.names())
	after, _ := decodeCursor(next)
	require.Equal(t, "b!docs", after.Items["b!docs:01A"].Root)
}

func TestFetchStreamEmitFailureLeavesTheItemForTheRetry(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(f, "/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
		[]map[string]any{file("01B", "b.pdf", "01ROOT", "BB", "qb"), file("01C", "c.pdf", "01ROOT", "CC", "qc")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	serveFile(f, "b!docs", "01C", "CC")
	c := testConnector(f, KindSharePoint)
	h := &failingRecorder{failOn: "c.pdf"}
	_, err := c.FetchStream(context.Background(), importsAll(), nil, h)
	require.ErrorContains(t, err, "handler refused c.pdf")
	require.Equal(t, []string{"Documents/a.docx", "Documents/b.pdf"}, h.names())
	require.Len(t, h.checkpoints, 1)
	mid, err := decodeCursor(h.checkpoints[0])
	require.NoError(t, err)
	require.Contains(t, mid.Roots["b!docs"].Next, "page=1")
	require.NotContains(t, mid.Items, "b!docs:01B")
	require.NotContains(t, mid.Items, "b!docs:01C")
	second := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), h.checkpoints[0], second)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/b.pdf", "Documents/c.pdf"}, second.names())
}

func TestFetchStreamGrandparentOnLaterPageIsFetchedForThePath(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	// Policies is known when a.docx arrives, but its parent HR is only on the next page.
	deltaScript(f, "/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			folder("01P", "Policies", "01Q"),
			file("01A", "a.docx", "01P", "AAAA", "qa"),
		},
		[]map[string]any{folder("01Q", "HR", "01ROOT")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.json("GET /drives/b!docs/items/01Q", http.StatusOK, folder("01Q", "HR", "01ROOT"))
	serveFile(f, "b!docs", "01A", "AAAA")
	rec := &recorder{}
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/HR/Policies/a.docx"}, rec.names())
}

func TestFetchFullStreamReemitsUnchangedAndTombstonesUnseen(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			file("01A", "a.docx", "01ROOT", "AAAA", "qa"),
			file("01B", "b.pdf", "01ROOT", "BB", "qb"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	// The forced full run enumerates from the start; b.pdf is gone, a.docx is unchanged.
	f.handle("GET /drives/b!docs/root/delta", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-9",
		})
	})
	rec := &recorder{}
	next, err := c.FetchFullStream(context.Background(), importsAll(), cur, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/a.docx", "-b!docs:01B"}, rec.names())
	require.True(t, next.CompleteListing, "a full attempt that ran to the end listed every live file")
	after, _ := decodeCursor(next)
	require.False(t, after.Roots["b!docs"].Full)
	require.False(t, after.Roots["b!docs"].Resync)
	require.Contains(t, after.Roots["b!docs"].Delta, "done-9")
}

func TestFetchFullStreamResumesWithoutReemittingTheFirstPage(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			file("01A", "a.docx", "01ROOT", "AAAA", "qa"),
			file("01B", "b.pdf", "01ROOT", "BB", "qb"),
			file("01C", "c.pdf", "01ROOT", "CC", "qc"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	serveFile(f, "b!docs", "01C", "CC")
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	// The full enumeration has two pages and no longer lists c.pdf.
	f.handle("GET /drives/b!docs/root/delta", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			_ = jsonEncode(w, map[string]any{
				"value":            []any{file("01B", "b.pdf", "01ROOT", "BB", "qb")},
				"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-9",
			})
			return
		}
		_ = jsonEncode(w, map[string]any{
			"value":           []any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
			"@odata.nextLink": f.server.URL + "/drives/b!docs/root/delta?page=1",
		})
	})
	first := &failingRecorder{failOn: "b.pdf"}
	_, err = c.FetchFullStream(context.Background(), importsAll(), cur, first)
	require.ErrorContains(t, err, "handler refused b.pdf")
	require.Equal(t, []string{"Documents/a.docx"}, first.names())
	require.NotEmpty(t, first.checkpoints)
	mid, _ := decodeCursor(first.checkpoints[len(first.checkpoints)-1])
	require.True(t, mid.Roots["b!docs"].Full, "the checkpoint sits inside the full run")
	require.Contains(t, mid.Roots["b!docs"].Next, "page=1")
	second := &recorder{}
	last := first.checkpoints[len(first.checkpoints)-1]
	next, err := c.FetchFullStream(context.Background(), importsAll(), last, second)
	require.NoError(t, err)
	require.Equal(
		t, []string{"Documents/b.pdf", "-b!docs:01C"}, second.names(), "a.docx was re-emitted by the first attempt",
	)
	require.False(t, next.CompleteListing, "a resumed full attempt did not emit the first page")
	after, _ := decodeCursor(next)
	require.False(t, after.Roots["b!docs"].Full)
	require.Contains(t, after.Items, "b!docs:01A")
}

func TestFetchFullStreamResumeDoesNotRestartFinishedRoots(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	// Two folder roots. Root A is one page; root B is two pages, and B's second page is where the first
	// full attempt is cut. Root B also held w.pdf, which the full enumeration no longer lists.
	deltaScript(f, "/drives/b!docs/items/01A/delta",
		[]map[string]any{folder("01A", "A", "01ROOT"), file("01X", "x.docx", "01A", "XX", "qx")},
	)
	deltaScript(f, "/drives/b!docs/items/01B/delta",
		[]map[string]any{folder("01B", "B", "01ROOT"), file("01Y", "y.pdf", "01B", "YY", "qy")},
		[]map[string]any{file("01Z", "z.pdf", "01B", "ZZ", "qz"), file("01W", "w.pdf", "01B", "WW", "qw")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	for id, body := range map[string]string{"01X": "XX", "01Y": "YY", "01Z": "ZZ", "01W": "WW"} {
		serveFile(f, "b!docs", id, body)
	}
	c := testConnector(f, KindSharePoint)
	both := sharePointConfig("b!docs:01A", "b!docs:01B")
	both.Imports = secutilsAll()
	cur, err := c.FetchStream(context.Background(), both, nil, &recorder{})
	require.NoError(t, err)
	deltaScript(f, "/drives/b!docs/items/01B/delta",
		[]map[string]any{folder("01B", "B", "01ROOT"), file("01Y", "y.pdf", "01B", "YY", "qy")},
		[]map[string]any{file("01Z", "z.pdf", "01B", "ZZ", "qz")},
	)
	startsA := func() int { return f.seen("GET /drives/b!docs/items/01A/delta?$select") }
	downloadsX := func() int { return f.seen("GET /download/01X") }
	before, beforeX := startsA(), downloadsX()
	first := &failingRecorder{failOn: "z.pdf"}
	_, err = c.FetchFullStream(context.Background(), both, cur, first)
	require.ErrorContains(t, err, "handler refused z.pdf")
	require.Equal(t, []string{"Documents/A/x.docx", "Documents/B/y.pdf"}, first.names())
	last := first.checkpoints[len(first.checkpoints)-1]
	mid, _ := decodeCursor(last)
	require.False(t, mid.Roots["b!docs:01A"].Full, "root A finished before the cut")
	require.True(t, mid.Roots["b!docs:01B"].Full)
	require.Contains(t, mid.Roots["b!docs:01B"].Next, "page=1")
	second := &recorder{}
	next, err := c.FetchFullStream(context.Background(), both, last, second)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/B/z.pdf", "-b!docs:01W"}, second.names(),
		"the resumed attempt walks only root B's remaining page and tombstones what B no longer lists")
	require.Equal(t, 1, startsA()-before, "root A was enumerated once across both full attempts")
	require.False(t, next.CompleteListing)
	after, _ := decodeCursor(next)
	require.False(t, after.fullRun())
	require.Equal(t, 1, downloadsX()-beforeX, "x.docx was downloaded once across both full attempts")
}

func TestFetchFullStreamResyncInsideFromScratchAttemptIsStillComplete(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(f, "/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			file("01A", "a.docx", "01ROOT", "AAAA", "qa"),
			file("01B", "b.pdf", "01ROOT", "BB", "qb"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	// The full run's first request is answered with a 410; the fresh start no longer lists b.pdf.
	f.handle("GET /drives/b!docs/root/delta", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fresh") != "1" {
			w.Header().Set("Location", f.server.URL+"/drives/b!docs/root/delta?fresh=1")
			w.WriteHeader(http.StatusGone)
			return
		}
		_ = jsonEncode(w, map[string]any{
			"value":            []any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-9",
		})
	})
	rec := &recorder{}
	next, err := c.FetchFullStream(context.Background(), importsAll(), cur, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/a.docx", "-b!docs:01B"}, rec.names())
	require.True(t, next.CompleteListing, "the from-scratch attempt still listed every live file")
}

func TestFetchStreamMidPageCheckpointResumesByFingerprint(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	// Page 1 holds datasource.CheckpointEvery+2 files; the handler refuses the last one.
	var page []map[string]any
	var names []string
	for i := 0; i < datasource.CheckpointEvery+2; i++ {
		id := "01F" + string(rune('A'+i/26)) + string(rune('A'+i%26))
		name := "f" + id + ".pdf"
		page = append(page, file(id, name, "01ROOT", "XX", "q"+id))
		names = append(names, "Documents/"+name)
		serveFile(f, "b!docs", id, "XX")
	}
	deltaScript(f, "/drives/b!docs/root/delta", []map[string]any{rootItem("01ROOT")}, page)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	c := testConnector(f, KindSharePoint)
	first := &failingRecorder{failOn: page[len(page)-1]["name"].(string)}
	_, err := c.FetchStream(context.Background(), importsAll(), nil, first)
	require.ErrorContains(t, err, "handler refused")
	require.Equal(t, names[:datasource.CheckpointEvery+1], first.names())
	require.Len(t, first.checkpoints, 2, "one after page 0, one after datasource.CheckpointEvery emits inside page 1")
	mid, err := decodeCursor(first.checkpoints[1])
	require.NoError(t, err)
	require.Contains(t, mid.Roots["b!docs"].Next, "page=1")
	require.Len(t, mid.Items, datasource.CheckpointEvery)
	second := &recorder{}
	next, err := c.FetchStream(context.Background(), importsAll(), first.checkpoints[1], second)
	require.NoError(t, err)
	require.Equal(t, names[datasource.CheckpointEvery:], second.names(), "the files before the checkpoint are skipped")
	require.False(t, next.CompleteListing)
}

func TestFetchStreamRetryOfAnItemThatIsNoLongerAFileTombstones(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(f, "/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.handle("GET /drives/b!docs/items/01A/content",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	c := testConnector(f, KindSharePoint)
	first := &recorder{}
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, first)
	require.NoError(t, err)
	require.Equal(t, []string{"!msgraph_fetch_failed:a.docx"}, first.names())
	// By the retry the item was renamed to an extension the knowledge base cannot import.
	f.json("GET /drives/b!docs/items/01A", http.StatusOK, file("01A", "a.xyz", "01ROOT", "AAAA", "qa"))
	second := &recorder{}
	next, err := c.FetchStream(context.Background(), importsAll(), cur, second)
	require.NoError(t, err)
	require.Equal(t, []string{"-b!docs:01A"}, second.names())
	after, _ := decodeCursor(next)
	require.NotContains(t, after.Items, "b!docs:01A")
}

func TestFetchStreamDeltaPageWithNeitherLinkFailsTheRun(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	f.handle("GET /drives/b!docs/root/delta", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{"value": []any{}})
	})
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.ErrorIs(t, err, datasource.ErrFetchFailed)
	require.ErrorContains(t, err, "neither")
}

func TestFetchStreamForbiddenDownloadIsReportedOnce(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.json(
		"GET /drives/b!docs/items/01A/content",
		http.StatusForbidden,
		map[string]any{"error": map[string]any{"code": "accessDenied"}},
	)
	c := testConnector(f, KindSharePoint)
	rec := &recorder{}
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, rec)
	require.NoError(t, err, "one forbidden download does not stop the delta walk")
	require.Equal(t, []string{"!msgraph_permission_denied:a.docx"}, rec.names())
	recorded, _ := decodeCursor(cur)
	require.Equal(t, reasonPermissionDenied, recorded.Items["b!docs:01A"].Failed,
		"a download 403 is as permanent as a 403 on the item lookup")
	second := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), cur, second)
	require.NoError(t, err)
	require.Empty(t, second.names(), "the same refusal is not reported again")
	require.Equal(t, 1, f.seen("GET /drives/b!docs/items/01A/content"))
}

func TestFetchStreamForbiddenDownloadHostIsRetriedNextRun(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.handle("GET /drives/b!docs/items/01A/content", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, f.server.URL+"/download/01A", http.StatusFound)
	})
	f.handle("GET /download/01A", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
	rec := &recorder{}
	cur, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"!msgraph_permission_denied:a.docx"}, rec.names())
	recorded, _ := decodeCursor(cur)
	require.Empty(t, recorded.Items["b!docs:01A"].Failed, "the pre-authenticated URL expires, so its 403 is retried")
}
