# Validation Tools - Quick Start Guide

## First Time Setup

```bash
# Install pre-commit hooks (one-time)
make install-hooks

# Install complexity tools (one-time)
make complexity-install
```

## Before Every Commit

```bash
# Format code
make fmt

# Run all validation
make validate
```

## Commands Reference

| Command | Description |
|---------|-------------|
| `make help` | Show all available commands |
| `make fmt` | Auto-format all Go code |
| `make lint` | Run syntax validation |
| `make complexity` | Check code complexity |
| `make validate` | Run full validation (lint + complexity) |
| `make build` | Build all packages |
| `make test` | Run all tests |
| `make pre-commit` | Run pre-commit checks manually |

## What Runs Where

### On Every Commit (via pre-commit hooks)
- ✅ Code formatting check
- ✅ Static analysis (go vet)
- ✅ Build validation
- ✅ Test compilation
- ✅ Module tidiness
- ✅ Whitespace/EOF cleanup

### On Push (via pre-commit hooks)
- ✅ Complexity analysis

### On Pull Requests (via GitHub Actions)
- ✅ All of the above
- ✅ golangci-lint
- ✅ Full complexity reports

## Bypassing Checks (Emergency Only)

```bash
# Skip all pre-commit hooks (NOT RECOMMENDED)
git commit --no-verify -m "Emergency fix"

# Skip specific hook
SKIP=go-complexity git commit -m "Fix"
```

## Troubleshooting

```bash
# Tool not found?
make complexity-install

# Formatting issues?
make fmt

# Build issues?
make clean && make build

# Pre-commit not working?
make install-hooks
```

## Documentation

See [docs/development/validation-tools.md](docs/development/validation-tools.md) for complete documentation.
