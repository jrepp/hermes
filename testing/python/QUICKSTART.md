# Python Testing Framework

See [README.md](python/README.md) for comprehensive documentation.

## Quick Start

```bash
# Set up Python environment
make python-setup

# Start Hermes
make up

# Run basic scenario
make scenario-basic-py

# Run tests
make test-python-integration
```

## Available Scenarios

- **Basic**: `make scenario-basic-py` - Basic distributed indexing
- **Migration**: `make scenario-migration-py` - Migration with conflict detection
- **Multi-Author**: `make scenario-multi-author-py` - Multi-author collaboration

## Why Python?

The new Python-based testing framework provides:

- ✅ Type-safe API interactions via `hc-hermes` client
- ✅ Better error handling and validation
- ✅ Pytest integration for automated testing
- ✅ Rich CLI output with progress indicators
- ✅ Retry logic for indexing waits
- ✅ Easier to maintain and extend
- ✅ IDE autocomplete and type checking

## Migration from Bash

The bash scripts in `scripts/` are still available but the Python framework is recommended for:

- New scenarios
- Automated testing
- CI/CD integration
- Complex validation logic

See `python/README.md` for migration guide and comparison.
