#!/bin/bash
# RFC-088 Phase 3: Monitoring Setup Script
# Automated setup of Prometheus, Grafana, and alerting for RFC-088

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
OUTPUT_DIR="${OUTPUT_DIR:-./monitoring-config}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"
GRAFANA_USER="${GRAFANA_USER:-admin}"
GRAFANA_PASSWORD="${GRAFANA_PASSWORD:-admin}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090}"
ALERTMANAGER_URL="${ALERTMANAGER_URL:-http://localhost:9093}"

echo "================================================"
echo "RFC-088 Phase 3: Monitoring Setup"
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
info "Creating monitoring configuration directory..."
mkdir -p "$OUTPUT_DIR"/{prometheus,grafana,alertmanager}
success "Directories created"

# Step 1: Create Prometheus Configuration
info "Step 1: Creating Prometheus configuration..."

cat > "$OUTPUT_DIR/prometheus/prometheus.yml" << 'EOF'
# Prometheus Configuration for RFC-088 Monitoring

global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    cluster: 'hermes-production'
    environment: 'production'

# Alertmanager configuration
alerting:
  alertmanagers:
    - static_configs:
        - targets:
            - alertmanager:9093

# Load alert rules
rule_files:
  - 'alerts.yml'

# Scrape configurations
scrape_configs:
  # Hermes API Server metrics
  - job_name: 'hermes-api'
    static_configs:
      - targets:
          - 'hermes:9090'
    metrics_path: '/metrics'
    scrape_interval: 10s

  # Hermes Indexer Worker metrics
  - job_name: 'hermes-indexer'
    static_configs:
      - targets:
          - 'indexer-worker:9091'
    metrics_path: '/metrics'
    scrape_interval: 10s

  # PostgreSQL metrics (if using postgres_exporter)
  - job_name: 'postgres'
    static_configs:
      - targets:
          - 'postgres-exporter:9187'
    scrape_interval: 30s

  # Redpanda metrics
  - job_name: 'redpanda'
    static_configs:
      - targets:
          - 'redpanda:9644'
    metrics_path: '/metrics'
    scrape_interval: 30s

  # Node exporter (system metrics)
  - job_name: 'node'
    static_configs:
      - targets:
          - 'node-exporter:9100'
    scrape_interval: 30s
EOF

success "Created: $OUTPUT_DIR/prometheus/prometheus.yml"

# Step 2: Create Prometheus Alert Rules
info "Step 2: Creating Prometheus alert rules..."

cat > "$OUTPUT_DIR/prometheus/alerts.yml" << 'EOF'
# Prometheus Alert Rules for RFC-088

groups:
  - name: hermes_api
    interval: 30s
    rules:
      # API Error Rate
      - alert: HighAPIErrorRate
        expr: |
          (
            sum(rate(http_requests_total{job="hermes-api",status=~"5.."}[5m]))
            /
            sum(rate(http_requests_total{job="hermes-api"}[5m]))
          ) > 0.01
        for: 5m
        labels:
          severity: warning
          component: api
        annotations:
          summary: "High API error rate (instance {{ $labels.instance }})"
          description: "API error rate is {{ $value | humanizePercentage }} (threshold: 1%)"

      # API High Latency
      - alert: HighAPILatency
        expr: |
          histogram_quantile(0.95,
            sum(rate(http_request_duration_seconds_bucket{job="hermes-api"}[5m])) by (le, endpoint)
          ) > 1.0
        for: 10m
        labels:
          severity: warning
          component: api
        annotations:
          summary: "High API latency (endpoint {{ $labels.endpoint }})"
          description: "P95 latency is {{ $value }}s (threshold: 1s)"

      # Semantic Search Error Rate
      - alert: HighSemanticSearchErrorRate
        expr: |
          (
            sum(rate(hermes_semantic_search_errors_total[5m]))
            /
            sum(rate(hermes_semantic_search_total[5m]))
          ) > 0.05
        for: 5m
        labels:
          severity: critical
          component: semantic-search
        annotations:
          summary: "High semantic search error rate"
          description: "Semantic search error rate is {{ $value | humanizePercentage }} (threshold: 5%)"

      # Semantic Search High Latency
      - alert: HighSemanticSearchLatency
        expr: |
          histogram_quantile(0.95,
            sum(rate(hermes_semantic_search_duration_seconds_bucket[5m])) by (le)
          ) > 0.2
        for: 10m
        labels:
          severity: warning
          component: semantic-search
        annotations:
          summary: "High semantic search latency"
          description: "P95 semantic search latency is {{ $value }}s (threshold: 200ms)"

  - name: hermes_indexer
    interval: 30s
    rules:
      # Indexer Processing Error Rate
      - alert: HighIndexerErrorRate
        expr: |
          (
            sum(rate(hermes_indexer_errors_total[5m]))
            /
            sum(rate(hermes_indexer_messages_processed_total[5m]))
          ) > 0.05
        for: 5m
        labels:
          severity: warning
          component: indexer
        annotations:
          summary: "High indexer error rate"
          description: "Indexer error rate is {{ $value | humanizePercentage }} (threshold: 5%)"

      # Kafka Consumer Lag
      - alert: HighKafkaConsumerLag
        expr: kafka_consumergroup_lag{group="hermes-indexer-workers"} > 10000
        for: 10m
        labels:
          severity: warning
          component: indexer
        annotations:
          summary: "High Kafka consumer lag"
          description: "Consumer lag is {{ $value }} messages (threshold: 10000)"

      # Indexer Worker Down
      - alert: IndexerWorkerDown
        expr: up{job="hermes-indexer"} == 0
        for: 2m
        labels:
          severity: critical
          component: indexer
        annotations:
          summary: "Indexer worker is down (instance {{ $labels.instance }})"
          description: "Indexer worker {{ $labels.instance }} has been down for 2 minutes"

      # LLM API High Latency
      - alert: HighLLMAPILatency
        expr: |
          histogram_quantile(0.95,
            sum(rate(hermes_llm_api_duration_seconds_bucket[5m])) by (le, provider)
          ) > 5.0
        for: 10m
        labels:
          severity: warning
          component: indexer
        annotations:
          summary: "High LLM API latency ({{ $labels.provider }})"
          description: "P95 LLM API latency is {{ $value }}s (threshold: 5s)"

  - name: database
    interval: 30s
    rules:
      # Database Connection Pool Exhaustion
      - alert: DatabaseConnectionPoolExhausted
        expr: |
          (
            pg_stat_activity_count{datname="hermes"}
            /
            pg_settings_max_connections
          ) > 0.9
        for: 5m
        labels:
          severity: critical
          component: database
        annotations:
          summary: "Database connection pool near exhaustion"
          description: "Database connections at {{ $value | humanizePercentage }} of max (threshold: 90%)"

      # Slow Vector Queries
      - alert: SlowVectorQueries
        expr: |
          histogram_quantile(0.95,
            sum(rate(hermes_vector_query_duration_seconds_bucket[5m])) by (le)
          ) > 1.0
        for: 10m
        labels:
          severity: warning
          component: database
        annotations:
          summary: "Slow vector database queries"
          description: "P95 vector query duration is {{ $value }}s (threshold: 1s)"

      # Database Disk Space
      - alert: DatabaseDiskSpaceLow
        expr: |
          (
            pg_database_size_bytes{datname="hermes"}
            /
            node_filesystem_size_bytes{mountpoint="/var/lib/postgresql"}
          ) > 0.8
        for: 15m
        labels:
          severity: warning
          component: database
        annotations:
          summary: "Database disk space low"
          description: "Database using {{ $value | humanizePercentage }} of disk (threshold: 80%)"

  - name: infrastructure
    interval: 30s
    rules:
      # Service Down
      - alert: ServiceDown
        expr: up == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Service is down ({{ $labels.job }})"
          description: "Service {{ $labels.job }} on {{ $labels.instance }} has been down for 2 minutes"

      # High Memory Usage
      - alert: HighMemoryUsage
        expr: |
          (
            node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes
          ) / node_memory_MemTotal_bytes > 0.9
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage ({{ $labels.instance }})"
          description: "Memory usage is {{ $value | humanizePercentage }} (threshold: 90%)"

      # High CPU Usage
      - alert: HighCPUUsage
        expr: |
          100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100) > 80
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High CPU usage ({{ $labels.instance }})"
          description: "CPU usage is {{ $value }}% (threshold: 80%)"
EOF

success "Created: $OUTPUT_DIR/prometheus/alerts.yml"

# Step 3: Create Alertmanager Configuration
info "Step 3: Creating Alertmanager configuration..."

cat > "$OUTPUT_DIR/alertmanager/alertmanager.yml" << 'EOF'
# Alertmanager Configuration for RFC-088

global:
  resolve_timeout: 5m
  smtp_smarthost: 'localhost:25'
  smtp_from: 'alertmanager@hermes.example.com'
  smtp_require_tls: false

# Templates for notifications
templates:
  - '/etc/alertmanager/templates/*.tmpl'

# Route tree
route:
  group_by: ['alertname', 'cluster', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  receiver: 'default'

  # Child routes
  routes:
    # Critical alerts
    - match:
        severity: critical
      receiver: 'critical'
      continue: true

    # Warning alerts
    - match:
        severity: warning
      receiver: 'warning'

    # Semantic search alerts
    - match:
        component: semantic-search
      receiver: 'semantic-search-team'

    # Indexer alerts
    - match:
        component: indexer
      receiver: 'indexer-team'

    # Database alerts
    - match:
        component: database
      receiver: 'database-team'

# Inhibit rules (prevent alert spam)
inhibit_rules:
  - source_match:
      severity: 'critical'
    target_match:
      severity: 'warning'
    equal: ['alertname', 'cluster', 'service']

# Receivers
receivers:
  # Default receiver
  - name: 'default'
    email_configs:
      - to: 'ops-team@example.com'

  # Critical alerts
  - name: 'critical'
    email_configs:
      - to: 'ops-team@example.com,oncall@example.com'
        send_resolved: true
    slack_configs:
      - api_url: 'YOUR_SLACK_WEBHOOK_URL'
        channel: '#alerts-critical'
        title: '🚨 Critical Alert: {{ .GroupLabels.alertname }}'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'

  # Warning alerts
  - name: 'warning'
    email_configs:
      - to: 'ops-team@example.com'
    slack_configs:
      - api_url: 'YOUR_SLACK_WEBHOOK_URL'
        channel: '#alerts'
        title: '⚠️ Warning: {{ .GroupLabels.alertname }}'

  # Semantic search team
  - name: 'semantic-search-team'
    email_configs:
      - to: 'search-team@example.com'

  # Indexer team
  - name: 'indexer-team'
    email_configs:
      - to: 'indexer-team@example.com'

  # Database team
  - name: 'database-team'
    email_configs:
      - to: 'dba-team@example.com'
EOF

success "Created: $OUTPUT_DIR/alertmanager/alertmanager.yml"

# Step 4: Create Grafana Dashboard JSON
info "Step 4: Creating Grafana dashboard..."

cat > "$OUTPUT_DIR/grafana/rfc-088-dashboard.json" << 'DASHBOARDEOF'
{
  "dashboard": {
    "id": null,
    "uid": "rfc088",
    "title": "RFC-088: Semantic Search Performance",
    "tags": ["rfc-088", "semantic-search", "hermes"],
    "timezone": "browser",
    "schemaVersion": 16,
    "version": 1,
    "refresh": "30s",
    "panels": [
      {
        "id": 1,
        "title": "Semantic Search Request Rate",
        "type": "graph",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
        "targets": [
          {
            "expr": "rate(hermes_semantic_search_total[5m])",
            "legendFormat": "Requests/sec"
          }
        ]
      },
      {
        "id": 2,
        "title": "Semantic Search Error Rate",
        "type": "graph",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
        "targets": [
          {
            "expr": "rate(hermes_semantic_search_errors_total[5m]) / rate(hermes_semantic_search_total[5m])",
            "legendFormat": "Error Rate"
          }
        ]
      },
      {
        "id": 3,
        "title": "Semantic Search Latency (P50, P95, P99)",
        "type": "graph",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8},
        "targets": [
          {
            "expr": "histogram_quantile(0.50, rate(hermes_semantic_search_duration_seconds_bucket[5m]))",
            "legendFormat": "P50"
          },
          {
            "expr": "histogram_quantile(0.95, rate(hermes_semantic_search_duration_seconds_bucket[5m]))",
            "legendFormat": "P95"
          },
          {
            "expr": "histogram_quantile(0.99, rate(hermes_semantic_search_duration_seconds_bucket[5m]))",
            "legendFormat": "P99"
          }
        ]
      },
      {
        "id": 4,
        "title": "Indexer Processing Rate",
        "type": "graph",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8},
        "targets": [
          {
            "expr": "rate(hermes_indexer_messages_processed_total[5m])",
            "legendFormat": "Messages/sec ({{instance}})"
          }
        ]
      },
      {
        "id": 5,
        "title": "Kafka Consumer Lag",
        "type": "graph",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 16},
        "targets": [
          {
            "expr": "kafka_consumergroup_lag{group='hermes-indexer-workers'}",
            "legendFormat": "Lag ({{partition}})"
          }
        ]
      },
      {
        "id": 6,
        "title": "Database Connection Pool",
        "type": "graph",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 16},
        "targets": [
          {
            "expr": "hermes_db_connections_open",
            "legendFormat": "Open Connections"
          },
          {
            "expr": "hermes_db_connections_idle",
            "legendFormat": "Idle Connections"
          },
          {
            "expr": "hermes_db_connections_in_use",
            "legendFormat": "In Use"
          }
        ]
      },
      {
        "id": 7,
        "title": "Vector Query Performance",
        "type": "graph",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 24},
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(hermes_vector_query_duration_seconds_bucket[5m]))",
            "legendFormat": "P95 Latency"
          }
        ]
      },
      {
        "id": 8,
        "title": "LLM API Performance",
        "type": "graph",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 24},
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(hermes_llm_api_duration_seconds_bucket[5m]))",
            "legendFormat": "P95 Latency ({{provider}})"
          }
        ]
      }
    ]
  },
  "overwrite": true
}
DASHBOARDEOF

success "Created: $OUTPUT_DIR/grafana/rfc-088-dashboard.json"

# Step 5: Create Docker Compose monitoring stack
info "Step 5: Creating Docker Compose monitoring stack..."

cat > "$OUTPUT_DIR/docker-compose.monitoring.yml" << 'EOF'
# Docker Compose for Monitoring Stack (RFC-088)
# Includes Prometheus, Grafana, and Alertmanager

version: '3.8'

services:
  # Prometheus
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    restart: unless-stopped
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/usr/share/prometheus/console_libraries'
      - '--web.console.templates=/usr/share/prometheus/consoles'
      - '--web.enable-lifecycle'
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - ./prometheus/alerts.yml:/etc/prometheus/alerts.yml:ro
      - prometheus-data:/prometheus
    networks:
      - monitoring

  # Alertmanager
  alertmanager:
    image: prom/alertmanager:latest
    container_name: alertmanager
    restart: unless-stopped
    command:
      - '--config.file=/etc/alertmanager/alertmanager.yml'
      - '--storage.path=/alertmanager'
    ports:
      - "9093:9093"
    volumes:
      - ./alertmanager/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro
      - alertmanager-data:/alertmanager
    networks:
      - monitoring

  # Grafana
  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    restart: unless-stopped
    environment:
      - GF_SECURITY_ADMIN_USER=${GRAFANA_USER:-admin}
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD:-admin}
      - GF_INSTALL_PLUGINS=grafana-piechart-panel
    ports:
      - "3000:3000"
    volumes:
      - grafana-data:/var/lib/grafana
      - ./grafana:/etc/grafana/provisioning/dashboards:ro
    networks:
      - monitoring
    depends_on:
      - prometheus

  # Node Exporter (system metrics)
  node-exporter:
    image: prom/node-exporter:latest
    container_name: node-exporter
    restart: unless-stopped
    command:
      - '--path.procfs=/host/proc'
      - '--path.sysfs=/host/sys'
      - '--collector.filesystem.mount-points-exclude=^/(sys|proc|dev|host|etc)($$|/)'
    ports:
      - "9100:9100"
    volumes:
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /:/rootfs:ro
    networks:
      - monitoring

  # Postgres Exporter (PostgreSQL metrics)
  postgres-exporter:
    image: prometheuscommunity/postgres-exporter:latest
    container_name: postgres-exporter
    restart: unless-stopped
    environment:
      DATA_SOURCE_NAME: "${DATABASE_URL}"
    ports:
      - "9187:9187"
    networks:
      - monitoring

networks:
  monitoring:
    driver: bridge
  # Connect to hermes network for scraping metrics
  hermes:
    external: true

volumes:
  prometheus-data:
  alertmanager-data:
  grafana-data:
EOF

success "Created: $OUTPUT_DIR/docker-compose.monitoring.yml"

# Step 6: Create Grafana provisioning
info "Step 6: Creating Grafana provisioning config..."

mkdir -p "$OUTPUT_DIR/grafana/datasources"
mkdir -p "$OUTPUT_DIR/grafana/dashboards"

cat > "$OUTPUT_DIR/grafana/datasources/prometheus.yml" << 'EOF'
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: false
EOF

cat > "$OUTPUT_DIR/grafana/dashboards/dashboard.yml" << 'EOF'
apiVersion: 1

providers:
  - name: 'RFC-088 Dashboards'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /etc/grafana/provisioning/dashboards
EOF

success "Created Grafana provisioning configs"

# Step 7: Create README
info "Step 7: Creating monitoring README..."

cat > "$OUTPUT_DIR/README.md" << 'EOF'
# RFC-088 Monitoring Stack

This directory contains monitoring configuration for RFC-088 semantic search deployment.

## Components

- **Prometheus**: Metrics collection and storage
- **Alertmanager**: Alert routing and notifications
- **Grafana**: Metrics visualization and dashboards
- **Node Exporter**: System metrics
- **Postgres Exporter**: PostgreSQL metrics

## Quick Start

1. Start monitoring stack:
   ```bash
   docker-compose -f docker-compose.monitoring.yml up -d
   ```

2. Access dashboards:
   - Grafana: http://localhost:3000 (admin/admin)
   - Prometheus: http://localhost:9090
   - Alertmanager: http://localhost:9093

3. Import RFC-088 dashboard:
   - Open Grafana
   - Go to Dashboards > Import
   - Upload `grafana/rfc-088-dashboard.json`

## Alert Rules

Alerts are defined in `prometheus/alerts.yml`:

### API Alerts
- High error rate (>1%)
- High latency (P95 > 1s)
- Semantic search errors (>5%)
- Semantic search latency (P95 > 200ms)

### Indexer Alerts
- High processing error rate (>5%)
- High Kafka consumer lag (>10k messages)
- Indexer worker down
- High LLM API latency (P95 > 5s)

### Database Alerts
- Connection pool exhaustion (>90%)
- Slow vector queries (P95 > 1s)
- Low disk space (>80% used)

### Infrastructure Alerts
- Service down
- High memory usage (>90%)
- High CPU usage (>80%)

## Configuration

### Alertmanager

Edit `alertmanager/alertmanager.yml` to configure:
- Email SMTP settings
- Slack webhooks
- PagerDuty integration
- Alert routing rules

### Prometheus

Edit `prometheus/prometheus.yml` to:
- Add scrape targets
- Adjust scrape intervals
- Configure remote storage

## Monitoring Best Practices

1. **Set up alerts**: Configure Alertmanager with your notification channels
2. **Tune thresholds**: Adjust alert thresholds based on your SLAs
3. **Review dashboards**: Customize Grafana dashboards for your needs
4. **Monitor costs**: Track OpenAI API usage and costs
5. **Capacity planning**: Use metrics to plan scaling

## Troubleshooting

### Prometheus not scraping metrics
- Check that Hermes services are exposing metrics on configured ports
- Verify network connectivity between Prometheus and targets

### No data in Grafana
- Check Prometheus datasource connection in Grafana
- Verify Prometheus is scraping metrics successfully

### Alerts not firing
- Check Alertmanager logs: `docker logs alertmanager`
- Verify alert rules in Prometheus UI
- Test Alertmanager configuration

## Support

See RFC-088 documentation:
- Monitoring setup: `docs/deployment/monitoring-setup.md`
- Troubleshooting: `docs/guides/troubleshooting.md`
EOF

success "Created: $OUTPUT_DIR/README.md"

# Step 8: Test Prometheus configuration
info "Step 8: Testing Prometheus configuration..."

if command -v promtool &> /dev/null; then
    if promtool check config "$OUTPUT_DIR/prometheus/prometheus.yml" 2>&1 | grep -q "SUCCESS"; then
        success "Prometheus configuration is valid"
    else
        warn "Prometheus configuration may have issues"
    fi

    if promtool check rules "$OUTPUT_DIR/prometheus/alerts.yml" 2>&1 | grep -q "SUCCESS"; then
        success "Prometheus alert rules are valid"
    else
        warn "Prometheus alert rules may have issues"
    fi
else
    info "promtool not found, skipping configuration validation"
    info "Install with: brew install prometheus (macOS) or apt-get install prometheus (Linux)"
fi

# Summary
echo ""
echo "================================================"
echo "Monitoring Setup Complete"
echo "================================================"
success "All monitoring configuration files created!"
echo ""
info "Files created in: $OUTPUT_DIR"
echo "  - prometheus/prometheus.yml (scrape config)"
echo "  - prometheus/alerts.yml (alert rules)"
echo "  - alertmanager/alertmanager.yml (notification config)"
echo "  - grafana/rfc-088-dashboard.json (dashboard)"
echo "  - docker-compose.monitoring.yml (monitoring stack)"
echo ""
info "Next Steps:"
echo ""
echo "1. Start monitoring stack:"
echo "   cd $OUTPUT_DIR"
echo "   docker-compose -f docker-compose.monitoring.yml up -d"
echo ""
echo "2. Access dashboards:"
echo "   - Grafana:      http://localhost:3000 (admin/admin)"
echo "   - Prometheus:   http://localhost:9090"
echo "   - Alertmanager: http://localhost:9093"
echo ""
echo "3. Configure alerts:"
echo "   - Edit $OUTPUT_DIR/alertmanager/alertmanager.yml"
echo "   - Set up email/Slack/PagerDuty integrations"
echo "   - Reload Alertmanager: docker restart alertmanager"
echo ""
echo "4. Import Grafana dashboard:"
echo "   - Open Grafana UI"
echo "   - Go to Dashboards > Import"
echo "   - Upload $OUTPUT_DIR/grafana/rfc-088-dashboard.json"
echo ""
