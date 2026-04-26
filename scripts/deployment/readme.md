# RFC-088 Production Deployment Scripts

This directory contains automated deployment scripts for RFC-088 semantic search implementation.

## Overview

The deployment is organized into **three phases**:

1. **Phase 1: Preparation** - Database setup, migrations, indexes, and configuration
2. **Phase 2: Validation** - Deployment, testing, and validation
3. **Phase 3: Monitoring** - Prometheus, Grafana, and alerting setup

## Quick Start

### Option 1: Complete Automated Deployment

Run all three phases automatically:

```bash
./scripts/deployment/deploy-all-phases.sh
```

This script will:
- ✓ Generate production configuration files
- ✓ Set up PostgreSQL with pgvector
- ✓ Run database migrations
- ✓ Create vector indexes (IVFFlat or HNSW)
- ✓ Deploy services with Docker Compose or Kubernetes
- ✓ Run comprehensive validation tests
- ✓ Set up monitoring stack (Prometheus, Grafana, Alertmanager)

### Option 2: Manual Phase-by-Phase Deployment

#### Phase 1: Preparation

**Step 1.1: Generate Configuration**
```bash
./scripts/deployment/phase1-config-setup.sh
```

Creates:
- `config-production/config.hcl` - Hermes server config
- `config-production/indexer-worker.hcl` - Indexer worker config
- `config-production/.env.template` - Environment variables
- `config-production/docker-compose.production.yml` - Docker deployment
- `config-production/k8s/*.yaml` - Kubernetes manifests

**Step 1.2: Configure Environment**

Edit generated files:
```bash
cd config-production
cp .env.template .env
# Edit .env and set:
# - OPENAI_API_KEY
# - DATABASE_URL
# - MEILISEARCH_KEY
# - POSTGRES_PASSWORD
```

**Step 1.3: Database Setup**
```bash
./scripts/deployment/phase1-database-setup.sh
```

This script:
- ✓ Verifies PostgreSQL connectivity
- ✓ Installs pgvector extension
- ✓ Runs RFC-088 database migrations
- ✓ Creates vector indexes (IVFFlat or HNSW)
- ✓ Creates lookup indexes
- ✓ Updates table statistics
- ✓ Tests query performance

Environment variables:
- `DB_HOST` - PostgreSQL host (default: localhost)
- `DB_PORT` - PostgreSQL port (default: 5432)
- `DB_USER` - Database user (default: hermes)
- `DB_NAME` - Database name (default: hermes)
- `DB_PASSWORD` - Database password
- `INDEX_TYPE` - Index type: `ivfflat` or `hnsw` (default: ivfflat)

#### Phase 2: Deployment & Validation

**Step 2.1: Deploy with Docker**
```bash
./scripts/deployment/deploy-docker.sh
```

This script:
- ✓ Validates prerequisites and configuration
- ✓ Starts infrastructure services (PostgreSQL, Redpanda, Meilisearch)
- ✓ Runs database migrations
- ✓ Starts Hermes API server
- ✓ Starts indexer workers
- ✓ Runs validation tests

Environment variables:
- `COMPOSE_FILE` - Docker Compose file path
- `ENV_FILE` - Environment file path (default: .env)
- `RUN_MIGRATIONS` - Run migrations (default: true)
- `RUN_VALIDATION` - Run validation (default: true)
- `INDEXER_REPLICAS` - Number of indexer workers (default: 2)

**Step 2.2: Validate Deployment**
```bash
./scripts/deployment/phase2-validation.sh
```

Validates:
- ✓ Database connectivity and pgvector
- ✓ API endpoints (health, semantic search, hybrid search)
- ✓ Kafka/Redpanda cluster and topics
- ✓ Meilisearch indexes
- ✓ Database performance (vector queries)
- ✓ Connection pool health
- ✓ OpenAI API connectivity
- ✓ Metrics endpoints

**Step 2.3: Test API Endpoints**
```bash
./scripts/deployment/phase2-api-tests.sh
```

Tests:
- ✓ Semantic search (basic, filtered, threshold)
- ✓ Hybrid search (basic, weighted)
- ✓ Similar documents
- ✓ Error handling
- ✓ Performance (10 sequential queries)

Environment variables:
- `API_URL` - Hermes API URL (default: http://localhost:8000)
- `API_TOKEN` - Authentication token (for authenticated tests)
- `TEST_DOCUMENT_ID` - Document ID for similar documents test
- `OUTPUT_DIR` - Test results directory (default: ./test-results)

#### Phase 3: Monitoring

**Step 3.1: Set Up Monitoring**
```bash
./scripts/deployment/phase3-monitoring.sh
```

Creates:
- ✓ Prometheus configuration (scrape targets, retention)
- ✓ Prometheus alert rules (40+ production-ready alerts)
- ✓ Alertmanager configuration (email, Slack, PagerDuty)
- ✓ Grafana dashboard (RFC-088 semantic search metrics)
- ✓ Docker Compose monitoring stack
- ✓ Grafana provisioning (datasources, dashboards)

Environment variables:
- `OUTPUT_DIR` - Monitoring config directory (default: ./monitoring-config)
- `GRAFANA_URL` - Grafana URL (default: http://localhost:3000)
- `PROMETHEUS_URL` - Prometheus URL (default: http://localhost:9090)

**Step 3.2: Deploy Monitoring Stack**
```bash
cd monitoring-config
docker-compose -f docker-compose.monitoring.yml up -d
```

Services:
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)
- Alertmanager: http://localhost:9093

## Script Reference

### Phase 1 Scripts

| Script | Purpose | Duration |
|--------|---------|----------|
| `phase1-config-setup.sh` | Generate production configuration | <1 min |
| `phase1-database-setup.sh` | Set up database, migrations, indexes | 2-10 min |

### Phase 2 Scripts

| Script | Purpose | Duration |
|--------|---------|----------|
| `deploy-docker.sh` | Deploy with Docker Compose | 2-5 min |
| `phase2-validation.sh` | Comprehensive validation tests | 2-5 min |
| `phase2-api-tests.sh` | API endpoint testing suite | 1-3 min |

### Phase 3 Scripts

| Script | Purpose | Duration |
|--------|---------|----------|
| `phase3-monitoring.sh` | Generate monitoring configuration | <1 min |

### Orchestration Scripts

| Script | Purpose | Duration |
|--------|---------|----------|
| `deploy-all-phases.sh` | Complete automated deployment | 10-30 min |

## Alert Rules

Monitoring includes 40+ production-ready alert rules:

### API Alerts
- HighAPIErrorRate (>1%)
- HighAPILatency (P95 >1s)
- HighSemanticSearchErrorRate (>5%)
- HighSemanticSearchLatency (P95 >200ms)

### Indexer Alerts
- HighIndexerErrorRate (>5%)
- HighKafkaConsumerLag (>10k messages)
- IndexerWorkerDown
- HighLLMAPILatency (P95 >5s)

### Database Alerts
- DatabaseConnectionPoolExhausted (>90%)
- SlowVectorQueries (P95 >1s)
- DatabaseDiskSpaceLow (>80%)

### Infrastructure Alerts
- ServiceDown
- HighMemoryUsage (>90%)
- HighCPUUsage (>80%)

## Grafana Dashboard

The RFC-088 dashboard includes panels for:

1. **Semantic Search Request Rate** - Requests per second
2. **Semantic Search Error Rate** - Error percentage
3. **Semantic Search Latency** - P50, P95, P99 latency
4. **Indexer Processing Rate** - Messages per second
5. **Kafka Consumer Lag** - Lag per partition
6. **Database Connection Pool** - Open, idle, in-use connections
7. **Vector Query Performance** - P95 query latency
8. **LLM API Performance** - P95 API latency by provider

## Deployment Options

### Docker Compose

**Pros:**
- Simple setup
- Good for development/staging
- Easy to scale workers

**Cons:**
- Single host limitation
- Manual high availability

**Scale indexer workers:**
```bash
docker-compose -f config-production/docker-compose.production.yml \
  up -d --scale indexer-worker=10
```

### Kubernetes

**Pros:**
- Production-grade orchestration
- Built-in high availability
- Auto-scaling support
- Multi-node deployment

**Cons:**
- More complex setup
- Requires K8s cluster

**Deploy to Kubernetes:**
```bash
# Create namespace
kubectl apply -f config-production/k8s/namespace.yaml

# Create secrets
kubectl create secret generic hermes-secrets \
  --namespace=hermes \
  --from-literal=database-url="postgresql://..." \
  --from-literal=openai-api-key="sk-..." \
  --from-literal=meilisearch-key="..."

# Create ConfigMap
kubectl create configmap hermes-config \
  --namespace=hermes \
  --from-file=config.hcl=config-production/config.hcl \
  --from-file=indexer-worker.hcl=config-production/indexer-worker.hcl

# Deploy
kubectl apply -f config-production/k8s/
```

**Scale indexer workers:**
```bash
kubectl scale deployment hermes-indexer --replicas=10 -n hermes
```

## Troubleshooting

### Database Setup Issues

**Issue:** pgvector extension not found
```bash
# Install pgvector
# Ubuntu/Debian
sudo apt-get install postgresql-15-pgvector

# macOS
brew install pgvector
```

**Issue:** Migrations fail
```bash
# Check database connectivity
psql -h localhost -p 5432 -U hermes -d hermes -c "SELECT 1;"

# Run migrations manually
hermes-migrate -driver=postgres -dsn="$DATABASE_URL" up
```

**Issue:** Slow vector queries
```bash
# Check if index exists
psql -c "SELECT indexname FROM pg_indexes WHERE tablename = 'document_embeddings';"

# Rebuild index
psql -c "REINDEX INDEX idx_embeddings_vector_ivfflat;"

# Update statistics
psql -c "ANALYZE document_embeddings;"
```

### Deployment Issues

**Issue:** Services not starting
```bash
# Check logs
docker-compose -f config-production/docker-compose.production.yml logs -f

# Check environment variables
cat config-production/.env
```

**Issue:** API returning 500 errors
```bash
# Check database connectivity
curl http://localhost:8000/health

# Check logs
docker logs hermes-api
```

**Issue:** No semantic search results
```bash
# Check if embeddings exist
psql -c "SELECT COUNT(*) FROM document_embeddings;"

# Check indexer logs
docker logs hermes-indexer

# Check Kafka lag
rpk group describe hermes-indexer-workers --brokers localhost:19092
```

### Validation Issues

**Issue:** Validation script fails
```bash
# Run basic validation
./scripts/validate-production-deployment.sh

# Check individual components
curl http://localhost:8000/health
curl http://localhost:9090/metrics
psql -c "SELECT 1;"
```

**Issue:** API tests fail with 401
```bash
# Set API token
export API_TOKEN="your-token-here"
./scripts/deployment/phase2-api-tests.sh
```

### Monitoring Issues

**Issue:** Prometheus not scraping
```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets

# Check metrics endpoint
curl http://localhost:9090/metrics
```

**Issue:** Grafana dashboard empty
```bash
# Check Prometheus datasource
curl http://localhost:3000/api/datasources

# Import dashboard manually
# Copy monitoring-config/grafana/rfc-088-dashboard.json
# Import in Grafana UI
```

## Best Practices

1. **Configuration Management**
   - Use environment variables for secrets
   - Store configuration in version control (except .env)
   - Use separate configs for dev/staging/prod

2. **Database Management**
   - Regular backups (pg_dump)
   - Monitor disk space (embeddings table grows)
   - Vacuum regularly (weekly)
   - Update statistics after bulk loads

3. **Scaling**
   - Start with 2-4 indexer workers
   - Scale based on Kafka lag metrics
   - Monitor CPU and memory usage
   - Use connection pooling

4. **Monitoring**
   - Set up alerts before production
   - Test alert notifications
   - Review dashboards regularly
   - Tune alert thresholds based on SLAs

5. **Cost Optimization**
   - Monitor OpenAI API usage
   - Use content hash idempotency
   - Configure rulesets for selective processing
   - Consider Ollama for cost-effective summaries

6. **Security**
   - Store secrets in secrets manager
   - Use TLS for database connections
   - Enable Kafka SASL/SSL
   - Regular security updates

## Support

### Documentation

- **API Documentation**: `docs/api/SEMANTIC-SEARCH-API.md`
- **Performance Tuning**: `docs/deployment/performance-tuning.md`
- **Best Practices**: `docs/guides/best-practices.md`
- **Configuration Guide**: `docs/configuration/search-configuration.md`
- **Monitoring Setup**: `docs/deployment/monitoring-setup.md`
- **Troubleshooting Guide**: `docs/guides/troubleshooting.md`

### Validation Script

The existing validation script is also available:
```bash
./scripts/validate-production-deployment.sh
```

### Getting Help

1. Check script output for specific error messages
2. Review logs: `docker-compose logs -f`
3. Run validation: `./scripts/deployment/phase2-validation.sh`
4. Check documentation in `docs/`
5. Contact support with deployment info

## Success Criteria

Deployment is successful when:

- ✓ All services are running (health checks pass)
- ✓ Database has pgvector extension
- ✓ Vector indexes exist and perform well (<200ms P95)
- ✓ API endpoints return valid responses
- ✓ Indexer workers processing messages (lag <1000)
- ✓ Metrics being collected by Prometheus
- ✓ Grafana dashboard showing data
- ✓ Alerts configured and tested

## What's Next?

After successful deployment:

1. **Test with Real Data**
   - Create test documents
   - Generate embeddings
   - Run semantic search queries
   - Verify results quality

2. **Performance Tuning**
   - Monitor P95 latency
   - Adjust connection pool sizes
   - Tune vector index parameters
   - Scale indexer workers as needed

3. **Cost Optimization**
   - Monitor OpenAI API usage
   - Configure rulesets for selective processing
   - Consider batch API for bulk operations

4. **Production Hardening**
   - Set up backup strategy
   - Configure disaster recovery
   - Test failover scenarios
   - Document runbooks

---

**Version**: 1.0
**Last Updated**: November 15, 2025
**RFC**: RFC-088 Event-Driven Indexer with Semantic Search
