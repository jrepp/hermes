# Pre-Commit Hooks

This document describes the pre-commit hooks configured for the Hermes project to ensure code quality, consistency, and maintainability.

## Table of Contents

- [Installation](#installation)
- [Hook Categories](#hook-categories)
- [Commit Message Validation](#commit-message-validation)
- [Go Code Quality Checks](#go-code-quality-checks)
- [Running Hooks Manually](#running-hooks-manually)
- [Skipping Hooks](#skipping-hooks)
- [Troubleshooting](#troubleshooting)

## Installation

### Prerequisites

Install pre-commit:

```bash
# macOS
brew install pre-commit

# Linux
pip install pre-commit

# Or using pipx
pipx install pre-commit
```

### Setup

Install the pre-commit hooks in your local repository:

```bash
# Install both pre-commit and commit-msg hooks
pre-commit install
pre-commit install --hook-type commit-msg
```

Verify installation:

```bash
pre-commit --version
```

## Hook Categories

### 1. Commit Message Validation

**Hook**: `commit-msg-validation`
**Stage**: `commit-msg`
**Script**: `scripts/check-commit-msg.sh`

Validates commit messages for quality and conventional commit format:

- **Format**: `type(scope): description`
- **Valid types**: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`
- **Length**: Subject line max 100 characters (recommended 72)
- **Structure**: Blank line between subject and body
- **Quality**: No WIP/TODO markers, no tool branding

**Examples**:

```bash
# ✓ Good commits
feat(api): add user authentication endpoint
fix(database): resolve connection pool timeout
docs(readme): update installation instructions
refactor(handlers): simplify draft creation logic

# ✗ Bad commits
Added some stuff
WIP: working on feature
Fixed bug.  # (ends with period)
updated files  # (lowercase, vague)
```

### 2. Go Code Quality Checks

#### Format & Style

- **gofmt**: Format Go code with standard formatting
- **goimports**: Organize imports and group local packages
- **golangci-lint**: Comprehensive linting (60+ linters)

#### Static Analysis

- **go vet**: Built-in Go static analysis
- **staticcheck**: Advanced static analysis
- **errcheck**: Ensure errors are checked
- **gosec**: Security vulnerability scanning

#### Compilation

- **go build**: Verify code compiles
- **go test compile**: Verify test code compiles
- **go mod tidy**: Ensure dependencies are tidy

#### Complexity Analysis

- **go-complexity**: Check cyclomatic and cognitive complexity
  - **Stage**: `pre-push` (only runs on push, not every commit)
  - **Thresholds**:
    - Cyclomatic complexity: ≤15
    - Cognitive complexity: ≤20

### 3. General File Quality

- **trailing-whitespace**: Remove trailing whitespace
- **end-of-file-fixer**: Ensure files end with newline
- **check-yaml**: Validate YAML syntax
- **check-json**: Validate JSON syntax
- **check-added-large-files**: Prevent files >1MB
- **check-merge-conflict**: Check for merge conflict markers
- **check-case-conflict**: Check for case conflicts in filenames
- **mixed-line-ending**: Ensure consistent line endings

## Running Hooks Manually

### Run all hooks on all files:

```bash
pre-commit run --all-files
```

### Run specific hook:

```bash
pre-commit run gofmt --all-files
pre-commit run golangci-lint --all-files
pre-commit run commit-msg-validation --hook-stage commit-msg --commit-msg-filename .git/COMMIT_EDITMSG
```

### Run complexity checks:

```bash
./scripts/check-complexity.sh
```

### Run on staged files only:

```bash
pre-commit run
```

### Run pre-push hooks:

```bash
pre-commit run --hook-stage pre-push --all-files
```

## Skipping Hooks

### Skip all hooks for a commit:

```bash
git commit --no-verify -m "message"
# or
SKIP=1 git commit -m "message"
```

### Skip specific hooks:

```bash
SKIP=gofmt,golangci-lint git commit -m "message"
```

### Skip only commit message validation:

```bash
git commit --no-verify -m "message"
```

**⚠️ Warning**: Only skip hooks when absolutely necessary (e.g., emergency hotfix). Skipped checks should be addressed in a follow-up commit.

## Configuration Files

### Pre-commit Configuration

**File**: `.pre-commit-config.yaml`

Modify this file to:
- Add/remove hooks
- Change hook execution order
- Adjust hook arguments
- Set file type filters

### Golangci-lint Configuration

**File**: `.golangci.yml`

Configure linters:
- Enable/disable specific linters
- Adjust complexity thresholds
- Add exclusion rules
- Set timeout values

### Complexity Check Script

**File**: `scripts/check-complexity.sh`

Customize complexity thresholds:
- Cyclomatic complexity: Line 28 (`-over 15`)
- Cognitive complexity: Line 44 (`-over 20`)

### Commit Message Validation Script

**File**: `scripts/check-commit-msg.sh`

Customize validation rules:
- Subject length limits
- Conventional commit patterns
- Warning vs error conditions

## Troubleshooting

### Hooks not running

```bash
# Reinstall hooks
pre-commit uninstall
pre-commit install
pre-commit install --hook-type commit-msg
```

### Tool not found errors

```bash
# Install missing Go tools
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
go install github.com/uudashr/gocognit/cmd/gocognit@latest
```

### Slow hook execution

- Complexity checks only run on `pre-push` stage
- Use `SKIP` environment variable for specific hooks during development
- Run `golangci-lint cache clean` to clear lint cache

### golangci-lint timeout

```bash
# Increase timeout in .golangci.yml
run:
  timeout: 10m  # Default is 5m
```

### Pre-commit version issues

```bash
# Update pre-commit
pip install --upgrade pre-commit
# or
brew upgrade pre-commit

# Update hook repositories
pre-commit autoupdate
```

## Best Practices

1. **Run hooks before pushing**: `pre-commit run --all-files`
2. **Address warnings**: Don't ignore linter warnings repeatedly
3. **Keep tools updated**: Run `pre-commit autoupdate` periodically
4. **Review skipped checks**: If you skip hooks, document why in the commit message
5. **Fix complexity issues**: Refactor functions flagged by complexity checks
6. **Write clear commit messages**: Follow conventional commit format

## CI/CD Integration

These hooks are also run in CI/CD pipelines to ensure code quality. Local hook failures indicate the CI build will likely fail.

### GitHub Actions

The same checks run in `.github/workflows/ci.yml`:

```yaml
- name: Run pre-commit
  run: pre-commit run --all-files
```

## Additional Resources

- [Pre-commit documentation](https://pre-commit.com/)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [golangci-lint linters](https://golangci-lint.run/usage/linters/)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Cognitive Complexity](https://www.sonarsource.com/resources/cognitive-complexity/)
