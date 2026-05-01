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

  lane "public" {
    schema                   = "generic"
    roots                    = ["docs"]
    folders                  = ["."]
    filename_pattern         = ".+\\.md$"
    enforce_filename_pattern = false
    require_frontmatter      = false
    allowed_extensions       = ["md", "mdx"]
  }

  lane "adr" {
    schema                    = "adr"
    roots                     = ["docs-internal"]
    folders                   = ["adr"]
    filename_pattern          = "^adr-(\\d{3})-(.+)\\.md$"
    enforce_filename_pattern  = true
    require_frontmatter       = true
    allowed_extensions        = ["md"]
    skip_templates            = ["_template-*", "templates/**"]
  }

  lane "memo" {
    schema                    = "memo"
    roots                     = ["docs-internal"]
    folders                   = ["memo", "plans"]
    filename_pattern          = "^(memo|trajectory)-(\\d{3}|[0-9]+)-(.+)\\.md$"
    enforce_filename_pattern  = false
    require_frontmatter       = true
    allowed_extensions        = ["md"]
    skip_templates            = ["_template-*", "templates/**"]
  }

  # Project metadata
  metadata {
    created_at = "2025-10-22T00:00:00Z"
    owner      = "hermes-dev-team"
    tags       = ["documentation", "public", "cms"]
  }
}
