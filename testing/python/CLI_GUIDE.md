# Hermes Testing CLI Guide

**Unified command-line interface for Hermes testing operations.**

The `hermes-test` CLI provides a single, cohesive interface for all testing operations, replacing the legacy bash scripts with a professional Python-based solution.

## Installation

```bash
# Install both packages in editable mode
cd /path/to/hermes/python-client
pip3 install -e .

cd ../testing/python
pip3 install -e .
```

This installs the `hermes-test` command. The script will be placed in:
- **Linux/macOS**: `~/.local/bin/hermes-test` or `~/Library/Python/3.9/bin/hermes-test`
- **Windows**: `%APPDATA%\Python\Scripts\hermes-test.exe`

**Add to PATH** (if needed):
```bash
# macOS (add to ~/.zshrc or ~/.bashrc)
export PATH="$HOME/Library/Python/3.9/bin:$PATH"

# Linux (add to ~/.bashrc)
export PATH="$HOME/.local/bin:$PATH"
```

Verify installation:
```bash
# Use full path if not in PATH
~/Library/Python/3.9/bin/hermes-test --help

# Or after adding to PATH
hermes-test --help
```

## Quick Start

```bash
# Start Hermes testing environment
cd /path/to/hermes/testing
make up

# Seed with test documents
hermes-test seed --scenario basic --count 10 --clean

# Run a scenario
hermes-test scenario basic --count 20 --wait --token-refresh

# Validate deployment
hermes-test validate --check-all

# Clean up
hermes-test clean --workspace all
```

## CLI Reference

### Global Options

```
hermes-test [global-options] <subcommand> [subcommand-options]

Global Options:
  -h, --help      Show help message and exit
  -v, --verbose   Enable verbose output (shows DEBUG logs)
```

### Environment Variables

The CLI respects the following environment variables:

```bash
# Hermes server configuration
export HERMES_BASE_URL="http://localhost:8001"

# OAuth authentication (optional, prompts if not set)
export HERMES_AUTH_TOKEN="eyJhbGc..."

# Dex OIDC for token refresh (optional)
export DEX_TEST_USERNAME="test@example.com"
export DEX_TEST_PASSWORD="password"
export DEX_CLIENT_ID="hermes-web"
export DEX_CLIENT_SECRET="ZXhhbXBsZS1hcHAtc2VjcmV0"
export DEX_ISSUER_URL="http://localhost:5558/dex"
```

## Subcommands

### `seed` - Seed Workspaces with Test Documents

Generate and upload test documents to Hermes workspaces.

**Usage:**
```bash
hermes-test seed [options]
```

**Options:**
```
--scenario {basic,migration,conflict,multi-author}
    Scenario type to run (default: basic)
    
--count COUNT
    Number of documents to generate (default: 10)
    
--workspace {testing,docs,all}
    Target workspace for seeding (default: all)
    
--clean
    Clean workspace before seeding (deletes existing documents)
```

**Examples:**
```bash
# Basic seeding: 10 documents in all workspaces
hermes-test seed

# Seed testing workspace with 20 documents, clean first
hermes-test seed --scenario basic --count 20 --workspace testing --clean

# Migration scenario: 5 documents across workspaces
hermes-test seed --scenario migration --count 5

# Multi-author scenario: 15 collaborative documents
hermes-test seed --scenario multi-author --count 15 --clean
```

**Output:**
- Progress bars showing upload status
- Document IDs and titles
- Indexing wait (optional with `--wait`)
- Summary statistics

### `scenario` - Run Test Scenarios

Execute end-to-end testing scenarios with document operations, validation, and assertions.

**Usage:**
```bash
hermes-test scenario {basic,migration,multi-author} [options]
```

**Scenarios:**
- `basic` - Create, read, update, delete operations
- `migration` - Document migration between workspaces
- `multi-author` - Collaborative editing with multiple contributors

**Options:**
```
--count COUNT
    Number of documents to operate on (default: 10)
    
--clean
    Clean workspace before running scenario
    
--wait
    Wait for indexing to complete after each operation
    
--token-refresh
    Enable automatic OAuth token refresh via Dex
```

**Examples:**
```bash
# Basic scenario with 20 documents, wait for indexing
hermes-test scenario basic --count 20 --wait

# Migration scenario with token refresh
hermes-test scenario migration --count 5 --token-refresh

# Multi-author scenario, clean first
hermes-test scenario multi-author --count 15 --clean --wait
```

**Output:**
- Scenario progress with step-by-step validation
- API response times
- Document state verification
- Search validation results
- Final statistics

### `validate` - Validate Hermes Deployment

Check Hermes API health, workspace statistics, and search functionality.

**Usage:**
```bash
hermes-test validate [options]
```

**Options:**
```
--check-all
    Run all validation checks (health, stats, search)
    
--health
    Check API health endpoint
    
--stats
    Show workspace statistics (document counts)
    
--search
    Test search functionality with sample queries
```

**Examples:**
```bash
# Quick health check
hermes-test validate --health

# Full validation suite
hermes-test validate --check-all

# Check stats and search only
hermes-test validate --stats --search
```

**Output:**
- API health status (✓ or ✗)
- Workspace document counts
- Search query results with relevance scores
- Response times

### `clean` - Clean Test Data

Remove test documents from workspaces.

**Usage:**
```bash
hermes-test clean [options]
```

**Options:**
```
--workspace {testing,docs,all}
    Workspace to clean (default: all)
    
--force
    Skip confirmation prompt
```

**Examples:**
```bash
# Clean all workspaces (with confirmation)
hermes-test clean

# Clean testing workspace only
hermes-test clean --workspace testing

# Force clean without prompt
hermes-test clean --workspace all --force
```

**Output:**
- List of documents to be deleted
- Confirmation prompt (unless `--force`)
- Deletion progress
- Summary of deleted documents

## Migration from Bash Scripts

All bash scripts in `testing/scripts/` are **deprecated**. Use the `hermes-test` CLI instead.

### Command Mapping

| Old Bash Script | New CLI Command | Notes |
|----------------|-----------------|-------|
| `./scripts/seed-workspaces.sh` | `hermes-test seed --scenario basic --count 10` | More scenarios available |
| `./scripts/scenario-basic.sh` | `hermes-test scenario basic --wait` | Token refresh available |
| `make seed-testing` | `hermes-test seed --workspace testing --clean` | Direct CLI usage |
| Manual cleanup | `hermes-test clean --workspace all` | Automated cleanup |

### Feature Comparison

| Feature | Bash Scripts | Python CLI |
|---------|-------------|-----------|
| **Type Safety** | ❌ No | ✅ Pydantic models |
| **Error Handling** | ❌ Basic | ✅ Comprehensive |
| **OAuth Refresh** | ❌ Manual | ✅ Automatic |
| **Progress Bars** | ❌ No | ✅ Rich library |
| **Validation** | ❌ Limited | ✅ Full assertions |
| **Testing** | ❌ No tests | ✅ Pytest suite |
| **Documentation** | ❌ Comments | ✅ Docstrings + guides |
| **Maintenance** | ❌ Shell scripting | ✅ Python ecosystem |

### Migration Timeline

- **Phase 1** (Current): Both bash and Python CLI available
- **Phase 2** (Next release): Bash scripts marked deprecated
- **Phase 3** (Future): Remove bash scripts entirely

## Advanced Usage

### Token Refresh

The CLI automatically refreshes OAuth tokens when using `--token-refresh` with Dex OIDC:

```bash
# Set Dex credentials
export DEX_TEST_USERNAME="test@example.com"
export DEX_TEST_PASSWORD="password"

# Run scenario with auto-refresh
hermes-test scenario basic --token-refresh
```

Token is automatically refreshed before expiration.

### Verbose Output

Enable verbose mode to see DEBUG logs:

```bash
hermes-test -v seed --scenario basic --count 10
```

Shows:
- API request/response details
- Document content validation
- Retry attempts
- Timing information

### Chaining Operations

```bash
# Clean, seed, run scenario, validate
hermes-test clean --force && \
  hermes-test seed --scenario basic --count 20 --clean && \
  hermes-test scenario basic --count 20 --wait && \
  hermes-test validate --check-all
```

### Custom Configuration

Override defaults via environment variables:

```bash
# Use different Hermes instance
export HERMES_BASE_URL="https://hermes.staging.example.com"
export HERMES_AUTH_TOKEN="$(cat ~/.hermes/staging-token)"

hermes-test validate --check-all
```

## Makefile Integration

The `testing/Makefile` provides convenience targets wrapping the CLI:

```bash
# From testing/ directory
make test-cli           # Test CLI is installed and working
make seed               # Seed with basic scenario
make seed-clean         # Seed with clean
make scenario-basic     # Run basic scenario
make scenario-migration # Run migration scenario
make validate           # Validate deployment
make clean-workspace    # Clean all workspaces
```

**Deprecated Targets** (use CLI instead):
- `seed-testing`, `seed-docs` → `hermes-test seed --workspace {testing,docs}`
- `scenario-basic-sh` → `hermes-test scenario basic`

## Troubleshooting

### "No module named 'hc_hermes'"

```bash
# Install dependencies
cd /path/to/hermes/python-client
pip3 install -e .
```

### "hermes-test: command not found"

The script is installed but not in your PATH.

```bash
# Option 1: Use full path
~/Library/Python/3.9/bin/hermes-test --help

# Option 2: Add to PATH (recommended)
# Add this to ~/.zshrc or ~/.bashrc
export PATH="$HOME/Library/Python/3.9/bin:$PATH"

# Then reload shell
source ~/.zshrc  # or source ~/.bashrc

# Option 3: Create symlink
sudo ln -s ~/Library/Python/3.9/bin/hermes-test /usr/local/bin/hermes-test

# Verify
hermes-test --help
```

### "Connection refused" errors

```bash
# Verify Hermes is running
curl -I http://localhost:8001/health

# Start if needed
cd /path/to/hermes/testing
make up
```

### "Invalid token" errors

```bash
# Refresh token manually
export HERMES_AUTH_TOKEN="$(curl -s http://localhost:5558/dex/token ...)"

# Or use token refresh
hermes-test scenario basic --token-refresh
```

## See Also

- **README.md** - Framework architecture and Python API
- **OAUTH_AUTOMATION_GUIDE.md** - OAuth token refresh details
- **testing/scripts/DEPRECATED.md** - Bash→Python migration guide
- **testing/Makefile** - Automation targets
- **docs-internal/INDEXER_IMPLEMENTATION_GUIDE.md** - Indexer details

## Support

For issues or questions:
1. Check `hermes-test --help` and subcommand help
2. Review examples in this guide
3. Check `testing/python/README.md` for Python API details
4. See `docs-internal/` for architecture documentation
