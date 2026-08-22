// Per-site configuration fragment — example.
//
// Site blocks normally live directly in local/config.hcl. Keep them in
// separate files here when a deployment has enough sites that one file becomes
// unwieldy, and concatenate or include them as your workflow requires.
//
// Files named *.example.hcl are committed; everything else in this directory
// is gitignored.

site "docs.jrepp.com" {
  // Additional hostnames for this same tenant. An alias shares the site's
  // schema and workspace; it is a second name, not a second site.
  aliases = ["www.jrepp.com"]

  // Externally reachable origin. Used for absolute links and OAuth redirects.
  base_url = "https://docs.jrepp.com"

  // Both default from the hostname; set them only to adopt existing storage.
  // workspace_path = "/srv/hermes/workspace/docs.jrepp.com"
  // schema_name    = "site_docs_jrepp_com"

  // Take the site out of service without deleting its config or data.
  // disabled = true
}
