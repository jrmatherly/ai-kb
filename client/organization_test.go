package client

import (
	"context"
	"io"
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

// inviteServer records the body of POST /api/v1/organizations/org-1/invite.
func inviteServer(t *testing.T, body *string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/organizations/org-1/invite" {
			t.Errorf("request = %s %s, want POST /api/v1/organizations/org-1/invite", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		b, _ := io.ReadAll(r.Body)
		*body = string(b)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
}

func TestInviteTenant(t *testing.T) {
	var body string
	srv := inviteServer(t, &body)
	defer srv.Close()

	candidate := TenantInviteCandidate{TenantID: 10002, TenantName: "Team B"}
	if err := NewClient(srv.URL).InviteTenant(context.Background(), "org-1", candidate.TenantID, "viewer"); err != nil {
		t.Fatal(err)
	}
	if want := `{"role":"viewer","tenant_id":10002}`; body != want {
		t.Errorf("body = %s, want %s", body, want)
	}
}

func TestInviteMemberByUserID(t *testing.T) {
	var body string
	srv := inviteServer(t, &body)
	defer srv.Close()

	if err := NewClient(srv.URL).InviteMember(context.Background(), "org-1", "u-1", "editor"); err != nil {
		t.Fatal(err)
	}
	if want := `{"role":"editor","user_id":"u-1"}`; body != want {
		t.Errorf("body = %s, want %s", body, want)
	}
}

// Mirrors internal/handler/organization.go SearchOrganizations: q and limit in,
// a top-level data array of searchable items out.
func TestSearchOrganizations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/organizations/search" {
			t.Errorf("request = %s %s, want GET /api/v1/organizations/search", r.Method, r.URL.Path)
		}
		if q := r.URL.Query(); q.Get("q") != "eng" || q.Get("limit") != "5" {
			t.Errorf("query = %s, want q=eng&limit=5", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"success":true,"total":1,"data":[` +
			`{"id":"org-1","name":"Eng","member_count":3,"is_already_member":true}]}`))
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL).SearchOrganizations(context.Background(), "eng", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "org-1" || got[0].MemberCount != 3 {
		t.Errorf("organizations = %+v, want one org-1 with 3 members", got)
	}
}
