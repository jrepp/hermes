# RFC-087 Implementation: Docker Compose Integration

**Parent**: [RFC-087-NOTIFICATION-BACKEND.md](./RFC-087-NOTIFICATION-BACKEND.md)

This document details the Docker Compose setup for testing the notification system with Redpanda and notification workers.

## Docker Compose Configuration

### Add to `testing/docker-compose.yml`

```yaml
services:
  # ... existing services (postgres, meilisearch, dex, hermes-central, hermes-edge, etc.) ...

  # =====================================================================
  # REDPANDA - Kafka-compatible message broker for notifications
  # =====================================================================
  redpanda:
    image: docker.redpanda.com/redpandadata/redpanda:v24.2.11
    container_name: hermes-redpanda
    command:
      - redpanda
      - start
      - --smp 1
      - --memory 512M
      - --reserve-memory 0M
      - --overprovisioned
      - --node-id 0
      - --kafka-addr internal://0.0.0.0:9092,external://0.0.0.0:19092
      - --advertise-kafka-addr internal://redpanda:9092,external://localhost:19092
    ports:
      - "19092:19092"  # Kafka API (external access)
      - "18081:8081"   # Schema Registry
      - "18082:8082"   # HTTP Proxy (Pandaproxy)
      - "19644:9644"   # Admin API
    volumes:
      - redpanda_data:/var/lib/redpanda/data
    healthcheck:
      test: ["CMD-SHELL", "rpk cluster health | grep -E 'Healthy:.+true' || exit 1"]
      interval: 5s
      timeout: 5s
      retries: 10
      start_period: 10s
    networks:
      - hermes-testing

  # =====================================================================
  # NOTIFICATION WORKERS
  # =====================================================================

  # Audit Worker - Replica 1
  notification-worker-audit-1:
    container_name: notification-worker-audit-1
    build:
      context: ..
      dockerfile: Dockerfile
      target: notification-worker  # Multi-stage build target
    environment:
      NOTIFICATION_BROKERS: redpanda:9092
      NOTIFICATION_TOPIC: hermes.notifications
      NOTIFICATION_GROUP: hermes-notification-workers
      NOTIFICATION_BACKENDS: audit
      WORKSPACE_ROOT: /workspace  # Document storage for templates
      TEMPLATE_PREFIX: /notification-templates
      TEMPLATE_CACHE_TTL: 300  # 5 minutes
    command:
      - notification-worker
      - -brokers=redpanda:9092
      - -topic=hermes.notifications
      - -group=hermes-notification-workers
      - -backends=audit
    volumes:
      - workspace_data:/workspace  # Mount workspace for template loading
    depends_on:
      redpanda:
        condition: service_healthy
    restart: unless-stopped
    networks:
      - hermes-testing

  # Audit Worker - Replica 2
  notification-worker-audit-2:
    container_name: notification-worker-audit-2
    build:
      context: ..
      dockerfile: Dockerfile
      target: notification-worker
    environment:
      NOTIFICATION_BROKERS: redpanda:9092
      NOTIFICATION_TOPIC: hermes.notifications
      NOTIFICATION_GROUP: hermes-notification-workers
      NOTIFICATION_BACKENDS: audit
    command:
      - notification-worker
      - -brokers=redpanda:9092
      - -topic=hermes.notifications
      - -group=hermes-notification-workers
      - -backends=audit
    depends_on:
      redpanda:
        condition: service_healthy
    restart: unless-stopped
    networks:
      - hermes-testing

  # Audit Worker - Replica 3
  notification-worker-audit-3:
    container_name: notification-worker-audit-3
    build:
      context: ..
      dockerfile: Dockerfile
      target: notification-worker
    environment:
      NOTIFICATION_BROKERS: redpanda:9092
      NOTIFICATION_TOPIC: hermes.notifications
      NOTIFICATION_GROUP: hermes-notification-workers
      NOTIFICATION_BACKENDS: audit
    command:
      - notification-worker
      - -brokers=redpanda:9092
      - -topic=hermes.notifications
      - -group=hermes-notification-workers
      - -backends=audit
    depends_on:
      redpanda:
        condition: service_healthy
    restart: unless-stopped
    networks:
      - hermes-testing

  # Mail Worker (Optional - for SMTP testing)
  notification-worker-mail:
    container_name: notification-worker-mail
    build:
      context: ..
      dockerfile: Dockerfile
      target: notification-worker
    environment:
      NOTIFICATION_BROKERS: redpanda:9092
      NOTIFICATION_TOPIC: hermes.notifications
      NOTIFICATION_GROUP: hermes-notification-workers
      NOTIFICATION_BACKENDS: mail
      WORKSPACE_ROOT: /workspace
      TEMPLATE_PREFIX: /notification-templates
      TEMPLATE_CACHE_TTL: 300
      # SMTP configuration
      SMTP_HOST: mailhog  # Use MailHog for testing
      SMTP_PORT: 1025
      SMTP_USERNAME: ""
      SMTP_PASSWORD: ""
      SMTP_FROM: noreply@hermes.example.com
    command:
      - notification-worker
      - -brokers=redpanda:9092
      - -topic=hermes.notifications
      - -group=hermes-notification-workers
      - -backends=mail
    volumes:
      - workspace_data:/workspace
    depends_on:
      redpanda:
        condition: service_healthy
      mailhog:
        condition: service_started
    restart: unless-stopped
    networks:
      - hermes-testing

  # MailHog - SMTP testing server (Optional)
  mailhog:
    image: mailhog/mailhog:v1.0.1
    container_name: hermes-mailhog
    ports:
      - "1025:1025"  # SMTP server
      - "8025:8025"  # Web UI
    networks:
      - hermes-testing

volumes:
  # ... existing volumes ...
  redpanda_data:
```

## Dockerfile Updates

### Multi-Stage Build for Notification Worker

Add to your `Dockerfile`:

```dockerfile
# =====================================================================
# Build stage for notification worker
# =====================================================================
FROM golang:1.21-alpine AS notification-worker-builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build notification worker binary
RUN CGO_ENABLED=0 GOOS=linux go build -o notification-worker ./cmd/notification-worker

# =====================================================================
# Notification worker runtime image
# =====================================================================
FROM alpine:3.18 AS notification-worker

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy notification worker binary
COPY --from=notification-worker-builder /build/notification-worker .

# Note: Templates are loaded from document storage at runtime
# No need to copy embedded template files

ENTRYPOINT ["/app/notification-worker"]
```

## Notification Worker Command

### Main Worker Implementation

```go
// cmd/notification-worker/main.go
package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "strings"
    "syscall"

    "github.com/hashicorp-forge/hermes/pkg/notifications"
    "github.com/hashicorp-forge/hermes/pkg/notifications/backends"
    "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/local"
    "github.com/twmb/franz-go/pkg/kgo"
)

type Config struct {
    Brokers         []string
    Topic           string
    ConsumerGroup   string
    EnabledBackends []string
}

func main() {
    cfg := parseConfig()

    // Initialize workspace for template loading
    workspaceRoot := os.Getenv("WORKSPACE_ROOT")
    if workspaceRoot == "" {
        workspaceRoot = "/workspace"
    }

    workspace, err := local.NewAdapter(local.Config{
        RootDir: workspaceRoot,
    })
    if err != nil {
        log.Fatalf("failed to initialize workspace: %v", err)
    }

    // Create template loader
    templatePrefix := os.Getenv("TEMPLATE_PREFIX")
    if templatePrefix == "" {
        templatePrefix = "/notification-templates"
    }
    templateLoader := backends.NewTemplateLoader(workspace, templatePrefix)

    // Setup signal handling
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    // Start background template watcher
    go templateLoader.WatchTemplates(ctx)

    // Initialize enabled backends with template loader
    backendList := initializeBackends(cfg.EnabledBackends, templateLoader)
    if len(backendList) == 0 {
        log.Fatal("no backends initialized")
    }

    // Create Kafka consumer
    client, err := kgo.NewClient(
        kgo.SeedBrokers(cfg.Brokers...),
        kgo.ConsumerGroup(cfg.ConsumerGroup),
        kgo.ConsumeTopics(cfg.Topic),
    )
    if err != nil {
        log.Fatalf("failed to create consumer: %v", err)
    }
    defer client.Close()

    log.Printf("Starting notification worker (backends=%v, group=%s)\n", cfg.EnabledBackends, cfg.ConsumerGroup)

    // Consume messages
    for {
        fetches := client.PollFetches(ctx)
        if errs := fetches.Errors(); len(errs) > 0 {
            for _, err := range errs {
                log.Printf("fetch error: %v\n", err)
            }
            continue
        }

        fetches.EachPartition(func(p kgo.FetchTopicPartition) {
            for _, record := range p.Records {
                if err := processMessage(ctx, backendList, record); err != nil {
                    log.Printf("failed to process message: %v\n", err)
                } else {
                    // Commit offset after successful processing
                    client.CommitRecords(ctx, record)
                }
            }
        })

        // Check for shutdown
        select {
        case <-ctx.Done():
            log.Println("Shutting down notification worker")
            return
        default:
        }
    }
}

func processMessage(ctx context.Context, backends []backends.Backend, record *kgo.Record) error {
    // Parse notification message
    var msg notifications.NotificationMessage
    if err := json.Unmarshal(record.Value, &msg); err != nil {
        return fmt.Errorf("failed to unmarshal message: %w", err)
    }

    log.Printf("Processing message: id=%s template=%s backends=%v", msg.ID, msg.Template, msg.Backends)

    // Route to appropriate backends based on message.Backends field
    for _, backend := range backends {
        for _, targetBackend := range msg.Backends {
            if backend.SupportsBackend(targetBackend) {
                if err := backend.Handle(ctx, &msg); err != nil {
                    log.Printf("backend %s failed: %v", backend.Name(), err)
                    // Continue with other backends
                } else {
                    log.Printf("backend %s processed message %s", backend.Name(), msg.ID)
                }
            }
        }
    }

    return nil
}

func initializeBackends(enabledBackends []string, templateLoader *backends.TemplateLoader) []backends.Backend {
    var backendList []backends.Backend

    for _, name := range enabledBackends {
        switch name {
        case "audit":
            // Audit backend doesn't need templates
            backendList = append(backendList, backends.NewAuditBackend())
            log.Printf("Initialized audit backend")

        case "mail":
            mailCfg := backends.MailBackendConfig{
                SMTPHost:     os.Getenv("SMTP_HOST"),
                SMTPPort:     getEnvInt("SMTP_PORT", 587),
                SMTPUsername: os.Getenv("SMTP_USERNAME"),
                SMTPPassword: os.Getenv("SMTP_PASSWORD"),
                FromAddress:  os.Getenv("SMTP_FROM"),
            }
            if backend, err := backends.NewMailBackend(mailCfg, templateLoader); err == nil {
                backendList = append(backendList, backend)
                log.Printf("Initialized mail backend with template loader")
            } else {
                log.Printf("Failed to initialize mail backend: %v", err)
            }

        case "slack":
            slackCfg := backends.SlackBackendConfig{
                BotToken: os.Getenv("SLACK_BOT_TOKEN"),
            }
            if backend, err := backends.NewSlackBackend(slackCfg, templateLoader); err == nil {
                backendList = append(backendList, backend)
                log.Printf("Initialized slack backend with template loader")
            } else {
                log.Printf("Failed to initialize slack backend: %v", err)
            }

        case "telegram":
            telegramCfg := backends.TelegramBackendConfig{
                BotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
            }
            if backend, err := backends.NewTelegramBackend(telegramCfg, templateLoader); err == nil {
                backendList = append(backendList, backend)
                log.Printf("Initialized telegram backend with template loader")
            } else {
                log.Printf("Failed to initialize telegram backend: %v", err)
            }

        case "discord":
            discordCfg := backends.DiscordBackendConfig{
                BotToken: os.Getenv("DISCORD_BOT_TOKEN"),
            }
            if backend, err := backends.NewDiscordBackend(discordCfg, templateLoader); err == nil {
                backendList = append(backendList, backend)
                log.Printf("Initialized discord backend with template loader")
            } else {
                log.Printf("Failed to initialize discord backend: %v", err)
            }

        default:
            log.Printf("Unknown backend: %s", name)
        }
    }

    return backendList
}

func parseConfig() Config {
    cfg := Config{}

    var brokers, backends string
    flag.StringVar(&brokers, "brokers", "localhost:9092", "Kafka broker addresses (comma-separated)")
    flag.StringVar(&cfg.Topic, "topic", "hermes.notifications", "Notification topic")
    flag.StringVar(&cfg.ConsumerGroup, "group", "hermes-notification-workers", "Consumer group")
    flag.StringVar(&backends, "backends", "audit", "Enabled backends (comma-separated: audit,mail,slack,telegram,discord)")
    flag.Parse()

    cfg.Brokers = strings.Split(brokers, ",")
    cfg.EnabledBackends = strings.Split(backends, ",")

    return cfg
}

func getEnvInt(key string, defaultValue int) int {
    val := os.Getenv(key)
    if val == "" {
        return defaultValue
    }

    var intVal int
    if _, err := fmt.Sscanf(val, "%d", &intVal); err != nil {
        return defaultValue
    }

    return intVal
}
```

## Usage

### Starting the Test Environment

```bash
cd testing
docker compose up -d
```

### Viewing Logs

```bash
# View all notification worker logs
docker compose logs -f notification-worker-audit-1 notification-worker-audit-2 notification-worker-audit-3

# View Redpanda logs
docker compose logs -f redpanda

# View mail worker and MailHog
docker compose logs -f notification-worker-mail mailhog
```

### Testing Notifications

#### Send a Test Notification

```bash
# Exec into hermes-central container
docker compose exec hermes-central sh

# Use rpk (Redpanda CLI) to publish a test message
rpk topic produce hermes.notifications --brokers redpanda:9092 <<EOF
{
  "id": "test-001",
  "type": "document_approved",
  "timestamp": "2025-11-13T10:30:00Z",
  "template": "document_approved",
  "template_context": {
    "DocumentShortName": "RFC-087",
    "DocumentTitle": "Notification System",
    "DocumentType": "RFC",
    "DocumentURL": "https://hermes.example.com/docs/RFC-087",
    "ApproverName": "Alice Smith",
    "Product": "Hermes"
  },
  "recipients": [
    {
      "email": "test@example.com",
      "name": "Test User"
    }
  ],
  "backends": ["audit", "mail"]
}
EOF
```

#### View Audit Logs

```bash
docker compose logs notification-worker-audit-1 | grep "Notification ID: test-001"
```

#### View Emails (MailHog Web UI)

Open http://localhost:8025 in your browser to see captured emails.

### Monitoring Redpanda

```bash
# Check cluster status
docker compose exec redpanda rpk cluster health

# List topics
docker compose exec redpanda rpk topic list

# View topic details
docker compose exec redpanda rpk topic describe hermes.notifications

# View consumer group lag
docker compose exec redpanda rpk group describe hermes-notification-workers
```

### Creating the Notification Topic

```bash
# Create topic with 3 partitions
docker compose exec redpanda rpk topic create hermes.notifications \
  --partitions 3 \
  --replicas 1 \
  --config retention.ms=604800000  # 7 days
```

## Scaling Workers

### Add More Workers

Simply scale up the number of worker replicas:

```bash
docker compose up -d --scale notification-worker-audit-1=5
```

### Different Backend Workers

To run workers with different backends:

```yaml
  notification-worker-slack:
    # ... same configuration as audit worker ...
    environment:
      SLACK_BOT_TOKEN: ${SLACK_BOT_TOKEN}
      NOTIFICATION_BACKENDS: slack
    command:
      - notification-worker
      - -brokers=redpanda:9092
      - -topic=hermes.notifications
      - -group=hermes-notification-workers
      - -backends=slack
```

## Troubleshooting

### Workers Not Consuming Messages

1. Check Redpanda health:
   ```bash
   docker compose logs redpanda | grep -i error
   ```

2. Check worker logs:
   ```bash
   docker compose logs notification-worker-audit-1 | grep -i error
   ```

3. Verify topic exists:
   ```bash
   docker compose exec redpanda rpk topic list
   ```

### Consumer Lag

Check consumer group lag:

```bash
docker compose exec redpanda rpk group describe hermes-notification-workers
```

If lag is increasing:
- Scale up workers
- Check worker logs for errors
- Verify backends are responding

### Email Not Sending

1. Check MailHog is running:
   ```bash
   docker compose ps mailhog
   ```

2. Check SMTP configuration in mail worker logs:
   ```bash
   docker compose logs notification-worker-mail
   ```

3. View MailHog web UI: http://localhost:8025

## Performance Testing

### Load Test Script

```bash
#!/bin/bash
# testing/load-test-notifications.sh

for i in {1..1000}; do
  docker compose exec -T redpanda rpk topic produce hermes.notifications --brokers redpanda:9092 <<EOF
{
  "id": "load-test-$i",
  "type": "document_approved",
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "template": "document_approved",
  "template_context": {
    "DocumentShortName": "TEST-$i",
    "ApproverName": "Load Tester"
  },
  "recipients": [{"email": "test@example.com"}],
  "backends": ["audit"]
}
EOF
  echo "Sent message $i"
done
```

Run the load test:

```bash
chmod +x testing/load-test-notifications.sh
./testing/load-test-notifications.sh
```

Monitor processing:

```bash
# Watch consumer lag
watch -n 1 'docker compose exec redpanda rpk group describe hermes-notification-workers'
```

---

**Related Documents**:
- [RFC-087-NOTIFICATION-BACKEND.md](./RFC-087-NOTIFICATION-BACKEND.md) - Main RFC
- [RFC-087-MESSAGE-SCHEMA.md](./RFC-087-MESSAGE-SCHEMA.md) - Message format and templates
- [RFC-087-BACKENDS.md](./RFC-087-BACKENDS.md) - Backend implementations
