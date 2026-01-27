package main

import (
	"testing"

	cfg "github.com/conductorone/baton-avalara/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/test"
	"github.com/spf13/viper"
)

func TestConfigs(t *testing.T) {
	testCases := []test.TestCase{
		// Add test cases here.
	}

	test.ExerciseTestCases(t, cfg.Config, func(v *viper.Viper) error {
		return nil
	}, testCases)
}
