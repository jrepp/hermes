# RFC-088 Deployment Quick Start Guide

**⚡ Get RFC-088 semantic search deployed in 15 minutes**

---

## Prerequisites

- Docker & Docker Compose installed
- PostgreSQL client (psql) installed
- 5GB free disk space
- OpenAI API key

---

## Option 1: One-Command Deployment (Recommended)

```bash
# Clone and navigate to project
cd /path/to/hermes

# Run complete deployment
./scripts/deployment/deploy-all-phases.sh
```

**What it does:**
1. ✅ Generates configuration files
2. ✅ Sets up database with pgvector
3. ✅ Deploys all services (Docker Compose)
4. ✅ Runs validation tests
5. ✅ Sets up monitoring (Prometheus, Grafana)

**Duration:** ~15-30 minutes

---

## Option 2: Step-by-Step Deployment

### Step 1: Generate Configuration (2 min)
```bash
./scripts/deployment/phase1-config-setup.sh
```

### Step 2: Set Secrets (1 min)
```bash
cd config-production
cp .env.template .env
# Edit .env and set:
# - OPENAI_API_KEY=sk-...
# - POSTGRES_PASSWORD=...
# - MEILISEARCH_KEY=...
```

### Step 3: Setup Database (5 min)
```bash
./scripts/deployment/phase1-database-setup.sh
```

### Step 4: Deploy Services (5 min)
```bash
./scripts/deployment/deploy-docker.sh
```

### Step 5: Validate (3 min)
```bash
./scripts/deployment/phase2-validation.sh
./scripts/deployment/phase2-api-tests.sh
```

### Step 6: Setup Monitoring (2 min)
```bash
./scripts/deployment/phase3-monitoring.sh
cd monitoring-config
docker-compose -f docker-compose.monitoring.yml up -d
```

---

## Access Your Deployment

After successful deployment:

| Service | URL | Credentials |
|---------|-----|-------------|
| Hermes API | http://localhost:8000 | - |
| API Health | http://localhost:8000/health | - |
| Metrics | http://localhost:9090/metrics | - |
| Grafana | http://localhost:3000 | admin / admin |
| Prometheus | http://localhost:9090 | - |
| Alertmanager | http://localhost:9093 | - |

---

## Test Semantic Search

```bash
# Test semantic search (replace YOUR_TOKEN with actual token)
curl -X POST http://localhost:8000/api/v2/search/semantic \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "query": "kubernetes deployment strategies",
    "limit": 10
  }'
```

---

## Scale Indexer Workers

```bash
# Scale to 10 workers
docker-compose -f config-production/docker-compose.production.yml \
  up -d --scale indexer-worker=10
```

---

## Check Status

```bash
# View all services
docker-compose -f config-production/docker-compose.production.yml ps

# View logs
docker-compose -f config-production/docker-compose.production.yml logs -f hermes
docker-compose -f config-production/docker-compose.production.yml logs -f indexer-worker

# Check health
curl http://localhost:8000/health
```

---

## Troubleshooting

### Services not starting?
```bash
# Check logs
docker-compose -f config-production/docker-compose.production.yml logs

# Verify environment variables
cat config-production/.env
```

### Database connection errors?
```bash
# Test database connectivity
psql -h localhost -p 5432 -U hermes -d hermes -c "SELECT 1;"

# Check pgvector
psql -h localhost -p 5432 -U hermes -d hermes -c "SELECT * FROM pg_extension WHERE extname = 'vector';"
```

### No search results?
```bash
# Check embeddings exist
psql -h localhost -p 5432 -U hermes -d hermes -c "SELECT COUNT(*) FROM document_embeddings;"

# Check indexer processing
docker logs hermes-indexer

# Check Kafka lag
rpk group describe hermes-indexer-workers --brokers localhost:19092
```

---

## Stop Services

```bash
# Stop all services
docker-compose -f config-production/docker-compose.production.yml down

# Stop and remove volumes (CAUTION: deletes all data)
docker-compose -f config-production/docker-compose.production.yml down -v

# Stop monitoring
cd monitoring-config
docker-compose -f docker-compose.monitoring.yml down
```

---

## Environment Variables Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OPENAI_API_KEY` | Yes | - | OpenAI API key for embeddings |
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `POSTGRES_PASSWORD` | Yes | - | PostgreSQL password |
| `MEILISEARCH_KEY` | Yes | - | Meilisearch master key |
| `REDPANDA_BROKERS` | No | localhost:9092 | Kafka broker addresses |
| `HERMES_BASE_URL` | No | http://localhost:8000 | Base URL for web UI |

---

## Configuration Files

| File | Purpose |
|------|---------|
| `config-production/config.hcl` | Main Hermes server config |
| `config-production/indexer-worker.hcl` | Indexer worker config |
| `config-production/.env` | Environment variables (secrets) |
| `config-production/docker-compose.production.yml` | Docker deployment |
| `monitoring-config/prometheus/prometheus.yml` | Prometheus scrape config |
| `monitoring-config/prometheus/alerts.yml` | Alert rules (40+ alerts) |
| `monitoring-config/grafana/rfc-088-dashboard.json` | Grafana dashboard |

---

## Common Commands

### Deployment
```bash
# Complete deployment
./scripts/deployment/deploy-all-phases.sh

# Deploy without monitoring
SKIP_PHASE3=true ./scripts/deployment/deploy-all-phases.sh

# Non-interactive (CI/CD)
AUTO_CONFIRM=true ./scripts/deployment/deploy-all-phases.sh
```

### Validation
```bash
# Run all validations
./scripts/deployment/phase2-validation.sh

# Run API tests
./scripts/deployment/phase2-api-tests.sh

# Basic validation
./scripts/validate-production-deployment.sh
```

### Monitoring
```bash
# Setup monitoring
./scripts/deployment/phase3-monitoring.sh

# View Grafana dashboard
open http://localhost:3000

# View Prometheus metrics
open http://localhost:9090
```

### Scaling
```bash
# Scale indexer workers
docker-compose -f config-production/docker-compose.production.yml \
  up -d --scale indexer-worker=10

# Check worker status
docker-compose -f config-production/docker-compose.production.yml ps indexer-worker
```

---

## Next Steps

After successful deployment:

1. **✅ Verify Services** - Check all health endpoints
2. **✅ Create Test Documents** - Generate sample data
3. **✅ Test Search** - Run semantic and hybrid queries
4. **✅ Monitor Performance** - Open Grafana dashboard
5. **✅ Configure Alerts** - Set up Slack/email notifications

---

## Full Documentation

For comprehensive documentation, see:

- **Deployment Guide**: `scripts/deployment/README.md` (13KB)
- **Summary**: `scripts/deployment/DEPLOYMENT-SUMMARY.md` (11KB)
- **API Docs**: `docs/api/SEMANTIC-SEARCH-API.md`
- **Performance Guide**: `docs/deployment/performance-tuning.md`
- **Troubleshooting**: `docs/guides/troubleshooting.md`

---

## Support

If you encounter issues:

1. Check script output for specific errors
2. Review logs: `docker-compose logs -f`
3. Run validation: `./scripts/deployment/phase2-validation.sh`
4. Check documentation in `scripts/deployment/README.md`
5. Review RFC-088 docs in `docs-internal/rfc/`

---

**Ready to deploy?** Run `./scripts/deployment/deploy-all-phases.sh` and follow the prompts! 🚀
