#!/bin/bash
# RFC-088 Phase 1: Database Setup Script
# This script sets up PostgreSQL with pgvector extension, runs migrations, and creates indexes

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-hermes}"
DB_NAME="${DB_NAME:-hermes}"
DB_PASSWORD="${DB_PASSWORD:-}"
INDEX_TYPE="${INDEX_TYPE:-ivfflat}" # ivfflat or hnsw
IVFFLAT_LISTS="${IVFFLAT_LISTS:-100}"
HNSW_M="${HNSW_M:-16}"
HNSW_EF_CONSTRUCTION="${HNSW_EF_CONSTRUCTION:-64}"

echo "================================================"
echo "RFC-088 Phase 1: Database Setup"
echo "================================================"
echo ""
echo "Configuration:"
echo "  Database: ${DB_HOST}:${DB_PORT}/${DB_NAME}"
echo "  User:     ${DB_USER}"
echo "  Index:    ${INDEX_TYPE}"
echo ""

# Helper functions
info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
    exit 1
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Function to run SQL query
run_sql() {
    local query="$1"
    if [ -n "$DB_PASSWORD" ]; then
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -A -c "$query" 2>&1
    else
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -A -c "$query" 2>&1
    fi
}

# Function to run SQL file
run_sql_file() {
    local file="$1"
    if [ -n "$DB_PASSWORD" ]; then
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$file" 2>&1
    else
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$file" 2>&1
    fi
}

# Step 1: Check PostgreSQL connectivity
info "Step 1: Checking PostgreSQL connectivity..."
if run_sql "SELECT 1;" > /dev/null; then
    success "Connected to PostgreSQL"
else
    error "Cannot connect to PostgreSQL at ${DB_HOST}:${DB_PORT}"
fi

# Step 2: Check PostgreSQL version
info "Step 2: Checking PostgreSQL version..."
PG_VERSION=$(run_sql "SHOW server_version;" | cut -d'.' -f1)
if [ "$PG_VERSION" -ge 11 ]; then
    success "PostgreSQL version: $PG_VERSION (>= 11 required)"
else
    error "PostgreSQL version $PG_VERSION is too old. Version 11+ required."
fi

# Step 3: Install pgvector extension
info "Step 3: Installing pgvector extension..."
PGVECTOR_RESULT=$(run_sql "CREATE EXTENSION IF NOT EXISTS vector;")
if [ $? -eq 0 ]; then
    success "pgvector extension installed"
    PGVECTOR_VERSION=$(run_sql "SELECT extversion FROM pg_extension WHERE extname = 'vector';")
    info "  Version: $PGVECTOR_VERSION"
else
    error "Failed to install pgvector extension. Install it with: apt-get install postgresql-${PG_VERSION}-pgvector"
fi

# Step 4: Run database migrations
info "Step 4: Running database migrations..."
MIGRATIONS_DIR="$(dirname "$(dirname "$0")")/../internal/migrate/migrations"
if [ ! -d "$MIGRATIONS_DIR" ]; then
    error "Migrations directory not found: $MIGRATIONS_DIR"
fi

# Check if hermes-migrate binary exists
if command -v hermes-migrate &> /dev/null; then
    info "  Using hermes-migrate binary..."
    if [ -n "$DB_PASSWORD" ]; then
        DSN="host=${DB_HOST} port=${DB_PORT} user=${DB_USER} password=${DB_PASSWORD} dbname=${DB_NAME} sslmode=disable"
    else
        DSN="host=${DB_HOST} port=${DB_PORT} user=${DB_USER} dbname=${DB_NAME} sslmode=disable"
    fi
    hermes-migrate -driver=postgres -dsn="$DSN" up
    success "Migrations completed"
else
    warn "hermes-migrate binary not found, running migrations manually..."

    # Run each up migration in order
    for migration in $(ls -1 "$MIGRATIONS_DIR"/*.up.sql | sort -V); do
        migration_name=$(basename "$migration")
        info "  Running: $migration_name"
        if run_sql_file "$migration" > /dev/null; then
            success "    $migration_name completed"
        else
            error "    Failed to run $migration_name"
        fi
    done
    success "Manual migrations completed"
fi

# Step 5: Verify tables exist
info "Step 5: Verifying tables exist..."
TABLES=("document_embeddings" "document_summaries" "document_revision_outbox")
for table in "${TABLES[@]}"; do
    TABLE_EXISTS=$(run_sql "SELECT 1 FROM information_schema.tables WHERE table_name = '$table';")
    if [ -n "$TABLE_EXISTS" ]; then
        success "  Table exists: $table"

        # Check row count
        if [ "$table" = "document_embeddings" ]; then
            ROW_COUNT=$(run_sql "SELECT COUNT(*) FROM $table;")
            info "    Rows: $ROW_COUNT"
        fi
    else
        error "  Table not found: $table"
    fi
done

# Step 6: Create vector indexes
info "Step 6: Creating vector indexes..."

# Check if index already exists
INDEX_EXISTS=$(run_sql "SELECT indexname FROM pg_indexes WHERE tablename = 'document_embeddings' AND (indexname LIKE '%vector%' OR indexname LIKE '%ivfflat%' OR indexname LIKE '%hnsw%');")

if [ -n "$INDEX_EXISTS" ]; then
    warn "Vector index already exists: $INDEX_EXISTS"
    read -p "Do you want to recreate it? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        info "  Dropping existing index..."
        run_sql "DROP INDEX IF EXISTS $INDEX_EXISTS;" > /dev/null
        success "  Index dropped"
    else
        info "  Keeping existing index"
        INDEX_TYPE="skip"
    fi
fi

if [ "$INDEX_TYPE" = "ivfflat" ]; then
    info "  Creating IVFFlat index (lists=$IVFFLAT_LISTS)..."
    CREATE_INDEX_SQL="CREATE INDEX IF NOT EXISTS idx_embeddings_vector_ivfflat
        ON document_embeddings
        USING ivfflat (embedding_vector vector_cosine_ops)
        WITH (lists = $IVFFLAT_LISTS);"

    if run_sql "$CREATE_INDEX_SQL" > /dev/null 2>&1; then
        success "  IVFFlat index created"
    else
        error "  Failed to create IVFFlat index"
    fi

elif [ "$INDEX_TYPE" = "hnsw" ]; then
    info "  Creating HNSW index (m=$HNSW_M, ef_construction=$HNSW_EF_CONSTRUCTION)..."
    CREATE_INDEX_SQL="CREATE INDEX IF NOT EXISTS idx_embeddings_vector_hnsw
        ON document_embeddings
        USING hnsw (embedding_vector vector_cosine_ops)
        WITH (m = $HNSW_M, ef_construction = $HNSW_EF_CONSTRUCTION);"

    if run_sql "$CREATE_INDEX_SQL" > /dev/null 2>&1; then
        success "  HNSW index created"
    else
        error "  Failed to create HNSW index"
    fi

elif [ "$INDEX_TYPE" != "skip" ]; then
    error "Invalid index type: $INDEX_TYPE (must be 'ivfflat' or 'hnsw')"
fi

# Step 7: Create lookup indexes
info "Step 7: Creating lookup indexes..."
LOOKUP_INDEX_SQL="CREATE INDEX IF NOT EXISTS idx_embeddings_lookup
    ON document_embeddings (document_id, model);"

if run_sql "$LOOKUP_INDEX_SQL" > /dev/null 2>&1; then
    success "  Lookup index created"
else
    warn "  Lookup index already exists or failed to create"
fi

# Step 8: Update statistics
info "Step 8: Updating table statistics..."
if run_sql "ANALYZE document_embeddings;" > /dev/null 2>&1; then
    success "  Statistics updated"
else
    warn "  Failed to update statistics"
fi

# Step 9: Verify index usage
info "Step 9: Verifying indexes..."
INDEXES=$(run_sql "SELECT indexname FROM pg_indexes WHERE tablename = 'document_embeddings' ORDER BY indexname;")
if [ -n "$INDEXES" ]; then
    success "  Indexes on document_embeddings:"
    echo "$INDEXES" | while read -r idx; do
        if [ -n "$idx" ]; then
            info "    - $idx"
        fi
    done
else
    warn "  No indexes found on document_embeddings"
fi

# Step 10: Test vector query performance (if data exists)
info "Step 10: Testing vector query performance..."
ROW_COUNT=$(run_sql "SELECT COUNT(*) FROM document_embeddings;")
if [ "$ROW_COUNT" -gt 0 ]; then
    info "  Testing query on $ROW_COUNT embeddings..."

    # Get a sample embedding
    SAMPLE_VECTOR=$(run_sql "SELECT embedding_vector::text FROM document_embeddings LIMIT 1;")

    if [ -n "$SAMPLE_VECTOR" ]; then
        # Time the query
        START=$(date +%s%N)
        run_sql "SELECT document_id FROM document_embeddings ORDER BY embedding_vector <=> '${SAMPLE_VECTOR}'::vector LIMIT 10;" > /dev/null
        END=$(date +%s%N)
        DURATION_MS=$(( (END - START) / 1000000 ))

        if [ "$DURATION_MS" -lt 100 ]; then
            success "  Query performance: ${DURATION_MS}ms (excellent)"
        elif [ "$DURATION_MS" -lt 500 ]; then
            success "  Query performance: ${DURATION_MS}ms (good)"
        elif [ "$DURATION_MS" -lt 1000 ]; then
            warn "  Query performance: ${DURATION_MS}ms (acceptable)"
        else
            warn "  Query performance: ${DURATION_MS}ms (needs optimization)"
        fi
    fi
else
    info "  Skipping performance test (no embeddings yet)"
fi

# Summary
echo ""
echo "================================================"
echo "Phase 1 Setup Complete"
echo "================================================"
success "Database setup completed successfully!"
echo ""
info "Next Steps:"
echo "  1. Configure OpenAI API key: export OPENAI_API_KEY='sk-...'"
echo "  2. Run Phase 2: ./scripts/deployment/phase2-validation.sh"
echo ""
