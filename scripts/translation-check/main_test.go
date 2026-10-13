package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		name     string
		old, new string
		escape   bool
		ok       bool
		want     string
	}{
		{
			name: "split string",
			old:  `var s = "hello world"`,
			new:  "var s = \"hello \" +\n\t\"world\"",
			ok:   true, want: `- "hello world"` + "\n" + `    + "hello " + "world"`,
		},
		{
			name: "trailing comma",
			old:  `var _ = f(1, []int{2})`,
			new:  "var _ = f(\n\t1,\n\t[]int{\n\t\t2,\n\t},\n)",
			ok:   true, want: "a.go: OK",
		},
		{
			name: "escaped rune", escape: true,
			old: "var r = '\u62a5'",
			new: `var r = '\u62a5'`,
			ok:  true, want: "a.go: OK",
		},
		{
			name: "renamed identifier",
			old:  `var s = "x"`,
			new:  `var t = "x"`,
			ok:   false, want: `FAIL a.go:2:5: "s" -> "t"`,
		},
		{
			name: "added nolint",
			old:  `var s = "x"`,
			new:  `var s = "x" //nolint:lll`,
			ok:   false, want: "FAIL token count",
		},
		{
			name: "non-literal operand is not folded",
			old:  `var s = x + "ab"`,
			new:  `var s = x + "a" + "b"`,
			ok:   false, want: "FAIL token count",
		},
		{
			name: "changed value under escape", escape: true,
			old: `var s = "hello"`,
			new: `var s = "world"`,
			ok:  false, want: "VALUE-CHANGED",
		},
		{
			name: "changed folded value under escape", escape: true,
			old: `var s = "hello world"`,
			new: `var s = "hello " + "there"`,
			ok:  false, want: "VALUE-CHANGED",
		},
		{
			name: "flag on folded literal",
			old:  `var b = s == "ab"`,
			new:  `var b = s == "a" + "b"`,
			ok:   true, want: "a.go:2:14 compared",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			oldSrc, newSrc := []byte("package p\n"+c.old+"\n"), []byte("package p\n"+c.new+"\n")
			ok := compare(&out, "a.go", "HEAD", oldSrc, newSrc, c.escape)
			if ok != c.ok || !strings.Contains(out.String(), c.want) {
				t.Errorf("ok = %v, want %v; output %q lacks %q", ok, c.ok, out.String(), c.want)
			}
		})
	}
}
