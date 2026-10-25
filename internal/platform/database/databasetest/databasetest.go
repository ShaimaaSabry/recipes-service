// Package databasetest gives integration tests a real, empty Postgres schema of their own.
package databasetest

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mamienellie/recipes-service/internal/platform/database"
)

// EnvVar names the database the tests use; `make test-integration` sets it to the Docker one.
const EnvVar = "TEST_DATABASE_URL"

// New returns a connection whose tables live in a fresh schema, dropped when the test ends, so
// tests never see each other's rows. It skips the test when TEST_DATABASE_URL is not set.
func New(t *testing.T) *gorm.DB {
	t.Helper()
	base := os.Getenv(EnvVar)
	if base == "" {
		t.Skipf("set %s to run tests against Postgres (make test-integration)", EnvVar)
	}

	admin, err := database.Open(base, false)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_%s_%d", strings.ToLower(sanitize(t.Name())), time.Now().UnixNano())
	if err := admin.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatal(err)
	}

	db, err := database.Open(withSearchPath(t, base, schema), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		_ = admin.Exec(`DROP SCHEMA "` + schema + `" CASCADE`).Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func withSearchPath(t *testing.T, databaseURL, schema string) string {
	u, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}

func sanitize(name string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)[:min(len(name), 30)]
}
