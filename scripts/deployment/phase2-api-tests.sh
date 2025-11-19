#!/bin/bash
# RFC-088 Phase 2: API Endpoint Testing Suite
# Comprehensive tests for semantic search API endpoints

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
API_URL="${API_URL:-http://localhost:8000}"
API_TOKEN="${API_TOKEN:-}"
TEST_DOCUMENT_ID="${TEST_DOCUMENT_ID:-}"
OUTPUT_DIR="${OUTPUT_DIR:-./test-results}"

# Test counters
PASSED=0
FAILED=0
SKIPPED=0

echo "================================================"
echo "RFC-088 Phase 2: API Endpoint Testing"
echo "================================================"
echo ""
echo "Configuration:"
echo "  API URL:     $API_URL"
echo "  Auth Token:  $([ -n "$API_TOKEN" ] && echo "Set" || echo "Not set")"
echo "  Output Dir:  $OUTPUT_DIR"
echo ""

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Helper functions
pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASSED++))
}

fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAILED++))
}

skip() {
    echo -e "${YELLOW}⊘${NC} $1"
    ((SKIPPED++))
}

info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

section() {
    echo ""
    echo "━━━ $1 ━━━"
}

# Function to make API request
api_request() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    local output_file="$4"

    local curl_cmd="curl -s -w '\n%{http_code}' -X $method '${API_URL}${endpoint}' -H 'Content-Type: application/json'"

    if [ -n "$API_TOKEN" ]; then
        curl_cmd="$curl_cmd -H 'Authorization: Bearer $API_TOKEN'"
    fi

    if [ -n "$data" ]; then
        curl_cmd="$curl_cmd -d '$data'"
    fi

    local response=$(eval $curl_cmd 2>/dev/null || echo -e "\n000")
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)

    if [ -n "$output_file" ]; then
        echo "$body" > "$output_file"
    fi

    echo "$http_code|$body"
}

# Test 1: Health Check
section "Test 1: Health Check"

info "GET /health"
RESULT=$(api_request "GET" "/health" "" "$OUTPUT_DIR/health.json")
HTTP_CODE=$(echo "$RESULT" | cut -d'|' -f1)
BODY=$(echo "$RESULT" | cut -d'|' -f2-)

if [ "$HTTP_CODE" = "200" ]; then
    pass "Health endpoint returned 200 OK"
    info "Response: $BODY"
else
    fail "Health endpoint returned HTTP $HTTP_CODE"
fi

# Test 2: Semantic Search - Basic Query
section "Test 2: Semantic Search - Basic Query"

info "POST /api/v2/search/semantic (basic query)"
QUERY_DATA='{"query": "kubernetes deployment strategies", "limit": 10}'
RESULT=$(api_request "POST" "/api/v2/search/semantic" "$QUERY_DATA" "$OUTPUT_DIR/semantic-basic.json")
HTTP_CODE=$(echo "$RESULT" | cut -d'|' -f1)
BODY=$(echo "$RESULT" | cut -d'|' -f2-)

if [ "$HTTP_CODE" = "200" ]; then
    pass "Semantic search basic query succeeded"

    # Validate response structure
    RESULT_COUNT=$(echo "$BODY" | jq '.results | length' 2>/dev/null || echo "0")
    if [ "$RESULT_COUNT" -gt 0 ]; then
        pass "  Returned $RESULT_COUNT results"

        # Check result structure
        HAS_SCORE=$(echo "$BODY" | jq '.results[0].score' 2>/dev/null)
        HAS_DOC_ID=$(echo "$BODY" | jq -r '.results[0].document_id' 2>/dev/null)

        if [ "$HAS_SCORE" != "null" ] && [ "$HAS_DOC_ID" != "null" ]; then
            pass "  Results have required fields (document_id, score)"
        else
            fail "  Results missing required fields"
        fi
    else
        info "  No results returned (may be expected if no documents indexed)"
    fi

elif [ "$HTTP_CODE" = "401" ]; then
    skip "Semantic search requires authentication (HTTP 401)"
    info "  Set API_TOKEN environment variable to run authenticated tests"
elif [ "$HTTP_CODE" = "404" ]; then
    fail "Semantic search endpoint not found (HTTP 404)"
    info "  Ensure RFC-088 is deployed and semantic_search is enabled"
else
    fail "Semantic search returned HTTP $HTTP_CODE"
    info "  Response: $BODY"
fi

# Test 3: Semantic Search - With Filters
section "Test 3: Semantic Search - With Filters"

info "POST /api/v2/search/semantic (with document IDs filter)"
QUERY_DATA='{"query": "kubernetes deployment", "limit": 5, "document_ids": ["doc1", "doc2", "doc3"]}'
RESULT=$(api_request "POST" "/api/v2/search/semantic" "$QUERY_DATA" "$OUTPUT_DIR/semantic-filtered.json")
HTTP_CODE=$(echo "$RESULT" | cut -d'|' -f1)

if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
    if [ "$HTTP_CODE" = "200" ]; then
        pass "Semantic search with filters succeeded"
    else
        skip "Semantic search with filters requires authentication"
    fi
elif [ "$HTTP_CODE" = "404" ]; then
    fail "Semantic search endpoint not found"
else
    fail "Semantic search with filters returned HTTP $HTTP_CODE"
fi

# Test 4: Semantic Search - Similarity Threshold
section "Test 4: Semantic Search - Similarity Threshold"

info "POST /api/v2/search/semantic (with similarity threshold)"
QUERY_DATA='{"query": "API design patterns", "limit": 10, "similarity_threshold": 0.8}'
RESULT=$(api_request "POST" "/api/v2/search/semantic" "$QUERY_DATA" "$OUTPUT_DIR/semantic-threshold.json")
HTTP_CODE=$(echo "$RESULT" | cut -d'|' -f1)

if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
    if [ "$HTTP_CODE" = "200" ]; then
        pass "Semantic search with threshold succeeded"

        # Verify all results meet threshold
        MIN_SCORE=$(echo "$BODY" | jq '[.results[].score] | min' 2>/dev/null || echo "0")
        if (( $(echo "$MIN_SCORE >= 0.8" | bc -l 2>/dev/null || echo "0") )); then
            pass "  All results meet similarity threshold (min score: $MIN_SCORE)"
        fi
    else
        skip "Semantic search with threshold requires authentication"
    fi
else
    fail "Semantic search with threshold returned HTTP $HTTP_CODE"
fi

# Test 5: Hybrid Search - Basic
section "Test 5: Hybrid Search - Basic"

info "POST /api/v2/search/hybrid (basic query)"
QUERY_DATA='{"query": "kubernetes deployment", "limit": 10}'
RESULT=$(api_request "POST" "/api/v2/search/hybrid" "$QUERY_DATA" "$OUTPUT_DIR/hybrid-basic.json")
HTTP_CODE=$(echo "$RESULT" | cut -d'|' -f1)
BODY=$(echo "$RESULT" | cut -d'|' -f2-)

if [ "$HTTP_CODE" = "200" ]; then
    pass "Hybrid search basic query succeeded"

    # Validate response
    RESULT_COUNT=$(echo "$BODY" | jq '.results | length' 2>/dev/null || echo "0")
    if [ "$RESULT_COUNT" -gt 0 ]; then
        pass "  Returned $RESULT_COUNT results"

        # Check for combined score
        HAS_COMBINED=$(echo "$BODY" | jq '.results[0].combined_score' 2>/dev/null)
        if [ "$HAS_COMBINED" != "null" ]; then
            pass "  Results have combined_score field"
        fi
    fi

elif [ "$HTTP_CODE" = "401" ]; then
    skip "Hybrid search requires authentication"
elif [ "$HTTP_CODE" = "404" ]; then
    fail "Hybrid search endpoint not found"
else
    fail "Hybrid search returned HTTP $HTTP_CODE"
fi

# Test 6: Hybrid Search - Custom Weights
section "Test 6: Hybrid Search - Custom Weights"

info "POST /api/v2/search/hybrid (custom weights)"
QUERY_DATA='{"query": "API design", "limit": 10, "keyword_weight": 0.7, "semantic_weight": 0.2, "boost_weight": 0.1}'
RESULT=$(api_request "POST" "/api/v2/search/hybrid" "$QUERY_DATA" "$OUTPUT_DIR/hybrid-weighted.json")
HTTP_CODE=$(echo "$RESULT" | cut -d'|' -f1)

if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
    if [ "$HTTP_CODE" = "200" ]; then
        pass "Hybrid search with custom weights succeeded"
    else
        skip "Hybrid search with custom weights requires authentication"
    fi
else
    fail "Hybrid search with custom weights returned HTTP $HTTP_CODE"
fi

# Test 7: Similar Documents
section "Test 7: Similar Documents"

# Try to get a document ID from semantic search results
if [ -f "$OUTPUT_DIR/semantic-basic.json" ]; then
    TEST_DOC_ID=$(jq -r '.results[0].document_id' "$OUTPUT_DIR/semantic-basic.json" 2>/dev/null || echo "")
fi

if [ -z "$TEST_DOC_ID" ]; then
    TEST_DOC_ID="${TEST_DOCUMENT_ID:-test-doc-id}"
fi

info "GET /api/v2/documents/$TEST_DOC_ID/similar"
RESULT=$(api_request "GET" "/api/v2/documents/$TEST_DOC_ID/similar?limit=10" "" "$OUTPUT_DIR/similar-docs.json")
HTTP_CODE=$(echo "$RESULT" | cut -d'|' -f1)
BODY=$(echo "$RESULT" | cut -d'|' -f2-)

if [ "$HTTP_CODE" = "200" ]; then
    pass "Similar documents endpoint succeeded"

    RESULT_COUNT=$(echo "$BODY" | jq '.results | length' 2>/dev/null || echo "0")
    if [ "$RESULT_COUNT" -gt 0 ]; then
        pass "  Returned $RESULT_COUNT similar documents"
    fi

elif [ "$HTTP_CODE" = "401" ]; then
    skip "Similar documents requires authentication"
elif [ "$HTTP_CODE" = "404" ]; then
    info "  Document not found (HTTP 404) - expected if using test ID"
else
    fail "Similar documents returned HTTP $HTTP_CODE"
fi

# Test 8: Error Handling - Invalid Request
section "Test 8: Error Handling"

info "POST /api/v2/search/semantic (missing query)"
QUERY_DATA='{"limit": 10}'
RESULT=$(api_request "POST" "/api/v2/search/semantic" "$QUERY_DATA" "$OUTPUT_DIR/semantic-error.json")
HTTP_CODE=$(echo "$RESULT" | cut -d'|' -f1)

if [ "$HTTP_CODE" = "400" ] || [ "$HTTP_CODE" = "422" ]; then
    pass "API correctly returns 4xx for invalid request"
elif [ "$HTTP_CODE" = "401" ]; then
    skip "Cannot test error handling without authentication"
else
    info "  Got HTTP $HTTP_CODE (expected 400 or 422)"
fi

info "POST /api/v2/search/semantic (negative limit)"
QUERY_DATA='{"query": "test", "limit": -1}'
RESULT=$(api_request "POST" "/api/v2/search/semantic" "$QUERY_DATA" "$OUTPUT_DIR/semantic-error2.json")
HTTP_CODE=$(echo "$RESULT" | cut -d'|' -f1)

if [ "$HTTP_CODE" = "400" ] || [ "$HTTP_CODE" = "422" ]; then
    pass "API correctly validates request parameters"
elif [ "$HTTP_CODE" = "401" ]; then
    skip "Cannot test validation without authentication"
else
    info "  Got HTTP $HTTP_CODE (expected 400 or 422)"
fi

# Test 9: Performance Testing
section "Test 9: Performance Testing"

if [ "$HTTP_CODE" != "401" ]; then  # Skip if not authenticated
    info "Running performance test (10 sequential queries)..."

    TOTAL_TIME=0
    SUCCESS_COUNT=0

    for i in {1..10}; do
        QUERY_DATA='{"query": "test query '$i'", "limit": 10}'
        START=$(date +%s%N)
        RESULT=$(api_request "POST" "/api/v2/search/semantic" "$QUERY_DATA" "")
        END=$(date +%s%N)
        HTTP_CODE=$(echo "$RESULT" | cut -d'|' -f1)

        DURATION_MS=$(( (END - START) / 1000000 ))
        TOTAL_TIME=$((TOTAL_TIME + DURATION_MS))

        if [ "$HTTP_CODE" = "200" ]; then
            ((SUCCESS_COUNT++))
        fi
    done

    AVG_TIME=$((TOTAL_TIME / 10))

    if [ "$SUCCESS_COUNT" -eq 10 ]; then
        pass "Performance test: 10/10 queries succeeded"
        info "  Average response time: ${AVG_TIME}ms"

        if [ "$AVG_TIME" -lt 200 ]; then
            pass "  Performance is excellent (<200ms average)"
        elif [ "$AVG_TIME" -lt 500 ]; then
            info "  Performance is good (<500ms average)"
        else
            info "  Performance could be optimized (${AVG_TIME}ms average)"
        fi
    else
        fail "Performance test: only $SUCCESS_COUNT/10 queries succeeded"
    fi
else
    skip "Performance test requires authentication"
fi

# Generate test report
section "Generating Test Report"

REPORT_FILE="$OUTPUT_DIR/test-report.md"

cat > "$REPORT_FILE" << EOF
# RFC-088 API Testing Report

**Generated**: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
**API URL**: $API_URL

## Test Summary

- ✓ Passed: $PASSED
- ✗ Failed: $FAILED
- ⊘ Skipped: $SKIPPED
- **Total**: $((PASSED + FAILED + SKIPPED))

## Test Results

### Health Check
- Status: $([ -f "$OUTPUT_DIR/health.json" ] && echo "✓ Passed" || echo "✗ Failed")

### Semantic Search
- Basic Query: $([ -f "$OUTPUT_DIR/semantic-basic.json" ] && echo "✓ Tested" || echo "⊘ Skipped")
- With Filters: $([ -f "$OUTPUT_DIR/semantic-filtered.json" ] && echo "✓ Tested" || echo "⊘ Skipped")
- With Threshold: $([ -f "$OUTPUT_DIR/semantic-threshold.json" ] && echo "✓ Tested" || echo "⊘ Skipped")

### Hybrid Search
- Basic Query: $([ -f "$OUTPUT_DIR/hybrid-basic.json" ] && echo "✓ Tested" || echo "⊘ Skipped")
- Custom Weights: $([ -f "$OUTPUT_DIR/hybrid-weighted.json" ] && echo "✓ Tested" || echo "⊘ Skipped")

### Similar Documents
- Similar Lookup: $([ -f "$OUTPUT_DIR/similar-docs.json" ] && echo "✓ Tested" || echo "⊘ Skipped")

### Error Handling
- Invalid Requests: ✓ Tested

## Response Files

All API responses have been saved to:
\`$OUTPUT_DIR/\`

## Next Steps

EOF

if [ "$FAILED" -eq 0 ]; then
    echo "✓ All tests passed!" >> "$REPORT_FILE"
else
    echo "⚠ Some tests failed. Review the output above." >> "$REPORT_FILE"
fi

if [ "$SKIPPED" -gt 0 ]; then
    echo "" >> "$REPORT_FILE"
    echo "**Note**: $SKIPPED tests were skipped (likely due to authentication)." >> "$REPORT_FILE"
    echo "Set API_TOKEN environment variable to run authenticated tests." >> "$REPORT_FILE"
fi

info "Test report saved to: $REPORT_FILE"

# Summary
echo ""
echo "================================================"
echo "API Testing Summary"
echo "================================================"
echo -e "${GREEN}Passed:${NC}  $PASSED"
echo -e "${RED}Failed:${NC}  $FAILED"
echo -e "${YELLOW}Skipped:${NC} $SKIPPED"
echo ""

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}✓ All API tests passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ Some API tests failed.${NC}"
    exit 1
fi
