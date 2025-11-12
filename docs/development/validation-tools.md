# Validation and Quality Tools

This document describes the validation and code quality tools available for the Hermes project.

## Quick Start

```bash
# Install pre-commit hooks (one-time setup)
make install-hooks

# Run validation before committing
make validate

# Check code complexity
make complexity

# Format code
make fmt
```

## Available Tools

### 1. Syntax Validation

Validates that all Go code is syntactically correct and compiles successfully.

**Run locally:**
```bash
./scripts/validate-go-syntax.sh
```

**What it checks:**
- ✅ Code formatting with `gofmt`
- ✅ All packages build successfully
- ✅ All tests compile (without running them)
- ✅ Static analysis with `go vet`

**CI Integration:** Runs automatically on all PRs via GitHub Actions

### 2. Complexity Analysis

Analyzes code complexity using cyclomatic and cognitive complexity metrics.

**Run locally:**
```bash
./scripts/check-complexity.sh
```

**What it checks:**
- Cyclomatic complexity (threshold: 15)
- Cognitive complexity (threshold: 20)
- Generates top 10 most complex functions report

**Metrics Explained:**
- **Cyclomatic Complexity**: Measures the number of linearly independent paths through code
  - ≤15 is acceptable
  - >15 suggests the function should be refactored
- **Cognitive Complexity**: Measures how difficult code is to understand
  - ≤20 is acceptable
  - >20 suggests the code should be simplified

**CI Integration:** Runs automatically on all PRs via GitHub Actions

**Example output:**
```
Running cyclomatic complexity check...
125 server (*Command).Run internal/cmd/commands/server/server.go:131:1
113 api DocumentHandler internal/api/v2/documents.go:46:1

Top 10 most complex functions (cyclomatic complexity):
...
```

### 3. Pre-commit Hooks

Automatically run validation checks before commits and pushes.

**Install hooks:**
```bash
# Using make
make install-hooks

# Or directly with pre-commit
pip3 install pre-commit
pre-commit install
pre-commit install --hook-type pre-push
```

**Hooks configuration:** `.pre-commit-config.yaml`

**On every commit:**
- Format checking (`gofmt`)
- Static analysis (`go vet`)
- Build validation
- Test compilation
- Module tidiness (`go mod tidy`)
- Trailing whitespace removal
- End-of-file fixing
- YAML validation
- Large file detection
- Merge conflict detection

**On push only:**
- Complexity analysis (to avoid slowing down commits)

**Run manually on all files:**
```bash
make run-hooks
# or
pre-commit run --all-files
```

### 4. GitHub Actions CI/CD

All validation runs automatically in CI/CD.

**Workflow file:** `.github/workflows/lint.yml`

**Steps:**
1. Code formatting check
2. Static analysis (`go vet`)
3. Linter (`golangci-lint`)
4. Build validation
5. Complexity analysis

**Triggered on:**
- Push to `main` branch
- All pull requests

## Using the Makefile

The project includes a comprehensive Makefile for common development tasks.

```bash
# Show all available commands
make help

# Common commands:
make fmt              # Format all Go code
make lint             # Run linters (gofmt, go vet)
make complexity       # Run complexity analysis
make build            # Build all packages
make test             # Run all tests
make test-integration # Run integration tests only
make test-coverage    # Run tests with coverage report
make vet              # Run go vet
make tidy             # Tidy go modules
make pre-commit       # Run pre-commit checks
make validate         # Run full validation (lint + complexity)
make clean            # Clean build artifacts
make install-hooks    # Install pre-commit hooks
make run-hooks        # Run pre-commit hooks on all files
```

## Integration with IDEs

### VS Code

Add to `.vscode/settings.json`:
```json
{
  "go.formatTool": "gofmt",
  "editor.formatOnSave": true,
  "go.vetOnSave": "package",
  "go.buildOnSave": "package",
  "go.lintOnSave": "package"
}
```

### GoLand/IntelliJ

1. Enable "gofmt" on save: `Preferences → Tools → File Watchers`
2. Enable "go vet" on save: `Preferences → Tools → File Watchers`
3. Add external tools for complexity checks:
   - `Tools → External Tools → Add`
   - Program: `$ProjectFileDir$/scripts/check-complexity.sh`

## Bypassing Checks (Emergency Only)

If you need to bypass pre-commit hooks in an emergency:

```bash
# Skip all hooks for a single commit (NOT RECOMMENDED)
git commit --no-verify -m "Emergency fix"

# Skip specific hooks
SKIP=go-complexity git commit -m "Fix with high complexity"
```

**Note:** CI checks will still run on the PR. Only bypass locally if absolutely necessary.

## Troubleshooting

### "gocyclo: command not found"

Install the tools:
```bash
make complexity-install
# or
go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
go install github.com/uudashr/gocognit/cmd/gocognit@latest
```

### "pre-commit: command not found"

Install pre-commit:
```bash
pip3 install pre-commit
```

### Formatting conflicts

If `gofmt` reports issues:
```bash
# Auto-fix formatting issues
make fmt
```

### Build failures

```bash
# Clean and rebuild
make clean
make build
```

## Best Practices

1. **Run validation before pushing:**
   ```bash
   make validate
   ```

2. **Keep functions simple:**
   - Aim for cyclomatic complexity ≤10 for new code
   - Refactor functions with complexity >15

3. **Format code consistently:**
   - Use `make fmt` or let pre-commit hooks handle it
   - Never commit unformatted code

4. **Review complexity reports:**
   - Check complexity trends on complex files
   - Consider refactoring when adding to high-complexity functions

5. **Use the Makefile:**
   - Standardizes commands across the team
   - Ensures everyone uses the same tools

## Configuration Files

- `.pre-commit-config.yaml` - Pre-commit hooks configuration
- `.github/workflows/lint.yml` - CI/CD validation workflow
- `Makefile` - Development commands
- `scripts/validate-go-syntax.sh` - Syntax validation script
- `scripts/check-complexity.sh` - Complexity analysis script

## Adding New Checks

To add a new validation check:

1. **Add to scripts:** Create or update a script in `scripts/`
2. **Add to pre-commit:** Update `.pre-commit-config.yaml`
3. **Add to CI:** Update `.github/workflows/lint.yml`
4. **Add to Makefile:** Add a convenient make target
5. **Document here:** Update this file

## Related Documentation

- [Contributing Guide](../CONTRIBUTING.md)
- [Code Style Guide](./code-style.md)
- [Testing Guide](./testing.md)
