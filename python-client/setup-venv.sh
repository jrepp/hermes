#!/bin/bash
#
# Setup Python 3.10+ Virtual Environment for hc-hermes
#
# This script recreates the virtual environment with Python 3.10+ to support
# modern union type syntax (PEP 604: X | Y instead of Union[X, Y])
#

set -e

echo "🔍 Checking for Python 3.10+..."

# Find Python 3.10+ installation
PYTHON_BIN=""
for version in python3.13 python3.12 python3.11 python3.10; do
    if command -v "$version" &> /dev/null; then
        PYTHON_BIN="$version"
        break
    fi
done

if [ -z "$PYTHON_BIN" ]; then
    echo "❌ ERROR: Python 3.10+ not found"
    echo ""
    echo "Install Python 3.10+ with Homebrew:"
    echo "  brew install python@3.10"
    echo ""
    echo "Or install Python 3.11+:"
    echo "  brew install python@3.11"
    echo ""
    exit 1
fi

PYTHON_VERSION=$("$PYTHON_BIN" --version)
echo "✅ Found $PYTHON_BIN: $PYTHON_VERSION"

# Remove old virtual environment
if [ -d ".venv" ]; then
    echo "🗑️  Removing old virtual environment..."
    rm -rf .venv
fi

# Create new virtual environment
echo "🔨 Creating virtual environment with $PYTHON_BIN..."
"$PYTHON_BIN" -m venv .venv

# Activate and upgrade pip
echo "📦 Upgrading pip..."
.venv/bin/pip install --upgrade pip --quiet

# Install package in development mode
echo "📦 Installing hc-hermes package with development dependencies..."
.venv/bin/pip install -e ".[dev]" --quiet

echo ""
echo "✅ Virtual environment setup complete!"
echo ""
echo "Activate with:"
echo "  source .venv/bin/activate"
echo ""
echo "Run tests with:"
echo "  PYTHONPATH=src pytest tests/ -v"
echo ""
echo "Run linting with:"
echo "  ruff check src/ tests/"
echo ""
echo "Run type checking with:"
echo "  mypy src/ tests/"
echo ""
