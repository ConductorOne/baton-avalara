package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Add this function at the beginning of the file.
func getBaseURL() string {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	} else if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	return baseURL
}

func main() {
	baseURL := getBaseURL()
	log.Printf("Starting test server with base URL: %s\n", baseURL)

	http.HandleFunc("/api/v2/definitions/securityroles", authMiddleware(handleSecurityRoles))
	http.HandleFunc("/api/v2/users", authMiddleware(handleUsers))
	http.HandleFunc("/api/v2/definitions/permissions", authMiddleware(handlePermissions))
	http.HandleFunc("/api/v2/accounts/", authMiddleware(handleUserEntitlements))
	http.HandleFunc("/api/v2/utilities/ping", authMiddleware(handlePing))

	port := 8080
	log.Printf("Starting test server on port %d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), nil)) //nolint:gosec,gocritic // This is a test server.
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Generate a correlation ID.
		correlationID := uuid.New().String()

		// Set X-Correlation-Id header in the response.
		w.Header().Set("X-Correlation-Id", correlationID)

		// Log the incoming request.
		logRequest(r.URL.Path, r)

		// Check for Authorization header.
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Printf("Authentication failed: Missing Authorization header")
			sendAuthError(w, "Missing Authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Basic" {
			log.Printf("Authentication failed: Invalid Authorization header format")
			sendAuthError(w, "Invalid Authorization header format")
			return
		}

		payload, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			log.Printf("Authentication failed: Invalid base64 encoding in Authorization header")
			sendAuthError(w, "Invalid base64 encoding in Authorization header")
			return
		}

		// Log the decoded credentials for debugging.
		log.Printf("Decoded credentials: %s", string(payload)) //nolint:gosec // G706: test-server only, debug logging of test credentials

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || pair[0] != "testuser" || pair[1] != "testpass" {
			log.Printf("Authentication failed: Invalid credentials")
			sendAuthError(w, "Invalid credentials")
			return
		}

		// Check for X-Avalara-Client header.
		clientHeader := r.Header.Get("X-Avalara-Client")
		if clientHeader == "" {
			log.Printf("Authentication failed: Missing X-Avalara-Client header")
			sendAuthError(w, "Missing X-Avalara-Client header")
			return
		}

		log.Printf("Authentication successful for user: %s", pair[0]) //nolint:gosec // G706: test-server only, debug logging
		next.ServeHTTP(w, r)
	}
}

func sendAuthError(w http.ResponseWriter, message string) {
	log.Printf("Authentication error: %s", message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "AuthenticationException",
			"message": nil,
		},
	})
	if err != nil {
		log.Printf("Error sending authentication error: %v", err)
	}
}

func handleSecurityRoles(w http.ResponseWriter, r *http.Request) {
	logRequest("/api/v2/definitions/securityroles", r)

	// Parse query parameters.
	queryParams := r.URL.Query()
	filter := queryParams.Get("$filter")
	skip, _ := strconv.Atoi(queryParams.Get("$skip"))

	// Define all security roles.
	allRoles := []map[string]interface{}{
		{"id": 1, "description": "AccountAdmin"},
		{"id": 2, "description": "AccountUser"},
		{"id": 3, "description": "BatchServiceAdmin"},
		{"id": 4, "description": "CompanyAdmin"},
		{"id": 5, "description": "CompanyUser"},
		{"id": 6, "description": "Compliance Root User"},
		{"id": 7, "description": "ComplianceAdmin"},
		{"id": 8, "description": "ComplianceUser"},
		{"id": 9, "description": "CSPAdmin"},
		{"id": 10, "description": "CSPTester"},
		{"id": 11, "description": "ECMAccountUser"},
		{"id": 12, "description": "ECMCompanyUser"},
		{"id": 13, "description": "FirmAdmin"},
		{"id": 14, "description": "FirmUser"},
		{"id": 15, "description": "Registrar"},
		{"id": 16, "description": "SiteAdmin"},
		{"id": 17, "description": "SSTAdmin"},
		{"id": 18, "description": "SystemAdmin"},
		{"id": 19, "description": "TechnicalSupportAdmin"},
		{"id": 20, "description": "TechnicalSupportUser"},
		{"id": 21, "description": "TreasuryAdmin"},
		{"id": 22, "description": "TreasuryUser"},
	}

	// Apply filtering if a filter is provided.
	filteredRoles := []map[string]interface{}{}
	if filter != "" {
		for _, role := range allRoles {
			if applyRoleFilter(role, filter) {
				filteredRoles = append(filteredRoles, role)
			}
		}
	} else {
		filteredRoles = allRoles
	}

	response := map[string]interface{}{
		"@recordsetCount": len(filteredRoles),
		"value":           filteredRoles[skip:],
	}

	// Update nextLink only for the first request.
	if skip == 0 && len(filteredRoles) > 0 {
		response["@nextLink"] = fmt.Sprintf(
			"%s/api/v2/definitions/securityroles?$skip=%d&$top=%d&$filter=%s",
			getBaseURL(), len(filteredRoles), len(filteredRoles), filter)
	}

	sendJSONResponse(w, response)
}

// Helper function to apply filter for security roles.
func applyRoleFilter(role map[string]interface{}, filter string) bool {
	// Implement basic filtering logic here.
	// For example, let's support filtering by id or description.
	if strings.Contains(filter, "id eq") {
		idStr := strings.TrimSpace(strings.TrimPrefix(filter, "id eq"))
		id, err := strconv.Atoi(idStr)
		if err == nil {
			return role["id"] == id
		}
	} else if strings.Contains(filter, "description eq") {
		description := strings.Trim(strings.TrimSpace(strings.TrimPrefix(filter, "description eq")), "'")
		return strings.EqualFold(role["description"].(string), description)
	}
	// Add more filter conditions as needed.

	// If no filter matches, return true (include the role).
	return true
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	logRequest("/api/v2/users", r)

	// Parse query parameters.
	queryParams := r.URL.Query()
	filter := queryParams.Get("$filter")
	include := queryParams.Get("$include")
	top, _ := strconv.Atoi(queryParams.Get("$top"))
	skip, _ := strconv.Atoi(queryParams.Get("$skip"))
	orderBy := queryParams.Get("$orderBy")

	// Define all users.
	allUsers := []map[string]interface{}{
		{
			"id": 12345, "accountId": 123456789, "companyId": 123456,
			"userName": "bobExample", "firstName": "Bob", "lastName": "Example",
			"email": "bob@example.org", "postalCode": "98110",
			"securityRoleId": "AccountUser", "passwordStatus": "UserCanChange",
			"isActive": true, "suppressNewUserEmail": false, "isDeleted": false,
		},
		{
			"id": 67890, "accountId": 123456789, "companyId": 123456,
			"userName": "aliceExample", "firstName": "Alice", "lastName": "Example",
			"email": "alice@example.org", "postalCode": "98111",
			"securityRoleId": "AccountAdmin", "passwordStatus": "UserCanChange",
			"isActive": true, "suppressNewUserEmail": false, "isDeleted": false,
		},
		{
			"id": 13579, "accountId": 123456789, "companyId": 123457,
			"userName": "charlieExample", "firstName": "Charlie", "lastName": "Example",
			"email": "charlie@example.org", "postalCode": "98112",
			"securityRoleId": "CompanyUser", "passwordStatus": "UserCanChange",
			"isActive": true, "suppressNewUserEmail": false, "isDeleted": false,
		},
		{
			"id": 24680, "accountId": 123456789, "companyId": 123457,
			"userName": "danaExample", "firstName": "Dana", "lastName": "Example",
			"email": "dana@example.org", "postalCode": "98113",
			"securityRoleId": "CompanyAdmin", "passwordStatus": "UserCanChange",
			"isActive": true, "suppressNewUserEmail": false, "isDeleted": false,
		},
		{
			"id": 35791, "accountId": 123456789, "companyId": 123458,
			"userName": "eveExample", "firstName": "Eve", "lastName": "Example",
			"email": "eve@example.org", "postalCode": "98114",
			"securityRoleId": "SystemAdmin", "passwordStatus": "UserCanChange",
			"isActive": true, "suppressNewUserEmail": false, "isDeleted": false,
		},
		{
			"id": 46802, "accountId": 123456789, "companyId": 123458,
			"userName": "frankExample", "firstName": "Frank", "lastName": "Example",
			"email": "frank@example.org", "postalCode": "98115",
			"securityRoleId": "TechnicalSupportAdmin", "passwordStatus": "UserCanChange",
			"isActive": true, "suppressNewUserEmail": false, "isDeleted": false,
		},
		{
			"id": 57913, "accountId": 123456789, "companyId": 123459,
			"userName": "graceExample", "firstName": "Grace", "lastName": "Example",
			"email": "grace@example.org", "postalCode": "98116",
			"securityRoleId": "ComplianceUser", "passwordStatus": "UserCanChange",
			"isActive": true, "suppressNewUserEmail": false, "isDeleted": false,
		},
		{
			"id": 68024, "accountId": 123456789, "companyId": 123459,
			"userName": "henryExample", "firstName": "Henry", "lastName": "Example",
			"email": "henry@example.org", "postalCode": "98117",
			"securityRoleId": "ComplianceAdmin", "passwordStatus": "UserCanChange",
			"isActive": true, "suppressNewUserEmail": false, "isDeleted": false,
		},
		{
			"id": 79135, "accountId": 123456789, "companyId": 123460,
			"userName": "isabelExample", "firstName": "Isabel", "lastName": "Example",
			"email": "isabel@example.org", "postalCode": "98118",
			"securityRoleId": "FirmUser", "passwordStatus": "UserCanChange",
			"isActive": true, "suppressNewUserEmail": false, "isDeleted": false,
		},
		{
			"id": 80246, "accountId": 123456789, "companyId": 123460,
			"userName": "jackExample", "firstName": "Jack", "lastName": "Example",
			"email": "jack@example.org", "postalCode": "98119",
			"securityRoleId": "FirmAdmin", "passwordStatus": "UserCanChange",
			"isActive": true, "suppressNewUserEmail": false, "isDeleted": false,
		},
	}

	// Apply filtering if a filter is provided.
	filteredUsers := []map[string]interface{}{}
	if filter != "" {
		for _, user := range allUsers {
			if applyFilter(user, filter) {
				filteredUsers = append(filteredUsers, user)
			}
		}
	} else {
		filteredUsers = allUsers
	}

	// Apply ordering if orderBy is provided.
	if orderBy != "" {
		sortUsers(filteredUsers, orderBy)
	}

	// Apply pagination.
	start := skip
	end := len(filteredUsers)
	if top > 0 {
		end = minCheck(start+top, end)
	}
	paginatedUsers := filteredUsers[start:end]

	// Handle $include parameter.
	if include == "FetchResult" {
		for i, user := range paginatedUsers {
			paginatedUsers[i] = addFetchResult(user)
		}
	}

	// Initialize the response.
	response := map[string]interface{}{
		"@recordsetCount": len(filteredUsers),
		"value":           paginatedUsers,
	}

	// Update nextLink if necessary.
	if end < len(filteredUsers) {
		nextSkip := skip + len(paginatedUsers)
		response["@nextLink"] = fmt.Sprintf(
			"%s/api/v2/users?$skip=%d&$top=%d&$filter=%s&$orderBy=%s&$include=%s",
			getBaseURL(), nextSkip, top, filter, orderBy, include)
	}

	sendJSONResponse(w, response)
}

// Helper function to apply filter.
func applyFilter(user map[string]interface{}, filter string) bool {
	if strings.Contains(filter, "lastName startsWith") {
		prefix := strings.Trim(strings.TrimSpace(strings.TrimPrefix(filter, "lastName startsWith")), "\"")
		lastName, ok := user["lastName"].(string)
		return ok && strings.HasPrefix(strings.ToLower(lastName), strings.ToLower(prefix))
	}
	// Add more filter conditions as needed.
	return true
}

func sortUsers(users []map[string]interface{}, orderBy string) {
	sort.Slice(users, func(i, j int) bool {
		parts := strings.Split(orderBy, " ")
		field := parts[0]
		ascending := len(parts) == 1 || strings.ToUpper(parts[1]) != "DESC"

		valI, okI := users[i][field].(string)
		valJ, okJ := users[j][field].(string)

		if !okI || !okJ {
			return false
		}

		if ascending {
			return valI < valJ
		}
		return valI > valJ
	})
}

func minCheck(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func handlePermissions(w http.ResponseWriter, r *http.Request) {
	logRequest("/api/v2/definitions/permissions", r)
	response := map[string]interface{}{
		"@recordsetCount": 3,
		"value": []string{
			"AccountSvc",
			"CompanySvc",
			"TaxSvc",
		},
		"@nextLink": fmt.Sprintf("%s/api/v2/definitions/permissions?$skip=3&$top=3", getBaseURL()),
	}
	sendJSONResponse(w, response)
}

func handleUserEntitlements(w http.ResponseWriter, r *http.Request) {
	logRequest("/api/v2/accounts/", r)
	response := map[string]interface{}{
		"permissions": []string{
			"CompanyFetch",
			"CompanySave",
			"NexusFetch",
			"NexusSave",
		},
		"accessLevel": "SingleAccount",
		"companies":   []int{123, 456, 789},
	}
	sendJSONResponse(w, response)
}

func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	if w.Header().Get("X-Correlation-Id") == "" {
		w.Header().Set("X-Correlation-Id", uuid.New().String())
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Log the response.
	responseJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
	} else {
		log.Printf("Response sent:\n%s", string(responseJSON))
	}

	err = json.NewEncoder(w).Encode(data)
	if err != nil {
		log.Printf("Error sending response: %v", err)
	}
}

func logRequest(endpoint string, r *http.Request) {
	log.Printf("Endpoint called: %s\n", endpoint)         //nolint:gosec // G706: test-server only, debug logging
	log.Printf("  Method: %s\n", r.Method)                //nolint:gosec // G706: test-server only, debug logging
	log.Printf("  Query parameters: %s\n", r.URL.RawQuery) //nolint:gosec // G706: test-server only, debug logging

	// Log headers.
	log.Println("  Headers:")
	for name, values := range r.Header {
		for _, value := range values {
			log.Printf("    %s: %s\n", name, value)
		}
	}

	// Log form parameters.
	err := r.ParseForm()
	if err == nil {
		log.Println("  Form parameters:")
		for key, values := range r.Form {
			for _, value := range values {
				log.Printf("    %s: %s\n", key, value)
			}
		}
	}

	// Log request body if it's a POST or PUT request.
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			log.Printf("  Request body:\n%s\n", string(bodyBytes))
			// Restore the body to be read again.
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}
}

// Add new function to handle errors.
func sendErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message, target string, details []string) {
	if w.Header().Get("X-Correlation-Id") == "" {
		w.Header().Set("X-Correlation-Id", uuid.New().String())
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	errorDetails := make([]map[string]string, len(details))
	for i, detail := range details {
		errorDetails[i] = map[string]string{"message": detail}
	}
	err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    errorCode,
			"message": message,
			"target":  target,
			"details": errorDetails,
		},
	})
	if err != nil {
		log.Printf("Error sending error response: %v", err)
	}
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	logRequest("/api/v2/utilities/ping", r)

	// Check for X-Avalara-Client header.
	clientHeader := r.Header.Get("X-Avalara-Client")
	if clientHeader == "" {
		sendErrorResponse(w, http.StatusBadRequest, "HeaderValidationException", "Missing X-Avalara-Client header",
			"X-Avalara-Client", []string{"The X-Avalara-Client header is required for all API calls"})
		return
	}

	// Simulate successful ping response.
	response := map[string]interface{}{
		"version":                "24.8.2",
		"authenticated":          true,
		"authenticationType":     "Basic",
		"authenticatedUserName":  "testuser",
		"authenticatedUserId":    12345,
		"authenticatedAccountId": 123456789,
		"crmid":                  "some-crm-id",
	}
	sendJSONResponse(w, response)
}

// Helper function to add FetchResult to a user.
func addFetchResult(user map[string]interface{}) map[string]interface{} {
	fetchResult := map[string]interface{}{
		"@recordsetCount": 1,
		"value": []map[string]interface{}{
			{
				"id":        user["id"],
				"userName":  user["userName"],
				"firstName": user["firstName"],
				"lastName":  user["lastName"],
				"email":     user["email"],
				// Add other fields as needed.
			},
		},
	}
	user["FetchResult"] = fetchResult
	return user
}
