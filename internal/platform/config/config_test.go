package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

const signingKey = "0123456789abcdef0123456789abcdef"

func TestLoad(t *testing.T) {
	t.Run("reads every setting", func(t *testing.T) {
		cfg, err := Load(env(map[string]string{
			"APP_ENV":           "prod",
			"PORT":              "9000",
			"DATABASE_URL":      "postgres://db",
			"JWT_SIGNING_KEY":   signingKey,
			"APPLE_CLIENT_IDS":  "com.mamienellie.app",
			"GOOGLE_CLIENT_IDS": " ios.apps.googleusercontent.com , android.apps.googleusercontent.com,",
			"PHOTOS_BUCKET":     "mamie-nellie-prod",
			"PHOTOS_URL":        "https://photos.example.net",
		}))

		require.NoError(t, err)
		require.Equal(t, Config{
			Env:             EnvProd,
			Port:            "9000",
			DatabaseURL:     "postgres://db",
			JWTSigningKey:   signingKey,
			AppleClientIDs:  []string{"com.mamienellie.app"},
			GoogleClientIDs: []string{"ios.apps.googleusercontent.com", "android.apps.googleusercontent.com"},
			PhotosBucket:    "mamie-nellie-prod",
			PhotosURL:       "https://photos.example.net",
		}, cfg)
		require.False(t, cfg.IsLocal())
	})

	t.Run("doesn't need Apple or Google locally", func(t *testing.T) {
		cfg, err := Load(env(map[string]string{"APP_ENV": "local", "DATABASE_URL": "postgres://db", "JWT_SIGNING_KEY": signingKey}))

		require.NoError(t, err)
		require.Empty(t, cfg.AppleClientIDs)
		require.Empty(t, cfg.GoogleClientIDs)
	})

	t.Run("needs the photos bucket in prod, and its URL with it anywhere", func(t *testing.T) {
		_, err := Load(env(map[string]string{
			"APP_ENV": "prod", "DATABASE_URL": "postgres://db", "JWT_SIGNING_KEY": signingKey,
			"APPLE_CLIENT_IDS": "a", "GOOGLE_CLIENT_IDS": "g",
		}))
		require.ErrorContains(t, err, "PHOTOS_BUCKET is required in prod")

		_, err = Load(env(map[string]string{"APP_ENV": "local", "DATABASE_URL": "postgres://db", "JWT_SIGNING_KEY": signingKey, "PHOTOS_BUCKET": "b"}))
		require.ErrorContains(t, err, "PHOTOS_BUCKET and PHOTOS_URL go together")
	})

	t.Run("defaults the port to 8080", func(t *testing.T) {
		cfg, err := Load(env(map[string]string{"APP_ENV": "local", "DATABASE_URL": "postgres://db", "JWT_SIGNING_KEY": signingKey}))

		require.NoError(t, err)
		require.Equal(t, "8080", cfg.Port)
		require.True(t, cfg.IsLocal())
	})

	t.Run("names every missing or wrong setting at once", func(t *testing.T) {
		_, err := Load(env(map[string]string{"APP_ENV": "staging", "JWT_SIGNING_KEY": "too-short"}))

		require.ErrorContains(t, err, `APP_ENV must be "local" or "prod", got "staging"`)
		require.ErrorContains(t, err, "DATABASE_URL is required")
		require.ErrorContains(t, err, "JWT_SIGNING_KEY must be at least 32 characters")
	})

	t.Run("needs Apple and Google client IDs in prod", func(t *testing.T) {
		_, err := Load(env(map[string]string{"APP_ENV": "prod", "DATABASE_URL": "postgres://db", "JWT_SIGNING_KEY": signingKey}))

		require.ErrorContains(t, err, "APPLE_CLIENT_IDS is required in prod")
		require.ErrorContains(t, err, "GOOGLE_CLIENT_IDS is required in prod")
	})
}
