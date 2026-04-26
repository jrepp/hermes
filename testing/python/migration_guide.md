# Migration Guide: Bash Scripts → Python Framework

**Date**: October 24, 2025  
**Purpose**: Guide for transitioning from bash-based testing to Python framework

## TL;DR

```bash
# Old (Bash)
./scripts/seed-workspaces.sh --scenario basic --count 10 --clean
./scripts/scenario-basic.sh

# New (Python) - Same functionality, better experience
make python-seed
make scenario-basic-py
```

## Why Migrate?

| Aspect | Bash Scripts | Python Framework |
|--------|-------------|------------------|
| **Type Safety** | ❌ None | ✅ Full (Pydantic) |
| **Error Handling** | ⚠️ Basic (`set -e`) | ✅ Comprehensive try/except |
| **API Client** | ❌ curl with jq | ✅ hc-hermes (type-safe) |
| **Validation** | ⚠️ Manual | ✅ Automated assertions |
| **Testing** | ❌ None | ✅ Pytest suite |
| **Output** | ⚠️ Basic text | ✅ Rich (colors, progress) |
| **Retry Logic** | ❌ Manual loops | ✅ Automatic (tenacity) |
| **IDE Support** | ❌ Limited | ✅ Full autocomplete |
| **Maintainability** | ⚠️ Medium | ✅ High (OOP, modular) |
| **Extensibility** | ⚠️ Copy/paste | ✅ Inheritance/composition |

## Command Mapping

### Seeding Workspaces

| Bash Command | Python Equivalent | Notes |
|-------------|-------------------|-------|
| `make seed` | `make python-seed` | Basic scenario |
| `make seed-clean` | `make python-seed` (includes --clean) | Cleans by default |
| `make seed-migration` | `make python-seed-migration` | Migration scenario |
| `make seed-multi-author` | `make python-seed-multi-author` | Multi-author |
| `./scripts/seed-workspaces.sh --scenario basic --count 5` | `cd python && python seed.py --scenario basic --count 5 --clean` | Direct CLI |

### Running Scenarios

| Bash Command | Python Equivalent | Notes |
|-------------|-------------------|-------|
| `make scenario-basic` | `make scenario-basic-py` | Basic E2E test |
| (No equivalent) | `make scenario-migration-py` | New scenario |
| (No equivalent) | `make scenario-multi-author-py` | New scenario |
| `make test-distributed` | `make test-distributed-py` | Full E2E |

### Validation

| Bash (curl) | Python Equivalent |
|-------------|-------------------|
| `curl $URL/health` | `validator.assert_healthy()` |
| `curl $URL/api/v2/documents \| jq '.total'` | `validator.get_document_stats()` |
| `curl "$URL/api/v2/search?q=test"` | `validator.assert_search_results("test")` |
| Manual wait loop | `validator.wait_for_indexing(count)` |

## Migration Steps

### 1. Set Up Python Environment

```bash
# From testing/ directory
make python-setup

# Verify installation
cd python
python -c "from generators import DocumentGenerator; print('✓ OK')"
```

### 2. Run Equivalent Commands

```bash
# Instead of:
./scripts/seed-workspaces.sh --scenario basic --count 10

# Use:
make python-seed

# Or:
cd python && python seed.py --scenario basic --count 10 --clean
```

### 3. Adopt Python Scripts

Replace bash script usage:

**Before**:
```bash
# In your workflow
./scripts/seed-workspaces.sh --scenario basic --count 10
./scripts/scenario-basic.sh
```

**After**:
```bash
# In your workflow
make python-seed
make scenario-basic-py
```

### 4. Use Pytest for Testing

**New capability** (no bash equivalent):
```bash
# Run unit tests
make test-python

# Run integration tests
make test-python-integration

# Run with coverage
make test-python-coverage
```

## Code Examples

### Seeding Workspaces

**Bash**:
```bash
#!/usr/bin/env bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/document-generator.sh"

for i in $(seq 1 10); do
    UUID=$(generate_uuid)
    CONTENT=$(generate_rfc "$i" "$UUID" "Test RFC")
    echo "$CONTENT" > "workspace/rfcs/RFC-$i.md"
done
```

**Python**:
```python
from seeding import WorkspaceSeeder

seeder = WorkspaceSeeder()
files = seeder.seed_basic(count=10, clean=True)
print(f"Created {len(files)} documents")
```

### Validation

**Bash**:
```bash
MAX_WAIT=120
ELAPSED=0
while [ $ELAPSED -lt $MAX_WAIT ]; do
    COUNT=$(curl -sf "$URL/api/v2/documents" | jq -r '.total // 0')
    if [ "$COUNT" -ge 10 ]; then
        echo "✓ All indexed"
        break
    fi
    sleep 5
    ELAPSED=$((ELAPSED + 5))
done
```

**Python**:
```python
from validation import HermesValidator

validator = HermesValidator()
validator.wait_for_indexing(expected_count=10)  # Auto-retry with tenacity
print("✓ All indexed")
```

### Running Scenarios

**Bash**:
```bash
#!/usr/bin/env bash
# scenario-basic.sh

# Health check
curl -sf "$HERMES_URL/health" || exit 1

# Seed documents
./scripts/seed-workspaces.sh --scenario basic --count 10

# Wait for indexing (manual loop)
# ... 20+ lines of polling logic ...

# Test search
curl -sf "$HERMES_URL/api/v2/search?q=test" | jq '.hits | length'
```

**Python**:
```python
from scenarios import ScenarioRunner

runner = ScenarioRunner()
stats = runner.run_basic_scenario(count=10, clean=True)
# Handles: health check, seeding, indexing wait, search validation
print(f"Indexed {stats['total']} documents")
```

## Programmatic Usage

### Document Generation

**Bash**:
```bash
# Must source library
source lib/document-generator.sh

# Generate RFC
UUID=$(generate_uuid)
CONTENT=$(generate_rfc "1" "$UUID" "My RFC" "draft" "alice@example.com")
echo "$CONTENT" > rfc-1.md
```

**Python**:
```python
from generators import DocumentGenerator, DocumentStatus

gen = DocumentGenerator()
content = gen.generate_rfc(
    number=1,
    title="My RFC",
    status=DocumentStatus.DRAFT,
    author="alice@example.com"
)
Path("rfc-1.md").write_text(content)
```

### Custom Scenarios

**Bash** (would require new script file):
```bash
#!/usr/bin/env bash
# custom-scenario.sh

source lib/document-generator.sh

# 50+ lines of bash logic...
```

**Python** (simple function):
```python
from scenarios import ScenarioRunner

def run_custom_scenario():
    runner = ScenarioRunner()
    
    # Seed custom documents
    runner.seeder.seed_basic(count=5)
    
    # Validate
    runner.validator.wait_for_indexing(5)
    runner.validator.assert_search_results("custom", min_results=1)
    
    # Get stats
    stats = runner.validator.get_document_stats()
    runner.validator.print_stats()

run_custom_scenario()
```

## Testing Migration

### No Bash Tests → Pytest Suite

Bash scripts had no automated tests. Python framework includes:

```python
# tests/test_generators.py
def test_generate_rfc(generator):
    content = generator.generate_rfc(number=1, title="Test")
    assert content.startswith("---\n")
    assert "uuid" in content

# tests/test_scenarios.py
@pytest.mark.integration
def test_basic_scenario(runner):
    stats = runner.run_basic_scenario(count=5, clean=True)
    assert stats["total"] >= 5
```

Run tests:
```bash
make test-python                # Unit tests
make test-python-integration    # Integration tests
make test-python-coverage       # With coverage
```

## Gradual Migration Strategy

### Phase 1: Try Python Alongside Bash ✅

```bash
# Keep using bash
make seed
make scenario-basic

# Try Python equivalent
make python-seed
make scenario-basic-py
```

### Phase 2: Prefer Python for New Work

- New scenarios: Write in Python
- Custom tests: Use pytest
- CI/CD: Use Python targets

### Phase 3: Full Migration (Optional)

- Update all workflows to use Python
- Bash scripts kept for reference
- Update documentation

## Troubleshooting

### Import Errors

**Problem**: `ModuleNotFoundError: No module named 'hc_hermes'`

**Solution**:
```bash
make python-setup
# Or manually:
cd python-client && pip install -e ".[dev,cli]"
cd testing/python && pip install -e ".[dev]"
```

### Path Issues

**Problem**: Scripts can't find modules when run directly

**Solution**: Use Make targets or `PYTHONPATH`:
```bash
# Preferred
make scenario-basic-py

# Or set PYTHONPATH
cd testing
PYTHONPATH=python:python-client/src python python/scenario_basic.py
```

### Lint Errors in IDE

**Problem**: VSCode shows import errors but code runs fine

**Solution**: Configure Python interpreter:
1. Open testing/python/ folder in VSCode
2. Select Python interpreter with installed packages
3. Restart Python language server

## Best Practices

### DO ✅

- Use Python for new scenarios
- Write pytest tests for validation logic
- Use Make targets for convenience
- Leverage type hints and Pydantic
- Handle errors with try/except
- Use rich console for output

### DON'T ❌

- Mix bash and Python in same workflow
- Skip error handling
- Ignore type errors from mypy
- Write 100+ line functions
- Use `subprocess` to call bash scripts
- Hardcode configuration (use config.py)

## Summary

| Feature | Bash | Python | Winner |
|---------|------|--------|--------|
| Quick one-liners | ✅ Easy | ⚠️ More setup | Bash |
| Type safety | ❌ None | ✅ Full | Python |
| Error handling | ⚠️ Basic | ✅ Comprehensive | Python |
| Testing | ❌ None | ✅ Pytest | Python |
| Maintainability | ⚠️ Medium | ✅ High | Python |
| Extensibility | ⚠️ Limited | ✅ Excellent | Python |
| IDE support | ❌ Limited | ✅ Full | Python |
| Learning curve | ✅ Low | ⚠️ Medium | Bash |
| **Overall** | **Good for simple scripts** | **Better for testing framework** | **Python** |

## Conclusion

- **Bash scripts**: Still available, good for quick ad-hoc tasks
- **Python framework**: Recommended for scenarios, testing, automation
- **Migration**: Gradual, no rush - both work side-by-side

**Recommendation**: Start using Python for new work, migrate existing workflows as needed.

## See Also

- `python/README.md` - Comprehensive Python framework documentation
- `python/IMPLEMENTATION_SUMMARY.md` - Implementation details
- `DISTRIBUTED_TESTING_PROGRESS.md` - Original bash implementation (Phase 1)
