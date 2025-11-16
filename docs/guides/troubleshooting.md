# Troubleshooting Guide
## RFC-088 Semantic Search and Document Indexer

**Version**: 2.0
**Audience**: All Users (Developers, Operators, Administrators)
**Last Updated**: November 15, 2025

---

## Overview

This guide provides troubleshooting procedures for common issues with the RFC-088 Event-Driven Document Indexer with Semantic Search.

**Quick Links**:
- [Common Error Messages](#common-error-messages)
- [Performance Issues](#performance-issues)
- [Database Problems](#database-problems)
- [Search Issues](#search-issues)
- [Indexer Problems](#indexer-problems)
- [Connectivity Issues](#connectivity-issues)

---

## Table of Contents

1. [Common Error Messages](#common-error-messages)
2. [Performance Issues](#performance-issues)
3. [Database Problems](#database-problems)
4. [Search Issues](#search-issues)
5. [Indexer Problems](#indexer-problems)
6. [Connectivity Issues](#connectivity-issues)
7. [Debugging Techniques](#debugging-techniques)
8. [Log Analysis](#log-analysis)

---

## Common Error Messages

### "semantic search not configured"

**Error**:
```json
{
  "error": "semantic search not configured",
  "code": "SERVICE_UNAVAILABLE"
}
```

**Causes**:
1. OpenAI API key not configured
2. Database connection failed
3. pgvector extension not installed
4. Indexer not running (no embeddings in database)

**Solutions**:

1. **Check OpenAI API key**:
```bash
# Verify API key is set
echo $OPENAI_API_KEY

# Test API key
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
```

2. **Check database connection**:
```bash
# Test database connectivity
psql -h localhost -U hermes -d hermes -c "SELECT 1;"

# Check pgvector extension
psql -h localhost -U hermes -d hermes -c "SELECT * FROM pg_extension WHERE extname = 'vector';"
```

3. **Check for embeddings**:
```sql
SELECT COUNT(*) FROM document_embeddings;
-- If 0, indexer hasn't processed any documents
```

4. **Check indexer logs**:
```bash
# Kubernetes
kubectl logs -l app=hermes-indexer --tail=100

# Docker
docker logs hermes-indexer --tail=100
```

---

### "rate limit exceeded"

**Error**:
```json
{
  "error": "rate limit exceeded",
  "code": "RATE_LIMIT_EXCEEDED",
  "retryAfter": 60
}
```

**Causes**:
1. User exceeded API rate limit
2. IP exceeded rate limit
3. Aggressive client behavior

**Solutions**:

1. **Check rate limit headers**:
```bash
curl -I https://hermes.example.com/api/v2/search/semantic \
  -H "Authorization: Bearer TOKEN"

# Headers:
# X-RateLimit-Limit: 100
# X-RateLimit-Remaining: 0
# X-RateLimit-Reset: 1699564800
```

2. **Implement exponential backoff**:
```javascript
async function searchWithRetry(query, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      return await search(query);
    } catch (err) {
      if (err.code === 'RATE_LIMIT_EXCEEDED') {
        const delay = Math.pow(2, i) * 1000; // Exponential backoff
        await new Promise(resolve => setTimeout(resolve, delay));
      } else {
        throw err;
      }
    }
  }
  throw new Error('Max retries exceeded');
}
```

3. **Request rate limit increase** (contact administrator)

---

### "query cannot be empty"

**Error**:
```json
{
  "error": "query cannot be empty",
  "code": "INVALID_REQUEST"
}
```

**Solutions**:

1. **Check request body**:
```javascript
// Bad
await search({ query: "" });

// Good
await search({ query: "kubernetes deployment" });
```

2. **Validate input before sending**:
```javascript
function validateQuery(query) {
  if (!query || query.trim() === "") {
    throw new Error("Query cannot be empty");
  }
  return query.trim();
}
```

---

### "limit must be between 1 and 100"

**Error**:
```json
{
  "error": "limit must be between 1 and 100",
  "code": "INVALID_REQUEST"
}
```

**Solutions**:

```javascript
// Bad
await search({ query: "test", limit: 0 });
await search({ query: "test", limit: 200 });

// Good
await search({ query: "test", limit: 10 });
await search({ query: "test", limit: 100 });
```

---

### "document not found or has no embeddings"

**Error**:
```json
{
  "error": "document not found or has no embeddings",
  "code": "NOT_FOUND"
}
```

**Causes**:
1. Document doesn't exist
2. Document hasn't been indexed yet
3. Document filtered out by rulesets

**Solutions**:

1. **Check document exists**:
```sql
SELECT id, title FROM documents WHERE id = 'doc123';
```

2. **Check if document has embeddings**:
```sql
SELECT COUNT(*) FROM document_embeddings WHERE document_id = 'doc123';
```

3. **Check indexer logs for that document**:
```bash
kubectl logs -l app=hermes-indexer | grep "doc123"
```

4. **Trigger reindex**:
```bash
# Send document to Kafka topic
kafka-console-producer --broker-list localhost:9092 \
  --topic document-revisions \
  --property "parse.key=true" \
  --property "key.separator=:"
> doc123:{"document_id":"doc123","operation":"create"}
```

---

## Performance Issues

### Slow Semantic Search (>500ms)

**Symptoms**:
- Semantic search endpoint taking >500ms
- User complaints about slow search

**Diagnosis**:

1. **Check for database indexes**:
```sql
-- Check if vector index exists
SELECT indexname, indexdef
FROM pg_indexes
WHERE tablename = 'document_embeddings'
  AND indexdef LIKE '%ivfflat%';

-- If no index, performance will be poor (full table scan)
```

2. **Analyze query plan**:
```sql
EXPLAIN ANALYZE
SELECT document_id,
       embedding_vector <=> '[0.1, 0.2, ...]'::vector AS similarity
FROM document_embeddings
WHERE model = 'text-embedding-3-small'
ORDER BY embedding_vector <=> '[0.1, 0.2, ...]'::vector
LIMIT 10;

-- Look for "Seq Scan" (BAD) vs "Index Scan" (GOOD)
```

**Solutions**:

1. **Create vector index** (see [Performance Tuning](../deployment/performance-tuning.md)):
```sql
CREATE INDEX idx_embeddings_vector_ivfflat
ON document_embeddings
USING ivfflat (embedding_vector vector_cosine_ops)
WITH (lists = 100);
```

2. **Create lookup index**:
```sql
CREATE INDEX idx_embeddings_lookup
ON document_embeddings (document_id, model);
```

3. **Run ANALYZE**:
```sql
ANALYZE document_embeddings;
```

4. **Check connection pool**:
```bash
# Check if connection pool is exhausted
curl http://localhost:9090/metrics | grep db_connections_wait_count
```

---

### High API Latency

**Symptoms**:
- P95 latency >1 second
- Slow response times across all endpoints

**Diagnosis**:

1. **Check database query latency**:
```sql
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
WHERE query LIKE '%document_embeddings%'
ORDER BY mean_exec_time DESC
LIMIT 10;
```

2. **Check connection pool utilization**:
```promql
# Prometheus query
(db_connections_in_use / db_connections_open) * 100
```

3. **Check CPU/memory**:
```bash
# Kubernetes
kubectl top pods -l app=hermes-api

# System
top -p $(pgrep hermes-api)
```

**Solutions**:

1. **Increase connection pool size**:
```hcl
database {
  max_idle_conns = 25  # Increase from 10
  max_open_conns = 50  # Increase from 25
}
```

2. **Add database indexes** (see above)

3. **Scale horizontally**:
```bash
# Kubernetes
kubectl scale deployment hermes-api --replicas=4
```

4. **Check for slow queries**:
```sql
-- Find and optimize slow queries
SELECT * FROM pg_stat_statements
WHERE mean_exec_time > 100  -- >100ms
ORDER BY mean_exec_time DESC;
```

---

## Database Problems

### Connection Pool Exhaustion

**Error**:
```
pq: sorry, too many clients already
```

**Symptoms**:
- High wait count in connection pool stats
- Errors connecting to database
- Slow response times

**Diagnosis**:

```sql
-- Check current connections
SELECT count(*) FROM pg_stat_activity;

-- Check max connections
SHOW max_connections;

-- Check connections by application
SELECT application_name, count(*)
FROM pg_stat_activity
GROUP BY application_name;
```

**Solutions**:

1. **Increase PostgreSQL max_connections**:
```sql
ALTER SYSTEM SET max_connections = 200;
-- Restart PostgreSQL
```

2. **Increase application connection pool**:
```hcl
database {
  max_open_conns = 50  # Increase limit
}
```

3. **Check for connection leaks**:
```bash
# Monitor connection count over time
watch -n 1 'psql -c "SELECT count(*) FROM pg_stat_activity;"'
```

4. **Reduce connection lifetime**:
```hcl
database {
  conn_max_lifetime = "5m"   # Recycle connections
  conn_max_idle_time = "2m"  # Close idle faster
}
```

---

### Slow Database Queries

**Symptoms**:
- All queries slow (>100ms)
- Database CPU high
- Query timeout errors

**Diagnosis**:

```sql
-- Check slow queries
SELECT
    query,
    calls,
    mean_exec_time,
    max_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- Check missing indexes
SELECT
    schemaname,
    tablename,
    attname,
    n_distinct,
    correlation
FROM pg_stats
WHERE tablename = 'document_embeddings';
```

**Solutions**:

1. **Create missing indexes**:
```sql
-- Vector index
CREATE INDEX idx_embeddings_vector_ivfflat
ON document_embeddings
USING ivfflat (embedding_vector vector_cosine_ops)
WITH (lists = 100);

-- Lookup index
CREATE INDEX idx_embeddings_lookup
ON document_embeddings (document_id, model);
```

2. **Update statistics**:
```sql
ANALYZE document_embeddings;
```

3. **Vacuum database**:
```sql
-- For maintenance window
VACUUM FULL document_embeddings;

-- Or continuously
VACUUM document_embeddings;
```

4. **Tune PostgreSQL settings** (see [Performance Tuning](../deployment/performance-tuning.md))

---

### pgvector Extension Not Found

**Error**:
```
ERROR: type "vector" does not exist
```

**Solutions**:

1. **Install pgvector extension**:
```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

2. **Verify installation**:
```sql
SELECT * FROM pg_extension WHERE extname = 'vector';
```

3. **Check PostgreSQL version** (requires 11+):
```sql
SELECT version();
```

4. **If extension not available**, install pgvector:
```bash
# Ubuntu/Debian
apt-get install postgresql-14-pgvector

# macOS
brew install pgvector

# Or build from source
git clone https://github.com/pgvector/pgvector.git
cd pgvector
make
sudo make install
```

---

## Search Issues

### No Search Results

**Symptoms**:
- Semantic search returns empty results
- Similar documents endpoint returns no results

**Diagnosis**:

1. **Check if embeddings exist**:
```sql
SELECT COUNT(*) FROM document_embeddings;
-- If 0, no documents have been indexed
```

2. **Check if documents match query**:
```sql
-- Test with low similarity threshold
SELECT document_id, embedding_vector <=> '[...]'::vector AS similarity
FROM document_embeddings
ORDER BY similarity
LIMIT 10;
```

3. **Check similarity threshold**:
```javascript
// Too high threshold may filter all results
await search({
  query: "test",
  minSimilarity: 0.99  // Try lower threshold like 0.5
});
```

**Solutions**:

1. **Lower similarity threshold**:
```javascript
await search({
  query: "kubernetes",
  minSimilarity: 0.5  // Lower from 0.8
});
```

2. **Check if indexer is running**:
```bash
kubectl get pods -l app=hermes-indexer
```

3. **Trigger reindex** of documents

4. **Verify documents are being embedded**:
```bash
kubectl logs -l app=hermes-indexer | grep "embedding generated"
```

---

### Search Results Not Relevant

**Symptoms**:
- Search returns results but they're not relevant
- Low similarity scores (<0.5)

**Diagnosis**:

1. **Check query quality**:
```javascript
// Bad: Too generic
await search({ query: "doc" });

// Good: Specific
await search({ query: "kubernetes deployment best practices" });
```

2. **Check if using appropriate search type**:
```javascript
// Use hybrid search for better results
await hybridSearch({
  query: "kubernetes",
  weights: {
    keywordWeight: 0.4,
    semanticWeight: 0.4,
    boostBoth: 0.2
  }
});
```

**Solutions**:

1. **Use hybrid search** for better relevance
2. **Improve query specificity**
3. **Adjust search weights**:
```javascript
// For exact terms (IDs, codes)
weights: { keywordWeight: 0.7, semanticWeight: 0.2, boostBoth: 0.1 }

// For concepts
weights: { keywordWeight: 0.2, semanticWeight: 0.7, boostBoth: 0.1 }
```

4. **Check document quality** (are they well-written?)

---

## Indexer Problems

### High Kafka Lag

**Symptoms**:
- Kafka consumer lag >1000 messages
- Documents not being indexed quickly
- Lag increasing over time

**Diagnosis**:

```bash
# Check consumer lag
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group hermes-indexer \
  --describe

# Output shows lag per partition
# GROUP           TOPIC                 PARTITION  CURRENT-OFFSET  LOG-END-OFFSET  LAG
# hermes-indexer  document-revisions    0          1000            5000            4000
```

**Solutions**:

1. **Increase worker count**:
```bash
kubectl scale deployment hermes-indexer --replicas=8
```

2. **Increase batch size**:
```hcl
indexer {
  batch_size = 20  # Increase from 10
}
```

3. **Check for slow processing**:
```bash
kubectl logs -l app=hermes-indexer | grep "processing time"
```

4. **Check OpenAI API latency**:
```bash
kubectl logs -l app=hermes-indexer | grep "openai"
```

5. **Increase Kafka partitions** (for future scalability):
```bash
kafka-topics --alter \
  --bootstrap-server localhost:9092 \
  --topic document-revisions \
  --partitions 16
```

---

### OpenAI API Rate Limiting

**Error**:
```
openai: rate limit exceeded
```

**Symptoms**:
- Indexer processing slowed down
- Error logs showing rate limit errors

**Diagnosis**:

```bash
# Check rate limit errors
kubectl logs -l app=hermes-indexer | grep "rate limit"

# Check metrics
curl http://localhost:9090/metrics | grep openai_api_calls_total
```

**Solutions**:

1. **Implement exponential backoff** (already implemented in RFC-088):
```hcl
openai {
  max_retries = 5
  retry_delay = "2s"
}
```

2. **Reduce worker count temporarily**:
```bash
kubectl scale deployment hermes-indexer --replicas=4
```

3. **Increase batch size** (process fewer, larger batches):
```hcl
indexer {
  batch_size = 20
}
```

4. **Contact OpenAI** for higher rate limits

5. **Use OpenAI batch API** (future enhancement for 50% cost savings)

---

### Documents Not Being Indexed

**Symptoms**:
- Documents created but no embeddings generated
- Similar documents endpoint returns 404

**Diagnosis**:

1. **Check if document in Kafka**:
```bash
kafka-console-consumer --bootstrap-server localhost:9092 \
  --topic document-revisions \
  --from-beginning | grep "doc123"
```

2. **Check indexer logs**:
```bash
kubectl logs -l app=hermes-indexer | grep "doc123"
```

3. **Check if document matches rulesets**:
```bash
# Test dry-run if available
hermes-indexer --config indexer.hcl --dry-run --file path/to/doc.md
```

4. **Check for processing errors**:
```bash
kubectl logs -l app=hermes-indexer | grep "error"
```

**Solutions**:

1. **Check ruleset configuration**:
```hcl
indexer {
  rulesets = [
    {
      name    = "docs"
      enabled = true  # Make sure enabled!
      rules {
        include = ["*.md"]  # Make sure includes your files
      }
    }
  ]
}
```

2. **Restart indexer**:
```bash
kubectl rollout restart deployment hermes-indexer
```

3. **Manually trigger reprocess**:
```bash
# Resend message to Kafka
kafka-console-producer --broker-list localhost:9092 \
  --topic document-revisions
> {"document_id":"doc123","operation":"update"}
```

---

## Connectivity Issues

### Cannot Connect to Database

**Error**:
```
dial tcp: connect: connection refused
```

**Diagnosis**:

```bash
# Test database connectivity
psql -h db.example.com -p 5432 -U hermes -d hermes

# Check network connectivity
ping db.example.com
telnet db.example.com 5432

# Check DNS resolution
nslookup db.example.com
```

**Solutions**:

1. **Verify database is running**:
```bash
# On database host
systemctl status postgresql

# Or check container
docker ps | grep postgres
```

2. **Check firewall rules**:
```bash
# Allow port 5432
sudo ufw allow 5432/tcp
```

3. **Check PostgreSQL configuration**:
```ini
# postgresql.conf
listen_addresses = '*'  # Or specific IP

# pg_hba.conf
host    all    all    0.0.0.0/0    md5  # Allow remote connections
```

4. **Verify credentials**:
```bash
# Test connection
psql -h localhost -U hermes -d hermes -c "SELECT 1;"
```

---

### Cannot Connect to Kafka

**Error**:
```
kafka: unable to connect to broker
```

**Diagnosis**:

```bash
# Test Kafka connectivity
kafka-broker-api-versions --bootstrap-server localhost:9092

# Check if topic exists
kafka-topics --list --bootstrap-server localhost:9092

# Check consumer group
kafka-consumer-groups --bootstrap-server localhost:9092 --list
```

**Solutions**:

1. **Verify Kafka is running**:
```bash
# Check Kafka process
ps aux | grep kafka

# Or container
docker ps | grep kafka
```

2. **Check network connectivity**:
```bash
telnet kafka.example.com 9092
```

3. **Verify broker configuration**:
```properties
# server.properties
listeners=PLAINTEXT://0.0.0.0:9092
advertised.listeners=PLAINTEXT://kafka.example.com:9092
```

4. **Check DNS resolution**:
```bash
nslookup kafka.example.com
```

---

### Cannot Connect to OpenAI API

**Error**:
```
dial tcp: i/o timeout
```

**Diagnosis**:

```bash
# Test OpenAI API connectivity
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"

# Check if API key is valid
echo $OPENAI_API_KEY
```

**Solutions**:

1. **Verify API key**:
```bash
# Get new API key from https://platform.openai.com/api-keys
export OPENAI_API_KEY="sk-..."
```

2. **Check network connectivity**:
```bash
# Test HTTPS connectivity
curl https://api.openai.com
```

3. **Check firewall/proxy**:
```bash
# May need to configure proxy
export HTTPS_PROXY=http://proxy.example.com:8080
```

4. **Check rate limits**:
```bash
# Look for rate limit headers
curl -I https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
```

---

## Debugging Techniques

### Enable Debug Logging

```hcl
# Configuration
log_level = "debug"
log_format = "json"  # Or "text" for human-readable
```

```bash
# Restart service
kubectl rollout restart deployment hermes-api
kubectl rollout restart deployment hermes-indexer
```

### Use Health Check Endpoints

```bash
# API health
curl http://localhost:8080/health

# Readiness
curl http://localhost:8080/ready

# Liveness
curl http://localhost:8080/live
```

### Check Prometheus Metrics

```bash
# Get all metrics
curl http://localhost:9090/metrics

# Filter specific metrics
curl http://localhost:9090/metrics | grep http_requests

# Check database connections
curl http://localhost:9090/metrics | grep db_connections
```

### Database Query Analysis

```sql
-- Check active queries
SELECT
    pid,
    state,
    query,
    age(clock_timestamp(), query_start) AS query_age
FROM pg_stat_activity
WHERE state != 'idle'
ORDER BY query_age DESC;

-- Check query performance
SELECT
    query,
    calls,
    total_exec_time,
    mean_exec_time,
    max_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 20;

-- Check table sizes
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

### Profile Application

```bash
# CPU profiling
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Memory profiling
curl http://localhost:6060/debug/pprof/heap > mem.prof
go tool pprof mem.prof

# Goroutine profiling
curl http://localhost:6060/debug/pprof/goroutine > goroutine.prof
go tool pprof goroutine.prof
```

---

## Log Analysis

### API Server Logs

```bash
# Kubernetes
kubectl logs -l app=hermes-api --tail=100 -f

# Docker
docker logs hermes-api --tail=100 -f

# Look for patterns
kubectl logs -l app=hermes-api | grep "error"
kubectl logs -l app=hermes-api | grep "slow query"
```

### Indexer Logs

```bash
# Kubernetes
kubectl logs -l app=hermes-indexer --tail=100 -f

# Docker
docker logs hermes-indexer --tail=100 -f

# Common patterns
kubectl logs -l app=hermes-indexer | grep "processed"
kubectl logs -l app=hermes-indexer | grep "error"
kubectl logs -l app=hermes-indexer | grep "rate limit"
```

### Database Logs

```bash
# PostgreSQL logs location
tail -f /var/log/postgresql/postgresql-14-main.log

# Filter slow queries
grep "duration:" /var/log/postgresql/postgresql-14-main.log | \
  awk '$3 > 1000' # Queries >1 second

# Filter errors
grep "ERROR" /var/log/postgresql/postgresql-14-main.log
```

---

## Getting Help

If you can't resolve the issue:

1. **Collect diagnostic information**:
   - Error messages and stack traces
   - Relevant log excerpts
   - Metrics/graphs showing the problem
   - Steps to reproduce

2. **Check documentation**:
   - [Performance Tuning Guide](../deployment/performance-tuning.md)
   - [Best Practices](./best-practices.md)
   - [API Documentation](../api/SEMANTIC-SEARCH-API.md)

3. **Contact support** with:
   - Clear problem description
   - Diagnostic information collected
   - What you've already tried

---

## Additional Resources

- [Performance Tuning Guide](../deployment/performance-tuning.md)
- [Monitoring Setup](../deployment/monitoring-setup.md)
- [Best Practices](./best-practices.md)
- [Search Configuration](../configuration/search-configuration.md)

---

*Last Updated: November 15, 2025*
*RFC-088 Implementation*
*Version 2.0*
