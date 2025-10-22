# Hermes Projects Configuration
# This file imports individual project configurations

# Import individual project definitions
import "projects/testing.hcl"
import "projects/docs.hcl"

# Global settings for projects system
projects {
  # Version of the projects configuration schema
  version = "1.0.0-alpha"
  
  # Directory where project configs are stored
  config_dir = "./projects"
  
  # Default provider settings
  defaults {
    local {
      indexing_enabled = true
      git_branch       = "main"
    }
  }
}
