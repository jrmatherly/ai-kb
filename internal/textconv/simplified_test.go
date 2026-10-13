package textconv

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func TestToSimplified(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"", ""},
		{"\u8acb\u806f\u7e6b\u5ba2\u670d\uff0c\u958b\u767c\u74b0\u5883", "\u8bf7\u8054\u7cfb\u5ba2\u670d\uff0c\u5f00\u53d1\u73af\u5883"},
		{"\u4e7e\u9686\u5e74\u9593\u4e7e\u4e7e\u6de8\u6de8\u7684\u982d\u9aee", "\u4e7e\u9686\u5e74\u95f4\u5e72\u5e72\u51c0\u51c0\u7684\u5934\u53d1"},
		{"\u4e00\u76ee\u77ad\u7136\uff0c\u4e0d\u77ad\u89e3\uff1f", "\u4e00\u76ee\u4e86\u7136\uff0c\u4e0d\u4e86\u89e3\uff1f"},
		{"😀\u81fa\u7063ABC𠮷\x00\u5f8c", "😀\u53f0\u6e7eABC𠮷\x00\u540e"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := ToSimplified(tc.input); got != tc.want {
				t.Errorf("ToSimplified(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// Captured from longbridgeapp/opencc v0.3.13's t2s conversion before removing
// that dependency. Every dictionary key is exercised alone, in context and
// repeated. Changing this digest requires reviewing FAQ hash compatibility.
func TestHistoricalConversionCorpus(t *testing.T) {
	h := sha256.New()
	count := 0
	check := func(input string) {
		if _, err := fmt.Fprintf(h, "%s\x00%s\x00", input, ToSimplified(input)); err != nil {
			t.Fatal(err)
		}
		count++
	}
	for _, data := range []string{phrasesText, charactersText} {
		for _, line := range strings.Split(strings.TrimSuffix(data, "\n"), "\n") {
			key, _, _ := strings.Cut(line, "\t")
			for _, input := range []string{key, "\u524d" + key + "\u5f8c", key + key} {
				check(input)
			}
		}
	}
	for _, input := range []string{"\u4e7e\u9686\u5e74\u9593\u4e7e\u4e7e\u6de8\u6de8\u7684\u982d\u9aee", "\u4e00\u76ee\u77ad\u7136\uff0c\u4e0d\u77ad\u89e3\uff1f", "\u8acb\u806f\u7e6b\u5ba2\u670d\uff0c\u958b\u767c\u74b0\u5883", "😀\u81fa\u7063ABC𠮷\x00\u5f8c", "\u4e7e\u5764\u5927\u632a\u79fb", "\u8457\u4f5c\u6b0a\u8207\u8edf\u9ad4\u958b\u767c", ""} {
		check(input)
	}
	const want = "f3afa240ee7460932bbef67207d23302c5d9722b70c1f706ce071d2f35bfbcc0"
	if got := fmt.Sprintf("%x", h.Sum(nil)); got != want || count != 13177 {
		t.Fatalf("historical conversion changed: count=%d digest=%s, want 13177/%s", count, got, want)
	}
}

func TestDictionaryPrecedenceAndAlternatives(t *testing.T) {
	c := newConverter("\u7532\u4e59\t\u9996\u9078 \u6b21\u9078\n\u7532\u4e59\u4e19\t\u6700\u9577\n", "\u7532\u4e59\u4e19\u4e01\t\u4e0d\u61c9\u512a\u5148\n\u4e01\t\u5c3e\n")
	if got := c.convert("\u7532\u4e59\u4e19\u4e01\u7532\u4e59"); got != "\u6700\u9577\u5c3e\u9996\u9078" {
		t.Fatalf("phrase precedence/longest match/first alternative changed: %q", got)
	}
}
