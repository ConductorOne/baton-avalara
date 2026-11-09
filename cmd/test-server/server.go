package main

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type config struct {
	Username    string
	Password    string
	Latency     time.Duration
	Ping401     bool // wrong credentials on ping return 401 instead of 200.
	AdminRemote bool // allow non-loopback clients on /_mock/.
}

type server struct {
	cfg   config
	store *store
}

func newHandler(cfg config, initial State) http.Handler {
	s := &server{cfg: cfg, store: newStore(initial)}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/definitions/securityroles", s.api(s.handleSecurityRoles))
	mux.HandleFunc("/api/v2/users", s.api(s.handleUsers))
	mux.HandleFunc("/api/v2/definitions/permissions", s.api(s.handlePermissions))
	mux.HandleFunc("/api/v2/accounts/", s.api(s.handleUserEntitlements))
	mux.HandleFunc("/api/v2/utilities/ping", s.api(s.handlePing))
	mux.HandleFunc("/_mock/", s.admin)
	return logMiddleware(mux)
}

// statusRecorder remembers the response status for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond)) //nolint:gosec // test server.
	})
}

// api wraps an Avalara endpoint: stats, latency, faults, auth.
func (s *server) api(next func(http.ResponseWriter, *http.Request, bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Correlation-Id", uuid.New().String())
		fault := s.store.record(r.Method, r.URL.Path)
		if s.cfg.Latency > 0 {
			time.Sleep(s.cfg.Latency)
		}
		if fault != nil {
			if fault.RetryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(fault.RetryAfter))
			}
			status := fault.Status
			if status == 0 {
				status = http.StatusInternalServerError
			}
			sendError(w, status, "InjectedFault", "Fault injected by the mock server", r.URL.Path, "Fault injected through /_mock/faults")
			return
		}
		authed, msg := s.checkAuth(r)
		if !authed {
			if r.URL.Path == "/api/v2/utilities/ping" && !s.cfg.Ping401 {
				next(w, r, false)
				return
			}
			sendError(w, http.StatusUnauthorized, "AuthenticationException", msg, "Authorization", msg)
			return
		}
		next(w, r, true)
	}
}

func (s *server) checkAuth(r *http.Request) (bool, string) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return false, "Missing Authorization header"
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || parts[0] != "Basic" {
		return false, "Invalid Authorization header format"
	}
	payload, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return false, "Invalid base64 encoding in Authorization header"
	}
	pair := strings.SplitN(string(payload), ":", 2)
	userOK := subtle.ConstantTimeCompare([]byte(pair[0]), []byte(s.cfg.Username)) == 1
	passOK := len(pair) == 2 && subtle.ConstantTimeCompare([]byte(pair[1]), []byte(s.cfg.Password)) == 1
	if !userOK || !passOK {
		return false, "Invalid credentials"
	}
	if r.Header.Get("X-Avalara-Client") == "" {
		return false, "Missing X-Avalara-Client header"
	}
	return true, ""
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("error writing response: %v", err)
	}
}

// sendError writes an error body in the real Avalara shape.
func sendError(w http.ResponseWriter, status int, code, message, target, detail string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
			"target":  target,
			"details": []map[string]interface{}{
				{
					"code":        code,
					"number":      status,
					"message":     message,
					"description": detail,
					"faultCode":   "Client",
					"helpLink":    "http://developer.avalara.com/avatax/errors/" + code,
					"refersTo":    target,
					"severity":    "Error",
				},
			},
		},
	})
}

// baseURL gives the origin used for @nextLink.
func baseURL(r *http.Request) string {
	if b := os.Getenv("BASE_URL"); b != "" {
		if !strings.HasPrefix(b, "http://") && !strings.HasPrefix(b, "https://") {
			b = "http://" + b
		}
		return strings.TrimRight(b, "/")
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// pageBounds returns the slice bounds for $skip and $top. It never panics.
func pageBounds(total, skip, top int) (int, int) {
	if skip < 0 {
		skip = 0
	}
	if skip > total {
		skip = total
	}
	end := total
	if top > 0 && skip+top < total {
		end = skip + top
	}
	return skip, end
}

func nextLink(r *http.Request, end, total int) string {
	if end >= total {
		return ""
	}
	q := r.URL.Query()
	q.Set("$skip", strconv.Itoa(end))
	return baseURL(r) + r.URL.Path + "?" + q.Encode()
}

func (s *server) handleSecurityRoles(w http.ResponseWriter, r *http.Request, _ bool) {
	q := r.URL.Query()
	filter := q.Get("$filter")
	skip, _ := strconv.Atoi(q.Get("$skip"))
	top, _ := strconv.Atoi(q.Get("$top"))

	roles := []Role{}
	for _, role := range s.store.snapshot().Roles {
		if filter == "" || applyRoleFilter(role, filter) {
			roles = append(roles, role)
		}
	}
	start, end := pageBounds(len(roles), skip, top)
	resp := map[string]interface{}{
		"@recordsetCount": len(roles),
		"value":           roles[start:end],
	}
	if link := nextLink(r, end, len(roles)); link != "" {
		resp["@nextLink"] = link
	}
	writeJSON(w, http.StatusOK, resp)
}

func applyRoleFilter(role Role, filter string) bool {
	if strings.Contains(filter, "id eq") {
		idStr := strings.TrimSpace(strings.TrimPrefix(filter, "id eq"))
		if id, err := strconv.Atoi(idStr); err == nil {
			return role.ID == id
		}
	} else if strings.Contains(filter, "description eq") {
		description := strings.Trim(strings.TrimSpace(strings.TrimPrefix(filter, "description eq")), "'")
		return strings.EqualFold(role.Description, description)
	}
	return true
}

func (s *server) handleUsers(w http.ResponseWriter, r *http.Request, _ bool) {
	q := r.URL.Query()
	filter := q.Get("$filter")
	include := q.Get("$include")
	orderBy := q.Get("$orderBy")
	skip, _ := strconv.Atoi(q.Get("$skip"))
	top, _ := strconv.Atoi(q.Get("$top"))

	users := []User{}
	for _, u := range s.store.snapshot().Users {
		if filter == "" || applyUserFilter(u, filter) {
			users = append(users, u)
		}
	}
	if orderBy != "" {
		sortUsers(users, orderBy)
	}
	start, end := pageBounds(len(users), skip, top)
	page := make([]map[string]interface{}, 0, end-start)
	for _, u := range users[start:end] {
		m := userToMap(u)
		if include == "FetchResult" {
			m["FetchResult"] = map[string]interface{}{
				"@recordsetCount": 1,
				"value": []map[string]interface{}{
					{"id": u.ID, "userName": u.UserName, "firstName": u.FirstName, "lastName": u.LastName, "email": u.Email},
				},
			}
		}
		page = append(page, m)
	}
	resp := map[string]interface{}{
		"@recordsetCount": len(users),
		"value":           page,
	}
	if link := nextLink(r, end, len(users)); link != "" {
		resp["@nextLink"] = link
	}
	writeJSON(w, http.StatusOK, resp)
}

func userToMap(u User) map[string]interface{} {
	data, _ := json.Marshal(u)
	m := map[string]interface{}{}
	_ = json.Unmarshal(data, &m)
	return m
}

func applyUserFilter(u User, filter string) bool {
	if strings.Contains(filter, "lastName startsWith") {
		prefix := strings.Trim(strings.TrimSpace(strings.TrimPrefix(filter, "lastName startsWith")), "\"")
		return strings.HasPrefix(strings.ToLower(u.LastName), strings.ToLower(prefix))
	}
	return true
}

func sortUsers(users []User, orderBy string) {
	parts := strings.Split(orderBy, " ")
	field := parts[0]
	ascending := len(parts) == 1 || strings.ToUpper(parts[1]) != "DESC"
	sort.SliceStable(users, func(i, j int) bool {
		a, okA := userToMap(users[i])[field].(string)
		b, okB := userToMap(users[j])[field].(string)
		if !okA || !okB {
			return false
		}
		if ascending {
			return a < b
		}
		return a > b
	})
}

func (s *server) handlePermissions(w http.ResponseWriter, r *http.Request, _ bool) {
	q := r.URL.Query()
	skip, _ := strconv.Atoi(q.Get("$skip"))
	top, _ := strconv.Atoi(q.Get("$top"))
	all := []string{"AccountSvc", "CompanySvc", "TaxSvc"}
	start, end := pageBounds(len(all), skip, top)
	resp := map[string]interface{}{"@recordsetCount": len(all), "value": all[start:end]}
	if link := nextLink(r, end, len(all)); link != "" {
		resp["@nextLink"] = link
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handleUserEntitlements(w http.ResponseWriter, _ *http.Request, _ bool) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"permissions": []string{"CompanyFetch", "CompanySave", "NexusFetch", "NexusSave"},
		"accessLevel": "SingleAccount",
		"companies":   []int{123, 456, 789},
	})
}

func (s *server) handlePing(w http.ResponseWriter, r *http.Request, authed bool) {
	if !authed {
		// The real API answers 200 and reports that the caller is not authenticated.
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"version":       "24.8.2",
			"authenticated": false,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":                "24.8.2",
		"authenticated":          true,
		"authenticationType":     "Basic",
		"authenticatedUserName":  s.cfg.Username,
		"authenticatedUserId":    12345,
		"authenticatedAccountId": 123456789,
		"crmid":                  "some-crm-id",
	})
}

// admin serves the /_mock/ control API.
func (s *server) admin(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AdminRemote && !isLoopback(r.RemoteAddr) {
		sendError(w, http.StatusForbidden, "Forbidden", "The admin API accepts loopback clients only", r.URL.Path, "Use --admin-allow-remote to change this")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/_mock")
	switch {
	case path == "/state" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.snapshot())
	case path == "/state" && r.Method == http.MethodPut:
		var st State
		if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
			sendError(w, http.StatusBadRequest, "BadRequest", "Invalid JSON body", "body", err.Error())
			return
		}
		s.store.replace(st)
		writeJSON(w, http.StatusOK, s.store.snapshot())
	case path == "/reset" && r.Method == http.MethodPost:
		s.store.reset()
		writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
	case path == "/users" && r.Method == http.MethodPost:
		var u User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			sendError(w, http.StatusBadRequest, "BadRequest", "Invalid JSON body", "body", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, s.store.addUser(u))
	case strings.HasPrefix(path, "/users/") && (r.Method == http.MethodPatch || r.Method == http.MethodDelete):
		id, err := strconv.Atoi(strings.TrimPrefix(path, "/users/"))
		if err != nil {
			sendError(w, http.StatusBadRequest, "BadRequest", "Invalid user id", "id", "The user id must be a number")
			return
		}
		s.adminUser(w, r, id)
	case path == "/faults" && r.Method == http.MethodPost:
		var f Fault
		if err := json.NewDecoder(r.Body).Decode(&f); err != nil || f.Path == "" {
			sendError(w, http.StatusBadRequest, "BadRequest", "A fault needs a path and valid JSON", "body", "Example: {\"path\":\"/api/v2/users\",\"status\":429,\"retryAfter\":2,\"count\":3}")
			return
		}
		s.store.addFault(f)
		writeJSON(w, http.StatusCreated, s.store.faultsCopy())
	case path == "/faults" && r.Method == http.MethodDelete:
		s.store.clearFaults()
		writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
	case path == "/stats" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.statsCopy())
	default:
		sendError(w, http.StatusNotFound, "NotFound", "Unknown admin route", r.URL.Path, "See docs/mock-server.md")
	}
}

func (s *server) adminUser(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method == http.MethodDelete {
		if !s.store.deleteUser(id) {
			sendError(w, http.StatusNotFound, "NotFound", "User not found", "id", "No user has this id")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var p UserPatch
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		sendError(w, http.StatusBadRequest, "BadRequest", "Invalid JSON body", "body", err.Error())
		return
	}
	u, ok := s.store.patchUser(id, p)
	if !ok {
		sendError(w, http.StatusNotFound, "NotFound", "User not found", "id", "No user has this id")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func isLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
