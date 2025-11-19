#!/bin/bash
# RFC-088 Phase 2: Enhanced Deployment Validation Script
# This script runs comprehensive validation beyond the basic production validation

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
API_URL="${API_URL:-http://localhost:8000}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-hermes}"
DB_NAME="${DB_NAME:-hermes}"
REDPANDA_BROKER="${REDPANDA_BROKER:-localhost:19092}"
MEILISEARCH_URL="${MEILISEARCH_URL:-http://localhost:7700}"

# Counters
PASSED=0
FAILED=0
WARNINGS=0

echo "================================================"
echo "RFC-088 Phase 2: Deployment Validation"
echo "================================================"
echo ""
echo "Configuration:"
echo "  API URL:         $API_URL"
echo "  Database:        ${DB_HOST}:${DB_PORT}/${DB_NAME}"
echo "  Redpanda:        $REDPANDA_BROKER"
echo "  Meilisearch:     $MEILISEARCH_URL"
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

run_sql() {
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -A -c "$1" 2>/dev/null
}

# Run basic validation first
section "Running Basic Production Validation"

if [ -f "./scripts/validate-production-deployment.sh" ]; then
    if ./scripts/validate-production-deployment.sh; then
        pass "Basic production validation passed"
    else
        fail "Basic production validation failed"
        exit 1
    fi
else
    warn "Basic validation script not found"
fi

# Phase 2: API Endpoint Testing
section "API Endpoint Testing"

# Test semantic search endpoint
info "Testing POST /api/v2/search/semantic..."
SEMANTIC_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${API_URL}/api/v2/search/semantic" \
    -H "Content-Type: application/json" \
    -d '{"query": "kubernetes deployment", "limit": 10}' 2>/dev/null || echo "000")

HTTP_CODE=$(echo "$SEMANTIC_RESPONSE" | tail -n1)
if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
    pass "Semantic search endpoint responding (HTTP $HTTP_CODE)"
    if [ "$HTTP_CODE" = "401" ]; then
        info "  Note: Authentication required (expected in production)"
    fi
else
    fail "Semantic search endpoint error (HTTP $HTTP_CODE)"
fi

# Test hybrid search endpoint
info "Testing POST /api/v2/search/hybrid..."
HYBRID_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${API_URL}/api/v2/search/hybrid" \
    -H "Content-Type: application/json" \
    -d '{"query": "kubernetes deployment", "limit": 10}' 2>/dev/null || echo "000")

HTTP_CODE=$(echo "$HYBRID_RESPONSE" | tail -n1)
if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
    pass "Hybrid search endpoint responding (HTTP $HTTP_CODE)"
else
    fail "Hybrid search endpoint error (HTTP $HTTP_CODE)"
fi

# Test similar documents endpoint (with dummy ID)
info "Testing GET /api/v2/documents/{id}/similar..."
SIMILAR_RESPONSE=$(curl -s -w "\n%{http_code}" "${API_URL}/api/v2/documents/test-doc-id/similar?limit=10" 2>/dev/null || echo "000")

HTTP_CODE=$(echo "$SIMILAR_RESPONSE" | tail -n1)
if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "404" ]; then
    pass "Similar documents endpoint responding (HTTP $HTTP_CODE)"
else
    fail "Similar documents endpoint error (HTTP $HTTP_CODE)"
fi

# Phase 2: Kafka/Redpanda Testing
section "Kafka/Redpanda Testing"

if command -v rpk &> /dev/null; then
    info "Testing Redpanda cluster health..."
    if rpk cluster health --brokers "$REDPANDA_BROKER" 2>&1 | grep -q "Healthy"; then
        pass "Redpanda cluster healthy"
    else
        fail "Redpanda cluster unhealthy"
    fi

    info "Checking document-revisions topic..."
    if rpk topic list --brokers "$REDPANDA_BROKER" 2>&1 | grep -q "document-revisions"; then
        pass "document-revisions topic exists"

        # Get topic details
        LAG=$(rpk group describe hermes-indexer-workers --brokers "$REDPANDA_BROKER" 2>&1 | grep -oP 'LAG\s+\K\d+' || echo "0")
        if [ "$LAG" -lt 1000 ]; then
            pass "Consumer lag is acceptable ($LAG messages)"
        else
            warn "Consumer lag is high ($LAG messages)"
        fi
    else
        fail "document-revisions topic not found"
        info "  Create with: rpk topic create hermes.document-revisions --brokers $REDPANDA_BROKER"
    fi
else
    warn "rpk command not found, skipping Kafka tests"
    info "  Install with: curl -LO https://github.com/redpanda-data/redpanda/releases/latest/download/rpk-linux-amd64.zip && unzip rpk-linux-amd64.zip"
fi

# Phase 2: Meilisearch Testing
section "Meilisearch Testing"

if curl -sf "${MEILISEARCH_URL}/health" > /dev/null; then
    pass "Meilisearch is healthy"

    # Check indexes
    info "Checking Meilisearch indexes..."
    INDEXES=$(curl -s "${MEILISEARCH_URL}/indexes" -H "Authorization: Bearer ${MEILISEARCH_KEY}" 2>/dev/null || echo "")

    if echo "$INDEXES" | grep -q "hermes"; then
        pass "Hermes indexes exist in Meilisearch"
        info "  Indexes: $(echo "$INDEXES" | jq -r '.results[].uid' 2>/dev/null || echo 'unable to parse')"
    else
        warn "No Hermes indexes found in Meilisearch"
    fi
else
    fail "Meilisearch is not responding"
fi

# Phase 2: Database Performance Testing
section "Database Performance Testing"

ROW_COUNT=$(run_sql "SELECT COUNT(*) FROM document_embeddings;")
if [ "$ROW_COUNT" -gt 0 ]; then
    info "Testing vector search performance with $ROW_COUNT embeddings..."

    # Test 1: Simple vector search
    SAMPLE_VECTOR=$(run_sql "SELECT embedding_vector::text FROM document_embeddings LIMIT 1;")
    if [ -n "$SAMPLE_VECTOR" ]; then
        START=$(date +%s%N)
        run_sql "SELECT document_id FROM document_embeddings ORDER BY embedding_vector <=> '${SAMPLE_VECTOR}'::vector LIMIT 10;" > /dev/null
        END=$(date +%s%N)
        DURATION_MS=$(( (END - START) / 1000000 ))

        if [ "$DURATION_MS" -lt 50 ]; then
            pass "Vector search performance: ${DURATION_MS}ms (excellent)"
        elif [ "$DURATION_MS" -lt 200 ]; then
            pass "Vector search performance: ${DURATION_MS}ms (good)"
        elif [ "$DURATION_MS" -lt 500 ]; then
            warn "Vector search performance: ${DURATION_MS}ms (acceptable)"
        else
            fail "Vector search performance: ${DURATION_MS}ms (needs optimization)"
        fi

        # Test 2: Filtered vector search
        START=$(date +%s%N)
        run_sql "SELECT document_id FROM document_embeddings WHERE model = 'text-embedding-3-small' ORDER BY embedding_vector <=> '${SAMPLE_VECTOR}'::vector LIMIT 10;" > /dev/null
        END=$(date +%s%N)
        DURATION_MS=$(( (END - START) / 1000000 ))

        if [ "$DURATION_MS" -lt 100 ]; then
            pass "Filtered vector search: ${DURATION_MS}ms (good)"
        elif [ "$DURATION_MS" -lt 300 ]; then
            warn "Filtered vector search: ${DURATION_MS}ms (could be optimized)"
        else
            fail "Filtered vector search: ${DURATION_MS}ms (slow)"
        fi
    fi
else
    info "Skipping performance tests (no embeddings yet)"
fi

# Phase 2: Connection Pool Testing
section "Connection Pool Testing"

ACTIVE_CONNS=$(run_sql "SELECT count(*) FROM pg_stat_activity WHERE datname = '${DB_NAME}';")
MAX_CONNS=$(run_sql "SHOW max_connections;")

info "Active connections: $ACTIVE_CONNS / $MAX_CONNS"
if [ "$ACTIVE_CONNS" -lt $((MAX_CONNS / 2)) ]; then
    pass "Connection pool usage is healthy"
else
    warn "Connection pool usage is high ($ACTIVE_CONNS / $MAX_CONNS)"
fi

# Phase 2: Index Health Check
section "Index Health Check"

INDEX_BLOAT=$(run_sql "SELECT schemaname, tablename, pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS total_size FROM pg_tables WHERE tablename = 'document_embeddings';")
info "Table size: $(echo "$INDEX_BLOAT" | awk '{print $3}')"

# Check index usage
INDEX_USAGE=$(run_sql "SELECT indexname, idx_scan FROM pg_stat_user_indexes WHERE tablename = 'document_embeddings' AND idx_scan > 0 ORDER BY idx_scan DESC;")
if [ -n "$INDEX_USAGE" ]; then
    pass "Indexes are being used"
    info "  Most used indexes:"
    echo "$INDEX_USAGE" | head -3 | while IFS='|' read -r indexname scans; do
        if [ -n "$indexname" ]; then
            info "    $indexname: $scans scans"
        fi
    done
else
    warn "No index usage statistics (may need more queries)"
fi

# Phase 2: OpenAI API Testing
section "OpenAI API Testing"

if [ -n "$OPENAI_API_KEY" ]; then
    info "Testing OpenAI API connectivity..."
    OPENAI_RESPONSE=$(curl -s -w "\n%{http_code}" https://api.openai.com/v1/models \
        -H "Authorization: Bearer $OPENAI_API_KEY" 2>/dev/null || echo "000")

    HTTP_CODE=$(echo "$OPENAI_RESPONSE" | tail -n1)
    if [ "$HTTP_CODE" = "200" ]; then
        pass "OpenAI API is accessible"

        # Check if embedding model is available
        if echo "$OPENAI_RESPONSE" | grep -q "text-embedding-3-small"; then
            pass "text-embedding-3-small model is available"
        else
            warn "text-embedding-3-small model not found in available models"
        fi
    else
        fail "OpenAI API error (HTTP $HTTP_CODE)"
    fi
else
    warn "OPENAI_API_KEY not set, skipping OpenAI API test"
fi

# Phase 2: Metrics Endpoint Testing
section "Metrics Endpoint Testing"

METRICS_PORT="${METRICS_PORT:-9090}"
METRICS_URL="http://localhost:${METRICS_PORT}/metrics"

if curl -sf "$METRICS_URL" > /dev/null; then
    pass "Prometheus metrics endpoint is accessible"

    # Check for specific metrics
    METRICS_CONTENT=$(curl -s "$METRICS_URL")

    if echo "$METRICS_CONTENT" | grep -q "hermes_"; then
        pass "Hermes-specific metrics are being exported"

        # Count metrics
        METRIC_COUNT=$(echo "$METRICS_CONTENT" | grep -c "^hermes_" || echo "0")
        info "  Total Hermes metrics: $METRIC_COUNT"

        # Check for RFC-088 specific metrics
        if echo "$METRICS_CONTENT" | grep -q "semantic_search"; then
            pass "Semantic search metrics are available"
        else
            warn "Semantic search metrics not found (may need queries first)"
        fi
    else
        warn "No Hermes-specific metrics found"
    fi
else
    warn "Metrics endpoint not accessible at $METRICS_URL"
fi

# Phase 2: Configuration Validation
section "Configuration Validation"

info "Checking configuration files..."
CONFIG_FILES=(
    "./config-production/config.hcl"
    "./config-production/indexer-worker.hcl"
)

for config_file in "${CONFIG_FILES[@]}"; do
    if [ -f "$config_file" ]; then
        pass "Config file exists: $config_file"

        # Check for required settings
        if grep -q "enable_semantic_search.*true" "$config_file" 2>/dev/null; then
            pass "  Semantic search enabled in config"
        fi

        if grep -q "text-embedding-3-small" "$config_file" 2>/dev/null; then
            pass "  Using recommended embedding model"
        fi
    else
        warn "Config file not found: $config_file"
    fi
done

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
        echo -e "${GREEN}✓ Phase 2 validation completed successfully!${NC}"
        echo ""
        info "Next Steps:"
        echo "  1. Run Phase 3 monitoring setup: ./scripts/deployment/phase3-monitoring.sh"
        echo "  2. Test API endpoints with real data"
        echo "  3. Monitor performance metrics"
        exit 0
    else
        echo -e "${YELLOW}⚠ Phase 2 validation completed with warnings.${NC}"
        echo "  Review warnings above and address if necessary."
        echo ""
        info "Next Steps:"
        echo "  1. Address any warnings if critical"
        echo "  2. Run Phase 3 monitoring setup: ./scripts/deployment/phase3-monitoring.sh"
        exit 0
    fi
else
    echo -e "${RED}✗ Phase 2 validation failed.${NC}"
    echo "  Fix the failed items above before proceeding."
    exit 1
fi
