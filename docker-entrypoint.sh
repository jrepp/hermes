#!/bin/sh
# Docker entrypoint script for Hermes
# Ensures shared directories exist with proper ownership for the hermes user
# This must run as root to fix volume mount ownership issues

set -e

# Fix permissions on shared volume mount (created by Docker with root ownership)
if [ -d /app/shared ]; then
    echo "Fixing permissions on /app/shared..."
    chown -R hermes:hermes /app/shared
    chmod 755 /app/shared
fi

# Fix permissions on workspace_data volume mount (if it exists and is root-owned)
if [ -d /app/workspace_data ] && [ "$(stat -c '%U' /app/workspace_data 2>/dev/null || echo hermes)" = "root" ]; then
    echo "Fixing permissions on /app/workspace_data..."
    chown -R hermes:hermes /app/workspace_data
    chmod 755 /app/workspace_data
fi

echo "Starting Hermes as user 'hermes'..."

# If first argument doesn't start with /, prepend /app/hermes
# This handles docker-compose command: ["server", ...] vs CMD ["/app/hermes", "server"]
if [ $# -gt 0 ] && [ "${1#/}" = "$1" ]; then
    set -- /app/hermes "$@"
fi

# Build the command string properly
CMD_STR=""
for arg in "$@"; do
    # Escape single quotes in arguments
    escaped=$(echo "$arg" | sed "s/'/'\\\\''/g")
    CMD_STR="$CMD_STR '$escaped'"
done

# Execute the command as hermes user using su with proper quoting
exec su hermes -c "exec $CMD_STR"
