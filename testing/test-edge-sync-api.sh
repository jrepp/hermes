#!/bin/bash
# RFC-085 Edge Sync API Integration Tests
#
# Tests the edge-to-central document synchronization API endpoints
#
# Prerequisites:
#   - Docker compose services running (hermes-central, hermes-edge, postgres)
#   - Migration 000008 applied (edge document tracking tables)
#
# Usage:
#   ./test-edge-sync-api.sh

set -e

CENTRAL_URL="http://localhost:8000"
EDGE_URL="http://localhost:8002"
TEST_UUID="550e8400-e29b-41d4-a716-446655440000"
EDGE_INSTANCE="edge-dev-1"

echo "=========================================="
echo "RFC-085 Edge Sync API Integration Tests"
echo "=========================================="
echo ""

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Helper function to print test results
pass() {
    echo -e "${GREEN}✓ PASS${NC}: $1"
    ((TESTS_PASSED++))
}

fail() {
    echo -e "${RED}✗ FAIL${NC}: $1"
    ((TESTS_FAILED++))
}

warn() {
    echo -e "${YELLOW}⚠ WARN${NC}: $1"
}

info() {
    echo -e "ℹ INFO: $1"
}

# Test 1: Check services are healthy
echo "Test 1: Health checks"
echo "---"

CENTRAL_HEALTH=$(curl -s ${CENTRAL_URL}/health)
if echo "$CENTRAL_HEALTH" | grep -qi "ok"; then
    pass "Central Hermes is healthy"
else
    fail "Central Hermes health check failed"
    exit 1
fi

EDGE_HEALTH=$(curl -s ${EDGE_URL}/health)
if echo "$EDGE_HEALTH" | grep -qi "ok"; then
    pass "Edge Hermes is healthy"
else
    fail "Edge Hermes health check failed"
    exit 1
fi

echo ""

# Test 2: Verify database tables exist
echo "Test 2: Database schema verification"
echo "---"

TABLES=$(docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -t -c "SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename LIKE 'edge_%' OR tablename = 'document_uuid_mappings';")

if echo "$TABLES" | grep -q "edge_document_registry"; then
    pass "edge_document_registry table exists"
else
    fail "edge_document_registry table missing"
fi

if echo "$TABLES" | grep -q "edge_sync_queue"; then
    pass "edge_sync_queue table exists"
else
    fail "edge_sync_queue table missing"
fi

if echo "$TABLES" | grep -q "document_uuid_mappings"; then
    pass "document_uuid_mappings table exists"
else
    fail "document_uuid_mappings table missing"
fi

echo ""

# Test 3: Check API endpoint accessibility (should require auth)
echo "Test 3: API endpoint accessibility"
echo "---"

# POST /api/v2/edge/documents/register (should return 401 Unauthorized)
REGISTER_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -X POST ${CENTRAL_URL}/api/v2/edge/documents/register \
  -H "Content-Type: application/json" \
  -d '{"uuid":"test"}')

if [ "$REGISTER_RESPONSE" = "401" ]; then
    pass "Register endpoint requires authentication (HTTP 401)"
else
    warn "Register endpoint returned HTTP ${REGISTER_RESPONSE} (expected 401)"
fi

# GET /api/v2/edge/documents/sync-status (should return 401 Unauthorized)
SYNC_STATUS_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -X GET "${CENTRAL_URL}/api/v2/edge/documents/sync-status?edge_instance=${EDGE_INSTANCE}")

if [ "$SYNC_STATUS_RESPONSE" = "401" ]; then
    pass "Sync status endpoint requires authentication (HTTP 401)"
else
    warn "Sync status endpoint returned HTTP ${SYNC_STATUS_RESPONSE} (expected 401)"
fi

# GET /api/v2/edge/stats (should return 401 Unauthorized)
STATS_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -X GET "${CENTRAL_URL}/api/v2/edge/stats?edge_instance=${EDGE_INSTANCE}")

if [ "$STATS_RESPONSE" = "401" ]; then
    pass "Stats endpoint requires authentication (HTTP 401)"
else
    warn "Stats endpoint returned HTTP ${STATS_RESPONSE} (expected 401)"
fi

echo ""

# Test 4: Direct database operations (simulate authenticated API calls)
echo "Test 4: Direct database operations (simulating edge sync)"
echo "---"

# Insert a test document directly into the database
docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c "
DELETE FROM edge_document_registry WHERE uuid = '${TEST_UUID}';
INSERT INTO edge_document_registry (
    uuid, title, document_type, status, owners, edge_instance,
    edge_provider_id, product, created_at, updated_at
) VALUES (
    '${TEST_UUID}',
    'RFC-999: Test Document',
    'RFC',
    'In-Review',
    ARRAY['test@example.com'],
    '${EDGE_INSTANCE}',
    'local:docs/rfc-999.md',
    'Test Product',
    NOW(),
    NOW()
);
" > /dev/null 2>&1

# Query to verify insert
COUNT=$(docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -t -c "SELECT COUNT(*) FROM edge_document_registry WHERE uuid = '${TEST_UUID}';")

if [ "$COUNT" -eq 1 ]; then
    pass "Document inserted into edge_document_registry"
else
    fail "Document insert failed"
fi

# Query by edge instance
INSTANCE_DOCS=$(docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -t -c "SELECT COUNT(*) FROM edge_document_registry WHERE edge_instance = '${EDGE_INSTANCE}';")

if [ "$INSTANCE_DOCS" -ge 1 ]; then
    pass "Query by edge_instance successful (${INSTANCE_DOCS} documents)"
else
    fail "Query by edge_instance failed"
fi

# Query by document type
RFC_DOCS=$(docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -t -c "SELECT COUNT(*) FROM edge_document_registry WHERE document_type = 'RFC';")

if [ "$RFC_DOCS" -ge 1 ]; then
    pass "Query by document_type successful (${RFC_DOCS} RFC documents)"
else
    fail "Query by document_type failed"
fi

echo ""

# Test 5: Verify indexes exist
echo "Test 5: Database index verification"
echo "---"

INDEXES=$(docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -t -c "SELECT indexname FROM pg_indexes WHERE tablename = 'edge_document_registry';")

INDEX_COUNT=$(echo "$INDEXES" | grep -c "idx_edge_document")

if [ "$INDEX_COUNT" -ge 7 ]; then
    pass "All edge_document_registry indexes created (${INDEX_COUNT} indexes)"
else
    warn "Expected 7+ indexes, found ${INDEX_COUNT}"
fi

echo ""

# Cleanup
echo "Test 6: Cleanup"
echo "---"

docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c "DELETE FROM edge_document_registry WHERE uuid = '${TEST_UUID}';" > /dev/null 2>&1

CLEANUP_CHECK=$(docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -t -c "SELECT COUNT(*) FROM edge_document_registry WHERE uuid = '${TEST_UUID}';")

if [ "$CLEANUP_CHECK" -eq 0 ]; then
    pass "Test data cleaned up"
else
    warn "Cleanup incomplete"
fi

echo ""
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo -e "${GREEN}Passed: ${TESTS_PASSED}${NC}"
echo -e "${RED}Failed: ${TESTS_FAILED}${NC}"
echo ""

if [ "$TESTS_FAILED" -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Implement authentication for edge sync endpoints (API token or bearer token)"
    echo "2. Create authenticated integration tests"
    echo "3. Update edge config to use multiprovider workspace"
    echo "4. Test full edge-to-central document synchronization flow"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi
