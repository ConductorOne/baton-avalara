package main

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Role is an Avalara security role definition.
type Role struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
}

// User is an Avalara user record.
type User struct {
	ID                   int    `json:"id"`
	AccountID            int    `json:"accountId"`
	CompanyID            int    `json:"companyId"`
	UserName             string `json:"userName"`
	FirstName            string `json:"firstName"`
	LastName             string `json:"lastName"`
	Email                string `json:"email"`
	PostalCode           string `json:"postalCode"`
	SecurityRoleID       string `json:"securityRoleId"`
	PasswordStatus       string `json:"passwordStatus"`
	IsActive             bool   `json:"isActive"`
	SuppressNewUserEmail bool   `json:"suppressNewUserEmail"`
	IsDeleted            bool   `json:"isDeleted"`
}

// State is the full mutable data set of the mock.
type State struct {
	Roles []Role `json:"roles"`
	Users []User `json:"users"`
}

// Fault is a rule that makes matching requests fail.
type Fault struct {
	Path       string `json:"path"`
	Method     string `json:"method,omitempty"`
	Status     int    `json:"status"`
	RetryAfter int    `json:"retryAfter,omitempty"`
	// Count is the number of requests to fail. Zero or less means no limit.
	Count     int `json:"count,omitempty"`
	remaining int
}

// store holds all mock data behind one mutex.
type store struct {
	mu      sync.Mutex
	initial State
	state   State
	faults  []*Fault
	stats   map[string]int
	nextID  int
}

func newStore(initial State) *store {
	s := &store{initial: cloneState(initial)}
	s.resetLocked()
	return s
}

func cloneState(in State) State {
	out := State{Roles: make([]Role, len(in.Roles)), Users: make([]User, len(in.Users))}
	copy(out.Roles, in.Roles)
	copy(out.Users, in.Users)
	return out
}

func (s *store) resetLocked() {
	s.state = cloneState(s.initial)
	s.faults = nil
	s.stats = map[string]int{}
	s.recomputeNextIDLocked()
}

func (s *store) recomputeNextIDLocked() {
	s.nextID = 100000
	for _, u := range s.state.Users {
		if u.ID >= s.nextID {
			s.nextID = u.ID + 1
		}
	}
}

func (s *store) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetLocked()
}

func (s *store) snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneState(s.state)
}

func (s *store) replace(st State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = cloneState(st)
	s.recomputeNextIDLocked()
}

func (s *store) addUser(u User) User {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u.ID == 0 {
		u.ID = s.nextID
		s.nextID++
	}
	if u.AccountID == 0 {
		u.AccountID = 123456789
	}
	if u.PasswordStatus == "" {
		u.PasswordStatus = "UserCanChange"
	}
	s.state.Users = append(s.state.Users, u)
	return u
}

// UserPatch holds the fields that PATCH can change.
type UserPatch struct {
	UserName       *string `json:"userName"`
	FirstName      *string `json:"firstName"`
	LastName       *string `json:"lastName"`
	Email          *string `json:"email"`
	SecurityRoleID *string `json:"securityRoleId"`
	IsActive       *bool   `json:"isActive"`
	IsDeleted      *bool   `json:"isDeleted"`
}

func (s *store) patchUser(id int, p UserPatch) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Users {
		u := &s.state.Users[i]
		if u.ID != id {
			continue
		}
		if p.UserName != nil {
			u.UserName = *p.UserName
		}
		if p.FirstName != nil {
			u.FirstName = *p.FirstName
		}
		if p.LastName != nil {
			u.LastName = *p.LastName
		}
		if p.Email != nil {
			u.Email = *p.Email
		}
		if p.SecurityRoleID != nil {
			u.SecurityRoleID = *p.SecurityRoleID
		}
		if p.IsActive != nil {
			u.IsActive = *p.IsActive
		}
		if p.IsDeleted != nil {
			u.IsDeleted = *p.IsDeleted
		}
		return *u, true
	}
	return User{}, false
}

func (s *store) deleteUser(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, u := range s.state.Users {
		if u.ID == id {
			s.state.Users = append(s.state.Users[:i], s.state.Users[i+1:]...)
			return true
		}
	}
	return false
}

func (s *store) addFault(f Fault) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f.remaining = f.Count
	s.faults = append(s.faults, &f)
}

func (s *store) clearFaults() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.faults = nil
}

// record counts a request and returns the first matching fault, if any.
func (s *store) record(method, path string) *Fault {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats[path]++
	for i, f := range s.faults {
		if f.Path != path || (f.Method != "" && f.Method != method) {
			continue
		}
		hit := *f
		if f.Count > 0 {
			f.remaining--
			if f.remaining <= 0 {
				s.faults = append(s.faults[:i], s.faults[i+1:]...)
			}
		}
		return &hit
	}
	return nil
}

func (s *store) statsCopy() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]int, len(s.stats))
	for k, v := range s.stats {
		out[k] = v
	}
	return out
}

func (s *store) faultsCopy() []Fault {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Fault, 0, len(s.faults))
	for _, f := range s.faults {
		out = append(out, *f)
	}
	return out
}

// defaultState returns the built-in 10 users and 22 roles.
func defaultState() State {
	descs := []string{
		"AccountAdmin", "AccountUser", "BatchServiceAdmin", "CompanyAdmin", "CompanyUser",
		"Compliance Root User", "ComplianceAdmin", "ComplianceUser", "CSPAdmin", "CSPTester",
		"ECMAccountUser", "ECMCompanyUser", "FirmAdmin", "FirmUser", "Registrar", "SiteAdmin",
		"SSTAdmin", "SystemAdmin", "TechnicalSupportAdmin", "TechnicalSupportUser",
		"TreasuryAdmin", "TreasuryUser",
	}
	st := State{}
	for i, d := range descs {
		st.Roles = append(st.Roles, Role{ID: i + 1, Description: d})
	}
	type def struct {
		id, company int
		name, role  string
	}
	defs := []def{
		{12345, 123456, "Bob", "AccountUser"},
		{67890, 123456, "Alice", "AccountAdmin"},
		{13579, 123457, "Charlie", "CompanyUser"},
		{24680, 123457, "Dana", "CompanyAdmin"},
		{35791, 123458, "Eve", "SystemAdmin"},
		{46802, 123458, "Frank", "TechnicalSupportAdmin"},
		{57913, 123459, "Grace", "ComplianceUser"},
		{68024, 123459, "Henry", "ComplianceAdmin"},
		{79135, 123460, "Isabel", "FirmUser"},
		{80246, 123460, "Jack", "FirmAdmin"},
	}
	for i, d := range defs {
		lower := string(append([]byte{d.name[0] + 32}, d.name[1:]...))
		st.Users = append(st.Users, User{
			ID: d.id, AccountID: 123456789, CompanyID: d.company,
			UserName: lower + "Example", FirstName: d.name, LastName: "Example",
			Email: lower + "@example.org", PostalCode: fmt.Sprintf("%d", 98110+i),
			SecurityRoleID: d.role, PasswordStatus: "UserCanChange", IsActive: true,
		})
	}
	return st
}

// generateUsers builds n users spread round-robin across the roles.
func generateUsers(n int, roles []Role) []User {
	users := make([]User, 0, n)
	for i := 0; i < n; i++ {
		role := "AccountUser"
		if len(roles) > 0 {
			role = roles[i%len(roles)].Description
			// Real Avalara returns role names without spaces.
			role = removeSpaces(role)
		}
		users = append(users, User{
			ID: 200000 + i, AccountID: 123456789, CompanyID: 123456 + i%5,
			UserName: fmt.Sprintf("user%d", i), FirstName: "User", LastName: fmt.Sprintf("Number%d", i),
			Email: fmt.Sprintf("user%d@example.org", i), PostalCode: "98110",
			SecurityRoleID: role, PasswordStatus: "UserCanChange", IsActive: true,
		})
	}
	return users
}

func removeSpaces(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

func decodeState(data []byte) (State, error) {
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, err
	}
	return st, nil
}
