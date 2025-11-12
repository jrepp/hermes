# Scripts Directory

This directory contains scripts for validation, testing, and development workflows.

## Available Scripts

### `validate-go-syntax.sh`
Validates Go syntax across the entire codebase.

**Usage:**
```bash
./scripts/validate-go-syntax.sh
```

**With complexity analysis:**
```bash
RUN_COMPLEXITY=true ./scripts/validate-go-syntax.sh
```

**Checks:**
- Code formatting (gofmt)
- Package builds
- Test compilation
- Static analysis (go vet)
- Optional: Complexity analysis

### `check-complexity.sh`
Analyzes code complexity using cyclomatic and cognitive metrics.

**Usage:**
```bash
./scripts/check-complexity.sh
```

**Output:**
- Functions exceeding complexity thresholds (15 cyclomatic, 20 cognitive)
- Top 10 most complex functions report
- Recommendations for refactoring

**Tools used:**
- `gocyclo` - Cyclomatic complexity
- `gocognit` - Cognitive complexity

## Integration

These scripts are integrated with:
- **Pre-commit hooks** (`.pre-commit-config.yaml`)
- **GitHub Actions** (`.github/workflows/lint.yml`)
- **Makefile** (`make validate`, `make complexity`)

## Adding New Scripts

When adding new scripts:

1. Make them executable: `chmod +x scripts/new-script.sh`
2. Add documentation to this README
3. Consider integration with:
   - Pre-commit hooks
   - GitHub Actions
   - Makefile targets
4. Update [docs/development/validation-tools.md](/docs/development/validation-tools.md)

## Related Documentation

- [Validation Tools Guide](/docs/development/validation-tools.md)
- [Quick Start Guide](/VALIDATION-QUICK-START.md)
