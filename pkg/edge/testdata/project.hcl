project "edge" {
  title         = "Edge Docs"
  friendly_name = "Edge"
  short_name    = "EDGE"
  status        = "active"

  provider "local" {
    migration_status = "active"
    workspace_path   = "."
  }

  provider "remote-hermes" {
    migration_status = "target"
    hermes_url        = "http://127.0.0.1:1"
    api_version       = "v2"
  }

  lane "memo" {
    schema                   = "memo"
    roots                    = ["docs"]
    folders                  = ["."]
    filename_pattern         = ".+\\.md$"
    enforce_filename_pattern = false
    require_frontmatter      = true
    allowed_extensions       = ["md"]
  }
}
