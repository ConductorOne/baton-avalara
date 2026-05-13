package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

const (
	SandboxBaseDomain = "sandbox-rest.avatax.com"
	ProductionBaseURL = "https://rest.avatax.com"
	SandboxBaseURL    = "https://sandbox-rest.avatax.com"
	TestBaseURL       = "http://localhost:8080"
)

// AvalaraClient represents the Avalara API client.
type AvalaraClient struct {
	baseURL      string
	httpClient   *uhttp.BaseHttpClient
	credentials  string
	clientHeader string
}

// PaginationOptions represents the pagination parameters.
type PaginationOptions struct {
	Top      int
	Skip     int
	OrderBy  string
	Filter   string
	NextLink string
}

// PaginatedResponse is a generic interface for paginated responses.
type PaginatedResponse interface {
	GetNextLink() string
}

// NewAvalaraClient creates a new instance of AvalaraClient.
func NewAvalaraClient(environment, baseURLOverride string, httpClient *uhttp.BaseHttpClient) *AvalaraClient {
	var baseURL string
	switch {
	case baseURLOverride != "":
		baseURL = baseURLOverride
	case strings.HasPrefix(strings.ToLower(environment), "http"):
		baseURL = environment
	default:
		switch environment {
		case "sandbox":
			baseURL = SandboxBaseURL
		case "test":
			baseURL = TestBaseURL
		default:
			baseURL = ProductionBaseURL
		}
	}

	appName := "baton-avalara"
	appVersion := "1.0.0"

	clientID := fmt.Sprintf("%s; %s; Go SDK; API_VERSION", appName, appVersion)

	return &AvalaraClient{
		baseURL:      baseURL,
		httpClient:   httpClient,
		clientHeader: clientID,
	}
}

// AddCredentials configures the client with username and password.
func (c *AvalaraClient) AddCredentials(username, password string) {
	c.credentials = base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// AvalaraError represents an error returned by the Avalara API.
type AvalaraError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Target  string `json:"target"`
	Details string `json:"details"`
}

func (e *AvalaraError) Error() string {
	return fmt.Sprintf("AvalaraError: %s (Code: %s, Target: %s, Details: %s)", e.Message, e.Code, e.Target, e.Details)
}

// AvalaraErrorResponse represents the structure of an error response.
type AvalaraErrorResponse struct {
	Error AvalaraError `json:"error"`
}

func (c *AvalaraClient) get(ctx context.Context, endpoint string, options *PaginationOptions, result interface{}) error {
	var u *url.URL
	var err error

	if options != nil && options.NextLink != "" {
		u, err = url.Parse(options.NextLink)
	} else {
		u, err = url.Parse(c.baseURL + endpoint)
	}
	if err != nil {
		return fmt.Errorf("error parsing URL: %w", err)
	}

	if options != nil && options.NextLink == "" {
		query := u.Query()
		if options.Top > 0 {
			query.Set("$top", strconv.Itoa(options.Top))
		}
		if options.Skip > 0 {
			query.Set("$skip", strconv.Itoa(options.Skip))
		}
		if options.OrderBy != "" {
			query.Set("$orderby", options.OrderBy)
		}
		if options.Filter != "" {
			query.Set("$filter", options.Filter)
		}
		u.RawQuery = query.Encode()
	}

	req, err := c.httpClient.NewRequest(ctx, "GET", u)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	// Set headers manually.
	req.Header.Set("Authorization", "Basic "+c.credentials)
	req.Header.Set("X-Avalara-Client", c.clientHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errorResp AvalaraErrorResponse
		if err := json.Unmarshal(bodyBytes, &errorResp); err == nil && errorResp.Error.Code != "" {
			return &errorResp.Error
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return &AvalaraError{
			Code:    "FormatException",
			Message: "The server returned the response in an unexpected format",
			Details: err.Error(),
		}
	}

	return nil
}

// Helper function to update PaginationOptions.
func updatePaginationOptions(options *PaginationOptions, response PaginatedResponse) *PaginationOptions {
	if options == nil {
		options = &PaginationOptions{}
	}
	options.NextLink = response.GetNextLink()
	return options
}

// GetUserRoles retrieves the security roles for the authenticated user with pagination.
func (c *AvalaraClient) GetUserRoles(ctx context.Context, options *PaginationOptions) (*SecurityRoleResponse, *PaginationOptions, error) {
	var result SecurityRoleResponse
	err := c.get(ctx, "/api/v2/definitions/securityroles", options, &result)
	if err != nil {
		return nil, options, err
	}
	return &result, updatePaginationOptions(options, &result), nil
}

// GetAccounts retrieves the accounts associated with the authenticated user with pagination.
func (c *AvalaraClient) GetAccounts(ctx context.Context, options *PaginationOptions) (*AccountResponse, *PaginationOptions, error) {
	var result AccountResponse
	err := c.get(ctx, "/api/v2/accounts", options, &result)
	if err != nil {
		return nil, options, err
	}
	return &result, updatePaginationOptions(options, &result), nil
}

// GetUsers retrieves the users associated with the authenticated user with pagination.
func (c *AvalaraClient) GetUsers(ctx context.Context, options *PaginationOptions) (*UserResponse, *PaginationOptions, error) {
	var result UserResponse
	err := c.get(ctx, "/api/v2/users", options, &result)
	if err != nil {
		return nil, options, err
	}
	return &result, updatePaginationOptions(options, &result), nil
}

// GetPermissions retrieves the list of permissions with pagination.
func (c *AvalaraClient) GetPermissions(ctx context.Context, options *PaginationOptions) (*PermissionResponse, *PaginationOptions, error) {
	var result PermissionResponse
	err := c.get(ctx, "/api/v2/definitions/permissions", options, &result)
	if err != nil {
		return nil, options, err
	}
	return &result, updatePaginationOptions(options, &result), nil
}

// GetUserEntitlements retrieves all entitlements for a single user.
func (c *AvalaraClient) GetUserEntitlements(ctx context.Context, accountID, userID int) (*EntitlementResponse, error) {
	endpoint := fmt.Sprintf("/api/v2/accounts/%d/users/%d/entitlements", accountID, userID)
	var result EntitlementResponse
	err := c.get(ctx, endpoint, nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// AccountModel represents the structure of an account in the API response.
type AccountModel struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	EffectiveDate   string `json:"effectiveDate"`
	AccountStatusID string `json:"accountStatusId"`
	AccountTypeID   string `json:"accountTypeId"`
	IsSamlEnabled   bool   `json:"isSamlEnabled"`
	IsDeleted       bool   `json:"isDeleted"`
}

// AccountResponse represents the structure of the API response for account queries.
type AccountResponse struct {
	RecordsetCount int            `json:"@recordsetCount,omitempty"`
	Value          []AccountModel `json:"value"`
	NextLink       string         `json:"@nextLink,omitempty"`
	PageKey        string         `json:"pageKey,omitempty"`
}

// Implement GetNextLink() for each response type.
func (r *AccountResponse) GetNextLink() string {
	return r.NextLink
}

// UserModel represents the structure of a user in the API response.
type UserModel struct {
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

// UserResponse represents the structure of the API response for user queries.
type UserResponse struct {
	RecordsetCount int         `json:"@recordsetCount,omitempty"`
	Value          []UserModel `json:"value"`
	NextLink       string      `json:"@nextLink,omitempty"`
	PageKey        string      `json:"pageKey,omitempty"`
}

// Implement GetNextLink() for each response type.
func (r *UserResponse) GetNextLink() string {
	return r.NextLink
}

// SecurityRoleModel represents the structure of a security role in the API response.
type SecurityRoleModel struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
}

// SecurityRoleResponse represents the structure of the API response for security role queries.
type SecurityRoleResponse struct {
	RecordsetCount int                 `json:"@recordsetCount,omitempty"`
	Value          []SecurityRoleModel `json:"value"`
	NextLink       string              `json:"@nextLink,omitempty"`
	PageKey        string              `json:"pageKey,omitempty"`
}

// Implement GetNextLink() for each response type.
func (r *SecurityRoleResponse) GetNextLink() string {
	return r.NextLink
}

// PermissionResponse represents the structure of the API response for permission queries.
type PermissionResponse struct {
	RecordsetCount int      `json:"@recordsetCount,omitempty"`
	Value          []string `json:"value"`
	NextLink       string   `json:"@nextLink,omitempty"`
	PageKey        string   `json:"pageKey,omitempty"`
}

// Implement GetNextLink() for each response type.
func (r *PermissionResponse) GetNextLink() string {
	return r.NextLink
}

// EntitlementResponse represents the structure of the API response for user entitlements.
type EntitlementResponse struct {
	Permissions []string `json:"permissions"`
	AccessLevel string   `json:"accessLevel"`
	Companies   []int    `json:"companies"`
}

// PingResponse represents the response from the Avalara Ping API.
type PingResponse struct {
	Version                string `json:"version"`
	Authenticated          bool   `json:"authenticated"`
	AuthenticationType     string `json:"authenticationType"`
	AuthenticatedUserName  string `json:"authenticatedUserName"`
	AuthenticatedUserID    int    `json:"authenticatedUserId"`
	AuthenticatedAccountID int    `json:"authenticatedAccountId"`
	AuthenticatedCompanyID int    `json:"authenticatedCompanyId"`
	CRMID                  string `json:"crmid"`
}

// Ping sends a request to the Avalara Ping API to check the connection and authentication status.
func (c *AvalaraClient) Ping(ctx context.Context) (*PingResponse, error) {
	var result PingResponse
	err := c.get(ctx, "/api/v2/utilities/ping", nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAvalaraClient creates and returns a configured AvalaraClient.
func GetAvalaraClient(ctx context.Context, environment, baseURL, username, password string) (*AvalaraClient, error) {
	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP client: %w", err)
	}

	baseHttpClient := uhttp.NewBaseHttpClient(httpClient)

	client := NewAvalaraClient(environment, baseURL, baseHttpClient)
	client.AddCredentials(username, password)
	return client, nil
}
