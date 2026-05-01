project "beta" {
  title         = "Beta Docs"
  friendly_name = "Beta"
  short_name    = "BETA"
  status        = "active"

  provider "local" {
    workspace_path = "."
  }

  lane "memo" {
    schema                    = "memo"
    roots                     = ["beta", "shared"]
    folders                   = ["memo"]
    filename_pattern          = "^memo-(\\d{3})-(.+)\\.md$"
    enforce_filename_pattern  = true
    require_frontmatter       = true
    allowed_extensions        = ["md"]
    skip_templates            = ["_template-*"]
  }
}
