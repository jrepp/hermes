// Hermes local configuration — example.
//
// Copy this to local/config.hcl and edit. Everything under local/ except this
// file, local/readme.md, and local/sites/*.example.hcl is gitignored, so real
// hostnames, credentials, and workspace data never reach the repository.
//
//   cp local/config.example.hcl local/config.hcl
//   ./build/bin/hermes server -config=local/config.hcl

// Fallback origin for links generated outside a request, where no site is in
// scope. Per-site links use that site's own base_url.
base_url   = "https://docs.jrepp.com"
log_format = "standard"

// ---------------------------------------------------------------------------
// Sites
//
// One listener, many subdomains. Each site block is an isolated tenant: its
// own PostgreSQL schema, its own workspace directory, its own search index
// namespace, and its own session scope. The block label is the hostname, and
// it is canonicalized (lowercased, port stripped, IDN punycoded) so the label
// you write here and the Host header that arrives at runtime always agree.
// ---------------------------------------------------------------------------

site "docs.jrepp.com" {
  // Additional hostnames routed to this same tenant. An alias is a second
  // name for one site, not a second site: it shares the same storage.
  aliases  = ["www.jrepp.com"]
  base_url = "https://docs.jrepp.com"

  // Defaults, shown for reference — omit them and Hermes derives the same
  // values from the hostname:
  //   workspace_path = "<local_workspace.base_path>/docs.jrepp.com"
  //   schema_name    = "site_docs_jrepp_com"
  //
  // Set schema_name only to adopt a pre-existing schema.
}

site "notes.jrepp.com" {
  base_url = "https://notes.jrepp.com"
}

// Hostnames matching no site above are rejected with 421 Misdirected Request.
// Naming a default routes them to that site instead. With exactly one site
// configured, that site is the default automatically.
// default_site = "docs.jrepp.com"

// ---------------------------------------------------------------------------
// Reverse proxy
//
// nginx terminates TLS and forwards to Hermes on loopback. X-Forwarded-Host is
// honored only from these addresses; from anywhere else the header is
// attacker-controlled and ignored, since believing it would let any client
// choose which tenant to read.
//
// Prefer `proxy_set_header Host $host;` in nginx so the real Host arrives
// directly. This list is the safety net for proxies that do not rewrite Host.
// ---------------------------------------------------------------------------

trusted_proxies = ["127.0.0.1/32", "::1/128"]

// ---------------------------------------------------------------------------
// Sessions
//
// The session cookie is an HMAC-signed token naming the user, the site it was
// issued for, and an expiry. It is host-only, HttpOnly, SameSite=Lax, and
// marked Secure whenever the site's base_url is https — which is why base_url
// matters even behind nginx: r.TLS is nil on a loopback connection, so the
// scheme configured here is the only thing that knows the browser is on HTTPS.
//
// Leave `key` as-is and set HERMES_SESSION_KEY in local/secrets.env instead.
// Placeholder values are refused; Hermes generates an ephemeral key and warns,
// which logs everyone out on restart rather than signing with a secret that is
// published in this repository. Generate a real one with:
//
//   openssl rand -base64 32
//
// The same key must be used by every instance serving these sites, or a
// session issued by one will not verify at another.
// ---------------------------------------------------------------------------

session {
  key = "change-me"
  ttl = "168h"
}

server {
  // Bind to loopback only. nginx is the sole public listener.
  addr = "127.0.0.1:8000"
}

// ---------------------------------------------------------------------------
// Storage
// ---------------------------------------------------------------------------

// Multi-site hosting requires the local workspace provider. Google Workspace
// and SharePoint address one tenant's storage, so every site would share it;
// Hermes refuses to start rather than let that happen silently.
local_workspace {
  // Per-site subdirectories are created under this path, one per hostname:
  //   local/workspace/docs.jrepp.com/{docs,drafts,folders}
  //   local/workspace/notes.jrepp.com/{docs,drafts,folders}
  base_path = "local/workspace"
  domain    = "jrepp.com" // email domain for generated addresses

  smtp {
    enabled = false
  }
}

postgres {
  host     = "localhost"
  port     = 5432
  dbname   = "hermes"
  user     = "hermes"
  // Overridden by HERMES_SERVER_POSTGRES_PASSWORD. Keep the real value in
  // local/secrets.env rather than here; see local/readme.md.
  password = "change-me"
}

providers {
  workspace = "local"
  search    = "meilisearch"
}

// Index names are prefixed per site, so docs.jrepp.com writes to
// docs_jrepp_com_docs and notes.jrepp.com to notes_jrepp_com_docs. The names
// below are the base; you do not add the prefix yourself.
//
// Multi-site hosting requires meilisearch or bleve. The Algolia path proxies
// frontend searches through a handler that is not site-aware, so Hermes
// refuses to start with sites configured and algolia selected rather than
// serving one tenant's index to another.
meilisearch {
  host                = "http://127.0.0.1:7700"
  api_key             = "change-me"
  docs_index_name     = "docs"
  drafts_index_name   = "drafts"
  projects_index_name = "projects"
  links_index_name    = "links"
}

// ---------------------------------------------------------------------------
// Authentication
// ---------------------------------------------------------------------------

// Each site sends users back to its own callback, derived from that site's
// base_url. The value below is only the fallback for a deployment with no site
// blocks. Register every site's callback on the Dex client:
//
//   staticClients:
//     - id: hermes
//       redirectURIs:
//         - https://docs.jrepp.com/auth/callback
//         - https://notes.jrepp.com/auth/callback
//
// OIDC requires the redirect_uri to match exactly, so a missing entry shows up
// as an "invalid redirect URI" error from Dex, not from Hermes.
dex {
  disabled      = false
  issuer_url    = "https://auth.jrepp.com/dex"
  client_id     = "hermes"
  client_secret = "change-me"
  redirect_url  = "https://docs.jrepp.com/auth/callback"
}

okta {
  disabled = true
}

// google_workspace is omitted entirely. The block requires docs_folder and
// several other arguments even when disabled, so leaving it out is how you
// turn Google Workspace off.

indexer {
  max_parallel_docs              = 5
  update_doc_headers             = false
  update_draft_headers           = false
  use_database_for_document_data = true
}

document_types {
  document_type "RFC" {
    long_name   = "Request for Comments"
    description = "Create an RFC document"
    flight_icon = "discussion-circle"
    template    = "template-rfc"
  }
}
