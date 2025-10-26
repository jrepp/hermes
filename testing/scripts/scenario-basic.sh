#!/usr/bin/env bash
#
# scenario-basic.sh - Run basic distributed indexing scenario
#
# This script:
# 1. Verifies Hermes is running
# 2. Seeds test documents
# 3. Waits for indexing
# 4. Verifies documents are indexed
# 5. Tests search functionality
#

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTING_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Source helpers
source "${SCRIPT_DIR}/lib/document-generator.sh"

# Configuration
HERMES_URL="${HERMES_URL:-http://localhost:8001}"
MAX_WAIT=120  # 2 minutes max wait for indexing

echo -e "${BLUE}=== Basic Distributed Indexing Scenario ===${NC}"
echo ""

# Step 1: Verify Hermes is running
echo -e "${YELLOW}[1/5] Verifying Hermes is running...${NC}"
if ! curl -sf "${HERMES_URL}/health" > /dev/null; then
    echo -e "${YELLOW}❌ Hermes is not running at ${HERMES_URL}${NC}"
    echo -e "${YELLOW}Start it with: cd testing && make up${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Hermes is healthy${NC}"
echo ""

# Step 2: Seed test documents
echo -e "${YELLOW}[2/5] Seeding test documents...${NC}"
"${SCRIPT_DIR}/seed-workspaces.sh" --scenario basic --count 10 --clean
echo ""

# Step 3: Wait for indexing
echo -e "${YELLOW}[3/5] Waiting for indexer to detect documents...${NC}"
echo -e "${BLUE}(Checking every 5 seconds, max ${MAX_WAIT} seconds)${NC}"

ELAPSED=0
EXPECTED_DOCS=10

while [ $ELAPSED -lt $MAX_WAIT ]; do
    # Query document count
    DOC_COUNT=$(curl -sf "${HERMES_URL}/api/v2/documents" 2>/dev/null | jq -r '.total // 0' || echo "0")
    
    echo -e "${BLUE}  Documents indexed: ${DOC_COUNT} / ${EXPECTED_DOCS}${NC}"
    
    if [ "$DOC_COUNT" -ge "$EXPECTED_DOCS" ]; then
        echo -e "${GREEN}✅ All documents indexed!${NC}"
        break
    fi
    
    sleep 5
    ELAPSED=$((ELAPSED + 5))
done

if [ $ELAPSED -ge $MAX_WAIT ]; then
    echo -e "${YELLOW}⚠️  Timeout waiting for indexing${NC}"
    echo -e "${YELLOW}Documents indexed: ${DOC_COUNT} / ${EXPECTED_DOCS}${NC}"
    echo -e "${YELLOW}Note: Indexer scans every 5 minutes. You may need to wait longer.${NC}"
    echo ""
    echo -e "${BLUE}Check indexer logs: docker compose logs hermes-indexer${NC}"
fi
echo ""

# Step 4: Verify documents via API
echo -e "${YELLOW}[4/5] Verifying documents via API...${NC}"

# Get documents by type
echo -e "${BLUE}Fetching RFCs...${NC}"
RFC_COUNT=$(curl -sf "${HERMES_URL}/api/v2/documents?type=RFC" 2>/dev/null | jq -r '.total // 0')
echo -e "${GREEN}  RFCs: ${RFC_COUNT}${NC}"

echo -e "${BLUE}Fetching PRDs...${NC}"
PRD_COUNT=$(curl -sf "${HERMES_URL}/api/v2/documents?type=PRD" 2>/dev/null | jq -r '.total // 0')
echo -e "${GREEN}  PRDs: ${PRD_COUNT}${NC}"

echo -e "${BLUE}Fetching Meeting Notes...${NC}"
MEETING_COUNT=$(curl -sf "${HERMES_URL}/api/v2/documents?type=MEETING" 2>/dev/null | jq -r '.total // 0')
echo -e "${GREEN}  Meetings: ${MEETING_COUNT}${NC}"

echo ""

# Step 5: Test search functionality
echo -e "${YELLOW}[5/5] Testing search functionality...${NC}"

# Search for "test"
echo -e "${BLUE}Searching for 'test'...${NC}"
SEARCH_RESULTS=$(curl -sf "${HERMES_URL}/api/v2/search?q=test" 2>/dev/null | jq -r '.hits | length')
echo -e "${GREEN}  Found ${SEARCH_RESULTS} results${NC}"

# Search for "RFC"
echo -e "${BLUE}Searching for 'RFC'...${NC}"
SEARCH_RESULTS=$(curl -sf "${HERMES_URL}/api/v2/search?q=RFC" 2>/dev/null | jq -r '.hits | length')
echo -e "${GREEN}  Found ${SEARCH_RESULTS} results${NC}"

# Search for "distributed"
echo -e "${BLUE}Searching for 'distributed'...${NC}"
SEARCH_RESULTS=$(curl -sf "${HERMES_URL}/api/v2/search?q=distributed" 2>/dev/null | jq -r '.hits | length')
echo -e "${GREEN}  Found ${SEARCH_RESULTS} results${NC}"

echo ""

# Summary
echo -e "${GREEN}=== Scenario Complete ===${NC}"
echo ""
echo -e "${GREEN}Summary:${NC}"
echo -e "  Total documents: ${DOC_COUNT}"
echo -e "  RFCs: ${RFC_COUNT}"
echo -e "  PRDs: ${PRD_COUNT}"
echo -e "  Meetings: ${MEETING_COUNT}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo -e "  - Open web UI: ${BLUE}open http://localhost:4201${NC}"
echo -e "  - View all documents: ${BLUE}curl ${HERMES_URL}/api/v2/documents | jq${NC}"
echo -e "  - Check indexer status: ${BLUE}docker compose logs hermes-indexer${NC}"
echo ""
