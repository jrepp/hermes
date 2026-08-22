package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
		db, err := NewDBForSchema(cfg, site.SchemaName, log)
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

// NewDBForSchema opens a connection pool scoped to one PostgreSQL schema.
func NewDBForSchema(
	cfg config.Postgres, schema string, log hclog.Logger,
) (*gorm.DB, error) {
	db, err := database.Connect(database.Config{
		Host:       cfg.Host,
		Port:       cfg.Port,
		User:       cfg.User,
		Password:   cfg.Password,
		DBName:     cfg.DBName,
		SSLMode:    "disable",
		SchemaName: schema,
	}, log)
	if err != nil {
		return nil, err
	}

	if err := setupJoinTables(db); err != nil {
		return nil, err
	}

	return db, nil
}

// MigrateSites runs migrations for every site schema.
//
// Each site gets its own schema *and its own golang-migrate version table
// inside it*, so the runs are genuinely independent: a new site added to an
// existing deployment migrates from zero rather than inheriting another site's
// version and coming up empty.
func MigrateSites(cfg config.Postgres, registry *sites.Registry, log hclog.Logger) error {
	if registry == nil || registry.Len() == 0 {
		return nil
	}

	dsn, err := database.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		DBName:   cfg.DBName,
		SSLMode:  "disable",
	}.DSN()
	if err != nil {
		return err
	}

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("connecting for site migrations: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("pinging for site migrations: %w", err)
	}

	if err := ensureSharedExtensions(sqlDB); err != nil {
		return err
	}

	for _, site := range registry.Sites() {
		if log != nil {
			log.Info("migrating site schema",
				"domain", site.Domain.String(), "schema", site.SchemaName)
		}
		if err := migrate.RunMigrationsInSchema(sqlDB, "postgres", site.SchemaName); err != nil {
			return fmt.Errorf(
				"migrating site %q (schema %q): %w",
				site.Domain.String(), site.SchemaName, err)
		}
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
