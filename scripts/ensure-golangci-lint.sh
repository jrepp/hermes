#!/usr/bin/env bash
# Verify the installed golangci-lint matches the major version that
# .golangci.yml is written for, and that it was built with a Go toolchain new
# enough for go.mod. A mismatch makes golangci-lint fail at config-load time
# with an error that does not name the real problem, so we check up front.
set -euo pipefail

REQUIRED_VERSION="${GOLANGCI_LINT_VERSION:?GOLANGCI_LINT_VERSION must be set}"
REQUIRED_MAJOR="${REQUIRED_VERSION%%.*}"   # e.g. "v2"

install_hint() {
  cat >&2 <<HINT

To install the expected version:
  make ci-install-tools

Or directly:
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \\
    | sh -s -- -b "\$(go env GOPATH)/bin" ${REQUIRED_VERSION}
HINT
}

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo "error: golangci-lint is not installed." >&2
  install_hint
  exit 1
fi

# `golangci-lint version` writes to stdout on v2 and stderr on some v1 builds.
version_output="$(golangci-lint version 2>&1 || true)"
installed_version="$(printf '%s' "$version_output" \
  | grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+' \
  | head -1)"

if [ -z "$installed_version" ]; then
  echo "error: could not determine golangci-lint version from: $version_output" >&2
  install_hint
  exit 1
fi

installed_major="v${installed_version#v}"
installed_major="${installed_major%%.*}"

if [ "$installed_major" != "$REQUIRED_MAJOR" ]; then
  cat >&2 <<MSG
error: golangci-lint major version mismatch.
  installed: ${installed_version}
  required:  ${REQUIRED_VERSION} (.golangci.yml declares 'version: "${REQUIRED_MAJOR#v}"')

A v1 binary cannot read a v2 config; it fails with a misleading
"can't load config" error.
MSG
  install_hint
  exit 1
fi

# golangci-lint refuses to load a config whose Go *language* version is newer
# than the one it was built with. It compares major.minor only, so a patch
# difference (built with 1.25.3, go.mod on 1.25.5) is fine and must not be
# reported as a mismatch.
go_mod_version="$(awk '/^go /{print $2; exit}' go.mod)"
built_with="$(printf '%s' "$version_output" \
  | grep -oE 'go[0-9]+\.[0-9]+(\.[0-9]+)?' \
  | head -1 | sed 's/^go//')"

lang_version() { printf '%s' "$1" | cut -d. -f1,2; }

if [ -n "$built_with" ] && [ -n "$go_mod_version" ]; then
  built_lang="$(lang_version "$built_with")"
  required_lang="$(lang_version "$go_mod_version")"
  lowest="$(printf '%s\n%s\n' "$built_lang" "$required_lang" | sort -V | head -1)"
  if [ "$lowest" = "$built_lang" ] && [ "$built_lang" != "$required_lang" ]; then
    cat >&2 <<MSG
error: golangci-lint was built with Go ${built_with} (language ${built_lang}),
but go.mod targets ${go_mod_version} (language ${required_lang}).
Install a build made with Go ${required_lang} or newer.
MSG
    install_hint
    exit 1
  fi
fi

echo "✓ golangci-lint ${installed_version} (built with go${built_with:-unknown})"
