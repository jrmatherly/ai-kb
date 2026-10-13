package dingtalk

import (
	"encoding/json"
	"testing"
)

// A list block can carry inline children (the item text) and block children
// (one per item) at the same time. Both kinds must survive rendering: an inline
// item used to shadow every block item that followed it.
func TestRenderListKeepsInlineAndBlockItems(t *testing.T) {
	result := renderDocument("\u6d4b\u8bd5\u6587\u6863", []json.RawMessage{
		rawJSON(`{"blockType":"unorderedList","unorderedList":{"list":{"level":0}},"children":[` +
			"{\"elementType\":\"text\",\"text\":\"\u7b2c\u4e00\u9879\"}," +
			"{\"blockType\":\"paragraph\",\"paragraph\":{\"text\":\"\u7b2c\u4e8c\u9879\u6574\u6bb5\u5185\u5bb9\"}}]}"),
	})

	want := "# \u6d4b\u8bd5\u6587\u6863\n\n- \u7b2c\u4e00\u9879\n- \u7b2c\u4e8c\u9879\u6574\u6bb5\u5185\u5bb9\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
	if len(result.UnknownTypes) != 0 {
		t.Fatalf("UnknownTypes = %#v, want none", result.UnknownTypes)
	}
}

// Items delivered as block children must keep one bullet line each instead of
// being flattened onto a single line.
func TestRenderListEmitsOneBulletPerBlockItem(t *testing.T) {
	result := renderDocument("\u6d4b\u8bd5\u6587\u6863", []json.RawMessage{
		rawJSON(`{"blockType":"unorderedList","unorderedList":{"list":{"level":0}},"children":[` +
			"{\"blockType\":\"paragraph\",\"paragraph\":{\"text\":\"\u7532\u9879\"}}," +
			"{\"blockType\":\"paragraph\",\"paragraph\":{\"text\":\"\u4e59\u9879\"}}," +
			"{\"blockType\":\"paragraph\",\"paragraph\":{\"text\":\"\u4e19\u9879\"}}]}"),
	})

	want := "# \u6d4b\u8bd5\u6587\u6863\n\n- \u7532\u9879\n- \u4e59\u9879\n- \u4e19\u9879\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
}

// Inline and block children keep their payload order, so an inline run that
// follows a block item is not hoisted above it or merged into another item.
func TestRenderListKeepsItemOrder(t *testing.T) {
	result := renderDocument("", []json.RawMessage{
		rawJSON(`{"blockType":"orderedList","orderedList":{"list":{"level":0}},"children":[` +
			"{\"blockType\":\"paragraph\",\"paragraph\":{\"text\":\"\u5757\u9879\"}}," +
			"{\"elementType\":\"text\",\"text\":\"\u5185\u8054\u9879\u4e00\"}," +
			"{\"elementType\":\"text\",\"text\":\"\u5185\u8054\u9879\u4e8c\"}]}"),
	})

	want := "1. \u5757\u9879\n1. \u5185\u8054\u9879\u4e00\u5185\u8054\u9879\u4e8c\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
}

// Nested lists are indented by their own level, and never sit at or above the
// parent list even when the payload repeats the parent level.
func TestRenderListIndentsNestedLists(t *testing.T) {
	result := renderDocument("", []json.RawMessage{
		rawJSON(`{"blockType":"unorderedList","unorderedList":{"list":{"level":0}},"children":[` +
			"{\"elementType\":\"text\",\"text\":\"\u7236\u9879\"}," +
			`{"blockType":"unorderedList","unorderedList":{"list":{"level":1}},"children":[` +
			"{\"elementType\":\"text\",\"text\":\"\u5b50\u9879\"}]}," +
			`{"blockType":"unorderedList","unorderedList":{"list":{"level":0}},"children":[` +
			"{\"elementType\":\"text\",\"text\":\"\u5b59\u9879\u540c\u5c42\u7ea7\"}]}]}"),
	})

	want := "- \u7236\u9879\n  - \u5b50\u9879\n  - \u5b59\u9879\u540c\u5c42\u7ea7\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
}

// A multi-line item stays inside its bullet: continuation lines are aligned
// under the first line rather than escaping to the document root.
func TestRenderListIndentsContinuationLines(t *testing.T) {
	// A document title keeps the leading indentation of the first list from
	// being trimmed away, matching how these lists appear in real documents.
	result := renderDocument("\u6d4b\u8bd5\u6587\u6863", []json.RawMessage{
		rawJSON(`{"blockType":"unorderedList","unorderedList":{"list":{"level":1}},"children":[` +
			"{\"blockType\":\"paragraph\",\"paragraph\":{\"text\":\"\u7b2c\u4e00\u884c\\n\u7b2c\u4e8c\u884c\"}}]}"),
	})

	want := "# \u6d4b\u8bd5\u6587\u6863\n\n  - \u7b2c\u4e00\u884c\n    \u7b2c\u4e8c\u884c\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
}

// The documented list shape is flat: one block per item, nesting carried by
// list.level, the items of one list linked by a shared listId across sibling
// blocks. This pins that main path — two sibling items at different levels —
// so the defensive block-child handling above cannot become the assumed model.
func TestRenderListFollowsTheFlatDocumentedShape(t *testing.T) {
	result := renderDocument("", []json.RawMessage{
		rawJSON(`{"blockType":"unorderedList","unorderedList":{"list":{"listId":"list-1","level":0}},` +
			"\"children\":[{\"text\":\"\u7236\u9879\"}]}"),
		rawJSON(`{"blockType":"unorderedList","unorderedList":{"list":{"listId":"list-1","level":1}},` +
			"\"children\":[{\"text\":\"\u5b50\u9879\"}]}"),
		rawJSON(`{"blockType":"unorderedList","unorderedList":{"list":{"listId":"list-2","level":0}},` +
			"\"children\":[{\"text\":\"\u53e6\u4e00\u7ec4\u7684\u9879\"}]}"),
	})

	want := "- \u7236\u9879\n  - \u5b50\u9879\n- \u53e6\u4e00\u7ec4\u7684\u9879\n"
	if result.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", result.Markdown, want)
	}
	if len(result.UnknownTypes) != 0 {
		t.Fatalf("UnknownTypes = %#v, want none", result.UnknownTypes)
	}
}
