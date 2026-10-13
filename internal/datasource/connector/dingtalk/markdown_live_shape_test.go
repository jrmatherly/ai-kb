package dingtalk

import (
	"encoding/json"
	"testing"
)

// Fixtures in this file are **live payloads**, copied from
// `GET /v1.0/doc/suites/documents/{id}/blocks` against a production tenant and
// only shortened where marked. They exist because the list renderer was once
// written against a shape the API never returns — `children` — while the real
// response carries one item per block with the text in the block's own field.
// Nine production documents, every list block: no `children`, no `list.level`,
// no `listId`. A fixture invented from the docs cannot catch that; these can.

// The live shape of an unordered list item: the whole item is `text`, nothing else.
func TestRenderListReadsTheLiveFlatShape(t *testing.T) {
	result := renderDocument("\u6d4b\u8bd5\u6587\u6863", []json.RawMessage{
		rawJSON(`{"blockType":"unorderedList","index":87,"id":"m050v0j3o1iic0pr7xg",` +
			"\"unorderedList\":{\"text\":\"\u786e\u8ba4\u662f\u5426\u81ea\u5b9a\u4e49\u5168\u7aef\u53e3\uff0c\u9ed8\u8ba4\u662f\u5e38\u89c1\u7aef\u53e3\"}}"),
		rawJSON(`{"blockType":"orderedList","index":88,"id":"m050w04e41a0sfbcy1",` +
			"\"orderedList\":{\"text\":\"\u7b2c\u4e00\u6b65\uff1a\u786e\u8ba4\u8d44\u4ea7\u8303\u56f4\"}}"),
	})

	want := "# \u6d4b\u8bd5\u6587\u6863\n\n- \u786e\u8ba4\u662f\u5426\u81ea\u5b9a\u4e49\u5168\u7aef\u53e3\uff0c\u9ed8\u8ba4\u662f\u5e38\u89c1\u7aef\u53e3\n1. \u7b2c\u4e00\u6b65\uff1a\u786e\u8ba4\u8d44\u4ea7\u8303\u56f4\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
	if len(result.UnknownTypes) != 0 {
		t.Fatalf("UnknownTypes = %#v, want none", result.UnknownTypes)
	}
}

// Consecutive live list blocks are consecutive items of one list, and every one
// of them must survive. This is the regression that shipped: 190 of 190 list
// items were dropped, silently, because the type was "known" and so never
// showed up in the unknown-type report either.
func TestRenderListKeepsEveryItemOfALiveDocument(t *testing.T) {
	result := renderDocument("\u6d4b\u8bd5\u6587\u6863", []json.RawMessage{
		rawJSON("{\"blockType\":\"unorderedList\",\"index\":1,\"id\":\"a\",\"unorderedList\":{\"text\":\"\u9879\u76ee\u5206\u4eab\uff08\u56e2\u961f\u7248\u4e2d\uff09\"}}"),
		rawJSON("{\"blockType\":\"unorderedList\",\"index\":2,\"id\":\"b\",\"unorderedList\":{\"text\":\"\u9879\u76ee\u62a5\u544a\u5bfc\u51fa\"}}"),
		rawJSON("{\"blockType\":\"unorderedList\",\"index\":3,\"id\":\"c\",\"unorderedList\":{\"text\":\"\u4efb\u52a1\u4e0b\u53d1\"}}"),
	})

	want := "# \u6d4b\u8bd5\u6587\u6863\n\n- \u9879\u76ee\u5206\u4eab\uff08\u56e2\u961f\u7248\u4e2d\uff09\n- \u9879\u76ee\u62a5\u544a\u5bfc\u51fa\n- \u4efb\u52a1\u4e0b\u53d1\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
}

// Multi-line items keep their continuation lines aligned under the first line,
// so the value cannot escape its bullet.
func TestRenderListKeepsLiveMultiLineItem(t *testing.T) {
	result := renderDocument("\u6d4b\u8bd5\u6587\u6863", []json.RawMessage{
		rawJSON(`{"blockType":"orderedList","index":4,"id":"d",` +
			"\"orderedList\":{\"text\":\"\u8fde\u63a5\u5230\u5185\u7f51\u63a2\u6d4b\u6a21\u5757\uff1a\\n\u6267\u884c\u6240\u4e0b\u8f7d\u7684\u63a2\u6d4bAgent\"}}"),
	})

	want := "# \u6d4b\u8bd5\u6587\u6863\n\n1. \u8fde\u63a5\u5230\u5185\u7f51\u63a2\u6d4b\u6a21\u5757\uff1a\n   \u6267\u884c\u6240\u4e0b\u8f7d\u7684\u63a2\u6d4bAgent\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
}

// The live `sdt` block is a table of contents and carries the whole thing in
// `text`. The type is still unmodelled, so the marker must stay — but dropping
// the text because the presentation is unknown loses the document's contents
// page. Verified live: 1,212 characters in one block.
func TestRenderUnknownBlockRendersItsLiveText(t *testing.T) {
	result := renderDocument("\u6d4b\u8bd5\u6587\u6863", []json.RawMessage{
		rawJSON(`{"blockType":"unknown","index":19,"id":"lqaap37i2lv2yn7i5w7",` +
			"\"unknown\":{\"rawType\":\"sdt\",\"text\":\"\u76ee\u5f55\u76ee\u5f551\u524d\u8a0011. \u4ea7\u54c1\u4ecb\u7ecd1\"}}"),
	})

	want := "# \u6d4b\u8bd5\u6587\u6863\n\n\u76ee\u5f55\u76ee\u5f551\u524d\u8a0011. \u4ea7\u54c1\u4ecb\u7ecd1\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
	if len(result.UnknownTypes) != 1 || result.UnknownTypes[0] != "unknown:sdt" {
		t.Fatalf("UnknownTypes = %#v, want [unknown:sdt]", result.UnknownTypes)
	}
}

// Not every unmodelled block has text: the live `card` and `sectionBr` blocks
// carry only `rawType`, so they stay a marker and render nothing.
func TestRenderUnknownBlockWithoutTextStaysAMarker(t *testing.T) {
	result := renderDocument("\u6d4b\u8bd5\u6587\u6863", []json.RawMessage{
		rawJSON(`{"blockType":"unknown","index":21,"id":"mulaiyzgurq2s6o7b2f",` +
			`"unknown":{"rawType":"sectionBr"}}`),
	})

	want := "# \u6d4b\u8bd5\u6587\u6863\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
	if len(result.UnknownTypes) != 1 || result.UnknownTypes[0] != "unknown:sectionBr" {
		t.Fatalf("UnknownTypes = %#v, want [unknown:sectionBr]", result.UnknownTypes)
	}
}

// A block carrying both its own text and inline children is a shape the API does
// not produce. Emitting both would duplicate the item, so the shape is reported
// instead of guessed at.
func TestRenderListFlagsOwnTextAndInlineChildrenTogether(t *testing.T) {
	result := renderDocument("\u6d4b\u8bd5\u6587\u6863", []json.RawMessage{
		rawJSON("{\"blockType\":\"unorderedList\",\"unorderedList\":{\"text\":\"\u7532\"}," +
			"\"children\":[{\"elementType\":\"text\",\"text\":\"\u7532\"}]}"),
	})

	want := "# \u6d4b\u8bd5\u6587\u6863\n\n- \u7532\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
	if len(result.UnknownTypes) != 1 || result.UnknownTypes[0] != "list_text_and_inline_children" {
		t.Fatalf("UnknownTypes = %#v, want [list_text_and_inline_children]", result.UnknownTypes)
	}
}

// Escaping still applies to the block's own text, so an item that starts with a
// marker character cannot turn into a nested list or a heading.
func TestRenderListEscapesLiveOwnText(t *testing.T) {
	result := renderDocument("\u6d4b\u8bd5\u6587\u6863", []json.RawMessage{
		rawJSON("{\"blockType\":\"unorderedList\",\"unorderedList\":{\"text\":\"- \u4e0d\u662f\u5b50\u5217\u8868\"}}"),
	})

	want := "# \u6d4b\u8bd5\u6587\u6863\n\n- \\- \u4e0d\u662f\u5b50\u5217\u8868\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
}
