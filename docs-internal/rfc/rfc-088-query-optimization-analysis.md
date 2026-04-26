# RFC-088 Query Optimization Analysis
## Semantic and Hybrid Search Performance Analysis

**Date**: November 15, 2025
**Status**: Analysis Complete
**Phase**: Week 8 Optimization

---

## Overview

This document analyzes the SQL queries used in RFC-088 semantic and hybrid search functionality, identifying optimization opportunities and providing recommendations for improved performance.

---

## Query Analysis

### 1. Semantic Search Core Query

**Location**: `pkg/search/semantic.go:122-138`

**Current Query**:
```sql
SELECT
    document_id,
    document_uuid,
    revision_id,
    chunk_index,
    chunk_text,
    content_hash,
    model,
    provider,
    (1 - (embedding_vector <=> $1::vector)) as similarity
FROM document_embeddings
WHERE embedding_vector IS NOT NULL
  AND model = $2
ORDER BY embedding_vector <=> $1::vector
LIMIT $3
```

**Analysis**:
- ✅ **Parameterized**: Uses proper parameterization ($1, $2, $3)
- ✅ **Selective filter**: Filters by `model` for index selectivity
- ✅ **pgvector operator**: Uses `<=>` cosine distance operator
- ✅ **NULL check**: Filters out NULL embeddings
- ⚠️ **Index dependency**: Performance heavily depends on proper indexing

**Performance Characteristics**:
- **Without index**: O(n) - Full table scan, ~100-1000ms for 10K docs
- **With IVFFlat index**: O(log n) - ~10-50ms for 100K docs
- **With HNSW index**: O(log n) - ~5-20ms for 1M docs

**Optimization Recommendations**:

1. **Primary Index** (CRITICAL):
```sql
-- IVFFlat index for general use (good balance)
CREATE INDEX idx_embeddings_vector_model_ivfflat
ON document_embeddings
USING ivfflat (embedding_vector vector_cosine_ops)
WITH (lists = 100)
WHERE model = 'text-embedding-3-small';

-- HNSW index for high-performance use (more memory)
CREATE INDEX idx_embeddings_vector_model_hnsw
ON document_embeddings
USING hnsw (embedding_vector vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
```

2. **Supporting Index** (for metadata queries):
```sql
CREATE INDEX idx_embeddings_model_document_id
ON document_embeddings (model, document_id)
INCLUDE (content_hash);
```

**Expected Impact**: 10-100x performance improvement with proper indexes

---

### 2. Filtered Semantic Search Query

**Location**: `pkg/search/semantic.go:215-249`

**Current Query Pattern**:
```sql
SELECT
    document_id,
    document_uuid,
    revision_id,
    chunk_index,
    chunk_text,
    content_hash,
    model,
    provider,
    (1 - (embedding_vector <=> $1::vector)) as similarity
FROM document_embeddings
WHERE embedding_vector IS NOT NULL
  AND model = $2
  [AND document_id = ANY($3)]  -- Optional filter
  [AND (1 - (embedding_vector <=> $1::vector)) >= $4]  -- Optional filter
ORDER BY embedding_vector <=> $1::vector
LIMIT $5
```

**Analysis**:
- ✅ **Dynamic query building**: Flexible filter composition
- ✅ **ANY operator**: Efficient for multiple document IDs
- ⚠️ **Redundant calculation**: Similarity computed twice (SELECT and WHERE)
- ⚠️ **Index selectivity**: Adding filters may prevent index usage

**Optimization Opportunity**:

The similarity calculation appears twice:
1. In SELECT: `(1 - (embedding_vector <=> $1::vector)) as similarity`
2. In WHERE (when MinSimilarity > 0): `(1 - (embedding_vector <=> $1::vector)) >= $4`

**Optimization**: Use a CTE or subquery to calculate once:

```sql
WITH scored_embeddings AS (
    SELECT
        document_id,
        document_uuid,
        revision_id,
        chunk_index,
        chunk_text,
        content_hash,
        model,
        provider,
        embedding_vector,
        (1 - (embedding_vector <=> $1::vector)) as similarity
    FROM document_embeddings
    WHERE embedding_vector IS NOT NULL
      AND model = $2
      [AND document_id = ANY($3)]
    ORDER BY embedding_vector <=> $1::vector
    LIMIT $5 * 2  -- Fetch more, filter below
)
SELECT *
FROM scored_embeddings
WHERE similarity >= $4
LIMIT $5;
```

**Trade-offs**:
- **Pro**: Calculates similarity once, cleaner code
- **Con**: May fetch more rows than needed
- **Decision**: Current implementation is fine unless profiling shows this as bottleneck

**Expected Impact**: Marginal (5-10%), only optimize if profiling shows need

---

### 3. Document Embedding Lookup

**Location**: `pkg/search/semantic.go:291-293`

**Current Query**:
```go
db.WithContext(ctx).
    Where("document_id = ? AND model = ?", documentID, s.model).
    First(&embedding).Error
```

**Generated SQL**:
```sql
SELECT * FROM document_embeddings
WHERE document_id = $1 AND model = $2
LIMIT 1
```

**Analysis**:
- ✅ **Simple lookup**: Efficient for single document
- ✅ **Composite key**: Uses both document_id and model
- ⚠️ **Index needed**: Should have index on (document_id, model)

**Optimization Recommendation**:

```sql
CREATE INDEX idx_embeddings_lookup
ON document_embeddings (document_id, model);
```

**Expected Impact**: 10-100x improvement for document lookups (1-2ms from 10-100ms)

---

### 4. Similar Documents Query

**Location**: `pkg/search/semantic.go:305-335`

**Current Pattern**:
1. Fetch document's embedding
2. Search using that embedding

**Query Flow**:
```sql
-- Step 1: Get document embedding
SELECT * FROM document_embeddings
WHERE document_id = $1 AND model = $2
LIMIT 1;

-- Step 2: Search similar
SELECT ...
FROM document_embeddings
WHERE embedding_vector IS NOT NULL
  AND model = $2
  AND document_id != $1  -- Exclude source document
ORDER BY embedding_vector <=> <fetched_embedding>
LIMIT $3;
```

**Analysis**:
- ✅ **Two-step approach**: Logical and correct
- ⚠️ **Two queries**: Could potentially be combined
- ⚠️ **Exclusion filter**: Adds extra WHERE condition

**Optimization Opportunity** (Advanced):

Could use a lateral join to combine into one query:

```sql
SELECT
    e2.document_id,
    e2.document_uuid,
    e2.revision_id,
    e2.chunk_index,
    e2.chunk_text,
    e2.content_hash,
    e2.model,
    e2.provider,
    (1 - (e2.embedding_vector <=> e1.embedding_vector)) as similarity
FROM document_embeddings e1
CROSS JOIN LATERAL (
    SELECT *
    FROM document_embeddings e2
    WHERE e2.embedding_vector IS NOT NULL
      AND e2.model = e1.model
      AND e2.document_id != e1.document_id
    ORDER BY e2.embedding_vector <=> e1.embedding_vector
    LIMIT $2
) e2
WHERE e1.document_id = $1
  AND e1.model = $3;
```

**Trade-offs**:
- **Pro**: Single round-trip to database
- **Pro**: Potentially better query planning
- **Con**: More complex query
- **Con**: Requires testing to verify performance improvement
- **Decision**: Keep current implementation for simplicity, optimize if needed

**Expected Impact**: Potentially 20-30% faster (saves one round-trip)

---

### 5. Hybrid Search Pattern

**Location**: `pkg/search/hybrid.go:84-86`

**Current Pattern**:
```go
keywordResults, keywordErr := h.performKeywordSearch(ctx, query, limit*2)
semanticResults, semanticErr := h.performSemanticSearch(ctx, query, limit*2)
```

**Analysis**:
- ❌ **Sequential execution**: Despite comment saying "parallel", runs sequentially
- ⚠️ **Double limit**: Fetches 2x limit from each (good for merging)
- ⚠️ **No true parallelism**: Misses opportunity for concurrency

**Optimization Opportunity**:

**Current (Sequential)**:
```
Total Time = Keyword Search Time + Semantic Search Time
            = 50ms + 100ms = 150ms
```

**Optimized (Parallel)**:
```
Total Time = max(Keyword Search Time, Semantic Search Time)
           = max(50ms, 100ms) = 100ms (33% faster!)
```

**Implementation**:

```go
// Perform both searches in parallel using goroutines
type searchResult[T any] struct {
    results T
    err     error
}

keywordChan := make(chan searchResult[[]KeywordSearchResult], 1)
semanticChan := make(chan searchResult[[]SemanticSearchResult], 1)

// Launch keyword search in goroutine
go func() {
    results, err := h.performKeywordSearch(ctx, query, limit*2)
    keywordChan <- searchResult[[]KeywordSearchResult]{results, err}
}()

// Launch semantic search in goroutine
go func() {
    results, err := h.performSemanticSearch(ctx, query, limit*2)
    semanticChan <- searchResult[[]SemanticSearchResult]{results, err}
}()

// Wait for both to complete
keywordResult := <-keywordChan
semanticResult := <-semanticChan

keywordResults, keywordErr := keywordResult.results, keywordResult.err
semanticResults, semanticErr := semanticResult.results, semanticResult.err
```

**Expected Impact**: 30-50% faster hybrid search (depending on relative search times)

---

## Index Recommendations Summary

### Critical Indexes (High Priority)

1. **Primary Vector Index**:
```sql
-- For general use (recommended starting point)
CREATE INDEX idx_embeddings_vector_ivfflat
ON document_embeddings
USING ivfflat (embedding_vector vector_cosine_ops)
WITH (lists = 100);

-- Analyze table for statistics
ANALYZE document_embeddings;
```

2. **Document Lookup Index**:
```sql
CREATE INDEX idx_embeddings_lookup
ON document_embeddings (document_id, model);
```

### Optional Indexes (Medium Priority)

3. **Model Filter Index**:
```sql
CREATE INDEX idx_embeddings_model
ON document_embeddings (model)
WHERE embedding_vector IS NOT NULL;
```

4. **Content Hash Index** (for deduplication):
```sql
CREATE INDEX idx_embeddings_content_hash
ON document_embeddings (content_hash, model);
```

### Advanced Indexes (Low Priority)

5. **HNSW Index** (for high-performance production):
```sql
-- Only after testing IVFFlat performance
-- Requires more memory but provides best query performance
CREATE INDEX idx_embeddings_vector_hnsw
ON document_embeddings
USING hnsw (embedding_vector vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
```

---

## Performance Testing Recommendations

### 1. Index Performance Testing

**Test with IVFFlat**:
```bash
# Warm up cache
SELECT COUNT(*) FROM document_embeddings WHERE embedding_vector IS NOT NULL;

# Benchmark query
EXPLAIN (ANALYZE, BUFFERS)
SELECT document_id, (1 - (embedding_vector <=> '[...]')) as similarity
FROM document_embeddings
WHERE model = 'text-embedding-3-small'
ORDER BY embedding_vector <=> '[...]'
LIMIT 10;
```

**Metrics to capture**:
- Planning time
- Execution time
- Rows scanned vs returned
- Buffer usage
- Index scan vs seq scan

### 2. Connection Pool Impact Testing

With the new connection pooling (from earlier optimization):

**Test query throughput**:
```bash
# Concurrent queries
for i in {1..100}; do
    curl -X POST https://api/v2/search/semantic \
      -d '{"query":"test","limit":10}' &
done
wait

# Check pool stats
SELECT * FROM pg_stat_database;
```

### 3. Hybrid Search Parallelism Testing

**Test sequential vs parallel**:
- Measure hybrid search latency before/after parallelization
- Expected: 30-50% improvement
- Monitor: CPU usage, database connections, memory

---

## Implementation Priority

### Phase 1: Critical (Week 8)
1. ✅ **Connection pooling** - COMPLETE
2. ⏳ **Create primary vector index** (IVFFlat)
3. ⏳ **Create document lookup index**
4. ⏳ **Parallelize hybrid search**

### Phase 2: Important (Week 9)
5. ⏳ **Test index performance**
6. ⏳ **Monitor query patterns**
7. ⏳ **Create supporting indexes as needed**
8. ⏳ **Consider HNSW index if IVFFlat insufficient**

### Phase 3: Optional (Week 10+)
9. ⏳ **Query result caching**
10. ⏳ **Prepared statement pooling**
11. ⏳ **Advanced query optimizations (CTEs, lateral joins)**

---

## Expected Cumulative Impact

| Optimization | Performance Gain | Complexity | Status |
|--------------|------------------|------------|--------|
| Connection pooling | 10-30% | Low | ✅ Complete |
| Vector index (IVFFlat) | 10-100x | Low | ⏳ Pending |
| Document lookup index | 10-100x | Low | ⏳ Pending |
| Hybrid search parallel | 30-50% | Medium | ⏳ Pending |
| HNSW index | 2-4x over IVFFlat | Medium | ⏳ Optional |
| Query optimization (CTE) | 5-10% | High | ⏳ Optional |

**Cumulative Expected Improvement**: 50-200x faster with all critical optimizations

---

## Monitoring and Validation

### Key Metrics to Track

1. **Query Latency**:
   - p50, p95, p99 latency
   - Target: <50ms p95 for 10K corpus

2. **Throughput**:
   - Queries per second
   - Target: >20 QPS for semantic search

3. **Connection Pool**:
   - Active connections
   - Wait time
   - Connection reuse rate

4. **Index Usage**:
   - Index scan vs seq scan ratio
   - Index hit rate
   - Index size vs table size

### SQL Monitoring Queries

```sql
-- Check index usage
SELECT schemaname, tablename, indexname, idx_scan, idx_tup_read, idx_tup_fetch
FROM pg_stat_user_indexes
WHERE tablename = 'document_embeddings'
ORDER BY idx_scan DESC;

-- Check slow queries
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
WHERE query LIKE '%document_embeddings%'
ORDER BY mean_exec_time DESC
LIMIT 10;

-- Check table statistics
SELECT * FROM pg_stat_user_tables
WHERE relname = 'document_embeddings';
```

---

## Conclusion

The semantic search queries are well-structured but require proper indexing for production performance. The most critical optimizations are:

1. **Connection pooling** (✅ complete) - 10-30% improvement
2. **Vector indexes** (⏳ pending) - 10-100x improvement
3. **Hybrid search parallelization** (⏳ pending) - 30-50% improvement

These three optimizations will provide 50-200x cumulative performance improvement, making the system production-ready for large-scale deployment.

---

*Analysis Date: November 15, 2025*
*Status: Recommendations ready for implementation*
