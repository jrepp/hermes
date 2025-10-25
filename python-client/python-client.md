# Python Client for Hermes

**Status**: Initial development, not yet published to PyPI

This directory contains the official Python client library for the HashiCorp Hermes document management system.

## Features

- ✅ Full V2 API coverage (documents, projects, search, reviews)
- ✅ Type-safe Pydantic models
- ✅ Async-first with synchronous facade
- ✅ OAuth 2.0 authentication
- ✅ Frontmatter parser for Markdown documents
- ✅ CLI tool for common operations
- ✅ Strict linting with ruff
- ✅ 100% type coverage with mypy

## Installation

### From Source

```bash
cd python-client

# Using uv (recommended)
uv pip install -e ".[dev,cli]"

# Or using pip
pip install -e ".[dev,cli]"
```

### Usage

See [README.md](README.md) for comprehensive usage examples.

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for development setup and contributing guidelines.

## Integration with Hermes

This client is designed to work with the Hermes V2 API. It supports:

- **Authentication**: Google OAuth, Dex OIDC, Okta
- **Document Operations**: CRUD, content management, related resources
- **Search**: Full-text search with filters
- **Projects**: Workspace project management
- **Reviews**: Document review workflows

## Testing

Run the test suite:

```bash
pytest
```

Integration tests require a running Hermes instance:

```bash
# Start Hermes testing environment
cd ../testing
make up

# Run integration tests
cd ../python-client
pytest tests/integration/
```

## Publishing

**Note**: Not yet published to PyPI. Will be published once stable.

```bash
# Build package
uv build

# Publish to PyPI (maintainers only)
uv publish
```

## License

Mozilla Public License 2.0 - See [../LICENSE](../LICENSE)
