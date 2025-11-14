# Edge Sync Authentication Testing

This document describes the authentication testing infrastructure for RFC-085 & RFC-086 implementation.

## Overview

The edge sync authentication system uses Bearer tokens with SHA-256 hashing to secure edge-to-central Hermes communication. This testing infrastructure validates that the authentication works correctly.

## Test Files

### 1. Go Integration Tests

**File**: `internal/api/v2/edge_sync_auth_test.go`

Comprehensive Go unit and integration tests for the authentication middleware.

**Run tests**:
```bash
# Run all tests
go test ./internal/api/v2/edge_sync_auth_test.go -v

# Run with short flag (skips integration tests)
go test ./internal/api/v2/edge_sync_auth_test.go -v -short

# Run specific test
go test ./internal/api/v2/edge_sync_auth_test.go -v -run TestEdgeSyncAuthenticationFlow
```

**Test Coverage**:
- ✅ Missing Authorization header rejection
- ✅ Invalid Authorization format rejection
- ✅ Empty Bearer token rejection
- ✅ Non-existent token rejection
- ✅ Revoked token rejection
- ✅ Expired token rejection
- ✅ Wrong token type rejection (registration vs edge/api)
- ✅ Valid edge token acceptance
- ✅ Valid API token acceptance
- ✅ Future expiration token acceptance
- ✅ Actual endpoint testing (sync-status, register, stats)

### 2. Shell Integration Tests

**File**: `testing/test-edge-sync-auth.sh`

End-to-end integration tests using curl to test the live API.

**Run tests**:
```bash
cd testing

# Start services
docker compose up -d

# Run tests
./test-edge-sync-auth.sh

# Or specify custom URL
./test-edge-sync-auth.sh http://localhost:8000
```

**Test Coverage**:
- ✅ Prerequisites check (database, tables, services)
- ✅ Token generation and storage
- ✅ Unauthenticated request rejection
- ✅ Invalid Authorization format rejection
- ✅ Invalid token rejection
- ✅ Valid token acceptance (sync-status endpoint)
- ✅ Valid token acceptance (stats endpoint)
- ✅ Token revocation enforcement
- ✅ Token expiration enforcement
- ✅ Wrong token type rejection

**Expected Output**:
```
=================================================================
RFC-085 Edge Sync API - Authentication Integration Tests
=================================================================
Central URL: http://localhost:8000
Edge Instance: edge-dev-test

=================================================================
Prerequisites Check
=================================================================
✓ PASS: Central Hermes is accessible
✓ PASS: PostgreSQL is accessible
✓ PASS: service_tokens table exists
✓ PASS: edge_document_registry table exists

...

=================================================================
Test Summary
=================================================================

Total Tests: 8
Passed: 8
Failed: 0

╔════════════════════════════════════════════════════════╗
║  ✓ All authentication tests passed successfully!      ║
║                                                        ║
║  RFC-086 Bearer Token Authentication: VERIFIED ✓      ║
╚════════════════════════════════════════════════════════╝
```

### 3. Token Generation Script

**File**: `testing/create-edge-token.sh`

Creates edge sync tokens for manual testing or deployment.

**Usage**:
```bash
cd testing

# Create token with default instance name
./create-edge-token.sh

# Create token for specific edge instance
./create-edge-token.sh edge-prod-01

# Token is saved to /tmp/edge-sync-token.txt
```

**Example Output**:
```
Creating edge sync token for instance: edge-dev-1

Generated token: hermes-edge-token-a1b2c3d4-5678-90ab-cdef-1234567890ab-a1b2c3d4e5f6g7h8
Token hash: f2e58e9517796bc3266a90dab28db555f5150b7ca81ed9dae6d53e643bb9bc5b

✓ Token created successfully!

Use this token for edge sync API calls:
  Authorization: Bearer hermes-edge-token-a1b2c3d4-...

Example:
  curl -H "Authorization: Bearer hermes-edge-token-a1b2c3d4-..." \
    http://localhost:8000/api/v2/edge/documents/sync-status?edge_instance=edge-dev-1

Token saved to: /tmp/edge-sync-token.txt
```

## Running Tests in CI/CD

### GitHub Actions Example

```yaml
name: Edge Sync Authentication Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:17.1-alpine
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: hermes_testing
        ports:
          - 5433:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Run migrations
        run: |
          go build ./cmd/hermes-migrate
          ./hermes-migrate -database "postgres://postgres:postgres@localhost:5433/hermes_testing?sslmode=disable"

      - name: Run Go integration tests
        run: |
          go test ./internal/api/v2/edge_sync_auth_test.go -v

      - name: Start Hermes server
        run: |
          go build ./cmd/hermes
          ./hermes server -config testing/config-central.hcl &
          sleep 5

      - name: Run shell integration tests
        run: |
          cd testing
          ./test-edge-sync-auth.sh http://localhost:8000
```

### Docker Compose Testing

```yaml
# testing/docker-compose.test.yml
version: '3.8'

services:
  postgres:
    image: postgres:17.1-alpine
    environment:
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: hermes_testing
    ports:
      - "5433:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

  migrate:
    build:
      context: ..
      dockerfile: testing/Dockerfile.hermes
    depends_on:
      postgres:
        condition: service_healthy
    command: /app/hermes-migrate -database "postgres://postgres:postgres@postgres:5432/hermes_testing?sslmode=disable"

  hermes-central:
    build:
      context: ..
      dockerfile: testing/Dockerfile.hermes
    depends_on:
      migrate:
        condition: service_completed_successfully
    ports:
      - "8000:8000"
    command: /app/hermes server -config /app/config-central.hcl

  test-runner:
    build:
      context: ..
      dockerfile: testing/Dockerfile.hermes
    depends_on:
      - hermes-central
    command: /app/test-edge-sync-auth.sh http://hermes-central:8000
```

**Run Docker tests**:
```bash
cd testing
docker compose -f docker-compose.test.yml up --abort-on-container-exit
```

## Manual Testing

### 1. Create a Token

```bash
cd testing
./create-edge-token.sh my-edge-instance
```

Save the token output.

### 2. Test Authentication

```bash
# Save token to variable
TOKEN="hermes-edge-token-..."

# Test unauthenticated (should fail with 401)
curl -v http://localhost:8000/api/v2/edge/documents/sync-status?edge_instance=my-edge-instance

# Test with Bearer token (should succeed with 200)
curl -v -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v2/edge/documents/sync-status?edge_instance=my-edge-instance
```

### 3. Test Token Revocation

```bash
# Get token hash
TOKEN_HASH=$(printf "%s" "$TOKEN" | shasum -a 256 | awk '{print $1}')

# Revoke token
docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c \
  "UPDATE service_tokens SET revoked = true, revoked_at = NOW(), revoked_reason = 'Manual test' WHERE token_hash = '$TOKEN_HASH';"

# Test with revoked token (should fail with 401)
curl -v -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v2/edge/documents/sync-status?edge_instance=my-edge-instance
```

### 4. Test Token Expiration

```bash
# Create token with 5 minute expiration
TOKEN="hermes-edge-token-$(uuidgen | tr '[:upper:]' '[:lower:]')-$(openssl rand -hex 8)"
TOKEN_HASH=$(printf "%s" "$TOKEN" | shasum -a 256 | awk '{print $1}')

docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c \
  "INSERT INTO service_tokens (id, created_at, updated_at, token_hash, token_type, revoked, expires_at)
   VALUES (gen_random_uuid(), NOW(), NOW(), '$TOKEN_HASH', 'edge', false, NOW() + INTERVAL '5 minutes');"

# Test immediately (should succeed)
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v2/edge/documents/sync-status?edge_instance=test

# Wait 6 minutes and test again (should fail with 401)
sleep 360
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v2/edge/documents/sync-status?edge_instance=test
```

## Troubleshooting

### Tests Fail with "PostgreSQL not available"

**Solution**: Ensure PostgreSQL is running and accessible:
```bash
docker compose -f testing/docker-compose.yml up -d postgres
docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c "SELECT 1"
```

### Tests Fail with "table does not exist"

**Solution**: Run migrations:
```bash
go build ./cmd/hermes-migrate
./hermes-migrate -database "postgres://postgres:postgres@localhost:5433/hermes_testing?sslmode=disable"
```

### Tests Fail with "Central Hermes not accessible"

**Solution**: Start Hermes server:
```bash
go build ./cmd/hermes
./hermes server -config testing/config-central.hcl
```

Or with Docker:
```bash
cd testing
docker compose up -d hermes-central
```

### Token Validation Fails

**Debug token hash**:
```bash
TOKEN="your-token-here"
TOKEN_HASH=$(printf "%s" "$TOKEN" | shasum -a 256 | awk '{print $1}')

echo "Token: $TOKEN"
echo "Hash: $TOKEN_HASH"

docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c \
  "SELECT id, token_type, revoked, expires_at FROM service_tokens WHERE token_hash = '$TOKEN_HASH';"
```

### Check Server Logs

```bash
# Docker logs
docker logs hermes-central

# Local server logs
tail -f /tmp/hermes-server.log

# Look for authentication errors
grep -i "auth\|token" /tmp/hermes-server.log
```

## Security Considerations

### Test Tokens

- Test tokens are automatically cleaned up after tests
- Never commit test tokens to version control
- Use unique tokens for each test run
- Clean up tokens after testing:
  ```bash
  docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c \
    "DELETE FROM service_tokens WHERE token_type = 'edge' AND created_at < NOW() - INTERVAL '1 hour';"
  ```

### Production Tokens

- Generate production tokens on the target server (not locally)
- Use strong random values (uuidgen + openssl rand)
- Store tokens securely (encrypted secrets management)
- Set expiration times (30-90 days recommended)
- Implement token rotation schedule
- Monitor token usage and failed authentications
- Revoke compromised tokens immediately

## References

- **RFC-085**: Multi-Provider Architecture with Document Synchronization
- **RFC-086**: Authentication and Bearer Token Management
- **Implementation Guide**: `docs/development/RFC-085-PHASE3-AUTHENTICATION.md`
- **Token Rotation Guide**: `docs/development/EDGE-TOKEN-ROTATION-GUIDE.md`
- **API Documentation**: `internal/api/v2/edge_sync.go`
- **Auth Middleware**: `internal/api/v2/edge_sync_auth.go`

## Support

For issues with authentication testing:

1. Check this document's troubleshooting section
2. Review server logs for authentication errors
3. Verify database migrations are applied
4. Ensure token format matches specification
5. Check token hash calculation matches server implementation

## Test Maintenance

### Adding New Tests

1. **Go tests**: Add new test functions to `edge_sync_auth_test.go`
2. **Shell tests**: Add new test sections to `test-edge-sync-auth.sh`
3. Update this document with new test coverage
4. Run all tests to ensure no regressions

### Updating Token Format

If token format changes:

1. Update `create-edge-token.sh` generation logic
2. Update `generateTestToken()` in Go tests
3. Update documentation and examples
4. Update token validation in `models.GenerateToken()`
5. Run full test suite to verify compatibility

---

**Last Updated**: 2025-11-13
**Test Coverage**: 18+ test scenarios
**Status**: ✅ All tests passing
