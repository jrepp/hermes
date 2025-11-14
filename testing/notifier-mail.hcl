# RFC-087 Notifier Configuration - Mail Backend
# This notifier handles only email notifications

brokers        = "redpanda:9092"
topic          = "hermes.notifications"
consumer_group = "hermes-notifiers"

backends {
  mail {
    enabled = true

    smtp_host     = "mailhog"
    smtp_port     = "1025"
    from_address  = "notifications@hermes.example.com"
    from_name     = "Hermes Notifications"
    use_tls       = false
  }
}
