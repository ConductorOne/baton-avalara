package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func newTestServer(t *testing.T, st State) *httptest.Server {
	t.Helper()
	t.Setenv("BASE_URL", "")
	ts := httptest.NewServer(newHandler(config{Username: "testuser", Password: "testpass"}, st))
	t.Cleanup(ts.Close)
	return ts
}

func do(t *testing.T, method, url, body, pass string) (*http.Response, map[string]interface{}) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("testuser", pass)
	req.Header.Set("X-Avalara-Client", "test")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out := map[string]interface{}{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func get(t *testing.T, url string) (*http.Response, map[string]interface{}) {
	return do(t, http.MethodGet, url, "", "testpass")
}

func TestDefaultCounts(t *testing.T) {
	ts := newTestServer(t, defaultState())
	_, u := get(t, ts.URL+"/api/v2/users")
	if len(u["value"].([]interface{})) != 10 {
		t.Fatalf("want 10 users")
	}
	_, r := get(t, ts.URL+"/api/v2/definitions/securityroles")
	if len(r["value"].([]interface{})) != 22 {
		t.Fatalf("want 22 roles")
	}
	if _, ok := r["@nextLink"]; ok {
		t.Fatalf("no nextLink expected when all roles fit")
	}
}

func TestUsersPaging(t *testing.T) {
	st := defaultState()
	st.Users = generateUsers(250, st.Roles)
	ts := newTestServer(t, st)

	seen := 0
	next := ts.URL + "/api/v2/users?$top=100"
	pages := 0
	for next != "" {
		_, body := get(t, next)
		seen += len(body["value"].([]interface{}))
		pages++
		link, _ := body["@nextLink"].(string)
		if link != "" && !strings.HasPrefix(link, ts.URL) {
			t.Fatalf("nextLink must use request host, got %s", link)
		}
		next = link
	}
	if seen != 250 || pages != 3 {
		t.Fatalf("seen=%d pages=%d", seen, pages)
	}
}

func TestLargeSkipDoesNotPanic(t *testing.T) {
	ts := newTestServer(t, defaultState())
	for _, p := range []string{"/api/v2/definitions/securityroles?$skip=9999", "/api/v2/users?$skip=9999&$top=5"} {
		resp, body := get(t, ts.URL+p)
		if resp.StatusCode != 200 || len(body["value"].([]interface{})) != 0 {
			t.Fatalf("%s: status %d body %v", p, resp.StatusCode, body)
		}
		if _, ok := body["@nextLink"]; ok {
			t.Fatalf("%s: unexpected nextLink", p)
		}
	}
}

func TestRolesTopAndFilter(t *testing.T) {
	ts := newTestServer(t, defaultState())
	_, body := get(t, ts.URL+"/api/v2/definitions/securityroles?$top=10")
	if len(body["value"].([]interface{})) != 10 || body["@nextLink"] == nil {
		t.Fatalf("want 10 roles and nextLink: %v", body)
	}
	_, body = get(t, ts.URL+"/api/v2/definitions/securityroles?$filter=id%20eq%203")
	if len(body["value"].([]interface{})) != 1 {
		t.Fatalf("filter failed: %v", body)
	}
}

func TestBaseURLEnv(t *testing.T) {
	ts := newTestServer(t, defaultState())
	t.Setenv("BASE_URL", "example.test:9")
	_, body := get(t, ts.URL+"/api/v2/users?$top=1")
	if link := body["@nextLink"].(string); !strings.HasPrefix(link, "http://example.test:9/api/v2/users") {
		t.Fatalf("got %s", link)
	}
}

func TestFaults(t *testing.T) {
	ts := newTestServer(t, defaultState())
	resp, _ := do(t, http.MethodPost, ts.URL+"/_mock/faults", `{"path":"/api/v2/users","status":429,"retryAfter":2,"count":2}`, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("fault create: %d", resp.StatusCode)
	}
	for i := 0; i < 2; i++ {
		resp, body := get(t, ts.URL+"/api/v2/users")
		if resp.StatusCode != 429 || resp.Header.Get("Retry-After") != "2" {
			t.Fatalf("want 429 with Retry-After, got %d", resp.StatusCode)
		}
		e := body["error"].(map[string]interface{})
		if _, ok := e["details"].([]interface{}); !ok {
			t.Fatalf("details must be a list: %v", e)
		}
	}
	if resp, _ := get(t, ts.URL+"/api/v2/users"); resp.StatusCode != 200 {
		t.Fatalf("fault should be used up, got %d", resp.StatusCode)
	}
	// Other paths are not affected, and DELETE clears rules.
	do(t, http.MethodPost, ts.URL+"/_mock/faults", `{"path":"/api/v2/users","status":500}`, "")
	do(t, http.MethodDelete, ts.URL+"/_mock/faults", "", "")
	if resp, _ := get(t, ts.URL+"/api/v2/users"); resp.StatusCode != 200 {
		t.Fatalf("faults not cleared")
	}
	_, stats := get(t, ts.URL+"/_mock/stats")
	if stats["/api/v2/users"].(float64) != 4 {
		t.Fatalf("stats: %v", stats)
	}
}

func TestAdminChanges(t *testing.T) {
	ts := newTestServer(t, defaultState())
	_, u := do(t, http.MethodPost, ts.URL+"/_mock/users", `{"userName":"new","securityRoleId":"FirmUser","isActive":true}`, "")
	id := int(u["id"].(float64))
	_, _ = do(t, http.MethodPatch, ts.URL+"/_mock/users/12345", `{"securityRoleId":"SystemAdmin","isActive":false}`, "")
	if resp, _ := do(t, http.MethodDelete, ts.URL+"/_mock/users/67890", "", ""); resp.StatusCode != 204 {
		t.Fatalf("delete status %d", resp.StatusCode)
	}
	if resp, _ := do(t, http.MethodPatch, ts.URL+"/_mock/users/1", `{}`, ""); resp.StatusCode != 404 {
		t.Fatalf("want 404")
	}

	_, state := get(t, ts.URL+"/_mock/state")
	users := state["users"].([]interface{})
	if len(users) != 10 {
		t.Fatalf("want 10 users (10 +1 -1), got %d", len(users))
	}
	var found bool
	for _, x := range users {
		m := x.(map[string]interface{})
		if int(m["id"].(float64)) == 12345 && m["securityRoleId"] == "SystemAdmin" && m["isActive"] == false {
			found = true
		}
		if int(m["id"].(float64)) == 67890 {
			t.Fatalf("user 67890 should be deleted")
		}
	}
	if !found || id == 0 {
		t.Fatalf("patch or create missing")
	}

	do(t, http.MethodPost, ts.URL+"/_mock/reset", "", "")
	_, state = get(t, ts.URL+"/_mock/state")
	if len(state["users"].([]interface{})) != 10 {
		t.Fatalf("reset failed")
	}

	do(t, http.MethodPut, ts.URL+"/_mock/state", `{"roles":[{"id":1,"description":"Only"}],"users":[]}`, "")
	_, body := get(t, ts.URL+"/api/v2/definitions/securityroles")
	if len(body["value"].([]interface{})) != 1 {
		t.Fatalf("PUT state failed")
	}
}

func TestPingWrongCredentials(t *testing.T) {
	ts := newTestServer(t, defaultState())
	resp, body := do(t, http.MethodGet, ts.URL+"/api/v2/utilities/ping", "", "wrong")
	if resp.StatusCode != 200 || body["authenticated"] != false {
		t.Fatalf("want 200 authenticated=false, got %d %v", resp.StatusCode, body)
	}
	resp, body = do(t, http.MethodGet, ts.URL+"/api/v2/users", "", "wrong")
	if resp.StatusCode != 401 || body["error"] == nil {
		t.Fatalf("want 401 error body, got %d", resp.StatusCode)
	}

	ts401 := httptest.NewServer(newHandler(config{Username: "testuser", Password: "testpass", Ping401: true}, defaultState()))
	defer ts401.Close()
	resp, _ = do(t, http.MethodGet, ts401.URL+"/api/v2/utilities/ping", "", "wrong")
	if resp.StatusCode != 401 {
		t.Fatalf("want 401 with Ping401, got %d", resp.StatusCode)
	}
}

func TestAdminLoopbackOnly(t *testing.T) {
	h := newHandler(config{Username: "u", Password: "p"}, defaultState())
	req := httptest.NewRequest(http.MethodGet, "/_mock/stats", nil)
	req.RemoteAddr = "10.1.2.3:5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
	h = newHandler(config{Username: "u", Password: "p", AdminRemote: true}, defaultState())
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestFixtureEdgeCases(t *testing.T) {
	data, err := os.ReadFile("testdata/fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	st, err := decodeState(data)
	if err != nil {
		t.Fatal(err)
	}
	var root, inactive, orphan bool
	roles := map[string]bool{}
	for _, r := range st.Roles {
		roles[removeSpaces(r.Description)] = true
	}
	for _, u := range st.Users {
		root = root || (u.SecurityRoleID == "ComplianceRootUser" && roles["ComplianceRootUser"])
		inactive = inactive || !u.IsActive
		orphan = orphan || !roles[u.SecurityRoleID]
	}
	if !root || !inactive || !orphan {
		t.Fatalf("fixture misses edge cases: %v %v %v", root, inactive, orphan)
	}
}

func TestConcurrentAccess(t *testing.T) {
	ts := newTestServer(t, defaultState())
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			get(t, ts.URL+"/api/v2/users")
			do(t, http.MethodPost, ts.URL+"/_mock/users", `{"userName":"x"}`, "")
			do(t, http.MethodPost, ts.URL+"/_mock/faults", `{"path":"/nope","status":500,"count":1}`, "")
		}()
	}
	wg.Wait()
}
