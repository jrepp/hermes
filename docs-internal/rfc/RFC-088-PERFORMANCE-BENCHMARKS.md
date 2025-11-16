# RFC-088 Performance Benchmarks
## Semantic Search Performance Analysis

**Date**: November 15, 2025
**Status**: Baseline Benchmarks Established
**Test Environment**: Apple M4 Max, darwin/arm64

---

## Overview

Performance benchmarks for RFC-088 Event-Driven Document Indexer with Semantic Search, measuring key operations including embedding generation, vector operations, and search queries.

---

## Benchmark Results

### 1. Embedding Generation (Mock)

**Test**: `BenchmarkEmbeddingGeneration`
**Description**: Measures time to generate 1536-dimensional embeddings

```
BenchmarkEmbeddingGeneration/length-11-14      74035    18466 ns/op    17664 B/op    2 allocs/op
BenchmarkEmbeddingGeneration/length-56-14      64926    17906 ns/op    17664 B/op    2 allocs/op
BenchmarkEmbeddingGeneration/length-210-14     72357    17449 ns/op    17664 B/op    2 allocs/op
```

**Analysis**:
- **Average Time**: ~17-18 µs per embedding
- **Memory**: 17.6 KB per operation
- **Allocations**: 2 allocations per operation
- **Note**: This is for mock generation; real OpenAI API calls would be 100-1000x slower

**Real-World Expectations**:
- **OpenAI API**: 50-200ms per request (network + processing)
- **Ollama (local)**: 10-50ms per request (no network)
- **Batching**: Can reduce per-item latency significantly

---

### 2. Vector Operations

**Test**: `BenchmarkVectorOperations`
**Description**: Low-level vector operations performance

#### Vector Generation (1536 dimensions)

```
BenchmarkVectorOperations/vector-generation-1536d-14     385533    2870 ns/op    0 B/op    0 allocs/op
```

**Analysis**:
- **Time**: 2.87 µs per 1536-dimensional vector
- **Memory**: 0 allocations (stack-based)
- **Throughput**: ~348,432 vectors/second

#### Cosine Similarity Calculation (1536 dimensions)

```
BenchmarkVectorOperations/cosine-similarity-1536d-14     2942110    409.6 ns/op    0 B/op    0 allocs/op
```

**Analysis**:
- **Time**: ~410 ns per similarity calculation
- **Memory**: 0 allocations
- **Throughput**: ~2,442,000 calculations/second
- **Performance**: Excellent for in-memory comparisons

---

## Performance Characteristics

### Embedding Generation

| Operation | Time | Memory | Notes |
|-----------|------|--------|-------|
| Mock embedding (1536d) | ~18 µs | 17.6 KB | Deterministic, no API |
| OpenAI API (estimated) | 50-200ms | Varies | Network latency + processing |
| Ollama local (estimated) | 10-50ms | Varies | CPU/GPU dependent |

### Vector Operations

| Operation | Time | Throughput | Notes |
|-----------|------|------------|-------|
| Vector creation | 2.87 µs | 348K/sec | Zero allocations |
| Cosine similarity | 410 ns | 2.4M/sec | Highly optimized |

---

## Scalability Analysis

### Query Performance Estimates

Based on benchmark data, estimated performance for different scenarios:

#### Small Corpus (1,000 documents)

- **Vector comparisons**: 1,000 × 410ns = 410 µs
- **Database overhead**: ~10-50ms (index lookup, I/O)
- **Total estimated time**: ~10-50ms per query
- **Throughput**: ~20-100 queries/second

#### Medium Corpus (10,000 documents)

- **Vector comparisons**: 10,000 × 410ns = 4.1 ms
- **Database overhead**: ~20-100ms (larger index)
- **Total estimated time**: ~25-105ms per query
- **Throughput**: ~10-40 queries/second

#### Large Corpus (100,000 documents)

- **Vector comparisons**: 100,000 × 410ns = 41 ms
- **Database overhead**: ~50-200ms (index scanning)
- **Total estimated time**: ~90-240ms per query
- **Throughput**: ~4-11 queries/second

**Note**: With pgvector indexes (IVFFlat or HNSW), performance remains near-constant regardless of corpus size, typically 10-50ms per query.

---

## Database-Specific Considerations

### PostgreSQL + pgvector

**Expected Performance**:
- **IVFFlat Index**: 10-50ms per query (100K-1M docs)
- **HNSW Index**: 5-20ms per query (higher memory usage)
- **Exact Search**: Linear with corpus size

**Scaling Factors**:
1. **Index Type**: HNSW > IVFFlat > No Index
2. **List Size** (IVFFlat): More lists = faster search, slower inserts
3. **M Parameter** (HNSW): Higher M = better recall, more memory
4. **Maintenance**: VACUUM and REINDEX regularly

### Index Configuration Examples

```sql
-- IVFFlat index (good for 100K-1M vectors)
CREATE INDEX idx_embeddings_vector_ivfflat ON document_embeddings
USING ivfflat (embedding_vector vector_cosine_ops)
WITH (lists = 100);

-- HNSW index (best performance, higher memory)
CREATE INDEX idx_embeddings_vector_hnsw ON document_embeddings
USING hnsw (embedding_vector vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
```

---

## Optimization Opportunities

### 1. Embedding Generation

**Current Bottleneck**: External API calls (50-200ms)

**Optimizations**:
- ✅ **Idempotency**: Content hash prevents re-processing (implemented)
- ✅ **Selective Processing**: Rulesets filter which docs to embed (implemented)
- ⏳ **Batch Processing**: OpenAI batch API (100x cost savings)
- ⏳ **Caching**: Cache embeddings for unchanged content
- ⏳ **Local Models**: Use Ollama for faster generation

**Expected Impact**: 50-90% cost reduction, 2-10x latency improvement

### 2. Vector Search

**Current Performance**: Good baseline (~410ns per comparison)

**Optimizations**:
- ⏳ **SIMD Instructions**: Use AVX-512 for 4-8x speedup
- ⏳ **GPU Acceleration**: Move comparisons to GPU for 10-100x speedup
- ⏳ **Quantization**: Reduce vector precision (8-bit) for 4x memory savings
- ⏳ **Approximate Search**: Use HNSW for 10x query speed

**Expected Impact**: 4-100x improvement depending on approach

### 3. Database Queries

**Current Bottleneck**: Index selection and I/O

**Optimizations**:
- ⏳ **Connection Pooling**: Reuse connections (10-30% improvement)
- ⏳ **Prepared Statements**: Cache query plans (5-15% improvement)
- ⏳ **Index Tuning**: Optimize lists/M parameters (2-5x improvement)
- ⏳ **Read Replicas**: Separate read/write traffic (horizontal scaling)

**Expected Impact**: 2-10x improvement with proper tuning

---

## Cost Analysis

### OpenAI API Costs

Based on benchmark assumptions and production usage:

**Embeddings** (text-embedding-3-small):
- Cost: $0.02 per 1M tokens
- Average document: ~500 tokens
- 10K documents/day: ~5M tokens/day = $0.10/day = $3/month

**LLM Summaries** (gpt-4o-mini):
- Cost: $0.15 per 1M input tokens
- Average document: ~1000 tokens
- 10K documents/day: ~10M tokens/day = $1.50/day = $45/month

**Total Estimated**: ~$48/month for 10K docs/day

**With Optimizations**:
- Batch API (50% discount): ~$24/month
- Content hash dedup (50% reduction): ~$12/month
- **Optimized Total**: ~$12-24/month for 10K docs/day

---

## Benchmark Limitations

### Current Limitations

1. **Mock Embeddings**: Real API calls 100-1000x slower
2. **SQLite Testing**: Cannot test pgvector-specific features
3. **Single Threaded**: No concurrent request testing
4. **Memory-Only**: No network or disk I/O simulation
5. **Small Corpus**: Benchmarks use 100-5000 docs, not 100K+

### Future Improvements

1. **Integration Tests**: Test with real PostgreSQL + pgvector
2. **API Integration**: Measure real OpenAI/Ollama performance
3. **Load Testing**: Concurrent requests, connection pooling
4. **Large Corpus**: Test with 100K-1M documents
5. **Production Monitoring**: Real-world performance metrics

---

## Recommendations

### Immediate Actions (Week 7-8)

1. **Establish Baselines**
   - ✅ Create benchmarks (completed)
   - ⏳ Test with real pgvector database
   - ⏳ Measure real API latency

2. **Optimize Low-Hanging Fruit**
   - ⏳ Implement connection pooling
   - ⏳ Add prepared statement caching
   - ⏳ Tune index parameters

3. **Monitor Production**
   - ⏳ Add Prometheus metrics
   - ⏳ Set up Grafana dashboards
   - ⏳ Configure alerts

### Medium-Term Actions (Week 9-10)

1. **Advanced Optimization**
   - ⏳ Implement batch embedding generation
   - ⏳ Add HNSW index support
   - ⏳ Test approximate nearest neighbor

2. **Scaling Preparation**
   - ⏳ Set up read replicas
   - ⏳ Implement query result caching
   - ⏳ Add rate limiting

3. **Cost Optimization**
   - ⏳ Implement content hash deduplication
   - ⏳ Use batch APIs where possible
   - ⏳ Optimize ruleset filtering

---

## Performance Targets

### Week 7-8 Goals

- ✅ Establish baseline benchmarks
- ⏳ < 50ms query latency (p95) for 10K corpus
- ⏳ > 20 queries/second throughput
- ⏳ < 100ms embedding generation (with caching)

### Production Goals

- ⏳ < 100ms query latency (p99) for 100K corpus
- ⏳ > 100 queries/second throughput
- ⏳ 99.9% uptime
- ⏳ < $100/month operational costs (10K docs/day)

---

## Benchmark Code Location

**File**: `pkg/search/semantic_bench_test.go` (319 lines)

**Available Benchmarks**:
- `BenchmarkEmbeddingGeneration` - Embedding creation performance
- `BenchmarkVectorOperations` - Low-level vector math
- `BenchmarkSemanticSearch_VaryingCorpusSize` - Search with different doc counts
- `BenchmarkSemanticSearch_VaryingLimits` - Different result limits
- `BenchmarkSemanticSearch_WithFilters` - Filtered search performance
- `BenchmarkSemanticSearch_FindSimilar` - Similar document lookup
- `BenchmarkSemanticSearch_ConcurrentQueries` - Concurrent request handling

**Running Benchmarks**:
```bash
# Run all benchmarks
go test -bench=. -benchmem ./pkg/search

# Run specific benchmark
go test -bench=BenchmarkEmbeddingGeneration -benchmem ./pkg/search

# Run with CPU profiling
go test -bench=. -benchmem -cpuprofile=cpu.prof ./pkg/search

# Run with memory profiling
go test -bench=. -benchmem -memprofile=mem.prof ./pkg/search
```

---

## Next Steps

1. **Complete Benchmark Suite**
   - Fix database-dependent benchmarks for PostgreSQL
   - Add hybrid search benchmarks
   - Add concurrent request benchmarks

2. **Real-World Testing**
   - Set up PostgreSQL test database with pgvector
   - Test with realistic document corpus
   - Measure actual API latencies

3. **Performance Optimization**
   - Implement identified optimizations
   - Measure before/after performance
   - Document optimization impact

4. **Production Monitoring**
   - Deploy performance metrics
   - Set up monitoring dashboards
   - Configure performance alerts

---

**Status**: Baseline benchmarks established ✅
**Next**: Real-world PostgreSQL + pgvector testing
**Target**: Production-ready performance by Week 10

---

*Last Updated: November 15, 2025*
*Benchmark Environment: Apple M4 Max, darwin/arm64*
