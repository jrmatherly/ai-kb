package types

import "strings"

// supportedLocales are IM channel reply languages (the language the model
// writes in), not UI locales: the UI and the embed widget ship only en-US.
var supportedLocales = map[string]struct{}{
	"zh-CN": {},
	"zh-TW": {},
	"en-US": {},
	"ko-KR": {},
	"ja-JP": {},
	"ru-RU": {},
}

// NormalizeSupportedLocale returns a trimmed, supported locale tag or an empty
// string when the value is blank or unsupported.
func NormalizeSupportedLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	if _, ok := supportedLocales[locale]; ok {
		return locale
	}
	return ""
}
