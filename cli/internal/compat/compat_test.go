package compat_test

import (
	"testing"

	"github.com/jrmatherly/ai-kb/cli/internal/compat"
)

func TestCompat(t *testing.T) {
	tests := []struct {
		name      string
		serverVer string
		cliVer    string
		wantLevel compat.Level
		wantHint  string
	}{
		{"equal", "2026.10.13", "2026.10.13", compat.OK, ""},
		{"missing micro is zero", "2026.10.13.0", "2026.10.13", compat.OK, ""},
		{"client older day", "2026.10.13", "2026.10.12", compat.OK, ""},
		{"client older year", "2027.1.1", "2026.12.31", compat.OK, ""},
		{
			"client newer micro", "2026.10.13", "2026.10.13.1", compat.SoftWarn,
			"server is older (server 2026.10.13, client 2026.10.13.1); some new features may be unavailable",
		},
		{
			"client newer month", "2026.10.31", "2026.11.1", compat.SoftWarn,
			"server is older (server 2026.10.31, client 2026.11.1); some new features may be unavailable",
		},
		{
			"client newer year never hard-errors", "2026.12.31", "2027.1.1", compat.SoftWarn,
			"server is older (server 2026.12.31, client 2027.1.1); some new features may be unavailable",
		},
		{
			"numeric not lexical", "2026.9.30", "2026.10.1", compat.SoftWarn,
			"server is older (server 2026.9.30, client 2026.10.1); some new features may be unavailable",
		},
		{
			"v-prefix and describe suffix", "v2026.10.12", "v2026.10.13-3-gabc1234", compat.SoftWarn,
			"server is older (server v2026.10.12, client v2026.10.13-3-gabc1234); some new features may be unavailable",
		},
		{"client unset", "2026.10.13", "(unknown)", compat.OK, ""}, // dev build; no warning
		{"server unset", "", "2026.10.13", compat.OK, ""},
		{"malformed server", "garbage", "2026.10.13", compat.OK, ""}, // fail-open
		{"two parts", "2026.10", "2026.11.1", compat.OK, ""},
		{"five parts", "2026.10.13.1.2", "2027.1.1", compat.OK, ""},
	}
	for _, tt := range tests {
		gotLevel, gotHint := compat.Compat(tt.serverVer, tt.cliVer)
		if gotLevel != tt.wantLevel || gotHint != tt.wantHint {
			t.Errorf("%s: Compat(%q, %q) = (%v, %q), want (%v, %q)",
				tt.name, tt.serverVer, tt.cliVer, gotLevel, gotHint, tt.wantLevel, tt.wantHint)
		}
	}
}
