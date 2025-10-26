#!/usr/bin/env bash
#
# Setup Python testing environment
#
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTING_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PYTHON_CLIENT_DIR="$(cd "${TESTING_DIR}/../python-client" && pwd)"

echo "=== Setting up Python Testing Environment ==="
echo ""

# Check Python version
if ! command -v python3 &> /dev/null; then
    echo "❌ Python 3 is required but not found"
    exit 1
fi

PYTHON_VERSION=$(python3 --version | awk '{print $2}')
echo "✓ Found Python $PYTHON_VERSION"

# Check for pip
if ! command -v pip3 &> /dev/null && ! command -v pip &> /dev/null; then
    echo "❌ pip is required but not found"
    exit 1
fi

PIP_CMD=$(command -v pip3 || command -v pip)
echo "✓ Found pip"

# Install python-client
echo ""
echo "Installing hc-hermes Python client..."
cd "$PYTHON_CLIENT_DIR"
$PIP_CMD install -e ".[dev,cli]"
echo "✓ Installed hc-hermes client"

# Install testing framework
echo ""
echo "Installing Python testing framework..."
cd "$SCRIPT_DIR"
$PIP_CMD install -e ".[dev]"
echo "✓ Installed testing framework"

# Verify imports
echo ""
echo "Verifying installation..."
python3 -c "import hc_hermes; print('✓ hc_hermes imported successfully')"
python3 -c "from generators import DocumentGenerator; print('✓ generators module working')"
python3 -c "from seeding import WorkspaceSeeder; print('✓ seeding module working')"
python3 -c "from validation import HermesValidator; print('✓ validation module working')"
python3 -c "from scenarios import ScenarioRunner; print('✓ scenarios module working')"

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "  1. Start Hermes: cd ../.. && make up"
echo "  2. Run scenario: python scenario_basic.py"
echo "  3. Run tests: pytest tests/ -v"
echo ""
