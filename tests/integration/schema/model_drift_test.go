//go:build integration

// Package schema compares the schema the migrations build against the schema
// the GORM models describe.
//
// The two are maintained by hand, in different files, and nothing else checks
// that they agree. The API test suite builds its tables with AutoMigrate,
// which derives columns from the models and therefore cannot disagree with
// them, so a column a model declares but no migration creates is invisible to
// every test and fatal in production: GORM names every column in its SELECT
// list, so one missing column breaks every query against that table.
//
// That is not hypothetical. models.Document declared file_id with no migration
// behind it, which made /api/v2/drafts -- and every other document read --
// return 500 on any properly migrated database.
package schema

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hashicorp-forge/hermes/internal/migrate"
	"github.com/hashicorp-forge/hermes/internal/test"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

const (
	migratedSchema = "from_migrations"
	modelSchema    = "from_models"
)

func TestMigrationsMatchModels(t *testing.T) {
	test.RequireDocker(t)

	ctx := context.Background()
	t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	pg, err := tcpostgres.Run(ctx,
		"pgvector/pgvector:pg17",
		tcpostgres.WithDatabase("hermes"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("starting postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := pg.Terminate(context.Background()); err != nil {
			t.Logf("terminating postgres: %v", err)
		}
	})

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	// Extensions live in one schema and both sides need their types.
	for _, ext := range []string{"vector", "citext", `"uuid-ossp"`} {
		if _, err := sqlDB.Exec(
			"CREATE EXTENSION IF NOT EXISTS " + ext + " SCHEMA public"); err != nil {
			t.Fatalf("installing %s: %v", ext, err)
		}
	}

	// Side one: what `hermes-migrate` produces.
	if err := migrate.RunMigrationsInSchema(sqlDB, "postgres", migratedSchema); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	// Side two: what the models describe.
	if _, err := sqlDB.Exec(`CREATE SCHEMA IF NOT EXISTS ` + modelSchema); err != nil {
		t.Fatalf("creating %s: %v", modelSchema, err)
	}
	modelDB, err := gorm.Open(
		postgres.Open(withSearchPath(dsn, modelSchema)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("connecting for AutoMigrate: %v", err)
	}
	if err := modelDB.AutoMigrate(models.ToAutoMigrate()...); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	migrated := columnsBySchema(t, sqlDB, migratedSchema)
	fromModels := columnsBySchema(t, sqlDB, modelSchema)

	var problems []string
	for table, modelCols := range fromModels {
		migratedCols, ok := migrated[table]
		if !ok {
			// A table only AutoMigrate creates is a join table or something
			// the migrations deliberately omit; report it, do not fail.
			t.Logf("note: table %q exists in the models but not in the migrations", table)
			continue
		}

		for col := range modelCols {
			if !migratedCols[col] {
				problems = append(problems, fmt.Sprintf("%s.%s", table, col))
			}
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("%d column(s) declared by a model but never created by a migration:\n  %s\n\n"+
			"GORM names every column in its SELECT list, so each of these makes "+
			"every query against that table fail with "+
			"\"column does not exist\" on a migrated database.\n"+
			"Add a migration for them.",
			len(problems), strings.Join(problems, "\n  "))
	}
}

// withSearchPath appends a search_path to a DSN in either of the two forms
// libpq accepts. The testcontainers helper hands back a URL, where a
// space-separated keyword would be parsed as part of the sslmode value.
func withSearchPath(dsn, schema string) string {
	value := schema + ",public"

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}

		return dsn + sep + "search_path=" + value
	}

	return dsn + " search_path=" + value
}

// columnsBySchema returns table -> set of column names for one schema.
func columnsBySchema(t *testing.T, db *sql.DB, schema string) map[string]map[string]bool {
	t.Helper()

	rows, err := db.Query(`
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema = $1`, schema)
	if err != nil {
		t.Fatalf("listing columns in %s: %v", schema, err)
	}
	defer func() { _ = rows.Close() }()

	out := map[string]map[string]bool{}
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			t.Fatalf("scanning columns: %v", err)
		}
		if out[table] == nil {
			out[table] = map[string]bool{}
		}
		out[table][column] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating columns: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("schema %s has no tables", schema)
	}

	return out
}
