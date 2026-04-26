# Parallel CI Infrastructure

This directory contains the new parallel CI infrastructure for Hermes, inspired by the data-access project workflows. The new system is designed to provide faster feedback, better resource utilization, and more flexible testing.

## Overview

The parallel CI infrastructure consists of two main workflows:

1. **`parallel-ci.yml`** - Main CI pipeline with parallel test execution
2. **`pr-status-check.yml`** - Semaphore workflow that acts as the single required status check

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Development Flow                        │
└─────────────────────────────────────────────────────────────┘

  Feature Branch Push
         ↓
    Create PR
         ↓
  ┌──────────────────┴──────────────────┐
  │                                     │
  Parallel CI                 PR Status Check (Semaphore)
  (comprehensive)             (required for merge)
    ✓ Detect changes            ✓ Waits for workflows
    ✓ Parallel linting          ✓ Polls for completion
    ✓ Go tests (matrix)         ✓ Single status check
    ✓ Web tests
    ✓ Python tests
    ✓ Integration tests
    ✓ E2E tests
    ✓ Coverage reports
  (20-30 min)                 (waits for parallel-ci)
         │                                     │
         └──────────────────┬──────────────────┘
                            ↓
                    Both must pass
                            ↓
                     Merge to Main
```

## Workflows

### 1. Parallel CI (`parallel-ci.yml`)

**Triggers**: Push to `main`, Pull Requests to `main`

The main CI pipeline with intelligent test selection and parallel execution.

**Key Features:**

- **Selective Execution**: Only runs tests affected by changed files
- **Parallel Linting**: Lints Go code in parallel by category (format, vet, golangci-lint, complexity)
- **Matrix Testing**: Tests Go packages in parallel
- **Component Isolation**: Separate jobs for Go, Web, Python, Integration, and E2E tests
- **Smart Caching**: Caches Go modules, Node modules, and build artifacts
- **Coverage Reports**: Aggregates coverage from all test suites

**Jobs:**

1. **detect-changes** - Analyzes changed files to determine what tests to run
2. **lint-go** - Parallel linting in 4 categories
3. **build-go** - Builds all Go binaries
4. **test-go** - Tests Go packages in parallel (9 packages)
5. **test-web** - Builds and tests web frontend
6. **test-python** - Tests Python client
7. **test-integration** - Integration tests in 3 suites (api, migration, e2e)
8. **test-e2e-playwright** - E2E Playwright tests
9. **coverage-summary** - Aggregates coverage reports
10. **parallel-ci-status** - Final status check (accepts success/skipped)

**Duration**: ~20-30 minutes (many jobs run in parallel)

**Selective Execution Example:**

- Change only `.go` files → Only Go tests run
- Change only `web/` files → Only web tests run
- Change `testing/` → Integration and E2E tests run
- No changes detected → All tests run (safety fallback)

### 2. PR Status Check (`pr-status-check.yml`)

**Triggers**: Pull Requests to `main`

A semaphore-based workflow that waits for other workflows to complete. This is the **single required status check** that gates PR merges.

**Why a Semaphore?**

GitHub doesn't support dynamic required status checks. This workflow:
1. Detects which workflows are required based on changed files
2. Polls the GitHub API for workflow completion
3. Reports success only when all required workflows pass
4. Provides a single, predictable status check for branch protection

**Key Features:**

- **Change Detection**: Determines required workflows based on file changes
- **Intelligent Waiting**: Only waits for workflows that should run
- **Fast Path for Docs**: Docs-only changes skip CI entirely
- **Startup Grace Period**: 3-minute grace period for workflows to start
- **Detailed Reporting**: Shows status of each workflow in GitHub Actions summary

**Required Workflows:**

- **Parallel CI**: Required for code changes
- **Lint**: Required for code changes
- **E2E Python Tests**: Required for testing infrastructure changes

**Duration**: Waits up to 90 minutes, but typically completes in 20-30 minutes

## Key Improvements Over Legacy CI

### 1. Parallel Execution

**Legacy**: Sequential test execution (lint → test → build)
**New**: Parallel execution across multiple jobs

Example timing comparison:
- **Legacy**: 40 minutes (sequential)
- **New**: 20 minutes (parallel)

### 2. Selective Execution

**Legacy**: Always runs all tests
**New**: Only runs tests affected by changes

Example for a web-only change:
- **Legacy**: Runs Go tests + Web tests (40 minutes)
- **New**: Only runs Web tests (5 minutes)

### 3. Better Resource Utilization

**Legacy**: Single job uses 1 runner
**New**: Up to 15 jobs use 15 runners simultaneously

### 4. Faster Feedback

**Legacy**: Wait for all tests before any failure
**New**: Get feedback as soon as individual test suites complete

### 5. Flexible Status Checks

**Legacy**: Multiple required status checks (hard to maintain)
**New**: Single required status check (pr-status-check)

## Caching Strategy

All workflows use aggressive caching for faster builds:

### Go Dependencies
- **Cache**: `~/go/pkg/mod` and `~/.cache/go-build`
- **Key**: `${{ runner.os }}-go-${{ hashFiles('**/go.sum', '**/go.mod') }}`
- **Savings**: ~2-3 minutes per job

### Node Modules
- **Cache**: Automatic via `actions/setup-node@v4` with `cache: 'yarn'`
- **Savings**: ~1-2 minutes

### golangci-lint Binary
- **Cache**: `~/go/bin/golangci-lint`
- **Key**: `${{ runner.os }}-golangci-lint-v1.55.2`
- **Savings**: ~30 seconds

### Build Artifacts
- **Go Binaries**: Shared between jobs via artifacts (7 days retention)
- **Coverage Reports**: Collected and aggregated (7 days retention)

## Branch Protection Configuration

To use this CI infrastructure, configure branch protection for `main`:

```yaml
Required status checks:
  - PR Status Check (Semaphore)  # Single required check!

Optional status checks (for visibility):
  - Parallel CI Status

Require branches to be up to date: Yes
```

**Important**: Only `PR Status Check (Semaphore)` needs to be required. The semaphore will ensure all necessary workflows pass.

## Local Development

Run the same checks locally before pushing:

```bash
# Full CI pipeline locally
make lint        # Lint all code
make build       # Build all packages
make test        # Run all Go tests
make web/build   # Build web
make web/test    # Test web

# Individual checks
make fmt                    # Format Go code
make complexity            # Check code complexity
make test-integration      # Run integration tests
make test-migration        # Run migration tests

# Build all binaries
make build-binaries        # Creates build/bin/ with all binaries
```

## Debugging Failed Workflows

### 1. Check workflow logs in GitHub Actions UI

Navigate to: Actions → Failed workflow → Job → Step

### 2. Download artifacts

```bash
gh run list --workflow=parallel-ci.yml
gh run download <run-id>
```

### 3. Re-run failed jobs

```bash
gh run rerun <run-id> --failed
```

### 4. Run locally with act

```bash
# Install act: https://github.com/nektos/act
act -j lint-go           # Run lint job
act -j test-go           # Run test job
act pull_request         # Simulate PR event
```

### 5. Check PR Status Check semaphore

If the PR Status Check is stuck:
- Check if required workflows started
- Look for workflow path filter mismatches
- Verify GitHub Actions is not experiencing outages

## Cost Optimization

Estimated GitHub Actions minutes usage:

**Per PR** (full run):
- Parallel CI: ~20 minutes × 15 jobs = 300 runner-minutes
- PR Status Check: ~5 minutes
- **Total**: ~305 runner-minutes per PR

**Monthly estimate** (for 100 PRs/month):
- PRs: 100 × 305 minutes = 30,500 runner-minutes
- **Total**: ~30,500 runner-minutes/month

GitHub Free: 2,000 minutes/month
GitHub Team: 3,000 minutes/month
GitHub Enterprise: 50,000 minutes/month

**Recommendation**: GitHub Team plan with self-hosted runners for private repos

## Selective Execution Details

The `detect-changes` job analyzes changed files and sets flags for what to run:

| Changed Files | Go Tests | Web Tests | Python Tests | Integration | E2E |
|---------------|----------|-----------|--------------|-------------|-----|
| `*.go` | ✅ | ❌ | ❌ | ✅ | ❌ |
| `web/**` | ❌ | ✅ | ❌ | ❌ | ❌ |
| `python-client/**` | ❌ | ❌ | ✅ | ❌ | ❌ |
| `testing/**` | ❌ | ❌ | ❌ | ✅ | ✅ |
| `internal/**` | ✅ | ❌ | ❌ | ✅ | ❌ |
| No changes detected | ✅ | ✅ | ✅ | ✅ | ✅ |

Jobs with skipped tests will complete with `skipped` status, which is accepted by the `parallel-ci-status` job.

## Troubleshooting

### Issue: PR Status Check always fails

**Cause**: Required workflows not starting due to path filters

**Solution**: Check `detect-workflows` job output to see what workflows are expected. Verify path filters in each workflow.

### Issue: Coverage reports missing

**Cause**: Tests didn't generate coverage files

**Solution**: Check individual test job logs. Ensure tests are actually running and generating `coverage.out` files.

### Issue: Workflows taking too long

**Cause**: Cache misses or network issues

**Solution**:
- Check cache hit rates in workflow logs
- Verify dependencies haven't changed significantly
- Consider increasing timeout if tests are legitimately slow

## Future Enhancements

### 1. Test Result Caching
Cache test results based on file hashes to skip unchanged tests entirely.

### 2. Differential Testing
Only test packages that depend on changed code.

### 3. Self-Hosted Runners
Deploy self-hosted runners for:
- Faster builds (dedicated hardware)
- Cost savings (no per-minute billing)
- Access to internal services

### 4. Advanced Coverage Reporting
- Upload to Codecov or Coveralls
- Fail PR if coverage drops
- Show coverage diff in PR comments

### 5. Performance Benchmarks
- Track performance metrics over time
- Fail PR if performance regresses
- Compare benchmark results in PR comments

### 6. Docker Image Caching
- Cache built Docker images
- Reuse images across workflow runs
- Faster integration test setup

## References

### Inspiration
This CI infrastructure was inspired by the patterns in:
- `~/dev/data-access/.github/workflows/`

### GitHub Actions Documentation
- [Workflow syntax](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions)
- [Caching dependencies](https://docs.github.com/en/actions/using-workflows/caching-dependencies-to-speed-up-workflows)
- [Using a matrix](https://docs.github.com/en/actions/using-jobs/using-a-matrix-for-your-jobs)

### Related Documentation
- [Pre-commit Hooks](../../docs/PRE_COMMIT_HOOKS.md)
- [Deployment Guide](../../scripts/deployment/README.md)

## Support

For issues or questions about the CI infrastructure:
1. Check this README
2. Review workflow logs in GitHub Actions
3. Open an issue with label `ci-infrastructure`
4. Contact the platform team

## Changelog

### 2024-11-18 - Initial Implementation
- Created parallel-ci.yml with selective execution
- Created pr-status-check.yml as semaphore
- Added comprehensive documentation
- Removed legacy ci.yml and lint.yml workflows
