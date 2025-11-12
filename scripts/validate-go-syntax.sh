#!/bin/bash
# Validate Go syntax across the entire codebase
# This script ensures all Go files are syntactically correct

set -e

echo "=== Validating Go Syntax ==="

# Check if gofmt finds any issues
echo "Running gofmt check..."
unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  echo "ERROR: The following files have syntax or formatting issues:"
  echo "$unformatted"
  echo ""
  echo "Run 'gofmt -w .' to fix"
  exit 1
fi

# Try to build all packages
echo "Building all packages..."
if ! go build ./...; then
  echo "ERROR: Some packages failed to build"
  exit 1
fi

# Try to compile all tests (without running them)
echo "Compiling all tests..."
if ! go test -run=^$ ./...; then
  echo "ERROR: Some tests failed to compile"
  exit 1
fi

# Run go vet
echo "Running go vet..."
if ! go vet ./...; then
  echo "ERROR: go vet found issues"
  exit 1
fi

echo "✓ All Go files are syntactically correct"
echo "✓ All packages build successfully"
echo "✓ All tests compile successfully"
echo "✓ No vet issues found"

# Run complexity analysis if requested
if [ "${RUN_COMPLEXITY:-false}" = "true" ]; then
  echo ""
  echo "Running complexity analysis..."
  ./scripts/check-complexity.sh
fi
