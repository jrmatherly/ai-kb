package tools

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

type fakeSpreadsheetConverter struct {
	gotContent  []byte
	gotFileType string
	xlsx        []byte
	err         error
}

func (f *fakeSpreadsheetConverter) ConvertSpreadsheet(
	_ context.Context, content []byte, fileType string,
) ([]byte, error) {
	f.gotContent, f.gotFileType = content, fileType
	return f.xlsx, f.err
}

func TestIsDataTableFileType(t *testing.T) {
	for _, ext := range []string{"csv", "xlsx", "xls", "xlsb", ".XLSX", ".xlsb"} {
		if !IsDataTableFileType(ext) {
			t.Errorf("IsDataTableFileType(%q) = false, want true", ext)
		}
	}
	for _, ext := range []string{"pdf", "png", "ods", "", "unknown"} {
		if IsDataTableFileType(ext) {
			t.Errorf("IsDataTableFileType(%q) = true, want false", ext)
		}
	}
}

func legacyWorkbookTool(conv *fakeSpreadsheetConverter, original []byte) *DataAnalysisTool {
	fs := &fakeFileService{readers: map[string]func() (io.ReadCloser, error){
		"kb/book": func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(original)), nil },
	}}
	tool := &DataAnalysisTool{BaseTool: dataAnalysisTool, fileService: fs, sessionID: "test-convert"}
	if conv != nil {
		tool.spreadsheetConverter = conv
	}
	return tool
}

func TestLoadFromKnowledgeConvertsLegacyWorkbooks(t *testing.T) {
	xlsxPath := filepath.Join(t.TempDir(), "converted.xlsx")
	writeWorkbook(t, xlsxPath, map[string][][]any{
		"Sales": {{"region", "amount"}, {"north", 10}, {"south", 32}},
	}, []string{"Sales"})
	xlsx, err := os.ReadFile(xlsxPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, fileType := range []string{"xls", "XLSB"} {
		t.Run(fileType, func(t *testing.T) {
			conv := &fakeSpreadsheetConverter{xlsx: xlsx}
			original := []byte("legacy workbook bytes")
			tool := legacyWorkbookTool(conv, original)
			tool.db = newTestDuckDB(t)
			ctx := context.Background()
			t.Cleanup(func() { tool.Cleanup(ctx) })

			knowledge := &types.Knowledge{ID: "k-1", FileType: fileType, FilePath: "kb/book"}
			schema, err := tool.LoadFromKnowledge(ctx, knowledge)
			if err != nil {
				t.Fatalf("LoadFromKnowledge: %v", err)
			}
			if !bytes.Equal(conv.gotContent, original) || conv.gotFileType != strings.ToLower(fileType) {
				t.Fatalf("converter got (%q, %q), want (%q, %q)",
					conv.gotContent, conv.gotFileType, original, strings.ToLower(fileType))
			}
			if schema.RowCount != 2 {
				t.Fatalf("RowCount = %d, want 2", schema.RowCount)
			}
			var total int
			query := `SELECT SUM(CAST(amount AS INTEGER)) FROM "k_k_1"`
			if err := tool.db.QueryRowContext(ctx, query).Scan(&total); err != nil {
				t.Fatal(err)
			}
			if total != 42 {
				t.Fatalf("SUM(amount) = %d, want 42", total)
			}
		})
	}
}

func TestLoadFromKnowledgeReportsConverterFailure(t *testing.T) {
	conv := &fakeSpreadsheetConverter{err: errors.New("rpc error: code = InvalidArgument desc = cannot read workbook")}
	tool := legacyWorkbookTool(conv, []byte("garbage"))

	knowledge := &types.Knowledge{ID: "k-2", FileType: "xlsb", FilePath: "kb/book"}
	_, err := tool.LoadFromKnowledge(context.Background(), knowledge)
	if err == nil || !strings.Contains(err.Error(), "convert .xlsb to .xlsx: rpc error: code = InvalidArgument") {
		t.Fatalf("err = %v, want the converter error", err)
	}
}

func TestLoadFromKnowledgeWithoutConverterFails(t *testing.T) {
	tool := legacyWorkbookTool(nil, []byte("legacy"))

	knowledge := &types.Knowledge{ID: "k-3", FileType: "xls", FilePath: "kb/book"}
	_, err := tool.LoadFromKnowledge(context.Background(), knowledge)
	if err == nil || !strings.Contains(err.Error(), "loading .xls needs docreader on the gRPC transport") {
		t.Fatalf("err = %v, want the missing-converter error", err)
	}
}
