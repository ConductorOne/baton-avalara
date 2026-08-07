package connector

import (
	"context"
	"fmt"
	"strconv"

	avalaraclient "github.com/conductorone/baton-avalara/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
)

const (
	RoleMemberEntitlement = "member"
	RoleType              = "role"
	EntitlementType       = "entitlement"
)

type roleBuilder struct {
	client       *avalaraclient.AvalaraClient
	resourceType *v2.ResourceType
}

func (r *roleBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return r.resourceType
}

// List returns all the roles from the Avalara API as resource objects.
func (r *roleBuilder) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	pToken *pagination.Token,
) ([]*v2.Resource, string, annotations.Annotations, error) {
	var rv []*v2.Resource

	options := &avalaraclient.PaginationOptions{
		Top: 100,
	}

	if pToken != nil && pToken.Token != "" {
		options.NextLink = pToken.Token
	}

	roles, nextOptions, err := r.client.GetUserRoles(ctx, options)
	if err != nil {
		return nil, "", nil, fmt.Errorf("avalara-connector: failed to list roles: %w", err)
	}

	for _, role := range roles.Value {
		resource, err := rs.NewRoleResource(
			role.Description,
			roleResourceType,
			strconv.Itoa(role.ID),
			[]rs.RoleTraitOption{},
			rs.WithResourceProfile(map[string]interface{}{
				"id":          strconv.Itoa(role.ID),
				"description": role.Description,
			}),
			rs.WithParentResourceID(parentResourceID),
		)
		if err != nil {
			return nil, "", nil, fmt.Errorf("avalara-connector: failed to create role resource: %w", err)
		}

		rv = append(rv, resource)
	}

	var nextPageToken string
	if nextOptions != nil && nextOptions.NextLink != "" {
		nextPageToken = nextOptions.NextLink
	}

	return rv, nextPageToken, nil, nil
}

func newRoleBuilder(client *avalaraclient.AvalaraClient) *roleBuilder {
	return &roleBuilder{
		client:       client,
		resourceType: roleResourceType,
	}
}

// Grants returns all the grants for a given role.
// For Avalara, we'll check users to see if they have the specified role.
func (r *roleBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var rv []*v2.Grant

	roleDescription, ok := rs.GetProfileStringValue(rs.GetProfile(resource), "description")
	if !ok {
		return nil, "", nil, fmt.Errorf("avalara-connector: failed to get role description from profile")
	}

	options := &avalaraclient.PaginationOptions{
		Top: 100,
	}

	if pToken != nil && pToken.Token != "" {
		options.NextLink = pToken.Token
	}

	users, nextOptions, err := r.client.GetUsers(ctx, options)
	if err != nil {
		return nil, "", nil, fmt.Errorf("avalara-connector: failed to list users: %w", err)
	}

	for _, user := range users.Value {
		if user.SecurityRoleID == roleDescription {
			userID, err := rs.NewResourceID(userResourceType, strconv.Itoa(user.ID))
			if err != nil {
				return nil, "", nil, fmt.Errorf("avalara-connector: failed to create user resource id: %w", err)
			}

			rv = append(rv, grant.NewGrant(resource, RoleMemberEntitlement, userID))
		}
	}

	var nextPageToken string
	if nextOptions != nil && nextOptions.NextLink != "" {
		nextPageToken = nextOptions.NextLink
	}

	return rv, nextPageToken, nil, nil
}

func (r *roleBuilder) Entitlements(
	_ context.Context,
	resource *v2.Resource,
	_ *pagination.Token,
) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement

	entitlementOptions := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(userResourceType),
		entitlement.WithDisplayName(fmt.Sprintf("%s Role", resource.DisplayName)),
		entitlement.WithDescription(fmt.Sprintf("Avalara %s role assignment", resource.DisplayName)),
	}

	rv = append(rv, entitlement.NewAssignmentEntitlement(resource, RoleMemberEntitlement, entitlementOptions...))

	return rv, "", nil, nil
}
