// Edge Hermes Configuration
// This is an edge Hermes instance that authors documents locally
// and delegates directory/permission/notification operations to central Hermes
//
// NOTE: This config currently uses "local" provider since multi-provider is not implemented yet
// TODO: Change to "multiprovider" once RFC-085 multi-provider manager is implemented
//
// Future configuration will be:
//   providers { workspace = "multiprovider" }
//   multiprovider {
//     primary { type = "local" }
//     secondary { type = "api"; api_workspace { base_url = "http://hermes-central:8000" } }
//   }

// Base URL for the application (edge instance frontend)
base_url = "http://localhost:4202"

// Logging format
log_format = "standard"

// Algolia configuration (placeholder for Meilisearch compatibility)
algolia {
  application_id            = "edge-app-id"
  docs_index_name           = "docs"
  drafts_index_name         = "drafts"
  internal_index_name       = "internal"
  links_index_name          = "links"
  missing_fields_index_name = "missing_fields"
  projects_index_name       = "projects"
  search_api_key            = "test-search-key"
  write_api_key             = "masterKey123"
}

// Datadog (disabled for testing)
datadog {
  enabled = false
  env     = "edge-testing"
}

// Document types - same as central for consistency
document_types {
  document_type "RFC" {
    long_name   = "Request for Comments"
    description = "Create a Request for Comments document to present a proposal to colleagues for their review and feedback."
    flight_icon = "discussion-circle"
    template    = "template-rfc"

    more_info_link {
      text = "More info on the RFC template"
      url  = "https://works.hashicorp.com/articles/rfc-template"
    }

    custom_field {
      name      = "Current Version"
      type      = "string"
      read_only = false
    }
    custom_field {
      name      = "PRD"
      type      = "string"
      read_only = false
    }
    custom_field {
      name      = "Stakeholders"
      type      = "people"
      read_only = false
    }
    custom_field {
      name      = "Target Version"
      type      = "string"
      read_only = false
    }

    check {
      label       = "I have updated the status to 'In Review'"
      helper_text = "Documents must be in review status before publishing"
      link {
        text = "Status guide"
        url  = "https://works.hashicorp.com/articles/rfc-status"
      }
    }
  }

  document_type "PRD" {
    long_name   = "Product Requirements"
    description = "Create a Product Requirements Document to summarize a problem statement and outline a phased approach to addressing the problem."
    flight_icon = "target"
    template    = "template-prd"

    more_info_link {
      text = "More info on the PRD template"
      url  = "https://works.hashicorp.com/articles/prd-template"
    }

    custom_field {
      name      = "RFC"
      type      = "string"
      read_only = false
    }
    custom_field {
      name      = "Stakeholders"
      type      = "people"
      read_only = false
    }
    custom_field {
      name      = "Target Release"
      type      = "string"
      read_only = false
    }
  }

  document_type "ADR" {
    long_name   = "Architectural Decision Record"
    description = "Document an architectural decision including context, alternatives considered, and rationale for the chosen solution."
    flight_icon = "layers"
    template    = "template-adr"

    more_info_link {
      text = "Learn about ADRs"
      url  = "https://adr.github.io/"
    }

    custom_field {
      name      = "Status"
      type      = "string"
      read_only = false
    }
    custom_field {
      name      = "Decision Owners"
      type      = "people"
      read_only = false
    }
    custom_field {
      name      = "Related RFCs"
      type      = "string"
      read_only = false
    }
    custom_field {
      name      = "Systems Impacted"
      type      = "string"
      read_only = false
    }

    check {
      label       = "I have documented all alternatives considered"
      helper_text = "ADRs should include at least 2-3 alternative approaches"
    }
    check {
      label       = "I have clearly stated the consequences of this decision"
      helper_text = "Include both positive and negative consequences"
    }
  }

  document_type "FRD" {
    long_name   = "Functional Requirements Document"
    description = "Create detailed functional specifications for engineering implementation, including technical requirements and acceptance criteria."
    flight_icon = "docs-link"
    template    = "template-frd"

    more_info_link {
      text = "FRD best practices"
      url  = "https://works.hashicorp.com/articles/frd-template"
    }

    custom_field {
      name      = "Related PRD"
      type      = "string"
      read_only = false
    }
    custom_field {
      name      = "Engineers"
      type      = "people"
      read_only = false
    }
    custom_field {
      name      = "Epic Link"
      type      = "string"
      read_only = false
    }
  }

  document_type "Memo" {
    long_name   = "Memo"
    description = "Create a Memo document to share an idea, update, or brief note with colleagues."
    flight_icon = "radio"
    template    = "template-memo"

    custom_field {
      name      = "Distribution List"
      type      = "people"
      read_only = false
    }
    custom_field {
      name      = "Category"
      type      = "string"
      read_only = false
    }
  }

  document_type "PATH" {
    long_name   = "Golden Path"
    description = "Create a Golden Path document to provide step-by-step guidance for repeatable workflows and processes."
    flight_icon = "map"
    template    = "template-path"

    more_info_link {
      text = "Golden Paths overview"
      url  = "https://works.hashicorp.com/articles/golden-paths"
    }

    custom_field {
      name      = "Category"
      type      = "string"
      read_only = false
    }
    custom_field {
      name      = "Time Investment"
      type      = "string"
      read_only = false
    }
    custom_field {
      name      = "Steps"
      type      = "string"
      read_only = false
    }
    custom_field {
      name      = "Related Paths"
      type      = "string"
      read_only = false
    }

    check {
      label       = "I have documented all prerequisites"
      helper_text = "Include both required and helpful prerequisites"
    }
    check {
      label       = "I have provided time estimates for each step"
      helper_text = "Help users plan their time effectively"
    }
    check {
      label       = "I have included working examples"
      helper_text = "Real examples help users understand the path"
    }
  }
}

// Email (disabled for testing)
email {
  enabled      = false
  from_address = "hermes-edge@example.com"
}

// Feature flags
feature_flags {
  flag "api_v2" {
    enabled = true
  }

  flag "projects" {
    enabled = false
  }
}

// Google Workspace configuration (not used by edge - delegates to central)
google_workspace {
  create_doc_shortcuts = false
  docs_folder          = "edge-docs-folder-id"
  domain               = "hermes.local"
  drafts_folder        = "edge-drafts-folder-id"
  shortcuts_folder     = "edge-shortcuts-folder-id"

  group_approvals {
    enabled = false
  }

  auth {
    client_email        = "edge@test-project.iam.gserviceaccount.com"
    create_docs_as_user = false
    private_key         = "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC0test\n-----END PRIVATE KEY-----\n"
    subject             = "edge@hermes.local"
    token_url           = "https://oauth2.googleapis.com/token"
  }

  oauth2 {
    client_id    = "edge-client-id"
    hd           = "hermes.local"
    redirect_uri = "http://localhost:8002/torii/redirect.html"
  }
}

// Indexer configuration
indexer {
  max_parallel_docs              = 5
  update_doc_headers             = false
  update_draft_headers           = false
  use_database_for_document_data = true
}

// Jira (disabled for testing)
jira {
  enabled   = false
  api_token = ""
  url       = ""
  user      = ""
}

// Meilisearch configuration (shared with central for global search)
meilisearch {
  host                = "http://meilisearch:7700"
  api_key             = "masterKey123"
  docs_index_name     = "docs"
  drafts_index_name   = "drafts"
  projects_index_name = "projects"
  links_index_name    = "links"
}

// Dex OIDC authentication (edge uses different redirect URL)
dex {
  disabled      = false
  issuer_url    = "http://localhost:5558/dex"
  client_id     = "hermes-testing"
  client_secret = "dGVzdGluZy1hcHAtc2VjcmV0"
  redirect_url  = "http://localhost:8002/auth/callback"
}

// Okta authentication (disabled - using Dex instead)
okta {
  disabled        = true
  auth_server_url = "https://test.okta.com"
  aws_region      = "us-east-1"
  client_id       = "test-client-id"
  jwt_signer      = "test-jwt-signer"
}

// PostgreSQL configuration (shared database with central)
postgres {
  dbname   = "hermes_testing"
  host     = "postgres"
  port     = 5432
  user     = "postgres"
  password = "postgres"
}

// Products - same as central
products {
  product "Engineering" {
    abbreviation = "ENG"
  }

  product "Labs" {
    abbreviation = "LAB"
  }

  product "Platform" {
    abbreviation = "PLT"
  }

  product "Security" {
    abbreviation = "SEC"
  }

  product "Infrastructure" {
    abbreviation = "INF"
  }

  product "Product Management" {
    abbreviation = "PM"
  }

  product "Design" {
    abbreviation = "DES"
  }
}

// Local workspace configuration
// Edge Hermes uses local workspace for document authoring
// Separate workspace path from central to avoid conflicts
local_workspace {
  base_path    = "/app/workspace_data"
  docs_path    = "/app/workspace_data/docs"
  drafts_path  = "/app/workspace_data/drafts"
  folders_path = "/app/workspace_data/folders"
  users_path   = "/app/workspace_data/users"
  tokens_path  = "/app/workspace_data/tokens"
  domain       = "hermes.local"

  smtp {
    enabled  = false
    host     = "localhost"
    port     = 1025
    username = ""
    password = ""
  }
}

// Provider selection - Currently using local, will change to multiprovider
// TODO: Once multi-provider manager is implemented, change this to:
//   providers { workspace = "multiprovider" }
providers {
  workspace            = "local"
  search               = "meilisearch"
  projects_config_path = "projects.hcl"
}

// Server configuration (internal port 8000, mapped to 8002 externally)
server {
  addr = "0.0.0.0:8000"
}

// Edge instance configuration (for future multi-provider sync)
// Environment variables:
//   HERMES_CENTRAL_URL=http://hermes-central:8000
//   HERMES_EDGE_INSTANCE_ID=edge-dev-1
//   HERMES_API_TOKEN=<auth token for central>
