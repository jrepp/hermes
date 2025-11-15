// RFC-089: Multi-Provider Configuration with Migration Support
// This configuration demonstrates:
// 1. Multiple storage providers (Local + S3)
// 2. Primary provider designation
// 3. Migration strategy configuration
// 4. Provider registry integration

base_url = "http://localhost:4200"
log_format = "standard"

// Algolia/Meilisearch configuration
algolia {
  application_id            = "rfc089-app-id"
  docs_index_name           = "docs"
  drafts_index_name         = "drafts"
  internal_index_name       = "internal"
  links_index_name          = "links"
  missing_fields_index_name = "missing_fields"
  projects_index_name       = "projects"
  search_api_key            = "test-search-key"
  write_api_key             = "masterKey123"
}

// RFC-089: Storage Providers Registry
// Supports multiple storage backends with migration capabilities
storage_providers {
  // Primary provider - Local Workspace (current production)
  provider "local-primary" {
    type        = "local"
    is_primary  = true
    is_writable = true
    status      = "active"

    config {
      base_path    = "/app/workspace_data"
      docs_path    = "/app/workspace_data/docs"
      drafts_path  = "/app/workspace_data/drafts"
      folders_path = "/app/workspace_data/folders"
      users_path   = "/app/workspace_data/users"
      tokens_path  = "/app/workspace_data/tokens"
      domain       = "hermes.local"
    }
  }

  // Secondary provider - S3/MinIO Archive (migration target)
  provider "s3-archive" {
    type        = "s3"
    is_primary  = false
    is_writable = true
    status      = "active"

    config {
      endpoint               = "http://minio:9000"
      region                = "us-east-1"
      bucket                = "hermes-documents"
      prefix                = "production"
      access_key            = "minioadmin"
      secret_key            = "minioadmin"
      versioning_enabled    = true
      metadata_store        = "manifest"
      use_ssl               = false
      path_template         = "{project}/{uuid}.md"
      default_mime_type     = "text/markdown"
      upload_concurrency    = 5
      download_concurrency  = 10
    }

    capabilities {
      versioning  = true
      permissions = false  // S3 doesn't natively support Hermes permissions - delegate to API
      search      = false  // S3 doesn't have search - use Meilisearch
      people      = false  // Delegate to central API
      teams       = false  // Delegate to central API
    }
  }

  // Tertiary provider - S3 Cold Storage (optional, for old documents)
  provider "s3-cold-storage" {
    type        = "s3"
    is_primary  = false
    is_writable = false  // Read-only archive
    status      = "active"

    config {
      endpoint               = "http://minio:9000"
      region                = "us-east-1"
      bucket                = "hermes-archive"
      prefix                = "archive"
      access_key            = "minioadmin"
      secret_key            = "minioadmin"
      versioning_enabled    = true
      metadata_store        = "manifest"
      use_ssl               = false
      path_template         = "{year}/{project}/{uuid}.md"
    }

    capabilities {
      versioning  = true
      permissions = false
      search      = false
      people      = false
      teams       = false
    }
  }
}

// RFC-089: Migration Configuration
// Defines migration strategy and scheduling
migration {
  enabled = true

  // Migration strategy for new documents
  // Options: "primary_only", "primary_and_replicate", "all_providers"
  write_strategy = "primary_only"

  // Where to read documents from
  // Options: "primary_only", "any_provider", "fastest_provider"
  read_strategy = "primary_only"

  // Automatic migration rules
  auto_migration {
    enabled = true

    // Migrate documents older than N days to archive
    rule "archive_old_documents" {
      enabled       = true
      source        = "local-primary"
      destination   = "s3-archive"
      schedule      = "0 2 * * *"  // Daily at 2 AM

      filter {
        status           = "WIP,In-Review,Approved,Obsolete"
        min_age_days     = 365      // Documents older than 1 year
        exclude_projects = []        // Empty = all projects
      }

      options {
        strategy          = "copy"        // "copy" or "move"
        validate          = true
        batch_size        = 100
        concurrency       = 5
        dry_run           = false
      }
    }

    // Migrate very old documents to cold storage
    rule "cold_storage" {
      enabled       = false  // Disabled by default
      source        = "s3-archive"
      destination   = "s3-cold-storage"
      schedule      = "0 3 * * 0"  // Weekly on Sunday at 3 AM

      filter {
        status       = "Obsolete"
        min_age_days = 730  // Documents older than 2 years
      }

      options {
        strategy    = "move"
        validate    = true
        batch_size  = 50
        concurrency = 3
      }
    }
  }

  // Manual migration job defaults
  defaults {
    concurrency          = 5
    batch_size           = 100
    validate_after       = true
    rollback_enabled     = true
    max_retries          = 3
    retry_delay_seconds  = 30
  }

  // Kafka/Redpanda configuration for migration task queue
  task_queue {
    brokers        = ["redpanda:9092"]
    topic          = "hermes.migration-tasks"
    consumer_group = "hermes-migration-workers"
    batch_size     = 10
    poll_interval  = "5s"
  }
}

// Provider selection - Use multi-provider setup
// Primary is "local-primary", with S3 as backup/archive
providers {
  workspace            = "multi"  // RFC-089: Multi-provider mode
  search               = "meilisearch"
  projects_config_path = "projects.hcl"
}

// Document types (same as central config)
document_types {
  document_type "RFC" {
    long_name   = "Request for Comments"
    description = "Create a Request for Comments document."
    flight_icon = "discussion-circle"
    template    = "template-rfc"
  }

  document_type "PRD" {
    long_name   = "Product Requirements"
    description = "Create a Product Requirements Document."
    flight_icon = "target"
    template    = "template-prd"
  }
}

// Email (disabled for testing)
email {
  enabled      = false
  from_address = "hermes-rfc089@example.com"
}

// Feature flags
feature_flags {
  flag "api_v2" {
    enabled = true
  }

  flag "projects" {
    enabled = false
  }

  flag "edge_document_sync" {
    enabled = true
  }

  // RFC-089: Enable multi-provider and migration features
  flag "multi_provider_storage" {
    enabled = true
  }

  flag "document_migration" {
    enabled = true
  }
}

// Google Workspace (placeholder - not used in RFC-089 testing)
google_workspace {
  create_doc_shortcuts = false
  docs_folder          = "test-docs"
  domain               = "hermes.local"
  drafts_folder        = "test-drafts"
  shortcuts_folder     = "test-shortcuts"

  auth {
    client_email        = "test@test-project.iam.gserviceaccount.com"
    create_docs_as_user = false
    private_key         = "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----\n"
    subject             = "test@hermes.local"
    token_url           = "https://oauth2.googleapis.com/token"
  }
}

// Indexer configuration
indexer {
  max_parallel_docs              = 5
  update_doc_headers             = false
  update_draft_headers           = false
  use_database_for_document_data = true

  redpanda_brokers = ["redpanda:9092"]
  topic            = "hermes.document-revisions"
  poll_interval    = "1s"
  batch_size       = 100
}

// Server configuration
server {
  addr = "0.0.0.0:8000"
}

// Database configuration
postgres {
  host     = "postgres"
  port     = 5432
  database = "hermes_testing"
  user     = "postgres"
  password = "postgres"
  sslmode  = "disable"
}
