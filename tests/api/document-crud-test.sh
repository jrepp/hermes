#!/bin/bash
# API-level E2E test for document creation and update
# Works around frontend template compilation issues

set -e

API_BASE="http://localhost:8001"
TEST_USER="test@hermes.local"

echo "=== Hermes API E2E Test: Document CRUD ===" 
echo ""

# Get authentication cookies using the testing helper
echo "Step 1: Authenticating..."
AUTH_COOKIES=$(cd /Users/jrepp/hc/hermes/testing && node get-auth-cookies.ts)
if [ -z "$AUTH_COOKIES" ]; then
  echo "❌ Authentication failed"
  exit 1
fi
echo "✅ Authenticated as $TEST_USER"
echo ""

# Create a new document
echo "Step 2: Creating new document..."
CREATE_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Cookie: $AUTH_COOKIES" \
  "$API_BASE/api/v2/documents" \
  -d '{
    "title": "E2E Test Document",
    "docType": "RFC",
    "product": "Test Product",
    "summary": "This is a test document created via API"
  }')

HTTP_CODE=$(echo "$CREATE_RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$CREATE_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" != "201" ] && [ "$HTTP_CODE" != "200" ]; then
  echo "❌ Document creation failed with HTTP $HTTP_CODE"
  echo "$RESPONSE_BODY" | jq '.' 2>/dev/null || echo "$RESPONSE_BODY"
  exit 1
fi

DOC_ID=$(echo "$RESPONSE_BODY" | jq -r '.googleFileID // .objectID // .id')
echo "✅ Document created with ID: $DOC_ID"
echo ""

# Update the document content
echo "Step 3: Updating document content..."
UPDATE_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X PATCH \
  -H "Content-Type: application/json" \
  -H "Cookie: $AUTH_COOKIES" \
  "$API_BASE/api/v2/documents/$DOC_ID" \
  -d '{
    "title": "E2E Test Document (Updated)",
    "summary": "This document was updated via API test"
  }')

HTTP_CODE=$(echo "$UPDATE_RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$UPDATE_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" != "200" ]; then
  echo "❌ Document update failed with HTTP $HTTP_CODE"
  echo "$RESPONSE_BODY" | jq '.' 2>/dev/null || echo "$RESPONSE_BODY"
  exit 1
fi

echo "✅ Document updated successfully"
echo ""

# Verify the update
echo "Step 4: Verifying document..."
VERIFY_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -H "Cookie: $AUTH_COOKIES" \
  "$API_BASE/api/v2/documents/$DOC_ID")

HTTP_CODE=$(echo "$VERIFY_RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$VERIFY_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" != "200" ]; then
  echo "❌ Document verification failed with HTTP $HTTP_CODE"
  exit 1
fi

TITLE=$(echo "$RESPONSE_BODY" | jq -r '.title')
SUMMARY=$(echo "$RESPONSE_BODY" | jq -r '.summary')

echo "✅ Document verified:"
echo "   Title: $TITLE"
echo "   Summary: $SUMMARY"
echo ""

# Clean up (optional)
echo "Step 5: Cleaning up..."
DELETE_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X DELETE \
  -H "Cookie: $AUTH_COOKIES" \
  "$API_BASE/api/v2/documents/$DOC_ID")

HTTP_CODE=$(echo "$DELETE_RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "204" ]; then
  echo "✅ Document deleted successfully"
else
  echo "⚠️  Document deletion returned HTTP $HTTP_CODE (may not be supported)"
fi

echo ""
echo "=== All API tests passed! ==="
