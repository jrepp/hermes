# Makefile for Hermes Go development tasks

.PHONY: help
help: ## Show this help message
	@echo "Hermes Development Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: fmt
fmt: ## Format all Go code
	@echo "Formatting Go code..."
	@gofmt -w .
	@echo "✓ Code formatted"

.PHONY: lint
lint: ## Run linters (gofmt, go vet)
	@echo "Running linters..."
	@./scripts/validate-go-syntax.sh
	@echo "✓ Linting complete"

.PHONY: complexity
complexity: ## Run complexity analysis
	@./scripts/check-complexity.sh

.PHONY: complexity-install
complexity-install: ## Install complexity analysis tools
	@echo "Installing complexity analysis tools..."
	@go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	@go install github.com/uudashr/gocognit/cmd/gocognit@latest
	@echo "✓ Tools installed"

.PHONY: build
build: ## Build all packages
	@echo "Building all packages..."
	@go build ./...
	@echo "✓ Build complete"

.PHONY: test
test: ## Run all tests
	@echo "Running tests..."
	@go test ./...

.PHONY: test-integration
test-integration: ## Run integration tests only
	@echo "Running integration tests..."
	@go test -tags=integration ./...

.PHONY: test-edge-sync
test-edge-sync: ## Run edge sync authentication integration tests
	@echo "Running edge sync authentication integration tests..."
	@go test -tags=integration -v ./tests/integration/edgesync/...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...
	@echo "✓ Vet complete"

.PHONY: tidy
tidy: ## Tidy go.mod and go.sum
	@echo "Tidying go modules..."
	@go mod tidy
	@echo "✓ Modules tidied"

.PHONY: pre-commit
pre-commit: fmt vet build ## Run pre-commit checks
	@echo "✓ Pre-commit checks complete"

.PHONY: validate
validate: lint complexity ## Run full validation (lint + complexity)
	@echo "✓ Full validation complete"

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@go clean -cache -testcache
	@rm -f coverage.out coverage.html
	@echo "✓ Clean complete"

.PHONY: install-hooks
install-hooks: ## Install pre-commit hooks
	@echo "Installing pre-commit hooks..."
	@pip3 install pre-commit
	@pre-commit install
	@pre-commit install --hook-type pre-push
	@echo "✓ Pre-commit hooks installed"

.PHONY: run-hooks
run-hooks: ## Run pre-commit hooks on all files
	@pre-commit run --all-files

.DEFAULT_GOAL := help
