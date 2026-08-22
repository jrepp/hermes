#!/usr/bin/env bash
#
# shellcheck disable=SC2029
#   The remote command strings interpolate local variables on purpose: the
#   install prefix, config path, and unit name are configuration from this
#   side, and expanding them here is what makes the command sent to the host
#   explicit and printable under --dry-run.
#
# Build Hermes and deploy it to a single host over ssh.
#
# Deliberately small and boring: build locally, copy two binaries, migrate,
# restart. It does not install packages, create databases, write nginx configs,
# or issue certificates -- those are one-time steps that belong in
# docs-internal/guides/deploy/multi-domain.md, where they can be read before
# they are run.
#
# Usage:
#   scripts/deployment/deploy-remote.sh user@host [--dry-run] [--skip-build]
#
# Environment:
#   HERMES_REMOTE_DIR     install prefix on the host        (default /opt/hermes)
#   HERMES_REMOTE_CONFIG  config path on the host           (default /etc/hermes/config.hcl)
#   HERMES_REMOTE_SERVICE systemd unit to restart           (default hermes)
#   HERMES_REMOTE_DSN     libpq DSN used for the migration; read on the host,
#                         so prefer setting it in the host's environment file

set -euo pipefail

readonly REMOTE_DIR="${HERMES_REMOTE_DIR:-/opt/hermes}"
readonly REMOTE_CONFIG="${HERMES_REMOTE_CONFIG:-/etc/hermes/config.hcl}"
readonly REMOTE_SERVICE="${HERMES_REMOTE_SERVICE:-hermes}"

TARGET=""
DRY_RUN=false
SKIP_BUILD=false

usage() {
  sed -n '3,20p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

for arg in "$@"; do
  case "$arg" in
    --dry-run)    DRY_RUN=true ;;
    --skip-build) SKIP_BUILD=true ;;
    -h|--help)    usage 0 ;;
    -*)           echo "unknown flag: $arg" >&2; usage 1 ;;
    *)
      if [ -n "$TARGET" ]; then
        echo "more than one target given: $TARGET and $arg" >&2
        exit 1
      fi
      TARGET="$arg"
      ;;
  esac
done

if [ -z "$TARGET" ]; then
  echo "no target given" >&2
  usage 1
fi

say() { printf '==> %s\n' "$*"; }

# run executes a command on the host, or prints it under --dry-run.
run() {
  if [ "$DRY_RUN" = true ]; then
    printf '    [dry-run] ssh %s %s\n' "$TARGET" "$*"
    return 0
  fi
  ssh "$TARGET" "$@"
}

copy() {
  if [ "$DRY_RUN" = true ]; then
    printf '    [dry-run] scp %s %s:%s\n' "$1" "$TARGET" "$2"
    return 0
  fi
  scp -q "$1" "$TARGET:$2"
}

# ---------------------------------------------------------------------------
# Preflight. Everything checked here fails cheaply; everything after it changes
# the host.
# ---------------------------------------------------------------------------
say "checking $TARGET"
if [ "$DRY_RUN" = false ]; then
  ssh -o BatchMode=yes -o ConnectTimeout=10 "$TARGET" true ||
    { echo "cannot reach $TARGET over ssh" >&2; exit 1; }

  ssh "$TARGET" "test -f '$REMOTE_CONFIG'" ||
    { echo "$TARGET has no config at $REMOTE_CONFIG; see the deployment guide" >&2; exit 1; }

  ssh "$TARGET" "systemctl cat '$REMOTE_SERVICE' >/dev/null 2>&1" ||
    { echo "$TARGET has no systemd unit named $REMOTE_SERVICE" >&2; exit 1; }
fi

# ---------------------------------------------------------------------------
# Build. The frontend first: build/linux only writes a placeholder index.html
# when none exists, so building in the other order ships a stub page.
# ---------------------------------------------------------------------------
if [ "$SKIP_BUILD" = true ]; then
  say "skipping build"
  for binary in hermes hermes-migrate; do
    test -x "build/bin/$binary" ||
      { echo "build/bin/$binary is missing; drop --skip-build" >&2; exit 1; }
  done
else
  say "building frontend"
  make web/build

  say "building linux binaries"
  make build/linux
fi

# ---------------------------------------------------------------------------
# Ship. Binaries land beside the running ones and are moved into place after
# the migration succeeds, so a failed migration leaves the old build running.
# ---------------------------------------------------------------------------
say "uploading binaries"
run "mkdir -p '$REMOTE_DIR/bin'"
copy build/bin/hermes "$REMOTE_DIR/bin/hermes.new"
copy build/bin/hermes-migrate "$REMOTE_DIR/bin/hermes-migrate.new"
run "chmod +x '$REMOTE_DIR/bin/hermes.new' '$REMOTE_DIR/bin/hermes-migrate.new'"

say "migrating every site in $REMOTE_CONFIG"
if [ -n "${HERMES_REMOTE_DSN:-}" ]; then
  run "'$REMOTE_DIR/bin/hermes-migrate.new' -dsn='$HERMES_REMOTE_DSN' -config='$REMOTE_CONFIG'"
else
  # No DSN passed from here: read it from the host's own environment file so
  # the database password never appears in a local shell history.
  run "set -a; . /etc/hermes/secrets.env; set +a; \
       test -n \"\${HERMES_MIGRATE_DSN:-}\" || { \
         echo 'set HERMES_MIGRATE_DSN in /etc/hermes/secrets.env, or pass HERMES_REMOTE_DSN' >&2; \
         exit 1; }; \
       '$REMOTE_DIR/bin/hermes-migrate.new' -dsn=\"\$HERMES_MIGRATE_DSN\" -config='$REMOTE_CONFIG'"
fi

say "installing binaries"
run "mv '$REMOTE_DIR/bin/hermes.new' '$REMOTE_DIR/bin/hermes' && \
     mv '$REMOTE_DIR/bin/hermes-migrate.new' '$REMOTE_DIR/bin/hermes-migrate'"

say "restarting $REMOTE_SERVICE"
run "sudo systemctl restart '$REMOTE_SERVICE'"

# ---------------------------------------------------------------------------
# Verify. /health is the one path that answers without a Host header, which is
# what makes it usable from the host itself.
# ---------------------------------------------------------------------------
say "waiting for $REMOTE_SERVICE"
if [ "$DRY_RUN" = false ]; then
  for _ in $(seq 1 30); do
    if ssh "$TARGET" "curl -sf -o /dev/null http://127.0.0.1:8000/health"; then
      say "healthy"
      ssh "$TARGET" "systemctl --no-pager --lines=5 status '$REMOTE_SERVICE'" || true
      exit 0
    fi
    sleep 2
  done

  echo "service did not become healthy; recent logs:" >&2
  ssh "$TARGET" "journalctl -u '$REMOTE_SERVICE' --no-pager --lines=40" >&2
  exit 1
fi

say "dry run complete; nothing was changed"
