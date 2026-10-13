package wecom

import (
	"testing"
)

func TestStripAtMentionBasic(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"no mention", "hello world", "hello world"},
		{"empty string", "", ""},
		{"mention only no content", "@Bot", "@Bot"},
		{"single-word bot double-space", "@Bot  /stop", "/stop"},
		{"single-word bot single-space", "@Bot /stop", "/stop"},
		{"single-word bot double-space chinese", "@Bot  \u4f60\u597d\u4e16\u754c", "\u4f60\u597d\u4e16\u754c"},
		{"multi-word bot double-space", "@AI KB Bot  /stop", "/stop"},
		{"multi-word bot double-space chinese", "@AI KB Bot  \u4ec0\u4e48\u662f\u4e0a\u4e0b\u6587\u5de5" +
			"\u7a0b", "\u4ec0\u4e48\u662f\u4e0a\u4e0b\u6587\u5de5\u7a0b"},
		{"multi-word bot single-space command", "@AI KB Bot /stop", "/stop"},
		{"multi-word bot single-space chinese", "@AI KB Bot \u4ec0\u4e48\u662f\u8001\u767b", "\u4ec0\u4e48\u662f" +
			"\u8001\u767b"},
		{"leading whitespace", "  @Bot  hello  ", "hello"},
		{"double-space in user content", "@Bot  hello  world", "hello  world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripAtMentionBasic(tt.content)
			if got != tt.want {
				t.Errorf("stripAtMentionBasic(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestLongConnClient_StripAtMention(t *testing.T) {
	t.Run("learns bot name from double-space then handles single-space", func(t *testing.T) {
		c := &LongConnClient{}

		// First message: double-space → learn "AI KB Bot"
		got := c.stripAtMention("@AI KB Bot  \u4ec0\u4e48\u662f\u4e0a\u4e0b\u6587\u5de5\u7a0b")
		if got != "\u4ec0\u4e48\u662f\u4e0a\u4e0b\u6587\u5de5\u7a0b" {
			t.Errorf("first message: got %q, want %q", got, "\u4ec0\u4e48\u662f\u4e0a\u4e0b\u6587\u5de5\u7a0b")
		}

		// Verify bot name was cached
		if name, _ := c.botDisplayName.Load().(string); name != "AI KB Bot" {
			t.Errorf("cached bot name = %q, want %q", name, "AI KB Bot")
		}

		// Second message: single-space → should use cached name
		got = c.stripAtMention("@AI KB Bot /stop")
		if got != "/stop" {
			t.Errorf("second message: got %q, want %q", got, "/stop")
		}

		// Single-space with chinese content
		got = c.stripAtMention("@AI KB Bot \u4ec0\u4e48\u662fB+\u6811")
		if got != "\u4ec0\u4e48\u662fB+\u6811" {
			t.Errorf("chinese single-space: got %q, want %q", got, "\u4ec0\u4e48\u662fB+\u6811")
		}
	})

	t.Run("preconfigured bot name", func(t *testing.T) {
		c := &LongConnClient{}
		c.botDisplayName.Store("AI KB Bot")

		// Single-space should work immediately without learning
		got := c.stripAtMention("@AI KB Bot /stop")
		if got != "/stop" {
			t.Errorf("got %q, want %q", got, "/stop")
		}
	})

	t.Run("cached name must not partial-match a longer bot name", func(t *testing.T) {
		c := &LongConnClient{}
		c.botDisplayName.Store("Bot")

		// "@BotX /stop" should NOT match cached "Bot" — "BotX" is a different name.
		// Falls through to Strategy 3 (strip first @word).
		got := c.stripAtMention("@BotX /stop")
		if got != "/stop" {
			t.Errorf("got %q, want %q", got, "/stop")
		}
	})

	t.Run("NewLongConnClient with bot name", func(t *testing.T) {
		c, err := NewLongConnClient("id", "secret", "", "My Bot", nil)
		if err != nil {
			t.Fatal(err)
		}

		got := c.stripAtMention("@My Bot /help")
		if got != "/help" {
			t.Errorf("got %q, want %q", got, "/help")
		}
	})

	t.Run("does not overwrite cached name from user double-spaces", func(t *testing.T) {
		c := &LongConnClient{}
		c.botDisplayName.Store("Bot")

		// User content has double space — should NOT overwrite "Bot" with "Bot hello"
		got := c.stripAtMention("@Bot  hello  world")
		if got != "hello  world" {
			t.Errorf("got %q, want %q", got, "hello  world")
		}

		// Cached name should still be "Bot"
		if name, _ := c.botDisplayName.Load().(string); name != "Bot" {
			t.Errorf("cached bot name = %q, want %q", name, "Bot")
		}
	})

	t.Run("no mention passthrough", func(t *testing.T) {
		c := &LongConnClient{}
		got := c.stripAtMention("hello world")
		if got != "hello world" {
			t.Errorf("got %q, want %q", got, "hello world")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		c := &LongConnClient{}
		got := c.stripAtMention("")
		if got != "" {
			t.Errorf("got %q, want %q", got, "")
		}
	})

	t.Run("cold start single-space command", func(t *testing.T) {
		c := &LongConnClient{}
		// No cached name, single space → heuristic detects " /" boundary
		got := c.stripAtMention("@AI KB Bot /stop")
		if got != "/stop" {
			t.Errorf("got %q, want %q", got, "/stop")
		}
	})

	t.Run("cold start single-space chinese", func(t *testing.T) {
		c := &LongConnClient{}
		// No cached name, single space → heuristic detects non-ASCII boundary
		got := c.stripAtMention("@AI KB Bot \u4ec0\u4e48\u662f\u8001\u767b")
		if got != "\u4ec0\u4e48\u662f\u8001\u767b" {
			t.Errorf("got %q, want %q", got, "\u4ec0\u4e48\u662f\u8001\u767b")
		}
	})

	t.Run("cold start all-ascii content falls back to first word", func(t *testing.T) {
		c := &LongConnClient{}
		// No cached name, all ASCII → heuristic can't find boundary, strips first @word
		got := c.stripAtMention("@AI KB Bot hello")
		if got != "KB Bot hello" {
			t.Errorf("got %q, want %q", got, "KB Bot hello")
		}
	})
}
