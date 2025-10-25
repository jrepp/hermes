#!/bin/bash
set -e

echo "Running pre-commit checks..."

# Lint
echo "→ Linting with ruff..."
ruff check .

# Format check
echo "→ Checking formatting..."
ruff format --check .

# Type check
echo "→ Type checking with mypy..."
mypy src tests

# Run tests
echo "→ Running tests..."
pytest

echo "✓ All checks passed!"
