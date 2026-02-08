package connector

import (
	"context"
	"fmt"
	"io"

	avalaraclient "github.com/conductorone/baton-avalara/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
)

type Avalara struct {
	client *avalaraclient.AvalaraClient
}

// ResourceSyncers returns a ResourceSyncer for each resource type that should be synced from the upstream service.
func (d *Avalara) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	return []connectorbuilder.ResourceSyncer{
		newUserBuilder(d.client),
		newRoleBuilder(d.client),
	}
}

// Asset takes an input AssetRef and attempts to fetch it using the connector's authenticated http client.
// It streams a response, always starting with a metadata object, following by chunked payloads for the asset.
func (d *Avalara) Asset(ctx context.Context, asset *v2.AssetRef) (string, io.ReadCloser, error) {
	return "", nil, nil
}

// Metadata returns metadata about the connector.
func (d *Avalara) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName: "Avalara",
		Description: "The Avalara connector allows you to sync users, roles, and entitlements from your Avalara account.",
	}, nil
}

// Validate is called to ensure that the connector is properly configured. It should exercise any API credentials.
// to be sure that they are valid.
func (d *Avalara) Validate(ctx context.Context) (annotations.Annotations, error) {
	// Use the Ping method to validate the connection.
	pingResponse, err := d.client.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to validate Avalara connection: %w", err)
	}

	// Check if the ping response indicates successful authentication.
	if !pingResponse.Authenticated {
		return nil, fmt.Errorf("Avalara authentication failed")
	}

	return nil, nil
}

// New returns a new instance of the connector.
func New(
	ctx context.Context,
	environment string,
	baseURL string,
	username, password string,
) (*Avalara, error) {
	client, err := avalaraclient.GetAvalaraClient(ctx, environment, baseURL, username, password)
	if err != nil {
		return nil, err
	}

	return &Avalara{
		client: client,
	}, nil
}
