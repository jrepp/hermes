#!/bin/bash
# RFC-088 Production Deployment Validation Script
# This script validates that the semantic search deployment is production-ready

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
PASSED=0
FAILED=0
WARNINGS=0

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-hermes}"
DB_NAME="${DB_NAME:-hermes}"
API_URL="${API_URL:-http://localhost:8080}"

echo "================================================"
echo "RFC-088 Production Deployment Validation"
echo "================================================"
echo ""
echo "Configuration:"
echo "  Database: ${DB_HOST}:${DB_PORT}/${DB_NAME}"
echo "  API URL:  ${API_URL}"
echo ""

# Helper functions
pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASSED++))
}

fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAILED++))
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
    ((WARNINGS++))
}

info() {
    echo "  $1"
}

section() {
    echo ""
    echo "--- $1 ---"
}

# Check if psql is available
if ! command -v psql &> /dev/null; then
    fail "psql command not found. Please install PostgreSQL client."
    exit 1
fi

# Check if curl is available
if ! command -v curl &> /dev/null; then
    fail "curl command not found. Please install curl."
    exit 1
fi

# Function to run SQL query
run_sql() {
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -A -c "$1" 2>/dev/null
}

# 1. Database Connectivity
section "Database Connectivity"

if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" &>/dev/null; then
    pass "Database connection successful"
else
    fail "Cannot connect to database"
    exit 1
fi

# 2. pgvector Extension
section "pgvector Extension"

PGVECTOR_VERSION=$(run_sql "SELECT extversion FROM pg_extension WHERE extname = 'vector';")
if [ -n "$PGVECTOR_VERSION" ]; then
    pass "pgvector extension installed (version: $PGVECTOR_VERSION)"
else
    fail "pgvector extension not installed"
    info "Install with: CREATE EXTENSION IF NOT EXISTS vector;"
fi

# 3. Database Tables
section "Database Tables"

if run_sql "SELECT 1 FROM information_schema.tables WHERE table_name = 'document_embeddings';" | grep -q 1; then
    pass "document_embeddings table exists"

    # Check row count
    ROW_COUNT=$(run_sql "SELECT COUNT(*) FROM document_embeddings;")
    if [ "$ROW_COUNT" -gt 0 ]; then
        pass "document_embeddings has data ($ROW_COUNT rows)"
    else
        warn "document_embeddings table is empty (no embeddings yet)"
    fi
else
    fail "document_embeddings table not found"
    info "Run migrations: make migrate"
fi

# 4. Database Indexes
section "Database Indexes"

# Check for vector index (IVFFlat or HNSW)
VECTOR_INDEX=$(run_sql "SELECT indexname FROM pg_indexes WHERE tablename = 'document_embeddings' AND indexdef LIKE '%ivfflat%';")
if [ -n "$VECTOR_INDEX" ]; then
    pass "IVFFlat vector index exists: $VECTOR_INDEX"
else
    HNSW_INDEX=$(run_sql "SELECT indexname FROM pg_indexes WHERE tablename = 'document_embeddings' AND indexdef LIKE '%hnsw%';")
    if [ -n "$HNSW_INDEX" ]; then
        pass "HNSW vector index exists: $HNSW_INDEX"
    else
        fail "No vector index found (IVFFlat or HNSW)"
        info "Create with: CREATE INDEX idx_embeddings_vector_ivfflat ON document_embeddings USING ivfflat (embedding_vector vector_cosine_ops) WITH (lists = 100);"
    fi
fi

# Check for lookup index
LOOKUP_INDEX=$(run_sql "SELECT indexname FROM pg_indexes WHERE tablename = 'document_embeddings' AND indexdef LIKE '%document_id%' AND indexdef LIKE '%model%';")
if [ -n "$LOOKUP_INDEX" ]; then
    pass "Document lookup index exists: $LOOKUP_INDEX"
else
    warn "Document lookup index not found"
    info "Recommended: CREATE INDEX idx_embeddings_lookup ON document_embeddings (document_id, model);"
fi

# 5. Database Statistics
section "Database Statistics"

LAST_ANALYZE=$(run_sql "SELECT last_analyze::date FROM pg_stat_user_tables WHERE schemaname = 'public' AND relname = 'document_embeddings';")
if [ -n "$LAST_ANALYZE" ] && [ "$LAST_ANALYZE" != "" ]; then
    pass "Statistics up to date (last analyzed: $LAST_ANALYZE)"
else
    warn "Table statistics may be stale"
    info "Run: ANALYZE document_embeddings;"
fi

# 6. PostgreSQL Configuration
section "PostgreSQL Configuration"

SHARED_BUFFERS=$(run_sql "SHOW shared_buffers;")
WORK_MEM=$(run_sql "SHOW work_mem;")
MAX_CONNECTIONS=$(run_sql "SHOW max_connections;")

info "shared_buffers: $SHARED_BUFFERS"
info "work_mem: $WORK_MEM"
info "max_connections: $MAX_CONNECTIONS"

# Check if shared_buffers is reasonable (at least 1GB for production)
SHARED_BUFFERS_MB=$(echo "$SHARED_BUFFERS" | sed 's/[^0-9]*//g')
if [ "$SHARED_BUFFERS_MB" -ge 1024 ]; then
    pass "shared_buffers configured appropriately (${SHARED_BUFFERS})"
else
    warn "shared_buffers may be too low for production (${SHARED_BUFFERS})"
    info "Recommended: 4GB (25% of available RAM)"
fi

# 7. API Health Check
section "API Health Check"

if curl -sf "${API_URL}/health" &>/dev/null; then
    pass "API health endpoint responding"

    # Check health details
    HEALTH=$(curl -s "${API_URL}/health")
    info "Health response: $HEALTH"
else
    fail "API health endpoint not responding"
    info "Check if API server is running"
fi

# 8. API Readiness Check
section "API Readiness"

if curl -sf "${API_URL}/ready" &>/dev/null; then
    pass "API ready endpoint responding"
else
    warn "API ready endpoint not responding or not ready"
    info "Check API logs for startup issues"
fi

# 9. Metrics Endpoint
section "Metrics Endpoint"

METRICS_PORT="${METRICS_PORT:-9090}"
METRICS_URL="http://localhost:${METRICS_PORT}/metrics"

if curl -sf "${METRICS_URL}" | grep -q "http_requests_total"; then
    pass "Prometheus metrics endpoint responding"

    # Check for specific metrics
    if curl -s "${METRICS_URL}" | grep -q "db_connections_open"; then
        pass "Database connection pool metrics available"
    else
        warn "Database connection pool metrics not found"
    fi

    if curl -s "${METRICS_URL}" | grep -q "search_semantic_total"; then
        pass "Search metrics available"
    else
        warn "Search metrics not found (may not have processed any searches yet)"
    fi
else
    warn "Prometheus metrics endpoint not accessible"
    info "Metrics URL: ${METRICS_URL}"
fi

# 10. Environment Variables
section "Environment Variables"

if [ -n "$OPENAI_API_KEY" ]; then
    pass "OPENAI_API_KEY is set"
else
    warn "OPENAI_API_KEY environment variable not set"
    info "Semantic search requires OpenAI API key"
fi

# 11. Performance Test (if embeddings exist)
section "Performance Test"

if [ "$ROW_COUNT" -gt 0 ]; then
    info "Running sample query to check performance..."

    # Get a sample embedding
    SAMPLE_VECTOR=$(run_sql "SELECT embedding_vector::text FROM document_embeddings LIMIT 1;")

    if [ -n "$SAMPLE_VECTOR" ]; then
        # Time the query
        START=$(date +%s%N)
        run_sql "SELECT document_id FROM document_embeddings ORDER BY embedding_vector <=> '${SAMPLE_VECTOR}'::vector LIMIT 10;" &>/dev/null
        END=$(date +%s%N)
        DURATION_MS=$(( (END - START) / 1000000 ))

        if [ "$DURATION_MS" -lt 100 ]; then
            pass "Vector search performance: ${DURATION_MS}ms (excellent)"
        elif [ "$DURATION_MS" -lt 500 ]; then
            pass "Vector search performance: ${DURATION_MS}ms (good)"
        elif [ "$DURATION_MS" -lt 1000 ]; then
            warn "Vector search performance: ${DURATION_MS}ms (acceptable, but could be optimized)"
            info "Consider adding/tuning indexes"
        else
            fail "Vector search performance: ${DURATION_MS}ms (too slow)"
            info "Add vector index: CREATE INDEX idx_embeddings_vector_ivfflat ON document_embeddings USING ivfflat (embedding_vector vector_cosine_ops) WITH (lists = 100);"
        fi
    fi
else
    info "Skipping performance test (no embeddings)"
fi

# 12. Index Usage Statistics
section "Index Usage Statistics"

if [ "$ROW_COUNT" -gt 0 ]; then
    INDEX_STATS=$(run_sql "SELECT indexname, idx_scan FROM pg_stat_user_indexes WHERE tablename = 'document_embeddings' ORDER BY idx_scan DESC;")

    if [ -n "$INDEX_STATS" ]; then
        info "Index usage:"
        echo "$INDEX_STATS" | while IFS='|' read -r indexname scans; do
            if [ -n "$indexname" ]; then
                info "  $indexname: $scans scans"
            fi
        done

        # Check if vector index is being used
        VECTOR_INDEX_SCANS=$(run_sql "SELECT COALESCE(SUM(idx_scan), 0) FROM pg_stat_user_indexes WHERE tablename = 'document_embeddings' AND (indexname LIKE '%vector%' OR indexname LIKE '%ivfflat%' OR indexname LIKE '%hnsw%');")

        if [ "$VECTOR_INDEX_SCANS" -gt 0 ]; then
            pass "Vector index is being used ($VECTOR_INDEX_SCANS scans)"
        else
            warn "Vector index exists but not being used yet"
            info "Index will be used after queries are executed"
        fi
    fi
fi

# Summary
echo ""
echo "================================================"
echo "Validation Summary"
echo "================================================"
echo -e "${GREEN}Passed:${NC}   $PASSED"
echo -e "${YELLOW}Warnings:${NC} $WARNINGS"
echo -e "${RED}Failed:${NC}   $FAILED"
echo ""

if [ "$FAILED" -eq 0 ]; then
    if [ "$WARNINGS" -eq 0 ]; then
        echo -e "${GREEN}✓ Production deployment is fully validated!${NC}"
        exit 0
    else
        echo -e "${YELLOW}⚠ Production deployment validated with warnings.${NC}"
        echo "  Review warnings above and address if necessary."
        exit 0
    fi
else
    echo -e "${RED}✗ Production deployment validation failed.${NC}"
    echo "  Fix the failed items above before deploying to production."
    exit 1
fi
