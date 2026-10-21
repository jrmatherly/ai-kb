// Package envlist parses the deployment allow-lists read from env vars
// (AIKB_IM_PLATFORMS, AIKB_DATASOURCE_TYPES, STORAGE_ALLOW_LIST).
package envlist

import (
	"slices"
	"strings"
)

// Parse splits value on commas, semicolons, pipes and whitespace, lowercases
// each name and enables those in known. A blank value enables every known
// name. Names outside known come back in unknown, in input order, for the
// caller to log or ignore.
func Parse(value string, known []string) (enabled map[string]struct{}, unknown []string) {
	enabled = make(map[string]struct{}, len(known))
	if strings.TrimSpace(value) == "" {
		for _, name := range known {
			enabled[name] = struct{}{}
		}
		return enabled, nil
	}
	for _, item := range strings.FieldsFunc(value, isSeparator) {
		name := strings.ToLower(strings.TrimSpace(item))
		if slices.Contains(known, name) {
			enabled[name] = struct{}{}
		} else {
			unknown = append(unknown, name)
		}
	}
	return enabled, unknown
}

func isSeparator(r rune) bool {
	switch r {
	case ',', ';', '|', '\n', '\t', ' ':
		return true
	default:
		return false
	}
}
