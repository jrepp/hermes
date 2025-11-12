#!/bin/bash
# Check Go code complexity across the entire codebase
# This script ensures code complexity stays manageable

set -e

echo "=== Go Complexity Analysis ==="

# Install gocyclo if not present
if ! command -v gocyclo &> /dev/null; then
    echo "Installing gocyclo..."
    go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
fi

# Install gocognit if not present
if ! command -v gocognit &> /dev/null; then
    echo "Installing gocognit..."
    go install github.com/uudashr/gocognit/cmd/gocognit@latest
fi

echo ""
echo "Running cyclomatic complexity check..."
echo "Threshold: Functions with complexity > 15 are flagged"
echo ""

# Run gocyclo with threshold of 15
# Exit code will be non-zero if any function exceeds threshold
if ! gocyclo -over 15 .; then
    echo ""
    echo "⚠️  WARNING: Some functions have high cyclomatic complexity (>15)"
    echo "Consider refactoring these functions to improve maintainability"
    echo ""
    CYCLO_FAIL=1
else
    echo "✓ All functions have acceptable cyclomatic complexity (≤15)"
fi

echo ""
echo "Running cognitive complexity check..."
echo "Threshold: Functions with cognitive complexity > 20 are flagged"
echo ""

# Run gocognit with threshold of 20
if ! gocognit -over 20 .; then
    echo ""
    echo "⚠️  WARNING: Some functions have high cognitive complexity (>20)"
    echo "Consider simplifying these functions to improve readability"
    echo ""
    COGNIT_FAIL=1
else
    echo "✓ All functions have acceptable cognitive complexity (≤20)"
fi

echo ""
echo "Generating complexity report..."
echo ""

# Generate top 10 most complex functions report
echo "Top 10 most complex functions (cyclomatic complexity):"
gocyclo -top 10 . | head -11

echo ""
echo "Top 10 most complex functions (cognitive complexity):"
gocognit -top 10 . | head -11

echo ""

# Exit with error if either check failed (for CI)
if [ -n "$CYCLO_FAIL" ] || [ -n "$COGNIT_FAIL" ]; then
    echo "❌ Complexity checks failed - see warnings above"
    echo ""
    echo "Note: These are warnings, not hard failures"
    echo "Review flagged functions and consider refactoring"
    exit 0  # Exit 0 for now, can be changed to exit 1 for strict enforcement
fi

echo "✓ Complexity analysis complete - all checks passed"
