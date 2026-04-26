# Database Migrations

This directory contains database schema migrations for Hermes, supporting both **PostgreSQL** and **SQLite**.

## Migration Strategy

Migrations are split into:
1. **Core schema** - SQL that works for both PostgreSQL and SQLite
2. **Database-specific extras** - Type conversions and optimizations

This approach minimizes duplicate SQL maintenance (71% reduction compared to separate migration files).

## Migration Files

### Structure

```
migrations/
  # Core schema (version 1)
  000001_core_schema.up.sql           # Works for both PostgreSQL and SQLite
  000001_core_schema.down.sql
  
  # PostgreSQL-specific enhancements
  000001_postgres_extras.up.sql       # UUID types, CITEXT, extensions
  000001_postgres_extras.down.sql
  
  # SQLite-specific enhancements
  000001_sqlite_extras.up.sql         # PRAGMAs, optimizations
  000001_sqlite_extras.down.sql
  
  # Indexer tables (version 2)
  000002_indexer_core.up.sql
  000002_indexer_core.down.sql
  000002_indexer_postgres.up.sql
  000002_indexer_postgres.down.sql
  000002_indexer_sqlite.up.sql
  000002_indexer_sqlite.down.sql
```

### Execution Order

When running migrations, the system:

1. **Runs core migration** (e.g., `000001_core_schema.up.sql`)
   - Creates tables with compatible types (TEXT, INTEGER, TIMESTAMP)
   - Defines all foreign keys and indexes

2. **Runs database-specific migration** (e.g., `000001_postgres_extras.up.sql`)
   - **PostgreSQL**: Converts TEXT→UUID, TEXT→CITEXT, INTEGER→BOOLEAN
   - **SQLite**: Configures PRAGMAs (foreign keys, WAL mode, performance)

## Creating New Migrations

### Using golang-migrate CLI

```bash
# Install golang-migrate
go install -tags 'postgres sqlite' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Create new migration
migrate create -ext sql -dir internal/db/migrations -seq add_feature
```

This creates:
- `000003_add_feature.up.sql`
- `000003_add_feature.down.sql`

### Recommended Structure

**Step 1**: Create core migration (`000003_add_feature_core.up.sql`)
```sql
-- Core schema (compatible with both PostgreSQL and SQLite)
CREATE TABLE new_feature (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    name TEXT,
    enabled INTEGER DEFAULT 0
);
```

**Step 2**: Create PostgreSQL extras (`000003_add_feature_postgres.up.sql`)
```sql
-- Convert types for PostgreSQL
ALTER TABLE new_feature ALTER COLUMN uuid TYPE UUID USING uuid::uuid;
ALTER TABLE new_feature ALTER COLUMN enabled TYPE BOOLEAN USING enabled::boolean;
```

**Step 3**: Create SQLite extras (`000003_add_feature_sqlite.up.sql`)
```sql
-- No changes needed (core schema is SQLite-compatible)
```

## Type Compatibility

### Core Schema Types

| Core Type | PostgreSQL Result | SQLite Result |
|-----------|------------------|---------------|
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `BIGSERIAL` | `INTEGER` |
| `TEXT` | `TEXT` | `TEXT` |
| `INTEGER` | `INTEGER` | `INTEGER` |
| `TIMESTAMP` | `TIMESTAMP` | `TEXT` (ISO8601) |

### PostgreSQL Extras

Convert generic types to PostgreSQL-specific types:

- `TEXT` → `UUID` (for UUIDs)
- `TEXT` → `CITEXT` (for case-insensitive strings like emails)
- `INTEGER` → `BOOLEAN` (for boolean flags)

### SQLite Extras

Configure database settings:

```sql
PRAGMA foreign_keys = ON;        -- Enable FK constraints
PRAGMA journal_mode = WAL;       -- Better concurrency
PRAGMA synchronous = NORMAL;     -- Performance vs safety balance
```

## Migration Workflow

### Development

```bash
# Start PostgreSQL (via Docker)
make docker/postgres/start

# Run migrations
./hermes server -config=testing/config.hcl

# Check version
psql -h localhost -p 5433 -U postgres -d hermes_testing \
  -c "SELECT version, dirty FROM schema_migrations;"
```

### Testing with SQLite

```bash
# Create SQLite config
cat > config-sqlite.hcl <<EOF
database {
  driver = "sqlite"
  path   = ".hermes/hermes.db"
}
EOF

# Run migrations
./hermes server -config=config-sqlite.hcl

# Check version
sqlite3 .hermes/hermes.db "SELECT version, dirty FROM schema_migrations;"
```

### Production Rollout

```bash
# 1. Backup database
pg_dump -h dbhost -U postgres hermes > backup-$(date +%Y%m%d).sql

# 2. Run migration
./hermes migrate -config=production-config.hcl

# 3. Verify
psql -h dbhost -U postgres hermes \
  -c "SELECT version FROM schema_migrations ORDER BY version;"

# 4. Rollback if needed
./hermes migrate down -config=production-config.hcl
```

## Troubleshooting

### Migration Failed: "dirty database"

If a migration fails mid-execution:

```bash
# Check current state
psql hermes -c "SELECT version, dirty FROM schema_migrations;"

# If dirty=true, you need to manually fix:
# 1. Review failed SQL in migration file
# 2. Fix any partially applied changes
# 3. Mark migration as clean:
psql hermes -c "UPDATE schema_migrations SET dirty=false WHERE version=X;"

# Or force version:
./hermes migrate force X -config=config.hcl
```

### Different Schema Between PostgreSQL and SQLite

This shouldn't happen if you follow the core + extras pattern. If it does:

1. **Check core migration** - Should only use compatible types
2. **Check extras migration** - Should handle type conversions
3. **Regenerate schema** - Drop database and re-run all migrations

```bash
# PostgreSQL
dropdb hermes_testing && createdb hermes_testing
./hermes server -config=testing/config.hcl

# SQLite
rm .hermes/hermes.db
./hermes server -config=config-sqlite.hcl
```

## Best Practices

### ✅ DO

- Use `TEXT` in core migrations, convert to `UUID`/`CITEXT` in PostgreSQL extras
- Use `INTEGER` for booleans in core, convert to `BOOLEAN` in PostgreSQL extras
- Add all indexes and foreign keys in core migrations
- Test migrations on both PostgreSQL AND SQLite before committing
- Write idempotent SQL (`IF NOT EXISTS`, `IF EXISTS`)

### ❌ DON'T

- Don't use PostgreSQL-specific types in core migrations (`UUID`, `CITEXT`, `JSONB`)
- Don't use SQLite-specific syntax in core migrations
- Don't mix schema changes and data migrations
- Don't skip down migrations (always write rollback SQL)

## Performance Considerations

### PostgreSQL

Extensions enabled:
- `uuid-ossp` - Fast UUID generation
- `citext` - Case-insensitive text comparison

Indexes:
- All foreign keys have indexes
- Deleted_at columns have indexes (soft deletes)
- UUID columns have unique indexes

### SQLite

Optimizations:
- WAL mode for better concurrency
- mmap_size increased for performance
- Foreign keys enabled by default

Limitations:
- Single writer (fine for local development)
- No ALTER TABLE for type changes (requires table recreation)

## References

- [golang-migrate Documentation](https://github.com/golang-migrate/migrate)
- [PostgreSQL Data Types](https://www.postgresql.org/docs/current/datatype.html)
- [SQLite Data Types](https://www.sqlite.org/datatype3.html)
- [Hermes Migration Guide](../../docs-internal/DATABASE_MIGRATION_REFACTORING_SUMMARY.md)
