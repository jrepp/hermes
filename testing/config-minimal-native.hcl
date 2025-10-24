// Minimal Native Development Configuration
// For running backend natively without Docker
// Uses: PostgreSQL 5432, Meilisearch 7700, No auth (open mode for testing)

base_url = "http://localhost:4200"
log_format = "standard"

// Algolia configuration (placeholder - using Meilisearch)
algolia {
  application_id            = "test-app-id"
  docs_index_name           = "docs"
  drafts_index_name         = "drafts"
  internal_index_name       = "internal"
  links_index_name          = "links"
  missing_fields_index_name = "missing_fields"
  projects_index_name       = "projects"
  search_api_key            = "test-search-key"
  write_api_key             = "masterKey123"
}

datadog {
  enabled = false
}

document_types {
  document_type "RFC" {
    long_name   = "Request for Comments"
    description = "Create an RFC document"
    flight_icon = "discussion-circle"
    template = "template-rfc"
  }
  
  document_type "PRD" {
    long_name   = "Product Requirements Document"
    description = "Create a PRD"
    flight_icon = "target"
    template = "template-prd"
  }
}

// Products configuration (required)
products {
  product "Engineering" {
    abbreviation = "ENG"
  }
  
  product "Product" {
    abbreviation = "PM"
  }
}

// Disable all auth providers for quick testing
dex {
  disabled = true
}

// Google Workspace (required block, but disabled)
google_workspace {
  create_doc_shortcuts = false
  docs_folder          = "placeholder-docs"
  domain               = "placeholder.local"
  drafts_folder        = "placeholder-drafts"
  shortcuts_folder     = "placeholder-shortcuts"
  
  group_approvals {
    enabled = false
  }
  
  auth {
    client_email        = "placeholder@placeholder.iam.gserviceaccount.com"
    create_docs_as_user = false
    private_key         = "-----BEGIN PRIVATE KEY-----\nplaceholder\n-----END PRIVATE KEY-----\n"
    subject             = "placeholder@placeholder.local"
    token_url           = "https://oauth2.googleapis.com/token"
  }
  
  oauth2 {
    client_id    = "placeholder"
    hd           = "placeholder.local"
    redirect_uri = "http://localhost:8000/torii/redirect.html"
  }
}

okta {
  disabled = true
}

providers {
  workspace = "local"
  search    = "meilisearch"
}

server {
  addr = "localhost:8000"
}

// Native PostgreSQL
postgres {
  dbname   = "hermes"
  host     = "localhost"
  password = "postgres"
  port     = 5432
  user     = "postgres"
}

// Native Meilisearch  
meilisearch {
  host                = "http://localhost:7700"
  api_key             = "masterKey123"
  docs_index_name     = "docs"
  drafts_index_name   = "drafts"
  projects_index_name = "projects"
  links_index_name    = "links"
}

indexer {
  max_parallel_docs              = 5
  update_doc_headers             = false
  update_draft_headers           = false
  use_database_for_document_data = true
}

jira {
  enabled = false
}

local_workspace {
  base_path    = "/Users/jrepp/hc/hermes/testing/workspace_data"
  docs_path    = "/Users/jrepp/hc/hermes/testing/workspace_data/docs"
  drafts_path  = "/Users/jrepp/hc/hermes/testing/workspace_data/drafts"
  folders_path = "/Users/jrepp/hc/hermes/testing/workspace_data/folders"
  users_path   = "/Users/jrepp/hc/hermes/testing/workspace_data/users"
  tokens_path  = "/Users/jrepp/hc/hermes/testing/workspace_data/tokens"
  domain       = "hermes.local"
  
  smtp {
    enabled  = false
    host     = "localhost"
    port     = 1025
    username = ""
    password = ""
  }
}
