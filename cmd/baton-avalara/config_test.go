package main

import (
	"testing"

	cfg "github.com/conductorone/baton-avalara/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/test"
	"github.com/spf13/viper"
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
