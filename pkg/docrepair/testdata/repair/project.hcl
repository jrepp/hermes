project "repair" {
  title         = "Repair Docs"
  friendly_name = "Repair"
  short_name    = "REPAIR"
  status        = "active"
  provider "local" { workspace_path = "." }

  lane "memo" {
    schema                    = "memo"
    roots                     = ["docs"]
    folders                   = ["."]
    filename_pattern          = ".+\\.md$"
    enforce_filename_pattern  = false
    require_frontmatter       = true
    allowed_extensions        = ["md"]
  }
}
