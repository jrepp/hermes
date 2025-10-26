# Hermes Python Testing Framework

**Professional Python-based testing infrastructure for Hermes distributed document management.**

This framework replaces bash-based testing scripts with a comprehensive Python solution leveraging the `hc-hermes` client library for automated seeding, validation, and E2E scenario testing.

## Features

- ✅ **Type-Safe Document Generation** - Pydantic models with validation
- ✅ **Automated Scenario Testing** - Basic, migration, multi-author scenarios
- ✅ **API Validation Framework** - Assertions for indexing, search, and document state
- ✅ **Pytest Integration** - Full test suite with fixtures and markers
- ✅ **Rich CLI Output** - Beautiful terminal output with progress indicators
- ✅ **Retry Logic** - Automatic retries for indexing waits with tenacity
- ✅ **Comprehensive Coverage** - Unit, integration, and E2E tests

## Architecture

```
testing/python/
├── __init__.py           # Package initialization
├── config.py             # Configuration management
├── generators.py         # Document generators (RFC, PRD, Meeting Notes)
├── seeding.py            # Workspace seeding utilities
├── validation.py         # API validation and assertions
├── scenarios.py          # Scenario orchestration
├── seed.py               # CLI for seeding workspaces
├── scenario_*.py         # Individual scenario runners
├── pyproject.toml        # Python package configuration
└── tests/                # Pytest test suite
    ├── conftest.py       # Pytest fixtures
    ├── test_generators.py
    ├── test_seeding.py
    └── test_scenarios.py
```

## Quick Start

### 1. Set Up Environment

```bash
# From testing/ directory
make python-setup

# Or manually
cd python
pip install -e ".[dev]"
cd ../python-client
pip install -e ".[dev,cli]"
```

### 2. Start Hermes

```bash
# From testing/ directory
make up
```

### 3. Run Scenarios

```bash
# Using Make (recommended)
make scenario-basic-py              # Run basic scenario
make scenario-migration-py          # Run migration scenario
make scenario-multi-author-py       # Run multi-author scenario

# Or run Python scripts directly
cd python
python scenario_basic.py
python scenario_migration.py
python scenario_multi_author.py
```

### 4. Run Tests

```bash
# From testing/ directory
make test-python                    # Unit tests only
make test-python-integration        # Integration tests (requires Hermes)
make test-python-all                # All tests
make test-python-coverage           # With coverage report
```

## Usage Guide

### Seeding Workspaces

#### Via Make

```bash
make python-seed                    # Basic: 10 documents
make python-seed-migration          # Migration: 5 docs in 2 workspaces
make python-seed-multi-author       # Multi-author: 10 documents
```

#### Via CLI

```bash
cd python

# Basic seeding
python seed.py --scenario basic --count 10 --clean

# Migration scenario
python seed.py --scenario migration --count 5 --clean

# Multi-author scenario
python seed.py --scenario multi-author --count 10 --clean
```

#### Programmatically

```python
from seeding import WorkspaceSeeder, ScenarioType

seeder = WorkspaceSeeder()

# Seed basic scenario
files = seeder.seed_basic(count=10, clean=True)
print(f"Created {len(files)} files")

# Seed migration scenario
source, target = seeder.seed_migration(count=5, clean=True)
print(f"Created {len(source)} source + {len(target)} target files")

# Seed multi-author scenario
files = seeder.seed_multi_author(count=10, clean=True)
```

### Running Scenarios

#### Via Make

```bash
# Basic distributed indexing
make scenario-basic-py

# Migration with conflict detection
make scenario-migration-py

# Multi-author collaboration
make scenario-multi-author-py

# Full distributed test (start + seed + run)
make test-distributed-py
```

#### Programmatically

```python
from scenarios import ScenarioRunner

runner = ScenarioRunner()

# Run basic scenario
stats = runner.run_basic_scenario(
    count=10,
    clean=True,
    wait_for_indexing=True
)
print(f"Indexed {stats['total']} documents")

# Run migration scenario
stats = runner.run_migration_scenario(count=5, clean=True)

# Run multi-author scenario
stats = runner.run_multi_author_scenario(count=10, clean=True)
```

### Validation

```python
from validation import HermesValidator

validator = HermesValidator()

# Health check
validator.assert_healthy()

# Wait for indexing
validator.wait_for_indexing(expected_count=10)

# Assert document count
validator.assert_document_count(expected_count=10, wait=True)

# Test search
results = validator.assert_search_results("RFC", min_results=1)

# Check document exists
doc = validator.assert_document_exists("DOC-123")

# Get statistics
stats = validator.get_document_stats()
validator.print_stats()
```

## Testing

### Test Structure

```python
# tests/test_generators.py - Unit tests for generators
# tests/test_seeding.py - Tests for workspace seeding
# tests/test_scenarios.py - Integration tests for scenarios
# tests/conftest.py - Pytest fixtures
```

### Pytest Markers

```bash
# Run unit tests only (fast, no Hermes required)
pytest -m "not integration and not slow"

# Run integration tests (requires Hermes)
pytest -m integration

# Run slow tests (includes indexing wait)
pytest -m slow

# Run basic scenario tests
pytest -m basic

# Run migration scenario tests
pytest -m migration

# Run multi-author scenario tests
pytest -m multi_author
```

### Example Test

```python
import pytest
from scenarios import ScenarioRunner

@pytest.mark.integration
@pytest.mark.basic
class TestBasicScenario:
    @pytest.mark.slow
    def test_basic_scenario(self, runner: ScenarioRunner) -> None:
        """Run complete basic scenario."""
        stats = runner.run_basic_scenario(count=5, clean=True)
        
        assert stats["total"] >= 5
        assert len(stats.get("by_type", {})) > 0
```

## Configuration

### Environment Variables

```bash
# Hermes API configuration
export HERMES_BASE_URL="http://localhost:8001"
export HERMES_AUTH_TOKEN="your-token"  # Optional for most operations

# Indexer configuration
export INDEXER_POLL_INTERVAL=5         # Poll every 5 seconds
export INDEXER_MAX_WAIT=120            # Max wait 2 minutes
```

### Config Object

```python
from config import config

# Access configuration
print(config.hermes_base_url)          # http://localhost:8001
print(config.workspaces_dir)           # ../workspaces
print(config.default_document_count)   # 10

# Update configuration
config.indexer_max_wait = 180          # 3 minutes
```

## Document Generators

### Supported Document Types

- **RFC** - Request for Comments with proper structure
- **PRD** - Product Requirements Document
- **Meeting Notes** - Meeting notes with attendees and action items
- **Documentation** - Documentation pages with categories

### Generator API

```python
from generators import DocumentGenerator, DocumentStatus

gen = DocumentGenerator()

# Generate RFC
rfc_content = gen.generate_rfc(
    number=1,
    title="My RFC",
    status=DocumentStatus.DRAFT,
    author="alice@example.com",
    product="Test Product"
)

# Generate PRD
prd_content = gen.generate_prd(
    number=2,
    title="My PRD",
    status=DocumentStatus.IN_REVIEW,
    author="bob@example.com"
)

# Generate Meeting Notes
meeting_content = gen.generate_meeting_notes(
    number=1,
    title="Team Sync",
    attendees=["alice@example.com", "bob@example.com"]
)

# Generate Documentation Page
doc_content = gen.generate_doc_page(
    title="Testing Guide",
    category="Testing",
    author="charlie@example.com"
)
```

### Document Structure

All documents include:
- YAML frontmatter with proper metadata
- Hermes UUID for stable identification
- Document type, status, authors
- Created and modified timestamps
- Searchable, realistic content
- Proper Markdown formatting

## Scenarios

### Basic Scenario

**Purpose**: Validate basic distributed indexing workflow

**Steps**:
1. Verify Hermes is running
2. Seed 10 test documents (RFC, PRD, Meeting Notes)
3. Wait for indexing (max 2 minutes)
4. Validate documents via API
5. Test search functionality

**Usage**:
```bash
make scenario-basic-py
# Or: cd python && python scenario_basic.py
```

### Migration Scenario

**Purpose**: Test document migration and conflict detection

**Steps**:
1. Verify Hermes is running
2. Create documents with same UUID in source and target workspaces
3. Modify content to trigger conflict detection
4. Wait for indexing
5. Verify duplicate/conflict handling

**Usage**:
```bash
make scenario-migration-py
# Or: cd python && python scenario_migration.py
```

**Note**: Full conflict detection requires backend support for `document_revisions` table and content hash tracking.

### Multi-Author Scenario

**Purpose**: Test multi-author collaboration workflows

**Steps**:
1. Verify Hermes is running
2. Create documents from different authors (alice, bob, charlie, diana)
3. Stagger timestamps (realistic timeline)
4. Vary statuses (draft, review, approved)
5. Wait for indexing
6. Validate document statistics

**Usage**:
```bash
make scenario-multi-author-py
# Or: cd python && python scenario_multi_author.py
```

## Migration from Bash Scripts

### Bash → Python Equivalents

| Bash Script | Python Equivalent | Notes |
|-------------|-------------------|-------|
| `scripts/lib/document-generator.sh` | `generators.py` | Full Pydantic models, type-safe |
| `scripts/seed-workspaces.sh` | `seeding.py` | Object-oriented, better error handling |
| `scripts/scenario-basic.sh` | `scenario_basic.py` | Uses hc-hermes client, rich output |
| Manual API calls with `curl` | `validation.py` | Type-safe API client with retries |
| N/A | `scenarios.py` | New orchestration layer |
| N/A | pytest tests | Automated test suite |

### Key Improvements

1. **Type Safety**: Pydantic models validate all data
2. **Better Error Handling**: Try/except with meaningful messages
3. **API Client**: Uses `hc-hermes` instead of raw curl
4. **Retry Logic**: Automatic retries with tenacity
5. **Rich Output**: Beautiful terminal output with progress bars
6. **Pytest Integration**: Full test suite with fixtures
7. **Maintainability**: Object-oriented, modular design

### Why Python?

- ✅ Type-safe API interactions via `hc-hermes`
- ✅ Better error handling and validation
- ✅ Easier to test and maintain
- ✅ Rich ecosystem (pytest, pydantic, rich)
- ✅ Async support for future enhancements
- ✅ IDE autocomplete and type checking
- ✅ Reusable across different scenarios

## Development

### Code Quality

```bash
# Lint with ruff
make lint-python

# Format with ruff
make format-python

# Type check with mypy (from python/ directory)
cd python && mypy .
```

### Contributing

1. Add new generators to `generators.py`
2. Add new scenarios to `scenarios.py`
3. Add corresponding CLI scripts (`scenario_*.py`)
4. Add pytest tests in `tests/`
5. Update this README
6. Add Make targets if needed

### Adding a New Scenario

1. **Create generator** (if needed):
```python
# generators.py
def generate_new_type(self, **kwargs) -> str:
    # Generate document with frontmatter
    pass
```

2. **Add seeding method**:
```python
# seeding.py
def seed_new_scenario(self, count: int, clean: bool = False) -> list[Path]:
    # Create documents and return file paths
    pass
```

3. **Add scenario runner**:
```python
# scenarios.py
def run_new_scenario(self, count: int, clean: bool = True) -> dict:
    # Orchestrate scenario steps with validation
    pass
```

4. **Create CLI script**:
```python
# scenario_new.py
from scenarios import runner
runner.run_new_scenario(count=10, clean=True)
```

5. **Add pytest test**:
```python
# tests/test_scenarios.py
@pytest.mark.integration
def test_new_scenario(runner):
    stats = runner.run_new_scenario(count=5)
    assert stats["total"] >= 5
```

6. **Add Make target**:
```makefile
# Makefile
scenario-new-py:
	cd python && python scenario_new.py
```

## Troubleshooting

### Hermes Not Running

**Error**: `Hermes is not healthy at http://localhost:8001`

**Solution**:
```bash
cd testing
make up
make status  # Verify services are running
```

### Indexing Timeout

**Error**: `Timeout waiting for indexing`

**Cause**: Indexer scans every 5 minutes by default

**Solutions**:
1. Wait longer (increase `INDEXER_MAX_WAIT`)
2. Trigger manual scan (if supported)
3. Check indexer logs: `make logs-hermes`
4. Run scenario without wait: `wait_for_indexing=False`

### Import Errors

**Error**: `Import "hc_hermes" could not be resolved`

**Solution**:
```bash
make python-setup
# Or manually:
cd python-client && pip install -e ".[dev,cli]"
cd ../python && pip install -e ".[dev]"
```

### Workspace Permissions

**Error**: `Permission denied` when writing files

**Solution**:
```bash
# Ensure workspace directories exist and are writable
mkdir -p workspaces/{testing,docs}/{rfcs,prds,meetings,docs}
chmod -R u+w workspaces/
```

## Examples

### Complete E2E Workflow

```bash
# 1. Start Hermes
make up

# 2. Set up Python environment (first time only)
make python-setup

# 3. Run basic scenario
make scenario-basic-py

# Output:
# ╭─────────────────────────────────────────╮
# │  Basic Distributed Indexing Scenario   │
# ╰─────────────────────────────────────────╯
#
# [1/5] Verifying Hermes is running...
# ✓ Hermes is healthy
#
# [2/5] Seeding 10 test documents...
# ⠋ Generating 3 RFCs...
# ⠋ Generating 3 PRDs...
# ⠋ Generating 4 meeting notes...
# ✓ Created 10 files
#
# [3/5] Waiting for indexer...
#   Documents indexed: 10 / 10
# ✓ All documents indexed!
#
# [4/5] Getting document statistics...
#
# [5/5] Testing search functionality...
# ✓ Search 'test' returned 10 results
# ✓ Search 'RFC' returned 3 results
# ✓ Search 'distributed' returned 10 results
#
# ══════════════════════════════════════════
# Scenario Complete
# ══════════════════════════════════════════
```

### Programmatic Usage

```python
#!/usr/bin/env python3
"""Custom scenario example."""

from generators import DocumentGenerator, DocumentStatus
from seeding import WorkspaceSeeder
from validation import HermesValidator
from scenarios import ScenarioRunner

# Initialize components
seeder = WorkspaceSeeder()
validator = HermesValidator()
runner = ScenarioRunner(seeder=seeder, validator=validator)

# Health check
validator.assert_healthy()

# Generate custom documents
gen = DocumentGenerator()
for i in range(5):
    content = gen.generate_rfc(
        number=i,
        title=f"Custom RFC {i}",
        status=DocumentStatus.APPROVED,
        author="custom@example.com"
    )
    # Write to workspace...

# Wait for indexing
validator.wait_for_indexing(expected_count=5)

# Validate results
stats = validator.get_document_stats()
validator.print_stats()

# Search validation
results = validator.assert_search_results("custom", min_results=5)
print(f"Found {len(results.hits)} custom documents")
```

## References

- **hc-hermes Client**: `../python-client/README.md`
- **Design Document**: `../docs-internal/DISTRIBUTED_TESTING_PROGRESS.md`
- **Architecture**: `../docs-internal/DISTRIBUTED_PROJECTS_ARCHITECTURE.md`
- **Testing Environment**: `../README.md`

## License

Mozilla Public License 2.0 - See [../../LICENSE](../../LICENSE)
