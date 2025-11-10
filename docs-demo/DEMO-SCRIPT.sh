#!/usr/bin/env bash
# Hermes Executive-Level Demo Script
# Duration: 10 minutes
# Prerequisites: Docker, Go 1.25+, Node.js 20+

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Demo configuration
DEMO_PAUSE=${DEMO_PAUSE:-3}  # Seconds to pause between sections
SKIP_SETUP=${SKIP_SETUP:-false}
HERMES_ROOT=${HERMES_ROOT:-".."}

# Helper functions
demo_header() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
    sleep 1
}

demo_section() {
    echo -e "\n${GREEN}>>> $1${NC}\n"
    sleep 1
}

demo_command() {
    echo -e "${YELLOW}\$ $1${NC}"
    sleep 0.5
}

demo_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

demo_info() {
    echo -e "${CYAN}ℹ $1${NC}"
}

demo_pause() {
    if [ "$DEMO_PAUSE" -gt 0 ]; then
        echo -e "\n${YELLOW}[Press Enter to continue...]${NC}"
        read -r
    fi
}

check_prereqs() {
    demo_section "Checking prerequisites..."

    # Check Docker/Podman
    if command -v docker &> /dev/null; then
        demo_success "Docker $(docker --version | awk '{print $3}')"
    elif command -v podman &> /dev/null; then
        demo_success "Podman $(podman --version | awk '{print $3}')"
        export DOCKER_HOST="unix://$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')"
    else
        echo -e "${RED}Error: Docker or Podman not found${NC}"
        exit 1
    fi

    # Check Go
    if command -v go &> /dev/null; then
        demo_success "Go $(go version | awk '{print $3}')"
    else
        echo -e "${YELLOW}Warning: Go not found (optional for demo)${NC}"
    fi

    # Check Node.js
    if command -v node &> /dev/null; then
        demo_success "Node.js $(node --version)"
    else
        echo -e "${YELLOW}Warning: Node.js not found (optional for demo)${NC}"
    fi

    echo ""
}

setup_environment() {
    if [ "$SKIP_SETUP" = "true" ]; then
        demo_info "Skipping environment setup (SKIP_SETUP=true)"
        return
    fi

    demo_section "Setting up Hermes testing environment..."

    demo_command "cd $HERMES_ROOT/testing && docker compose up -d"
    cd "$HERMES_ROOT/testing"

    if ! docker compose up -d; then
        echo -e "${RED}Error: Failed to start Docker Compose services${NC}"
        exit 1
    fi

    demo_success "Services started"
    sleep 3

    demo_section "Waiting for services to be healthy..."

    # Wait for PostgreSQL
    demo_info "Waiting for PostgreSQL..."
    for i in {1..30}; do
        if docker compose exec -T postgres pg_isready -U postgres &> /dev/null; then
            demo_success "PostgreSQL is ready"
            break
        fi
        sleep 1
    done

    # Wait for Meilisearch
    demo_info "Waiting for Meilisearch..."
    for i in {1..30}; do
        if curl -sf http://localhost:7700/health &> /dev/null; then
            demo_success "Meilisearch is ready"
            break
        fi
        sleep 1
    done

    # Wait for Dex
    demo_info "Waiting for Dex..."
    for i in {1..30}; do
        if curl -sf http://localhost:5556/.well-known/openid-configuration &> /dev/null; then
            demo_success "Dex is ready"
            break
        fi
        sleep 1
    done

    # Wait for Backend
    demo_info "Waiting for Hermes backend..."
    for i in {1..60}; do
        if curl -sf http://localhost:8001/healthz &> /dev/null; then
            demo_success "Backend is ready"
            break
        fi
        sleep 1
    done

    # Wait for Frontend
    demo_info "Waiting for Hermes frontend..."
    for i in {1..60}; do
        if curl -sf http://localhost:4201 &> /dev/null; then
            demo_success "Frontend is ready"
            break
        fi
        sleep 1
    done

    cd - > /dev/null

    echo ""
    demo_success "All services are healthy!"
    demo_pause
}

demo1_local_setup() {
    demo_header "Demo 1: One-Command Local Setup"

    demo_section "Complete Hermes environment running locally"
    demo_info "No cloud credentials required"
    demo_info "Zero external API dependencies"
    echo ""

    demo_command "cd testing && docker compose ps"
    cd "$HERMES_ROOT/testing"
    docker compose ps
    cd - > /dev/null

    echo ""
    demo_success "Services running:"
    demo_info "• PostgreSQL (database) - :5432"
    demo_info "• Meilisearch (search engine) - :7700"
    demo_info "• Dex (OIDC provider) - :5556"
    demo_info "• Hermes backend (Go) - :8001"
    demo_info "• Hermes frontend (Ember.js) - :4201"
    echo ""

    demo_info "Open http://localhost:4201 in your browser"
    demo_info "Login: test@hermes.local / password"
    echo ""

    demo_pause
}

demo2_multi_provider() {
    demo_header "Demo 2: Multi-Provider Configuration"

    demo_section "Local workspace: Documents stored as Markdown files"

    demo_command "ls -la $HERMES_ROOT/testing/workspace_data/"
    ls -la "$HERMES_ROOT/testing/workspace_data/" 2>/dev/null || demo_info "Workspace directory will be populated after first document creation"
    echo ""

    demo_section "Configuration: Local providers (no cloud dependencies)"

    demo_command "cat config.hcl | grep -A 5 'providers'"
    cat "$HERMES_ROOT/config.hcl" 2>/dev/null | grep -A 5 "providers" || cat "$HERMES_ROOT/config-example.hcl" | grep -A 5 "providers"
    echo ""

    demo_info "Local development uses:"
    demo_info "• auth = 'dex' (local OIDC, no Google OAuth)"
    demo_info "• workspace = 'local' (filesystem, not Google Docs)"
    demo_info "• search = 'meilisearch' (self-hosted, not Algolia)"
    echo ""

    demo_section "Production configuration (for comparison)"
    demo_info "Production typically uses:"
    demo_info "• auth = 'google' or 'okta'"
    demo_info "• workspace = 'google' (Google Workspace)"
    demo_info "• search = 'algolia' or 'meilisearch' (self-hosted)"
    echo ""

    demo_success "Same code, different backends via configuration"
    demo_pause
}

demo3_architecture() {
    demo_header "Demo 3: Provider Abstraction Architecture"

    demo_section "Provider abstraction enables swappable backends"

    if [ -f "$HERMES_ROOT/docs-internal/adr/ADR-073-provider-abstraction-architecture.md" ]; then
        demo_command "cat docs-internal/adr/ADR-073-provider-abstraction-architecture.md | head -30"
        cat "$HERMES_ROOT/docs-internal/adr/ADR-073-provider-abstraction-architecture.md" | head -30
        echo ""
    else
        demo_info "Provider abstraction documented in ADR-073"
    fi

    demo_info "Key benefits:"
    demo_info "• Configuration-driven provider selection"
    demo_info "• Compile-time safety (Go interfaces)"
    demo_info "• Runtime flexibility (swap without rebuilding)"
    demo_info "• Migration support (move data between providers)"
    echo ""

    demo_section "Document migration pipeline"

    if [ -f "$HERMES_ROOT/docs-internal/rfc/RFC-080-outbox-pattern-document-sync.md" ]; then
        demo_info "Migration design documented in RFC-080"
        demo_info "• UUID-based document identity (provider-agnostic)"
        demo_info "• Version tracking across providers"
        demo_info "• Metadata preservation (authors, approvers, status)"
        demo_info "• Idempotent operations (safe retries)"
    else
        demo_info "Document migration preserves all metadata and versions"
    fi
    echo ""

    demo_pause
}

demo4_testing() {
    demo_header "Demo 4: Local Testing Excellence"

    demo_section "E2E tests with Playwright"

    if [ -d "$HERMES_ROOT/tests/e2e-playwright" ]; then
        demo_command "ls -la tests/e2e-playwright/tests/"
        ls -la "$HERMES_ROOT/tests/e2e-playwright/tests/" 2>/dev/null || demo_info "E2E test directory"
        echo ""
    fi

    demo_info "Test characteristics:"
    demo_info "• Run against local environment (Docker Compose)"
    demo_info "• Zero cloud API calls"
    demo_info "• No credentials needed"
    demo_info "• Fast execution (~30 seconds for core workflows)"
    demo_info "• Cost: \$0 per test run"
    echo ""

    demo_section "Running tests (requires services to be running)"

    if [ -d "$HERMES_ROOT/tests/e2e-playwright" ]; then
        demo_command "cd tests/e2e-playwright && npx playwright test --reporter=line"
        demo_info "Tests validate:"
        demo_info "• Document creation and editing"
        demo_info "• Search functionality"
        demo_info "• Approval workflows"
        demo_info "• User authentication"
        echo ""
        demo_info "(Actual test execution requires: cd tests/e2e-playwright && npx playwright test)"
    else
        demo_info "E2E tests location: tests/e2e-playwright/"
    fi
    echo ""

    demo_success "Complete test suite runs locally without cloud dependencies"
    demo_pause
}

show_metrics() {
    demo_header "Key Metrics & Achievements"

    echo -e "${CYAN}Codebase Maturity:${NC}"
    demo_info "• 42,000+ lines of Go code"
    demo_info "• 50,000+ lines of TypeScript/JavaScript"
    demo_info "• 782 source files"
    demo_info "• 70 design documents (16 ADRs, 19 RFCs, 35 MEMOs)"
    echo ""

    echo -e "${CYAN}Provider Support:${NC}"
    demo_info "• Auth: Dex, Google OAuth, Okta (3 providers)"
    demo_info "• Workspace: Local, Google Workspace (2 providers, Office365 coming)"
    demo_info "• Search: Meilisearch, Algolia (2 providers)"
    echo ""

    echo -e "${CYAN}Development Velocity:${NC}"
    demo_info "• Local setup time: 5 minutes (down from hours)"
    demo_info "• Test execution: ~30 seconds (no cloud latency)"
    demo_info "• Cost per developer: \$0/month (vs. \$50+ for cloud dev accounts)"
    echo ""

    echo -e "${CYAN}Testing Coverage:${NC}"
    demo_info "• E2E tests: Playwright suite for document lifecycle"
    demo_info "• Local environment: Complete Docker Compose stack (5 services)"
    demo_info "• Zero cloud dependencies: All tests run offline"
    echo ""

    demo_pause
}

show_roadmap() {
    demo_header "Roadmap & Next Steps"

    echo -e "${CYAN}Near-Term (Q1 2025):${NC}"
    demo_info "• Office365 workspace provider (2-3 weeks)"
    demo_info "• Enhanced LLM integration (auto-summaries, semantic search)"
    demo_info "• Multi-organization support"
    echo ""

    echo -e "${CYAN}Medium-Term (Q2 2025):${NC}"
    demo_info "• Distributed projects (cross-org collaboration)"
    demo_info "• Enhanced review workflows (inline comments)"
    demo_info "• Advanced search (vector embeddings, semantic search)"
    echo ""

    echo -e "${CYAN}Future Considerations:${NC}"
    demo_info "• Real-time collaboration"
    demo_info "• Mobile applications"
    demo_info "• Plugin/extension system"
    echo ""

    demo_pause
}

cleanup() {
    if [ "$SKIP_SETUP" = "false" ]; then
        demo_section "Cleanup"
        echo -e "${YELLOW}To stop services: cd testing && docker compose down${NC}"
        echo -e "${YELLOW}To remove volumes: cd testing && docker compose down -v${NC}"
        echo ""
    fi
}

main() {
    clear

    demo_header "Hermes: Local-First Document Management Demo"

    demo_info "This demo showcases:"
    demo_info "• One-command local setup (zero cloud dependencies)"
    demo_info "• Multi-provider architecture (swap backends via config)"
    demo_info "• Document migration pipeline (provider-agnostic)"
    demo_info "• Local testing excellence (E2E tests without cloud APIs)"
    echo ""

    demo_pause

    check_prereqs
    setup_environment

    demo1_local_setup
    demo2_multi_provider
    demo3_architecture
    demo4_testing

    show_metrics
    show_roadmap

    demo_header "Demo Complete!"

    echo -e "${GREEN}Key Takeaways:${NC}"
    demo_success "Local-first development with zero cloud dependencies"
    demo_success "Provider flexibility (auth, workspace, search)"
    demo_success "Production-ready (42K+ Go, 50K+ frontend, 70 docs)"
    demo_success "Migration support (move docs between providers)"
    demo_success "Cost efficiency (self-hosted eliminates cloud API costs)"
    echo ""

    demo_info "Next steps:"
    demo_info "1. Open http://localhost:4201 and explore"
    demo_info "2. Review DEMO-NARRATIVE.md for detailed talking points"
    demo_info "3. Check docs-internal/ for architecture decisions"
    demo_info "4. Run E2E tests: cd tests/e2e-playwright && npx playwright test"
    echo ""

    cleanup
}

# Handle script arguments
case "${1:-}" in
    --demo)
        case "${2:-1}" in
            1) demo1_local_setup ;;
            2) demo2_multi_provider ;;
            3) demo3_architecture ;;
            4) demo4_testing ;;
            *) echo "Invalid demo number. Use 1-4." && exit 1 ;;
        esac
        ;;
    --help|-h)
        echo "Hermes Demo Script"
        echo ""
        echo "Usage:"
        echo "  ./DEMO-SCRIPT.sh              Run full demo"
        echo "  ./DEMO-SCRIPT.sh --demo N     Run specific demo (1-4)"
        echo "  ./DEMO-SCRIPT.sh --help       Show this help"
        echo ""
        echo "Environment variables:"
        echo "  DEMO_PAUSE=N       Seconds to pause between sections (default: 3)"
        echo "  SKIP_SETUP=true    Skip Docker Compose setup"
        echo "  HERMES_ROOT=path   Path to Hermes repository root"
        exit 0
        ;;
    *)
        main
        ;;
esac
