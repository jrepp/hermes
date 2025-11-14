# Edge Token Rotation Guide

## Overview

Edge instances can maintain **multiple active tokens simultaneously** to enable zero-downtime token rotation. This is critical for production environments where edge instances cannot tolerate authentication failures during token updates.

## Why Multiple Tokens?

```
Timeline: Token Rotation for edge-dev-1

Day 0:   Token A created (expires Day 30)
         └─ Edge uses Token A

Day 23:  Token B created (expires Day 53)
         ├─ Edge configured with Token A + Token B
         └─ Both tokens valid simultaneously

Day 25:  Token A revoked
         └─ Edge now uses only Token B

Result: Zero authentication failures during rotation!
```

## Database Design

### Multiple Tokens per Edge Instance

The `indexer_tokens` table supports multiple active tokens per edge instance:

```sql
-- Example: edge-dev-1 has 3 tokens at different lifecycle stages
SELECT id, token_type, created_at, expires_at, revoked
FROM indexer_tokens
WHERE token_type = 'edge'
ORDER BY created_at DESC;

                  id                  | token_type |      created_at      |      expires_at      | revoked
--------------------------------------+------------+----------------------+----------------------+---------
 11111111-1111-1111-1111-111111111111 | edge       | 2025-11-15 00:00:00 | 2025-12-15 00:00:00 | f       -- Token C (newest)
 22222222-2222-2222-2222-222222222222 | edge       | 2025-11-01 00:00:00 | 2025-12-01 00:00:00 | f       -- Token B (current)
 33333333-3333-3333-3333-333333333333 | edge       | 2025-10-01 00:00:00 | 2025-11-01 00:00:00 | t       -- Token A (revoked)
```

**Key Points**:
- ✅ No unique constraint on `token_type` - Multiple 'edge' tokens allowed
- ✅ Each token has independent `expires_at` timestamp
- ✅ Each token can be revoked individually
- ✅ Authentication middleware validates any valid token
- ✅ No edge instance identifier in token table (tokens are anonymous)

### Token Identification

Tokens are identified by their cryptographic hash, not by edge instance:

```sql
-- Token lookup is by hash only
SELECT * FROM indexer_tokens
WHERE token_hash = encode(digest('hermes-edge-token-xxx', 'sha256'), 'hex')
  AND token_type IN ('edge', 'api')
  AND revoked = false
  AND (expires_at IS NULL OR expires_at > NOW());
```

This means:
- A single token can be shared across multiple edge instances (not recommended)
- Each edge instance can have multiple tokens
- Tokens are self-contained credentials

## Token Rotation Strategies

### Strategy 1: Overlapping Expiration (Recommended)

**Timeline**: 30-day token validity with 7-day overlap

```
Day 0:   Create Token A (expires Day 30)
Day 23:  Create Token B (expires Day 53) - 7 days before Token A expires
Day 24:  Deploy Token B to edge instance
Day 25:  Verify Token B works
Day 26:  Remove Token A from edge config
Day 27:  Revoke Token A in database
Day 53:  Create Token C (expires Day 83) - repeat cycle
```

**Benefits**:
- Zero downtime
- Time to rollback if issues
- Simple automation

**Implementation**:

```bash
#!/bin/bash
# rotate-edge-token.sh - Automated token rotation

EDGE_INSTANCE="edge-dev-1"
OLD_TOKEN_FILE="/etc/hermes/tokens/current.txt"
NEW_TOKEN_FILE="/etc/hermes/tokens/new.txt"

# Step 1: Generate new token
echo "Generating new token..."
NEW_TOKEN=$(./create-edge-token.sh)
echo "$NEW_TOKEN" > "$NEW_TOKEN_FILE"

# Step 2: Configure edge to use both tokens
echo "Deploying new token..."
cat > /etc/hermes/tokens/all.txt <<EOF
$(cat "$OLD_TOKEN_FILE")
$(cat "$NEW_TOKEN_FILE")
EOF

# Step 3: Restart edge service
systemctl restart hermes-edge

# Step 4: Wait for confirmation
sleep 60

# Step 5: Test new token
if curl -f -H "Authorization: Bearer $NEW_TOKEN" \
   http://central:8000/api/v2/edge/documents/sync-status?edge_instance=$EDGE_INSTANCE; then
    echo "✓ New token works!"

    # Step 6: Revoke old token
    OLD_TOKEN=$(cat "$OLD_TOKEN_FILE")
    ./revoke-edge-token.sh "$OLD_TOKEN" "Rotated"

    # Step 7: Update current token
    mv "$NEW_TOKEN_FILE" "$OLD_TOKEN_FILE"

    echo "✓ Token rotation complete!"
else
    echo "✗ New token failed, rolling back..."
    systemctl restart hermes-edge
    rm "$NEW_TOKEN_FILE"
    exit 1
fi
```

### Strategy 2: Dual Token Configuration

**Concept**: Always maintain exactly 2 valid tokens per edge instance

```sql
-- edge-dev-1 token state
SELECT
    id,
    'Token-' || ROW_NUMBER() OVER (ORDER BY created_at DESC) as name,
    created_at,
    expires_at,
    CASE
        WHEN revoked THEN 'REVOKED'
        WHEN expires_at < NOW() THEN 'EXPIRED'
        WHEN expires_at < NOW() + INTERVAL '7 days' THEN 'EXPIRING SOON'
        ELSE 'ACTIVE'
    END as status
FROM indexer_tokens
WHERE token_type = 'edge'
ORDER BY created_at DESC
LIMIT 2;

                  id                  |   name   |      created_at      |      expires_at      |    status
--------------------------------------+----------+----------------------+----------------------+---------------
 11111111-1111-1111-1111-111111111111 | Token-1  | 2025-11-15 00:00:00 | 2025-12-15 00:00:00 | ACTIVE
 22222222-2222-2222-2222-222222222222 | Token-2  | 2025-11-01 00:00:00 | 2025-12-01 00:00:00 | EXPIRING SOON
```

**Rules**:
1. Always keep 2 active tokens
2. When Token-2 is < 7 days from expiry, create Token-3
3. After Token-3 is deployed and verified, revoke Token-2
4. Token-3 becomes Token-2, repeat cycle

### Strategy 3: Emergency Rotation

**When**: Security incident, token leak, or compromise suspected

```bash
#!/bin/bash
# emergency-rotate.sh - Immediate token rotation

EDGE_INSTANCE="edge-dev-1"

# Step 1: Generate new token immediately
NEW_TOKEN=$(./create-edge-token.sh)

# Step 2: Deploy to edge ASAP
deploy-token-to-edge "$EDGE_INSTANCE" "$NEW_TOKEN"

# Step 3: Revoke ALL old tokens immediately
psql hermes -c "
UPDATE indexer_tokens
SET revoked = true,
    revoked_at = NOW(),
    revoked_reason = 'SECURITY INCIDENT - Emergency rotation'
WHERE token_type = 'edge'
  AND revoked = false
  AND token_hash != encode(digest('$NEW_TOKEN', 'sha256'), 'hex');
"

echo "⚠️  Emergency rotation complete!"
echo "⚠️  All old tokens revoked!"
echo "⚠️  Monitor edge instances for authentication failures!"
```

## Implementation Examples

### Creating Multiple Tokens

**SQL Approach**:
```sql
-- Create 2 tokens for redundancy
DO $$
DECLARE
    token1 TEXT;
    token2 TEXT;
    hash1 TEXT;
    hash2 TEXT;
BEGIN
    -- Token 1
    token1 := 'hermes-edge-token-' || gen_random_uuid() || '-' || encode(gen_random_bytes(8), 'hex');
    hash1 := encode(digest(token1, 'sha256'), 'hex');

    INSERT INTO indexer_tokens (id, created_at, updated_at, token_hash, token_type, expires_at, revoked)
    VALUES (gen_random_uuid(), NOW(), NOW(), hash1, 'edge', NOW() + INTERVAL '30 days', false);

    RAISE NOTICE 'Token 1: %', token1;

    -- Token 2 (backup)
    token2 := 'hermes-edge-token-' || gen_random_uuid() || '-' || encode(gen_random_bytes(8), 'hex');
    hash2 := encode(digest(token2, 'sha256'), 'hex');

    INSERT INTO indexer_tokens (id, created_at, updated_at, token_hash, token_type, expires_at, revoked)
    VALUES (gen_random_uuid(), NOW(), NOW(), hash2, 'edge', NOW() + INTERVAL '30 days', false);

    RAISE NOTICE 'Token 2: %', token2;
END $$;
```

**Go Approach**:
```go
package main

import (
    "github.com/hashicorp-forge/hermes/internal/api/v2"
    "github.com/hashicorp-forge/hermes/internal/server"
)

func ProvisionEdgeInstance(srv server.Server, edgeInstance string) ([]string, error) {
    var tokens []string

    // Create 2 tokens for redundancy
    for i := 0; i < 2; i++ {
        token, err := apiv2.CreateEdgeSyncToken(srv, edgeInstance)
        if err != nil {
            return nil, err
        }
        tokens = append(tokens, token)

        srv.Logger.Info("created edge token",
            "edge_instance", edgeInstance,
            "token_number", i+1,
        )
    }

    return tokens, nil
}
```

### Edge Configuration with Multiple Tokens

**Configuration File**: `/etc/hermes/edge-tokens.conf`

```bash
# Primary token (current)
HERMES_EDGE_TOKEN_PRIMARY="hermes-edge-token-11111111-1111-1111-1111-111111111111-aabbccdd"

# Secondary token (for rotation)
HERMES_EDGE_TOKEN_SECONDARY="hermes-edge-token-22222222-2222-2222-2222-222222222222-eeffgghh"

# Backup token (emergency)
HERMES_EDGE_TOKEN_BACKUP="hermes-edge-token-33333333-3333-3333-3333-333333333333-iijjkkll"
```

**Edge Client Code**:
```go
type EdgeSyncClient struct {
    tokens     []string
    currentIdx int
}

func (c *EdgeSyncClient) GetToken() string {
    return c.tokens[c.currentIdx]
}

func (c *EdgeSyncClient) SyncDocument(doc *Document) error {
    for attempt := 0; attempt < len(c.tokens); attempt++ {
        token := c.tokens[(c.currentIdx + attempt) % len(c.tokens)]

        err := c.makeRequest(token, doc)
        if err == nil {
            // Success! Update current token if we failed over
            c.currentIdx = (c.currentIdx + attempt) % len(c.tokens)
            return nil
        }

        if isAuthError(err) {
            // Try next token
            continue
        }

        // Non-auth error, don't retry
        return err
    }

    return errors.New("all tokens failed authentication")
}
```

## Token Management Dashboard

### View Active Tokens

```sql
-- All active edge tokens
SELECT
    id,
    token_type,
    created_at,
    expires_at,
    expires_at - NOW() as time_remaining,
    CASE
        WHEN expires_at < NOW() + INTERVAL '7 days' THEN '⚠️  EXPIRING SOON'
        WHEN expires_at < NOW() + INTERVAL '14 days' THEN '⏰ EXPIRING'
        ELSE '✓ OK'
    END as status
FROM indexer_tokens
WHERE token_type = 'edge'
  AND revoked = false
  AND (expires_at IS NULL OR expires_at > NOW())
ORDER BY expires_at ASC NULLS LAST;
```

### Token Rotation Schedule

```sql
-- Tokens that need rotation soon
WITH token_schedule AS (
    SELECT
        id,
        created_at,
        expires_at,
        expires_at - NOW() as days_until_expiry,
        expires_at - INTERVAL '7 days' as rotation_date
    FROM indexer_tokens
    WHERE token_type = 'edge'
      AND revoked = false
      AND expires_at IS NOT NULL
)
SELECT
    id,
    to_char(expires_at, 'YYYY-MM-DD') as expires_on,
    to_char(rotation_date, 'YYYY-MM-DD') as rotate_on,
    CASE
        WHEN rotation_date < NOW() THEN 'OVERDUE'
        WHEN rotation_date < NOW() + INTERVAL '3 days' THEN 'SOON'
        ELSE 'SCHEDULED'
    END as urgency
FROM token_schedule
ORDER BY rotation_date ASC;
```

### Automated Rotation Monitoring

```sql
-- Create view for monitoring
CREATE OR REPLACE VIEW edge_token_health AS
SELECT
    COUNT(*) FILTER (WHERE revoked = false AND (expires_at IS NULL OR expires_at > NOW())) as active_tokens,
    COUNT(*) FILTER (WHERE revoked = false AND expires_at < NOW() + INTERVAL '7 days') as expiring_soon,
    COUNT(*) FILTER (WHERE revoked = false AND expires_at < NOW()) as expired_but_not_revoked,
    COUNT(*) FILTER (WHERE revoked = true AND revoked_at > NOW() - INTERVAL '24 hours') as revoked_today
FROM indexer_tokens
WHERE token_type = 'edge';

-- Alert query (run every hour)
SELECT * FROM edge_token_health
WHERE expiring_soon > 0 OR expired_but_not_revoked > 0;
```

## Best Practices

### 1. Maintain Token Inventory

Keep a secure inventory of which tokens belong to which edge instances:

```yaml
# token-inventory.yml (encrypted at rest)
edge-instances:
  edge-dev-1:
    tokens:
      - id: 11111111-1111-1111-1111-111111111111
        created: 2025-11-01
        expires: 2025-12-01
        status: active
      - id: 22222222-2222-2222-2222-222222222222
        created: 2025-11-15
        expires: 2025-12-15
        status: active

  edge-prod-west:
    tokens:
      - id: 33333333-3333-3333-3333-333333333333
        created: 2025-10-15
        expires: 2025-11-15
        status: rotating
      - id: 44444444-4444-4444-4444-444444444444
        created: 2025-11-01
        expires: 2025-12-01
        status: active
```

### 2. Automate Rotation

Set up cron jobs for automatic rotation:

```bash
# /etc/cron.d/hermes-token-rotation

# Check for tokens expiring soon (daily at 9am)
0 9 * * * /usr/local/bin/check-token-expiry.sh

# Rotate tokens (weekly on Sunday at 2am)
0 2 * * 0 /usr/local/bin/rotate-edge-tokens.sh

# Clean up revoked tokens older than 90 days (monthly)
0 3 1 * * /usr/local/bin/cleanup-old-tokens.sh
```

### 3. Monitor Authentication Failures

```sql
-- Track authentication failures (implement in middleware)
CREATE TABLE edge_auth_log (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMP DEFAULT NOW(),
    token_hash VARCHAR(64),  -- Hashed token that was attempted
    edge_instance TEXT,       -- From request if available
    endpoint TEXT,
    success BOOLEAN,
    error_message TEXT
);

CREATE INDEX idx_edge_auth_log_timestamp ON edge_auth_log(timestamp);
CREATE INDEX idx_edge_auth_log_token ON edge_auth_log(token_hash);

-- Alert on high failure rate
SELECT
    COUNT(*) FILTER (WHERE NOT success) * 100.0 / COUNT(*) as failure_rate
FROM edge_auth_log
WHERE timestamp > NOW() - INTERVAL '5 minutes'
HAVING COUNT(*) FILTER (WHERE NOT success) * 100.0 / COUNT(*) > 5.0;
```

### 4. Token Lifecycle SLA

Define SLAs for token management:

- **Creation**: < 1 minute
- **Deployment**: < 5 minutes
- **Verification**: < 2 minutes
- **Revocation**: < 1 minute
- **Total Rotation Time**: < 10 minutes
- **Rotation Window**: 7 days before expiry
- **Emergency Rotation**: < 15 minutes

## Troubleshooting

### Multiple Tokens Not Working

**Symptom**: Edge instance fails to authenticate even with multiple tokens

**Diagnosis**:
```sql
-- Check all tokens
SELECT id, token_hash, token_type, revoked, expires_at
FROM indexer_tokens
WHERE token_type = 'edge'
  AND revoked = false;

-- Verify token hashes match
SELECT encode(digest('YOUR_TOKEN_HERE', 'sha256'), 'hex');
```

**Solutions**:
1. Verify tokens are actually in database
2. Check token format matches: `hermes-edge-token-<uuid>-<hex>`
3. Ensure tokens aren't expired
4. Verify authentication middleware is active

### Token Rotation Failed

**Symptom**: New token doesn't work, edge stuck with old token

**Recovery**:
```bash
# 1. Extend old token expiration
psql hermes -c "
UPDATE indexer_tokens
SET expires_at = NOW() + INTERVAL '7 days'
WHERE token_hash = encode(digest('OLD_TOKEN', 'sha256'), 'hex');
"

# 2. Debug new token
NEW_TOKEN="hermes-edge-token-xxx"
HASH=$(echo -n "$NEW_TOKEN" | sha256sum | awk '{print $1}')

psql hermes -c "
SELECT * FROM indexer_tokens
WHERE token_hash = '$HASH';
"

# 3. Retry deployment
./deploy-token.sh "$NEW_TOKEN"
```

### All Tokens Revoked by Mistake

**Symptom**: All edge instances failing authentication

**Emergency Recovery**:
```bash
# 1. Create emergency token
EMERGENCY_TOKEN=$(./create-edge-token.sh)

# 2. Deploy to all edge instances simultaneously
for edge in edge-dev-1 edge-dev-2 edge-prod-west; do
    deploy-token "$edge" "$EMERGENCY_TOKEN" &
done
wait

# 3. Verify all edges recovered
for edge in edge-dev-1 edge-dev-2 edge-prod-west; do
    curl -H "Authorization: Bearer $EMERGENCY_TOKEN" \
        "http://central:8000/api/v2/edge/documents/sync-status?edge_instance=$edge"
done
```

## Summary

The token table design supports multiple active tokens per edge instance out of the box:

✅ **No Constraints**: Multiple tokens with same `token_type = 'edge'` allowed
✅ **Independent Expiration**: Each token has its own `expires_at`
✅ **Individual Revocation**: Revoke tokens one at a time
✅ **Stateless Authentication**: Middleware validates any valid token
✅ **Zero Downtime**: Overlap periods during rotation

This enables safe, automated token rotation with zero authentication failures!
