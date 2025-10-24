// Hermes Native Development Configuration
// For running backend natively (non-Docker) with local workspace
// Uses standard ports: PostgreSQL 5432, Meilisearch 7700, Dex 5556

base_url = "http://localhost:4200"
log_format = "standard"

// Algolia configuration (placeholder - using Meilisearch for search)
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
  env     = "local-native"
}

document_types {
  document_type "RFC" {
    long_name   = "Request for Comments"
    description = "Create an RFC document for technical proposals"
    flight_icon = "discussion-circle"
    template = "template-rfc"
    
    custom_field {
      name = "Current Version"
      type = "string"
    }
    custom_field {
      name = "Stakeholders"
      type = "people"
    }
  }
  
  document_type "PRD" {
    long_name   = "Product Requirements Document"
    description = "Create a PRD to define product requirements"
    flight_icon = "target"
    template = "template-prd"
    
    custom_field {
      name = "RFC"
      type = "string"
    }
    custom_field {
      name = "Stakeholders"
      type = "people"
    }
  }
}

// Dex OIDC authentication (running natively on port 5556)
dex {
  disabled      = false
  issuer_url    = "http://localhost:5556/dex"
  client_id     = "hermes-local"
  client_secret = "local-dev-secret"
  redirect_url  = "http://localhost:8000/auth/callback"
}

// Disable other auth providers
google_workspace {
  disabled = true
}

okta {
  disabled = true
}

// Provider selection
providers {
  workspace = "local"
  search    = "meilisearch"
}

// Server configuration
server {
  addr = "localhost:8000"
}

// PostgreSQL (native, standard port 5432)
postgres {
  dbname   = "hermes"
  host     = "localhost"
  password = "postgres"
  port     = 5432
  user     = "postgres"
}

// Meilisearch configuration (native, standard port 7700)
meilisearch {
  host              = "http://localhost:7700"
  api_key           = "masterKey123"
  docs_index_name   = "docs"
  drafts_index_name = "drafts"
  projects_index_name = "projects"
  links_index_name  = "links"
}

// Indexer configuration
indexer {
  max_parallel_docs          = 5
  update_doc_headers         = false
  update_draft_headers       = false
  use_database_for_document_data = true
}

// Jira integration (disabled for local development)
jira {
  enabled = false
}

// Local workspace configuration
local_workspace {
  base_path    = "/Users/jrepp/hc/hermes/testing/workspace_data"
  docs_path    = "/Users/jrepp/hc/hermes/testing/workspace_data/docs"
  drafts_path  = "/Users/jrepp/hc/hermes/testing/workspace_data/drafts"
  folders_path = "/Users/jrepp/hc/hermes/testing/workspace_data/folders"
  users_path   = "/Users/jrepp/hc/hermes/testing/workspace_data"
  tokens_path  = "/Users/jrepp/hc/hermes/testing/workspace_data"
  domain       = "hermes.local"
  
  smtp {
    enabled  = false
    host     = "localhost"
    port     = 1025
    username = ""
    password = ""
  }
}
