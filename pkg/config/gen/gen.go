package main

import (
	cfg "github.com/conductorone/baton-avalara/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/config"
)

func main() {
	config.Generate("avalara", cfg.Config)
}
