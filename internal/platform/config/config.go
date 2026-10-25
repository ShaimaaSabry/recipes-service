// Package config reads the service's settings from environment variables. The same build runs
// locally and in prod; only these values differ.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Env names where the service runs.
type Env string

const (
	EnvLocal Env = "local"
	EnvProd  Env = "prod"
)

// Config is every setting the service needs.
type Config struct {
	Env         Env
	Port        string
	DatabaseURL string
}

// IsLocal reports whether the service is running on a developer's machine.
func (c Config) IsLocal() bool { return c.Env == EnvLocal }

// Load reads the settings through getenv (os.Getenv in main) and fails on anything missing or
// invalid, so a misconfigured server stops at startup instead of on its first request.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Env:         Env(strings.TrimSpace(getenv("APP_ENV"))),
		Port:        strings.TrimSpace(getenv("PORT")),
		DatabaseURL: strings.TrimSpace(getenv("DATABASE_URL")),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	var problems []error
	if cfg.Env != EnvLocal && cfg.Env != EnvProd {
		problems = append(problems, fmt.Errorf("APP_ENV must be %q or %q, got %q", EnvLocal, EnvProd, cfg.Env))
	}
	if cfg.DatabaseURL == "" {
		problems = append(problems, errors.New("DATABASE_URL is required"))
	}
	if len(problems) > 0 {
		return Config{}, fmt.Errorf("invalid config: %w", errors.Join(problems...))
	}
	return cfg, nil
}

// FromEnvironment loads the settings from the process environment.
func FromEnvironment() (Config, error) { return Load(os.Getenv) }
