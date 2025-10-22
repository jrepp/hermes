# Example: RFC Project with Google Workspace Provider
# This is a TEMPLATE - not active in OSS deployment

project "rfcs" {
  title         = "Request for Comments"
  friendly_name = "RFC"
  short_name    = "RFC"  # Used in RFC-001, RFC-002, etc.
  description   = "Technical design documents and proposals"
  
  status = "active"
  
  # Google Workspace provider configuration
  provider "google" {
    migration_status = "active"
    
    # Google Workspace configuration
    workspace_id          = env("GOOGLE_WORKSPACE_ID")  # Never hardcode!
    service_account_email = env("GOOGLE_SERVICE_ACCOUNT_EMAIL")
    credentials_path      = env("GOOGLE_CREDENTIALS_PATH")
    
    # Shared drives to index
    shared_drive_ids = [
      env("GOOGLE_RFCS_DRIVE_ID"),
    ]
    
    # Indexing configuration
    indexing {
      enabled = true
    }
  }
  
  metadata {
    created_at = "2025-01-01T00:00:00Z"
    owner      = "platform-team"
    tags       = ["rfc", "google-workspace", "template"]
    notes      = "TEMPLATE ONLY - Replace env vars with actual values in production"
  }
}
