// Local workspace project definition
// This file defines projects for local Hermes instance

project "my-app" {
  title         = "My Application"
  friendly_name = "My App"
  short_name    = "myapp"
  description   = "Main application project"
  status        = "active"
  
  // Local workspace provider
  provider "local" {
    workspace_path = ".hermes/workspace_data"
    docs_dir       = "docs/my-app"
    drafts_dir     = "drafts/my-app"
  }
  
  metadata {
    owner      = "local-developer"
    team       = "Engineering"
    repository = "https://github.com/company/my-app"
    created_at = "2025-10-24"
    
    tags = ["backend", "api", "microservices"]
    
    notes = "Local development project for My Application"
  }
}

project "docs" {
  title         = "Documentation"
  friendly_name = "Project Documentation"
  short_name    = "docs"
  description   = "General documentation and guides"
  status        = "active"
  
  provider "local" {
    workspace_path = ".hermes/workspace_data"
    docs_dir       = "docs/general"
    drafts_dir     = "drafts/general"
  }
  
  metadata {
    owner = "local-developer"
    team  = "Engineering"
    
    tags = ["documentation", "guides", "how-to"]
    
    notes = "General project documentation"
  }
}
