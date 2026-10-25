// Package database opens the Postgres connection and applies each module's migrations.
package database

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open connects to Postgres. SQL logging is on only when verbose (locally). Errors are translated,
// so repositories can check for gorm.ErrDuplicatedKey instead of Postgres error codes.
func Open(databaseURL string, verbose bool) (*gorm.DB, error) {
	level := logger.Warn
	if verbose {
		level = logger.Info
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger:         logger.Default.LogMode(level),
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return db, nil
}

// Migrate applies a module's pending migrations. Each module keeps its own history table
// (e.g. "recipes_migrations"), so modules never depend on each other's schema versions.
func Migrate(ctx context.Context, db *gorm.DB, module string, migrations fs.FS) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("migrate %s: %w", module, err)
	}
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		sqlDB,
		migrations,
		goose.WithTableName(module+"_migrations"),
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return fmt.Errorf("migrate %s: %w", module, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate %s: %w", module, err)
	}
	return nil
}
