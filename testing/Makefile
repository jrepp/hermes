# Hermes Testing Environment Makefile

.PHONY: help
help: ## Show this help message
	@echo "Hermes Testing Environment"
	@echo "=========================="
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build all containers
	docker-compose build

.PHONY: build-hermes
build-hermes: ## Build only Hermes backend
	docker-compose build hermes

.PHONY: build-web
build-web: ## Build only web frontend
	docker-compose build web

.PHONY: up
up: ## Start all services (build if needed)
	docker-compose up --build -d
	@echo "Waiting for services to be healthy..."
	@sleep 5
	@$(MAKE) status

.PHONY: down
down: ## Stop all services
	docker-compose down

.PHONY: restart
restart: ## Restart all services
	docker-compose restart

.PHONY: restart-hermes
restart-hermes: ## Restart only Hermes backend
	docker-compose restart hermes

.PHONY: restart-web
restart-web: ## Restart only web frontend
	docker-compose restart web

.PHONY: clean
clean: ## Stop services and remove volumes
	docker-compose down -v

.PHONY: status
status: ## Show service status
	docker-compose ps

.PHONY: logs
logs: ## Show logs from all services
	docker-compose logs -f

.PHONY: logs-hermes
logs-hermes: ## Show logs from Hermes backend
	docker-compose logs -f hermes

.PHONY: logs-web
logs-web: ## Show logs from web frontend
	docker-compose logs -f web

.PHONY: logs-postgres
logs-postgres: ## Show logs from PostgreSQL
	docker-compose logs -f postgres

.PHONY: logs-meilisearch
logs-meilisearch: ## Show logs from Meilisearch
	docker-compose logs -f meilisearch

.PHONY: shell-hermes
shell-hermes: ## Open shell in Hermes container
	docker-compose exec hermes /bin/sh

.PHONY: shell-web
shell-web: ## Open shell in web container
	docker-compose exec web /bin/sh

.PHONY: shell-postgres
shell-postgres: ## Open PostgreSQL shell
	docker-compose exec postgres psql -U postgres -d hermes_test

.PHONY: canary
canary: ## Run canary test in container
	docker-compose exec hermes /app/hermes canary -search-backend=meilisearch

.PHONY: test
test: up canary ## Start services and run canary test
	@echo ""
	@echo "✅ All tests passed!"

.PHONY: open
open: ## Open web UI in browser
	@echo "Opening http://localhost:4201"
	@open http://localhost:4201 || xdg-open http://localhost:4201 || echo "Please open http://localhost:4201 in your browser"

# === Distributed Testing Scenarios (Bash) ===

.PHONY: seed
seed: ## Seed workspaces with test documents (bash)
	./scripts/seed-workspaces.sh --scenario basic --count 10

.PHONY: seed-clean
seed-clean: ## Clean and re-seed workspaces
	./scripts/seed-workspaces.sh --scenario basic --count 10 --clean

.PHONY: seed-migration
seed-migration: ## Seed migration test scenario
	./scripts/seed-workspaces.sh --scenario migration --count 5

.PHONY: seed-conflict
seed-conflict: ## Seed conflict test scenario
	./scripts/seed-workspaces.sh --scenario conflict --count 5

.PHONY: seed-multi-author
seed-multi-author: ## Seed multi-author scenario
	./scripts/seed-workspaces.sh --scenario multi-author --count 10

.PHONY: scenario-basic
scenario-basic: ## Run basic distributed indexing scenario
	./scripts/scenario-basic.sh

.PHONY: test-distributed
test-distributed: up seed scenario-basic ## Start environment and run distributed tests
	@echo ""
	@echo "✅ Distributed testing complete!"

.PHONY: workspace-clean
workspace-clean: ## Clean all workspace data (preserves structure)
	rm -rf workspaces/testing/* workspaces/docs/*
	@echo "✅ Workspaces cleaned"

.PHONY: rebuild
rebuild: clean build up ## Clean, rebuild, and start everything

.PHONY: rebuild-hermes
rebuild-hermes: ## Rebuild and restart Hermes backend
	docker-compose build --no-cache hermes
	docker-compose up -d hermes

.PHONY: rebuild-web
rebuild-web: ## Rebuild and restart web frontend
	docker-compose build --no-cache web
	docker-compose up -d web

# === Python Testing Framework ===

.PHONY: python-setup
python-setup: ## Set up Python testing environment
	@echo "Setting up Python testing environment..."
	cd python && python3 -m pip install -e ".[dev]"
	cd ../python-client && python3 -m pip install -e ".[dev,cli]"
	@echo "✅ Python environment ready"
	@echo ""
	@echo "CLI available: hermes-test --help"

# === Unified CLI (hermes-test) ===

.PHONY: test-cli
test-cli: ## Test the hermes-test CLI installation
	@which hermes-test || (cd python && python3 hermes_test.py --help)

.PHONY: seed
seed: ## Seed workspaces (basic scenario, 10 docs) - Use: make seed COUNT=20 SCENARIO=migration
	cd python && python3 hermes_test.py seed --scenario ${SCENARIO:-basic} --count ${COUNT:-10} ${CLEAN:+--clean}

.PHONY: seed-clean
seed-clean: ## Seed workspaces with clean - Use: make seed-clean COUNT=20
	cd python && python3 hermes_test.py seed --scenario ${SCENARIO:-basic} --count ${COUNT:-10} --clean

.PHONY: scenario-basic
scenario-basic: ## Run basic scenario - Use: make scenario-basic COUNT=20
	cd python && python3 hermes_test.py scenario basic --count ${COUNT:-10} ${WAIT:+--wait}

.PHONY: scenario-migration
scenario-migration: ## Run migration scenario
	cd python && python3 hermes_test.py scenario migration --count ${COUNT:-10} ${WAIT:+--wait}

.PHONY: scenario-multi-author
scenario-multi-author: ## Run multi-author scenario
	cd python && python3 hermes_test.py scenario multi-author --count ${COUNT:-10} ${WAIT:+--wait}

.PHONY: validate
validate: ## Validate Hermes deployment
	cd python && python3 hermes_test.py validate --check-all

.PHONY: clean-workspace
clean-workspace: ## Clean test workspaces - Use: make clean-workspace WORKSPACE=testing
	cd python && python3 hermes_test.py clean --workspace ${WORKSPACE:-all} -y

# === Legacy Python Targets (deprecated - use hermes-test CLI) ===

.PHONY: python-seed
python-seed: seed ## [DEPRECATED] Use 'make seed' instead

.PHONY: python-seed-migration
python-seed-migration: ## [DEPRECATED] Use 'make seed SCENARIO=migration' instead
	@echo "⚠️  DEPRECATED: Use 'make seed SCENARIO=migration' instead"
	cd python && python seed.py --scenario migration --count 5 --clean

.PHONY: python-seed-multi-author
python-seed-multi-author: ## [DEPRECATED] Use 'make seed SCENARIO=multi-author' instead
	@echo "⚠️  DEPRECATED: Use 'make seed SCENARIO=multi-author' instead"
	cd python && python seed.py --scenario multi-author --count 10 --clean

.PHONY: scenario-basic-py
scenario-basic-py: scenario-basic ## [DEPRECATED] Use 'make scenario-basic' instead

.PHONY: scenario-migration-py
scenario-migration-py: scenario-migration ## [DEPRECATED] Use 'make scenario-migration' instead

.PHONY: scenario-multi-author-py
scenario-multi-author-py: scenario-multi-author ## [DEPRECATED] Use 'make scenario-multi-author' instead

.PHONY: test-python
test-python: ## Run pytest tests (unit only)
	cd python && pytest tests/ -v -m "not integration and not slow"

.PHONY: test-python-integration
test-python-integration: up ## Run pytest integration tests
	cd python && pytest tests/ -v -m integration

.PHONY: test-python-all
test-python-all: up ## Run all pytest tests
	cd python && pytest tests/ -v

.PHONY: test-python-coverage
test-python-coverage: ## Run tests with coverage
	cd python && pytest tests/ -v --cov=. --cov-report=html --cov-report=term

.PHONY: test-distributed-py
test-distributed-py: up python-seed scenario-basic-py ## Full distributed test with Python
	@echo ""
	@echo "✅ Python distributed testing complete!"

.PHONY: lint-python
lint-python: ## Lint Python code with ruff
	cd python && ruff check .

.PHONY: format-python
format-python: ## Format Python code with ruff
	cd python && ruff format .
