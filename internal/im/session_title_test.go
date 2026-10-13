package im

import "testing"

func TestShortID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"shorter than 8", "abc", "abc"},
		{"exactly 8", "12345678", "12345678"},
		{"longer than 8 keeps suffix", "aaaaaaaaXXXXXXXX", "XXXXXXXX"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortID(tt.in); got != tt.want {
				t.Errorf("shortID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestBuildUserSessionTitle(t *testing.T) {
	tests := []struct {
		name string
		msg  *IncomingMessage
		want string
	}{
		{
			name: "group with username",
			msg: &IncomingMessage{
				Platform: "feishu",
				UserName: "\u674e\u56db",
				ChatType: ChatTypeGroup,
				ChatID:   "oc_aaaaaaaaaaaaaaaa",
			},
			want: "\u674e\u56db · group aaaaaaaa",
		},
		{
			name: "direct message with username",
			msg: &IncomingMessage{
				Platform: "feishu",
				UserName: "\u674e\u56db",
				ChatType: ChatTypeDirect,
			},
			want: "\u674e\u56db · dm",
		},
		{
			name: "group without username falls back to user id",
			msg: &IncomingMessage{
				Platform: "wecom",
				UserID:   "WeCom_ZhangSan",
				ChatType: ChatTypeGroup,
				ChatID:   "wc_group_1234",
			},
			want: "user ZhangSan · group oup_1234",
		},
		{
			name: "no user identity at all",
			msg: &IncomingMessage{
				Platform: "slack",
				ChatType: ChatTypeDirect,
			},
			want: "user · dm",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildUserSessionTitle(tt.msg); got != tt.want {
				t.Errorf("buildUserSessionTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildThreadSessionTitle(t *testing.T) {
	tests := []struct {
		name string
		msg  *IncomingMessage
		want string
	}{
		{
			name: "chat + thread",
			msg: &IncomingMessage{
				Platform: "slack",
				ChatID:   "C0123456789",
				ThreadID: "1700000000.111000",
			},
			want: "chat 23456789 · thread 0.111000",
		},
		{
			name: "thread without chat id",
			msg: &IncomingMessage{
				Platform: "feishu",
				ThreadID: "om_thread_abcdefgh",
			},
			want: "thread abcdefgh",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildThreadSessionTitle(tt.msg); got != tt.want {
				t.Errorf("buildThreadSessionTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIMInitialSessionTitle(t *testing.T) {
	tests := []struct {
		name string
		msg  *IncomingMessage
		want string
	}{
		{
			name: "text message starts untitled so it gets a content-based title later",
			msg: &IncomingMessage{
				Platform: "feishu",
				UserName: "\u674e\u56db",
				ChatType: ChatTypeDirect,
				Content:  "\u5982\u4f55\u914d\u7f6e\u5355\u70b9\u767b\u5f55?",
			},
			want: "",
		},
		{
			name: "whitespace-only content falls back to the identity title",
			msg: &IncomingMessage{
				Platform: "feishu",
				UserName: "\u674e\u56db",
				ChatType: ChatTypeDirect,
				Content:  "   ",
			},
			want: "\u674e\u56db · dm",
		},
		{
			name: "non-text message (no content) falls back to the identity title",
			msg: &IncomingMessage{
				Platform: "wecom",
				UserID:   "WeCom_ZhangSan",
				ChatType: ChatTypeGroup,
				ChatID:   "wc_group_1234",
			},
			want: "user ZhangSan · group oup_1234",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := imInitialSessionTitle(tt.msg, buildUserSessionTitle); got != tt.want {
				t.Errorf("imInitialSessionTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
