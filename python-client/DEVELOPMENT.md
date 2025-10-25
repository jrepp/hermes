# hc-hermes Python Client

This directory contains the Python client library for the Hermes document management system.

See the main [README.md](../README.md) for installation and usage instructions.

## Development

### Setup

```bash
# Install uv
curl -LsSf https://astral.sh/uv/install.sh | sh

# Create virtual environment
uv venv

# Activate virtual environment
source .venv/bin/activate  # or .venv\Scripts\activate on Windows

# Install dependencies
uv pip install -e ".[dev,cli]"
```

### Running Tests

```bash
# Run all tests
pytest

# Run with coverage
pytest --cov

# Run specific test file
pytest tests/test_models.py

# Run with verbose output
pytest -v
```

### Linting and Type Checking

```bash
# Lint with ruff
ruff check .

# Format code
ruff format .

# Type check
mypy src tests

# All checks
ruff check . && ruff format --check . && mypy src tests
```

### Building

```bash
# Build package
uv build

# Install locally
uv pip install -e .
```

## Project Structure

```
python-client/
├── src/hc_hermes/        # Source code
│   ├── __init__.py       # Package exports
│   ├── client.py         # Synchronous client
│   ├── client_async.py   # Async client
│   ├── config.py         # Configuration
│   ├── exceptions.py     # Exception classes
│   ├── http_client.py    # HTTP client with auth
│   ├── models.py         # Pydantic models
│   ├── utils.py          # Frontmatter utilities
│   └── cli.py            # Command-line interface
├── tests/                # Test suite
│   ├── test_config.py
│   ├── test_models.py
│   └── test_utils.py
├── pyproject.toml        # Package configuration
├── README.md             # Documentation
└── .python-version       # Python version
```
