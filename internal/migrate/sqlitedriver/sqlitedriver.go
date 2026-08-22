// Package sqlitedriver registers the SQLite migration driver.
//
// It exists so that importing SQLite support is a deliberate act. The driver
// pulls in modernc.org/sqlite -- a pure-Go SQLite implementation weighing tens
// of megabytes -- and ADR-019 keeps that out of the server binary. Because
// internal/migrate is reachable from cmd/hermes through internal/db, an
// unconditional import there would link SQLite into the server whether or not
// it can ever use it, which is what was happening.
//
// Blank-import this package from a binary that needs SQLite:
//
//	import _ "github.com/hashicorp-forge/hermes/internal/migrate/sqlitedriver"
package sqlitedriver

import (
	"database/sql"

	migratedb "github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/sqlite"

	"github.com/hashicorp-forge/hermes/internal/migrate"
)

func init() {
	migrate.RegisterDriver("sqlite", func(db *sql.DB) (migratedb.Driver, error) {
		return sqlite.WithInstance(db, &sqlite.Config{})
	})
}
