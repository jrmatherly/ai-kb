package seafile

import (
	"strings"
	"testing"
)

func TestKnowledgeFileName(t *testing.T) {
	if got := knowledgeFileName("01-\u673a\u573a\u7ebf", "/\u4ea7\u54c1/\u4ea7\u54c1A/\u9700\u6c42: v2.docx"); got != "01-\u673a\u573a\u7ebf/\u4ea7\u54c1/\u4ea7\u54c1A/\u9700\u6c42_ v2.docx" {
		t.Fatalf("filename = %q", got)
	}
	p := "/" + strings.Repeat("\u5c42\u7ea7/", 20) + "\u8bf4\u660e.pdf"
	if got := knowledgeFileName("\u8d44\u6599\u5e93", p); got != "\u8d44\u6599\u5e93"+p {
		t.Fatalf("deep filename was truncated: %q", got)
	}
}

func TestExternalIDAndFingerprint(t *testing.T) {
	if got := externalID(testRepoID, "/\u4ea7\u54c1/a:b.pdf"); got != "seafile:"+testRepoID+":/\u4ea7\u54c1/a:b.pdf" {
		t.Fatalf("externalID = %q", got)
	}
	for _, tc := range []struct {
		oid         string
		mtime, size int64
		want        string
	}{
		{"abc123", 123, 456, "o:abc123|m:123|s:456"},
		{"", 0, 0, "o:|m:0|s:0"},
	} {
		if got := fingerprint(tc.oid, tc.mtime, tc.size); got != tc.want {
			t.Fatalf("fingerprint = %q, want %q", got, tc.want)
		}
	}
}

func TestValidEntryName(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"\u8bf4\u660e.pdf", true},
		{"\u9700\u6c42: v2.docx", true},
		{"a%2Fb.pdf", true},
		{".hidden.pdf", true},
		{" spaced .pdf", true},
		{"", false},
		{".", false},
		{"..", false},
		{"../outside.pdf", false},
		{"/absolute.pdf", false},
		{"a/b.pdf", false},
		{"a\\b.pdf", false},
		{"a\x00b.pdf", false},
		{"a\nb.pdf", false},
		{"a\rb.pdf", false},
		{"a\tb.pdf", false},
		{"a\x7fb.pdf", false},
	} {
		if got := validEntryName(tc.name); got != tc.want {
			t.Errorf("validEntryName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
