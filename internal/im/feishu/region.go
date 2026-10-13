package feishu

import "github.com/jrmatherly/ai-kb/internal/im"

// Open Platform API origins. Feishu and Lark are the same product deployed on
// two isolated clouds; the API surface is identical, only the host differs.
const (
	feishuOpenBaseURL = "https://open.feishu.cn"
	larkOpenBaseURL   = "https://open.larksuite.com"
)

// Region selects which cloud an adapter talks to. Apps, tenants, tokens and
// resource keys (image_key, file_key, card_id) are scoped to a single cloud and
// are never valid on the other, so a Region is fixed per channel at creation.
type Region struct {
	// Platform is the im.Platform reported on messages parsed by this adapter.
	Platform im.Platform
	// OpenBaseURL is the Open Platform API origin, without a trailing slash.
	OpenBaseURL string
	// Label prefixes log lines so operators can tell the clouds apart.
	Label string
}

const (
	// thinkingText is the placeholder shown in a streaming card before the
	// first answer chunk arrives.
	thinkingText = "Thinking..."
	// imageFallbackLabel labels the plain link an image degrades to when it
	// cannot be uploaded to the platform.
	imageFallbackLabel = "Image"
)

var (
	// RegionFeishu is the Chinese mainland cloud (Feishu).
	RegionFeishu = Region{
		Platform:    im.PlatformFeishu,
		OpenBaseURL: feishuOpenBaseURL,
		Label:       "Feishu",
	}

	// RegionLark is the international cloud (Lark).
	RegionLark = Region{
		Platform:    im.PlatformLark,
		OpenBaseURL: larkOpenBaseURL,
		Label:       "Lark",
	}
)
