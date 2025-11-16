# Best Practices Guide
## RFC-088 Semantic Search and Document Indexer

**Version**: 2.0
**Audience**: All Users (Developers, Operators, Administrators)
**Last Updated**: November 15, 2025

---

## Overview

This guide provides best practices for deploying, operating, and maintaining the RFC-088 Event-Driven Document Indexer with Semantic Search in production environments.

**Key Topics**:
- Production deployment
- Security considerations
- Cost optimization
- Scalability patterns
- Backup and recovery
- Operational excellence

---

## Table of Contents

1. [Production Deployment](#production-deployment)
2. [Security Best Practices](#security-best-practices)
3. [Cost Optimization](#cost-optimization)
4. [Scalability Patterns](#scalability-patterns)
5. [Backup and Recovery](#backup-and-recovery)
6. [Operational Excellence](#operational-excellence)
7. [Development Workflow](#development-workflow)
8. [Common Pitfalls](#common-pitfalls)

---

## Production Deployment

### Pre-Deployment Checklist

Before deploying to production:

#### Database Setup

- [ ] **PostgreSQL Version**: Use PostgreSQL 14+ for best pgvector performance
- [ ] **pgvector Extension**: Install and verify extension version (0.5.0+)
- [ ] **Database Indexes**: Create IVFFlat or HNSW indexes on `embedding_vector`
- [ ] **Lookup Indexes**: Create composite index on `(document_id, model)`
- [ ] **Statistics**: Run `ANALYZE` on `document_embeddings` table
- [ ] **Memory Configuration**: Tune `shared_buffers`, `work_mem` (see [Performance Tuning](../deployment/performance-tuning.md))
- [ ] **Backup Strategy**: Configure automated backups with point-in-time recovery

#### Application Configuration

- [ ] **Connection Pooling**: Configure appropriate pool size for expected traffic
- [ ] **Authentication**: Use secure authentication (OAuth, JWT, mTLS)
- [ ] **Rate Limiting**: Configure per-user and per-IP rate limits
- [ ] **Logging**: Enable structured logging with appropriate log levels
- [ ] **Metrics**: Configure Prometheus metrics export
- [ ] **Health Checks**: Verify `/health` endpoint responds correctly

#### Infrastructure

- [ ] **High Availability**: Deploy multiple replicas (minimum 2 for API servers)
- [ ] **Load Balancer**: Configure load balancing with health checks
- [ ] **SSL/TLS**: Use valid certificates (Let's Encrypt, AWS ACM, etc.)
- [ ] **Networking**: Configure proper VPC, subnets, security groups
- [ ] **Resource Limits**: Set CPU and memory limits in orchestrator (Kubernetes, ECS)
- [ ] **Monitoring**: Set up dashboards and alerts

#### External Dependencies

- [ ] **Kafka/Redpanda**: Verify message broker is running and accessible
- [ ] **Meilisearch**: Verify keyword search is indexed and responsive
- [ ] **OpenAI API**: Verify API key is valid and has sufficient quota
- [ ] **S3 Storage**: Verify bucket access and lifecycle policies

### Deployment Strategy

#### Blue-Green Deployment

Recommended for production deployments to minimize downtime:

```bash
# Step 1: Deploy new version (green) alongside existing (blue)
kubectl apply -f deployment-green.yaml

# Step 2: Wait for green to be healthy
kubectl rollout status deployment/hermes-api-green

# Step 3: Run smoke tests against green
./scripts/smoke-tests.sh https://green.hermes.internal

# Step 4: Switch traffic to green
kubectl patch service hermes-api -p '{"spec":{"selector":{"version":"green"}}}'

# Step 5: Monitor for errors
kubectl logs -f deployment/hermes-api-green

# Step 6: If successful, decommission blue
kubectl delete deployment hermes-api-blue

# Step 7: If issues, rollback to blue
kubectl patch service hermes-api -p '{"spec":{"selector":{"version":"blue"}}}'
```

#### Rolling Update

For less critical updates:

```bash
# Update deployment with new image
kubectl set image deployment/hermes-api hermes=hermes:v2.0.1

# Monitor rollout
kubectl rollout status deployment/hermes-api

# Rollback if needed
kubectl rollout undo deployment/hermes-api
```

### Environment Configuration

**Development**:
```hcl
environment = "development"

database {
  max_idle_conns = 5
  max_open_conns = 10
}

log_level = "debug"
```

**Staging**:
```hcl
environment = "staging"

database {
  max_idle_conns = 10
  max_open_conns = 25
}

log_level = "info"
```

**Production**:
```hcl
environment = "production"

database {
  max_idle_conns = 25
  max_open_conns = 50
  conn_max_lifetime = "5m"
}

log_level = "warn"
rate_limit_enabled = true
```

---

## Security Best Practices

### Authentication and Authorization

#### API Authentication

**Use Bearer Tokens**:
```javascript
// Good: Secure token-based authentication
const response = await fetch('/api/v2/search/semantic', {
  headers: {
    'Authorization': `Bearer ${secureToken}`,
  },
});
```

**Never Hard-Code Credentials**:
```javascript
// Bad: Hard-coded credentials
const token = "sk-1234567890abcdef";

// Good: Load from environment or secrets manager
const token = process.env.HERMES_API_TOKEN;
```

#### Access Control

Implement proper authorization:

```go
// Example: Role-based access control
func (h *Handler) SearchSemanticHandler(w http.ResponseWriter, r *http.Request) {
    user := GetUserFromContext(r.Context())

    // Check if user has search permission
    if !user.HasPermission("search:semantic") {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Proceed with search
    // ...
}
```

### Data Protection

#### Sensitive Data

**Document Content**:
- Classify documents by sensitivity level
- Use document-level access controls
- Audit access to sensitive documents
- Consider encrypting sensitive fields at rest

**Personal Information**:
- Redact PII before embedding generation
- Implement data retention policies
- Support right-to-be-forgotten requests (document deletion)

#### API Security

**Rate Limiting**:
```go
// Implement rate limiting per user
rateLimiter := NewRateLimiter(100, time.Minute) // 100 requests per minute

if !rateLimiter.Allow(userID) {
    http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
    return
}
```

**Input Validation**:
```go
// Validate all inputs
if query == "" {
    http.Error(w, "query cannot be empty", http.StatusBadRequest)
    return
}

if limit < 1 || limit > 100 {
    http.Error(w, "limit must be between 1 and 100", http.StatusBadRequest)
    return
}
```

### Network Security

#### TLS Configuration

**Always Use HTTPS**:
```nginx
# nginx configuration
server {
    listen 443 ssl http2;
    server_name hermes.example.com;

    ssl_certificate /etc/ssl/certs/hermes.crt;
    ssl_certificate_key /etc/ssl/private/hermes.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    location / {
        proxy_pass http://hermes-api:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name hermes.example.com;
    return 301 https://$host$request_uri;
}
```

#### Database Security

**Connection Security**:
```hcl
database {
  host     = "db.internal.example.com"
  port     = 5432
  sslmode  = "require"  # Always use SSL
  user     = "hermes_app"
  password = "${env.DB_PASSWORD}"  # Never hard-code!
}
```

**Network Isolation**:
- Place database in private subnet
- Use security groups to restrict access
- Only allow connections from application servers

### Secrets Management

**Use Secrets Manager**:
```bash
# Bad: Environment variables in deployment files
env:
  - name: OPENAI_API_KEY
    value: "sk-1234567890abcdef"

# Good: Load from secrets manager
env:
  - name: OPENAI_API_KEY
    valueFrom:
      secretKeyRef:
        name: hermes-secrets
        key: openai-api-key
```

**Rotate Credentials Regularly**:
- Database passwords: Every 90 days
- API keys: Every 180 days
- TLS certificates: Before expiration

---

## Cost Optimization

### Embedding Generation Costs

OpenAI API costs are the primary expense. Optimize with:

#### 1. Idempotency (Already Implemented)

Content hash prevents re-generating embeddings for unchanged documents:

```go
// Automatic idempotency check
contentHash := calculateSHA256(documentContent)

existing := db.First(&DocumentEmbedding{}, "content_hash = ?", contentHash)
if existing != nil {
    // Embedding already exists, skip generation
    return existing, nil
}

// Only generate if content changed
embedding := generateEmbedding(documentContent)
```

**Savings**: 90-99% reduction in API calls for static documents

#### 2. Selective Processing with Rulesets

Only embed relevant documents:

```hcl
# Example: Only embed documentation and RFCs
ruleset {
  name = "documentation-only"

  rules {
    include = ["*.md", "*.pdf"]
    include_document_types = ["RFC", "Guide", "Documentation"]
    exclude = ["*test*", "*tmp*"]
  }
}
```

**Savings**: 50-90% reduction in documents processed

#### 3. Batch Processing (Future Enhancement)

Use OpenAI batch API for bulk operations:

```go
// Future: Batch API for 50% cost savings
batchRequest := openai.BatchEmbeddingRequest{
    Documents: documentBatch,  // Up to 2048 documents
    Model:     "text-embedding-3-small",
}

// Process overnight, check status in morning
batchID := client.CreateBatch(batchRequest)
```

**Savings**: 50% reduction in API costs

#### 4. Model Selection

Choose appropriate model for use case:

| Model | Dimensions | Cost per 1M tokens | Use Case |
|-------|------------|-------------------|----------|
| text-embedding-3-small | 1536 | $0.02 | General purpose (recommended) |
| text-embedding-3-large | 3072 | $0.13 | High accuracy requirements |
| text-embedding-ada-002 | 1536 | $0.10 | Legacy (use 3-small instead) |

**Savings**: Using text-embedding-3-small saves 80% vs ada-002

### Database Costs

#### Storage Optimization

**Embedding Size**:
- 1536 dimensions × 4 bytes = 6KB per document
- 1M documents = 6GB embeddings + 12GB indexes = 18GB total

**Reduce Storage**:
```sql
-- Delete old embeddings for deleted documents
DELETE FROM document_embeddings
WHERE document_id IN (
    SELECT de.document_id
    FROM document_embeddings de
    LEFT JOIN documents d ON de.document_id = d.id
    WHERE d.id IS NULL
);

-- Vacuum to reclaim space
VACUUM FULL document_embeddings;
```

#### Compute Optimization

**Right-Size Database**:
- Development: 2 CPU, 4GB RAM
- Staging: 4 CPU, 8GB RAM
- Production: 8 CPU, 16GB RAM (can scale up based on usage)

**Use Read Replicas**:
- Offload read queries to replicas
- Primary handles writes only
- Can use smaller primary instance

### Infrastructure Costs

#### Auto-Scaling

Scale resources based on demand:

```yaml
# Kubernetes HPA
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: hermes-api-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: hermes-api
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

#### Spot/Preemptible Instances

Use for indexer workers (can tolerate interruptions):

```yaml
# Kubernetes node selector for spot instances
spec:
  nodeSelector:
    workload-type: spot
  tolerations:
  - key: "spot"
    operator: "Equal"
    value: "true"
    effect: "NoSchedule"
```

**Savings**: 60-90% reduction in compute costs for workers

---

## Scalability Patterns

### Horizontal Scaling

#### API Servers

Scale based on request rate:

```
1-10 req/s:   2 replicas
10-100 req/s: 4-8 replicas
100+ req/s:   10+ replicas (with load testing)
```

#### Indexer Workers

Scale based on Kafka lag:

```bash
# Check Kafka lag
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group hermes-indexer --describe

# If lag > 1000 messages, add workers
kubectl scale deployment hermes-indexer --replicas=4
```

### Vertical Scaling

When to scale up instead of out:

**Database**:
- CPU: Scale when >80% utilization
- Memory: Scale when cache hit ratio <90%
- IOPS: Scale when disk queue length >10

**API Servers**:
- CPU: Scale when P95 latency >200ms
- Memory: Scale when memory >80% and increasing

### Caching Strategies

#### Application-Level Caching

Cache frequently accessed data:

```go
// Example: Cache popular search queries
type SearchCache struct {
    cache *lru.Cache
    ttl   time.Duration
}

func (c *SearchCache) Search(query string) ([]Result, bool) {
    key := fmt.Sprintf("search:%s", query)

    if cached, ok := c.cache.Get(key); ok {
        return cached.([]Result), true
    }

    return nil, false
}

func (c *SearchCache) Set(query string, results []Result) {
    key := fmt.Sprintf("search:%s", query)
    c.cache.Add(key, results)
}
```

**Cache popular queries** (5-minute TTL):
- Common searches: "kubernetes", "docker", "api"
- Autocomplete suggestions
- Similar documents for popular docs

#### Database Query Cache

PostgreSQL caches query plans and results:

```sql
-- Monitor cache hit ratio (should be >90%)
SELECT
    sum(heap_blks_read) as heap_read,
    sum(heap_blks_hit) as heap_hit,
    sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) as ratio
FROM pg_statio_user_tables;
```

### Message Queue Scaling

#### Kafka/Redpanda Configuration

**Partitions**:
```bash
# Create topic with multiple partitions for parallel processing
kafka-topics --create \
  --topic document-revisions \
  --partitions 8 \
  --replication-factor 3
```

**Consumer Groups**:
- Each worker is a consumer in the same group
- Partitions distributed across workers
- Add workers to increase throughput

---

## Backup and Recovery

### Database Backups

#### Automated Backups

**Full Backups** (Daily):
```bash
#!/bin/bash
# Automated daily backup script

BACKUP_DIR="/backups/postgres"
DATE=$(date +%Y%m%d)

# Full backup with pg_dump
pg_dump -h localhost -U hermes -d hermes \
  --format=custom \
  --file="${BACKUP_DIR}/hermes_${DATE}.dump"

# Compress
gzip "${BACKUP_DIR}/hermes_${DATE}.dump"

# Upload to S3
aws s3 cp "${BACKUP_DIR}/hermes_${DATE}.dump.gz" \
  s3://hermes-backups/postgres/${DATE}/

# Retain backups for 30 days
find "${BACKUP_DIR}" -name "*.dump.gz" -mtime +30 -delete
```

**Incremental Backups** (Hourly):
```bash
# WAL archiving for point-in-time recovery
archive_mode = on
archive_command = 'aws s3 cp %p s3://hermes-backups/wal/%f'
```

#### Backup Verification

Test restores regularly:

```bash
# Monthly restore test
pg_restore -h test-db -U hermes -d hermes_test \
  /backups/hermes_20251115.dump.gz

# Verify data integrity
psql -h test-db -U hermes -d hermes_test -c \
  "SELECT COUNT(*) FROM document_embeddings;"
```

### Disaster Recovery

#### Recovery Time Objective (RTO)

Target: **< 1 hour** for production recovery

**Recovery Steps**:
1. Provision new database instance (10 min)
2. Restore from backup (20 min for 100GB)
3. Apply WAL logs for point-in-time recovery (10 min)
4. Verify data integrity (10 min)
5. Update application configuration (5 min)
6. Switch traffic to recovered instance (5 min)

#### Recovery Point Objective (RPO)

Target: **< 5 minutes** data loss

**Achieve with**:
- Continuous WAL archiving (5-minute intervals)
- Database replication (streaming replication)
- Event sourcing (replay from Kafka)

### Data Retention Policies

Define retention based on compliance and cost:

```sql
-- Delete embeddings for documents older than 2 years
DELETE FROM document_embeddings
WHERE created_at < NOW() - INTERVAL '2 years';

-- Archive instead of delete
INSERT INTO document_embeddings_archive
SELECT * FROM document_embeddings
WHERE created_at < NOW() - INTERVAL '1 year';

DELETE FROM document_embeddings
WHERE created_at < NOW() - INTERVAL '1 year';
```

---

## Operational Excellence

### Monitoring

#### Key Metrics to Monitor

**Application Metrics**:
- Request rate (requests/second)
- Error rate (errors/second, %)
- Latency (p50, p95, p99)
- Active connections

**Search Metrics**:
- Semantic search latency
- Hybrid search latency
- Search result count distribution
- Search success rate

**Database Metrics**:
- Query latency
- Connection pool utilization
- Cache hit ratio
- Index usage
- Table size growth

**Infrastructure Metrics**:
- CPU utilization
- Memory utilization
- Disk I/O
- Network throughput

#### Alerting

Configure alerts for critical issues:

```yaml
# Example: Prometheus alert rules
groups:
- name: hermes-alerts
  rules:
  # High error rate
  - alert: HighErrorRate
    expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
    for: 5m
    annotations:
      summary: "High error rate detected"

  # High latency
  - alert: HighLatency
    expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 1.0
    for: 5m
    annotations:
      summary: "P95 latency > 1 second"

  # Database connection pool exhaustion
  - alert: ConnectionPoolExhaustion
    expr: db_connections_wait_count > 100
    for: 5m
    annotations:
      summary: "Connection pool under pressure"
```

### Logging

#### Structured Logging

Use structured logs for better observability:

```go
// Good: Structured logging
logger.Info("search completed",
    "query", query,
    "results", len(results),
    "latency_ms", latency.Milliseconds(),
    "user_id", userID,
)

// Bad: Unstructured logging
logger.Info(fmt.Sprintf("Search for '%s' returned %d results in %dms",
    query, len(results), latency.Milliseconds()))
```

#### Log Levels

Set appropriate log levels:

- **DEBUG**: Development only (verbose)
- **INFO**: Normal operations, important events
- **WARN**: Recoverable errors, degraded performance
- **ERROR**: Errors requiring attention
- **FATAL**: Critical errors, application crash

### Incident Response

#### Runbooks

Document common scenarios:

**Scenario: High Latency**
1. Check database connection pool (`GetPoolStats()`)
2. Check database query latency (`pg_stat_statements`)
3. Check index usage (`pg_stat_user_indexes`)
4. Check CPU/memory on database server
5. If needed: Scale up database, add read replicas

**Scenario: Service Down**
1. Check pod/container status
2. Check application logs for errors
3. Check database connectivity
4. Check external dependencies (Kafka, Meilisearch, OpenAI)
5. If needed: Restart service, rollback deployment

---

## Development Workflow

### Local Development

#### Setup

```bash
# 1. Start dependencies with Docker Compose
docker-compose up -d postgres kafka meilisearch

# 2. Run migrations
make migrate

# 3. Create indexes
psql -f scripts/create-indexes.sql

# 4. Start application
make run
```

#### Testing

```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Run with coverage
make test-coverage

# Benchmark tests
make benchmark
```

### Code Quality

#### Linting

```bash
# Run all linters
make lint

# Auto-fix issues
make lint-fix
```

#### Code Review Checklist

- [ ] Tests added for new functionality
- [ ] Error handling implemented
- [ ] Logging added for important operations
- [ ] Documentation updated
- [ ] Performance considered (indexes, caching, etc.)
- [ ] Security reviewed (input validation, authentication)

---

## Common Pitfalls

### 1. Missing Database Indexes

**Problem**: Semantic search takes >500ms

**Cause**: No index on `embedding_vector`

**Solution**: Create IVFFlat or HNSW index (see [Performance Tuning](../deployment/performance-tuning.md))

### 2. Connection Pool Exhaustion

**Problem**: "Too many clients" errors

**Cause**: `max_open_conns` too low

**Solution**: Increase `max_open_conns` or reduce query latency

### 3. Regenerating Embeddings Unnecessarily

**Problem**: High OpenAI API costs

**Cause**: Not checking content hash before generation

**Solution**: Idempotency is already implemented - verify it's being used

### 4. Not Monitoring Kafka Lag

**Problem**: Indexing falling behind

**Cause**: Not enough workers or workers are slow

**Solution**: Monitor lag, scale workers, optimize processing

### 5. Hardcoded Credentials

**Problem**: Security vulnerability

**Cause**: Credentials in code or config files

**Solution**: Use environment variables or secrets manager

### 6. No Rate Limiting

**Problem**: API abuse, high costs

**Cause**: Unlimited API access

**Solution**: Implement per-user rate limiting

### 7. Insufficient Error Handling

**Problem**: Cascading failures

**Cause**: Not handling partial failures gracefully

**Solution**: Implement circuit breakers, fallbacks, timeouts

### 8. No Backup Strategy

**Problem**: Data loss risk

**Cause**: No automated backups

**Solution**: Implement daily backups with verification

---

## Additional Resources

- [Performance Tuning Guide](../deployment/performance-tuning.md)
- [API Documentation](../api/SEMANTIC-SEARCH-API.md)
- [Troubleshooting Guide](./troubleshooting.md)
- [RFC-088 Implementation Summary](../../docs-internal/rfc/RFC-088-IMPLEMENTATION-SUMMARY.md)

---

*Last Updated: November 15, 2025*
*RFC-088 Implementation*
*Version 2.0*
