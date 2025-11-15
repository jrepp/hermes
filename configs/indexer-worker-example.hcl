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

# LLM provider configuration (RFC-088)
llm {
  # OpenAI configuration (requires API key)
  openai_api_key = "sk-..."  # Set via environment variable: OPENAI_API_KEY

  # Ollama configuration (local LLM server)
  ollama_url = "http://localhost:11434"  # Default Ollama server

  # AWS Bedrock configuration (uses AWS credentials from environment/IAM)
  bedrock_region = "us-east-1"  # AWS region for Bedrock
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
          model      = "text-embedding-3-small"  # OpenAI embedding model
          dimensions = 1536                       # Vector dimensions
          provider   = "openai"                   # LLM provider
          chunk_size = 8000                       # Characters per chunk (0 = no chunking)
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

    # Ruleset 4: Use local Ollama for cost-effective summaries
    {
      name = "local-llm-summaries"

      conditions = {
        document_type = "Meeting Notes,Memo"
      }

      pipeline = ["search_index", "llm_summary"]

      config = {
        llm_summary = {
          model      = "llama3"      # Ollama model (local)
          max_tokens = 300
          style      = "bullet-points"
        }
      }
    },

    # Ruleset 5: Use AWS Bedrock Claude for high-quality analysis
    {
      name = "bedrock-claude-analysis"

      conditions = {
        document_type = "Strategy,Vision"
      }

      pipeline = ["search_index", "llm_summary"]

      config = {
        llm_summary = {
          model      = "us.anthropic.claude-3-7-sonnet-20250219-v1:0"  # AWS Bedrock
          max_tokens = 1000
          style      = "executive"
        }
      }
    },
  ]

  # ============================================================================
  # LLM Model Examples:
  # ============================================================================
  #
  # Chat/Summary Models (OpenAI):
  #   - gpt-4o               # Most capable, highest cost
  #   - gpt-4o-mini          # Fast and cost-effective (recommended default)
  #   - gpt-4-turbo          # Previous generation
  #   - gpt-3.5-turbo        # Fastest, lowest cost
  #
  # Chat/Summary Models (AWS Bedrock):
  #   - us.anthropic.claude-3-7-sonnet-20250219-v1:0    # Latest Claude (recommended)
  #   - us.anthropic.claude-3-5-sonnet-20241022-v2:0    # Previous Claude
  #   - anthropic.claude-3-opus-20240229-v1:0           # Most capable Claude
  #   - anthropic.claude-3-haiku-20240307-v1:0          # Fast, cost-effective
  #   - amazon.titan-text-express-v1                    # Amazon's model
  #
  # Chat/Summary Models (Ollama - Local):
  #   - llama3               # Meta's Llama 3 (8B)
  #   - llama3:70b           # Llama 3 70B (requires more resources)
  #   - mistral              # Mistral 7B
  #   - mistral-large        # Mistral Large
  #   - codellama            # Code-focused Llama
  #   - phi                  # Microsoft Phi
  #   - qwen2                # Alibaba Qwen 2
  #   - gemma2               # Google Gemma 2
  #
  # Embeddings Models (OpenAI):
  #   - text-embedding-3-small   # 1536 dimensions, cost-effective (recommended)
  #   - text-embedding-3-large   # 3072 dimensions, highest quality
  #   - text-embedding-ada-002   # 1536 dimensions, previous generation
  #
  # ============================================================================
  #
  # Embeddings Pipeline Step Configuration:
  # ============================================================================
  #
  # The embeddings step generates vector embeddings for semantic search.
  #
  # Configuration options:
  #   - model: Embedding model name (e.g., "text-embedding-3-small")
  #   - dimensions: Vector dimensions (1536, 3072, etc.)
  #   - provider: LLM provider ("openai", "bedrock", "ollama")
  #   - chunk_size: Max characters per chunk (0 = no chunking)
  #
  # Chunking:
  #   Large documents can be split into chunks for better semantic search.
  #   Each chunk gets its own embedding vector.
  #
  # Example configurations:
  #
  #   embeddings = {
  #     model      = "text-embedding-3-small"
  #     dimensions = 1536
  #     provider   = "openai"
  #     chunk_size = 8000  # Split docs longer than 8000 chars
  #   }
  #
  #   embeddings = {
  #     model      = "text-embedding-3-large"
  #     dimensions = 3072
  #     provider   = "openai"
  #     chunk_size = 0     # No chunking
  #   }
  #
  # ============================================================================
}

# Logging configuration
log {
  level  = "info"
  format = "json"
}
