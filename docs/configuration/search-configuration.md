# Search Configuration Guide
## RFC-088 Semantic Search and Document Indexer

**Version**: 2.0
**Audience**: System Administrators, DevOps Engineers
**Last Updated**: November 15, 2025

---

## Overview

This guide explains how to configure the RFC-088 Event-Driven Document Indexer with Semantic Search, including rulesets for selective indexing, embedding model selection, pipeline configuration, and worker deployment.

**Key Configuration Areas**:
- Rulesets for document filtering
- Embedding model selection
- Indexer pipeline configuration
- Kafka/Redpanda setup
- Worker deployment and scaling

---

## Table of Contents

1. [Ruleset Configuration](#ruleset-configuration)
2. [Embedding Models](#embedding-models)
3. [Pipeline Configuration](#pipeline-configuration)
4. [Kafka/Redpanda Setup](#kafkaredpanda-setup)
5. [Worker Deployment](#worker-deployment)
6. [Configuration Examples](#configuration-examples)
7. [Troubleshooting](#troubleshooting)

---

## Ruleset Configuration

Rulesets control which documents are indexed and embedded. This is critical for cost optimization and relevance.

### Ruleset Structure

```hcl
indexer {
  rulesets = [
    {
      name        = "documentation"
      description = "Index all documentation files"
      enabled     = true

      rules {
        # File patterns to include
        include = ["*.md", "*.rst", "docs/**/*"]

        # File patterns to exclude
        exclude = ["**/test/**", "**/tmp/**", "**/.git/**"]

        # Document types to include
        include_document_types = ["Documentation", "Guide", "README"]

        # Document types to exclude
        exclude_document_types = ["Binary", "Archive"]

        # Minimum document size (bytes)
        min_size = 100

        # Maximum document size (bytes)
        max_size = 10485760  # 10MB

        # Process only if modified within last N days
        max_age_days = 365
      }

      # Embedding configuration for this ruleset
      embedding {
        model      = "text-embedding-3-small"
        dimensions = 1536
        chunk_size = 8000  # Characters per chunk
        chunk_overlap = 200
      }
    }
  ]
}
```

### Example Rulesets

#### 1. Documentation Only

Index only documentation files:

```hcl
indexer {
  rulesets = [
    {
      name        = "docs-only"
      description = "Index documentation files only"
      enabled     = true

      rules {
        include = [
          "*.md",
          "*.rst",
          "*.txt",
          "docs/**/*",
          "*.pdf"
        ]

        exclude = [
          "**/node_modules/**",
          "**/vendor/**",
          "**/.git/**",
          "*test*"
        ]

        min_size = 100        # Skip empty files
        max_size = 5242880    # 5MB max
      }

      embedding {
        model      = "text-embedding-3-small"
        dimensions = 1536
      }
    }
  ]
}
```

#### 2. Code and Documentation

Index both code and documentation:

```hcl
indexer {
  rulesets = [
    {
      name        = "code-and-docs"
      description = "Index source code and documentation"
      enabled     = true

      rules {
        include = [
          "*.go",
          "*.py",
          "*.js",
          "*.ts",
          "*.java",
          "*.md",
          "*.rst"
        ]

        exclude = [
          "**/test/**",
          "**/*_test.go",
          "**/vendor/**",
          "**/node_modules/**"
        ]

        max_size = 1048576  # 1MB max for code files
      }

      embedding {
        model      = "text-embedding-3-small"
        dimensions = 1536
        chunk_size = 4000  # Smaller chunks for code
      }
    }
  ]
}
```

#### 3. Multi-Ruleset Configuration

Different rules for different document types:

```hcl
indexer {
  rulesets = [
    # Ruleset 1: RFCs and design docs
    {
      name        = "rfcs"
      description = "Index RFC documents"
      enabled     = true

      rules {
        include_document_types = ["RFC"]
        include                = ["docs/rfc/**/*.md"]
        min_size               = 1000  # RFCs are substantial
      }

      embedding {
        model      = "text-embedding-3-large"  # Higher quality for RFCs
        dimensions = 3072
        chunk_size = 12000  # Larger chunks
      }
    },

    # Ruleset 2: General documentation
    {
      name        = "general-docs"
      description = "Index general documentation"
      enabled     = true

      rules {
        include = ["docs/**/*.md"]
        exclude = ["docs/rfc/**"]  # Exclude RFCs (handled by ruleset 1)
      }

      embedding {
        model      = "text-embedding-3-small"  # Standard quality
        dimensions = 1536
      }
    },

    # Ruleset 3: Source code
    {
      name        = "source-code"
      description = "Index source code"
      enabled     = false  # Disabled by default

      rules {
        include = ["**/*.go", "**/*.py"]
        exclude = ["**/*_test.go", "**/vendor/**"]
      }

      embedding {
        model      = "text-embedding-3-small"
        dimensions = 1536
        chunk_size = 2000  # Smaller chunks for code
      }
    }
  ]
}
```

### Ruleset Best Practices

**Cost Optimization**:
- Use `exclude` patterns to skip non-relevant files
- Set reasonable `max_size` limits
- Use `max_age_days` to skip old documents
- Start with narrow rulesets, expand gradually

**Performance**:
- Use specific `include` patterns instead of `**/*`
- Exclude large directories (`node_modules`, `vendor`, `.git`)
- Set appropriate `chunk_size` for document type

**Relevance**:
- Create separate rulesets for different document types
- Use `include_document_types` for fine-grained control
- Test rulesets with sample documents before production

---

## Embedding Models

### Available Models

| Model | Dimensions | Cost per 1M tokens | Performance | Use Case |
|-------|------------|-------------------|-------------|----------|
| **text-embedding-3-small** | 1536 | $0.02 | Fast | General purpose (recommended) |
| **text-embedding-3-large** | 3072 | $0.13 | Slower | High accuracy requirements |
| text-embedding-ada-002 | 1536 | $0.10 | Fast | Legacy (use 3-small instead) |

### Model Selection Guide

#### Use text-embedding-3-small (Recommended)

**Best for**:
- General documentation
- Product guides
- API documentation
- Blog posts
- Most use cases

**Configuration**:
```hcl
embedding {
  model      = "text-embedding-3-small"
  dimensions = 1536
  chunk_size = 8000
}
```

**Pros**:
- 80% cheaper than ada-002
- Excellent quality
- Fast generation
- Lower storage requirements

#### Use text-embedding-3-large

**Best for**:
- Critical documents (RFCs, legal, compliance)
- Multi-language content
- Highly technical content
- When accuracy is paramount

**Configuration**:
```hcl
embedding {
  model      = "text-embedding-3-large"
  dimensions = 3072
  chunk_size = 12000
}
```

**Pros**:
- Highest accuracy
- Better multi-language support
- Better for complex queries

**Cons**:
- 6.5x more expensive
- 2x storage requirements
- Slower generation

### Dimension Reduction

Reduce dimensions for lower storage costs:

```hcl
embedding {
  model      = "text-embedding-3-small"
  dimensions = 512  # Reduce from 1536 to 512 (3x storage savings)
}
```

**Trade-offs**:
- **768 dimensions**: 10-15% quality loss, 50% storage savings
- **512 dimensions**: 20-30% quality loss, 66% storage savings
- **256 dimensions**: 40-50% quality loss, 83% storage savings

**Recommendation**: Use full dimensions (1536 or 3072) unless storage is a critical constraint.

### Chunking Configuration

#### Chunk Size

Controls how documents are split:

```hcl
embedding {
  chunk_size    = 8000   # Characters per chunk
  chunk_overlap = 200    # Overlap between chunks
}
```

**Guidelines**:

| Document Type | Chunk Size | Reasoning |
|--------------|------------|-----------|
| Short docs (<5KB) | 8000 | Keep document whole |
| Long docs (>20KB) | 4000-8000 | Balance context and granularity |
| Code | 2000-4000 | Smaller logical units |
| Legal/contracts | 12000 | Maintain context |

#### Chunk Overlap

Prevents loss of context at chunk boundaries:

```hcl
chunk_overlap = 200  # 200 characters overlap
```

**Guidelines**:
- **0**: No overlap (not recommended)
- **100-200**: Standard overlap (recommended)
- **400-800**: High overlap (for critical context)

---

## Pipeline Configuration

The indexer pipeline processes documents from Kafka to database.

### Basic Configuration

```hcl
indexer {
  # Worker configuration
  workers = 4  # Number of concurrent workers

  # Kafka configuration
  kafka {
    brokers = ["localhost:9092"]
    topic   = "document-revisions"
    group   = "hermes-indexer"
  }

  # Database configuration
  database {
    host     = "localhost"
    port     = 5432
    user     = "hermes"
    password = "${env.DB_PASSWORD}"
    dbname   = "hermes"
    sslmode  = "require"

    # Connection pool
    max_idle_conns     = 10
    max_open_conns     = 25
    conn_max_lifetime  = "5m"
  }

  # OpenAI configuration
  openai {
    api_key = "${env.OPENAI_API_KEY}"
    timeout = "30s"
  }

  # Processing options
  batch_size       = 10    # Documents per batch
  retry_attempts   = 3     # Retry failed documents
  retry_delay      = "5s"  # Delay between retries
  checkpoint_interval = "10s"  # Kafka commit interval
}
```

### Production Configuration

```hcl
indexer {
  workers = 8  # Higher concurrency for production

  kafka {
    brokers = [
      "kafka-1.internal:9092",
      "kafka-2.internal:9092",
      "kafka-3.internal:9092"
    ]
    topic           = "document-revisions"
    group           = "hermes-indexer-prod"
    session_timeout = "30s"
    heartbeat_interval = "3s"
  }

  database {
    host     = "db.internal.example.com"
    port     = 5432
    user     = "hermes_indexer"
    password = "${env.DB_PASSWORD}"
    dbname   = "hermes_prod"
    sslmode  = "verify-full"
    sslrootcert = "/etc/ssl/certs/ca-bundle.crt"

    # Larger pool for production
    max_idle_conns     = 25
    max_open_conns     = 50
    conn_max_lifetime  = "5m"
    conn_max_idle_time = "2m"
  }

  openai {
    api_key         = "${env.OPENAI_API_KEY}"
    timeout         = "60s"
    max_retries     = 3
    retry_delay     = "1s"
    organization_id = "${env.OPENAI_ORG_ID}"
  }

  # Optimized processing
  batch_size          = 20
  retry_attempts      = 5
  retry_delay         = "10s"
  checkpoint_interval = "5s"

  # Monitoring
  metrics_enabled = true
  metrics_port    = 9090

  # Logging
  log_level = "info"
  log_format = "json"
}
```

### Performance Tuning

#### Worker Count

Choose based on available resources:

```hcl
# Low resources (2 CPU, 4GB RAM)
workers = 2

# Medium resources (4 CPU, 8GB RAM)
workers = 4

# High resources (8 CPU, 16GB RAM)
workers = 8

# Very high resources (16 CPU, 32GB RAM)
workers = 16
```

**Rule of thumb**: `workers = CPU_cores / 2`

#### Batch Size

Balance throughput and memory:

```hcl
# Conservative (low memory)
batch_size = 5

# Standard (recommended)
batch_size = 10

# Aggressive (high throughput)
batch_size = 20
```

**Trade-off**: Larger batches = higher throughput but more memory

#### Checkpoint Interval

How often to commit Kafka offsets:

```hcl
# Frequent commits (low data loss risk)
checkpoint_interval = "1s"

# Standard (recommended)
checkpoint_interval = "10s"

# Infrequent commits (higher throughput)
checkpoint_interval = "30s"
```

**Trade-off**: Shorter intervals = less reprocessing on restart, slightly lower throughput

---

## Kafka/Redpanda Setup

### Topic Creation

```bash
# Create topic with appropriate partitions
kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic document-revisions \
  --partitions 8 \
  --replication-factor 3 \
  --config retention.ms=604800000  # 7 days

# Verify topic
kafka-topics --describe \
  --bootstrap-server localhost:9092 \
  --topic document-revisions
```

### Partition Count

Choose based on expected throughput:

| Documents/sec | Partitions | Workers |
|--------------|------------|---------|
| <10 | 2 | 2 |
| 10-50 | 4 | 4 |
| 50-200 | 8 | 8 |
| 200+ | 16+ | 16+ |

**Rule of thumb**: `partitions = workers` for even distribution

### Retention Policy

```bash
# Short retention (cost-conscious)
--config retention.ms=86400000  # 1 day

# Medium retention (recommended)
--config retention.ms=604800000  # 7 days

# Long retention (audit/debugging)
--config retention.ms=2592000000  # 30 days
```

### Consumer Group

Each deployment should have its own consumer group:

```hcl
kafka {
  group = "hermes-indexer-prod"  # Production
  # group = "hermes-indexer-staging"  # Staging
  # group = "hermes-indexer-dev"  # Development
}
```

**Important**: Different groups can process the same messages independently.

---

## Worker Deployment

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hermes-indexer
  labels:
    app: hermes-indexer
spec:
  replicas: 4  # Start with 4 workers
  selector:
    matchLabels:
      app: hermes-indexer
  template:
    metadata:
      labels:
        app: hermes-indexer
    spec:
      containers:
      - name: indexer
        image: hermes-indexer:v2.0.0
        command: ["hermes-indexer"]
        args:
          - "--config=/etc/hermes/indexer.hcl"

        env:
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: hermes-secrets
              key: db-password

        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: hermes-secrets
              key: openai-api-key

        resources:
          requests:
            cpu: "1000m"
            memory: "2Gi"
          limits:
            cpu: "2000m"
            memory: "4Gi"

        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10

        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5

        volumeMounts:
        - name: config
          mountPath: /etc/hermes
          readOnly: true

      volumes:
      - name: config
        configMap:
          name: hermes-indexer-config
```

### Auto-Scaling

Scale workers based on Kafka lag:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: hermes-indexer-scaler
spec:
  scaleTargetRef:
    name: hermes-indexer
  minReplicaCount: 2
  maxReplicaCount: 16
  triggers:
  - type: kafka
    metadata:
      bootstrapServers: kafka-1.internal:9092
      consumerGroup: hermes-indexer-prod
      topic: document-revisions
      lagThreshold: "100"  # Scale up if lag > 100 messages
```

### Docker Compose

For development or small deployments:

```yaml
version: '3.8'

services:
  indexer:
    image: hermes-indexer:v2.0.0
    command: hermes-indexer --config=/etc/hermes/indexer.hcl
    environment:
      - DB_PASSWORD=${DB_PASSWORD}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    volumes:
      - ./configs/indexer.hcl:/etc/hermes/indexer.hcl:ro
    depends_on:
      - postgres
      - kafka
    restart: unless-stopped
    deploy:
      replicas: 4
      resources:
        limits:
          cpus: '2'
          memory: 4G
```

---

## Configuration Examples

### Example 1: Development Setup

```hcl
# config/indexer-dev.hcl

indexer {
  workers = 2

  kafka {
    brokers = ["localhost:9092"]
    topic   = "document-revisions"
    group   = "hermes-indexer-dev"
  }

  database {
    host     = "localhost"
    port     = 5432
    user     = "hermes"
    password = "dev_password"
    dbname   = "hermes_dev"
    sslmode  = "disable"

    max_idle_conns = 5
    max_open_conns = 10
  }

  openai {
    api_key = "${env.OPENAI_API_KEY}"
    timeout = "30s"
  }

  rulesets = [
    {
      name    = "test-docs"
      enabled = true

      rules {
        include = ["docs/**/*.md"]
        max_size = 1048576  # 1MB
      }

      embedding {
        model      = "text-embedding-3-small"
        dimensions = 1536
      }
    }
  ]

  batch_size  = 5
  log_level   = "debug"
  log_format  = "text"
}
```

### Example 2: Production Setup

```hcl
# config/indexer-prod.hcl

indexer {
  workers = 8

  kafka {
    brokers = [
      "kafka-1.internal:9092",
      "kafka-2.internal:9092",
      "kafka-3.internal:9092"
    ]
    topic           = "document-revisions"
    group           = "hermes-indexer-prod"
    session_timeout = "30s"
  }

  database {
    host     = "db.internal.example.com"
    port     = 5432
    user     = "hermes_indexer"
    password = "${env.DB_PASSWORD}"
    dbname   = "hermes_prod"
    sslmode  = "verify-full"

    max_idle_conns     = 25
    max_open_conns     = 50
    conn_max_lifetime  = "5m"
  }

  openai {
    api_key     = "${env.OPENAI_API_KEY}"
    timeout     = "60s"
    max_retries = 3
  }

  rulesets = [
    {
      name        = "documentation"
      description = "All documentation"
      enabled     = true

      rules {
        include = ["docs/**/*.md", "*.md"]
        exclude = ["**/test/**", "**/tmp/**"]
        min_size = 100
        max_size = 10485760  # 10MB
      }

      embedding {
        model      = "text-embedding-3-small"
        dimensions = 1536
        chunk_size = 8000
      }
    }
  ]

  batch_size          = 20
  retry_attempts      = 5
  checkpoint_interval = "5s"

  metrics_enabled = true
  metrics_port    = 9090

  log_level  = "info"
  log_format = "json"
}
```

---

## Troubleshooting

### Problem: Documents Not Being Indexed

**Check**:
1. Verify ruleset matches document:
```bash
# Check if document matches include patterns
hermes-indexer --config indexer.hcl --dry-run --file path/to/doc.md
```

2. Check Kafka messages:
```bash
kafka-console-consumer --bootstrap-server localhost:9092 \
  --topic document-revisions \
  --from-beginning \
  --max-messages 10
```

3. Check worker logs:
```bash
kubectl logs -f deployment/hermes-indexer
```

### Problem: High Kafka Lag

**Diagnosis**:
```bash
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group hermes-indexer-prod \
  --describe
```

**Solutions**:
1. Increase worker count
2. Increase batch size
3. Optimize database queries (add indexes)
4. Check OpenAI API rate limits

### Problem: OpenAI API Rate Limits

**Symptoms**: Errors like "rate_limit_exceeded"

**Solutions**:
1. Add retry logic with exponential backoff:
```hcl
openai {
  max_retries = 5
  retry_delay = "2s"
}
```

2. Reduce worker count temporarily
3. Increase batch size to reduce API calls
4. Contact OpenAI for higher limits

### Problem: High Memory Usage

**Diagnosis**:
```bash
kubectl top pods -l app=hermes-indexer
```

**Solutions**:
1. Reduce batch size
2. Reduce chunk size
3. Reduce worker count
4. Increase memory limits

---

## Additional Resources

- [Performance Tuning Guide](../deployment/performance-tuning.md)
- [Best Practices](./best-practices.md)
- [API Documentation](../api/SEMANTIC-SEARCH-API.md)
- [Troubleshooting Guide](./troubleshooting.md)

---

*Last Updated: November 15, 2025*
*RFC-088 Implementation*
*Version 2.0*
