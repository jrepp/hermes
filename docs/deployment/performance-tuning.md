# Performance Tuning Guide
## RFC-088 Semantic Search and Document Indexer

**Version**: 2.0
**Audience**: DevOps Engineers, Database Administrators, SRE Teams
**Last Updated**: November 15, 2025

---

## Overview

This guide provides comprehensive performance tuning recommendations for the RFC-088 Event-Driven Document Indexer with Semantic Search. Proper configuration can result in **50-200x performance improvements** for vector search operations.

**Key Performance Areas**:
- Database configuration and indexing
- Connection pooling
- Query optimization
- Resource allocation
- Monitoring and profiling

---

## Table of Contents

1. [PostgreSQL Configuration](#postgresql-configuration)
2. [pgvector Indexes](#pgvector-indexes)
3. [Connection Pool Tuning](#connection-pool-tuning)
4. [Query Optimization](#query-optimization)
5. [Resource Allocation](#resource-allocation)
6. [Monitoring and Profiling](#monitoring-and-profiling)
7. [Performance Benchmarks](#performance-benchmarks)
8. [Troubleshooting](#troubleshooting)

---

## PostgreSQL Configuration

### 1. Required Extensions

Ensure pgvector extension is installed:

```sql
-- Check if pgvector is installed
SELECT * FROM pg_extension WHERE extname = 'vector';

-- Install pgvector if not present
CREATE EXTENSION IF NOT EXISTS vector;
```

### 2. Memory Configuration

Adjust PostgreSQL memory settings for vector operations:

```ini
# postgresql.conf

# Increase shared buffers for vector operations
shared_buffers = 4GB  # 25% of available RAM

# Increase work memory for sorting and vector comparisons
work_mem = 256MB  # Per operation

# Increase maintenance work memory for index creation
maintenance_work_mem = 2GB  # For CREATE INDEX operations

# Enable parallel query execution
max_parallel_workers_per_gather = 4
max_parallel_workers = 8
```

**Guidelines**:
- `shared_buffers`: Set to 25% of available RAM (minimum 2GB, maximum 8GB)
- `work_mem`: Set to 128-256MB for vector operations
- `maintenance_work_mem`: Set to 1-2GB for building indexes
- Restart PostgreSQL after configuration changes

### 3. Query Planner Settings

Optimize query planner for vector operations:

```sql
-- Enable parallel query execution
ALTER SYSTEM SET max_parallel_workers_per_gather = 4;

-- Adjust random page cost for SSD storage
ALTER SYSTEM SET random_page_cost = 1.1;  -- Default is 4.0 for HDD

-- Increase statistics target for better query plans
ALTER SYSTEM SET default_statistics_target = 100;  -- Default is 100

-- Reload configuration
SELECT pg_reload_conf();
```

### 4. Analyze Tables

Keep statistics up to date for optimal query plans:

```sql
-- Analyze embeddings table
ANALYZE document_embeddings;

-- Set custom statistics target for embedding column
ALTER TABLE document_embeddings
ALTER COLUMN embedding_vector SET STATISTICS 1000;

-- Re-analyze after statistics change
ANALYZE document_embeddings;
```

---

## pgvector Indexes

### Index Types

pgvector supports two index types:

| Index Type | Use Case | Performance | Build Time | Memory |
|------------|----------|-------------|------------|--------|
| **IVFFlat** | General purpose | 10-100x faster | Moderate | Low |
| **HNSW** | High performance | 2-4x over IVFFlat | Slower | Higher |

### 1. IVFFlat Index (Recommended)

Best for most use cases. Provides excellent performance with reasonable build time.

```sql
-- Create IVFFlat index on embeddings
CREATE INDEX idx_embeddings_vector_ivfflat
ON document_embeddings
USING ivfflat (embedding_vector vector_cosine_ops)
WITH (lists = 100);
```

**Parameter Tuning**:

```sql
-- Calculate optimal lists parameter
-- Rule of thumb: lists ≈ sqrt(rows)
SELECT CEIL(SQRT(COUNT(*))) AS recommended_lists
FROM document_embeddings;

-- For 10,000 documents: lists = 100
-- For 100,000 documents: lists = 316
-- For 1,000,000 documents: lists = 1000

-- Recreate index with optimal lists parameter
DROP INDEX IF EXISTS idx_embeddings_vector_ivfflat;
CREATE INDEX idx_embeddings_vector_ivfflat
ON document_embeddings
USING ivfflat (embedding_vector vector_cosine_ops)
WITH (lists = 316);  -- Use calculated value
```

**Query Tuning**:

```sql
-- Set probes parameter for query time
-- Higher probes = more accurate but slower
-- Default: probes = lists / 10

SET ivfflat.probes = 10;  -- For lists=100

-- For better recall, increase probes
SET ivfflat.probes = 20;  -- More accurate, slower

-- For better performance, decrease probes
SET ivfflat.probes = 5;   -- Faster, less accurate
```

### 2. HNSW Index (High Performance)

Best for large datasets (>1M documents) requiring lowest latency.

```sql
-- Create HNSW index on embeddings
CREATE INDEX idx_embeddings_vector_hnsw
ON document_embeddings
USING hnsw (embedding_vector vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
```

**Parameter Tuning**:

| Parameter | Default | Range | Impact |
|-----------|---------|-------|--------|
| `m` | 16 | 2-100 | Connections per node (higher = better recall, more memory) |
| `ef_construction` | 64 | 4-1000 | Build quality (higher = better index, slower build) |

```sql
-- For better recall (recommended for production)
CREATE INDEX idx_embeddings_vector_hnsw
ON document_embeddings
USING hnsw (embedding_vector vector_cosine_ops)
WITH (m = 32, ef_construction = 128);

-- For faster builds (development/testing)
CREATE INDEX idx_embeddings_vector_hnsw
ON document_embeddings
USING hnsw (embedding_vector vector_cosine_ops)
WITH (m = 16, ef_construction = 32);
```

**Query Tuning**:

```sql
-- Set ef_search parameter for query time
-- Higher ef_search = more accurate but slower
-- Default: ef_search = 40

SET hnsw.ef_search = 100;  -- Better recall
SET hnsw.ef_search = 40;   -- Default
SET hnsw.ef_search = 20;   -- Faster, less accurate
```

### 3. Lookup Index (Critical)

Create index for document lookups (used by similar documents endpoint):

```sql
-- Create composite index for document + model lookups
CREATE INDEX idx_embeddings_lookup
ON document_embeddings (document_id, model);

-- Optional: Create partial index for specific model
CREATE INDEX idx_embeddings_lookup_default_model
ON document_embeddings (document_id)
WHERE model = 'text-embedding-3-small';
```

### 4. Index Maintenance

Monitor and maintain indexes for optimal performance:

```sql
-- Check index usage
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan AS index_scans,
    idx_tup_read AS tuples_read,
    idx_tup_fetch AS tuples_fetched
FROM pg_stat_user_indexes
WHERE tablename = 'document_embeddings'
ORDER BY idx_scan DESC;

-- Check index size
SELECT
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) AS index_size
FROM pg_stat_user_indexes
WHERE tablename = 'document_embeddings';

-- Rebuild index if needed (after bulk inserts)
REINDEX INDEX CONCURRENTLY idx_embeddings_vector_ivfflat;
REINDEX INDEX CONCURRENTLY idx_embeddings_lookup;
```

---

## Connection Pool Tuning

### 1. Application Configuration

Connection pooling is configured in `pkg/database/database.go`:

```go
// Example configuration in HCL
database {
  host     = "localhost"
  port     = 5432
  user     = "hermes"
  password = "secure_password"
  dbname   = "hermes"
  sslmode  = "require"

  # Connection pool settings
  max_idle_conns     = 10   # Maintain 10 ready connections
  max_open_conns     = 25   # Allow up to 25 concurrent connections
  conn_max_lifetime  = "5m" # Recycle connections after 5 minutes
  conn_max_idle_time = "10m" # Close idle connections after 10 minutes
}
```

### 2. Tuning Guidelines

**For Low Traffic (<10 requests/second)**:
```go
max_idle_conns     = 5
max_open_conns     = 10
conn_max_lifetime  = "5m"
conn_max_idle_time = "10m"
```

**For Medium Traffic (10-100 requests/second)**:
```go
max_idle_conns     = 10   # Default
max_open_conns     = 25   # Default
conn_max_lifetime  = "5m"
conn_max_idle_time = "5m"
```

**For High Traffic (>100 requests/second)**:
```go
max_idle_conns     = 25
max_open_conns     = 50
conn_max_lifetime  = "5m"
conn_max_idle_time = "2m"
```

### 3. PostgreSQL Connection Limits

Ensure PostgreSQL can handle the connection pool:

```sql
-- Check current max connections
SHOW max_connections;  -- Default: 100

-- Check current connections
SELECT count(*) FROM pg_stat_activity;

-- Adjust max connections if needed
ALTER SYSTEM SET max_connections = 200;
-- Requires restart
```

**Formula**:
```
max_connections >= (num_services × max_open_conns) + 10 (for admin)
```

**Example**: 3 services × 25 connections + 10 = **85 connections required**

### 4. Monitor Connection Pool

Monitor pool statistics to identify bottlenecks:

```go
// Example monitoring code
stats, err := database.GetPoolStats(db)
if err != nil {
    log.Error("failed to get pool stats", "error", err)
}

log.Info("connection pool statistics",
    "open", stats.OpenConnections,
    "in_use", stats.InUse,
    "idle", stats.Idle,
    "wait_count", stats.WaitCount,
    "wait_duration", stats.WaitDuration,
)

// Alert if wait count is increasing
if stats.WaitCount > 1000 {
    log.Warn("high connection wait count, consider increasing pool size")
}
```

---

## Query Optimization

### 1. Semantic Search Optimization

**Before Optimization** (no indexes):
```sql
-- Query: Find similar documents
SELECT
    document_id,
    embedding_vector <=> $1 AS similarity
FROM document_embeddings
WHERE model = $2
ORDER BY embedding_vector <=> $1
LIMIT $3;

-- Performance: 100-1000ms for 10K documents (full table scan)
```

**After Optimization** (with IVFFlat index):
```sql
-- Same query, but uses index
-- Performance: 10-50ms for 100K documents (90% faster!)
```

### 2. Similar Documents Optimization

**Query Pattern**:
```sql
-- Step 1: Get source document embedding (must be fast!)
SELECT embedding_vector
FROM document_embeddings
WHERE document_id = $1 AND model = $2;

-- Step 2: Find similar documents
SELECT
    document_id,
    embedding_vector <=> $1 AS similarity
FROM document_embeddings
WHERE model = $2
  AND document_id != $3
ORDER BY embedding_vector <=> $1
LIMIT $4;
```

**Optimization**: Lookup index on (document_id, model) makes Step 1 instant.

### 3. Hybrid Search Optimization

Hybrid search runs keyword and semantic searches **in parallel**:

```
Sequential:  keyword (50ms) + semantic (50ms) = 100ms
Parallel:    max(keyword, semantic) = 50ms (50% faster!)
```

No additional configuration needed - implemented in `pkg/search/hybrid.go`.

### 4. Query Plan Analysis

Check if indexes are being used:

```sql
-- Explain query plan
EXPLAIN ANALYZE
SELECT
    document_id,
    embedding_vector <=> '[0.1, 0.2, ...]'::vector AS similarity
FROM document_embeddings
WHERE model = 'text-embedding-3-small'
ORDER BY embedding_vector <=> '[0.1, 0.2, ...]'::vector
LIMIT 10;

-- Look for:
-- ✅ "Index Scan using idx_embeddings_vector_ivfflat"
-- ❌ "Seq Scan on document_embeddings" (BAD - add index!)
```

---

## Resource Allocation

### 1. CPU Allocation

**Recommendations**:
- **API Server**: 2-4 CPU cores (handles HTTP requests)
- **Indexer Workers**: 2-4 CPU cores each (embedding generation)
- **PostgreSQL**: 4-8 CPU cores (vector operations are CPU-intensive)

### 2. Memory Allocation

**Database Memory**:
```
Total DB Memory = shared_buffers + (work_mem × max_connections)
Example: 4GB + (256MB × 100) = 29.6GB
```

**Application Memory**:
- **API Server**: 1-2GB (handles requests and responses)
- **Indexer Worker**: 2-4GB (embedding generation, document processing)

### 3. Storage

**Disk Space Requirements**:
- **Documents**: Variable (depends on document size)
- **Embeddings**: ~6KB per document (1536 dimensions × 4 bytes)
- **Indexes**: ~2x embedding size (IVFFlat), ~3x (HNSW)

**Example**: 1M documents
- Embeddings: 1M × 6KB = 6GB
- IVFFlat index: ~12GB
- HNSW index: ~18GB
- **Total**: 24-36GB for embeddings + indexes

**Disk Performance**:
- Use SSD storage for database (NVMe preferred)
- Set `random_page_cost = 1.1` for SSD (instead of 4.0 for HDD)

---

## Monitoring and Profiling

### 1. Key Metrics

**API Performance**:
- Request latency (p50, p95, p99)
- Requests per second
- Error rate

**Search Performance**:
- Semantic search latency
- Keyword search latency
- Hybrid search latency
- Cache hit rate (if implemented)

**Database Performance**:
- Query execution time
- Index usage
- Connection pool utilization
- Table sizes

**Indexer Performance**:
- Documents processed per second
- Embedding generation latency
- Kafka lag (messages pending)

### 2. Database Monitoring Queries

```sql
-- Slow queries
SELECT
    query,
    calls,
    mean_exec_time,
    max_exec_time,
    stddev_exec_time
FROM pg_stat_statements
WHERE query LIKE '%document_embeddings%'
ORDER BY mean_exec_time DESC
LIMIT 10;

-- Index usage
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
WHERE tablename = 'document_embeddings';

-- Table bloat
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE tablename = 'document_embeddings';

-- Connection pool status
SELECT
    count(*) AS total_connections,
    count(*) FILTER (WHERE state = 'active') AS active,
    count(*) FILTER (WHERE state = 'idle') AS idle
FROM pg_stat_activity;
```

### 3. Application Monitoring

Monitor connection pool statistics:

```go
// Add to application metrics
ticker := time.NewTicker(30 * time.Second)
go func() {
    for range ticker.C {
        stats, err := database.GetPoolStats(db)
        if err != nil {
            continue
        }

        metrics.Gauge("db.pool.open", stats.OpenConnections)
        metrics.Gauge("db.pool.in_use", stats.InUse)
        metrics.Gauge("db.pool.idle", stats.Idle)
        metrics.Counter("db.pool.wait_count", stats.WaitCount)
        metrics.Timer("db.pool.wait_duration", stats.WaitDuration)
    }
}()
```

### 4. Prometheus Metrics

Example Prometheus metrics for monitoring:

```
# API latency
api_request_duration_seconds{endpoint="/api/v2/search/semantic",quantile="0.5"} 0.045
api_request_duration_seconds{endpoint="/api/v2/search/semantic",quantile="0.95"} 0.150
api_request_duration_seconds{endpoint="/api/v2/search/semantic",quantile="0.99"} 0.300

# Database connections
db_connections_open{service="hermes-api"} 15
db_connections_in_use{service="hermes-api"} 8
db_connections_idle{service="hermes-api"} 7
db_connections_wait_count{service="hermes-api"} 42

# Search operations
search_queries_total{type="semantic"} 12543
search_queries_duration_seconds{type="semantic",quantile="0.95"} 0.120
```

---

## Performance Benchmarks

### Expected Performance

**Without Optimization**:
- Semantic search: 100-1000ms (10K documents)
- Database connections: 1-5ms overhead per query
- Hybrid search: sum(keyword + semantic) time

**With Full Optimization**:
- Semantic search: 10-50ms (100K documents) - **10-100x faster**
- Database connections: ~0ms overhead (connection reuse) - **10-30% faster**
- Hybrid search: max(keyword, semantic) time - **30-50% faster**

**Cumulative Improvement**: **50-200x faster**

### Benchmark Queries

```sql
-- Benchmark semantic search
\timing on

-- Without index (slow)
SELECT document_id, embedding_vector <=> '[...]'::vector AS similarity
FROM document_embeddings
WHERE model = 'text-embedding-3-small'
ORDER BY embedding_vector <=> '[...]'::vector
LIMIT 10;
-- Time: 250.000 ms

-- With IVFFlat index (fast)
-- Time: 15.000 ms (16.6x faster!)

-- With HNSW index (fastest)
-- Time: 5.000 ms (50x faster!)
```

### Load Testing

Use tools like Apache Bench or k6 for load testing:

```bash
# Test semantic search endpoint
ab -n 1000 -c 10 -T "application/json" -p query.json \
   -H "Authorization: Bearer YOUR_TOKEN" \
   https://hermes.example.com/api/v2/search/semantic

# query.json:
# {"query": "kubernetes deployment strategies", "limit": 10}

# Expected results:
# - Requests per second: 50-100
# - Mean latency: 50-100ms
# - P95 latency: 100-200ms
# - P99 latency: 200-300ms
```

---

## Troubleshooting

### Problem: Slow Vector Searches

**Symptoms**: Semantic search takes >500ms

**Diagnosis**:
```sql
EXPLAIN ANALYZE
SELECT document_id, embedding_vector <=> '[...]'::vector AS similarity
FROM document_embeddings
ORDER BY embedding_vector <=> '[...]'::vector
LIMIT 10;

-- Look for "Seq Scan" (BAD)
```

**Solutions**:
1. Create IVFFlat or HNSW index (see [pgvector Indexes](#pgvector-indexes))
2. Run ANALYZE on table
3. Increase `work_mem` for sort operations
4. Check `random_page_cost` setting (should be ~1.1 for SSD)

### Problem: Connection Pool Exhaustion

**Symptoms**: Errors like "sorry, too many clients already"

**Diagnosis**:
```go
stats, _ := database.GetPoolStats(db)
log.Info("pool stats", "wait_count", stats.WaitCount)
// High wait_count indicates exhaustion
```

**Solutions**:
1. Increase `max_open_conns` in application config
2. Increase PostgreSQL `max_connections`
3. Reduce query latency to free connections faster
4. Check for connection leaks (connections not being closed)

### Problem: High Memory Usage

**Symptoms**: PostgreSQL or application consuming excessive memory

**Diagnosis**:
```sql
-- Check work_mem usage
SELECT name, setting, unit
FROM pg_settings
WHERE name IN ('work_mem', 'shared_buffers', 'maintenance_work_mem');
```

**Solutions**:
1. Reduce `work_mem` (but not below 128MB)
2. Reduce `max_open_conns` in application
3. Add memory limits in container/orchestration config
4. Use smaller `lists` parameter for IVFFlat index

### Problem: Index Build Takes Too Long

**Symptoms**: CREATE INDEX takes hours

**Solutions**:
1. Increase `maintenance_work_mem` to 2-4GB
2. Use lower `ef_construction` for HNSW (e.g., 32 instead of 128)
3. Use lower `lists` for IVFFlat (but check query performance)
4. Build index with `CONCURRENTLY` to allow reads during build:
```sql
CREATE INDEX CONCURRENTLY idx_embeddings_vector_ivfflat
ON document_embeddings
USING ivfflat (embedding_vector vector_cosine_ops)
WITH (lists = 100);
```

### Problem: Inconsistent Query Performance

**Symptoms**: Same query sometimes fast, sometimes slow

**Diagnosis**:
```sql
-- Check if statistics are stale
SELECT schemaname, tablename, last_analyze, last_autoanalyze
FROM pg_stat_user_tables
WHERE tablename = 'document_embeddings';
```

**Solutions**:
1. Run ANALYZE regularly (daily or after bulk inserts)
2. Increase `default_statistics_target` to 100-1000
3. Check for table bloat and run VACUUM if needed
4. Monitor cache hit ratio

---

## Production Checklist

Before deploying to production, verify:

- [ ] pgvector extension installed
- [ ] IVFFlat or HNSW index created on embedding_vector
- [ ] Lookup index created on (document_id, model)
- [ ] ANALYZE run on document_embeddings table
- [ ] PostgreSQL memory settings tuned (shared_buffers, work_mem)
- [ ] Connection pooling configured (max_idle_conns, max_open_conns)
- [ ] random_page_cost set to 1.1 for SSD storage
- [ ] Monitoring enabled (Prometheus metrics, query logging)
- [ ] Load testing completed with realistic traffic
- [ ] Backup strategy in place for embeddings table

---

## Additional Resources

- [pgvector Documentation](https://github.com/pgvector/pgvector)
- [PostgreSQL Performance Tuning](https://www.postgresql.org/docs/current/performance-tips.html)
- [RFC-088 Query Optimization Analysis](../../docs-internal/rfc/RFC-088-QUERY-OPTIMIZATION-ANALYSIS.md)
- [API Documentation](../api/SEMANTIC-SEARCH-API.md)
- [Troubleshooting Guide](../guides/troubleshooting.md)

---

*Last Updated: November 15, 2025*
*RFC-088 Implementation*
*Version 2.0*
