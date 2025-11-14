#!/bin/bash
# Create an edge sync API token for testing
#
# Usage: ./create-edge-token.sh [edge-instance-name]

set -e

EDGE_INSTANCE="${1:-edge-dev-1}"

echo "Creating edge sync token for instance: $EDGE_INSTANCE"
echo ""

# Generate a token directly in the database
# Token format: hermes-<type>-token-<uuid>-<random> (standard format)
TOKEN="hermes-edge-token-$(uuidgen | tr '[:upper:]' '[:lower:]')-$(openssl rand -hex 8)"

# Hash the token for storage (SHA-256)
TOKEN_HASH=$(printf "%s" "$TOKEN" | shasum -a 256 | awk '{print $1}')

echo "Generated token: $TOKEN"
echo "Token hash: $TOKEN_HASH"
echo ""

# Insert into service_tokens table (renamed from indexer_tokens in migration 000009)
docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing <<EOF
INSERT INTO service_tokens (
    id,
    created_at,
    updated_at,
    token_hash,
    token_type,
    revoked
) VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    '$TOKEN_HASH',
    'edge',
    false
) RETURNING id, token_type, created_at;
EOF

echo ""
echo "✓ Token created successfully!"
echo ""
echo "Use this token for edge sync API calls:"
echo "  Authorization: Bearer $TOKEN"
echo ""
echo "Example:"
echo "  curl -H \"Authorization: Bearer $TOKEN\" \\"
echo "    http://localhost:8000/api/v2/edge/documents/sync-status?edge_instance=$EDGE_INSTANCE"
echo ""

# Save token to file for easy access
echo "$TOKEN" > /tmp/edge-sync-token.txt
echo "Token saved to: /tmp/edge-sync-token.txt"
