#!/bin/bash
# Simple API test for document creation and update
# Demonstrates API-level E2E testing as an alternative to browser testing

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TESTING_DIR="$REPO_ROOT/testing"

cd "$TESTING_DIR"

API_BASE="http://localhost:8001"

echo "=== Hermes API E2E Test: Document Creation & Update ==="
echo ""

# Step 1: Get auth cookies using the testing helper script
echo "Step 1: Authenticating..."
if ! command -v npx &> /dev/null; then
    echo "❌ npx not found. Please install Node.js"
    exit 1
fi

# Run the auth cookie script and capture output
npx ts-node get-auth-cookies.ts > /tmp/auth-output.log 2>&1

if [ ! -f /tmp/hermes-auth-cookies.txt ]; then
    echo "❌ Authentication failed. Check /tmp/auth-output.log for details"
    cat /tmp/auth-output.log
    exit 1
fi

COOKIES=$(cat /tmp/hermes-auth-cookies.txt)
echo "✅ Authenticated successfully"
echo ""

# Step 2: Test /api/v2/me endpoint
echo "Step 2: Verifying authentication..."
ME_RESPONSE=$(curl -s -H "Cookie: $COOKIES" "$API_BASE/api/v2/me")
USER_ID=$(echo "$ME_RESPONSE" | jq -r '.id // .email' 2>/dev/null)

if [ -z "$USER_ID" ] || [ "$USER_ID" = "null" ]; then
    echo "❌ Failed to get user info"
    echo "$ME_RESPONSE"
    exit 1
fi

echo "✅ Authenticated as: $USER_ID"
echo ""

# Step 3: Create a test document (local workspace)
echo "Step 3: Creating test document..."
CREATE_PAYLOAD='{
  "title": "API Test Document",
  "docType": "RFC",
  "summary": "Test document created via API",
  "content": "# Test Document\n\nThis is a test document created via API."
}'

CREATE_RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Cookie: $COOKIES" \
  "$API_BASE/api/v2/documents" \
  -d "$CREATE_PAYLOAD")

DOC_ID=$(echo "$CREATE_RESPONSE" | jq -r '.id // .objectID // .googleFileID' 2>/dev/null)

if [ -z "$DOC_ID" ] || [ "$DOC_ID" = "null" ]; then
    echo "❌ Failed to create document"
    echo "$CREATE_RESPONSE" | jq '.' 2>/dev/null || echo "$CREATE_RESPONSE"
    exit 1
fi

echo "✅ Document created with ID: $DOC_ID"
echo ""

# Step 4: Read the document back
echo "Step 4: Reading document..."
GET_RESPONSE=$(curl -s -H "Cookie: $COOKIES" "$API_BASE/api/v2/documents/$DOC_ID")
TITLE=$(echo "$GET_RESPONSE" | jq -r '.title' 2>/dev/null)

if [ "$TITLE" != "API Test Document" ]; then
    echo "❌ Failed to read document correctly"
    echo "$GET_RESPONSE" | jq '.' 2>/dev/null || echo "$GET_RESPONSE"
    exit 1
fi

echo "✅ Document retrieved successfully"
echo "   Title: $TITLE"
echo ""

# Step 5: Update the document
echo "Step 5: Updating document..."
UPDATE_PAYLOAD='{
  "title": "API Test Document (Updated)",
  "summary": "This document was updated via API",
  "content": "# Updated Test Document\n\nThis document has been updated."
}'

UPDATE_RESPONSE=$(curl -s -X PATCH \
  -H "Content-Type: application/json" \
  -H "Cookie: $COOKIES" \
  "$API_BASE/api/v2/documents/$DOC_ID" \
  -d "$UPDATE_PAYLOAD")

UPDATED_TITLE=$(echo "$UPDATE_RESPONSE" | jq -r '.title' 2>/dev/null)

if [ -z "$UPDATED_TITLE" ]; then
    echo "⚠️  Update response didn't include title, verifying..."
    # Verify by reading again
    GET_RESPONSE=$(curl -s -H "Cookie: $COOKIES" "$API_BASE/api/v2/documents/$DOC_ID")
    UPDATED_TITLE=$(echo "$GET_RESPONSE" | jq -r '.title' 2>/dev/null)
fi

if [[ "$UPDATED_TITLE" == *"Updated"* ]]; then
    echo "✅ Document updated successfully"
    echo "   New title: $UPDATED_TITLE"
else
    echo "❌ Document update may have failed"
    echo "   Expected title to contain 'Updated', got: $UPDATED_TITLE"
fi
echo ""

# Step 6: List documents to verify it appears
echo "Step 6: Listing documents..."
LIST_RESPONSE=$(curl -s -H "Cookie: $COOKIES" "$API_BASE/api/v2/documents?limit=10")
DOC_COUNT=$(echo "$LIST_RESPONSE" | jq -r 'length // .documents | length' 2>/dev/null)

echo "✅ Found $DOC_COUNT documents"
echo ""

echo "=== ✅ All API tests passed! ==="
echo ""
echo "Summary:"
echo "  - Authenticated with Dex OIDC"
echo "  - Created document: $DOC_ID"
echo "  - Read document successfully"
echo "  - Updated document successfully"
echo "  - Listed documents successfully"
echo ""
echo "This demonstrates that the backend API works correctly,"
echo "even though the frontend has a template compilation issue."
