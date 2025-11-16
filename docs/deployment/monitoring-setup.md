# Monitoring Setup Guide
## RFC-088 Semantic Search and Document Indexer

**Version**: 2.0
**Audience**: SRE Teams, Operations, DevOps Engineers
**Last Updated**: November 15, 2025

---

## Overview

This guide provides comprehensive monitoring setup for the RFC-088 Event-Driven Document Indexer with Semantic Search using Prometheus, Grafana, and standard alerting tools.

**Key Monitoring Areas**:
- Prometheus metrics collection
- Grafana dashboard configuration
- Alert rules and notifications
- Performance indicators and SLIs
- Health check endpoints
- Troubleshooting dashboards

---

## Table of Contents

1. [Prometheus Setup](#prometheus-setup)
2. [Metrics Reference](#metrics-reference)
3. [Grafana Dashboards](#grafana-dashboards)
4. [Alert Rules](#alert-rules)
5. [Health Checks](#health-checks)
6. [Service Level Indicators](#service-level-indicators)
7. [Troubleshooting](#troubleshooting)

---

## Prometheus Setup

### Metrics Endpoint

The application exposes Prometheus metrics at `/metrics`:

```go
// Metrics are exposed on the configured port
// Default: http://localhost:9090/metrics
```

### Prometheus Configuration

Add scrape config to `prometheus.yml`:

```yaml
scrape_configs:
  # Hermes API Server
  - job_name: 'hermes-api'
    scrape_interval: 15s
    scrape_timeout: 10s
    static_configs:
      - targets:
          - 'api-1.internal:9090'
          - 'api-2.internal:9090'
    relabel_configs:
      - source_labels: [__address__]
        target_label: instance

  # Hermes Indexer Workers
  - job_name: 'hermes-indexer'
    scrape_interval: 15s
    static_configs:
      - targets:
          - 'indexer-1.internal:9090'
          - 'indexer-2.internal:9090'
          - 'indexer-3.internal:9090'
          - 'indexer-4.internal:9090'
    relabel_configs:
      - source_labels: [__address__]
        target_label: instance

  # PostgreSQL Exporter
  - job_name: 'postgres'
    scrape_interval: 15s
    static_configs:
      - targets:
          - 'postgres-exporter:9187'

  # Kafka Exporter
  - job_name: 'kafka'
    scrape_interval: 15s
    static_configs:
      - targets:
          - 'kafka-exporter:9308'
```

### Kubernetes Service Discovery

For Kubernetes deployments, use service discovery:

```yaml
scrape_configs:
  - job_name: 'hermes-api'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - hermes-prod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: hermes-api
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_port]
        action: replace
        target_label: __address__
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2

  - job_name: 'hermes-indexer'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - hermes-prod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: hermes-indexer
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
```

### Pod Annotations

Annotate pods for auto-discovery:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: hermes-api
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "9090"
    prometheus.io/path: "/metrics"
```

---

## Metrics Reference

### API Metrics

#### Request Metrics

```
# Total HTTP requests
http_requests_total{method="POST", endpoint="/api/v2/search/semantic", status="200"}

# Request duration histogram
http_request_duration_seconds{method="POST", endpoint="/api/v2/search/semantic"}

# Request size
http_request_size_bytes{method="POST", endpoint="/api/v2/search/semantic"}

# Response size
http_response_size_bytes{method="POST", endpoint="/api/v2/search/semantic"}

# Active requests
http_requests_in_flight{endpoint="/api/v2/search/semantic"}
```

#### Search Metrics

```
# Semantic search operations
search_semantic_total{status="success|error"}
search_semantic_duration_seconds

# Hybrid search operations
search_hybrid_total{status="success|error"}
search_hybrid_duration_seconds

# Similar documents operations
search_similar_total{status="success|error"}
search_similar_duration_seconds

# Search result counts
search_results_count{search_type="semantic|hybrid|similar"}

# Cache hit rate (if caching is implemented)
search_cache_hits_total{search_type="semantic"}
search_cache_misses_total{search_type="semantic"}
```

### Database Metrics

#### Connection Pool

```
# Connection pool statistics
db_connections_open{service="hermes-api"}
db_connections_in_use{service="hermes-api"}
db_connections_idle{service="hermes-api"}
db_connections_wait_count{service="hermes-api"}
db_connections_wait_duration_seconds{service="hermes-api"}
db_connections_max_idle_closed{service="hermes-api"}
db_connections_max_lifetime_closed{service="hermes-api"}
```

#### Query Performance

```
# Query execution time
db_query_duration_seconds{query_type="semantic_search|hybrid_search|lookup"}

# Query counts
db_queries_total{query_type="semantic_search", status="success|error"}

# Rows affected
db_rows_affected{operation="insert|update|delete"}
```

### Indexer Metrics

#### Document Processing

```
# Documents processed
indexer_documents_processed_total{status="success|error"}

# Processing duration
indexer_document_duration_seconds

# Documents per second
rate(indexer_documents_processed_total[1m])

# Current processing queue size
indexer_queue_size
```

#### Embedding Generation

```
# Embeddings generated
indexer_embeddings_generated_total{model="text-embedding-3-small"}

# Embedding generation duration (includes API call)
indexer_embedding_duration_seconds{model="text-embedding-3-small"}

# Embedding cache hits (idempotency)
indexer_embedding_cache_hits_total
indexer_embedding_cache_misses_total

# OpenAI API calls
indexer_openai_api_calls_total{status="success|error|rate_limited"}
indexer_openai_api_duration_seconds
```

#### Kafka Consumer

```
# Messages consumed
kafka_consumer_messages_total{topic="document-revisions"}

# Consumer lag
kafka_consumer_lag{topic="document-revisions", partition="0"}

# Offset commits
kafka_consumer_offset_commits_total{status="success|error"}

# Rebalances
kafka_consumer_rebalances_total
```

### System Metrics

```
# Process CPU usage
process_cpu_seconds_total

# Process memory
process_resident_memory_bytes
process_virtual_memory_bytes

# Go runtime metrics
go_goroutines
go_threads
go_gc_duration_seconds
```

---

## Grafana Dashboards

### API Performance Dashboard

```json
{
  "dashboard": {
    "title": "Hermes API Performance",
    "rows": [
      {
        "title": "Request Rate",
        "panels": [
          {
            "title": "Requests per Second",
            "targets": [
              {
                "expr": "rate(http_requests_total{job=\"hermes-api\"}[1m])"
              }
            ],
            "type": "graph"
          },
          {
            "title": "Requests by Endpoint",
            "targets": [
              {
                "expr": "rate(http_requests_total{job=\"hermes-api\"}[5m])",
                "legendFormat": "{{endpoint}}"
              }
            ],
            "type": "graph"
          }
        ]
      },
      {
        "title": "Latency",
        "panels": [
          {
            "title": "Request Latency (p50, p95, p99)",
            "targets": [
              {
                "expr": "histogram_quantile(0.50, rate(http_request_duration_seconds_bucket[5m]))",
                "legendFormat": "p50"
              },
              {
                "expr": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
                "legendFormat": "p95"
              },
              {
                "expr": "histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))",
                "legendFormat": "p99"
              }
            ],
            "type": "graph",
            "yaxes": [{"format": "s"}]
          }
        ]
      },
      {
        "title": "Error Rate",
        "panels": [
          {
            "title": "Error Rate (%)",
            "targets": [
              {
                "expr": "rate(http_requests_total{status=~\"5..\"}[5m]) / rate(http_requests_total[5m]) * 100"
              }
            ],
            "type": "graph",
            "alert": {
              "conditions": [
                {
                  "evaluator": {"params": [5], "type": "gt"},
                  "query": {"params": ["A", "5m", "now"]},
                  "reducer": {"type": "avg"},
                  "type": "query"
                }
              ],
              "name": "High Error Rate"
            }
          }
        ]
      }
    ]
  }
}
```

### Search Performance Dashboard

**Key Panels**:

```promql
# Semantic Search Latency (p95)
histogram_quantile(0.95,
  rate(search_semantic_duration_seconds_bucket[5m])
)

# Hybrid Search Latency (p95)
histogram_quantile(0.95,
  rate(search_hybrid_duration_seconds_bucket[5m])
)

# Search Requests per Second
rate(search_semantic_total[1m])

# Search Success Rate
rate(search_semantic_total{status="success"}[5m]) /
rate(search_semantic_total[5m]) * 100

# Average Results per Search
avg(search_results_count)
```

### Database Dashboard

**Key Panels**:

```promql
# Connection Pool Utilization
(db_connections_in_use / db_connections_open) * 100

# Connection Wait Count
rate(db_connections_wait_count[5m])

# Query Latency (p95)
histogram_quantile(0.95,
  rate(db_query_duration_seconds_bucket[5m])
)

# Active Connections
db_connections_in_use

# Idle Connections
db_connections_idle
```

### Indexer Dashboard

**Key Panels**:

```promql
# Documents Processed per Second
rate(indexer_documents_processed_total[1m])

# Processing Latency (p95)
histogram_quantile(0.95,
  rate(indexer_document_duration_seconds_bucket[5m])
)

# Kafka Consumer Lag
kafka_consumer_lag

# Embedding Generation Success Rate
rate(indexer_embeddings_generated_total{status="success"}[5m]) /
rate(indexer_embeddings_generated_total[5m]) * 100

# OpenAI API Call Duration
histogram_quantile(0.95,
  rate(indexer_openai_api_duration_seconds_bucket[5m])
)
```

### Complete Dashboard JSON

Save as `hermes-dashboard.json`:

```json
{
  "dashboard": {
    "title": "Hermes - Semantic Search & Indexer",
    "tags": ["hermes", "semantic-search", "rfc-088"],
    "timezone": "utc",
    "panels": [
      {
        "title": "API Request Rate",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
        "targets": [
          {
            "expr": "rate(http_requests_total{job=\"hermes-api\"}[5m])",
            "legendFormat": "{{instance}} - {{endpoint}}"
          }
        ],
        "type": "timeseries"
      },
      {
        "title": "API Latency (p95)",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "{{endpoint}}"
          }
        ],
        "type": "timeseries",
        "fieldConfig": {
          "defaults": {
            "unit": "s"
          }
        }
      },
      {
        "title": "Search Operations",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8},
        "targets": [
          {
            "expr": "rate(search_semantic_total[5m])",
            "legendFormat": "Semantic"
          },
          {
            "expr": "rate(search_hybrid_total[5m])",
            "legendFormat": "Hybrid"
          },
          {
            "expr": "rate(search_similar_total[5m])",
            "legendFormat": "Similar"
          }
        ],
        "type": "timeseries"
      },
      {
        "title": "Database Connection Pool",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8},
        "targets": [
          {
            "expr": "db_connections_open",
            "legendFormat": "Open"
          },
          {
            "expr": "db_connections_in_use",
            "legendFormat": "In Use"
          },
          {
            "expr": "db_connections_idle",
            "legendFormat": "Idle"
          }
        ],
        "type": "timeseries"
      },
      {
        "title": "Indexer Throughput",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 16},
        "targets": [
          {
            "expr": "rate(indexer_documents_processed_total{status=\"success\"}[5m])",
            "legendFormat": "Success"
          },
          {
            "expr": "rate(indexer_documents_processed_total{status=\"error\"}[5m])",
            "legendFormat": "Error"
          }
        ],
        "type": "timeseries"
      },
      {
        "title": "Kafka Consumer Lag",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 16},
        "targets": [
          {
            "expr": "kafka_consumer_lag",
            "legendFormat": "Partition {{partition}}"
          }
        ],
        "type": "timeseries",
        "alert": {
          "name": "High Kafka Lag",
          "conditions": [
            {
              "evaluator": {"params": [1000], "type": "gt"},
              "query": {"params": ["A", "5m", "now"]}
            }
          ]
        }
      }
    ],
    "refresh": "30s",
    "time": {
      "from": "now-1h",
      "to": "now"
    }
  }
}
```

Import in Grafana:
```bash
curl -X POST http://admin:password@grafana:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @hermes-dashboard.json
```

---

## Alert Rules

### Prometheus Alert Rules

Save as `hermes-alerts.yml`:

```yaml
groups:
- name: hermes-api-alerts
  interval: 30s
  rules:
  # High Error Rate
  - alert: HighErrorRate
    expr: |
      (
        rate(http_requests_total{job="hermes-api",status=~"5.."}[5m]) /
        rate(http_requests_total{job="hermes-api"}[5m])
      ) > 0.05
    for: 5m
    labels:
      severity: warning
      service: hermes-api
    annotations:
      summary: "High error rate on {{$labels.instance}}"
      description: "Error rate is {{ $value | humanizePercentage }} (threshold: 5%)"

  # High Latency
  - alert: HighLatency
    expr: |
      histogram_quantile(0.95,
        rate(http_request_duration_seconds_bucket{job="hermes-api"}[5m])
      ) > 1.0
    for: 5m
    labels:
      severity: warning
      service: hermes-api
    annotations:
      summary: "High latency on {{$labels.instance}}"
      description: "P95 latency is {{ $value }}s (threshold: 1s)"

  # Service Down
  - alert: ServiceDown
    expr: up{job="hermes-api"} == 0
    for: 1m
    labels:
      severity: critical
      service: hermes-api
    annotations:
      summary: "Service down: {{$labels.instance}}"
      description: "hermes-api instance {{$labels.instance}} is down"

- name: hermes-database-alerts
  interval: 30s
  rules:
  # Connection Pool Exhaustion
  - alert: ConnectionPoolPressure
    expr: |
      (db_connections_in_use / db_connections_open) > 0.9
    for: 5m
    labels:
      severity: warning
      service: hermes-database
    annotations:
      summary: "Connection pool under pressure"
      description: "Pool utilization is {{ $value | humanizePercentage }}"

  # High Connection Wait Count
  - alert: HighConnectionWaitCount
    expr: rate(db_connections_wait_count[5m]) > 10
    for: 5m
    labels:
      severity: warning
      service: hermes-database
    annotations:
      summary: "High connection wait count"
      description: "{{ $value }} waits per second"

  # Slow Queries
  - alert: SlowQueries
    expr: |
      histogram_quantile(0.95,
        rate(db_query_duration_seconds_bucket[5m])
      ) > 0.5
    for: 5m
    labels:
      severity: warning
      service: hermes-database
    annotations:
      summary: "Slow database queries detected"
      description: "P95 query latency is {{ $value }}s (threshold: 0.5s)"

- name: hermes-indexer-alerts
  interval: 30s
  rules:
  # High Kafka Lag
  - alert: HighKafkaLag
    expr: kafka_consumer_lag > 1000
    for: 5m
    labels:
      severity: warning
      service: hermes-indexer
    annotations:
      summary: "High Kafka consumer lag"
      description: "Lag is {{ $value }} messages (threshold: 1000)"

  # Indexer Processing Errors
  - alert: IndexerProcessingErrors
    expr: |
      rate(indexer_documents_processed_total{status="error"}[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
      service: hermes-indexer
    annotations:
      summary: "High indexer error rate"
      description: "{{ $value }} errors per second"

  # OpenAI API Rate Limit
  - alert: OpenAIRateLimit
    expr: |
      rate(indexer_openai_api_calls_total{status="rate_limited"}[5m]) > 0.01
    for: 2m
    labels:
      severity: warning
      service: hermes-indexer
    annotations:
      summary: "OpenAI API rate limiting detected"
      description: "{{ $value }} rate-limited calls per second"

  # Indexer Throughput Drop
  - alert: LowIndexerThroughput
    expr: |
      rate(indexer_documents_processed_total{status="success"}[5m]) < 0.5
    for: 10m
    labels:
      severity: info
      service: hermes-indexer
    annotations:
      summary: "Low indexer throughput"
      description: "Processing {{ $value }} documents per second (expected >0.5)"
```

Load alerts:
```bash
# Add to prometheus.yml
rule_files:
  - "hermes-alerts.yml"

# Reload Prometheus
curl -X POST http://localhost:9090/-/reload
```

### Alertmanager Configuration

Configure notifications in `alertmanager.yml`:

```yaml
global:
  resolve_timeout: 5m

route:
  receiver: 'default'
  group_by: ['alertname', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  routes:
  # Critical alerts go to PagerDuty
  - match:
      severity: critical
    receiver: pagerduty
    continue: true

  # Warnings go to Slack
  - match:
      severity: warning
    receiver: slack

  # Info goes to email
  - match:
      severity: info
    receiver: email

receivers:
- name: 'default'
  webhook_configs:
  - url: 'http://alertmanager-webhook:5001/'

- name: 'pagerduty'
  pagerduty_configs:
  - service_key: '<PAGERDUTY_SERVICE_KEY>'
    description: '{{ .GroupLabels.alertname }}: {{ .CommonAnnotations.summary }}'

- name: 'slack'
  slack_configs:
  - api_url: '<SLACK_WEBHOOK_URL>'
    channel: '#hermes-alerts'
    title: '{{ .GroupLabels.alertname }}'
    text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'

- name: 'email'
  email_configs:
  - to: 'team@example.com'
    from: 'alertmanager@example.com'
    smarthost: 'smtp.example.com:587'
    auth_username: 'alertmanager@example.com'
    auth_password: '<SMTP_PASSWORD>'
```

---

## Health Checks

### API Health Check

```bash
# Basic health check
curl http://localhost:8080/health

# Response
{
  "status": "healthy",
  "timestamp": "2025-11-15T10:30:00Z",
  "checks": {
    "database": "ok",
    "meilisearch": "ok",
    "openai": "ok"
  }
}
```

### Readiness Check

```bash
# Kubernetes readiness probe
curl http://localhost:8080/ready

# Response when ready
{
  "status": "ready",
  "timestamp": "2025-11-15T10:30:00Z"
}

# Response when not ready (503)
{
  "status": "not ready",
  "reason": "database connection failed"
}
```

### Liveness Check

```bash
# Kubernetes liveness probe
curl http://localhost:8080/live

# Response (200 OK)
OK
```

---

## Service Level Indicators

### Availability SLI

**Target**: 99.9% availability (43 minutes downtime per month)

```promql
# Availability over 30 days
(
  sum(up{job="hermes-api"}) /
  count(up{job="hermes-api"})
) * 100
```

### Latency SLI

**Target**: P95 latency < 200ms

```promql
# P95 latency for semantic search
histogram_quantile(0.95,
  rate(search_semantic_duration_seconds_bucket[30d])
)
```

### Error Rate SLI

**Target**: Error rate < 1%

```promql
# Error rate over 30 days
(
  rate(http_requests_total{status=~"5.."}[30d]) /
  rate(http_requests_total[30d])
) * 100
```

### Success Rate Dashboard

```promql
# Success rate (inverted error rate)
(
  1 - (
    rate(http_requests_total{status=~"5.."}[5m]) /
    rate(http_requests_total[5m])
  )
) * 100
```

---

## Troubleshooting

### High CPU Usage

**Check**:
```promql
rate(process_cpu_seconds_total{job="hermes-api"}[5m])
```

**Investigate**:
1. Check query latency (slow database queries?)
2. Check goroutine count (goroutine leak?)
3. Check GC time (memory pressure?)

### High Memory Usage

**Check**:
```promql
process_resident_memory_bytes{job="hermes-api"}
```

**Investigate**:
1. Check connection pool size
2. Check batch size in indexer
3. Check for memory leaks (heap profiling)

### Slow Searches

**Check**:
```promql
histogram_quantile(0.95,
  rate(search_semantic_duration_seconds_bucket[5m])
)
```

**Investigate**:
1. Database indexes (see [Performance Tuning](./performance-tuning.md))
2. Database connection pool utilization
3. Query patterns (large result sets?)

---

## Additional Resources

- [Performance Tuning Guide](./performance-tuning.md)
- [Best Practices](../guides/best-practices.md)
- [Troubleshooting Guide](../guides/troubleshooting.md)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)

---

*Last Updated: November 15, 2025*
*RFC-088 Implementation*
*Version 2.0*
