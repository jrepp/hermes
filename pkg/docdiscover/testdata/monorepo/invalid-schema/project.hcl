project "invalid" {
  title         = "Invalid"
  friendly_name = "Invalid"
  short_name    = "INV"
  status        = "active"
  provider "local" { workspace_path = "." }

  lane "weird" {
    schema              = "weird"
    roots               = ["."]
    folders             = ["docs"]
    require_frontmatter = true
  }
}
