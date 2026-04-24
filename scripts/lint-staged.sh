#!/bin/bash
# Run golangci-lint with strict config, but only flag issues introduced in the
# current commit (--new-from-rev=HEAD). Pre-existing issues in untouched code
# are not blocked. This supports the incremental burn-down workflow.
set -euo pipefail

command -v golangci-lint >/dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

exec golangci-lint run --timeout=5m --new-from-rev=HEAD
