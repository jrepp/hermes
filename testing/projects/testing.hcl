# Hermes Testing Environment Project
# Short name used in document IDs and URLs

project "testing" {
  # Human-readable metadata
  title         = "Hermes Testing Environment"
  friendly_name = "Hermes Testing"
  short_name    = "TEST"  # Used in document identifiers like TEST-001
  description   = "Local testing workspace for Hermes development"
  
  # Project status
  status = "active"
  
  # Local filesystem provider
  provider "local" {
    migration_status = "active"
    
    # Relative to workspace_base_path (/app/workspaces in container)
    # Container: /app/workspaces/testing
    # Native dev: ./testing/workspaces/testing
    workspace_path = "testing"
    
    git {
      repository = "https://github.com/hashicorp-forge/hermes"
      branch     = "main"
    }
    
    # Indexing configuration
    indexing {
      enabled = true
      allowed_extensions = ["md", "txt", "json", "yaml", "yml"]
    }
  }
  
  # Project metadata
  metadata {
    created_at = "2025-10-22T00:00:00Z"
    owner      = "hermes-dev-team"
    tags       = ["testing", "development", "local"]
  }
}
