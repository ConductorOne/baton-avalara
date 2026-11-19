package main

import (
	"testing"

	cfg "github.com/conductorone/baton-avalara/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestConfigs(t *testing.T) {
	testCases := []test.TestCase{
		{
			Configs: map[string]string{
				"username": "user",
				"password": "pass",
			},
			IsValid: true,
			Message: "valid config",
		},
		{
			Configs: map[string]string{
				"password": "pass",
			},
			IsValid: false,
			Message: "missing username",
		},
		{
			Configs: map[string]string{
				"username": "user",
			},
			IsValid: false,
			Message: "missing password",
		},
	}

	test.ExerciseTestCases(t, cfg.Config, func(v *viper.Viper) error {
		return nil
	}, testCases)
}

// TestContainerizedConfigFromC1 checks the config that C1 sends to the containerized connector.
// The lambda path reads auth-method, passes it to field.Validate only when it is not empty,
// and then validates the config.
func TestContainerizedConfigFromC1(t *testing.T) {
	validate := func(input map[string]string) error {
		v := test.MakeViper(input)
		var opts []field.Option
		if am := v.GetString("auth-method"); am != "" {
			opts = append(opts, field.WithAuthMethod(am))
		}
		return field.Validate(cfg.Config, v, opts...)
	}

	t.Run("username, password and environment", func(t *testing.T) {
		require.NoError(t, validate(map[string]string{
			"username":    "user",
			"password":    "s3cret",
			"environment": "sandbox",
		}))
	})

	t.Run("with auth-method avalara_credentials", func(t *testing.T) {
		require.NoError(t, validate(map[string]string{
			"username":    "user",
			"password":    "s3cret",
			"environment": "sandbox",
			"auth-method": "avalara_credentials",
		}))
	})

	t.Run("missing password", func(t *testing.T) {
		require.Error(t, validate(map[string]string{
			"username":    "user",
			"environment": "sandbox",
		}))
	})
}
