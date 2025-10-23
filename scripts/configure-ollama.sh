#!/usr/bin/env bash
#
# configure-ollama.sh - Configure Ollama for Hermes document processing
#
# This script checks for Ollama installation, pulls required models,
# and validates the setup is ready for document summarization and embeddings.
#
# Usage:
#   ./scripts/configure-ollama.sh [--check-only] [--pull-models]
#
# Options:
#   --check-only    Only check if Ollama is configured, don't pull models
#   --pull-models   Force pull models even if they exist
#   --help          Show this help message

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
OLLAMA_HOST="${OLLAMA_HOST:-http://localhost:11434}"
SUMMARIZATION_MODEL="${OLLAMA_SUMMARIZATION_MODEL:-llama3.2}"
EMBEDDING_MODEL="${OLLAMA_EMBEDDING_MODEL:-nomic-embed-text}"
CHECK_ONLY=false
FORCE_PULL=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --check-only)
            CHECK_ONLY=true
            shift
            ;;
        --pull-models)
            FORCE_PULL=true
            shift
            ;;
        --help)
            grep '^#' "$0" | grep -v '#!/usr/bin/env' | sed 's/^# //' | sed 's/^#//'
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Run with --help for usage information"
            exit 1
            ;;
    esac
done

print_header() {
    echo -e "${BLUE}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# Check if Ollama is installed
check_ollama_installed() {
    print_header "Checking Ollama Installation"
    
    if command -v ollama &> /dev/null; then
        OLLAMA_VERSION=$(ollama --version 2>&1 | head -n 1 || echo "unknown")
        print_success "Ollama is installed: $OLLAMA_VERSION"
        return 0
    else
        print_error "Ollama is not installed"
        echo ""
        echo "Install Ollama:"
        echo "  macOS:   brew install ollama"
        echo "  Linux:   curl -fsSL https://ollama.com/install.sh | sh"
        echo "  Windows: Download from https://ollama.com/download"
        echo ""
        return 1
    fi
}

# Check if Ollama service is running
check_ollama_running() {
    print_header "Checking Ollama Service"
    
    if curl -s -f "$OLLAMA_HOST/api/version" > /dev/null 2>&1; then
        VERSION_INFO=$(curl -s "$OLLAMA_HOST/api/version" 2>&1)
        print_success "Ollama service is running at $OLLAMA_HOST"
        print_info "Version: $VERSION_INFO"
        return 0
    else
        print_error "Ollama service is not running"
        echo ""
        echo "Start Ollama:"
        echo "  ollama serve"
        echo ""
        echo "Or run in background:"
        echo "  ollama serve > /tmp/ollama.log 2>&1 &"
        echo ""
        return 1
    fi
}

# Check if a model is available
check_model() {
    local model=$1
    
    if ollama list | grep -q "^$model"; then
        local size=$(ollama list | grep "^$model" | awk '{print $2}')
        print_success "Model '$model' is available ($size)"
        return 0
    else
        print_warning "Model '$model' is not available"
        return 1
    fi
}

# Pull a model
pull_model() {
    local model=$1
    
    echo -e "${BLUE}Pulling model '$model'...${NC}"
    if ollama pull "$model"; then
        print_success "Successfully pulled '$model'"
        return 0
    else
        print_error "Failed to pull '$model'"
        return 1
    fi
}

# Test model with a simple prompt
test_model() {
    local model=$1
    local test_type=$2
    
    print_info "Testing $test_type with '$model'..."
    
    if [ "$test_type" = "summarization" ]; then
        # Test summarization model
        local response=$(curl -s "$OLLAMA_HOST/api/generate" \
            -d "{\"model\": \"$model\", \"prompt\": \"Summarize: Hello world\", \"stream\": false}" \
            2>&1)
        
        if echo "$response" | grep -q '"response"'; then
            print_success "$test_type model is working"
            return 0
        else
            print_error "$test_type model test failed"
            echo "Response: $response"
            return 1
        fi
    else
        # Test embedding model
        local response=$(curl -s "$OLLAMA_HOST/api/embeddings" \
            -d "{\"model\": \"$model\", \"prompt\": \"Hello world\"}" \
            2>&1)
        
        if echo "$response" | grep -q '"embedding"'; then
            print_success "$test_type model is working"
            return 0
        else
            print_error "$test_type model test failed"
            echo "Response: $response"
            return 1
        fi
    fi
}

# Main configuration flow
main() {
    echo ""
    print_header "Ollama Configuration for Hermes"
    echo ""
    
    # Step 1: Check installation
    if ! check_ollama_installed; then
        exit 1
    fi
    echo ""
    
    # Step 2: Check service
    if ! check_ollama_running; then
        exit 1
    fi
    echo ""
    
    # Step 3: Check models
    print_header "Checking Models"
    
    SUMMARIZATION_AVAILABLE=false
    EMBEDDING_AVAILABLE=false
    
    if check_model "$SUMMARIZATION_MODEL"; then
        SUMMARIZATION_AVAILABLE=true
    fi
    
    if check_model "$EMBEDDING_MODEL"; then
        EMBEDDING_AVAILABLE=true
    fi
    
    echo ""
    
    # Step 4: Pull models if needed
    if [ "$CHECK_ONLY" = true ]; then
        if [ "$SUMMARIZATION_AVAILABLE" = true ] && [ "$EMBEDDING_AVAILABLE" = true ]; then
            print_success "All required models are available"
            exit 0
        else
            print_error "Some required models are missing"
            exit 1
        fi
    fi
    
    if [ "$FORCE_PULL" = true ] || [ "$SUMMARIZATION_AVAILABLE" = false ]; then
        print_header "Pulling Summarization Model"
        if ! pull_model "$SUMMARIZATION_MODEL"; then
            exit 1
        fi
        echo ""
    fi
    
    if [ "$FORCE_PULL" = true ] || [ "$EMBEDDING_AVAILABLE" = false ]; then
        print_header "Pulling Embedding Model"
        if ! pull_model "$EMBEDDING_MODEL"; then
            exit 1
        fi
        echo ""
    fi
    
    # Step 5: Test models
    print_header "Testing Models"
    
    if ! test_model "$SUMMARIZATION_MODEL" "summarization"; then
        exit 1
    fi
    
    if ! test_model "$EMBEDDING_MODEL" "embedding"; then
        exit 1
    fi
    
    echo ""
    
    # Step 6: Show summary
    print_header "Configuration Summary"
    echo ""
    print_success "Ollama is ready for Hermes document processing"
    echo ""
    echo "Models available:"
    echo "  • Summarization: $SUMMARIZATION_MODEL"
    echo "  • Embeddings:    $EMBEDDING_MODEL"
    echo ""
    echo "Configuration:"
    echo "  • Ollama Host: $OLLAMA_HOST"
    echo ""
    echo "Next steps:"
    echo "  1. Copy configs/config-ollama-example.hcl to config.hcl"
    echo "  2. Configure environment variables or update config.hcl"
    echo "  3. Run: ./hermes server -config=config.hcl"
    echo ""
    echo "For more information, see: docs-internal/README-ollama.md"
    echo ""
}

# Run main function
main
