package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/migrate"
	"github.com/hashicorp-forge/hermes/internal/sites"
	"github.com/hashicorp-forge/hermes/pkg/database"
	"github.com/hashicorp-forge/hermes/pkg/domain"
)

// SiteDBs holds one connection pool per site.
//
// Sites are isolated by PostgreSQL schema, and a schema is selected through
// the connection string, so isolation is a property of the pool rather than
// of the query. One pool per site is the cost of that; the alternative --
// one pool with a runtime `SET search_path` per request -- does not survive
// connection reuse.
type SiteDBs struct {
	byDomain map[domain.Name]*gorm.DB
	fallback *gorm.DB
}

// NewSiteDBsWithPools builds a SiteDBs from pools the caller already opened.
//
// NewSiteDBs is the normal path; this exists for callers that manage
// connections themselves, and for tests that need a site map without a live
// server.
func NewSiteDBsWithPools(byDomain map[domain.Name]*gorm.DB, fallback *gorm.DB) *SiteDBs {
	s := &SiteDBs{
		byDomain: make(map[domain.Name]*gorm.DB, len(byDomain)),
		fallback: fallback,
	}
	for name, db := range byDomain {
		s.byDomain[name] = db
	}

	return s
}

// NewSiteDBs opens a connection pool for every site in the registry.
//
// fallback is used when a request carries no site, which is every request in a
// deployment with no `site` blocks. Passing nil is allowed and makes an
// unscoped request an error rather than a silent read of the default schema.
func NewSiteDBs(
	cfg config.Postgres, registry *sites.Registry, fallback *gorm.DB, log hclog.Logger,
) (*SiteDBs, error) {
	s := &SiteDBs{
		byDomain: make(map[domain.Name]*gorm.DB),
		fallback: fallback,
	}
	if registry == nil || registry.Len() == 0 {
		return s, nil
	}

	for _, site := range registry.Sites() {
		db, err := NewDBForSite(site.Postgres, log)
		if err != nil {
			return nil, fmt.Errorf(
				"opening database for site %q: %w", site.Domain.String(), err)
		}
		s.byDomain[site.Domain] = db
	}

	// With sites configured there is no process-wide database to fall back to,
	// because a multi-site deployment keeps no Hermes tables in public. Work
	// that genuinely has no site -- the instance heartbeat, the health probe --
	// still needs somewhere to go, so it goes to the primary site.
	if s.fallback == nil {
		primary := registry.Default()
		if primary == nil {
			primary = registry.Sites()[0]
		}
		s.fallback = s.byDomain[primary.Domain]
		if log != nil {
			log.Info("site-less operations will use the primary site",
				"site", primary.Domain.String(), "schema", primary.SchemaName)
		}
	}

	return s, nil
}

// Each calls fn for every site pool, stopping at the first error.
//
// Ordering follows the registry, so the primary site is not special here;
// callers that need it should ask the registry.
func (s *SiteDBs) Each(fn func(domain.Name, *gorm.DB) error) error {
	for name, db := range s.byDomain {
		if err := fn(name, db); err != nil {
			return fmt.Errorf("site %q: %w", name.String(), err)
		}
	}

	return nil
}

// Fallback returns the pool used for work that carries no site.
func (s *SiteDBs) Fallback() *gorm.DB { return s.fallback }

// For returns the pool serving the site named in ctx.
//
// A request with no site resolves to the fallback. A request naming a site
// that has no pool is an error and must be treated as one: falling back there
// would run one tenant's query against another's schema, which is the exact
// failure this package exists to prevent.
func (s *SiteDBs) For(ctx context.Context) (*gorm.DB, error) {
	name, ok := domain.FromContext(ctx)
	if !ok || name.IsZero() {
		if s.fallback == nil {
			return nil, fmt.Errorf("db: request has no site and no default database")
		}

		return s.fallback, nil
	}

	db, ok := s.byDomain[name]
	if !ok {
		return nil, fmt.Errorf("db: no database configured for site %q", name.String())
	}

	return db, nil
}

// Len returns the number of site-scoped pools.
func (s *SiteDBs) Len() int { return len(s.byDomain) }

// Close closes every site pool. The fallback is not closed, since its lifetime
// is owned by whoever passed it in.
func (s *SiteDBs) Close() error {
	var firstErr error
	for name, db := range s.byDomain {
		sqlDB, err := db.DB()
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("site %q: %w", name.String(), err)
			}
			continue
		}
		if err := sqlDB.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("site %q: %w", name.String(), err)
		}
	}

	return firstErr
}

// NewDBForSite opens a connection pool for one site.
//
// The configuration carries the site's schema and, when the site overrides it,
// its own host, database, and credentials -- so two sites may live in
// different schemas of one database, in different databases, or on entirely
// different servers, and the caller does not have to care which.
func NewDBForSite(cfg config.Postgres, log hclog.Logger) (*gorm.DB, error) {
	db, err := database.Connect(databaseConfig(cfg), log)
	if err != nil {
		return nil, err
	}

	if err := setupJoinTables(db); err != nil {
		return nil, err
	}

	return db, nil
}

// NewDBForSchema opens a connection pool scoped to one PostgreSQL schema of
// the given database.
//
// Deprecated: use NewDBForSite, which also carries any per-site connection
// override. Retained for callers that only vary the schema.
func NewDBForSchema(
	cfg config.Postgres, schema string, log hclog.Logger,
) (*gorm.DB, error) {
	cfg.SchemaName = schema

	return NewDBForSite(cfg, log)
}

func databaseConfig(cfg config.Postgres) database.Config {
	return database.Config{
		Host:       cfg.Host,
		Port:       cfg.Port,
		User:       cfg.User,
		Password:   cfg.Password,
		DBName:     cfg.DBName,
		SSLMode:    cfg.EffectiveSSLMode(),
		SchemaName: cfg.SchemaName,
	}
}

// MigrateSites runs migrations for every site.
//
// Each site gets its own schema *and its own golang-migrate version table
// inside it*, so the runs are genuinely independent: a new site added to an
// existing deployment migrates from zero rather than inheriting another site's
// version and coming up empty.
//
// Sites may live in different databases, so this connects once per site rather
// than once overall. That is a handful of short-lived connections at startup,
// which is not worth optimising into a per-backend cache.
func MigrateSites(_ config.Postgres, registry *sites.Registry, log hclog.Logger) error {
	if registry == nil || registry.Len() == 0 {
		return nil
	}

	for _, site := range registry.Sites() {
		if err := migrateSite(site, log); err != nil {
			return err
		}
	}

	return nil
}

func migrateSite(site *sites.Site, log hclog.Logger) error {
	// Connect without a schema: the schema does not exist yet, and the
	// extension install below is database-wide.
	connCfg := site.Postgres
	connCfg.SchemaName = ""

	dsn, err := databaseConfig(connCfg).DSN()
	if err != nil {
		return fmt.Errorf("site %q: %w", site.Domain.String(), err)
	}

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("site %q: connecting for migrations: %w", site.Domain.String(), err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("site %q: pinging for migrations: %w", site.Domain.String(), err)
	}

	if err := ensureSharedExtensions(sqlDB); err != nil {
		return fmt.Errorf("site %q: %w", site.Domain.String(), err)
	}

	if log != nil {
		log.Info("migrating site schema",
			"domain", site.Domain.String(),
			"schema", site.SchemaName,
			"host", site.Postgres.Host,
			"dbname", site.Postgres.DBName)
	}

	if err := migrate.RunMigrationsInSchema(sqlDB, "postgres", site.SchemaName); err != nil {
		return fmt.Errorf(
			"migrating site %q (schema %q): %w",
			site.Domain.String(), site.SchemaName, err)
	}

	if err := applySiteIdentity(sqlDB, site); err != nil {
		return fmt.Errorf("site %q: %w", site.Domain.String(), err)
	}

	return nil
}

// sharedExtensions are the PostgreSQL extensions Hermes migrations create.
//
// They are installed once, in public, before any site is migrated.
var sharedExtensions = []string{"vector", "uuid-ossp", "citext"}

// ensureSharedExtensions installs every extension the migrations need into the
// public schema, before any site migration runs.
//
// An extension is a database-scoped object that lives in exactly one schema.
// The migrations say `CREATE EXTENSION IF NOT EXISTS <x>` with no SCHEMA
// clause, which puts it in the first entry of the running search_path -- so
// under per-site migration it lands in whichever site migrated first. Every
// later site then finds the extension already present, skips creating it, and
// fails on the type: `citext` and `vector` live in a schema that site's
// search_path does not include.
//
// Installing them in public makes their types resolvable from every site,
// which is the reason public stays on the search_path at all.
func ensureSharedExtensions(sqlDB *sql.DB) error {
	for _, ext := range sharedExtensions {
		quoted := pq.QuoteIdentifier(ext)
		if _, err := sqlDB.Exec(
			"CREATE EXTENSION IF NOT EXISTS " + quoted + " SCHEMA public",
		); err != nil {
			return fmt.Errorf(
				"installing the %s extension: %w\n\n"+
					"Multi-site Hermes needs %s available on the server. "+
					"pgvector ships separately (package postgresql-NN-pgvector, or the "+
					"pgvector/pgvector image); citext and uuid-ossp are in postgresql-contrib",
				ext, err, ext)
		}

		// IF NOT EXISTS makes the SCHEMA clause a no-op when the extension is
		// already present, so a database upgraded from a single-tenant install
		// may still have it somewhere private. That is not something to paper
		// over: it surfaces as a migration failure on the second site, which
		// reads as a bug in Hermes rather than as a fixable state.
		var schema string
		err := sqlDB.QueryRow(`
			SELECT n.nspname
			FROM pg_extension e
			JOIN pg_namespace n ON n.oid = e.extnamespace
			WHERE e.extname = $1`, ext).Scan(&schema)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			continue
		case err != nil:
			return fmt.Errorf("locating the %s extension: %w", ext, err)
		case schema != "public":
			return fmt.Errorf(
				"the %s extension is installed in schema %q, not public.\n"+
					"Every site's search_path includes public but not %q, so sites "+
					"other than the first would fail on its types.\n"+
					"Move it with: ALTER EXTENSION %s SET SCHEMA public",
				ext, schema, schema, ext)
		}
	}

	return nil
}

// tenantTables are the top-level object tables a site owns.
//
// Child and join tables are deliberately absent: they reach their tenant
// through a foreign key, and stamping every one of them would add write cost
// and a second place for the two to disagree.
var tenantTables = []string{
	"documents",
	"projects",
	"products",
	"users",
	"groups",
	"document_types",
	"workspace_projects",
}

// TenantColumn is the column naming the site that owns a row.
const TenantColumn = "domain"

// applySiteIdentity records which site a schema belongs to, and stamps that
// site onto every top-level object table.
//
// The schema alone already isolates tenants, so this is not how isolation is
// enforced -- it is how it stays checkable. A schema is just a namespace: dump
// one and restore it into another, point `schema_name` at the wrong place, or
// recover a backup into the wrong environment, and nothing in the data itself
// would object. With the site stamped on each row, the rows are
// self-describing, and the CHECK constraint turns a mis-restore into an error
// instead of a silent tenant merge.
//
// The column carries a per-schema DEFAULT, so the application never sets it
// and cannot forget to. GORM does not know the column exists, which is the
// point: there is no code path that can write the wrong value.
func applySiteIdentity(sqlDB *sql.DB, site *sites.Site) error {
	schema := database.QuoteSchemaName(site.SchemaName)
	name := site.Domain.String()
	literal := pq.QuoteLiteral(name)

	if _, err := sqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS ` + schema + `.site_identity (
			domain      TEXT PRIMARY KEY,
			schema_name TEXT NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("creating site_identity: %w", err)
	}

	// A schema belongs to exactly one site, for its whole life. Finding
	// another name here means two sites resolved to one schema -- almost
	// always a `schema_name` override pointing somewhere already in use, which
	// would otherwise merge two tenants silently.
	var existing string
	err := sqlDB.QueryRow(
		`SELECT domain FROM ` + schema + `.site_identity LIMIT 1`).Scan(&existing)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err := sqlDB.Exec(
			`INSERT INTO `+schema+`.site_identity (domain, schema_name) VALUES ($1, $2)`,
			name, site.SchemaName); err != nil {
			return fmt.Errorf("recording site identity: %w", err)
		}
	case err != nil:
		return fmt.Errorf("reading site identity: %w", err)
	case existing != name:
		return fmt.Errorf(
			"schema %q already belongs to site %q, but %q is configured to use it.\n"+
				"Two sites sharing a schema share their data. Give one of them a "+
				"different schema_name, or drop the schema if it is disused",
			site.SchemaName, existing, name)
	}

	for _, table := range tenantTables {
		if err := stampTenantColumn(sqlDB, site.SchemaName, table, literal); err != nil {
			return fmt.Errorf("stamping %s.%s: %w", site.SchemaName, table, err)
		}
	}

	return nil
}

// stampTenantColumn adds the tenant column to one table, backfills it, and
// constrains it to this site.
//
// Every step is idempotent: migrations run on every startup, and a site added
// to an existing deployment must not be treated differently from one that has
// been there for months.
func stampTenantColumn(sqlDB *sql.DB, schemaName, table, literal string) error {
	qualified := database.QuoteSchemaName(schemaName) + "." + pq.QuoteIdentifier(table)

	// A table listed here may not exist in every schema version.
	var present sql.NullString
	if err := sqlDB.QueryRow(
		"SELECT to_regclass($1)::text", schemaName+"."+table).Scan(&present); err != nil {
		return fmt.Errorf("checking for the table: %w", err)
	}
	if !present.Valid {
		return nil
	}

	column := pq.QuoteIdentifier(TenantColumn)
	constraint := pq.QuoteIdentifier("chk_" + table + "_" + TenantColumn)

	statements := []string{
		// Added nullable first so an existing table with rows can take it.
		`ALTER TABLE ` + qualified + ` ADD COLUMN IF NOT EXISTS ` + column + ` TEXT`,
		// The default is what makes the application's ignorance of this column
		// safe: an INSERT that never mentions it still gets the right value.
		`ALTER TABLE ` + qualified + ` ALTER COLUMN ` + column + ` SET DEFAULT ` + literal,
		`UPDATE ` + qualified + ` SET ` + column + ` = ` + literal + ` WHERE ` + column + ` IS NULL`,
		`ALTER TABLE ` + qualified + ` ALTER COLUMN ` + column + ` SET NOT NULL`,
	}
	for _, stmt := range statements {
		if _, err := sqlDB.Exec(stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}

	// CHECK constraints have no IF NOT EXISTS, so add it only when absent.
	if _, err := sqlDB.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM pg_constraint c
				JOIN pg_class t ON t.oid = c.conrelid
				JOIN pg_namespace n ON n.oid = t.relnamespace
				WHERE n.nspname = ` + pq.QuoteLiteral(schemaName) + `
				  AND t.relname = ` + pq.QuoteLiteral(table) + `
				  AND c.conname = ` + pq.QuoteLiteral("chk_"+table+"_"+TenantColumn) + `
			) THEN
				EXECUTE 'ALTER TABLE ` + qualified + ` ADD CONSTRAINT ' ||
					` + pq.QuoteLiteral(constraint) + ` ||
					' CHECK (` + column + ` = ` + strings.ReplaceAll(literal, "'", "''") + `)';
			END IF;
		END $$`); err != nil {
		return fmt.Errorf("adding the tenant check constraint: %w", err)
	}

	return nil
}
