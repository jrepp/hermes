# RFC-088 Indexer Worker Configuration for Testing
#
# This configuration is used by both:
# - indexer-relay: Publishes outbox events to Redpanda
# - indexer-consumer: Consumes events and executes pipelines

postgres {
  host     = "postgres"  # Container hostname
  port     = 5432
  user     = "postgres"
  password = "postgres"
  db_name  = "hermes_testing"
  ssl_mode = "disable"
}

search {
  provider = "meilisearch"

  meilisearch {
    host    = "http://meilisearch:7700"
    api_key = "masterKey123"

    docs_index     = "hermes_docs"
    drafts_index   = "hermes_drafts"
    projects_index = "hermes_projects"
  }
}

indexer {
  # Redpanda configuration (env vars override these)
  redpanda_brokers = ["redpanda:9092"]
  topic            = "hermes.document-revisions"
  consumer_group   = "hermes-indexer-workers"

  # Relay settings
  poll_interval = "1s"
  batch_size    = 100

  # Pipeline rulesets for testing
  rulesets = [
    # Test ruleset 1: All documents get indexed in search
    {
      name = "all-documents"
      conditions = {}
      pipeline = ["search_index"]
    },

    # Test ruleset 2: RFCs get full processing
    {
      name = "rfcs-full-pipeline"
      conditions = {
        document_type = "RFC"
      }
      pipeline = ["search_index"]  # Start with just search, add more steps later
      # pipeline = ["search_index", "llm_summary", "embeddings"]  # Full pipeline
    },
  ]
}

log {
  level  = "info"
  format = "console"  # Use console format for docker logs
}
