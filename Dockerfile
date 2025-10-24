# Multi-stage build for Hermes backend
# NOTE: Requires web/dist to be pre-built on host (run `make web/build` first)
# This avoids building web assets twice and running out of memory in Docker

FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code (excluding items in .dockerignore)
COPY . .

# Verify web/dist exists (must be built on host before docker build)
RUN test -d web/dist || (echo "ERROR: web/dist not found! Run 'make web/build' first" && exit 1)

# Build the binary (with embedded web assets from host)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hermes ./cmd/hermes

# Final stage - minimal runtime image
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/hermes /app/hermes

# Copy configs (optional, can be mounted as volume)
COPY --from=builder /build/configs /app/configs

# Copy entrypoint script
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Create non-root user and directories
# Note: Directories created here will have correct ownership, but Docker volumes
# mounted at runtime will override ownership. The entrypoint script fixes this.
RUN adduser -D -u 1000 hermes && \
    chown -R hermes:hermes /app && \
    mkdir -p /app/workspace_data /app/shared && \
    chown -R hermes:hermes /app/workspace_data /app/shared

# Expose port
EXPOSE 8000

# Use entrypoint to fix volume permissions, then run as hermes user
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/app/hermes", "server"]
