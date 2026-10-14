package tools

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
)

// excelDataSheets returns, in workbook order, the worksheets that hold a value
// or formula. It reads the workbook in Go, so .xls, .xlsb and corrupt files
// fail here before DuckDB sees them: spatial's st_read_meta segfaulted on them
// and read_xlsx spins on a truncated zip without honoring cancellation. Cell
// content, not DuckDB's inferred row count, decides emptiness, so header-only
// sheets and uncached formulas keep their columns. Chart and dialog sheets are
// skipped because read_xlsx cannot read them.
func excelDataSheets(ctx context.Context, filename string) ([]string, error) {
	book, err := zip.OpenReader(filename)
	if errors.Is(err, zip.ErrFormat) {
		return nil, fmt.Errorf("not an .xlsx workbook: %w", err)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = book.Close() }()
	var workbook struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			ID   string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	var relationships struct {
		Items []struct {
			ID     string `xml:"Id,attr"`
			Type   string `xml:"Type,attr"`
			Target string `xml:"Target,attr"`
			Mode   string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	readXML := func(name string, out any) error {
		r, openErr := book.Open(name)
		if openErr != nil {
			return fmt.Errorf("not an .xlsx workbook: %w", openErr)
		}
		decodeErr := xml.NewDecoder(r).Decode(out)
		closeErr := r.Close()
		if decodeErr != nil {
			return fmt.Errorf("read %s: %w", name, decodeErr)
		}
		return closeErr
	}
	if err := readXML("xl/workbook.xml", &workbook); err != nil {
		return nil, err
	}
	if err := readXML("xl/_rels/workbook.xml.rels", &relationships); err != nil {
		return nil, err
	}
	targets := make(map[string]string, len(relationships.Items))
	for _, rel := range relationships.Items {
		if !strings.HasSuffix(rel.Type, "/worksheet") || rel.Mode == "External" || rel.Target == "" {
			continue
		}
		target, err := url.Parse(rel.Target)
		if err != nil {
			return nil, fmt.Errorf("invalid worksheet relationship: %w", err)
		}
		if target.IsAbs() || target.Host != "" {
			continue
		}
		if strings.HasPrefix(target.Path, "/") {
			targets[rel.ID] = strings.TrimPrefix(target.Path, "/")
		} else {
			targets[rel.ID] = path.Join("xl", target.Path)
		}
	}
	kept := make([]string, 0, len(workbook.Sheets))
	for _, sheet := range workbook.Sheets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		target, ok := targets[sheet.ID]
		if !ok || strings.TrimSpace(sheet.Name) == "" {
			continue
		}
		r, err := book.Open(target)
		if err != nil {
			return nil, fmt.Errorf("open worksheet %q: %w", sheet.Name, err)
		}
		hasContent, scanErr := excelSheetHasContent(ctx, r)
		closeErr := r.Close()
		if scanErr != nil {
			return nil, fmt.Errorf("read worksheet %q: %w", sheet.Name, scanErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if hasContent {
			kept = append(kept, sheet.Name)
		}
	}
	return kept, nil
}

// Stop at the first value or formula; only a fully read, well-formed worksheet
// can be classified as empty. Shared-string indices count as content without
// loading the shared-string table. Formatting and dimensions alone do not.
func excelSheetHasContent(ctx context.Context, r io.Reader) (bool, error) {
	decoder := xml.NewDecoder(r)
	var stack []string
	cellDepth := 0
	seenRoot := false
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		token, err := decoder.Token()
		if err == io.EOF {
			if !seenRoot {
				return false, fmt.Errorf("missing worksheet element")
			}
			return false, nil
		}
		if err != nil {
			return false, err
		}
		switch v := token.(type) {
		case xml.StartElement:
			if len(stack) == 0 {
				if seenRoot || v.Name.Local != "worksheet" {
					return false, fmt.Errorf("unexpected worksheet root %q", v.Name.Local)
				}
				seenRoot = true
			}
			stack = append(stack, v.Name.Local)
			if len(stack) == 4 && stack[1] == "sheetData" && stack[2] == "row" && stack[3] == "c" {
				cellDepth = len(stack)
			}
			if cellDepth != 0 && v.Name.Local == "f" {
				return true, nil
			}
		case xml.CharData:
			if len(stack) == 0 && strings.TrimSpace(string(v)) != "" {
				return false, fmt.Errorf("text outside worksheet element")
			}
			if cellDepth != 0 && len(v) != 0 && (stack[len(stack)-1] == "v" || stack[len(stack)-1] == "t") {
				return true, nil
			}
		case xml.EndElement:
			if len(stack) == cellDepth {
				cellDepth = 0
			}
			stack = stack[:len(stack)-1]
		}
	}
}
