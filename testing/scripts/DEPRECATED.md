# ⚠️ DEPRECATED: Bash Scripts

**The bash scripts in this directory have been deprecated in favor of the unified Python testing framework.**

## Migration Guide

All functionality from the bash scripts has been ported to Python with enhanced features:

### Old → New Command Mapping

| Old Bash Script | New Python Command | Notes |
|----------------|-------------------|-------|
| `./scripts/seed-workspaces.sh --scenario basic --count 10` | `hermes-test seed --scenario basic --count 10` | Same options, better output |
| `./scripts/seed-workspaces.sh --scenario migration` | `hermes-test seed --scenario migration` | More scenarios available |
| `./scripts/scenario-basic.sh` | `hermes-test scenario basic` | Integrated with auth system |
| Manual jq/curl validation | `hermes-test validate --check-all` | Comprehensive validation |

### Quick Start with Python CLI

```bash
# Install (one-time setup)
cd testing/python
pip install -e .

# Seed workspaces
hermes-test seed --scenario basic --count 10 --clean

# Run scenario
hermes-test scenario basic --count 20 --wait

# Validate deployment
hermes-test validate --check-all

# Clean workspaces
hermes-test clean --workspace testing
```

### Why Python?

The new Python framework offers:

✅ **Better error handling** - Detailed error messages and stack traces
✅ **OAuth authentication** - Integrated token management and refresh
✅ **Rich output** - Beautiful colored terminal output with progress indicators
✅ **Type safety** - Pydantic models for validation
✅ **Comprehensive docs** - Inline help and detailed guides
✅ **Unified codebase** - Single language for all testing needs
✅ **CI/CD ready** - GitHub Actions integration
✅ **Token refresh** - Automatic token renewal for long-running tests
✅ **Better validation** - Comprehensive health checks and assertions

## Bash Scripts (Deprecated)

These scripts are kept for reference but **should not be used** for new work:

- ❌ `seed-workspaces.sh` → Use `hermes-test seed`
- ❌ `scenario-basic.sh` → Use `hermes-test scenario basic`
- ❌ `lib/document-generator.sh` → Use `generators.py` module

## Documentation

For comprehensive documentation of the Python framework:

- **CLI Help**: `hermes-test --help`
- **User Guide**: `testing/python/README.md`
- **OAuth Guide**: `testing/python/OAUTH_AUTOMATION_GUIDE.md`
- **API Docs**: Inline docstrings in Python modules

## Removal Timeline

These bash scripts will be removed in a future release. Please migrate to the Python CLI.

If you have use cases not covered by the Python framework, please:
1. Open an issue describing your use case
2. Contribute enhancements to the Python framework
3. Update your workflows to use `hermes-test` CLI

---

**Last Update**: October 25, 2025
**Status**: Deprecated - Do not use for new work
**Replacement**: `testing/python/hermes_test.py` CLI tool
