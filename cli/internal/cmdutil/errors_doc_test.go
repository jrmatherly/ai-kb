package cmdutil

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// loadErrorReferenceSection returns the body of the cli/AGENTS.md "Error code
// reference" section, delimited by the ERROR_REFERENCE_START/END markers.
func loadErrorReferenceSection(t *testing.T) string {
	t.Helper()
	// From cli/internal/cmdutil/, go up two levels to find cli/AGENTS.md.
	docPath, err := filepath.Abs("../../AGENTS.md")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}
	doc := string(content)

	const startMarker = "<!-- ERROR_REFERENCE_START -->"
	const endMarker = "<!-- ERROR_REFERENCE_END -->"
	startIdx := strings.Index(doc, startMarker)
	endIdx := strings.Index(doc, endMarker)
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		t.Fatalf("error-reference markers missing or malformed in %s:\n  start=%d end=%d", docPath, startIdx, endIdx)
	}
	return doc[startIdx:endIdx]
}

// documentedRows maps each code in the reference table to its Exit and Default
// hint cells. Only the first column names codes: hint cells also carry
// backticked flags, paths and commands.
func documentedRows(refSection string) map[string][2]string {
	rows := map[string][2]string{}
	for _, line := range strings.Split(refSection, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 4 {
			continue
		}
		// Skips the header ("Code") and separator ("---") rows, which carry no
		// backticks in the first column.
		first := strings.TrimSpace(cells[0])
		if len(first) < 3 || !strings.HasPrefix(first, "`") || !strings.HasSuffix(first, "`") {
			continue
		}
		rows[strings.Trim(first, "`")] = [2]string{strings.TrimSpace(cells[1]), strings.TrimSpace(cells[3])}
	}
	return rows
}

// TestAllCodes_DocumentedInAGENTS verifies every typed code returned by
// AllCodes() surfaces in cli/AGENTS.md "Error code reference" section
// (delimited by ERROR_REFERENCE_START/END markers).
//
// Prevents drift: a contributor adding a new ErrorCode without updating
// the doc fails this test, forcing the doc to stay current.
func TestAllCodes_DocumentedInAGENTS(t *testing.T) {
	refSection := loadErrorReferenceSection(t)

	missing := []string{}
	for _, c := range AllCodes() {
		needle := "`" + string(c) + "`"
		if !strings.Contains(refSection, needle) {
			missing = append(missing, string(c))
		}
	}
	if len(missing) > 0 {
		t.Errorf("the following error codes are registered in AllCodes() but not listed in cli/AGENTS.md \"Error code reference\" section between the ERROR_REFERENCE markers:\n  - %s\n\nAdd a row for each missing code to keep agent-facing docs in sync.",
			strings.Join(missing, "\n  - "))
	}
}

// TestDocumentedCodes_RegisteredInAllCodes is the reverse of
// TestAllCodes_DocumentedInAGENTS: every code the doc advertises must still be
// a code the CLI can emit.
//
// Without this direction a removed code lingers in the doc forever, and agents
// branch on codes that will never arrive - the failure mode that left three
// `mcp.*` codes documented for two minor versions after they were deleted.
func TestDocumentedCodes_RegisteredInAllCodes(t *testing.T) {
	refSection := loadErrorReferenceSection(t)

	registered := make(map[string]bool, len(AllCodes()))
	for _, c := range AllCodes() {
		registered[string(c)] = true
	}

	documented := documentedRows(refSection)
	// Guards against the parser silently matching nothing, which would make this
	// check pass vacuously if the table format ever changes.
	if len(documented) == 0 {
		t.Fatal("parsed zero error codes out of the reference table; the table format or the parser changed")
	}

	stale := []string{}
	for c := range documented {
		if !registered[c] {
			stale = append(stale, c)
		}
	}
	if len(stale) > 0 {
		t.Errorf("the following error codes are documented in cli/AGENTS.md "+
			"\"Error code reference\" but are not registered in AllCodes():\n  - %s\n\n"+
			"Drop the row if the code was removed, or register the code if the row is correct.",
			strings.Join(stale, "\n  - "))
	}
}

// TestDocumentedRows_MatchExitCodeAndHint checks the Exit and Default hint
// columns against ExitCode and defaultHint: the exit cell starts with the exit
// a typed error of that code produces, and the hint cell is defaultHint with
// Markdown backticks allowed, or a "(no canonical hint...)" note when the code
// has none. Listing a code was not enough: "verify tenant context" drifted
// from the real hint and passed.
func TestDocumentedRows_MatchExitCodeAndHint(t *testing.T) {
	rows := documentedRows(loadErrorReferenceSection(t))
	if len(rows) == 0 {
		t.Fatal("parsed zero rows out of the reference table; the table format or the parser changed")
	}
	for _, c := range AllCodes() {
		row, ok := rows[string(c)]
		if !ok {
			continue // TestAllCodes_DocumentedInAGENTS reports the missing row
		}
		exit, hint := row[0], row[1]
		wantExit := strconv.Itoa(ExitCode(NewError(c, "x")))
		if exit != wantExit && !strings.HasPrefix(exit, wantExit+" ") {
			t.Errorf("%s: Exit cell %q does not start with %s, the exit ExitCode returns for the typed code",
				c, exit, wantExit)
		}
		wantHint := defaultHint(c)
		if wantHint == "" {
			if !strings.HasPrefix(hint, "(no canonical hint") {
				t.Errorf("%s: defaultHint is empty, but the hint cell is %q, not a \"(no canonical hint...)\" note",
					c, hint)
			}
			continue
		}
		if got := strings.ReplaceAll(hint, "`", ""); got != strings.ReplaceAll(wantHint, "`", "") {
			t.Errorf("%s: Default hint cell\n  doc:  %s\n  code: %s", c, hint, wantHint)
		}
	}
}
