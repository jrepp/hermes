project "alpha" {
  title         = "Alpha Docs"
  friendly_name = "Alpha"
  short_name    = "ALPHA"
  status        = "active"

  provider "local" {
    workspace_path = "."
  }

  lane "adr" {
    schema                    = "adr"
    roots                     = ["alpha"]
    folders                   = ["adr"]
    filename_pattern          = "^adr-(\\d{3})-(.+)\\.md$"
    enforce_filename_pattern  = true
    require_frontmatter       = true
    allowed_extensions        = ["md"]
    skip_templates            = ["_template-*"]
  }

  lane "guides" {
    schema                    = "generic"
    roots                     = ["alpha"]
    folders                   = ["guides"]
    filename_pattern          = ".+\\.md$"
    enforce_filename_pattern  = false
    require_frontmatter       = false
    allowed_extensions        = ["md"]
  }
}
