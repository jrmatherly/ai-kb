// Command translation-check proves a Go translation changed only comments and string literals.
//
//	go run ./scripts/translation-check [-rev HEAD] [-escape] <file.go>...
//
// Each file is compared with its version at -rev. Every token other than comments and string or
// rune literals must be identical; a difference exits 1. What wrapping for lll and gofumpt forces
// is tolerated: both sides fold "a" + "b" into one literal when every operand is a string
// literal, drop a comma right before ) } or ], and compare rune literals like strings, by value.
// Changed literals are listed, and a literal is flagged when a translation can break it: it sits
// in a _test.go file, it is compared, switched on, compiled as a regexp or passed to
// strings.Contains-style matchers, or its old text also occurs in another tracked file. -escape
// additionally requires each changed literal to keep its exact value (for \uXXXX escaping of
// Chinese that must stay); a folded literal is checked by its concatenated value.
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
	lit  string // source text; a folded literal joins its pieces with " + "
	val  string // decoded value of a STRING or CHAR
	pos  token.Pos
}

func isLit(k token.Token) bool { return k == token.STRING || k == token.CHAR }

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
	var out bytes.Buffer
	ok := compare(&out, path, rev, oldSrc, newSrc, escape)
	_, _ = out.WriteTo(os.Stdout)
	return ok
}

func compare(w *bytes.Buffer, path, rev string, oldSrc, newSrc []byte, escape bool) bool {
	fset := token.NewFileSet()
	oldToks := scan(fset, path, oldSrc)
	newToks := scan(fset, path, newSrc)
	if len(oldToks) != len(newToks) {
		fmt.Fprintf(w, "%s: FAIL token count %d -> %d (code changed, not only text)\n",
			path, len(oldToks), len(newToks))
		return false
	}
	ctx := literalContexts(path, newSrc)
	ok := true
	for i, o := range oldToks {
		n := newToks[i]
		if o.kind != n.kind || (!isLit(o.kind) && o.lit != n.lit) {
			fmt.Fprintf(w, "%s: FAIL %s: %q -> %q\n", path, fset.Position(n.pos), o.lit, n.lit)
			ok = false
			continue
		}
		if !isLit(o.kind) || o.lit == n.lit {
			continue
		}
		if c := ctx[fset.Position(n.pos).Offset]; c == "import" || c == "tag" {
			fmt.Fprintf(w, "%s: FAIL %s: %s changed: %s -> %s\n", path, fset.Position(n.pos), c, o.lit, n.lit)
			ok = false
			continue
		}
		ov, nv := o.val, n.val
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
		fmt.Fprintf(w, "%s: %s %s\n    - %s\n    + %s\n",
			path, fset.Position(n.pos), strings.Join(flags, ","), o.lit, n.lit)
	}
	if ok {
		fmt.Fprintf(w, "%s: OK (only comments and string literals differ)\n", path)
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
			return normalize(out)
		}
		if kind == token.COMMENT {
			if directive.MatchString(lit) {
				out = append(out, tok{kind, lit, "", pos})
			} else {
				pending = append(pending, tok{kind, lit, "", pos})
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
		var val string
		if isLit(kind) {
			val, _ = strconv.Unquote(lit)
		}
		out = append(out, tok{kind, lit, val, pos})
	}
}

// normalize undoes what wrapping a long line forces: a trailing comma before a closer, and a
// literal split into "a" + "b". A run folds only when no non-literal operand touches it, since
// x + "a" + "b" and "a" + "b"[i] are not a single literal.
func normalize(ts []tok) []tok {
	var out []tok
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		if t.kind == token.COMMA && i+1 < len(ts) && closes(ts[i+1].kind) {
			continue
		}
		if t.kind == token.STRING && (len(out) == 0 || out[len(out)-1].kind != token.ADD) {
			j := i
			for j+2 < len(ts) && ts[j+1].kind == token.ADD && ts[j+2].kind == token.STRING {
				j += 2
			}
			if j > i && (j+1 == len(ts) || (ts[j+1].kind != token.ADD && ts[j+1].kind != token.LBRACK)) {
				for k := i + 2; k <= j; k += 2 {
					t.lit += " + " + ts[k].lit
					t.val += ts[k].val
				}
				i = j
			}
		}
		out = append(out, t)
	}
	return out
}

func closes(k token.Token) bool { return k == token.RPAREN || k == token.RBRACE || k == token.RBRACK }

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
	// A folded literal carries the position of its first piece, so marking every piece covers it.
	var mark func(e ast.Expr, why string)
	mark = func(e ast.Expr, why string) {
		switch x := e.(type) {
		case *ast.BasicLit:
			if isLit(x.Kind) {
				marks[fset.Position(x.Pos()).Offset] = why
			}
		case *ast.BinaryExpr:
			if x.Op == token.ADD {
				mark(x.X, why)
				mark(x.Y, why)
			}
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
