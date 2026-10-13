package wecom

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/im"
)

// mixedMsg builds a botMessage with msgtype "mixed" from text/image parts.
func mixedMsg(parts ...botMixedItem) *botMessage {
	msg := &botMessage{}
	msg.MsgType = "mixed"
	msg.MsgID = "msg_mixed_1"
	msg.From.UserID = "user1"
	msg.Mixed.MsgItem = parts
	return msg
}

func textItem(content string) botMixedItem {
	var item botMixedItem
	item.MsgType = "text"
	item.Text.Content = content
	return item
}

func imageItem(url, aesKey string) botMixedItem {
	var item botMixedItem
	item.MsgType = "image"
	item.Image.URL = url
	item.Image.AESKey = aesKey
	return item
}

func TestConvertMixedMessage(t *testing.T) {
	c := &LongConnClient{}

	tests := []struct {
		name        string
		msg         *botMessage
		chatType    im.ChatType
		wantNil     bool
		wantType    im.MessageType
		wantContent string
		wantFileKey string
		wantAESKey  string
	}{
		{
			name:        "text plus image: image kept as attachment, text as caption",
			msg:         mixedMsg(textItem("\u8fd9\u5f20\u56fe\u91cc\u662f\u4ec0\u4e48\uff1f"), imageItem("https://img.example/1", "key1")),
			chatType:    im.ChatTypeDirect,
			wantType:    im.MessageTypeImage,
			wantContent: "\u8fd9\u5f20\u56fe\u91cc\u662f\u4ec0\u4e48\uff1f",
			wantFileKey: "https://img.example/1",
			wantAESKey:  "key1",
		},
		{
			name: "multiple images with text: first image kept",
			msg: mixedMsg(
				textItem("\u8fd9\u4e24\u5f20\u56fe\u6709\u4ec0\u4e48\u533a\u522b\uff1f"),
				imageItem("https://img.example/1", "key1"),
				imageItem("https://img.example/2", "key2"),
			),
			chatType:    im.ChatTypeDirect,
			wantType:    im.MessageTypeImage,
			wantContent: "\u8fd9\u4e24\u5f20\u56fe\u6709\u4ec0\u4e48\u533a\u522b\uff1f",
			wantFileKey: "https://img.example/1",
			wantAESKey:  "key1",
		},
		{
			name:        "text only: stays text message",
			msg:         mixedMsg(textItem("\u7eaf\u6587\u5b57\u95ee\u9898"), textItem("\u7b2c\u4e8c\u6bb5")),
			chatType:    im.ChatTypeDirect,
			wantType:    im.MessageTypeText,
			wantContent: "\u7eaf\u6587\u5b57\u95ee\u9898\n\u7b2c\u4e8c\u6bb5",
			wantFileKey: "",
			wantAESKey:  "",
		},
		{
			name:        "image only: image message without caption",
			msg:         mixedMsg(imageItem("https://img.example/only", "keyOnly")),
			chatType:    im.ChatTypeDirect,
			wantType:    im.MessageTypeImage,
			wantContent: "",
			wantFileKey: "https://img.example/only",
			wantAESKey:  "keyOnly",
		},
		{
			name:        "group chat: mention stripped from caption",
			msg:         mixedMsg(textItem("@AI KB Bot  \u8fd9\u5f20\u56fe\u91cc\u662f\u4ec0\u4e48\uff1f"), imageItem("https://img.example/1", "key1")),
			chatType:    im.ChatTypeGroup,
			wantType:    im.MessageTypeImage,
			wantContent: "\u8fd9\u5f20\u56fe\u91cc\u662f\u4ec0\u4e48\uff1f",
			wantFileKey: "https://img.example/1",
			wantAESKey:  "key1",
		},
		{
			name:     "empty mixed message returns nil",
			msg:      mixedMsg(textItem("   "), imageItem("", "")),
			chatType: im.ChatTypeDirect,
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.convertMixedMessage(tt.msg, "", tt.chatType, "req_1")
			if tt.wantNil {
				if got != nil {
					t.Fatalf("convertMixedMessage() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("convertMixedMessage() = nil, want non-nil")
			}
			if got.MessageType != tt.wantType {
				t.Errorf("MessageType = %v, want %v", got.MessageType, tt.wantType)
			}
			if got.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", got.Content, tt.wantContent)
			}
			if got.FileKey != tt.wantFileKey {
				t.Errorf("FileKey = %q, want %q", got.FileKey, tt.wantFileKey)
			}
			if got.Extra["aes_key"] != tt.wantAESKey {
				t.Errorf("Extra[aes_key] = %q, want %q", got.Extra["aes_key"], tt.wantAESKey)
			}
			if got.Extra["req_id"] != "req_1" {
				t.Errorf("Extra[req_id] = %q, want %q", got.Extra["req_id"], "req_1")
			}
		})
	}
}
