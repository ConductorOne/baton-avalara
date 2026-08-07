package connector

import (
	"context"
	"fmt"

	avalaraclient "github.com/conductorone/baton-avalara/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
)

type userBuilder struct {
	client       *avalaraclient.AvalaraClient
	resourceType *v2.ResourceType
}

func (o *userBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return o.resourceType
}

// List returns all the users from the Avalara API as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *userBuilder) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	pToken *pagination.Token,
) ([]*v2.Resource, string, annotations.Annotations, error) {
	pageSize := 100
	var users []*v2.Resource

	options := &avalaraclient.PaginationOptions{
		Top:  pageSize,
		Skip: 0,
	}

	if pToken != nil && pToken.Token != "" {
		options.NextLink = pToken.Token
	}

	resp, nextOptions, err := o.client.GetUsers(ctx, options)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to get users: %w", err)
	}

	for _, user := range resp.Value {
		resource, err := userResource(ctx, &user, parentResourceID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("failed to create user resource: %w", err)
		}
		users = append(users, resource)
	}

	var nextPageToken string
	if nextOptions != nil && nextOptions.NextLink != "" {
		nextPageToken = nextOptions.NextLink
	}

	return users, nextPageToken, nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *userBuilder) Entitlements(
	_ context.Context,
	resource *v2.Resource,
	_ *pagination.Token,
) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *userBuilder) Grants(
	ctx context.Context,
	resource *v2.Resource,
	pToken *pagination.Token,
) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newUserBuilder(client *avalaraclient.AvalaraClient) *userBuilder {
	return &userBuilder{
		client:       client,
		resourceType: userResourceType,
	}
}

func userResource(ctx context.Context, user *avalaraclient.UserModel, parent *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"id":                   user.ID,
		"firstName":            user.FirstName,
		"lastName":             user.LastName,
		"email":                user.Email,
		"userName":             user.UserName,
		"accountId":            user.AccountID,
		"companyId":            user.CompanyID,
		"isActive":             user.IsActive,
		"suppressNewUserEmail": user.SuppressNewUserEmail,
		"isDeleted":            user.IsDeleted,
	}

	status := v2.UserTrait_Status_STATUS_ENABLED
	if !user.IsActive {
		status = v2.UserTrait_Status_STATUS_DISABLED
	}

	userTraitOptions := []rs.UserTraitOption{
		rs.WithUserLogin(user.UserName),
		rs.WithEmail(user.Email, true),
	}

	resource, err := rs.NewUserResource(
		user.UserName,
		userResourceType,
		user.ID,
		userTraitOptions,
		rs.WithResourceProfile(profile),
		rs.WithResourceStatus(v2.Status_ResourceStatus(status), ""),
		rs.WithParentResourceID(parent),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user resource: %w", err)
	}

	return resource, nil
}
