# Example: Migration Scenario - Google to Local
# Shows a project with BOTH source (Google) and target (local) providers
# This is a TEMPLATE - not active in OSS deployment

project "platform-docs" {
  title         = "Platform Documentation"
  friendly_name = "Platform Docs"
  short_name    = "PLAT"
  description   = "Internal platform documentation (migrating from Google to Git)"
  
  status = "active"
  
  # SOURCE: Google Workspace (being migrated FROM)
  provider "google" {
    migration_status   = "source"
    migration_started  = "2025-09-01T00:00:00Z"
    
    workspace_id          = env("GOOGLE_WORKSPACE_ID")
    service_account_email = env("GOOGLE_SERVICE_ACCOUNT_EMAIL")
    credentials_path      = env("GOOGLE_CREDENTIALS_PATH")
    
    shared_drive_ids = [
      env("GOOGLE_PLATFORM_DRIVE_ID"),
    ]
    
    indexing {
      enabled = true
    }
  }
  
  # TARGET: Local Git (being migrated TO)
  provider "local" {
    migration_status   = "target"
    migration_started  = "2025-09-01T00:00:00Z"
    
    workspace_path = "./migrated-platform-docs"
    
    git {
      repository = env("GIT_PLATFORM_DOCS_REPO")
      branch     = "main"
    }
    
    indexing {
      enabled            = true
      allowed_extensions = ["md"]
    }
  }
  
  # Migration tracking configuration
  migration {
    # Track content hash for conflict detection
    conflict_detection_enabled = true
    
    # Sync interval during migration
    sync_interval = "1h"
    
    # Notification settings
    notify_on_conflict = true
    notification_email = env("MIGRATION_NOTIFY_EMAIL")
  }
  
  metadata {
    created_at = "2025-09-01T00:00:00Z"
    owner      = "platform-team"
    tags       = ["migration", "google-to-git", "template"]
    notes      = "TEMPLATE: Shows dual-provider setup during active migration"
  }
}
