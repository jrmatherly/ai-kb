package im

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/jrmatherly/ai-kb/internal/logger"
)

// PlatformsEnv is the deployment allow-list of IM platforms. Unset or blank
// means every platform (compose templates pass empty values through); a list
// that names only unknown platforms means none. Separators and case handling
// mirror STORAGE_ALLOW_LIST (internal/storageallowlist).
const PlatformsEnv = "AIKB_IM_PLATFORMS"

// AllPlatforms lists every platform with an adapter, in adapter.go order.
func AllPlatforms() []Platform {
	return []Platform{
		PlatformWeCom, PlatformFeishu, PlatformLark, PlatformSlack, PlatformTelegram,
		PlatformDingtalk, PlatformMattermost, PlatformWeChat, PlatformQQBot, PlatformYunzhijia,
	}
}

var unknownPlatformWarning sync.Once

// EnabledPlatforms returns the allow-listed platforms in AllPlatforms order.
func EnabledPlatforms() []Platform {
	raw := strings.TrimSpace(os.Getenv(PlatformsEnv))
	all := AllPlatforms()
	if raw == "" {
		return all
	}
	wanted := make(map[Platform]bool)
	var unknown []string
	for _, item := range strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '|', '\n', '\t', ' ':
			return true
		default:
			return false
		}
	}) {
		name := Platform(strings.ToLower(strings.TrimSpace(item)))
		known := false
		for _, p := range all {
			if p == name {
				known = true
				break
			}
		}
		if known {
			wanted[name] = true
		} else {
			unknown = append(unknown, string(name))
		}
	}
	if len(unknown) > 0 {
		unknownPlatformWarning.Do(func() {
			logger.Warnf(context.Background(), "[IM] %s names unknown platforms, ignored: %s",
				PlatformsEnv, strings.Join(unknown, ", "))
		})
	}
	enabled := make([]Platform, 0, len(wanted))
	for _, p := range all {
		if wanted[p] {
			enabled = append(enabled, p)
		}
	}
	return enabled
}

// EnabledPlatformNames is EnabledPlatforms as strings, for payloads and messages.
func EnabledPlatformNames() []string {
	enabled := EnabledPlatforms()
	names := make([]string, 0, len(enabled))
	for _, p := range enabled {
		names = append(names, string(p))
	}
	return names
}

// KnownPlatform reports whether p has an adapter in this build.
func KnownPlatform(p Platform) bool {
	for _, known := range AllPlatforms() {
		if known == p {
			return true
		}
	}
	return false
}

// PlatformEnabled reports whether the deployment allow-list includes p.
func PlatformEnabled(p Platform) bool {
	for _, enabled := range EnabledPlatforms() {
		if enabled == p {
			return true
		}
	}
	return false
}
