import "projects/alpha.hcl"
import "projects/beta.hcl"
import "projects/_template-ignored.hcl"

projects {
  version             = "1.0.0-alpha"
  config_dir          = "./projects"
  workspace_base_path = "."
}
