# RFC-087 Notifier Configuration - Ntfy Backend
# This notifier handles only ntfy push notifications

brokers        = "redpanda:9092"
topic          = "hermes.notifications"
consumer_group = "hermes-notifiers"

backends {
  ntfy {
    enabled = true

    # server_url = "https://ntfy.sh"  # Optional, defaults to ntfy.sh
    topic = "hermes-dev-test-notifications"
  }
}
