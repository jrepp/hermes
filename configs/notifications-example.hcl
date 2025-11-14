# RFC-087 Notification System Configuration Example
#
# This example shows how to configure the RFC-087 notification system
# with Redpanda/Kafka and various notification backends.

notifications {
  # Enable the RFC-087 notification system
  enabled = true

  # Kafka/Redpanda broker addresses (comma-separated for multiple brokers)
  brokers = "localhost:9092"

  # Topic for publishing notifications
  topic = "hermes.notifications"

  # Enabled backends (comma-separated)
  # Available: audit, mail, slack, telegram, discord
  backends = "audit,mail"

  # Optional: Override embedded templates with custom templates
  # If not specified, uses embedded templates from internal/notifications/templates
  # templates_path = "/path/to/custom/templates"

  # SMTP configuration for mail backend
  smtp {
    host         = "smtp.example.com"
    port         = "587"
    username     = "notifications@example.com"
    password     = "smtp-password-here"
    from_address = "notifications@hermes.example.com"
    from_name    = "Hermes Notifications"
    use_tls      = true
  }
}

# Example: Production configuration with Redpanda cluster
# notifications {
#   enabled  = true
#   brokers  = "redpanda-1:9092,redpanda-2:9092,redpanda-3:9092"
#   topic    = "hermes.notifications"
#   backends = "audit,mail,slack"
#
#   smtp {
#     host         = "smtp.sendgrid.net"
#     port         = "587"
#     username     = "apikey"
#     password     = env("SENDGRID_API_KEY")
#     from_address = "notifications@company.com"
#     from_name    = "Company Hermes"
#     use_tls      = true
#   }
# }

# Example: Development configuration with Mailhog
# notifications {
#   enabled  = true
#   brokers  = "localhost:19092"
#   topic    = "hermes.notifications"
#   backends = "audit,mail"
#
#   smtp {
#     host         = "localhost"
#     port         = "1025"
#     from_address = "dev@hermes.local"
#     from_name    = "Hermes Dev"
#     use_tls      = false
#   }
# }
