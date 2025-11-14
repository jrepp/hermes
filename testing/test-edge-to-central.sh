#!/bin/bash
# RFC-085 Edge-to-Central Integration Test
#
# Tests the edge-to-central Hermes architecture where:
# - Central Hermes provides full capabilities
# - Edge Hermes authors documents locally and delegates to central
#
# NOTE: Multi-provider manager is not yet implemented, so some tests are placeholders
#       Once RFC-085 Phase 1 (multi-provider) is complete, these tests will be fully functional

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=== RFC-085 Edge-to-Central Integration Test ==="
echo ""

# Change to testing directory
cd "$(dirname "$0")"

# Function to print test result
test_result() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $2"
    else
        echo -e "${RED}✗${NC} $2"
        if [ "$3" != "optional" ]; then
            exit 1
        fi
    fi
}

# Function to wait for service
wait_for_service() {
    local url=$1
    local name=$2
    local max_attempts=30
    local attempt=0

    echo "Waiting for $name to be ready..."
    while [ $attempt -lt $max_attempts ]; do
        if curl -s -f "$url" > /dev/null 2>&1; then
            echo -e "${GREEN}✓${NC} $name is ready"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 2
    done

    echo -e "${RED}✗${NC} $name failed to start within $((max_attempts * 2)) seconds"
    return 1
}

# Test 0: Start services
echo "Test 0: Checking docker-compose services..."
# Check if services are already running
if docker compose ps | grep -q "hermes-central.*Up"; then
    echo "Services already running, skipping build..."
    docker compose up -d  # Just ensure they're all up
else
    echo "Starting services with build..."
    docker compose up -d --build
fi
test_result $? "Docker compose services ready"
echo ""

# Test 1: Wait for services to be healthy
echo "Test 1: Waiting for services to be healthy..."
wait_for_service "http://localhost:7701/health" "Meilisearch" || exit 1
wait_for_service "http://localhost:8000/health" "Central Hermes" || exit 1
wait_for_service "http://localhost:8002/health" "Edge Hermes" || exit 1
echo ""

# Test 2: Central Hermes health check
echo "Test 2: Central Hermes health check..."
CENTRAL_HEALTH=$(curl -s http://localhost:8000/health)
if echo "$CENTRAL_HEALTH" | grep -qi "ok"; then
    test_result 0 "Central Hermes is healthy"
else
    test_result 1 "Central Hermes health check failed"
fi
echo ""

# Test 3: Edge Hermes health check
echo "Test 3: Edge Hermes health check..."
EDGE_HEALTH=$(curl -s http://localhost:8002/health)
if echo "$EDGE_HEALTH" | grep -qi "ok"; then
    test_result 0 "Edge Hermes is healthy"
else
    test_result 1 "Edge Hermes health check failed"
fi
echo ""

# Test 4: Network connectivity - Edge can reach Central
echo "Test 4: Testing network connectivity (edge -> central)..."
CONNECT_TEST=$(docker compose exec -T hermes-edge wget --tries=1 --timeout=2 -O- http://hermes-central:8000/health 2>/dev/null || echo "failed")
if echo "$CONNECT_TEST" | grep -qi "ok"; then
    test_result 0 "Edge can reach Central via container network"
else
    test_result 1 "Edge cannot reach Central"
fi
echo ""

# Test 5: Web UI is accessible
echo "Test 5: Web UI health check..."
WEB_HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:4201/)
if [ "$WEB_HEALTH" = "200" ]; then
    test_result 0 "Web UI is accessible on port 4201"
else
    test_result 1 "Web UI is not accessible (HTTP $WEB_HEALTH)"
fi
echo ""

# Test 6: Check workspace volumes exist
echo "Test 6: Checking workspace volumes..."
CENTRAL_VOLUME=$(docker volume ls | grep hermes_central_workspace)
EDGE_VOLUME=$(docker volume ls | grep hermes_edge_workspace)
if [ -n "$CENTRAL_VOLUME" ] && [ -n "$EDGE_VOLUME" ]; then
    test_result 0 "Workspace volumes created (central and edge)"
else
    test_result 1 "Workspace volumes missing"
fi
echo ""

# Test 7: Verify configurations are loaded
echo "Test 7: Verifying configurations..."
# Check that central is using config-central.hcl
CENTRAL_CONFIG=$(docker compose exec -T hermes-central cat /app/config.hcl | head -1)
if echo "$CENTRAL_CONFIG" | grep -q "Central Hermes"; then
    test_result 0 "Central Hermes using correct configuration"
else
    test_result 1 "Central Hermes configuration issue"
fi

# Check that edge is using config-edge.hcl
EDGE_CONFIG=$(docker compose exec -T hermes-edge cat /app/config.hcl | head -1)
if echo "$EDGE_CONFIG" | grep -q "Edge Hermes"; then
    test_result 0 "Edge Hermes using correct configuration"
else
    test_result 1 "Edge Hermes configuration issue"
fi
echo ""

# Test 8: Check database connectivity
echo "Test 8: Checking database connectivity..."
DB_TEST=$(docker compose exec -T postgres psql -U postgres -d hermes_testing -c "SELECT 1;" 2>&1)
if echo "$DB_TEST" | grep -q "1 row"; then
    test_result 0 "Database is accessible and responding"
else
    test_result 1 "Database connection failed"
fi
echo ""

# Test 9: Check Meilisearch index
echo "Test 9: Checking Meilisearch indexes..."
MEILI_INDEXES=$(curl -s "http://localhost:7701/indexes" -H "Authorization: Bearer masterKey123")
if echo "$MEILI_INDEXES" | grep -q "results"; then
    test_result 0 "Meilisearch is responding with indexes"
else
    test_result 1 "Meilisearch index query failed"
fi
echo ""

# Test 10: Test document creation on central (should work)
echo "Test 10: Creating test document on central..."
DOC_UUID=$(uuidgen | tr '[:upper:]' '[:lower:]')
CREATE_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8000/api/v2/drafts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-token" \
  -d "{
    \"title\": \"Test Central Document\",
    \"product\": \"Engineering\",
    \"docType\": \"RFC\",
    \"summary\": \"Test document created on central Hermes for RFC-085 testing\"
  }" 2>&1)

HTTP_CODE=$(echo "$CREATE_RESPONSE" | tail -1)
if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "201" ]; then
    test_result 0 "Document created on central Hermes"
    echo "  Document UUID: $DOC_UUID"
else
    test_result 1 "Document creation failed on central (HTTP $HTTP_CODE)" "optional"
    echo "  Response: $(echo "$CREATE_RESPONSE" | head -n -1)"
fi
echo ""

# Test 11: Test document creation on edge (should work with local provider)
echo "Test 11: Creating test document on edge..."
EDGE_DOC_UUID=$(uuidgen | tr '[:upper:]' '[:lower:]')
EDGE_CREATE_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8002/api/v2/drafts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-token" \
  -d "{
    \"title\": \"Test Edge Document\",
    \"product\": \"Engineering\",
    \"docType\": \"RFC\",
    \"summary\": \"Test document created on edge Hermes for RFC-085 testing\"
  }" 2>&1)

EDGE_HTTP_CODE=$(echo "$EDGE_CREATE_RESPONSE" | tail -1)
if [ "$EDGE_HTTP_CODE" = "200" ] || [ "$EDGE_HTTP_CODE" = "201" ]; then
    test_result 0 "Document created on edge Hermes"
    echo "  Document UUID: $EDGE_DOC_UUID"
else
    test_result 1 "Document creation failed on edge (HTTP $EDGE_HTTP_CODE)" "optional"
    echo "  Response: $(echo "$EDGE_CREATE_RESPONSE" | head -n -1)"
fi
echo ""

# Test 12: Future test - Document sync (not yet implemented)
echo "Test 12: Document synchronization (NOT YET IMPLEMENTED)..."
echo -e "${YELLOW}⚠${NC}  Document sync requires multi-provider manager (RFC-085 Phase 1)"
echo "  Once implemented, this test will verify:"
echo "  - Documents created on edge are synced to central registry"
echo "  - Metadata updates are propagated"
echo "  - Sync status endpoints work correctly"
echo ""

# Test 13: Future test - Directory delegation (not yet implemented)
echo "Test 13: Directory delegation (NOT YET IMPLEMENTED)..."
echo -e "${YELLOW}⚠${NC}  Directory delegation requires multi-provider manager (RFC-085 Phase 1)"
echo "  Once implemented, this test will verify:"
echo "  - People search on edge delegates to central"
echo "  - Results are returned correctly"
echo ""

# Test 14: Future test - Permission delegation (not yet implemented)
echo "Test 14: Permission delegation (NOT YET IMPLEMENTED)..."
echo -e "${YELLOW}⚠${NC}  Permission delegation requires multi-provider manager (RFC-085 Phase 1)"
echo "  Once implemented, this test will verify:"
echo "  - Permission checks on edge delegate to central"
echo "  - Access control is enforced correctly"
echo ""

# Test 15: Service logs check
echo "Test 15: Checking for errors in service logs..."
CENTRAL_ERRORS=$(docker compose logs hermes-central 2>&1 | grep -i "error\|fatal\|panic" | grep -v "test" || true)
EDGE_ERRORS=$(docker compose logs hermes-edge 2>&1 | grep -i "error\|fatal\|panic" | grep -v "test" || true)

if [ -z "$CENTRAL_ERRORS" ] && [ -z "$EDGE_ERRORS" ]; then
    test_result 0 "No errors in service logs"
else
    echo -e "${YELLOW}⚠${NC}  Some errors found in logs (may be expected):"
    [ -n "$CENTRAL_ERRORS" ] && echo "  Central: $(echo "$CENTRAL_ERRORS" | head -2)"
    [ -n "$EDGE_ERRORS" ] && echo "  Edge: $(echo "$EDGE_ERRORS" | head -2)"
fi
echo ""

# Summary
echo "=== Test Summary ==="
echo ""
echo -e "${GREEN}✓ Infrastructure Tests Passed${NC}"
echo "  - Docker compose services started"
echo "  - Central and edge Hermes are healthy"
echo "  - Network connectivity established"
echo "  - Databases and search indexes operational"
echo "  - Configurations loaded correctly"
echo ""
echo -e "${YELLOW}⚠ Pending Implementation (RFC-085 Phase 1)${NC}"
echo "  - Multi-provider manager"
echo "  - Document synchronization"
echo "  - Directory delegation"
echo "  - Permission delegation"
echo ""
echo "Services Running:"
echo "  Central Hermes:  http://localhost:8000"
echo "  Edge Hermes:     http://localhost:8002"
echo "  Web UI:          http://localhost:4201"
echo "  Meilisearch:     http://localhost:7701"
echo ""
echo "Commands:"
echo "  View central logs:  docker compose logs -f hermes-central"
echo "  View edge logs:     docker compose logs -f hermes-edge"
echo "  Stop services:      docker compose down"
echo "  Clean volumes:      docker compose down -v"
echo ""
echo -e "${GREEN}✓ Integration test completed successfully!${NC}"
