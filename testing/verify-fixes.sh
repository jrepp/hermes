#!/bin/bash
# Quick verification script for RFC-088 fixes

set -e

echo "=========================================="
echo "RFC-088 Fix Verification"
echo "=========================================="
echo ""

echo "1. Checking Dockerfile..."
if grep -q "hermes-notify" Dockerfile && grep -q "hermes-indexer" Dockerfile; then
    echo "   ✅ Dockerfile has correct binary names"
else
    echo "   ❌ Dockerfile issues found"
    exit 1
fi

echo ""
echo "2. Checking docker-compose.yml..."
if grep -q "hermes-indexer:" docker-compose.yml && ! grep -q "indexer-relay:" docker-compose.yml; then
    echo "   ✅ docker-compose.yml updated correctly"
else
    echo "   ❌ docker-compose.yml issues found"
    exit 1
fi

echo ""
echo "3. Checking config-central.hcl..."
if grep -q "redpanda_brokers" config-central.hcl; then
    echo "   ✅ config-central.hcl has RFC-088 relay config"
else
    echo "   ❌ config-central.hcl missing relay config"
    exit 1
fi

echo ""
echo "=========================================="
echo "✅ All fixes verified successfully!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. docker compose build"
echo "  2. docker compose up -d"
echo "  3. docker compose logs -f hermes-central | grep relay"
echo "  4. ./test-comprehensive-e2e.sh --verbose"
