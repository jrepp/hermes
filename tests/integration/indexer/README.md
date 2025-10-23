# Indexer Integration Tests with Ollama

This directory contains integration tests for the Hermes indexer with AI capabilities using Ollama.

## Overview

These tests validate the complete indexer pipeline with:
- **Local Workspace**: In-memory filesystem (no external dependencies)
- **Ollama AI Provider**: Local Llama models for summarization and embeddings
- **Meilisearch**: Vector search (via testcontainer)
- **PostgreSQL**: Document metadata and revisions (via testcontainer)

## Prerequisites

### 1. Install Ollama

**macOS**:
```bash
brew install ollama
```

**Or download from**: https://ollama.ai/download

### 2. Start Ollama Service

```bash
ollama serve
```

This starts the Ollama API server at `http://localhost:11434`.

### 3. Pull Required Models

**For Summarization Tests**:
```bash
# Llama 3.2 3B - Fast, good quality (required for tests)
ollama pull llama3.2
```

**For Embedding Tests**:
```bash
# Nomic Embed Text - 768 dimensions, optimized for search (required for tests)
ollama pull nomic-embed-text
```

### 4. Verify Ollama Setup

```bash
# Check Ollama is running
curl http://localhost:11434/api/version

# Expected output:
# {"version":"0.x.x"}

# Check models are available
ollama list

# Expected output should include:
# llama3.2:latest
# nomic-embed-text:latest
```

## Running the Tests

### Run All Indexer Integration Tests

```bash
# From repository root
go test -tags=integration -v ./tests/integration/indexer/...
```

### Run Specific Test Suites

**Summarization Tests Only**:
```bash
go test -tags=integration -v ./tests/integration/indexer -run TestOllama_Summarize
```

**Embedding Tests Only**:
```bash
go test -tags=integration -v ./tests/integration/indexer -run TestOllama_Embedding
```

**Pipeline Integration Tests**:
```bash
go test -tags=integration -v ./tests/integration/indexer -run TestOllama_FullPipeline
```

### Run with Verbose Output

```bash
go test -tags=integration -v -count=1 ./tests/integration/indexer/...
```

## Test Architecture

### Test Flow

```
1. TestMain (setup)
   ├── Start PostgreSQL container (testcontainer)
   ├── Start Meilisearch container (testcontainer)
   ├── Verify Ollama is running (fail fast if not)
   └── Run all tests

2. Individual Tests
   ├── Create in-memory local workspace
   ├── Create test documents
   ├── Build indexer pipeline
   ├── Execute commands
   └── Verify results

3. TestMain (teardown)
   └── Stop all containers
```

### Test Fixtures

**LocalWorkspaceFixture**:
- Creates temporary directory
- Initializes local workspace adapter
- Creates sample documents (RFC, PRD, FRD)
- Provides document IDs for testing

**DatabaseFixture**:
- PostgreSQL connection from testcontainer
- GORM instance with auto-migration
- Document revisions and summaries tables

**OllamaFixture**:
- Verifies Ollama is running
- Creates Ollama AI provider
- Configures models (llama3.2, nomic-embed-text)

## Test Coverage

### 1. Summarization Tests (`ollama_summarize_test.go`)

**Test Cases**:
- ✅ Basic summarization with executive summary
- ✅ Key points extraction
- ✅ Topics extraction
- ✅ Tag suggestions
- ✅ Status analysis
- ✅ Summary caching (database)
- ✅ Content hash validation
- ✅ Stale summary detection

**What's Validated**:
- Ollama generates valid JSON responses
- Summary stored in database with correct fields
- Content hash matches document content
- Cached summaries retrieved correctly
- Stale summaries detected on content change

### 2. Embedding Tests (`ollama_embedding_test.go`)

**Test Cases**:
- ✅ Single embedding generation
- ✅ Chunked embeddings with overlap
- ✅ Vector dimensions validation (768 for nomic-embed-text)
- ✅ Chunk position tracking
- ✅ Multiple documents batch processing

**What's Validated**:
- Embeddings have correct dimensions
- Chunks split at word boundaries
- Overlap preserved between chunks
- Chunk metadata (start_pos, end_pos) correct
- Batch processing completes successfully

### 3. Full Pipeline Tests (`ollama_pipeline_test.go`)

**Test Cases**:
- ✅ Complete AI-enhanced indexing pipeline
- ✅ Document discovery → UUID assignment → Hashing → Summarization → Embedding
- ✅ Error handling and rollback
- ✅ Parallel processing of multiple documents
- ✅ Incremental indexing (skip already processed)

**What's Validated**:
- Pipeline executes all commands in order
- Context passed correctly between commands
- Database state consistent after pipeline
- Errors properly propagated
- Parallel execution works correctly

## Configuration

### Environment Variables

These tests respect the following environment variables:

```bash
# Ollama Configuration
export OLLAMA_BASE_URL="http://localhost:11434"  # Default
export OLLAMA_SUMMARIZE_MODEL="llama3.2"         # Default
export OLLAMA_EMBEDDING_MODEL="nomic-embed-text" # Default

# Test Behavior
export SKIP_OLLAMA_TESTS="true"  # Skip tests if Ollama not available
export TEST_TIMEOUT="10m"        # Timeout for long-running tests
```

### Skip Tests if Ollama Not Available

Tests will automatically skip if Ollama is not running:

```go
// In test setup
if !ollamaAvailable() {
    t.Skip("Ollama not running at http://localhost:11434")
}
```

### Custom Ollama Host

If running Ollama on a different host/port:

```bash
export OLLAMA_BASE_URL="http://ollama-server.local:11434"
go test -tags=integration -v ./tests/integration/indexer/...
```

## Troubleshooting

### Ollama Not Running

**Error**:
```
--- SKIP: TestOllama_Summarize (0.00s)
    ollama_summarize_test.go:45: Ollama not running at http://localhost:11434
```

**Solution**:
```bash
# Start Ollama service
ollama serve

# In another terminal, verify
curl http://localhost:11434/api/version
```

### Model Not Found

**Error**:
```
Error: ollama returned status 404: model 'llama3.2' not found
```

**Solution**:
```bash
ollama pull llama3.2
ollama pull nomic-embed-text
```

### Tests Timeout

**Error**:
```
panic: test timed out after 10m0s
```

**Solution**:
```bash
# Increase timeout
go test -tags=integration -v -timeout 20m ./tests/integration/indexer/...

# Or use smaller documents for faster tests
# (tests automatically use small sample documents)
```

### Docker Containers Fail to Start

**Error**:
```
failed to start PostgreSQL container: Cannot connect to the Docker daemon
```

**Solution**:
```bash
# Ensure Docker is running
docker ps

# If not, start Docker Desktop or docker daemon
```

### Port Conflicts

**Error**:
```
failed to start Meilisearch: Bind for 0.0.0.0:7700 failed: port is already allocated
```

**Solution**:
```bash
# Stop conflicting service
lsof -ti:7700 | xargs kill -9

# Or let testcontainers use random ports (default behavior)
```

## Performance Expectations

### Apple Silicon (M1/M2/M3)

**Summarization** (~30-50 tokens/sec):
- Single document: ~5-10 seconds
- 10 documents: ~60-90 seconds
- Test suite: ~2-3 minutes

**Embeddings** (~100 embeddings/sec):
- Single document: <1 second
- 10 documents: ~5-10 seconds
- Test suite: ~30-60 seconds

**Full Pipeline** (with parallel processing):
- 10 documents: ~90-120 seconds
- Test suite: ~5-7 minutes

### Intel/AMD (x86_64)

**Summarization** (~10-20 tokens/sec):
- Single document: ~15-30 seconds
- 10 documents: ~3-5 minutes
- Test suite: ~8-10 minutes

**Embeddings** (~50 embeddings/sec):
- Single document: ~2 seconds
- 10 documents: ~15-20 seconds
- Test suite: ~2-3 minutes

**Full Pipeline**:
- 10 documents: ~5-7 minutes
- Test suite: ~15-20 minutes

## Test Data

### Sample Documents

Tests use realistic sample documents:

**RFC (Request for Comments)**:
- Title: "Indexer Refactor with AI Enhancement"
- Content: Technical proposal with architecture, goals, implementation plan
- Expected Summary: Architecture overview, key points about provider-agnostic design
- Expected Tags: "indexer", "ai", "architecture", "refactor"

**PRD (Product Requirements Document)**:
- Title: "Semantic Search Feature"
- Content: Product requirements, user stories, acceptance criteria
- Expected Summary: Feature overview, user benefits, implementation scope
- Expected Tags: "search", "semantic", "vector", "feature"

**FRD (Functional Requirements Document)**:
- Title: "Document Migration System"
- Content: Functional requirements, use cases, validation rules
- Expected Summary: System capabilities, migration workflows, validation
- Expected Tags: "migration", "cross-provider", "validation", "requirements"

## CI/CD Integration

### GitHub Actions

```yaml
name: Indexer Integration Tests

on: [pull_request]

jobs:
  indexer-tests:
    runs-on: ubuntu-latest
    
    services:
      ollama:
        image: ollama/ollama:latest
        ports:
          - 11434:11434
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Setup Ollama Models
        run: |
          docker exec ${{ job.services.ollama.id }} ollama pull llama3.2
          docker exec ${{ job.services.ollama.id }} ollama pull nomic-embed-text
      
      - name: Run Integration Tests
        run: go test -tags=integration -v -timeout 30m ./tests/integration/indexer/...
```

## Related Documentation

- [INDEXER_REFACTOR_IMPLEMENTATION.md](../../../docs-internal/INDEXER_REFACTOR_IMPLEMENTATION.md) - Complete implementation guide
- [README-ollama.md](../../../docs-internal/README-ollama.md) - Ollama provider documentation
- [README-local-workspace.md](../../../docs-internal/README-local-workspace.md) - Local workspace adapter guide
- [../README.md](../README.md) - Integration tests overview

## Next Steps

After running these tests successfully:

1. **Implement Vector Search Adapter**: Add Meilisearch vector search implementation
2. **Add CLI Integration**: Connect indexer commands to Hermes CLI
3. **Performance Benchmarks**: Add benchmark tests for large document sets
4. **API Integration Tests**: Test indexer via REST API endpoints
5. **Production Deployment**: Deploy with Ollama on dedicated GPU server
