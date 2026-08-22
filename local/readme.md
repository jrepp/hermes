# `local/` — deployment-specific configuration

Everything in this directory is gitignored except this file,
`config.example.hcl`, and `sites/*.example.hcl`. It is where real hostnames,
credentials, and workspace data live so they never reach the repository.

```
local/
  config.hcl              # your config          (ignored)
  secrets.env             # exported to the process (ignored)
  workspace/              # per-site document storage (ignored)
    docs.jrepp.com/
    notes.jrepp.com/
  readme.md               # committed
  config.example.hcl      # committed
  sites/*.example.hcl     # committed
```

## Getting started

```bash
cp local/config.example.hcl local/config.hcl
$EDITOR local/config.hcl
make build-binaries
./build/bin/hermes server -config=local/config.hcl
```

`config.example.hcl` is exercised by `TestExampleConfigIsUsable`, so it is
always loadable against the current schema.

## Secrets

The HCL loader decodes with a nil `hcl.EvalContext`, so there are no functions
— no `env()`, no interpolation. Secrets come from environment variables that
override the parsed config. Keep them in `local/secrets.env`:

```bash
export HERMES_SERVER_POSTGRES_PASSWORD=...
export HERMES_SESSION_KEY="$(openssl rand -base64 32)"
```

and source it before starting the server:

```bash
set -a; . local/secrets.env; set +a
./build/bin/hermes server -config=local/config.hcl
```

The full set of overrides lives in `internal/cmd/commands/server/server.go`;
the common ones are `HERMES_SERVER_ADDR`, `HERMES_BASE_URL`,
`HERMES_SERVER_POSTGRES_PASSWORD`, `HERMES_AUTH_PROVIDER`,
`HERMES_WORKSPACE_PROVIDER`, and `HERMES_SEARCH_PROVIDER`.

### Session signing key

`HERMES_SESSION_KEY` is the one secret you cannot skip. The session cookie is
an HMAC-signed token, and this is the key that signs it.

Generate 32 or more characters (`openssl rand -base64 32`) and keep the value
stable: it is what lets a session survive a restart, and every instance serving
the same sites must use the same key or a session issued by one will be
rejected by another.

If it is unset — or still holds a placeholder such as the `change-me` in
`config.example.hcl` — Hermes generates a random key at startup and warns. That
is safe but not what you want in a deployment: everyone is logged out on every
restart. It is deliberately not a hard failure, so a fresh checkout runs, and
deliberately not a fixed default, because a default would be a signing secret
published in this repository.

Rotating the key invalidates every live session, which is the intended way to
force a global logout.

## Sessions across subdomains

A session issued on `docs.jrepp.com` does not work on `notes.jrepp.com`, by
two independent mechanisms:

- The cookie is **host-only** — no `Domain` attribute — so the browser does not
  send it to a sibling subdomain at all.
- The token itself names the site it was issued for, and verification requires
  that name to match the site handling the request.

The second is not redundant. Cookies do not follow the same-origin policy: any
host under `jrepp.com`, including one Hermes does not serve, can set a cookie
scoped to `Domain=jrepp.com`, and the browser will then send it everywhere
under that domain. Without the embedded site name, such a cookie would be a
cross-tenant takeover.

Aliases are the deliberate exception. `www.jrepp.com` is an alias of
`docs.jrepp.com`, so requests to it resolve to the canonical site and its
sessions work — an alias is one tenant with two names, not two tenants.

## Serving several subdomains

One Hermes process, one listener, many hostnames. Each `site` block is an
isolated tenant:

```hcl
site "docs.jrepp.com" {
  aliases  = ["www.jrepp.com"]
  base_url = "https://docs.jrepp.com"
}

site "notes.jrepp.com" {
  base_url = "https://notes.jrepp.com"
}
```

From the hostname alone, Hermes derives:

| Resource        | Value for `docs.jrepp.com`          |
| --------------- | ----------------------------------- |
| PostgreSQL      | schema `site_docs_jrepp_com`        |
| Workspace       | `<base_path>/docs.jrepp.com/`       |
| Base URL        | `https://docs.jrepp.com`            |

Override `schema_name` or `workspace_path` only to adopt storage that already
exists.

### How hostnames are matched

The block label is canonicalized before matching: lowercased, port stripped,
trailing dot removed, and internationalized names converted to punycode. So
`DOCS.JREPP.COM:8000`, `docs.jrepp.com.`, and `docs.jrepp.com` are one
hostname. See `pkg/domain` for the exact rules.

An **alias** is a second name for one tenant, not a second tenant: it resolves
to the same schema and the same directory. Two sites may never claim the same
hostname — the server refuses to start rather than route requests to whichever
tenant wins a map iteration.

A hostname matching no site is rejected with `421 Misdirected Request`. Set
`default_site` to route unmatched hosts somewhere instead. With exactly one
site configured, that site is the default automatically.

## Behind nginx

nginx terminates TLS and forwards to Hermes on loopback. Pass the original
hostname through:

```nginx
server {
    listen 443 ssl;
    server_name docs.jrepp.com notes.jrepp.com;

    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

`proxy_set_header Host $host` is the important line: it makes the real hostname
arrive as the `Host` header, which is what Hermes routes on.

`X-Forwarded-Host` is honored **only** from addresses listed in
`trusted_proxies`. From anywhere else it is ignored, because a client that
could set it would get to choose which tenant to read.

Bind Hermes itself to loopback so nginx is the only public listener:

```hcl
server {
  addr = "127.0.0.1:8000"
}
```
