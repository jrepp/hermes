# Example: Remote Hermes Federation
# Shows how to federate with another Hermes instance
# This is a TEMPLATE - not active in OSS deployment

project "enterprise-arch" {
  title         = "Enterprise Architecture"
  friendly_name = "Architecture"
  short_name    = "ARCH"
  description   = "Enterprise architecture documents from internal Hermes instance"
  
  status = "active"
  
  # Remote Hermes provider (federation)
  provider "remote-hermes" {
    migration_status = "active"
    
    # Remote Hermes instance configuration
    hermes_url  = env("REMOTE_HERMES_URL")  # e.g., "https://hermes.internal.example.com"
    api_version = "v2"
    
    # Authentication configuration
    authentication {
      method         = "oidc"
      client_id      = env("FEDERATION_CLIENT_ID")
      client_secret  = env("FEDERATION_CLIENT_SECRET")
      token_endpoint = env("OIDC_TOKEN_ENDPOINT")
    }
    
    # Sync configuration
    sync_mode = "read-only"  # or "bidirectional"
    cache_ttl = 3600         # Cache for 1 hour
    
    # Project filter (which projects to sync from remote)
    project_filter = ["arch-*", "platform-*"]
  }
  
  metadata {
    created_at = "2025-10-01T00:00:00Z"
    owner      = "enterprise-team"
    tags       = ["federation", "remote", "template"]
    notes      = "TEMPLATE: Shows federation with internal corporate Hermes"
  }
}
