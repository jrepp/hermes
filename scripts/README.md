# Hermes Scripts

This directory contains utility scripts for development and testing.

## canary-local.sh

**Purpose**: Validate local docker-compose environment by running end-to-end tests.

**What it does**:
1. ✅ Checks if docker-compose services are running (starts them if needed)
2. ✅ Builds the `hermes` binary if needed
3. ✅ Runs comprehensive canary test with Meilisearch backend
4. ✅ Tests full document lifecycle: create → draft → index → search → publish → cleanup

**Usage**:

```bash
# Direct execution
./scripts/canary-local.sh

# Or via Makefile
make canary
```

**What it validates**:
- PostgreSQL connectivity and CRUD operations
- Meilisearch connectivity and indexing
- Draft document creation and indexing
- Document search functionality (drafts and published)
- Document publishing workflow (WIP → Approved)
- Search index management
- Cleanup operations

**Output**: Colorized output showing each step with ✅/❌ indicators.

**Exit codes**:
- `0` - All tests passed
- `1` - One or more tests failed

**Requirements**:
- Docker and docker-compose installed
- Port 5432 (PostgreSQL) available
- Port 7700 (Meilisearch) available

## configure-ollama.sh

**Purpose**: Configure Ollama for local AI document processing (summarization and embeddings).

**What it does**:
1. ✅ Checks if Ollama is installed
2. ✅ Verifies Ollama service is running
3. ✅ Checks for required models (llama3.2, nomic-embed-text)
4. ✅ Pulls missing models automatically
5. ✅ Tests models with sample requests
6. ✅ Provides configuration summary and next steps

**Usage**:

```bash
# Full configuration (check + pull models + test)
./scripts/configure-ollama.sh
make ollama/configure

# Check only (don't pull models)
./scripts/configure-ollama.sh --check-only
make ollama/check

# Force pull models (even if they exist)
./scripts/configure-ollama.sh --pull-models
make ollama/pull

# Show help
./scripts/configure-ollama.sh --help
```

**What it validates**:
- Ollama binary is installed (brew install ollama)
- Ollama service is running at http://localhost:11434
- Summarization model (llama3.2) is available
- Embedding model (nomic-embed-text) is available
- Models respond correctly to test prompts

**Output**: Colorized output with status indicators:
- ✓ (green) - Success
- ✗ (red) - Error
- ⚠ (yellow) - Warning
- ℹ (blue) - Information

**Model details**:
- **llama3.2** (2.0 GB) - Text generation for document summarization
- **nomic-embed-text** (274 MB) - 768-dimensional vector embeddings

**Configuration**:
Environment variables can override defaults:
- `OLLAMA_HOST` - Ollama API endpoint (default: http://localhost:11434)
- `OLLAMA_SUMMARIZATION_MODEL` - Summarization model (default: llama3.2)
- `OLLAMA_EMBEDDING_MODEL` - Embedding model (default: nomic-embed-text)

**Requirements**:
- Ollama installed (brew install ollama)
- ~2.3 GB disk space for models
- Ollama service running (ollama serve)

**Exit codes**:
- `0` - Configuration successful, ready to use
- `1` - Configuration failed, see error messages

**Related Makefile targets**:
- `make ollama/configure` - Full setup
- `make ollama/check` - Check if ready
- `make ollama/pull` - Force update models
- `make ollama/serve` - Start service (foreground)
- `make ollama/serve/background` - Start service (background)
- `make ollama/stop` - Stop service
- `make ollama/test` - Run integration tests

**See also**: 
- `docs-internal/README-ollama.md` - Complete Ollama guide
- `configs/config-ollama-example.hcl` - Configuration template
- `tests/integration/indexer/` - Integration tests

## Development Workflow

```bash
# Start development environment
docker-compose up -d

# Run canary test to validate setup
make canary

# Develop...

# Run canary test before committing
make canary

# Stop environment
docker-compose down
```

## Troubleshooting

If the canary test fails:

1. **Check docker-compose services**:
   ```bash
   docker-compose ps
   ```
   Both services should show "Up" status and be healthy.

2. **Check logs**:
   ```bash
   docker-compose logs postgres
   docker-compose logs meilisearch
   ```

3. **Restart services**:
   ```bash
   docker-compose down
   docker-compose up -d
   sleep 5  # Wait for services to be ready
   make canary
   ```

4. **Clear data and restart**:
   ```bash
   docker-compose down -v
   docker-compose up -d
   sleep 5
   make canary
   ```
