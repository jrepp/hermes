# RFC-088 Production Deployment Guide
## Event-Driven Document Indexer with Semantic Search

**Version**: 1.0
**Date**: November 15, 2025
**Status**: Production Ready

---

## Overview

This guide covers deploying the RFC-088 Event-Driven Document Indexer to production, including all dependencies, configuration, and operational considerations.

---

## Architecture Components

### Required Services

1. **PostgreSQL 15+ with pgvector**
   - Primary database
   - Vector similarity search
   - Document embeddings storage

2. **Redpanda/Kafka**
   - Event streaming
   - Document revision events
   - Reliable message delivery

3. **Meilisearch** (optional but recommended)
   - Keyword search
   - Hybrid search capability

4. **OpenAI API** or **Ollama**
   - LLM summaries
   - Embedding generation

---

## PostgreSQL Setup

### Installation

```bash
# Install PostgreSQL 15+
apt-get install postgresql-15

# Install pgvector extension
apt-get install postgresql-15-pgvector
```

### Database Configuration

```sql
-- Create database
CREATE DATABASE hermes;

-- Connect to database
\c hermes

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Verify installation
SELECT * FROM pg_extension WHERE extname = 'vector';
```

### Connection String

```bash
DATABASE_URL="postgresql://hermes:password@localhost:5432/hermes?sslmode=require"
```

### Performance Tuning

```postgresql.conf
# Memory settings for vector operations
shared_buffers = 4GB
effective_cache_size = 12GB
maintenance_work_mem = 2GB
work_mem = 256MB

# Parallelism for vector queries
max_parallel_workers_per_gather = 4
max_parallel_workers = 8
```

---

## Migration

### Run RFC-088 Migrations

```bash
# Run all migrations including RFC-088
hermes migrate up

# Verify RFC-088 tables exist
psql $DATABASE_URL -c "\dt document_*"

# Expected tables:
# - document_summaries
# - document_embeddings
# - document_revision_outbox
```

### Verify pgvector Column

```sql
-- Check embedding_vector column exists
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_name = 'document_embeddings'
AND column_name = 'embedding_vector';

-- Verify indexes
SELECT indexname FROM pg_indexes
WHERE tablename = 'document_embeddings';
```

---

## Redpanda/Kafka Setup

### Redpanda Installation (Recommended)

```bash
# Install Redpanda
curl -1sLf 'https://packages.vectorized.io/nzc4ZYQK3WRGd9sy/redpanda/cfg/setup/bash.deb.sh' | sudo -E bash
apt-get install redpanda

# Start Redpanda
systemctl start redpanda
```

### Configuration (`/etc/redpanda/redpanda.yaml`)

```yaml
redpanda:
  data_directory: /var/lib/redpanda/data
  seed_servers: []
  rpc_server:
    address: 0.0.0.0
    port: 33145
  kafka_api:
    - address: 0.0.0.0
      port: 9092
  admin:
    - address: 0.0.0.0
      port: 9644

pandaproxy:
  pandaproxy_api:
    - address: 0.0.0.0
      port: 8082

schema_registry:
  schema_registry_api:
    - address: 0.0.0.0
      port: 8081
```

### Create Topics

```bash
# Create document revision topic
rpk topic create document-revisions \
  --brokers localhost:9092 \
  --partitions 3 \
  --replicas 1 \
  --config retention.ms=604800000  # 7 days
```

### Kafka Alternative

```bash
# If using Kafka instead
kafka-topics.sh --create \
  --topic document-revisions \
  --bootstrap-server localhost:9092 \
  --partitions 3 \
  --replication-factor 2
```

---

## LLM Provider Setup

### Option 1: OpenAI

```bash
# Set API key
export OPENAI_API_KEY="sk-..."

# Test connectivity
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
```

### Option 2: Ollama (Self-Hosted)

```bash
# Install Ollama
curl https://ollama.ai/install.sh | sh

# Pull models
ollama pull llama2
ollama pull mistral

# Start service
systemctl start ollama
```

---

## Indexer Worker Configuration

### Create `indexer-worker.hcl`

```hcl
# LLM Configuration
llm {
  openai_api_key = env("OPENAI_API_KEY")
  default_model = "gpt-4o-mini"
}

# Embeddings Configuration
embeddings {
  model = "text-embedding-3-small"
  dimensions = 1536
  provider = "openai"
  chunk_size = 8000
}

# Kafka/Redpanda Configuration
kafka {
  brokers = ["redpanda-1:9092", "redpanda-2:9092", "redpanda-3:9092"]
  topic = "document-revisions"
  consumer_group = "indexer-worker"
  enable_tls = true
  sasl_username = env("KAFKA_USERNAME")
  sasl_password = env("KAFKA_PASSWORD")
  sasl_mechanism = "SCRAM-SHA-256"
  security_protocol = "SASL_SSL"
}

# Rulesets
ruleset "published-rfcs" {
  conditions = {
    document_type = "RFC"
    status = "Approved"
  }
  pipeline = ["search_index", "llm_summary", "embeddings"]
}

ruleset "all-documents" {
  conditions = {}
  pipeline = ["search_index"]
}
```

---

## Server Configuration

### Update `config.hcl`

```hcl
# Database
database {
  dsn = env("DATABASE_URL")
  max_open_conns = 25
  max_idle_conns = 10
  conn_max_lifetime = "1h"
}

# Search Provider
search {
  provider = "meilisearch"
  meilisearch_url = "http://meilisearch:7700"
  meilisearch_key = env("MEILISEARCH_KEY")
}

# Workspace Provider
workspace {
  provider = "google"
  google_credentials = env("GOOGLE_CREDENTIALS")
}

# Feature Flags
features {
  enable_semantic_search = true
  enable_hybrid_search = true
}
```

---

## Docker Deployment

### docker-compose.yml

```yaml
version: '3.8'

services:
  postgres:
    image: pgvector/pgvector:pg15
    environment:
      POSTGRES_DB: hermes
      POSTGRES_USER: hermes
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  redpanda:
    image: vectorized/redpanda:latest
    command:
      - redpanda
      - start
      - --smp 1
      - --memory 1G
      - --reserve-memory 0M
      - --overprovisioned
      - --node-id 0
      - --check=false
    ports:
      - "9092:9092"
      - "9644:9644"
    volumes:
      - redpanda-data:/var/lib/redpanda/data

  meilisearch:
    image: getmeili/meilisearch:latest
    environment:
      MEILI_MASTER_KEY: ${MEILI_MASTER_KEY}
      MEILI_ENV: production
    volumes:
      - meilisearch-data:/meili_data
    ports:
      - "7700:7700"

  hermes:
    image: hermes:latest
    depends_on:
      - postgres
      - redpanda
      - meilisearch
    environment:
      DATABASE_URL: postgresql://hermes:${DB_PASSWORD}@postgres:5432/hermes
      OPENAI_API_KEY: ${OPENAI_API_KEY}
      MEILI_MASTER_KEY: ${MEILI_MASTER_KEY}
    ports:
      - "8000:8000"
    volumes:
      - ./config.hcl:/app/config.hcl

  indexer-worker:
    image: hermes-indexer:latest
    depends_on:
      - postgres
      - redpanda
    environment:
      DATABASE_URL: postgresql://hermes:${DB_PASSWORD}@postgres:5432/hermes
      OPENAI_API_KEY: ${OPENAI_API_KEY}
      KAFKA_BROKERS: redpanda:9092
    volumes:
      - ./indexer-worker.hcl:/app/config.hcl
    command: hermes-indexer --config /app/config.hcl

volumes:
  postgres-data:
  redpanda-data:
  meilisearch-data:
```

### Start Services

```bash
# Start all services
docker-compose up -d

# Check logs
docker-compose logs -f hermes indexer-worker

# Check health
curl http://localhost:8000/health
```

---

## Monitoring

### Key Metrics

**Database Metrics**:
- Connection pool usage
- Query latency (especially vector queries)
- Table sizes (document_embeddings)
- Index usage (idx_embeddings_vector_cosine)

**Indexer Metrics**:
- Messages consumed/sec
- Processing latency
- Error rate
- LLM API latency/cost
- Embeddings generation rate

**Search Metrics**:
- Semantic search query latency
- Hybrid search performance
- Result relevance (user feedback)

### Prometheus Metrics

```yaml
# /metrics endpoint includes:
- hermes_indexer_messages_processed_total
- hermes_indexer_processing_duration_seconds
- hermes_llm_api_requests_total
- hermes_llm_api_duration_seconds
- hermes_embeddings_generated_total
- hermes_semantic_search_queries_total
- hermes_semantic_search_duration_seconds
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "RFC-088 Indexer Performance",
    "panels": [
      {
        "title": "Processing Throughput",
        "targets": ["rate(hermes_indexer_messages_processed_total[5m])"]
      },
      {
        "title": "LLM API Latency",
        "targets": ["histogram_quantile(0.95, hermes_llm_api_duration_seconds)"]
      },
      {
        "title": "Search Query Performance",
        "targets": ["histogram_quantile(0.99, hermes_semantic_search_duration_seconds)"]
      }
    ]
  }
}
```

---

## Scaling

### Horizontal Scaling

**Indexer Workers**:
```bash
# Scale up indexer workers
docker-compose up -d --scale indexer-worker=5
```

Each worker will consume from different partitions.

**Hermes Servers**:
```bash
# Scale API servers
docker-compose up -d --scale hermes=3
```

Use load balancer (nginx, HAProxy) in front.

### Vertical Scaling

**PostgreSQL**:
- Increase `shared_buffers` for vector operations
- Add more CPUs for parallel vector queries
- Increase `work_mem` for sorting

**Redpanda**:
- Increase memory (`--memory 4G`)
- Add more partitions for parallelism

---

## Cost Optimization

### OpenAI API Costs

**Embeddings** (text-embedding-3-small):
- Cost: $0.02 per 1M tokens
- Average document: ~500 tokens
- 10K documents/day: ~$0.10/day

**LLM Summaries** (gpt-4o-mini):
- Cost: $0.15 per 1M input tokens
- Average document: ~1000 tokens
- 10K documents/day: ~$1.50/day

**Total Estimated**: ~$50/month for 10K docs/day

### Optimization Strategies

1. **Idempotency** - Content hash prevents re-processing
2. **Selective Processing** - Use rulesets to only process important docs
3. **Caching** - Cache embeddings for unchanged content
4. **Batch Processing** - Use batch APIs when possible

---

## Security

### API Keys

```bash
# Store in secrets manager (AWS Secrets Manager, HashiCorp Vault)
aws secretsmanager create-secret \
  --name hermes/openai-api-key \
  --secret-string "$OPENAI_API_KEY"
```

### Database Encryption

```postgresql.conf
# Enable SSL
ssl = on
ssl_cert_file = '/path/to/server.crt'
ssl_key_file = '/path/to/server.key'
```

### Network Security

```bash
# Firewall rules
- Allow 8000/tcp from load balancer
- Allow 5432/tcp from hermes/indexer only
- Allow 9092/tcp from indexer only
- Deny all other traffic
```

---

## Troubleshooting

### Slow Vector Queries

```sql
-- Check index usage
EXPLAIN ANALYZE
SELECT document_id, 1 - (embedding_vector <=> '[0.1,0.2,...]') as similarity
FROM document_embeddings
ORDER BY embedding_vector <=> '[0.1,0.2,...]'
LIMIT 10;

-- Rebuild index if needed
REINDEX INDEX idx_embeddings_vector_cosine;
```

### High Memory Usage

```bash
# Check PostgreSQL memory
SELECT pg_size_pretty(pg_database_size('hermes'));
SELECT pg_size_pretty(pg_total_relation_size('document_embeddings'));

# Vacuum and analyze
VACUUM ANALYZE document_embeddings;
```

### Indexer Lag

```bash
# Check Kafka lag
rpk group describe indexer-worker

# Increase workers
docker-compose up -d --scale indexer-worker=10
```

---

## Backup and Recovery

### PostgreSQL Backup

```bash
# Daily backup
pg_dump hermes | gzip > hermes_$(date +%Y%m%d).sql.gz

# Backup embeddings table separately (large)
pg_dump -t document_embeddings hermes | gzip > embeddings_$(date +%Y%m%d).sql.gz
```

### Disaster Recovery

```bash
# Restore database
gunzip < hermes_backup.sql.gz | psql hermes

# Re-run migrations
hermes migrate up

# Rebuild embeddings from outbox (if needed)
# Indexer will automatically process any pending events
```

---

## Operational Runbook

### Daily Tasks

1. Check processing lag
2. Monitor API costs
3. Review error logs
4. Check disk space (embeddings table grows)

### Weekly Tasks

1. Vacuum database
2. Review slow queries
3. Check index health
4. Update models (if new versions available)

### Monthly Tasks

1. Review and optimize rulesets
2. Analyze search quality metrics
3. Cost optimization review
4. Capacity planning

---

## Support Contacts

- **Database Issues**: DBA team
- **Kafka/Redpanda**: Platform team
- **OpenAI API**: External vendor
- **Application**: Hermes dev team

---

**Document Version**: 1.0
**Last Updated**: November 15, 2025
**Next Review**: December 15, 2025
