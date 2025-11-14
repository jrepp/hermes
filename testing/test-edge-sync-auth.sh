#!/bin/bash
# RFC-085 Edge Sync API - Authentication Integration Tests
#
# This script tests the Bearer token authentication for edge-to-central communication
# as specified in RFC-086.
#
# Prerequisites:
#   - Docker Compose services running (postgres, hermes-central)
#   - Migration 000009 applied (service_tokens table)
#   - Migration 000008 applied (edge_document_registry table)
#
# Usage:
#   ./test-edge-sync-auth.sh [central-url]
#
# Environment Variables:
#   HERMES_CENTRAL_URL - Central Hermes URL (default: http://localhost:8000)
#   EDGE_INSTANCE - Edge instance name (default: edge-dev-test)

set -e

CENTRAL_URL="${HERMES_CENTRAL_URL:-${1:-http://localhost:8000}}"
EDGE_INSTANCE="${EDGE_INSTANCE:-edge-dev-test}"

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# Helper functions
pass() {
    echo -e "${GREEN}✓ PASS${NC}: $1"
    ((TESTS_PASSED++))
    ((TESTS_TOTAL++))
}

fail() {
    echo -e "${RED}✗ FAIL${NC}: $1"
    ((TESTS_FAILED++))
    ((TESTS_TOTAL++))
}

warn() {
    echo -e "${YELLOW}⚠ WARN${NC}: $1"
}

info() {
    echo -e "${BLUE}ℹ INFO${NC}: $1"
}

section() {
    echo ""
    echo "================================================================="
    echo "$1"
    echo "================================================================="
}

# Cleanup function
cleanup() {
    if [ -n "$TEST_TOKEN" ]; then
        info "Cleaning up test token..."
        docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c \
            "DELETE FROM service_tokens WHERE token_hash = '$TEST_TOKEN_HASH'" >/dev/null 2>&1 || true
    fi
}

trap cleanup EXIT

section "RFC-085 Edge Sync API - Authentication Integration Tests"
echo "Central URL: $CENTRAL_URL"
echo "Edge Instance: $EDGE_INSTANCE"
echo ""

# Check prerequisites
section "Prerequisites Check"

# Check if central server is running
info "Checking if central Hermes is available..."
if ! curl -s -f "$CENTRAL_URL/health" >/dev/null 2>&1; then
    fail "Central Hermes is not accessible at $CENTRAL_URL"
    exit 1
fi
pass "Central Hermes is accessible"

# Check if PostgreSQL is available
info "Checking if PostgreSQL is available..."
if ! docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c "SELECT 1" >/dev/null 2>&1; then
    fail "PostgreSQL is not accessible"
    exit 1
fi
pass "PostgreSQL is accessible"

# Check if service_tokens table exists
info "Checking if service_tokens table exists..."
TABLE_EXISTS=$(docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -t -c \
    "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='service_tokens');" | tr -d ' ')

if [ "$TABLE_EXISTS" != "t" ]; then
    fail "service_tokens table does not exist (migration 000009 not applied)"
    exit 1
fi
pass "service_tokens table exists"

# Check if edge_document_registry table exists
info "Checking if edge_document_registry table exists..."
TABLE_EXISTS=$(docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -t -c \
    "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='edge_document_registry');" | tr -d ' ')

if [ "$TABLE_EXISTS" != "t" ]; then
    fail "edge_document_registry table does not exist (migration 000008 not applied)"
    exit 1
fi
pass "edge_document_registry table exists"

# Generate test token
section "Test Token Generation"

info "Generating test token..."
TEST_TOKEN="hermes-edge-token-$(uuidgen | tr '[:upper:]' '[:lower:]')-$(openssl rand -hex 8)"
TEST_TOKEN_HASH=$(printf "%s" "$TEST_TOKEN" | shasum -a 256 | awk '{print $1}')

info "Token: ${TEST_TOKEN:0:40}..."
info "Hash: ${TEST_TOKEN_HASH:0:20}..."

# Insert test token into database
info "Inserting test token into database..."
docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c \
    "INSERT INTO service_tokens (id, created_at, updated_at, token_hash, token_type, revoked)
     VALUES (gen_random_uuid(), NOW(), NOW(), '$TEST_TOKEN_HASH', 'edge', false);" >/dev/null

# Verify token was inserted
TOKEN_COUNT=$(docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -t -c \
    "SELECT COUNT(*) FROM service_tokens WHERE token_hash = '$TEST_TOKEN_HASH';" | tr -d ' ')

if [ "$TOKEN_COUNT" != "1" ]; then
    fail "Failed to insert test token into database"
    exit 1
fi
pass "Test token created and stored in database"

# Test 1: Reject requests without Authorization header
section "Test 1: Reject Unauthenticated Requests"

RESPONSE=$(curl -s -w "\n%{http_code}" "$CENTRAL_URL/api/v2/edge/documents/sync-status?edge_instance=$EDGE_INSTANCE")
BODY=$(echo "$RESPONSE" | sed '$ d')
STATUS=$(echo "$RESPONSE" | tail -n 1)

if [ "$STATUS" = "401" ]; then
    if echo "$BODY" | grep -q "authorization"; then
        pass "Correctly rejected request without Authorization header (HTTP 401)"
    else
        fail "HTTP 401 but unexpected error message: $BODY"
    fi
else
    fail "Expected HTTP 401, got HTTP $STATUS"
fi

# Test 2: Reject invalid Authorization format
section "Test 2: Reject Invalid Authorization Format"

RESPONSE=$(curl -s -w "\n%{http_code}" \
    -H "Authorization: InvalidFormat token123" \
    "$CENTRAL_URL/api/v2/edge/documents/sync-status?edge_instance=$EDGE_INSTANCE")
BODY=$(echo "$RESPONSE" | sed '$ d')
STATUS=$(echo "$RESPONSE" | tail -n 1)

if [ "$STATUS" = "401" ]; then
    pass "Correctly rejected invalid Authorization format (HTTP 401)"
else
    fail "Expected HTTP 401, got HTTP $STATUS"
fi

# Test 3: Reject invalid/non-existent token
section "Test 3: Reject Invalid Token"

FAKE_TOKEN="hermes-edge-token-$(uuidgen | tr '[:upper:]' '[:lower:]')-fakefake"
RESPONSE=$(curl -s -w "\n%{http_code}" \
    -H "Authorization: Bearer $FAKE_TOKEN" \
    "$CENTRAL_URL/api/v2/edge/documents/sync-status?edge_instance=$EDGE_INSTANCE")
BODY=$(echo "$RESPONSE" | sed '$ d')
STATUS=$(echo "$RESPONSE" | tail -n 1)

if [ "$STATUS" = "401" ]; then
    if echo "$BODY" | grep -qi "invalid\|expired"; then
        pass "Correctly rejected invalid token (HTTP 401)"
    else
        fail "HTTP 401 but unexpected error message: $BODY"
    fi
else
    fail "Expected HTTP 401, got HTTP $STATUS"
fi

# Test 4: Accept valid Bearer token - Sync Status
section "Test 4: Accept Valid Bearer Token - Sync Status"

RESPONSE=$(curl -s -w "\n%{http_code}" \
    -H "Authorization: Bearer $TEST_TOKEN" \
    "$CENTRAL_URL/api/v2/edge/documents/sync-status?edge_instance=$EDGE_INSTANCE&limit=10")
BODY=$(echo "$RESPONSE" | sed '$ d')
STATUS=$(echo "$RESPONSE" | tail -n 1)

if [ "$STATUS" = "200" ]; then
    if echo "$BODY" | grep -q "$EDGE_INSTANCE"; then
        pass "Successfully authenticated and retrieved sync status (HTTP 200)"
        info "Response contains edge_instance field"
    else
        fail "HTTP 200 but response doesn't contain edge_instance: $BODY"
    fi
else
    fail "Expected HTTP 200, got HTTP $STATUS - Response: $BODY"
fi

# Test 5: Accept valid Bearer token - Edge Stats
section "Test 5: Accept Valid Bearer Token - Edge Stats"

RESPONSE=$(curl -s -w "\n%{http_code}" \
    -H "Authorization: Bearer $TEST_TOKEN" \
    "$CENTRAL_URL/api/v2/edge/stats?edge_instance=$EDGE_INSTANCE")
BODY=$(echo "$RESPONSE" | sed '$ d')
STATUS=$(echo "$RESPONSE" | tail -n 1)

if [ "$STATUS" = "200" ]; then
    if echo "$BODY" | grep -q "edge_instance"; then
        pass "Successfully retrieved edge stats with authentication (HTTP 200)"
    else
        fail "HTTP 200 but unexpected response format: $BODY"
    fi
else
    fail "Expected HTTP 200, got HTTP $STATUS - Response: $BODY"
fi

# Test 6: Test token revocation
section "Test 6: Token Revocation"

info "Revoking test token..."
docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c \
    "UPDATE service_tokens SET revoked = true, revoked_at = NOW(), revoked_reason = 'Test revocation'
     WHERE token_hash = '$TEST_TOKEN_HASH';" >/dev/null

RESPONSE=$(curl -s -w "\n%{http_code}" \
    -H "Authorization: Bearer $TEST_TOKEN" \
    "$CENTRAL_URL/api/v2/edge/documents/sync-status?edge_instance=$EDGE_INSTANCE")
BODY=$(echo "$RESPONSE" | sed '$ d')
STATUS=$(echo "$RESPONSE" | tail -n 1)

if [ "$STATUS" = "401" ]; then
    if echo "$BODY" | grep -qi "revoked\|expired"; then
        pass "Correctly rejected revoked token (HTTP 401)"
    else
        fail "HTTP 401 but unexpected error message: $BODY"
    fi
else
    fail "Expected HTTP 401 for revoked token, got HTTP $STATUS"
fi

# Test 7: Test token expiration
section "Test 7: Token Expiration"

info "Creating expired token..."
EXPIRED_TOKEN="hermes-edge-token-$(uuidgen | tr '[:upper:]' '[:lower:]')-$(openssl rand -hex 8)"
EXPIRED_TOKEN_HASH=$(printf "%s" "$EXPIRED_TOKEN" | shasum -a 256 | awk '{print $1}')

docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c \
    "INSERT INTO service_tokens (id, created_at, updated_at, token_hash, token_type, revoked, expires_at)
     VALUES (gen_random_uuid(), NOW(), NOW(), '$EXPIRED_TOKEN_HASH', 'edge', false, NOW() - INTERVAL '1 day');" >/dev/null

RESPONSE=$(curl -s -w "\n%{http_code}" \
    -H "Authorization: Bearer $EXPIRED_TOKEN" \
    "$CENTRAL_URL/api/v2/edge/documents/sync-status?edge_instance=$EDGE_INSTANCE")
BODY=$(echo "$RESPONSE" | sed '$ d')
STATUS=$(echo "$RESPONSE" | tail -n 1)

# Cleanup expired token
docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c \
    "DELETE FROM service_tokens WHERE token_hash = '$EXPIRED_TOKEN_HASH';" >/dev/null

if [ "$STATUS" = "401" ]; then
    if echo "$BODY" | grep -qi "expired\|revoked"; then
        pass "Correctly rejected expired token (HTTP 401)"
    else
        fail "HTTP 401 but unexpected error message: $BODY"
    fi
else
    fail "Expected HTTP 401 for expired token, got HTTP $STATUS"
fi

# Test 8: Test wrong token type
section "Test 8: Wrong Token Type"

info "Creating registration token (wrong type)..."
WRONG_TYPE_TOKEN="hermes-registration-token-$(uuidgen | tr '[:upper:]' '[:lower:]')-$(openssl rand -hex 8)"
WRONG_TYPE_HASH=$(printf "%s" "$WRONG_TYPE_TOKEN" | shasum -a 256 | awk '{print $1}')

docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c \
    "INSERT INTO service_tokens (id, created_at, updated_at, token_hash, token_type, revoked)
     VALUES (gen_random_uuid(), NOW(), NOW(), '$WRONG_TYPE_HASH', 'registration', false);" >/dev/null

RESPONSE=$(curl -s -w "\n%{http_code}" \
    -H "Authorization: Bearer $WRONG_TYPE_TOKEN" \
    "$CENTRAL_URL/api/v2/edge/documents/sync-status?edge_instance=$EDGE_INSTANCE")
BODY=$(echo "$RESPONSE" | sed '$ d')
STATUS=$(echo "$RESPONSE" | tail -n 1)

# Cleanup wrong type token
docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c \
    "DELETE FROM service_tokens WHERE token_hash = '$WRONG_TYPE_HASH';" >/dev/null

if [ "$STATUS" = "403" ] || [ "$STATUS" = "401" ]; then
    if echo "$BODY" | grep -qi "token type\|forbidden"; then
        pass "Correctly rejected wrong token type (HTTP $STATUS)"
    else
        warn "HTTP $STATUS but unexpected error message: $BODY"
        pass "Correctly rejected wrong token type (HTTP $STATUS)"
    fi
else
    fail "Expected HTTP 403/401 for wrong token type, got HTTP $STATUS"
fi

# Summary
section "Test Summary"

echo ""
echo "Total Tests: $TESTS_TOTAL"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
fi
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}╔════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  ✓ All authentication tests passed successfully!      ║${NC}"
    echo -e "${GREEN}║                                                        ║${NC}"
    echo -e "${GREEN}║  RFC-086 Bearer Token Authentication: VERIFIED ✓      ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════╝${NC}"
    echo ""
    exit 0
else
    echo -e "${RED}╔════════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║  ✗ Some authentication tests failed                    ║${NC}"
    echo -e "${RED}║                                                        ║${NC}"
    echo -e "${RED}║  Please review the failures above                     ║${NC}"
    echo -e "${RED}╚════════════════════════════════════════════════════════╝${NC}"
    echo ""
    exit 1
fi
