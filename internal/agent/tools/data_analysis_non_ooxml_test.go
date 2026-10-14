package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// DuckDB 1.5.5's st_read_meta crashed the process on these inputs, and
// read_xlsx spins forever on a truncated zip, so each must fail in Go first.
func TestLoadFromExcelRejectsNonOOXMLWithoutCrashing(t *testing.T) {
	dir := t.TempDir()
	truncated := filepath.Join(dir, "truncated.xlsx")
	if err := os.WriteFile(truncated, []byte("PK\x03\x04junkjunk"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"legacy xls":     "../../infrastructure/docparser/anydoc/testdata/upstream/xls/sheet.xls",
		"binary xlsb":    "../../../docreader/tests/fixtures/pandas-test1.xlsb",
		"truncated xlsx": truncated,
	}
	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			tool := &DataAnalysisTool{db: newPlainDuckDB(t), loadedTables: map[string]bool{}, sessionID: "test"}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			schema, err := tool.LoadFromExcel(ctx, file, "t_"+filepath.Base(file)[:4])
			if err == nil || schema != nil {
				t.Fatalf("LoadFromExcel(%s) = %v, %v; want an error", file, schema, err)
			}
		})
	}
}
