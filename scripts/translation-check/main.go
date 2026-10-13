// Command translation-check proves a Go translation changed only comments and string literals.
//
//	go run ./scripts/translation-check [-rev HEAD] [-escape] <file.go>...
//
// Each file is compared with its version at -rev. Every token other than comments and string
// literals must be identical; a difference exits 1. Changed literals are listed, and a literal is
// flagged when a translation can break it: it sits in a _test.go file, it is compared, switched on,
// compiled as a regexp or passed to strings.Contains-style matchers, or its old text also occurs
// in another tracked file. -escape additionally requires each changed literal to keep its exact
// value (for \uXXXX escaping of Chinese that must stay).
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type tok struct {
	kind token.Token
	lit  string
	pos  token.Pos
}

func main() {
	rev := flag.String("rev", "HEAD", "revision to compare against")
	escape := flag.Bool("escape", false, "require changed literals to keep their value")
	flag.Parse()
	failed := false
	for _, path := range flag.Args() {
		if !check(path, *rev, *escape) {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func check(path, rev string, escape bool) bool {
	newSrc, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
		return false
	}
	oldSrc, err := exec.Command("git", "show", rev+":"+path).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: not in %s, nothing to compare\n", path, rev)
		return false
	}
	fset := token.NewFileSet()
	oldToks := scan(fset, path, oldSrc)
	newToks := scan(fset, path, newSrc)
	if len(oldToks) != len(newToks) {
		fmt.Printf("%s: FAIL token count %d -> %d (code changed, not only text)\n", path, len(oldToks), len(newToks))
		return false
	}
	ctx := literalContexts(path, newSrc)
	ok := true
	for i, o := range oldToks {
		n := newToks[i]
		if o.kind != n.kind || (o.kind != token.STRING && o.lit != n.lit) {
			fmt.Printf("%s: FAIL %s: %q -> %q\n", path, fset.Position(n.pos), o.lit, n.lit)
			ok = false
			continue
		}
		if o.kind != token.STRING || o.lit == n.lit {
			continue
		}
		if c := ctx[fset.Position(n.pos).Offset]; c == "import" || c == "tag" {
			fmt.Printf("%s: FAIL %s: %s changed: %s -> %s\n", path, fset.Position(n.pos), c, o.lit, n.lit)
			ok = false
			continue
		}
		ov, _ := strconv.Unquote(o.lit)
		nv, _ := strconv.Unquote(n.lit)
		var flags []string
		if escape && ov != nv {
			flags = append(flags, "VALUE-CHANGED")
			ok = false
		}
		if strings.HasSuffix(path, "_test.go") {
			flags = append(flags, "test")
		}
		if c := ctx[fset.Position(n.pos).Offset]; c != "" {
			flags = append(flags, c)
		}
		if run := longestNonASCII(ov); run != "" && usedElsewhere(run, path, rev) {
			flags = append(flags, "old-text-used-elsewhere:"+run)
		}
		fmt.Printf("%s: %s %s\n    - %s\n    + %s\n",
			path, fset.Position(n.pos), strings.Join(flags, ","), o.lit, n.lit)
	}
	if ok {
		fmt.Printf("%s: OK (only comments and string literals differ)\n", path)
	}
	return ok
}

func scan(fset *token.FileSet, path string, src []byte) []tok {
	var s scanner.Scanner
	s.Init(fset.AddFile(path, -1, len(src)), src, nil, scanner.ScanComments)
	var out, pending, preamble []tok
	for {
		pos, kind, lit := s.Scan()
		if kind == token.EOF {
			return out
		}
		if kind == token.COMMENT {
			if directive.MatchString(lit) {
				out = append(out, tok{kind, lit, pos})
			} else {
				pending = append(pending, tok{kind, lit, pos})
			}
			continue
		}
		// The comments right before `import "C"` are C source (the cgo preamble), so they must match too.
		if kind == token.STRING && lit == `"C"` && len(out) > 0 && out[len(out)-1].kind == token.IMPORT {
			imp := out[len(out)-1]
			out = append(append(out[:len(out)-1], preamble...), imp)
		}
		if kind == token.IMPORT {
			preamble = pending
		}
		pending = nil
		// Automatic semicolons carry "\n" or ";" depending on a trailing comment; compare the kind only.
		if kind == token.SEMICOLON {
			lit = ";"
		}
		out = append(out, tok{kind, lit, pos})
	}
}

// directive matches comments the toolchain or a generator reads: build constraints, go: directives,
// cgo exports, linter suppressions, and swag lines that shape the generated API spec.
var directive = regexp.MustCompile(`^//(go:|nolint|export |line |\s*\+build|\s*@(Router|ID|Security|Accept|Produce)\b)`)

// literalContexts marks string literals whose exact text code depends on.
func literalContexts(path string, src []byte) map[int]string {
	marks := map[int]string{}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return marks
	}
	mark := func(e ast.Expr, why string) {
		if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			marks[fset.Position(lit.Pos()).Offset] = why
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ImportSpec:
			mark(x.Path, "import")
		case *ast.Field:
			if x.Tag != nil {
				mark(x.Tag, "tag")
			}
		case *ast.BinaryExpr:
			if x.Op == token.EQL || x.Op == token.NEQ {
				mark(x.X, "compared")
				mark(x.Y, "compared")
			}
		case *ast.CaseClause:
			for _, e := range x.List {
				mark(e, "case")
			}
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && matcher[sel.Sel.Name] {
				for _, a := range x.Args {
					mark(a, "matcher:"+sel.Sel.Name)
				}
			}
		}
		return true
	})
	return marks
}

var matcher = map[string]bool{
	"Contains": true, "HasPrefix": true, "HasSuffix": true, "Index": true, "EqualFold": true,
	"MustCompile": true, "Compile": true, "MatchString": true, "ContainsAny": true, "TrimPrefix": true,
	"TrimSuffix": true, "Split": true, "Replace": true, "ReplaceAll": true, "Count": true,
}

func longestNonASCII(s string) string {
	best, cur := "", []rune{}
	for _, r := range s + " " {
		if r > unicode.MaxASCII && !unicode.IsSpace(r) && !unicode.IsPunct(r) {
			cur = append(cur, r)
			continue
		}
		if len(cur) > len([]rune(best)) {
			best = string(cur)
		}
		cur = cur[:0]
	}
	return best
}

func usedElsewhere(text, self, rev string) bool {
	// Search the old tree: in a batch that edits both ends, the working tree no longer has the old text.
	out, _ := exec.Command("git", "grep", "-l", "-F", "-e", text, rev, "--",
		".", ":!"+self, ":!website-docs", ":!CHANGELOG.md").Output()
	return len(bytes.TrimSpace(out)) > 0
}
