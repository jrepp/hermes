# Hermes Documentation Project
# Public-facing documentation for the open-source Hermes project

project "docs" {
  # Human-readable metadata
  title         = "Hermes Documentation (CMS)"
  friendly_name = "Hermes Documentation"
  short_name    = "DOCS"  # Used in document identifiers
  description   = "Public documentation for the open-source Hermes project"
  
  # Project status
  status = "active"
  
  # Local filesystem provider for CMS content
  provider "local" {
    migration_status = "active"
    
    # Relative to workspace_base_path (/app/workspaces in container)
    # Container: /app/workspaces/docs
    # Native dev: ./testing/workspaces/docs
    workspace_path = "docs"
    
    git {
      repository = "https://github.com/hashicorp-forge/hermes"
      branch     = "main"
    }
    
    # Indexing configuration
    indexing {
      enabled            = true
      allowed_extensions = ["md", "mdx"]
      public_read_access = true  # Public documentation
    }
  }
  
  # Project metadata
  metadata {
    created_at = "2025-10-22T00:00:00Z"
    owner      = "hermes-dev-team"
    tags       = ["documentation", "public", "cms"]
  }
}
