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

// MinSigningKeyLength is the shortest access token signing key accepted, in bytes.
const MinSigningKeyLength = 32

// Config is every setting the service needs.
type Config struct {
	Env         Env
	Port        string
	DatabaseURL string
	// JWTSigningKey signs the access tokens the app sends with each request.
	JWTSigningKey string
	// AppleClientIDs are the app's bundle IDs that Sign in with Apple tokens must be issued for.
	AppleClientIDs []string
	// GoogleClientIDs are the app's Google OAuth client IDs (iOS, Android, web) that Google
	// Sign-In tokens must be issued for.
	GoogleClientIDs []string
	// PhotosBucket is the S3 bucket recipe photos are uploaded to, and PhotosURL the CloudFront
	// address they're served from. Locally both may be empty: photo uploads are then refused.
	PhotosBucket string
	PhotosURL    string
}

// IsLocal reports whether the service is running on a developer's machine.
func (c Config) IsLocal() bool { return c.Env == EnvLocal }

// Load reads the settings through getenv (os.Getenv in main) and fails on anything missing or
// invalid, so a misconfigured server stops at startup instead of on its first request.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Env:             Env(strings.TrimSpace(getenv("APP_ENV"))),
		Port:            strings.TrimSpace(getenv("PORT")),
		DatabaseURL:     strings.TrimSpace(getenv("DATABASE_URL")),
		JWTSigningKey:   strings.TrimSpace(getenv("JWT_SIGNING_KEY")),
		AppleClientIDs:  list(getenv("APPLE_CLIENT_IDS")),
		GoogleClientIDs: list(getenv("GOOGLE_CLIENT_IDS")),
		PhotosBucket:    strings.TrimSpace(getenv("PHOTOS_BUCKET")),
		PhotosURL:       strings.TrimSpace(getenv("PHOTOS_URL")),
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
	if len(cfg.JWTSigningKey) < MinSigningKeyLength {
		problems = append(problems, fmt.Errorf("JWT_SIGNING_KEY must be at least %d characters", MinSigningKeyLength))
	}
	// Locally, POST /v1/auth/dev signs in without them.
	if cfg.Env == EnvProd && len(cfg.AppleClientIDs) == 0 {
		problems = append(problems, errors.New("APPLE_CLIENT_IDS is required in prod"))
	}
	if cfg.Env == EnvProd && len(cfg.GoogleClientIDs) == 0 {
		problems = append(problems, errors.New("GOOGLE_CLIENT_IDS is required in prod"))
	}
	if cfg.Env == EnvProd && cfg.PhotosBucket == "" {
		problems = append(problems, errors.New("PHOTOS_BUCKET is required in prod"))
	}
	if (cfg.PhotosBucket == "") != (cfg.PhotosURL == "") {
		problems = append(problems, errors.New("PHOTOS_BUCKET and PHOTOS_URL go together: set both or neither"))
	}
	if len(problems) > 0 {
		return Config{}, fmt.Errorf("invalid config: %w", errors.Join(problems...))
	}
	return cfg, nil
}

// FromEnvironment loads the settings from the process environment.
func FromEnvironment() (Config, error) { return Load(os.Getenv) }

// list splits a comma-separated setting, dropping blanks.
func list(raw string) []string {
	var values []string
	for _, value := range strings.Split(raw, ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return values
}
