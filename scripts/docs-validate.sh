#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() {
    echo -e "${BLUE}==>${NC} $*"
}

success() {
    echo -e "${GREEN}==>${NC} $*"
}

error() {
    echo -e "${RED}Error:${NC} $*" >&2
}

check_uv() {
    if ! command -v uv >/dev/null 2>&1; then
        error "uv is not installed"
        echo "Install with: curl -LsSf https://astral.sh/uv/install.sh | sh"
        exit 1
    fi
}

run_docuchango() {
    local docuchango_cmd="${DOCUCHANGO_CMD:-docuchango}"
    local cmd="$1"
    shift

    case "${cmd}" in
        validate)
            info "Validating documentation in ${PROJECT_ROOT}"
            uv run "${docuchango_cmd}" validate --repo-root . "$@"
            ;;
        migrate)
            info "Migrating documentation metadata in ${PROJECT_ROOT}"
            uv run "${docuchango_cmd}" migrate --repo-root . "$@"
            ;;
        fix)
            info "Running documentation validation fixes"
            uv run "${docuchango_cmd}" validate --repo-root . fix "$@"
            ;;
        bootstrap)
            info "Running docuchango bootstrap flow"
            uv run "${docuchango_cmd}" bootstrap "$@"
            ;;
        *)
            uv run "${docuchango_cmd}" "${cmd}" --repo-root . "$@"
            ;;
    esac
}

usage() {
    cat << 'EOF'
Documentation validation wrapper for docuchango.

Usage: $(basename "$0") [command]

Commands:
  validate        Validate all documentation (default)
  validate --skip-build  Skip heavy build step (if supported)
  --quick         Shortcut for: validate --skip-build
  fix             Auto-fix issues (alias for validate fix)
  migrate         Migrate documentation metadata schema
  bootstrap       Run docuchango bootstrap workflow

Environment:
  DOCUCHANGO_CMD (optional): set a custom docuchango executable path
EOF
}

main() {
    check_uv

    local cmd="${1:-validate}"
    case "${cmd}" in
        -h|--help)
            usage
            exit 0
            ;;
        --quick)
            run_docuchango validate --skip-build
            ;;
        fix)
            shift || true
            run_docuchango validate fix "$@"
            ;;
        validate|migrate|bootstrap)
            shift || true
            run_docuchango "${cmd}" "$@"
            ;;
        *)
            run_docuchango "${cmd}" "${@:2}"
            ;;
    esac

    success "Documentation operation complete"
}

cd "${PROJECT_ROOT}"
main "$@"
