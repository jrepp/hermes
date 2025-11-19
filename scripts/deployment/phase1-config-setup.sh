#!/bin/bash
# RFC-088 Phase 1: Configuration Setup Script
# This script creates production configuration files for Hermes and the indexer worker

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
OUTPUT_DIR="${OUTPUT_DIR:-./config-production}"
OPENAI_API_KEY="${OPENAI_API_KEY:-}"
DATABASE_URL="${DATABASE_URL:-postgresql://hermes:password@localhost:5432/hermes?sslmode=require}"
MEILISEARCH_URL="${MEILISEARCH_URL:-http://localhost:7700}"
MEILISEARCH_KEY="${MEILISEARCH_KEY:-}"
REDPANDA_BROKERS="${REDPANDA_BROKERS:-localhost:9092}"
ENVIRONMENT="${ENVIRONMENT:-production}"

echo "================================================"
echo "RFC-088 Phase 1: Configuration Setup"
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

# Create output directory
info "Creating configuration directory: $OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"
success "Directory created"

# Step 1: Create main Hermes server configuration
info "Step 1: Creating Hermes server configuration..."

cat > "$OUTPUT_DIR/config.hcl" << 'EOF'
# Hermes Production Configuration - RFC-088 Semantic Search Enabled
# Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")

# Database configuration
database {
  # Connection string from environment
  dsn = env("DATABASE_URL")

  # Connection pool settings (optimized for production)
  max_open_conns = 25
  max_idle_conns = 10
  conn_max_lifetime = "1h"
  conn_max_idle_time = "10m"
}

# Search provider configuration
search {
  provider = "meilisearch"
  meilisearch_url = env("MEILISEARCH_URL")
  meilisearch_key = env("MEILISEARCH_KEY")
}

# Feature flags
features {
  # RFC-088: Enable semantic search
  enable_semantic_search = true

  # RFC-088: Enable hybrid search (keyword + semantic)
  enable_hybrid_search = true

  # RFC-088: Enable similar documents endpoint
  enable_similar_documents = true
}

# Semantic search configuration (RFC-088)
semantic_search {
  enabled = true

  # OpenAI embedding model
  model = "text-embedding-3-small"
  dimensions = 1536

  # Default similarity threshold (0.0 - 1.0)
  similarity_threshold = 0.7

  # Maximum results to return
  max_results = 100
}

# LLM provider configuration (RFC-088)
llm {
  # OpenAI configuration
  openai_api_key = env("OPENAI_API_KEY")

  # Default models
  default_embedding_model = "text-embedding-3-small"
  default_chat_model = "gpt-4o-mini"
}

# Redpanda/Kafka configuration (RFC-088)
kafka {
  brokers = env("REDPANDA_BROKERS")
  topic = "hermes.document-revisions"

  # TLS/SASL configuration (optional)
  enable_tls = false
  # sasl_mechanism = "SCRAM-SHA-256"
  # sasl_username = env("KAFKA_USERNAME")
  # sasl_password = env("KAFKA_PASSWORD")
}

# HTTP server configuration
server {
  addr = "0.0.0.0:8000"
  read_timeout = "30s"
  write_timeout = "30s"
  idle_timeout = "120s"
}

# Metrics and monitoring
metrics {
  enabled = true
  addr = "0.0.0.0:9090"
}

# Health checks
health {
  enabled = true
  # Endpoints: /health, /ready, /live
}

# Logging
log {
  level = "info"
  format = "json"
}
EOF

success "Created: $OUTPUT_DIR/config.hcl"

# Step 2: Create indexer worker configuration
info "Step 2: Creating indexer worker configuration..."

cat > "$OUTPUT_DIR/indexer-worker.hcl" << 'EOF'
# Hermes Indexer Worker Configuration - RFC-088
# Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")

# Database configuration (for outbox relay - if needed)
database {
  dsn = env("DATABASE_URL")
  max_open_conns = 10
  max_idle_conns = 5
}

# Search provider configuration
search {
  provider = "meilisearch"
  meilisearch_url = env("MEILISEARCH_URL")
  meilisearch_key = env("MEILISEARCH_KEY")
}

# LLM provider configuration (RFC-088)
llm {
  # OpenAI configuration
  openai_api_key = env("OPENAI_API_KEY")

  # Ollama configuration (optional, for local LLM)
  # ollama_url = "http://localhost:11434"

  # AWS Bedrock configuration (optional)
  # bedrock_region = "us-east-1"
}

# Indexer configuration (RFC-088)
indexer {
  # Redpanda/Kafka configuration
  redpanda_brokers = env("REDPANDA_BROKERS")
  topic = "hermes.document-revisions"
  consumer_group = "hermes-indexer-workers"

  # Worker settings
  workers = 4
  batch_size = 10
  poll_interval = "1s"

  # TLS/SASL configuration (optional)
  enable_tls = false
  # sasl_mechanism = "SCRAM-SHA-256"
  # sasl_username = env("KAFKA_USERNAME")
  # sasl_password = env("KAFKA_PASSWORD")

  # Pipeline rulesets
  # Each ruleset defines conditions for matching documents and processing steps
  rulesets = [
    # Ruleset 1: Published RFCs get full processing (embeddings + summary)
    {
      name = "published-rfcs"

      # Conditions to match (AND logic)
      conditions = {
        document_type = "RFC"
        status = "In-Review,Approved"
      }

      # Pipeline steps to execute (in order)
      pipeline = [
        "search_index",  # Update Meilisearch
        "embeddings",    # Generate embeddings for semantic search
        # "llm_summary", # Generate AI summary (uncomment if needed)
      ]

      # Step-specific configuration
      config = {
        embeddings = {
          model = "text-embedding-3-small"
          dimensions = 1536
          provider = "openai"
          chunk_size = 8000  # Split large documents into 8KB chunks
        }

        # llm_summary = {
        #   model = "gpt-4o-mini"
        #   max_tokens = 500
        #   style = "executive"
        # }
      }
    },

    # Ruleset 2: All documents get search indexing
    {
      name = "all-documents"

      # No conditions = matches all documents
      conditions = {}

      # Only update search index for all documents
      pipeline = ["search_index"]
    },

    # Ruleset 3: Design documents get embeddings for better search
    {
      name = "design-docs"

      conditions = {
        document_type = "PRD,Design Doc"
      }

      pipeline = ["search_index", "embeddings"]

      config = {
        embeddings = {
          model = "text-embedding-3-small"
          dimensions = 1536
          provider = "openai"
          chunk_size = 8000
        }
      }
    },
  ]
}

# Metrics and monitoring
metrics {
  enabled = true
  addr = "0.0.0.0:9091"
}

# Logging
log {
  level = "info"
  format = "json"
}
EOF

success "Created: $OUTPUT_DIR/indexer-worker.hcl"

# Step 3: Create environment variables template
info "Step 3: Creating environment variables template..."

cat > "$OUTPUT_DIR/.env.template" << 'EOF'
# Hermes Production Environment Variables - RFC-088
# Copy this file to .env and fill in the values

# Database
DATABASE_URL="postgresql://hermes:PASSWORD@localhost:5432/hermes?sslmode=require"

# Search
MEILISEARCH_URL="http://localhost:7700"
MEILISEARCH_KEY=""

# LLM Provider (OpenAI)
OPENAI_API_KEY=""

# Kafka/Redpanda
REDPANDA_BROKERS="localhost:9092,localhost:9093,localhost:9094"

# Optional: Kafka SASL authentication
# KAFKA_USERNAME=""
# KAFKA_PASSWORD=""

# Optional: Ollama (local LLM)
# OLLAMA_URL="http://localhost:11434"

# Optional: AWS Bedrock
# BEDROCK_REGION="us-east-1"
# AWS_ACCESS_KEY_ID=""
# AWS_SECRET_ACCESS_KEY=""

# Optional: Base URL for the web interface
# HERMES_BASE_URL="https://hermes.example.com"
EOF

success "Created: $OUTPUT_DIR/.env.template"

# Step 4: Create Docker Compose production configuration
info "Step 4: Creating Docker Compose production configuration..."

cat > "$OUTPUT_DIR/docker-compose.production.yml" << 'EOF'
# Docker Compose for RFC-088 Production Deployment
# This includes all services needed for semantic search: PostgreSQL, Redpanda, Meilisearch, Hermes, and Indexer

version: '3.8'

services:
  # PostgreSQL with pgvector extension
  postgres:
    image: pgvector/pgvector:pg15
    container_name: hermes-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: hermes
      POSTGRES_USER: hermes
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U hermes"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - hermes

  # Redpanda (Kafka-compatible message broker)
  redpanda:
    image: docker.redpanda.com/redpandadata/redpanda:latest
    container_name: hermes-redpanda
    restart: unless-stopped
    command:
      - redpanda
      - start
      - --smp 2
      - --memory 2G
      - --reserve-memory 0M
      - --overprovisioned
      - --node-id 0
      - --kafka-addr internal://0.0.0.0:9092,external://0.0.0.0:19092
      - --advertise-kafka-addr internal://redpanda:9092,external://localhost:19092
    ports:
      - "19092:19092"  # Kafka API (external)
      - "9644:9644"    # Admin API
      - "8081:8081"    # Schema Registry
    volumes:
      - redpanda-data:/var/lib/redpanda/data
    healthcheck:
      test: ["CMD-SHELL", "rpk cluster health | grep -E 'Healthy:.+true' || exit 1"]
      interval: 10s
      timeout: 10s
      retries: 5
      start_period: 30s
    networks:
      - hermes

  # Meilisearch (keyword search)
  meilisearch:
    image: getmeili/meilisearch:latest
    container_name: hermes-meilisearch
    restart: unless-stopped
    environment:
      MEILI_MASTER_KEY: ${MEILISEARCH_KEY}
      MEILI_ENV: production
      MEILI_NO_ANALYTICS: "true"
    volumes:
      - meilisearch-data:/meili_data
    ports:
      - "7700:7700"
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:7700/health"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - hermes

  # Hermes API server
  hermes:
    image: hermes:latest
    container_name: hermes-api
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
      redpanda:
        condition: service_healthy
      meilisearch:
        condition: service_healthy
    environment:
      DATABASE_URL: postgresql://hermes:${POSTGRES_PASSWORD}@postgres:5432/hermes?sslmode=disable
      OPENAI_API_KEY: ${OPENAI_API_KEY}
      MEILISEARCH_URL: http://meilisearch:7700
      MEILISEARCH_KEY: ${MEILISEARCH_KEY}
      REDPANDA_BROKERS: redpanda:9092
      HERMES_BASE_URL: ${HERMES_BASE_URL:-http://localhost:8000}
    ports:
      - "8000:8000"   # API
      - "9090:9090"   # Metrics
    volumes:
      - ./config.hcl:/app/config.hcl:ro
    command: ["server", "-config=/app/config.hcl"]
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "-O", "/dev/null", "http://localhost:8000/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    networks:
      - hermes

  # Indexer worker (RFC-088)
  indexer-worker:
    image: hermes:latest
    container_name: hermes-indexer
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
      redpanda:
        condition: service_healthy
      meilisearch:
        condition: service_healthy
      hermes:
        condition: service_healthy
    environment:
      DATABASE_URL: postgresql://hermes:${POSTGRES_PASSWORD}@postgres:5432/hermes?sslmode=disable
      OPENAI_API_KEY: ${OPENAI_API_KEY}
      MEILISEARCH_URL: http://meilisearch:7700
      MEILISEARCH_KEY: ${MEILISEARCH_KEY}
      REDPANDA_BROKERS: redpanda:9092
    volumes:
      - ./indexer-worker.hcl:/app/config.hcl:ro
    command: ["/app/hermes-indexer", "-config=/app/config.hcl"]
    networks:
      - hermes
    deploy:
      replicas: 2  # Scale up for higher throughput

networks:
  hermes:
    driver: bridge

volumes:
  postgres-data:
  redpanda-data:
  meilisearch-data:
EOF

success "Created: $OUTPUT_DIR/docker-compose.production.yml"

# Step 5: Create Kubernetes deployment manifests
info "Step 5: Creating Kubernetes deployment manifests..."

mkdir -p "$OUTPUT_DIR/k8s"

# Namespace
cat > "$OUTPUT_DIR/k8s/namespace.yaml" << 'EOF'
apiVersion: v1
kind: Namespace
metadata:
  name: hermes
  labels:
    name: hermes
EOF

# ConfigMap
cat > "$OUTPUT_DIR/k8s/configmap.yaml" << 'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: hermes-config
  namespace: hermes
data:
  config.hcl: |
    # Load from ConfigMap - customize as needed
    # See config.hcl for full configuration
EOF

# Deployment for Hermes API
cat > "$OUTPUT_DIR/k8s/deployment-api.yaml" << 'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hermes-api
  namespace: hermes
  labels:
    app: hermes
    component: api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hermes
      component: api
  template:
    metadata:
      labels:
        app: hermes
        component: api
    spec:
      containers:
      - name: hermes
        image: hermes:latest
        imagePullPolicy: Always
        command: ["server", "-config=/app/config/config.hcl"]
        ports:
        - name: http
          containerPort: 8000
        - name: metrics
          containerPort: 9090
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: hermes-secrets
              key: database-url
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: hermes-secrets
              key: openai-api-key
        - name: MEILISEARCH_URL
          value: "http://meilisearch:7700"
        - name: MEILISEARCH_KEY
          valueFrom:
            secretKeyRef:
              name: hermes-secrets
              key: meilisearch-key
        - name: REDPANDA_BROKERS
          value: "redpanda:9092"
        volumeMounts:
        - name: config
          mountPath: /app/config
        livenessProbe:
          httpGet:
            path: /health
            port: 8000
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8000
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
      volumes:
      - name: config
        configMap:
          name: hermes-config
EOF

# Deployment for Indexer Worker
cat > "$OUTPUT_DIR/k8s/deployment-indexer.yaml" << 'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hermes-indexer
  namespace: hermes
  labels:
    app: hermes
    component: indexer
spec:
  replicas: 4
  selector:
    matchLabels:
      app: hermes
      component: indexer
  template:
    metadata:
      labels:
        app: hermes
        component: indexer
    spec:
      containers:
      - name: indexer
        image: hermes:latest
        imagePullPolicy: Always
        command: ["/app/hermes-indexer", "-config=/app/config/indexer-worker.hcl"]
        ports:
        - name: metrics
          containerPort: 9091
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: hermes-secrets
              key: database-url
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: hermes-secrets
              key: openai-api-key
        - name: MEILISEARCH_URL
          value: "http://meilisearch:7700"
        - name: MEILISEARCH_KEY
          valueFrom:
            secretKeyRef:
              name: hermes-secrets
              key: meilisearch-key
        - name: REDPANDA_BROKERS
          value: "redpanda:9092"
        volumeMounts:
        - name: config
          mountPath: /app/config
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "2000m"
      volumes:
      - name: config
        configMap:
          name: hermes-config
EOF

# Service
cat > "$OUTPUT_DIR/k8s/service.yaml" << 'EOF'
apiVersion: v1
kind: Service
metadata:
  name: hermes-api
  namespace: hermes
  labels:
    app: hermes
    component: api
spec:
  type: ClusterIP
  ports:
  - name: http
    port: 8000
    targetPort: 8000
  - name: metrics
    port: 9090
    targetPort: 9090
  selector:
    app: hermes
    component: api
EOF

success "Created Kubernetes manifests in: $OUTPUT_DIR/k8s/"

# Step 6: Create README
info "Step 6: Creating deployment README..."

cat > "$OUTPUT_DIR/README.md" << 'EOF'
# RFC-088 Production Deployment Configuration

This directory contains production-ready configuration files for deploying Hermes with RFC-088 semantic search capabilities.

## Files

- `config.hcl` - Main Hermes server configuration
- `indexer-worker.hcl` - Indexer worker configuration
- `.env.template` - Environment variables template
- `docker-compose.production.yml` - Docker Compose deployment
- `k8s/` - Kubernetes deployment manifests

## Quick Start (Docker Compose)

1. Copy environment template:
   ```bash
   cp .env.template .env
   ```

2. Edit `.env` and fill in values:
   - `DATABASE_URL` - PostgreSQL connection string
   - `OPENAI_API_KEY` - OpenAI API key
   - `MEILISEARCH_KEY` - Meilisearch master key

3. Start services:
   ```bash
   docker-compose -f docker-compose.production.yml up -d
   ```

4. Run database setup:
   ```bash
   ./scripts/deployment/phase1-database-setup.sh
   ```

5. Validate deployment:
   ```bash
   ./scripts/deployment/phase2-validation.sh
   ```

## Kubernetes Deployment

1. Create namespace:
   ```bash
   kubectl apply -f k8s/namespace.yaml
   ```

2. Create secrets:
   ```bash
   kubectl create secret generic hermes-secrets \
     --namespace=hermes \
     --from-literal=database-url="postgresql://..." \
     --from-literal=openai-api-key="sk-..." \
     --from-literal=meilisearch-key="..."
   ```

3. Create ConfigMap (customize config.hcl first):
   ```bash
   kubectl create configmap hermes-config \
     --namespace=hermes \
     --from-file=config.hcl=config.hcl \
     --from-file=indexer-worker.hcl=indexer-worker.hcl
   ```

4. Deploy services:
   ```bash
   kubectl apply -f k8s/deployment-api.yaml
   kubectl apply -f k8s/deployment-indexer.yaml
   kubectl apply -f k8s/service.yaml
   ```

## Scaling

### Docker Compose
```bash
docker-compose -f docker-compose.production.yml up -d --scale indexer-worker=10
```

### Kubernetes
```bash
kubectl scale deployment hermes-indexer --replicas=10 -n hermes
```

## Monitoring

- Hermes API metrics: http://localhost:9090/metrics
- Indexer metrics: http://localhost:9091/metrics
- Health check: http://localhost:8000/health

Import Grafana dashboards from `docs/deployment/monitoring-setup.md`.

## Next Steps

1. Run Phase 2 validation: `./scripts/deployment/phase2-validation.sh`
2. Set up monitoring: See `docs/deployment/monitoring-setup.md`
3. Configure alerts: See `docs/deployment/monitoring-setup.md`
4. Run performance tests: See `docs/deployment/performance-tuning.md`

## Support

See documentation:
- API: `docs/api/SEMANTIC-SEARCH-API.md`
- Performance: `docs/deployment/performance-tuning.md`
- Best Practices: `docs/guides/best-practices.md`
- Troubleshooting: `docs/guides/troubleshooting.md`
EOF

success "Created: $OUTPUT_DIR/README.md"

# Summary
echo ""
echo "================================================"
echo "Configuration Setup Complete"
echo "================================================"
success "All configuration files created in: $OUTPUT_DIR"
echo ""
info "Files created:"
echo "  - config.hcl (Hermes server config)"
echo "  - indexer-worker.hcl (Indexer worker config)"
echo "  - .env.template (Environment variables)"
echo "  - docker-compose.production.yml (Docker deployment)"
echo "  - k8s/*.yaml (Kubernetes manifests)"
echo "  - README.md (Deployment guide)"
echo ""
warn "Next Steps:"
echo "  1. Edit $OUTPUT_DIR/.env.template and save as .env"
echo "  2. Set OPENAI_API_KEY, DATABASE_URL, and other secrets"
echo "  3. Run database setup: ./scripts/deployment/phase1-database-setup.sh"
echo "  4. Deploy with Docker Compose or Kubernetes"
echo ""
