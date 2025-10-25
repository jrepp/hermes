#!/bin/bash
# Quick demo of Python client with Hermes testing environment

set -e

echo "========================================="
echo "Hermes Python Client Demo"
echo "========================================="
echo ""

# Check if testing environment is running
echo "1. Checking if Hermes testing environment is running..."
if ! curl -s http://localhost:8001/health > /dev/null 2>&1; then
    echo "   ❌ Testing environment not running"
    echo "   Start it with: cd ../testing && make up"
    exit 1
fi
echo "   ✓ Hermes backend running on port 8001"

# Check if frontend is running
if curl -s http://localhost:4201 > /dev/null 2>&1; then
    echo "   ✓ Hermes frontend running on port 4201"
else
    echo "   ⚠️  Frontend not running (optional)"
fi

# Set environment
export HERMES_BASE_URL="http://localhost:8001"
export PYTHONPATH="$(pwd)/src"

echo ""
echo "2. Configuration:"
echo "   Base URL: $HERMES_BASE_URL"
echo "   PYTHONPATH: $PYTHONPATH"

# Activate virtual environment if it exists
if [ -d ".venv" ]; then
    echo "   Virtual env: .venv"
    source .venv/bin/activate
else
    echo "   ⚠️  No virtual environment found"
    echo "   Create one with: python3.10 -m venv .venv && source .venv/bin/activate"
fi

echo ""
echo "3. Running Python client examples..."
echo ""

# Example 1: Get web config
echo "   Example 1: Getting web configuration"
python3 << 'EOF'
import sys
sys.path.insert(0, "src")

from hc_hermes import Hermes

client = Hermes(base_url="http://localhost:8001")
config = client.get_web_config()
print(f"   - Auth Provider: {config.auth_provider}")
print(f"   - Base URL: {config.base_url}")
EOF

echo ""

# Example 2: List projects
echo "   Example 2: Listing workspace projects"
python3 << 'EOF'
import sys
sys.path.insert(0, "src")

from hc_hermes import Hermes

client = Hermes(base_url="http://localhost:8001")
projects = client.projects.list()
print(f"   - Found {len(projects)} projects")
for i, project in enumerate(projects[:3], 1):
    print(f"     {i}. {project.name} - {project.title or 'No title'}")
if len(projects) > 3:
    print(f"     ... and {len(projects) - 3} more")
EOF

echo ""

# Example 3: Search
echo "   Example 3: Searching documents"
python3 << 'EOF'
import sys
sys.path.insert(0, "src")

from hc_hermes import Hermes

client = Hermes(base_url="http://localhost:8001")
try:
    results = client.search.query("test", hits_per_page=5)
    print(f"   - Found {results.nb_hits} total results")
    for i, hit in enumerate(results.hits[:3], 1):
        status = hit.status.value if hit.status else "N/A"
        print(f"     {i}. {hit.title} ({status})")
except Exception as e:
    print(f"   - Search error (may need auth): {e}")
EOF

echo ""
echo "========================================="
echo "✓ Demo complete!"
echo "========================================="
echo ""
echo "Next steps:"
echo "  - See examples/ directory for more usage patterns"
echo "  - Run integration tests: python examples/integration_test.py"
echo "  - Try the CLI: python -m hc_hermes.cli --help"
echo "  - Read QUICKSTART.md for detailed usage guide"
echo ""
