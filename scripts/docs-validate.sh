#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
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

# docuchango needs Python 3.10 or newer (it imports typing.TypeGuard). Without
# a pin, `uv run` resolves whatever interpreter it finds first, which on macOS
# is often the 3.9 shipped inside Xcode -- and the failure surfaces as an
# ImportError from deep inside a dependency rather than as a version problem.
DOCS_PYTHON="${DOCS_PYTHON:-3.12}"

# docuchango only recognises documents under a root it understands. Given the
# repository root it scans zero files and reports success, which is worse than
# failing -- it looks like validation passed. Pointed at docs-internal it finds
# the ADR/RFC/memo tree. Coverage is still partial: it picks up 21 of the 187
# markdown files there, and nothing under docs/. Override to look elsewhere.
DOCS_ROOT="${DOCS_ROOT:-docs-internal}"

run_docuchango() {
    local docuchango_cmd="${DOCUCHANGO_CMD:-docuchango}"
    local cmd="$1"
    shift

    # --with installs docuchango into an ephemeral environment on the pinned
    # interpreter. Without it, `uv run docuchango` finds whatever console
    # script is already on PATH and runs it under that script's own shebang,
    # so --python has no effect -- which is how a 3.9 install kept being used
    # no matter what was requested.
    local -a uv_run=(uv run --python "${DOCS_PYTHON}" --with "${DOCUCHANGO_PKG:-docuchango}")

    case "${cmd}" in
        validate)
            info "Validating documentation in ${PROJECT_ROOT}"
            "${uv_run[@]}" "${docuchango_cmd}" validate --repo-root "${DOCS_ROOT}" "$@"
            ;;
        migrate)
            info "Migrating documentation metadata in ${PROJECT_ROOT}"
            "${uv_run[@]}" "${docuchango_cmd}" migrate --repo-root "${DOCS_ROOT}" "$@"
            ;;
        fix)
            info "Running documentation validation fixes"
            "${uv_run[@]}" "${docuchango_cmd}" validate --repo-root "${DOCS_ROOT}" fix "$@"
            ;;
        bootstrap)
            info "Running docuchango bootstrap flow"
            "${uv_run[@]}" "${docuchango_cmd}" bootstrap "$@"
            ;;
        *)
            "${uv_run[@]}" "${docuchango_cmd}" "${cmd}" --repo-root "${DOCS_ROOT}" "$@"
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
  DOCS_PYTHON    (optional): Python version for uv to use (default 3.12;
                             docuchango requires 3.10+)
  DOCUCHANGO_PKG (optional): package uv installs (default docuchango); set to
                             a path or VCS URL to test an unreleased version
  DOCS_ROOT      (optional): root docuchango scans (default docs-internal; the
                             repository root matches nothing and scans 0 files)
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
