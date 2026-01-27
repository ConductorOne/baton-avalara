package main

import (
	"testing"

	cfg "github.com/conductorone/baton-avalara/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/test"
)

func TestConfigs(t *testing.T) {
	testCases := []test.TestCase{
		// Add test cases here.
	}

	test.ExerciseTestCases(t, cfg.Config, func(c *cfg.Avalara) error {
		return cfg.ValidateConfig(c)
	}, testCases)
}
