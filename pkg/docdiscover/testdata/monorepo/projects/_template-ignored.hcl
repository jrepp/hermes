project "ignored" {
  title         = "Ignored"
  friendly_name = "Ignored"
  short_name    = "IGN"
  status        = "active"
  provider "local" { workspace_path = "." }
}
