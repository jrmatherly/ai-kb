package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// Mirrors internal/handler/organization.go SearchTenantsForInvite: one exact
// workspace ID in q, tenant-grouped candidates back.
func searchTenantsServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/organizations/org-1/search-tenants" {
			t.Errorf("request = %s %s, want GET /api/v1/organizations/org-1/search-tenants", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("q") != "10002" {
			_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":[{"tenant_id":10002,"tenant_name":"Team B"}]}`))
	}))
}

func TestSearchTenantsForInvite(t *testing.T) {
	srv := searchTenantsServer(t)
	defer srv.Close()

	got, err := NewClient(srv.URL).SearchTenantsForInvite(context.Background(), "org-1", "10002")
	if err != nil {
		t.Fatal(err)
	}
	want := []TenantInviteCandidate{{TenantID: 10002, TenantName: "Team B"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidates = %+v, want %+v", got, want)
	}
}
