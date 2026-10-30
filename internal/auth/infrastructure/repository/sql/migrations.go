package sql

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrations are the auth module's schema changes, applied in order at startup.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		panic(err) // the directory is embedded above, so this cannot happen
	}
	return sub
}
