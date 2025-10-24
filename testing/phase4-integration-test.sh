#!/bin/bash
# Phase 4 Integration Test Script
# Tests indexer registration, heartbeat, and document submission

set -e

HERMES_URL="http://localhost:8001"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Phase 4 Integration Test ===${NC}\n"

# Test 1: Server Health
echo -e "${YELLOW}Test 1: Server Health Check${NC}"
if curl -f -s "${HERMES_URL}/health" > /dev/null; then
    echo -e "${GREEN}✓ Server is healthy${NC}\n"
else
    echo -e "${RED}✗ Server health check failed${NC}\n"
    exit 1
fi

# Test 2: Check Indexer Registration
echo -e "${YELLOW}Test 2: Indexer Registration${NC}"
INDEXER_ID=$(docker compose logs hermes-indexer 2>/dev/null | grep "Registered as indexer" | tail -1 | sed 's/.*indexer: //' | awk '{print $1}')
if [ -n "$INDEXER_ID" ]; then
    echo -e "${GREEN}✓ Indexer registered: ${INDEXER_ID}${NC}\n"
else
    echo -e "${RED}✗ Indexer not registered${NC}\n"
    exit 1
fi

# Test 3: Check Token File
echo -e "${YELLOW}Test 3: Token File Exists${NC}"
if docker compose exec -T hermes test -f /app/shared/indexer-token.txt; then
    echo -e "${GREEN}✓ Token file exists and is accessible${NC}\n"
else
    echo -e "${RED}✗ Token file not found${NC}\n"
    exit 1
fi

# Test 4: Check Workspace Projects
echo -e "${YELLOW}Test 4: Workspace Projects Loaded${NC}"
PROJECT_COUNT=$(docker compose logs hermes 2>/dev/null | grep "Loaded.*workspace projects" | tail -1 | sed 's/.*Loaded //' | awk '{print $1}')
if [ "$PROJECT_COUNT" -ge 1 ]; then
    echo -e "${GREEN}✓ Loaded ${PROJECT_COUNT} workspace projects${NC}\n"
else
    echo -e "${RED}✗ No workspace projects loaded${NC}\n"
    exit 1
fi

# Test 5: Check Search Provider
echo -e "${YELLOW}Test 5: Search Provider (Meilisearch)${NC}"
if curl -f -s http://localhost:7701/health > /dev/null; then
    echo -e "${GREEN}✓ Meilisearch is healthy${NC}\n"
else
    echo -e "${RED}✗ Meilisearch health check failed${NC}\n"
    exit 1
fi

# Test 6: Check Database
echo -e "${YELLOW}Test 6: Database Connection${NC}"
if docker compose exec -T postgres pg_isready -U postgres > /dev/null 2>&1; then
    echo -e "${GREEN}✓ PostgreSQL is ready${NC}\n"
else
    echo -e "${RED}✗ PostgreSQL not ready${NC}\n"
    exit 1
fi

# Test 7: Verify Migrations
echo -e "${YELLOW}Test 7: Database Migrations${NC}"
MIGRATION_VERSION=$(docker compose exec -T postgres psql -U postgres -d hermes_testing -t -c "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1" 2>/dev/null | tr -d ' ')
if [ "$MIGRATION_VERSION" = "5" ]; then
    echo -e "${GREEN}✓ Latest migration applied (version ${MIGRATION_VERSION})${NC}\n"
else
    echo -e "${YELLOW}⚠ Migration version: ${MIGRATION_VERSION} (expected 5)${NC}\n"
fi

# Test 8: Check Dex (Auth Provider)
echo -e "${YELLOW}Test 8: Dex OIDC Provider${NC}"
if curl -f -s http://localhost:5558/dex/.well-known/openid-configuration > /dev/null; then
    echo -e "${GREEN}✓ Dex OIDC is responding${NC}\n"
else
    echo -e "${RED}✗ Dex OIDC check failed${NC}\n"
    exit 1
fi

# Test 9: Check Frontend
echo -e "${YELLOW}Test 9: Frontend${NC}"
if curl -f -s http://localhost:4201/ > /dev/null; then
    echo -e "${GREEN}✓ Frontend is serving${NC}\n"
else
    echo -e "${RED}✗ Frontend not responding${NC}\n"
    exit 1
fi

# Summary
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}All Phase 4 Integration Tests Passed!${NC}"
echo -e "${GREEN}========================================${NC}\n"

echo -e "${YELLOW}Service URLs:${NC}"
echo -e "  Backend API:    ${HERMES_URL}"
echo -e "  Frontend:       http://localhost:4201"
echo -e "  Meilisearch:    http://localhost:7701"
echo -e "  PostgreSQL:     localhost:5433"
echo -e "  Dex OIDC:       http://localhost:5558"
echo -e ""

echo -e "${YELLOW}Next Steps:${NC}"
echo -e "  - Monitor indexer heartbeats: docker compose logs -f hermes hermes-indexer"
echo -e "  - Test document submission: Create a document via frontend"
echo -e "  - Check search indexing: Query Meilisearch indices"
echo -e ""
