package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

const (
	expectedProtocol = "https"
)

// Custom RoundTripper for testing.
type testRoundTripper struct {
	response *http.Response
	err      error
}

func (t *testRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return t.response, t.err
}

// Helper function to create a test client with custom transport.
func newTestClient(response *http.Response, err error) *AvalaraClient {
	transport := &testRoundTripper{response: response, err: err}
	httpClient := &http.Client{Transport: transport}
	baseHttpClient := uhttp.NewBaseHttpClient(httpClient)
	return NewAvalaraClient("", "", baseHttpClient)
}

func TestNewAvalaraClient(t *testing.T) {
	t.Run("Production environment", func(t *testing.T) {
		client := NewAvalaraClient("", "", nil)
		if client.baseURL != ProductionBaseURL {
			t.Errorf("Expected baseURL to be %s, got %s", ProductionBaseURL, client.baseURL)
		}
	})

	t.Run("Sandbox environment", func(t *testing.T) {
		client := NewAvalaraClient("sandbox", "", nil)
		if client.baseURL != SandboxBaseURL {
			t.Errorf("Expected baseURL to be %s, got %s", SandboxBaseURL, client.baseURL)
		}
	})

	t.Run("Client header", func(t *testing.T) {
		client := NewAvalaraClient("", "", nil)
		expectedPrefix := "baton-avalara; 1.0.0; Go SDK; API_VERSION"
		if !strings.HasPrefix(client.clientHeader, expectedPrefix) {
			t.Errorf("Expected clientHeader to start with %s, got %s", expectedPrefix, client.clientHeader)
		}
	})
}

func TestAvalaraClient_AddCredentials(t *testing.T) {
	// Create a new AvalaraClient.
	client := NewAvalaraClient("", "", nil)

	// Test credentials.
	username := "testuser"
	password := "testpass"

	// Add credentials.
	client.AddCredentials(username, password)

	// Check if credentials are set correctly.
	expectedCredentials := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	if client.credentials != expectedCredentials {
		t.Errorf("AddCredentials failed. Expected %s, got %s", expectedCredentials, client.credentials)
	}
}

// Tests that the client can fetch user roles based on the documented API below.
// https://developer.avalara.com/api-reference/avatax/rest/v2/methods/Definitions/ListSecurityRoles/
// curl 'https://sandbox-rest.avatax.com/api/v2/definitions/securityroles' \
// -H 'X-Avalara-Client: {X-Avalara-Client}' \
// -H 'Authorization: Basic <Base64-encoded credentials>' \
// -H 'Content-Type: application/json'
// Response
// HEADER
// Content-type: application/json
// BODY
//
//	{
//	 	"value": [
//	 	 	{
//	 	 	 	"id": 3,
//	 	 	 	"description": "AccountAdmin"
//	 	 	}
//	 	]
//	}
// Parameters
// Request Body
// $filter
// query
// Optional
// string
// A filter statement to identify specific records to retrieve. For more information on filtering, see Filtering in REST.
//
// $top
// query
// Optional
// integer
// If nonzero, return no more than this number of results. Used with $skip to provide pagination for large datasets.
// Unless otherwise specified, the maximum number of records that can be returned from an API call is 1,000 records.
//
// $skip
// query
// Optional
// integer
// If nonzero, skip this number of results before returning data. Used with $top to provide pagination for large datasets.
//
// $orderBy
// query
// Optional
// string
// A comma separated list of sort statements in the format (fieldname) [ASC|DESC], for example id ASC.
//
// X-Avalara-Client
// header
// Optional
// string
// Identifies the software you are using to call this API. For more information on the client headers, see Client Headers. .
//
// Parameters - Response Body
// @recordsetCount
// Optional
// integer
// value
// Optional
//
// array
// SecurityRoleModel
// @nextLink
// Optional

// string
// pageKey
// Optional.
func TestAvalaraClient_GetUserRoles(t *testing.T) {
	// Create a mock response.
	mockResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"@recordsetCount": 2,
			"value": [
				{
					"id": 1,
					"description": "SystemAdmin"
				},
				{
					"id": 2,
					"description": "AccountAdmin"
				}
			],
			"@nextLink": "https://rest.avatax.com/api/v2/definitions/securityroles?$skip=2&$top=2"
		}`)),
	}

	// Create a test client with the mock response.
	client := newTestClient(mockResponse, nil)

	// Call GetUserRoles.
	ctx := context.Background()
	options := &PaginationOptions{
		Top: 2,
	}
	result, nextOptions, err := client.GetUserRoles(ctx, options)

	// Check for errors.
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the result.
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check recordset count.
	if result.RecordsetCount != 2 {
		t.Errorf("Expected RecordsetCount to be 2, got %d", result.RecordsetCount)
	}

	// Check number of roles returned.
	if len(result.Value) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(result.Value))
	}

	// Check first role.
	expectedFirstRole := SecurityRoleModel{
		ID:          1,
		Description: "SystemAdmin",
	}
	if !reflect.DeepEqual(result.Value[0], expectedFirstRole) {
		t.Errorf("Unexpected first role: got %+v, want %+v", result.Value[0], expectedFirstRole)
	}

	// Check second role.
	expectedSecondRole := SecurityRoleModel{
		ID:          2,
		Description: "AccountAdmin",
	}
	if !reflect.DeepEqual(result.Value[1], expectedSecondRole) {
		t.Errorf("Unexpected second role: got %+v, want %+v", result.Value[1], expectedSecondRole)
	}

	// Check next options.
	if nextOptions == nil {
		t.Fatal("Expected non-nil nextOptions")
	}
	expectedNextLink := "https://rest.avatax.com/api/v2/definitions/securityroles?$skip=2&$top=2"
	if nextOptions.NextLink != expectedNextLink {
		t.Errorf("Expected NextLink to be %s, got %s", expectedNextLink, nextOptions.NextLink)
	}
}

func TestAvalaraClient_GetUserRoles_RequestDetails(t *testing.T) {
	// Create a custom RoundTripper to capture the request.
	var capturedRequest *http.Request
	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"value": []}`)),
		},
		err: nil,
	}
	mockTransport.roundTrip = func(req *http.Request) (*http.Response, error) {
		capturedRequest = req
		return mockTransport.response, mockTransport.err
	}

	// Create a test client with the mock transport.
	httpClient := &http.Client{Transport: mockTransport}
	baseHttpClient := uhttp.NewBaseHttpClient(httpClient)
	client := NewAvalaraClient("sandbox", "", baseHttpClient)
	client.AddCredentials("testuser", "testpass")

	// Call GetUserRoles with nextLink.
	ctx := context.Background()
	options := &PaginationOptions{
		NextLink: "https://SandboxBaseDomain/api/v2/definitions/securityroles?$top=10&$skip=20",
	}
	_, _, err := client.GetUserRoles(ctx, options)

	// Check for errors.
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the request details.
	if capturedRequest == nil {
		t.Fatal("No request was captured")
	}

	// Check URL components.
	expectedURL := "https://SandboxBaseDomain/api/v2/definitions/securityroles?$top=10&$skip=20"
	if capturedRequest.URL.String() != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, capturedRequest.URL.String())
	}

	// Check headers.
	expectedHeaders := map[string]string{
		"Authorization":    "Basic " + base64.StdEncoding.EncodeToString([]byte("testuser:testpass")),
		"X-Avalara-Client": client.clientHeader,
		"Accept":           "application/json",
		"Content-Type":     "application/json",
	}

	for key, expectedValue := range expectedHeaders {
		if value := capturedRequest.Header.Get(key); value != expectedValue {
			t.Errorf("Expected header %s to be %s, got %s", key, expectedValue, value)
		}
	}
}

func TestAvalaraClient_GetUserRoles_WithNextLink(t *testing.T) {
	mockResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"@recordsetCount": 2,
			"value": [
				{
					"id": 3,
					"description": "CompanyAdmin"
				},
				{
					"id": 4,
					"description": "CompanyUser"
				}
			],
			"@nextLink": "https://rest.avatax.com/api/v2/definitions/securityroles?$skip=4&$top=2"
		}`)),
	}

	client := newTestClient(mockResponse, nil)

	ctx := context.Background()
	options := &PaginationOptions{
		NextLink: "https://rest.avatax.com/api/v2/definitions/securityroles?$skip=2&$top=2",
	}
	result, nextOptions, err := client.GetUserRoles(ctx, options)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Value) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(result.Value))
	}

	if result.Value[0].ID != 3 || result.Value[0].Description != "CompanyAdmin" {
		t.Errorf("Unexpected first role: %+v", result.Value[0])
	}

	if nextOptions == nil {
		t.Fatal("Expected non-nil nextOptions")
	}
	expectedNextLink := "https://rest.avatax.com/api/v2/definitions/securityroles?$skip=4&$top=2"
	if nextOptions.NextLink != expectedNextLink {
		t.Errorf("Expected NextLink to be %s, got %s", expectedNextLink, nextOptions.NextLink)
	}
}

func TestAvalaraClient_GetAccounts(t *testing.T) {
	// TODO: Implement test
}

// Test that the client can fetch users based on the documented API below.
// https://developer.avalara.com/api-reference/avatax/rest/v2/methods/Users/QueryUsers/
// curl 'https://sandbox-rest.avatax.com/api/v2/users' \
// -H 'X-Avalara-Client: {X-Avalara-Client}' \
// -H 'Authorization: Basic <Base64-encoded credentials>' \
// -H 'Content-Type: application/json'
// Response
// HEADER
// Content-type: application/json
// BODY
//
//	{
//	 	"value": [
//	 	 	{
//	 	 	 	"id": 12345,
//	 	 	 	"accountId": 123456789,
//	 	 	 	"companyId": 123456,
//	 	 	 	"userName": "bobExample",
//	 	 	 	"firstName": "Bob",
//	 	 	 	"lastName": "Example",
//	 	 	 	"email": "bob@example.org",
//	 	 	 	"postalCode": "98110",
//	 	 	 	"securityRoleId": "AccountUser",
//	 	 	 	"passwordStatus": "UserCanChange",
//	 	 	 	"isActive": true,
//	 	 	 	"suppressNewUserEmail": false,
//	 	 	 	"isDeleted": false
//	 	 	}
//	 	]
//	}
// Parameters
// Request Body
// $include
// query
// Optional
// string
// Optional fetch commands.

// $filter
// query
// Optional
// string
// A filter statement to identify specific records to retrieve. For more information on filtering, see Filtering in REST.
// Not filterable: SuppressNewUserEmail

// $top
// query
// Optional
// integer
// If nonzero, return no more than this number of results. Used with $skip to provide pagination for large datasets.
// Unless otherwise specified, the maximum number of records that can be returned from an API call is 1,000 records.

// $skip
// query
// Optional
// integer
// If nonzero, skip this number of results before returning data. Used with $top to provide pagination for large datasets.

// $orderBy
// query
// Optional
// string
// A comma separated list of sort statements in the format (fieldname) [ASC|DESC], for example id ASC.

// X-Avalara-Client
// header
// Optional
// string
// Identifies the software you are using to call this API. For more information on the client header, see Client Headers.
// Parameters
// Response Body
// @recordsetCount
// Optional
// integer

// value
// Optional
// array
// UserModel

// @nextLink
// Optional
// string

// pageKey
// Optional
// string.
func TestAvalaraClient_GetUsers(t *testing.T) {
	// Create a mock response with a 200 status code.
	mockResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"@recordsetCount": 2,
			"value": [
				{
					"id": 12345,
					"accountId": 123456789,
					"companyId": 123456,
					"userName": "bobExample",
					"firstName": "Bob",
					"lastName": "Example",
					"email": "bob@example.org",
					"postalCode": "98110",
					"securityRoleId": "AccountUser",
					"passwordStatus": "UserCanChange",
					"isActive": true,
					"suppressNewUserEmail": false,
					"isDeleted": false
				},
				{
					"id": 67890,
					"accountId": 123456789,
					"companyId": 123456,
					"userName": "aliceExample",
					"firstName": "Alice",
					"lastName": "Example",
					"email": "alice@example.org",
					"postalCode": "98111",
					"securityRoleId": "AccountAdmin",
					"passwordStatus": "UserCanChange",
					"isActive": true,
					"suppressNewUserEmail": false,
					"isDeleted": false
				}
			],
			"@nextLink": "https://rest.avatax.com/api/v2/users?$skip=2&$top=2"
		}`)),
	}

	// Create a test client with the mock response.
	client := newTestClient(mockResponse, nil)

	// Call GetUsers
	ctx := context.Background()
	options := &PaginationOptions{
		Top:  2,
		Skip: 0,
	}
	result, updatedOptions, err := client.GetUsers(ctx, options)

	// Check for errors.
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the result.
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check recordset count.
	if result.RecordsetCount != 2 {
		t.Errorf("Expected RecordsetCount to be 2, got %d", result.RecordsetCount)
	}

	// Check number of users returned.
	if len(result.Value) != 2 {
		t.Errorf("Expected 2 users, got %d", len(result.Value))
	}

	// Check first user.
	expectedFirstUser := UserModel{
		ID:                   12345,
		AccountID:            123456789,
		CompanyID:            123456,
		UserName:             "bobExample",
		FirstName:            "Bob",
		LastName:             "Example",
		Email:                "bob@example.org",
		PostalCode:           "98110",
		SecurityRoleID:       "AccountUser",
		PasswordStatus:       "UserCanChange",
		IsActive:             true,
		SuppressNewUserEmail: false,
		IsDeleted:            false,
	}
	if !reflect.DeepEqual(result.Value[0], expectedFirstUser) {
		t.Errorf("Unexpected first user: got %+v, want %+v", result.Value[0], expectedFirstUser)
	}

	// Check second user.
	expectedSecondUser := UserModel{
		ID:                   67890,
		AccountID:            123456789,
		CompanyID:            123456,
		UserName:             "aliceExample",
		FirstName:            "Alice",
		LastName:             "Example",
		Email:                "alice@example.org",
		PostalCode:           "98111",
		SecurityRoleID:       "AccountAdmin",
		PasswordStatus:       "UserCanChange",
		IsActive:             true,
		SuppressNewUserEmail: false,
		IsDeleted:            false,
	}
	if !reflect.DeepEqual(result.Value[1], expectedSecondUser) {
		t.Errorf("Unexpected second user: got %+v, want %+v", result.Value[1], expectedSecondUser)
	}

	// Check next link.
	expectedNextLink := "https://rest.avatax.com/api/v2/users?$skip=2&$top=2"
	if updatedOptions.NextLink != expectedNextLink {
		t.Errorf("Expected NextLink to be %s, got %s", expectedNextLink, result.NextLink)
	}
}

// Test that the client can fetch permissions based on the documented API below.
// https://developer.avalara.com/api-reference/avatax/rest/v2/methods/Definitions/ListPermissions/
// curl 'https://sandbox-rest.avatax.com/api/v2/definitions/permissions' \
// -H 'X-Avalara-Client: {X-Avalara-Client}' \
// -H 'Authorization: Basic <Base64-encoded credentials>' \
// -H 'Content-Type: application/json'
// Response
// HEADER
// Content-type: application/json
// BODY
// {}
// Parameters
// Request Body
// $top
// query
// Optional
// integer
// If nonzero, return no more than this number of results. Used with $skip to provide pagination for large datasets.
// Unless otherwise specified, the maximum number of records that can be returned from an API call is 1,000 records.

// $skip
// query
// Optional
// integer
// If nonzero, skip this number of results before returning data. Used with $top to provide pagination for large datasets.

// X-Avalara-Client
// header
// Optional
// string
// Identifies the software you are using to call this API. For more information on the client header, see Client Headers.
// Parameters
// Response Body
// down arrow
// @recordsetCount
// Optional
// integer

// value
// Optional
// array

// @nextLink
// Optional
// string

// pageKey
// Optional
// string.
func TestAvalaraClient_GetPermissions(t *testing.T) {
	// Create a mock response.
	mockResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"@recordsetCount": 3,
			"value": [
				"AccountSvc",
				"CompanySvc",
				"TaxSvc"
			],
			"@nextLink": "https://rest.avatax.com/api/v2/definitions/permissions?$skip=3&$top=3"
		}`)),
	}

	// Create a test client with the mock response.
	client := newTestClient(mockResponse, nil)

	// Call GetPermissions.
	ctx := context.Background()
	options := &PaginationOptions{
		Top:  3,
		Skip: 0,
	}
	result, nextOptions, err := client.GetPermissions(ctx, options)

	// Check for errors.
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the result.
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check recordset count.
	if result.RecordsetCount != 3 {
		t.Errorf("Expected RecordsetCount to be 3, got %d", result.RecordsetCount)
	}

	// Check number of permissions returned
	if len(result.Value) != 3 {
		t.Errorf("Expected 3 permissions, got %d", len(result.Value))
	}

	// Check permissions.
	expectedPermissions := []string{"AccountSvc", "CompanySvc", "TaxSvc"}
	if !reflect.DeepEqual(result.Value, expectedPermissions) {
		t.Errorf("Unexpected permissions: got %v, want %v", result.Value, expectedPermissions)
	}

	// Check next link.
	expectedNextLink := "https://rest.avatax.com/api/v2/definitions/permissions?$skip=3&$top=3"
	if nextOptions.NextLink != expectedNextLink {
		t.Errorf("Expected NextLink to be %s, got %s", expectedNextLink, result.NextLink)
	}
}

func TestAvalaraClient_GetPermissions_RequestDetails(t *testing.T) {
	// Create a custom RoundTripper to capture the request.
	var capturedRequest *http.Request
	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"value": []}`)),
		},
		err: nil,
	}
	mockTransport.roundTrip = func(req *http.Request) (*http.Response, error) {
		capturedRequest = req
		return mockTransport.response, mockTransport.err
	}

	// Create a test client with the mock transport.
	httpClient := &http.Client{Transport: mockTransport}
	baseHttpClient := uhttp.NewBaseHttpClient(httpClient)
	client := NewAvalaraClient("sandbox", "", baseHttpClient)
	client.AddCredentials("testuser", "testpass")

	// Call GetPermissions.
	ctx := context.Background()
	options := &PaginationOptions{
		Top:  10,
		Skip: 5,
	}
	_, nextOptions, err := client.GetPermissions(ctx, options)

	// Check for errors.
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the request details.
	if capturedRequest == nil {
		t.Fatal("No request was captured")
	}

	// Check URL components.
	expectedScheme := expectedProtocol
	expectedHost := SandboxBaseDomain
	expectedPath := "/api/v2/definitions/permissions"
	expectedQuery := url.Values{
		"$top":  []string{"10"},
		"$skip": []string{"5"},
	}

	if capturedRequest.URL.Scheme != expectedScheme {
		t.Errorf("Expected scheme %s, got %s", expectedScheme, capturedRequest.URL.Scheme)
	}
	if capturedRequest.URL.Host != expectedHost {
		t.Errorf("Expected host %s, got %s", expectedHost, capturedRequest.URL.Host)
	}
	if capturedRequest.URL.Path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, capturedRequest.URL.Path)
	}
	if !reflect.DeepEqual(capturedRequest.URL.Query(), expectedQuery) {
		t.Errorf("Expected query %v, got %v", expectedQuery, capturedRequest.URL.Query())
	}

	// Check headers.
	expectedHeaders := map[string]string{
		"Authorization":    "Basic " + base64.StdEncoding.EncodeToString([]byte("testuser:testpass")),
		"X-Avalara-Client": client.clientHeader,
		"Accept":           "application/json",
		"Content-Type":     "application/json",
	}

	for key, expectedValue := range expectedHeaders {
		if value := capturedRequest.Header.Get(key); value != expectedValue {
			t.Errorf("Expected header %s to be %s, got %s", key, expectedValue, value)
		}
	}
	expectedNextLink := ""
	if nextOptions.NextLink != expectedNextLink {
		t.Errorf("Expected NextLink to be %s, got %s", expectedNextLink, nextOptions.NextLink)
	}
}

// Test that the client can fetch user entitlements based on the documented API below.
// https://developer.avalara.com/api-reference/avatax/rest/v2/methods/Users/GetUserEntitlements/
// curl -g 'https://sandbox-rest.avatax.com/api/v2/accounts/{accountId}/users/{id}/entitlements' \
// -H 'X-Avalara-Client: {X-Avalara-Client}' \
// -H 'Authorization: Basic <Base64-encoded credentials>' \
// -H 'Content-Type: application/json'
// Response
// HEADER
// Content-type: application/json
// BODY
//
//	{
//	 	"permissions": [
//	 	 	"CompanyFetch",
//	 	 	"CompanySave",
//	 	 	"NexusFetch",
//	 	 	"NexusSave"
//	 	],
//	 	"accessLevel": "SingleAccount",
//	 	"companies": [
//	 	 	123,
//	 	 	456,
//	 	 	789
//	 	]
//	}
// Parameters
// Request Body
// id
// path
// Required
// integer
// The ID of the user to retrieve.

// accountId
// path
// Required
// integer
// The accountID of the user you wish to get.

// X-Avalara-Client
// header
// Optional
// string
// Identifies the software you are using to call this API. For more information on the client header,
// see Client Headers .
// Parameters
// Response Body
// permissions
// Optional
// array
// List of API names and categories that this user is permitted to access.

// accessLevel
// Optional
// string
// What access privileges does the current user have to see companies?

// Enum:
// None,SingleCompany,SingleAccount,AllCompanies,FirmManagedAccounts
// companies
// Optional
// array
// The identities of all companies this user is permitted to access.
func TestAvalaraClient_GetUserEntitlements(t *testing.T) {
	// Create a mock response.
	mockResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"permissions": [
				"CompanyFetch",
				"CompanySave",
				"NexusFetch",
				"NexusSave"
			],
			"accessLevel": "SingleAccount",
			"companies": [
				123,
				456,
				789
			]
		}`)),
	}

	// Create a test client with the mock response.
	client := newTestClient(mockResponse, nil)

	// Call GetUserEntitlements
	ctx := context.Background()
	accountID := 12345
	userID := 67890
	result, err := client.GetUserEntitlements(ctx, accountID, userID)

	// Check for errors.
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the result.
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check permissions.
	expectedPermissions := []string{"CompanyFetch", "CompanySave", "NexusFetch", "NexusSave"}
	if !reflect.DeepEqual(result.Permissions, expectedPermissions) {
		t.Errorf("Unexpected permissions: got %v, want %v", result.Permissions, expectedPermissions)
	}

	// Check access level.
	expectedAccessLevel := "SingleAccount"
	if result.AccessLevel != expectedAccessLevel {
		t.Errorf("Unexpected access level: got %s, want %s", result.AccessLevel, expectedAccessLevel)
	}

	// Check companies.
	expectedCompanies := []int{123, 456, 789}
	if !reflect.DeepEqual(result.Companies, expectedCompanies) {
		t.Errorf("Unexpected companies: got %v, want %v", result.Companies, expectedCompanies)
	}
}

func TestAvalaraClient_GetUserEntitlements_RequestDetails(t *testing.T) {
	// Create a custom RoundTripper to capture the request.
	var capturedRequest *http.Request
	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		},
		err: nil,
	}
	mockTransport.roundTrip = func(req *http.Request) (*http.Response, error) {
		capturedRequest = req
		return mockTransport.response, mockTransport.err
	}

	// Create a test client with the mock transport.
	httpClient := &http.Client{Transport: mockTransport}
	baseHttpClient := uhttp.NewBaseHttpClient(httpClient)
	client := NewAvalaraClient("sandbox", "", baseHttpClient)
	client.AddCredentials("testuser", "testpass")

	// Call GetUserEntitlements.
	ctx := context.Background()
	accountID := 12345
	userID := 67890
	_, err := client.GetUserEntitlements(ctx, accountID, userID)

	// Check for errors.
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the request details.
	if capturedRequest == nil {
		t.Fatal("No request was captured")
	}

	// Check URL components.
	expectedScheme := expectedProtocol
	expectedHost := SandboxBaseDomain
	expectedPath := fmt.Sprintf("/api/v2/accounts/%d/users/%d/entitlements", accountID, userID)

	if capturedRequest.URL.Scheme != expectedScheme {
		t.Errorf("Expected scheme %s, got %s", expectedScheme, capturedRequest.URL.Scheme)
	}
	if capturedRequest.URL.Host != expectedHost {
		t.Errorf("Expected host %s, got %s", expectedHost, capturedRequest.URL.Host)
	}
	if capturedRequest.URL.Path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, capturedRequest.URL.Path)
	}

	// Check headers.
	expectedHeaders := map[string]string{
		"Authorization":    "Basic " + base64.StdEncoding.EncodeToString([]byte("testuser:testpass")),
		"X-Avalara-Client": client.clientHeader,
		"Accept":           "application/json",
		"Content-Type":     "application/json",
	}

	for key, expectedValue := range expectedHeaders {
		if value := capturedRequest.Header.Get(key); value != expectedValue {
			t.Errorf("Expected header %s to be %s, got %s", key, expectedValue, value)
		}
	}
}

func TestGetAvalaraClient(t *testing.T) {
	// Create a context with a logger
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	ctx = ctxzap.ToContext(ctx, logger)

	// Test cases.
	testCases := []struct {
		name        string
		environment string
		username    string
		password    string
	}{
		{
			name:        "Production environment",
			environment: "",
			username:    "testuser",
			password:    "testpass",
		},
		{
			name:        "Sandbox environment",
			environment: "sandbox",
			username:    "sandboxuser",
			password:    "sandboxpass",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := GetAvalaraClient(ctx, tc.environment, "", tc.username, tc.password)

			// Check for errors.
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			// Check if client is not nil.
			if client == nil {
				t.Fatal("Expected non-nil client")
			}

			// Check base URL.
			expectedBaseURL := ProductionBaseURL
			if tc.environment == "sandbox" {
				expectedBaseURL = SandboxBaseURL
			}
			if client.baseURL != expectedBaseURL {
				t.Errorf("Expected baseURL to be %s, got %s", expectedBaseURL, client.baseURL)
			}

			// Check credentials.
			expectedCredentials := base64.StdEncoding.EncodeToString([]byte(tc.username + ":" + tc.password))
			if client.credentials != expectedCredentials {
				t.Errorf("Expected credentials to be %s, got %s", expectedCredentials, client.credentials)
			}

			// Check client header.
			expectedPrefix := "baton-avalara; 1.0.0; Go SDK; API_VERSION"
			if !strings.HasPrefix(client.clientHeader, expectedPrefix) {
				t.Errorf("Expected clientHeader to start with %s, got %s", expectedPrefix, client.clientHeader)
			}

			// Check if httpClient is set.
			if client.httpClient == nil {
				t.Error("Expected httpClient to be set, got nil")
			}
		})
	}
}

type mockRoundTripper struct {
	response  *http.Response
	err       error
	roundTrip func(*http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

// Test that the client can ping the Avalara API based on the documented API below.
// curl 'https://sandbox-rest.avatax.com/api/v2/utilities/ping' \
// -H 'X-Avalara-Client: {X-Avalara-Client}' \
// -H 'Authorization: Basic <Base64-encoded credentials>' \
// -H 'Content-Type: application/json'
//
// Response
// HEADER
// Content-type: application/json
// BODY
//
//	{
//	 	"version": "1.0.0.0",
//	 	"authenticated": true,
//	 	"authenticationType": "UsernamePassword",
//	 	"authenticatedUserName": "TestUser",
//	 	"authenticatedUserId": 98765,
//	 	"authenticatedAccountId": 123456789,
//	 	"authenticatedCompanyId": 123456789,
//	 	"crmid": "1111"
//	}
//
// Parameters
// Request Body
// X-Avalara-Client
// header
// Optional
// string
// Identifies the software you are using to call this API. For more information on the client header,
// see Client Headers.
//
// Parameters
// Response Body
// version
// Optional
// string
// Version number
//
// authenticated
// Optional
// boolean
// Returns true if you provided authentication for this API call; false if you did not.
//
// authenticationType
// Optional
// string
// Returns the type of authentication you provided, if authenticated.
//
// Enum:
// None,UsernamePassword,AccountIdLicenseKey,OpenIdBearerToken
//
// authenticatedUserName
// Optional
// string
// The username of the currently authenticated user, if any.
//
// authenticatedUserId
// Optional
// integer
// The ID number of the currently authenticated user, if any.
//
// authenticatedAccountId
// Optional
// integer
// The ID number of the currently authenticated user's account, if any.
//
// authenticatedCompanyId
// Optional
// integer
// The ID number of the currently authenticated user's company, if any.
//
// crmid
// Optional
// string
// The connected Salesforce account.
func TestAvalaraClient_Ping(t *testing.T) {
	// Create a mock response.
	mockResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"version": "1.0.0.0",
			"authenticated": true,
			"authenticationType": "UsernamePassword",
			"authenticatedUserName": "TestUser",
			"authenticatedUserId": 98765,
			"authenticatedAccountId": 123456789,
			"authenticatedCompanyId": 123456789,
			"crmid": "1111"
		}`)),
	}

	// Create a test client with the mock response.
	client := newTestClient(mockResponse, nil)

	// Call Ping.
	ctx := context.Background()
	result, err := client.Ping(ctx)

	// Check for errors.
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the result.
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check individual fields.
	expectedResult := &PingResponse{
		Version:                "1.0.0.0",
		Authenticated:          true,
		AuthenticationType:     "UsernamePassword",
		AuthenticatedUserName:  "TestUser",
		AuthenticatedUserID:    98765,
		AuthenticatedAccountID: 123456789,
		AuthenticatedCompanyID: 123456789,
		CRMID:                  "1111",
	}

	if !reflect.DeepEqual(result, expectedResult) {
		t.Errorf("Unexpected result: got %+v, want %+v", result, expectedResult)
	}
}

func TestAvalaraClient_Ping_RequestDetails(t *testing.T) {
	// Create a custom RoundTripper to capture the request.
	var capturedRequest *http.Request
	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		},
		err: nil,
	}
	mockTransport.roundTrip = func(req *http.Request) (*http.Response, error) {
		capturedRequest = req
		return mockTransport.response, mockTransport.err
	}

	// Create a test client with the mock transport.
	httpClient := &http.Client{Transport: mockTransport}
	baseHttpClient := uhttp.NewBaseHttpClient(httpClient)
	client := NewAvalaraClient("sandbox", "", baseHttpClient)
	client.AddCredentials("testuser", "testpass")

	// Call Ping.
	ctx := context.Background()
	_, err := client.Ping(ctx)

	// Check for errors.
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the request details.
	if capturedRequest == nil {
		t.Fatal("No request was captured")
	}

	// Check URL components.
	expectedScheme := expectedProtocol
	expectedHost := SandboxBaseDomain
	expectedPath := "/api/v2/utilities/ping"

	if capturedRequest.URL.Scheme != expectedScheme {
		t.Errorf("Expected scheme %s, got %s", expectedScheme, capturedRequest.URL.Scheme)
	}
	if capturedRequest.URL.Host != expectedHost {
		t.Errorf("Expected host %s, got %s", expectedHost, capturedRequest.URL.Host)
	}
	if capturedRequest.URL.Path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, capturedRequest.URL.Path)
	}

	// Check headers.
	expectedHeaders := map[string]string{
		"Authorization":    "Basic " + base64.StdEncoding.EncodeToString([]byte("testuser:testpass")),
		"X-Avalara-Client": client.clientHeader,
		"Accept":           "application/json",
		"Content-Type":     "application/json",
	}

	for key, expectedValue := range expectedHeaders {
		if value := capturedRequest.Header.Get(key); value != expectedValue {
			t.Errorf("Expected header %s to be %s, got %s", key, expectedValue, value)
		}
	}
}
