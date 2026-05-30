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
lint: ## Run full repo lint (golangci-lint, go vet, syntax check)
	@echo "Running full repo lint..."
	@./scripts/validate-go-syntax.sh
	@go vet ./...
	@command -v golangci-lint > /dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@golangci-lint run --timeout=5m ./...
	@echo "✓ Lint complete"

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

.PHONY: build-binaries
build-binaries: ## Build all binaries
	@echo "Building binaries..."
	@mkdir -p build/bin
	@go build -o build/bin/hermes ./cmd/hermes
	@go build -o build/bin/hermes-migrate ./cmd/hermes-migrate
	@go build -o build/bin/hermes-notify ./cmd/hermes-notify
	@go build -o build/bin/hermes-indexer ./cmd/hermes-indexer
	@echo "✓ Binaries built in build/bin/"

.PHONY: build/linux
build/linux: web/build ## Build web assets and Linux binaries for Docker image builds
	@echo "Building Linux binaries..."
	@mkdir -p build/bin
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/bin/hermes ./cmd/hermes
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/bin/hermes-migrate ./cmd/hermes-migrate
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/bin/hermes-notify ./cmd/hermes-notify
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/bin/hermes-indexer ./cmd/hermes-indexer
	@echo "✓ Linux binaries built in build/bin/"

.PHONY: web/set-yarn-version
web/set-yarn-version: ## Enable Corepack and activate the web package's pinned Yarn version
	@corepack enable
	@cd web && corepack prepare yarn@$$(node -p "require('./package.json').packageManager.split('@')[1]") --activate

.PHONY: web/build
web/build: web/set-yarn-version ## Install web dependencies and build production assets
	@cd web && yarn install --immutable
	@cd web && yarn build

.PHONY: web/test
web/test: web/set-yarn-version ## Install web dependencies and run frontend tests
	@cd web && yarn install --immutable
	@cd web && yarn test

.PHONY: build-indexer
build-indexer: ## Build hermes-indexer binary
	@echo "Building hermes-indexer..."
	@mkdir -p build/bin
	@go build -o build/bin/hermes-indexer ./cmd/hermes-indexer
	@echo "✓ hermes-indexer built: build/bin/hermes-indexer"

.PHONY: build-notify
build-notify: ## Build hermes-notify binary
	@echo "Building hermes-notify..."
	@mkdir -p build/bin
	@go build -o build/bin/hermes-notify ./cmd/hermes-notify
	@echo "✓ hermes-notify built: build/bin/hermes-notify"

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

.PHONY: test-migration
test-migration: ## Run RFC-089 migration e2e integration tests
	@echo "========================================="
	@echo "RFC-089 Migration E2E Integration Tests"
	@echo "========================================="
	@echo ""
	@echo "Prerequisites will be checked automatically."
	@echo "If services are not running, start them with:"
	@echo "  cd testing && docker compose up -d postgres minio"
	@echo ""
	@go test -tags=integration -v -timeout=10m ./tests/integration/migration/...

.PHONY: test-migration-quick
test-migration-quick: ## Run migration tests without verbose output
	@echo "Running migration tests (quick mode)..."
	@go test -tags=integration -timeout=5m ./tests/integration/migration/...

.PHONY: test-migration-phase
test-migration-phase: ## Run specific migration test phase (use PHASE=Phase7_WorkerProcessing)
	@if [ -z "$(PHASE)" ]; then \
		echo "Error: PHASE variable not set"; \
		echo "Usage: make test-migration-phase PHASE=Phase7_WorkerProcessing"; \
		exit 1; \
	fi
	@echo "Running migration test phase: $(PHASE)"
	@go test -tags=integration -v -timeout=5m ./tests/integration/migration/ -run TestMigrationE2E/$(PHASE)

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@mkdir -p build/coverage
	@go test -coverprofile=build/coverage/coverage.out ./...
	@go tool cover -html=build/coverage/coverage.out -o build/coverage/coverage.html
	@echo "✓ Coverage report generated: build/coverage/coverage.html"

.PHONY: test-services-up
test-services-up: ## Start required test services (PostgreSQL, MinIO, Redpanda)
	@echo "Starting test services..."
	@cd testing && docker compose up -d postgres minio redpanda
	@echo "Waiting for services to be healthy..."
	@sleep 5
	@echo "✓ Test services started"

.PHONY: test-services-down
test-services-down: ## Stop test services
	@echo "Stopping test services..."
	@cd testing && docker compose down
	@echo "✓ Test services stopped"

.PHONY: test-services-logs
test-services-logs: ## View logs from test services
	@cd testing && docker compose logs -f postgres minio redpanda

.PHONY: db-migrate
db-migrate: ## Run database migrations
	@echo "Running database migrations..."
	@go run main.go migrate -config testing/config-central.hcl
	@echo "✓ Database migrations complete"

.PHONY: db-migrate-test
db-migrate-test: ## Run database migrations for test environment
	@echo "Running test database migrations..."
	@POSTGRES_HOST=localhost POSTGRES_PORT=5433 POSTGRES_DB=hermes_testing go run main.go migrate -config testing/config-central.hcl
	@echo "✓ Test database migrations complete"

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

.PHONY: ci
ci: ## Run full CI checks locally (matches GitHub Actions)
	@echo "========================================="
	@echo "Running Local CI Pipeline"
	@echo "========================================="
	@echo ""
	@echo "Step 1/5: Format check..."
	@./scripts/validate-go-syntax.sh
	@echo "✓ Format check complete"
	@echo ""
	@echo "Step 2/5: Linting (go vet + golangci-lint + complexity)..."
	@go vet ./...
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
	else \
		echo "⚠️  golangci-lint not found, skipping (install: make ci-install-tools)"; \
	fi
	@./scripts/check-complexity.sh
	@echo "✓ Linting complete"
	@echo ""
	@echo "Step 3/5: Building binaries..."
	@$(MAKE) build-binaries
	@echo "✓ Build complete"
	@echo ""
	@echo "Step 4/5: Running Go tests..."
	@go test -v -race -coverprofile=build/coverage/coverage.out -covermode=atomic ./...
	@echo "✓ Go tests complete"
	@echo ""
	@echo "Step 5/5: Running integration tests..."
	@echo "⚠️  Skipping integration tests (require test services)"
	@echo "   To run: make test-services-up && make test-integration"
	@echo ""
	@echo "========================================="
	@echo "✅ Local CI Pipeline Complete"
	@echo "========================================="
	@echo ""
	@echo "Note: Web and Python tests not included in this target."
	@echo "To run them separately:"
	@echo "  - Web: make web/build && make web/test"
	@echo "  - Python: cd python-client && pytest"

.PHONY: ci-install-tools
ci-install-tools: ## Install tools needed for local CI
	@echo "Installing CI tools..."
	@echo "Installing golangci-lint..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.55.2
	@$(MAKE) complexity-install
	@echo "✓ CI tools installed"

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@go clean -cache -testcache
	@rm -rf build/bin/* build/coverage/* build/reports/* build/test/* build/logs/* build/tmp/*
	@rm -f coverage.out coverage.html # Legacy files in root
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
