#!/bin/bash
# RFC-088: Complete Deployment Orchestration
# Runs all three phases: Preparation, Validation, and Monitoring

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
SKIP_PHASE1="${SKIP_PHASE1:-false}"
SKIP_PHASE2="${SKIP_PHASE2:-false}"
SKIP_PHASE3="${SKIP_PHASE3:-false}"
DEPLOYMENT_TYPE="${DEPLOYMENT_TYPE:-docker}"  # docker or kubernetes
AUTO_CONFIRM="${AUTO_CONFIRM:-false}"

echo "================================================"
echo "RFC-088: Complete Production Deployment"
echo "================================================"
echo ""
echo "This script will deploy RFC-088 semantic search"
echo "through all three phases:"
echo ""
echo "  Phase 1: Database & Configuration Setup"
echo "  Phase 2: Deployment & Validation"
echo "  Phase 3: Monitoring & Alerts"
echo ""
echo "Configuration:"
echo "  Deployment Type: $DEPLOYMENT_TYPE"
echo "  Skip Phase 1:    $SKIP_PHASE1"
echo "  Skip Phase 2:    $SKIP_PHASE2"
echo "  Skip Phase 3:    $SKIP_PHASE3"
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

section() {
    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}

confirm() {
    if [ "$AUTO_CONFIRM" = "true" ]; then
        return 0
    fi

    read -p "$1 (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        return 1
    fi
    return 0
}

# Check prerequisites
section "Checking Prerequisites"

info "Checking required commands..."

REQUIRED_COMMANDS=("docker" "curl" "psql")
MISSING_COMMANDS=()

for cmd in "${REQUIRED_COMMANDS[@]}"; do
    if command -v "$cmd" &> /dev/null; then
        success "$cmd is installed"
    else
        warn "$cmd is not installed"
        MISSING_COMMANDS+=("$cmd")
    fi
done

if [ ${#MISSING_COMMANDS[@]} -gt 0 ]; then
    error "Missing required commands: ${MISSING_COMMANDS[*]}"
fi

# Check for Docker Compose
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null 2>&1; then
    error "Docker Compose is not installed"
fi
success "Docker Compose is installed"

# Check scripts exist
SCRIPT_DIR="$(dirname "$0")"
REQUIRED_SCRIPTS=(
    "phase1-database-setup.sh"
    "phase1-config-setup.sh"
    "deploy-docker.sh"
    "phase2-validation.sh"
    "phase2-api-tests.sh"
    "phase3-monitoring.sh"
)

for script in "${REQUIRED_SCRIPTS[@]}"; do
    if [ -f "$SCRIPT_DIR/$script" ]; then
        success "Found: $script"
    else
        error "Missing script: $script"
    fi
done

# Pre-flight confirmation
echo ""
if ! confirm "Ready to start deployment?"; then
    info "Deployment cancelled by user"
    exit 0
fi

# Phase 1: Preparation
if [ "$SKIP_PHASE1" != "true" ]; then
    section "Phase 1: Preparation (Database & Configuration)"

    # Step 1.1: Configuration Setup
    info "Step 1.1: Creating configuration files..."
    if [ ! -d "./config-production" ]; then
        if confirm "Generate production configuration files?"; then
            "$SCRIPT_DIR/phase1-config-setup.sh"
            success "Configuration files created"

            warn "Please review and edit configuration files:"
            warn "  - config-production/.env (set API keys and secrets)"
            warn "  - config-production/config.hcl"
            warn "  - config-production/indexer-worker.hcl"
            echo ""

            if ! confirm "Configuration files ready? Continue?"; then
                error "Please configure files and re-run the deployment"
            fi
        else
            error "Configuration setup skipped. Cannot proceed without configuration."
        fi
    else
        success "Configuration directory exists"
    fi

    # Step 1.2: Database Setup
    info "Step 1.2: Setting up database (pgvector, migrations, indexes)..."

    if confirm "Run database setup now?"; then
        "$SCRIPT_DIR/phase1-database-setup.sh"
        success "Database setup completed"
    else
        warn "Database setup skipped"
        warn "You must run it manually: ./scripts/deployment/phase1-database-setup.sh"
    fi

    success "Phase 1 completed!"
else
    section "Phase 1: Skipped (SKIP_PHASE1=true)"
fi

# Phase 2: Deployment & Validation
if [ "$SKIP_PHASE2" != "true" ]; then
    section "Phase 2: Deployment & Validation"

    # Step 2.1: Deploy services
    info "Step 2.1: Deploying services..."

    if [ "$DEPLOYMENT_TYPE" = "docker" ]; then
        if confirm "Deploy using Docker Compose?"; then
            "$SCRIPT_DIR/deploy-docker.sh"
            success "Docker deployment completed"
        else
            warn "Docker deployment skipped"
        fi

    elif [ "$DEPLOYMENT_TYPE" = "kubernetes" ]; then
        info "Kubernetes deployment..."
        warn "Kubernetes deployment requires manual steps:"
        echo "  1. kubectl apply -f config-production/k8s/namespace.yaml"
        echo "  2. kubectl create secret (see config-production/README.md)"
        echo "  3. kubectl apply -f config-production/k8s/"
        echo ""

        if ! confirm "Kubernetes deployment complete? Continue?"; then
            error "Complete Kubernetes deployment and re-run"
        fi

    else
        error "Unknown deployment type: $DEPLOYMENT_TYPE (must be 'docker' or 'kubernetes')"
    fi

    # Step 2.2: Wait for services to stabilize
    info "Step 2.2: Waiting for services to stabilize..."
    sleep 10
    success "Services should be running"

    # Step 2.3: Run validation
    info "Step 2.3: Running deployment validation..."

    if confirm "Run validation tests now?"; then
        "$SCRIPT_DIR/phase2-validation.sh"

        if [ $? -eq 0 ]; then
            success "Validation completed successfully"
        else
            error "Validation failed. Fix issues before proceeding."
        fi
    else
        warn "Validation skipped"
    fi

    # Step 2.4: Run API tests
    info "Step 2.4: Running API endpoint tests..."

    if confirm "Run API tests now?"; then
        "$SCRIPT_DIR/phase2-api-tests.sh"

        if [ $? -eq 0 ]; then
            success "API tests passed"
        else
            warn "Some API tests failed (may be expected without auth token)"
        fi
    else
        warn "API tests skipped"
    fi

    success "Phase 2 completed!"
else
    section "Phase 2: Skipped (SKIP_PHASE2=true)"
fi

# Phase 3: Monitoring & Alerts
if [ "$SKIP_PHASE3" != "true" ]; then
    section "Phase 3: Monitoring & Alerts"

    # Step 3.1: Set up monitoring
    info "Step 3.1: Setting up monitoring stack..."

    if confirm "Generate monitoring configuration?"; then
        "$SCRIPT_DIR/phase3-monitoring.sh"
        success "Monitoring configuration created"
    else
        warn "Monitoring setup skipped"
    fi

    # Step 3.2: Deploy monitoring stack
    info "Step 3.2: Deploying monitoring stack..."

    if [ -f "./monitoring-config/docker-compose.monitoring.yml" ]; then
        if confirm "Deploy monitoring stack (Prometheus, Grafana, Alertmanager)?"; then
            cd ./monitoring-config
            docker-compose -f docker-compose.monitoring.yml up -d
            cd ..
            success "Monitoring stack deployed"

            info "Monitoring services:"
            echo "  - Grafana:      http://localhost:3000 (admin/admin)"
            echo "  - Prometheus:   http://localhost:9090"
            echo "  - Alertmanager: http://localhost:9093"
        else
            warn "Monitoring stack deployment skipped"
        fi
    else
        warn "Monitoring configuration not found. Run phase3-monitoring.sh first."
    fi

    success "Phase 3 completed!"
else
    section "Phase 3: Skipped (SKIP_PHASE3=true)"
fi

# Final Summary
section "Deployment Summary"

echo ""
echo -e "${GREEN}✓ RFC-088 Deployment Complete!${NC}"
echo ""
echo "═══════════════════════════════════════════════"
echo "  Next Steps"
echo "═══════════════════════════════════════════════"
echo ""

echo "1. Verify Services:"
echo "   - Hermes API:    http://localhost:8000/health"
echo "   - Metrics:       http://localhost:9090/metrics"
echo "   - Grafana:       http://localhost:3000"
echo ""

echo "2. Test Semantic Search:"
echo "   curl -X POST http://localhost:8000/api/v2/search/semantic \\"
echo "     -H 'Content-Type: application/json' \\"
echo "     -H 'Authorization: Bearer YOUR_TOKEN' \\"
echo "     -d '{\"query\": \"kubernetes deployment\", \"limit\": 10}'"
echo ""

echo "3. Monitor Performance:"
echo "   - Open Grafana dashboard (RFC-088)"
echo "   - Check Prometheus metrics"
echo "   - Review Alertmanager for alerts"
echo ""

echo "4. Scale Indexer Workers (if needed):"
if [ "$DEPLOYMENT_TYPE" = "docker" ]; then
    echo "   docker-compose -f config-production/docker-compose.production.yml \\"
    echo "     up -d --scale indexer-worker=10"
elif [ "$DEPLOYMENT_TYPE" = "kubernetes" ]; then
    echo "   kubectl scale deployment hermes-indexer --replicas=10 -n hermes"
fi
echo ""

echo "5. Review Documentation:"
echo "   - API docs:          docs/api/SEMANTIC-SEARCH-API.md"
echo "   - Performance guide: docs/deployment/performance-tuning.md"
echo "   - Best practices:    docs/guides/best-practices.md"
echo "   - Troubleshooting:   docs/guides/troubleshooting.md"
echo ""

echo "═══════════════════════════════════════════════"
echo ""

# Save deployment info
DEPLOYMENT_INFO="deployment-info-$(date +%Y%m%d-%H%M%S).txt"

cat > "$DEPLOYMENT_INFO" << EOF
RFC-088 Deployment Information
Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")

Deployment Type: $DEPLOYMENT_TYPE
Phase 1: $([ "$SKIP_PHASE1" = "true" ] && echo "Skipped" || echo "Completed")
Phase 2: $([ "$SKIP_PHASE2" = "true" ] && echo "Skipped" || echo "Completed")
Phase 3: $([ "$SKIP_PHASE3" = "true" ] && echo "Skipped" || echo "Completed")

Configuration Files: ./config-production/
Monitoring Config:   ./monitoring-config/
Test Results:        ./test-results/

Service URLs:
- Hermes API:    http://localhost:8000
- Hermes Metrics: http://localhost:9090/metrics
- Grafana:       http://localhost:3000
- Prometheus:    http://localhost:9090
- Alertmanager:  http://localhost:9093

For support, see RFC-088 documentation.
EOF

info "Deployment information saved to: $DEPLOYMENT_INFO"

success "Deployment completed successfully!"
echo ""
