# RFC-087 Notifier Configuration - Audit Backend
# This notifier handles only audit logging

brokers        = "redpanda:9092"
topic          = "hermes.notifications"
consumer_group = "hermes-notifiers"

backends {
  audit {
    enabled = true
  }
}
