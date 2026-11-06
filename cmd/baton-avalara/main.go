package main

import (
	"context"

	cfg "github.com/conductorone/baton-avalara/pkg/config"
	"github.com/conductorone/baton-avalara/pkg/connector"
	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorrunner"
)

var version = "dev"

func main() {
	ctx := context.Background()

	config.RunConnector(
		ctx,
		"baton-avalara",
		version,
		cfg.Config,
		connector.New,
		connectorrunner.WithDefaultCapabilitiesConnectorBuilderV2(&connector.Avalara{}),
	)
}
