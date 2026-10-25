package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoad(t *testing.T) {
	t.Run("reads every setting", func(t *testing.T) {
		cfg, err := Load(env(map[string]string{"APP_ENV": "prod", "PORT": "9000", "DATABASE_URL": "postgres://db"}))

		require.NoError(t, err)
		require.Equal(t, Config{Env: EnvProd, Port: "9000", DatabaseURL: "postgres://db"}, cfg)
		require.False(t, cfg.IsLocal())
	})

	t.Run("defaults the port to 8080", func(t *testing.T) {
		cfg, err := Load(env(map[string]string{"APP_ENV": "local", "DATABASE_URL": "postgres://db"}))

		require.NoError(t, err)
		require.Equal(t, "8080", cfg.Port)
		require.True(t, cfg.IsLocal())
	})

	t.Run("names every missing or wrong setting at once", func(t *testing.T) {
		_, err := Load(env(map[string]string{"APP_ENV": "staging"}))

		require.ErrorContains(t, err, `APP_ENV must be "local" or "prod", got "staging"`)
		require.ErrorContains(t, err, "DATABASE_URL is required")
	})
}
