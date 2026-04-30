#!/bin/bash
# Guard against new API-layer direct writes to search indexes.
# T1 intentionally allows the audited legacy sites until they are migrated to
# search_outbox_events; this script fails new sites outside that baseline.

set -euo pipefail

SEARCH_WRITE_RE='(\.SearchProvider\.[[:alnum:]_]+Index\(\)\.(Index|Delete)\(|[[:alnum:]_]+\.[[:alnum:]_]+Index\(\)\.(Index|Delete)\(|saveProjectInAlgolia\(|indexAndValidateDocument\(|links\.(SaveDocumentRedirectDetails|DeleteDocumentRedirectDetails)\()'
failures=0

is_exempt_path() {
  case "$1" in
    *_test.go|tests/*|pkg/search/outbox/*|pkg/indexer/relay/*|internal/cmd/commands/canary/canary.go)
      return 0
      ;;
    internal/cmd/commands/*backfill*|internal/cmd/commands/*reindex*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

is_legacy_t1_baseline() {
  local file="$1"
  local text="$2"

  case "$file" in
    *)
      return 1
      ;;
  esac
}

count_matches() {
  local file="$1"
  local pattern="$2"
  local matches

  matches=$(git grep -nE "$pattern" -- "$file" || true)
  if [ -z "$matches" ]; then
    printf '0'
  else
    printf '%s\n' "$matches" | wc -l | tr -d ' '
  fi
}

require_max() {
  local file="$1"
  local pattern="$2"
  local max="$3"
  local label="$4"
  local count

  count=$(count_matches "$file" "$pattern")
  if [ "$count" -gt "$max" ]; then
    printf 'ERROR: %s has %s matches; expected at most %s until T1 migration removes the baseline.\n' "$label" "$count" "$max" >&2
    failures=1
  fi
}

while IFS=: read -r file line text; do
  [ -z "${file:-}" ] && continue

  if is_exempt_path "$file"; then
    continue
  fi

  if is_legacy_t1_baseline "$file" "$text"; then
    continue
  fi

  printf 'ERROR: unapproved direct search write at %s:%s: %s\n' "$file" "$line" "$text" >&2
  failures=1
done < <(git grep -nE "$SEARCH_WRITE_RE" -- 'internal/api/**/*.go' || true)

require_max internal/api/v2/drafts.go 'srv\.SearchProvider\.DraftIndex\(\)\.Index\(' 0 'draft direct index baseline'
require_max internal/api/v2/drafts.go 'srv\.SearchProvider\.DraftIndex\(\)\.Delete\(' 0 'draft direct delete baseline'
require_max internal/api/v2/reviews.go 'srv\.SearchProvider\.DocumentIndex\(\)\.Index\(' 0 'review direct document index baseline'
require_max internal/api/v2/reviews.go 'srv\.SearchProvider\.DraftIndex\(\)\.Delete\(' 0 'review direct draft delete baseline'
require_max internal/api/v2/documents.go 'srv\.SearchProvider\.DocumentIndex\(\)\.Index\(' 0 'document direct index baseline'
require_max internal/api/v2/approvals.go 'srv\.SearchProvider\.DocumentIndex\(\)\.Index\(' 0 'approval direct index baseline'
require_max internal/api/v2/approvals.go 'indexAndValidateDocument\(' 0 'approval helper-call baseline'
require_max internal/api/v2/projects.go 'saveProjectInAlgolia\(' 0 'project helper-call baseline'
require_max internal/api/v2/projects.go 'provider\.ProjectIndex\(\)\.Index\(' 0 'project direct index baseline'
require_max internal/api/v2/reviews.go 'links\.(SaveDocumentRedirectDetails|DeleteDocumentRedirectDetails)\(' 0 'review direct link-write baseline'

if [ "$failures" -ne 0 ]; then
  printf '\nDirect API-layer search writes must enqueue search_outbox_events instead.\n' >&2
  printf 'Allowed paths: tests, pkg/search/outbox, pkg/indexer/relay, canary, and backfill/reindex tooling.\n' >&2
  exit 1
fi

printf 'Search direct-write guard passed. No new unapproved direct search writes found.\n'
