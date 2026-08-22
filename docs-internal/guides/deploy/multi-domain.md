---
title: Multi-Domain Deployment
type: Guide
status: Reference
tags: [deployment, multi-tenancy, nginx, systemd, postgresql, sessions]
related: [rfc/rfc-023-multi-domain-hosting-and-signed-sessions.md, adr/adr-012-multi-provider-auth-architecture.md]
created: 2026-08-22
author: Hermes Team
project_id: hermes
doc_uuid: 281001e7-465e-4665-a19f-9b503c3af2bd
---

# Multi-Domain Deployment

How to install and run one Hermes process serving several subdomains as
isolated tenants, behind nginx.

The worked example serves `docs.jrepp.com` and `notes.jrepp.com` from one host.
Substitute your own hostnames throughout.

> **Design rationale** lives in [RFC-023](../../rfc/rfc-023-multi-domain-hosting-and-signed-sessions.md).
> **Configuration reference** lives in [`local/readme.md`](../../../local/readme.md).
> This guide is the operational procedure.

## What isolation you get

| Resource | Isolated? | How |
|---|---|---|
| Database | Yes | One PostgreSQL schema per site, one connection pool each — or a separate server per site |
| Row ownership | Yes | `domain` column on every top-level object table, constrained per schema |
| Documents | Yes | One directory per site under `local_workspace.base_path` |
| Search index | Yes | Index names prefixed per site (`docs_jrepp_com_docs`) |
| Sessions | Yes | Host-only cookie, and the signed token names its site |

## Prerequisites

On the server:

- **PostgreSQL 13+** with three extensions available: `vector` (pgvector),
  `citext`, and `uuid-ossp`. The last two are in `postgresql-contrib`;
  pgvector ships separately.

  ```bash
  sudo apt install postgresql-16 postgresql-contrib postgresql-16-pgvector
  ```

  Hermes installs the extensions into the `public` schema on first run, which
  needs a role with rights to do so. If your Hermes role is not a superuser,
  create them yourself once, as one:

  ```sql
  CREATE EXTENSION IF NOT EXISTS vector      SCHEMA public;
  CREATE EXTENSION IF NOT EXISTS citext      SCHEMA public;
  CREATE EXTENSION IF NOT EXISTS "uuid-ossp" SCHEMA public;
  ```

  They must be in `public`. An extension lives in exactly one schema, and every
  site's `search_path` ends in `public` precisely so their types resolve. If one
  is installed elsewhere, Hermes refuses to start and tells you how to move it.

- **nginx** and **certbot**, for TLS.
- **Meilisearch** (or Bleve, which is embedded and needs nothing installed).
  Algolia is not supported for multi-site — see [Required providers](#required-providers).
- **An identity provider** — Dex, Okta, or Google Workspace.

DNS: point every hostname you intend to serve at the server, including aliases.

## Required providers

Multi-site hosting constrains two provider choices, and Hermes refuses to start
rather than let either one leak:

| Provider | Multi-site | Why |
|---|---|---|
| workspace = `local` | Supported | Roots each site's documents at `<base_path>/<domain>` |
| workspace = `google` / `sharepoint` | Refused | Each addresses one tenant's storage, so every site would share it |
| search = `meilisearch` / `bleve` | Supported | Index names and index directories are prefixed per site |
| search = `algolia` | Refused | The frontend queries Algolia through a proxy that is not site-aware |

To use Google Workspace or Algolia, run one process per site.

## 1. Create the database

```sql
CREATE ROLE hermes LOGIN PASSWORD 'use-a-real-one';
CREATE DATABASE hermes OWNER hermes;
```

One database holds every site. Sites are separated by schema, which Hermes
creates on demand — you do not create them by hand.

### Putting a site on its own database

The default is one database, one schema per site. A site can name its own
server instead:

```hcl
site "notes.jrepp.com" {
  database {
    host    = "notes-db.internal"
    dbname  = "hermes_notes"
    sslmode = "require"
  }
}
```

Unset fields inherit from the global `postgres` block, so overriding only
`host` moves the site to another server with the same credentials. Supply each
site's password as `HERMES_SITE_<SLUG>_POSTGRES_PASSWORD` — for
`notes.jrepp.com` that is `HERMES_SITE_NOTES_JREPP_COM_POSTGRES_PASSWORD` —
rather than writing it into the config.

Every database a site uses needs the three extensions from the prerequisites,
not just the first one.

### How a row knows which site owns it

Each site's schema has a `site_identity` table naming its owner, and every
top-level object table carries a `domain` column fixed to that owner by a
`CHECK` constraint.

The schema already isolates tenants; this is what keeps the isolation
*checkable*. A schema is only a namespace — restore a dump into the wrong one,
or point `schema_name` at a schema already in use, and nothing in the data
would object. With the stamp, that becomes an error rather than a silent merge,
and a bare dump can be traced to its owner.

The application never writes the column: it has a per-schema `DEFAULT` and the
models do not know it exists, so no code path can set it wrong. Deployments
with no `site` blocks get neither the column nor the table.

## 2. Build

The Go binaries embed the frontend, so build the frontend first or you will
ship a placeholder page.

```bash
make web/build      # yarn install + ember build -> web/dist
make build/linux    # static linux/amd64 binaries -> build/bin/
```

`build/linux` writes a stub `web/dist/index.html` only when none exists, so the
order matters. If you are unsure what you have, check that
`web/dist/index.html` is larger than a few hundred bytes.

Copy `build/bin/hermes` and `build/bin/hermes-migrate` to the server, e.g.
`/opt/hermes/bin/`.

## 3. Configure

```bash
cp local/config.example.hcl local/config.hcl
$EDITOR local/config.hcl
```

The parts that matter for multi-domain:

```hcl
site "docs.jrepp.com" {
  aliases  = ["www.jrepp.com"]
  base_url = "https://docs.jrepp.com"
}

site "notes.jrepp.com" {
  base_url = "https://notes.jrepp.com"
}

trusted_proxies = ["127.0.0.1/32", "::1/128"]

session {
  ttl = "168h"
}

server {
  addr = "127.0.0.1:8000"   # nginx is the only public listener
}

local_workspace {
  base_path = "/var/lib/hermes/workspace"
}
```

Everything else — schema names, workspace subdirectories — is derived from the
hostname. Do not set `schema_name` or `workspace_path` unless you are adopting
storage that already exists.

`base_url` must be `https://` for each site. It is what tells Hermes to mark
session cookies `Secure`: behind nginx the connection Hermes sees is plain
loopback HTTP, so the connection cannot be used to work that out.

### Secrets

The HCL loader has no functions — no `env()`, no interpolation — so secrets
come from environment variables that override the parsed config. Put them in a
file only root can read:

```bash
# /etc/hermes/secrets.env
HERMES_SESSION_KEY=<openssl rand -base64 32>
HERMES_SERVER_POSTGRES_PASSWORD=<the password from step 1>
```

`HERMES_SESSION_KEY` is the one you cannot skip. It signs session cookies. Keep
it stable — it is what lets sessions survive a restart — and identical across
every instance serving these sites. If it is unset, or still holds the
`change-me` placeholder from the example, Hermes generates a random key and
warns: safe, but everyone is logged out on every restart.

### Identity provider

Each site sends users back to its own callback, derived from that site's
`base_url`. Register all of them on the OIDC client. For Dex:

```yaml
staticClients:
  - id: hermes
    redirectURIs:
      - https://docs.jrepp.com/auth/callback
      - https://notes.jrepp.com/auth/callback
```

OIDC compares `redirect_uri` exactly, so a missing entry surfaces as an
"invalid redirect URI" error from the provider, not from Hermes.

## 4. Migrate

```bash
/opt/hermes/bin/hermes-migrate -config=/etc/hermes/config.hcl
```

`-config` migrates every site the file declares, each into its own schema of its
own database, and prints the mapping. It reads the file through the same
registry and runs the same migration path the server does, so a config the
server would refuse to start on is refused here too rather than half-applied,
and the result is exactly what the server expects: extensions installed, tenant
columns stamped, and all.

Connections come from the config in this mode, so `-dsn` is not needed. Site
passwords are read from the environment as described above.

To migrate one site — after adding a single site, say — use `-schema` instead:

```bash
hermes-migrate -dsn="..." -schema=site_docs_jrepp_com
```

The server also migrates on startup, so this step is optional. Run it
separately when you want migration failures to be visible before a restart,
which is the safer habit.

## 5. Run under systemd

A ready-to-copy unit is in
[`scripts/deployment/systemd/hermes.service`](../../../scripts/deployment/systemd/hermes.service):

```ini
# /etc/systemd/system/hermes.service
[Unit]
Description=Hermes
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=hermes
Group=hermes
WorkingDirectory=/opt/hermes
EnvironmentFile=/etc/hermes/secrets.env
ExecStart=/opt/hermes/bin/hermes server -config=/etc/hermes/config.hcl
Restart=on-failure
RestartSec=5s

# The process needs no privileges beyond its own data.
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/hermes
StateDirectory=hermes

[Install]
WantedBy=multi-user.target
```

`ProtectSystem=strict` makes the filesystem read-only apart from
`ReadWritePaths`, so the workspace path in your config must be listed there.

```bash
sudo useradd --system --home /var/lib/hermes --shell /usr/sbin/nologin hermes
sudo install -d -o hermes -g hermes /var/lib/hermes/workspace
sudo chmod 600 /etc/hermes/secrets.env
sudo systemctl daemon-reload
sudo systemctl enable --now hermes
journalctl -u hermes -f
```

A healthy start logs one `serving site` line per site, with its schema and
workspace path. Read them: they are the derivation, and a surprise here is
easier to fix now than after the first document is written.

## 6. nginx and TLS

One server block per site keeps the certificates and logs separate. A template
is in
[`scripts/deployment/systemd/nginx-site.conf.example`](../../../scripts/deployment/systemd/nginx-site.conf.example):

```nginx
server {
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name docs.jrepp.com www.jrepp.com;

    ssl_certificate     /etc/letsencrypt/live/docs.jrepp.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/docs.jrepp.com/privkey.pem;

    client_max_body_size 64m;

    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 120s;
    }
}

server {
    listen 80;
    listen [::]:80;
    server_name docs.jrepp.com www.jrepp.com;
    return 301 https://$host$request_uri;
}
```

`proxy_set_header Host $host;` is the line that matters. It is what Hermes
routes on. Without it nginx sends the upstream address and every request
resolves to the wrong site — or to none.

`X-Forwarded-Host` is honoured **only** from an address in `trusted_proxies`.
From anywhere else it is ignored, because a client that could set it would get
to choose which tenant to read.

Certificates:

```bash
sudo certbot --nginx -d docs.jrepp.com -d www.jrepp.com
sudo certbot --nginx -d notes.jrepp.com
```

Include every alias in the same certificate as its canonical hostname.

## 7. Verify

```bash
# Each site answers over TLS.
curl -sS -o /dev/null -w '%{http_code}\n' https://docs.jrepp.com/
curl -sS -o /dev/null -w '%{http_code}\n' https://notes.jrepp.com/

# An unconfigured hostname is refused, not served by whichever site sorts first.
curl -sS -o /dev/null -w '%{http_code}\n' -H 'Host: nope.jrepp.com' http://127.0.0.1:8000/
# 421

# Schemas exist, one per site.
psql -U hermes -d hermes -c '\dn'
```

`/health` answers on the loopback listener without a Host header, because a
liveness probe addresses the process rather than a tenant:

```bash
curl -sS http://127.0.0.1:8000/health
```

Use that for systemd, a monitor, or an uptime check. It is the one path that
bypasses hostname routing.

Then log in to each site in a browser and confirm you land back on the site you
started from.

## Deploying an update

Once the host is set up, updates go through
[`scripts/deployment/deploy-remote.sh`](../../../scripts/deployment/deploy-remote.sh):

```bash
scripts/deployment/deploy-remote.sh deploy@jrepp.com --dry-run   # print the plan
scripts/deployment/deploy-remote.sh deploy@jrepp.com             # do it
```

It builds the frontend, builds static Linux binaries, uploads them beside the
running ones, migrates every site, moves the new binaries into place, restarts,
and waits for `/health`. The binaries are swapped only after the migration
succeeds, so a failed migration leaves the previous build running.

It deliberately does not install packages, create databases, write nginx
configs, or issue certificates. Those are one-time steps, and they belong here
where they can be read before they are run.

The migration DSN is read from `/etc/hermes/secrets.env` on the host, as
`HERMES_MIGRATE_DSN`, so the database password never passes through a local
shell history. `HERMES_REMOTE_DSN` overrides that if you would rather supply it
from your side.

## Adding a site

1. Point DNS at the server.
2. Add a `site` block to the config.
3. `hermes-migrate -dsn=... -config=/etc/hermes/config.hcl` — existing sites are
   already at their current version and are left alone; the new one migrates
   from zero.
4. Add an nginx server block and a certificate.
5. Register the new callback URL on the identity provider.
6. `sudo systemctl restart hermes`.

Sites are configuration. There is no runtime provisioning, and adding one
requires a restart.

## Removing a site

Set `disabled = true` on its `site` block and restart. That stops serving it
while leaving the schema and workspace directory in place, which is what you
want until you are certain. Deleting data is a separate, deliberate step:

```sql
DROP SCHEMA site_notes_jrepp_com CASCADE;
```

## Routine operations

**Force everyone to log out.** Change `HERMES_SESSION_KEY` and restart. Every
existing session stops verifying immediately.

**Back up.** The database and the workspace directory, together:

```bash
pg_dump -U hermes -Fc hermes > hermes-$(date +%F).dump
tar czf workspace-$(date +%F).tar.gz -C /var/lib/hermes workspace
```

Both hold every site. To restore one site, restore the dump elsewhere and copy
that site's schema across.

**Upgrade.** Build, replace the binaries, run `hermes-migrate` with `-config`,
restart. Migrations are applied per site, so a failure names the site it
happened on.

## Known limitations

- **Adding a site requires a restart.** Sites are structural configuration, not
  runtime state.
- **Semantic and hybrid search do not work at all.** `/api/v2/search/semantic`
  and `/api/v2/search/hybrid` always return 503: nothing in the server
  constructs the embeddings backend they need (RFC-088 is unfinished). This is
  not a multi-site limitation — it is the same on a single-site deployment.
- **Project configuration is process-level.** It is loaded once and, in
  practice, read by nothing: `Server.ProjectConfig` is populated at startup and
  no handler consults it.
- **Site-less work runs against the primary site.** The instance heartbeat, the
  health probe, and the search outbox statistics use the site named by
  `default_site`, or the first one declared. The startup log says which.
- **The indexer is per site.** Events publish to `<topic>.<site-slug>`, and
  `HERMES_INDEXER_TOKEN_PATH` yields one registration token per site, written
  to `<path>.<site-slug>`. A consumer configured for the base topic or the bare
  token path sees nothing; point each indexer at its site's pair.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `421 Misdirected Request` | Hostname matches no `site` block | Add the hostname as a site or an alias; check nginx sends `Host $host` |
| Every request hits the wrong site | nginx not passing `Host` | Add `proxy_set_header Host $host;` |
| `400 Bad Request` on every request | Host header unparseable | Check for a stray port or scheme in nginx's `proxy_set_header Host` |
| Startup: `hostname ... is claimed by both site` | Two sites share a hostname or alias | Remove the duplicate; the server refuses to guess |
| Startup: `extension is installed in schema "x", not public` | Extension installed privately, likely by an older single-tenant setup | `ALTER EXTENSION vector SET SCHEMA public` |
| Startup: `installing the vector extension` | pgvector is not on the server, or the role cannot create extensions | Install `postgresql-NN-pgvector`, or create the three extensions as a superuser |
| Startup: `migration reported success but ... does not exist` | Site tables did not land in the site schema | Check the search_path in the logged DSN; do not set `schema_name` by hand unless adopting existing storage |
| Logged out on every restart | No `HERMES_SESSION_KEY`, or it is a placeholder | Set a real one in the environment file; look for the warning at startup |
| Login lands on the wrong site | The callback URL is not registered per site | Add each site's `/auth/callback` to the OIDC client |
| Provider says "invalid redirect URI" | Same | Same |
| Session works on one subdomain, not another | Working as intended | Sessions are per site; logging in again is the expected flow |
| `no database configured for site` | A request resolved to a site with no pool | Restart after adding the site; sites are read at startup |
| Startup: `multi-site hosting does not support the Algolia search provider` | `search = "algolia"` with sites configured | Switch to meilisearch or bleve, or run one process per site |
| Startup: `multi-site hosting supports only the local workspace provider` | `workspace = "google"` or `"sharepoint"` with sites configured | Same |
| Startup: `site "x" has no search provider` | Wiring drift; should not happen | File a bug — the check exists to stop this reaching requests |
| Startup: `schema "x" already belongs to site "y"` | Two sites resolved to one schema, usually a `schema_name` override | Give one a different schema, or drop the disused schema |
| `violates check constraint "chk_..._domain"` | A row claiming a site other than the schema's owner — normally a dump restored into the wrong schema | Restore into the right schema; the constraint is what stopped a silent tenant merge |
| Searches return nothing on a new site | Documents predate the site, or the outbox has not drained | Check the per-site outbox relay in the logs |
| Health check returns 421 | Probing a path other than `/health` without a Host header | Only `/health` bypasses routing; probe that, or send `Host:` |

## References

- [RFC-023: Multi-Domain Hosting and Signed Sessions](../../rfc/rfc-023-multi-domain-hosting-and-signed-sessions.md)
- [ADR-012: Multi-Provider Auth Architecture](../../adr/adr-012-multi-provider-auth-architecture.md)
- [ADR-007: Local File Workspace System](../../adr/adr-007-local-file-workspace-system.md)
- Configuration reference: [`local/readme.md`](../../../local/readme.md), [`local/config.example.hcl`](../../../local/config.example.hcl)
