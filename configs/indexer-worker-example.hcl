# Example configuration for RFC-088 Event-Driven Indexer Worker
#
# This configuration file shows how to configure the indexer worker for:
# - Outbox relay mode: Publishes events from DB to Redpanda
# - Consumer mode: Consumes events and executes pipelines

# Database configuration
postgres {
  host     = "localhost"
  port     = 5432
  user     = "postgres"
  password = "postgres"
  db_name  = "hermes"
  ssl_mode = "disable"
}

# Search provider configuration
search {
  provider = "meilisearch"

  meilisearch {
    host = "http://localhost:7700"
    api_key = "masterKey123"

    # Index names
    docs_index     = "hermes_docs"
    drafts_index   = "hermes_drafts"
    projects_index = "hermes_projects"
  }
}

# Indexer configuration (RFC-088)
indexer {
  # Redpanda/Kafka configuration
  redpanda_brokers = ["localhost:19192"]
  topic            = "hermes.document-revisions"
  consumer_group   = "hermes-indexer-workers"

  # Outbox relay settings
  poll_interval = "1s"   # How often to poll the outbox table
  batch_size    = 100    # How many outbox entries to process per batch

  # Pipeline rulesets
  # Each ruleset defines conditions for matching documents and the pipeline steps to execute

  rulesets = [
    # Ruleset 1: Published RFCs get full processing
    {
      name = "published-rfcs"

      # Conditions to match (AND logic)
      conditions = {
        provider_type = "google"
        document_type = "RFC"
        status        = "In-Review,Approved"
      }

      # Pipeline steps to execute (in order)
      pipeline = [
        "search_index",   # Update Meilisearch
        "embeddings",     # Generate embeddings for semantic search
        "llm_summary",    # Generate AI summary
      ]

      # Step-specific configuration
      config = {
        embeddings = {
          model      = "text-embedding-3-small"
          dimensions = 1536
        }

        llm_summary = {
          model      = "gpt-4o-mini"
          max_tokens = 500
          style      = "executive"
        }
      }
    },

    # Ruleset 2: All documents get search indexing
    {
      name = "all-documents"

      # No conditions = matches all documents
      conditions = {}

      pipeline = ["search_index"]
    },

    # Ruleset 3: Long design docs get deep analysis
    {
      name = "design-docs-deep-analysis"

      conditions = {
        document_type      = "PRD,RFC"
        content_length_gt  = "5000"  # Only analyze long documents
      }

      pipeline = [
        "search_index",
        "embeddings",
        "llm_summary",
        "llm_validation",  # Custom step: check for completeness
      ]

      config = {
        llm_validation = {
          checks = ["has_motivation", "has_alternatives", "has_success_metrics"]
        }
      }
    },
  ]
}

# Logging configuration
log {
  level  = "info"
  format = "json"
}
