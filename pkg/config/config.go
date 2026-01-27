package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	UsernameField = field.StringField(
		"username",
		field.WithDescription("The Avalara username used to connect to the Avalara API"),
		field.WithRequired(true),
	)
	PasswordField = field.StringField(
		"password",
		field.WithDescription("The Avalara password used to connect to the Avalara API"),
		field.WithRequired(true),
	)
	EnvironmentField = field.StringField(
		"environment",
		field.WithDescription("The Avalara environment to connect to (production or sandbox)"),
		field.WithDefaultValue("production"),
	)

	FieldRelationships = []field.SchemaFieldRelationship{
		field.FieldsRequiredTogether(
			UsernameField,
			PasswordField,
		),
	}
)

//go:generate go run ./gen
var Config = field.NewConfiguration([]field.SchemaField{
	UsernameField,
	PasswordField,
	EnvironmentField,
}, field.WithConstraints(FieldRelationships...))

func ValidateConfig(cfg *Avalara) error {
	return nil
}
