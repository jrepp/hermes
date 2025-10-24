# Local Hermes Configuration Example
# This config runs Hermes in local mode with SQLite and local workspace

# Server configuration
server {
  # Bind to localhost only (local development)
  addr = "127.0.0.1:8000"
  
  # Base URL for generating links
  base_url = "http://localhost:8000"
  
  # Development mode (disables some auth checks)
  dev_mode = true
}

# Database configuration (SQLite for local storage)
database {
  # Use SQLite for local development (no PostgreSQL needed)
  driver = "sqlite"
  
  # SQLite database file location (relative to workspace root)
  dsn = ".hermes/data/hermes.db"
  
  # Auto-run migrations on startup
  auto_migrate = true
}

# Workspace provider (local filesystem)
workspace {
  provider = "local"
  
  local {
    # Base path for document storage (relative to workspace root)
    base_path = ".hermes/workspace_data"
    
    # Document type directories
    docs_dir   = "docs"
    drafts_dir = "drafts"
  }
}

# Search provider (Meilisearch or none for local dev)
search {
  provider = "meilisearch"  # or "none" to disable search
  
  meilisearch {
    # Local Meilisearch instance
    url = "http://localhost:7700"
    key = "masterKey123"
    
    # Index names
    docs_index     = "docs"
    drafts_index   = "drafts"
    projects_index = "projects"
  }
}

# Authentication (disabled for local dev)
auth {
  # Use local dev mode auth (no OIDC/OAuth)
  provider = "dev"
  
  # Dev mode users (loaded from users.json)
  dev {
    users_file = ".hermes/users.json"
  }
}

# Instance identity (for distributed mode)
instance {
  # Unique identifier for this Hermes instance
  # This is automatically generated on first run
  # Can be manually set for multi-instance setups
  # instance_id = "local-workspace-001"
  
  # Environment (development, staging, production)
  environment = "development"
}

# Indexer configuration (for syncing to central Hermes)
indexer {
  # Disable indexer for standalone local mode
  enabled = false
  
  # Enable to sync to central Hermes:
  # enabled        = true
  # central_url    = "https://hermes.company.com"
  # workspace_path = ".hermes/workspace_data"
  # type           = "local-workspace"
  # heartbeat_interval = "5m"
}

# Projects configuration (load from HCL file)
projects {
  # Load project definitions from projects.hcl
  config_file = ".hermes/projects.hcl"
  
  # Auto-sync projects to database on startup
  auto_sync = true
}

# Document types (optional customization)
document_types {
  # Use default document types (RFC, PRD, FRD)
  use_defaults = true
  
  # Add custom document types:
  # custom {
  #   type = "ADR"
  #   name = "Architecture Decision Record"
  #   description = "Document key architectural decisions"
  #   long_name = "Architecture Decision Record"
  # }
}

# Logging
log {
  level = "info"  # debug, info, warn, error
  format = "text" # text or json
}
