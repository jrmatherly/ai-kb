package middleware

import "testing"

func TestSystemLinksNeedNoAuth(t *testing.T) {
	if !isNoAuthAPI("/api/v1/system/links", "GET") {
		t.Fatal("the login page reads /api/v1/system/links before a session exists")
	}
	if isNoAuthAPI("/api/v1/system/capabilities", "GET") {
		t.Fatal("/system/capabilities must stay authenticated")
	}
}
