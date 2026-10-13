package main

import "testing"

func TestCalVerNewer(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"v2026.10.13.1", "v2026.10.13", true},
		{"v2026.10.13", "v2026.10.12.5", true},
		{"v2026.10.13.1", "v2026.10.12.5", true},
		{"v2026.10.13", "v2026.10.13.1", false},
		{"v2026.10.12.5", "v2026.10.13", false},
		{"v2026.10.13", "v2026.10.13.0", false},
		{"v2026.10.1", "v2026.9.30", true},
		{"v2027.1.1", "v2026.12.31", true},
		{"vlatest", "v2026.10.13", false},
		{"v2026.11", "v2026.10.13", false},
		{"v2026.11.1-rc1", "v2026.10.13", false},
		{"v2026.11.1.1.1", "v2026.10.13", false},
		{"v2026.11.1", "vdev", false},
	} {
		if got := calVerNewer(tc.a, tc.b); got != tc.want {
			t.Errorf("calVerNewer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestFindBestAssetPicksDesktopAsset(t *testing.T) {
	v := "v2026.10.13"
	var assets []githubAsset
	for _, n := range []string{
		"ai-kb-lite_" + v + "_linux_amd64.tar.gz",
		"ai-kb-lite_" + v + "_linux_amd64.tar.gz.sha256",
		"ai-kb-lite_" + v + "_linux_arm64.tar.gz",
		"ai-kb-lite_" + v + "_darwin_amd64.tar.gz",
		"ai-kb-lite_" + v + "_darwin_arm64.tar.gz",
		"ai-kb-lite-app_" + v + "_macOS_arm64.dmg.sha256",
		"ai-kb-lite-app_" + v + "_macOS_arm64.dmg",
		"ai-kb-lite-app_" + v + "_macOS_amd64.dmg",
		"ai-kb-lite-app_" + v + "_linux_amd64.tar.gz.sha256",
		"ai-kb-lite-app_" + v + "_linux_amd64.tar.gz",
		"ai-kb-lite-app_" + v + "_windows_amd64_setup.exe",
		"ai-kb-lite-app_" + v + "_windows_amd64_setup.exe.sha256",
	} {
		assets = append(assets, githubAsset{Name: n, BrowserDownloadURL: "https://example.test/" + n})
	}
	for _, tc := range []struct{ goos, goarch, want string }{
		{"linux", "amd64", "ai-kb-lite-app_" + v + "_linux_amd64.tar.gz"},
		{"linux", "arm64", ""},
		{"darwin", "arm64", "ai-kb-lite-app_" + v + "_macOS_arm64.dmg"},
		{"darwin", "amd64", "ai-kb-lite-app_" + v + "_macOS_amd64.dmg"},
		{"windows", "amd64", "ai-kb-lite-app_" + v + "_windows_amd64_setup.exe"},
	} {
		url, name := findBestAsset(assets, tc.goos, tc.goarch)
		if name != tc.want || (tc.want != "" && url != "https://example.test/"+tc.want) {
			t.Errorf("%s/%s: got %q (%q), want %q", tc.goos, tc.goarch, name, url, tc.want)
		}
	}
}
