#!/bin/bash
# RFC-088: Docker Deployment Script
# Automated deployment of Hermes with semantic search using Docker Compose

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_FILE="${COMPOSE_FILE:-./config-production/docker-compose.production.yml}"
ENV_FILE="${ENV_FILE:-.env}"
RUN_MIGRATIONS="${RUN_MIGRATIONS:-true}"
RUN_VALIDATION="${RUN_VALIDATION:-true}"
INDEXER_REPLICAS="${INDEXER_REPLICAS:-2}"

echo "================================================"
echo "RFC-088: Docker Deployment"
echo "================================================"
echo ""

# Helper functions
info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
    exit 1
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Step 1: Check prerequisites
info "Step 1: Checking prerequisites..."

if ! command -v docker &> /dev/null; then
    error "Docker not found. Please install Docker."
fi
success "Docker installed"

if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    error "Docker Compose not found. Please install Docker Compose."
fi
success "Docker Compose installed"

# Step 2: Check configuration files
info "Step 2: Checking configuration files..."

if [ ! -f "$COMPOSE_FILE" ]; then
    error "Compose file not found: $COMPOSE_FILE"
fi
success "Compose file found: $COMPOSE_FILE"

if [ ! -f "$ENV_FILE" ]; then
    warn "Environment file not found: $ENV_FILE"
    warn "Using .env.template to create .env file..."

    if [ -f "./config-production/.env.template" ]; then
        cp ./config-production/.env.template "$ENV_FILE"
        warn "Please edit $ENV_FILE and set required values (OPENAI_API_KEY, etc.)"
        read -p "Press Enter after editing $ENV_FILE to continue, or Ctrl+C to cancel..."
    else
        error "No .env.template found. Please create $ENV_FILE manually."
    fi
fi
success "Environment file found: $ENV_FILE"

# Step 3: Load environment variables
info "Step 3: Loading environment variables..."
set -a
source "$ENV_FILE"
set +a

# Validate required variables
REQUIRED_VARS=("OPENAI_API_KEY" "POSTGRES_PASSWORD" "MEILISEARCH_KEY")
MISSING_VARS=()

for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        MISSING_VARS+=("$var")
    fi
done

if [ ${#MISSING_VARS[@]} -gt 0 ]; then
    error "Missing required environment variables: ${MISSING_VARS[*]}"
fi
success "All required environment variables are set"

# Step 4: Pull Docker images
info "Step 4: Pulling Docker images..."
if docker compose -f "$COMPOSE_FILE" pull; then
    success "Docker images pulled"
else
    warn "Failed to pull some images (will try to use local images)"
fi

# Step 5: Start infrastructure services
info "Step 5: Starting infrastructure services (PostgreSQL, Redpanda, Meilisearch)..."
docker compose -f "$COMPOSE_FILE" up -d postgres redpanda meilisearch

# Wait for services to be healthy
info "  Waiting for services to be healthy..."
for i in {1..30}; do
    if docker compose -f "$COMPOSE_FILE" ps | grep -q "healthy"; then
        success "  Infrastructure services are healthy"
        break
    fi
    if [ $i -eq 30 ]; then
        error "  Timeout waiting for infrastructure services"
    fi
    sleep 2
done

# Step 6: Run database migrations
if [ "$RUN_MIGRATIONS" = "true" ]; then
    info "Step 6: Running database migrations..."

    # Check if hermes-migrate is available
    if command -v hermes-migrate &> /dev/null; then
        info "  Using hermes-migrate binary..."
        hermes-migrate -driver=postgres -dsn="$DATABASE_URL" up
        success "  Migrations completed"
    else
        warn "  hermes-migrate not found, skipping automatic migrations"
        warn "  Please run: ./scripts/deployment/phase1-database-setup.sh"
    fi
else
    info "Step 6: Skipping database migrations (RUN_MIGRATIONS=false)"
fi

# Step 7: Start Hermes API server
info "Step 7: Starting Hermes API server..."
docker compose -f "$COMPOSE_FILE" up -d hermes

# Wait for Hermes to be healthy
info "  Waiting for Hermes API to be healthy..."
for i in {1..30}; do
    if curl -sf http://localhost:8000/health > /dev/null 2>&1; then
        success "  Hermes API is healthy"
        break
    fi
    if [ $i -eq 30 ]; then
        error "  Timeout waiting for Hermes API"
    fi
    sleep 2
done

# Step 8: Start indexer workers
info "Step 8: Starting indexer workers (replicas=$INDEXER_REPLICAS)..."
docker compose -f "$COMPOSE_FILE" up -d --scale indexer-worker=$INDEXER_REPLICAS indexer-worker
success "Indexer workers started"

# Step 9: Verify all services
info "Step 9: Verifying all services..."
docker compose -f "$COMPOSE_FILE" ps

# Step 10: Run validation
if [ "$RUN_VALIDATION" = "true" ]; then
    info "Step 10: Running deployment validation..."
    sleep 5  # Give services a moment to stabilize

    if [ -f "./scripts/validate-production-deployment.sh" ]; then
        ./scripts/validate-production-deployment.sh
    else
        warn "Validation script not found, skipping"
    fi
else
    info "Step 10: Skipping validation (RUN_VALIDATION=false)"
fi

# Summary
echo ""
echo "================================================"
echo "Deployment Complete"
echo "================================================"
success "All services are running!"
echo ""
info "Service URLs:"
echo "  - Hermes API:       http://localhost:8000"
echo "  - Hermes Metrics:   http://localhost:9090/metrics"
echo "  - Meilisearch:      http://localhost:7700"
echo "  - Redpanda Admin:   http://localhost:9644"
echo ""
info "Check logs:"
echo "  docker compose -f $COMPOSE_FILE logs -f hermes"
echo "  docker compose -f $COMPOSE_FILE logs -f indexer-worker"
echo ""
info "Scale indexer workers:"
echo "  docker compose -f $COMPOSE_FILE up -d --scale indexer-worker=10"
echo ""
info "Stop all services:"
echo "  docker compose -f $COMPOSE_FILE down"
echo ""
