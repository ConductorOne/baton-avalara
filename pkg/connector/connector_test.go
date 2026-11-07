package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"

	cfg "github.com/conductorone/baton-avalara/pkg/config"
)

// newTestConnector starts a fake Avalara API. The handler receives each request.
func newTestConnector(t *testing.T, h http.HandlerFunc) (*Avalara, string) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	cb, _, err := New(context.Background(), &cfg.Avalara{
		Username: "u", Password: "p", BaseUrl: srv.URL,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return cb.(*Avalara), srv.URL
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Error(err)
	}
}

func opts(token string) rs.SyncOpAttrs {
	return rs.SyncOpAttrs{PageToken: pagination.Token{Token: token}}
}

func TestUserListPassthrough(t *testing.T) {
	var srvURL string
	a, url := newTestConnector(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			writeJSON(t, w, map[string]any{"value": []map[string]any{{"id": 2, "userName": "b", "email": "b@x.com", "isActive": true}}})
			return
		}
		writeJSON(t, w, map[string]any{
			"value":     []map[string]any{{"id": 1, "userName": "a", "email": "a@x.com", "isActive": true}},
			"@nextLink": srvURL + "/api/v2/users?page=2",
		})
	})
	srvURL = url

	ub := newUserBuilder(a.client)
	rv, res, err := ub.List(context.Background(), nil, opts(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(rv) != 1 || res.NextPageToken != srvURL+"/api/v2/users?page=2" {
		t.Fatalf("unexpected first page: %d resources, token %q", len(rv), res.NextPageToken)
	}

	rv, res, err = ub.List(context.Background(), nil, opts(res.NextPageToken))
	if err != nil {
		t.Fatal(err)
	}
	if len(rv) != 1 || rv[0].DisplayName != "b" || res.NextPageToken != "" {
		t.Fatalf("unexpected second page: %+v token %q", rv, res.NextPageToken)
	}
}

func TestRoleListPassthrough(t *testing.T) {
	var srvURL string
	a, url := newTestConnector(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			writeJSON(t, w, map[string]any{"value": []map[string]any{{"id": 20, "description": "Two"}}})
			return
		}
		writeJSON(t, w, map[string]any{
			"value":     []map[string]any{{"id": 10, "description": "One"}},
			"@nextLink": srvURL + "/api/v2/definitions/securityroles?page=2",
		})
	})
	srvURL = url

	rb := newRoleBuilder(a.client)
	rv, res, err := rb.List(context.Background(), nil, opts(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(rv) != 1 || rv[0].Id.Resource != "10" || res.NextPageToken == "" {
		t.Fatalf("unexpected first page: %+v token %q", rv, res.NextPageToken)
	}
	// The profile must be set on the resource and on the role trait.
	if d, ok := rs.GetProfileStringValue(rs.GetProfile(rv[0]), "description"); !ok || d != "One" {
		t.Errorf("resource profile description = %q, %v", d, ok)
	}
	if id, ok := rs.GetProfileStringValue(rs.GetProfile(rv[0]), "id"); !ok || id != "10" {
		t.Errorf("resource profile id = %q, %v", id, ok)
	}
	trait, err := rs.GetRoleTrait(rv[0])
	if err != nil {
		t.Fatal(err)
	}
	if d, ok := rs.GetProfileStringValue(trait.GetProfile(), "description"); !ok || d != "One" {
		t.Errorf("trait profile description = %q, %v", d, ok)
	}
	if id, ok := rs.GetProfileStringValue(trait.GetProfile(), "id"); !ok || id != "10" {
		t.Errorf("trait profile id = %q, %v", id, ok)
	}
	rv, res, err = rb.List(context.Background(), nil, opts(res.NextPageToken))
	if err != nil {
		t.Fatal(err)
	}
	if len(rv) != 1 || rv[0].Id.Resource != "20" || res.NextPageToken != "" {
		t.Fatalf("unexpected second page: %+v token %q", rv, res.NextPageToken)
	}
}

func TestRoleGrants(t *testing.T) {
	a, _ := newTestConnector(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"value": []map[string]any{
			{"id": 1, "userName": "a", "securityRoleId": "Admin"},
			{"id": 2, "userName": "b", "securityRoleId": "Viewer"},
			{"id": 3, "userName": "c", "securityRoleId": "Admin"},
		}})
	})

	rb := newRoleBuilder(a.client)
	role, err := rs.NewRoleResource("Admin", roleResourceType, "10", nil,
		rs.WithResourceProfile(map[string]interface{}{"id": "10", "description": "Admin"}))
	if err != nil {
		t.Fatal(err)
	}

	grants, res, err := rb.Grants(context.Background(), role, opts(""))
	if err != nil {
		t.Fatal(err)
	}
	if res.NextPageToken != "" {
		t.Fatalf("unexpected token %q", res.NextPageToken)
	}
	var ids []string
	for _, g := range grants {
		if g.Entitlement.Id != "role:10:"+RoleMemberEntitlement {
			t.Errorf("unexpected entitlement id %q", g.Entitlement.Id)
		}
		ids = append(ids, g.Principal.Id.Resource)
	}
	if len(ids) != 2 || ids[0] != "1" || ids[1] != "3" {
		t.Fatalf("unexpected grants: %v", ids)
	}

	// A role without a description in its profile must fail.
	bad := &v2.Resource{Id: role.Id}
	if _, _, err := rb.Grants(context.Background(), bad, opts("")); err == nil {
		t.Fatal("expected error for missing description")
	}
}
