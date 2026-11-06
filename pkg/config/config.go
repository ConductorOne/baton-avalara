package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	UsernameField = field.StringField(
		"username",
		field.WithDisplayName("Username"),
		field.WithPlaceholder("Your Avalara username"),
		field.WithDescription("The Avalara username used to connect to the Avalara API"),
		field.WithRequired(true),
	)
	PasswordField = field.StringField(
		"password",
		field.WithDisplayName("Password"),
		field.WithPlaceholder("Your Avalara password"),
		field.WithDescription("The Avalara password used to connect to the Avalara API"),
		field.WithIsSecret(true),
		field.WithRequired(true),
	)
	EnvironmentField = field.StringField(
		"environment",
		field.WithDisplayName("Environment"),
		field.WithPlaceholder("Production or sandbox"),
		field.WithDescription("The Avalara environment to connect to (production or sandbox)"),
		field.WithDefaultValue("production"),
	)
	BaseURLField = field.StringField(
		"base-url",
		field.WithDescription("Override the Avalara API URL (for testing)"),
		field.WithHidden(true),
		field.WithExportTarget(field.ExportTargetCLIOnly),
	)
)

//go:generate go run ./gen
var Config = field.NewConfiguration(
	[]field.SchemaField{
		UsernameField,
		PasswordField,
		EnvironmentField,
		BaseURLField,
	},
	field.WithConnectorDisplayName("Avalara"),
	field.WithIconUrl("/static/app-icons/avalara.svg"),
	field.WithHelpUrl("/docs/baton/avalara"),
)
