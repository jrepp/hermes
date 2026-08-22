---
id: rfc-023
title: "Multi-Domain Hosting and Signed Sessions"
status: In Progress
created: 2026-08-21
author: Hermes Team
project_id: hermes
doc_uuid: 86b78af5-e125-46c6-b89a-773cd0f47bc7
type: RFC
subtype: Architecture
tags: [multi-tenancy, sessions, auth, domains, deployment]
related:
  - ADR-012
  - ADR-014
  - ADR-007
  - ADR-009
---

# RFC-023: Multi-Domain Hosting and Signed Sessions

One Hermes process should be able to serve several subdomains of one apex domain — `docs.jrepp.com`, `notes.jrepp.com` — as isolated tenants, behind a single nginx front end. This RFC introduces a canonical domain type, a hostname-to-tenant registry, per-domain storage scoping, and the signed session cookie that multi-tenancy makes mandatory. The binding rules on OIDC session handling live in [ADR-012](../adr/adr-012-multi-provider-auth-architecture.md) and [ADR-014](../adr/adr-014-dex-authentication-implementation.md); this RFC does not revisit them, it makes the implementation match them.

## Motivation

Hermes today is single-tenant by construction. `base_url` is global, the workspace adapter writes to one set of directories, and the database has one schema. Serving a second hostname means running a second process with a second config, second database, and second workspace — which is defensible at organizational scale and absurd for a handful of personal subdomains on one small host.

Two things forced this work rather than merely motivating it.

**The session cookie was not a session.** `internal/auth/auth.go` read the `hermes_session` cookie and returned its value as the authenticated user's email address. `internal/api/auth.go` set that cookie to the plaintext email after a successful Dex exchange. There was no signature, no MAC, and no server-side session store. Anyone could send:

```
Cookie: hermes_session=someone@example.com
```

and be that person. On `localhost` behind no network this is invisible. Exposed to the internet it is a complete authentication bypass with no exploit required beyond a browser devtools console. ADR-012 and ADR-014 both describe the OIDC path as issuing a session cookie; the implementation shipped the identity itself in the cookie instead. This RFC treats that as a defect against those ADRs, not a decision to revisit.

**Hostnames were not canonical.** Nothing in the codebase agreed on what a hostname *is*. `Host` headers arrive with ports, with trailing dots, in mixed case, and in Unicode. Any storage keyed on the raw string will split one tenant across several namespaces — or, worse, merge two.

## Goals

- One listener serves many hostnames, each an isolated tenant with its own storage namespace.
- A canonical domain type that cannot be constructed incorrectly, threaded from the HTTP edge to the storage engine.
- A session cookie that is unforgeable and cannot be replayed against another tenant.
- A gitignored `local/` configuration area suitable for a real deployment, with a committed, tested example.
- Existing single-site deployments continue to work with no configuration change.

## Non-Goals

- TLS termination in Hermes. nginx terminates TLS and proxies to a loopback listener; Hermes speaks plain HTTP.
- Cross-tenant features — shared search, shared users, or a tenant-spanning admin view.
- Tenant provisioning at runtime. Sites are configuration; adding one is a config change and a restart.
- Replacing the identity provider. Dex, Okta, Google, and SharePoint keep their existing roles.

## Proposal

### Overview

A request's hostname is resolved to a `Site` before anything else touches it. The `Site` carries every per-tenant resource derived from that hostname: PostgreSQL schema, workspace directory, base URL, and session scope. Downstream code reads the domain from the request context rather than from global configuration.

```text
                    ┌──────────────────────────────────────────────┐
   HTTPS            │ nginx                                        │
  ───────────────►  │  TLS + certbot, proxy_set_header Host $host  │
                    └───────────────────┬──────────────────────────┘
                                        │ plain HTTP, loopback
                                        ▼
             ┌───────────────────────────────────────────────────┐
             │ middleware.DomainResolver                         │
             │  Host ─► domain.Parse ─► sites.Registry.Lookup    │
             │  unknown host ─► 421   unparseable ─► 400         │
             └───────────────────┬───────────────────────────────┘
                                 │ ctx: domain.Name + *sites.Site
                                 ▼
             ┌───────────────────────────────────────────────────┐
             │ auth.AuthenticateRequest                          │
             │  verify HMAC, require token.Domain == ctx.Domain  │
             └───────────────────┬───────────────────────────────┘
                                 ▼
                        handlers, storage, search
```

### Detailed Design

#### `pkg/domain` — the canonical type

```go
// Name is a canonical DNS hostname identifying a Hermes site.
type Name struct {
    name string // unexported: cannot be produced by conversion
}

func Parse(raw string) (Name, error)
func (n Name) SchemaName() string
func (n Name) PathSegment() string
```

The unexported field is the design. `domain.Name("Docs.JRepp.com:8000")` does not compile, so every value in the program came through `Parse` and is already canonical. There is exactly one place normalization happens, and no function taking a `Name` has to defend itself.

`Parse` trims space, strips a numeric port, strips a trailing root dot, rejects invalid UTF-8, applies `idna.Lookup.ToASCII`, lowercases, validates label and total lengths, and then re-canonicalizes to confirm a fixed point.

Two bugs found while building it are worth recording, because both are the kind that would have surfaced as data corruption rather than as an error:

- `Parse("https://docs.jrepp.com")` returned `https`. The port stripper split on the first colon and the remainder was a valid single-label hostname. Ports are now required to be numeric, and anything else is an error.
- `Parse("\xff")` returned `xn--zn7c`, which does not itself re-parse. `idna.Lookup` maps undecodable bytes to U+FFFD and punycodes the result. A canonicalization that is not idempotent means the same tenant can land in two namespaces depending on which layer normalized first. Found by `FuzzParse` in 0.02 seconds; fixed with a UTF-8 check plus an explicit fixed-point assertion; the crasher is committed as a seed corpus entry.

`SchemaName` encodes `.` as `_` and `-` as `__`. The doubling is not cosmetic: a plain substitution maps both `a.b.com` and `a-b.com` to `site_a_b_com` and silently merges two tenants into one schema.

#### `internal/sites` — the registry

```hcl
site "docs.jrepp.com" {
  aliases  = ["www.jrepp.com"]
  base_url = "https://docs.jrepp.com"
}
```

`NewRegistry` builds the hostname-to-`Site` map at startup and derives defaults: `base_url` from the hostname, `schema_name` from `SchemaName()`, `workspace_path` from `<local_workspace.base_path>/<hostname>`.

It **refuses to start** on ambiguity: a hostname claimed by two sites, a site aliasing itself, a `default_site` naming a site that does not exist, or a site with no resolvable workspace path. A hostname claimed twice would otherwise route to whichever tenant a map iteration happened to yield — a non-deterministic cross-tenant read that no test would reliably catch.

Aliases fold onto the canonical domain rather than becoming separate keys, so `www.jrepp.com` and `docs.jrepp.com` share one storage namespace instead of quietly opening two.

#### `internal/middleware` — host routing

`DomainResolver` prefers `r.Host` and consults `X-Forwarded-Host` only when the peer address falls inside a configured `trusted_proxies` CIDR — otherwise the header is attacker-controlled and believing it lets any client pick which tenant to read. It stores the **site's canonical** domain in the context, not the spelling the client used, which is what makes alias folding work end to end.

Unknown hostnames get `421 Misdirected Request`; unparseable ones get `400`. The resolver is only installed when at least one `site` block is configured, so single-site deployments are untouched.

#### `internal/session` — signed cookies

```
hs1.<base64url(payload)>.<base64url(HMAC-SHA256(key, "hs1." + payload))>
```

The payload carries email, site, issue time, expiry, and a 16-byte random nonce. Design points:

- **The MAC covers the exact received bytes**, and is checked before the payload is decoded. No attacker-controlled structure is ever interpreted. This also means the JSON encoding does not need to be canonical.
- **The version prefix is inside the MAC**, so a future format cannot be confused for this one.
- **The token names its site**, and `Verify` requires that name to match the request's site. This is *not* redundant with the host-only cookie, and the reason is the core of the threat model — see below.
- **The stored domain is re-parsed, not trusted.** A token minted by an older build must not resolve to something `Parse` would reject today.
- **All verification failures are indistinguishable to the client.** Expiry, bad signature, and wrong site all render as a generic 401; the distinction is logged, not returned.

The key is derived by hashing the configured secret rather than decoding it. Accepting base64 would make the encoding ambiguous — a 44-character secret is both valid base64 and 44 perfectly good bytes — and an operator who guessed wrong would get a silently weaker key.

#### Storage scoping

`local.Config` gained a `Domain` field. When set, every default path roots at `<base_path>/<domain>/`, and `Validate` rejects an explicitly configured path that escapes that root. Isolation is enforced by construction rather than by convention, because a stray absolute path in one site's block is exactly how one tenant's documents become readable by another.

`local_workspace`'s `docs_path`, `drafts_path`, `folders_path`, `users_path`, and `tokens_path` became optional. A multi-domain deployment must leave them unset: one shared `docs_path` would put every tenant's documents in the same directory.

#### What per-schema migration exposed

Migrating a schema per site surfaced four defects, three of them pre-existing
and latent under the per-test-schema pattern the API tests already used:

- **Extensions are database-scoped objects that live in one schema.** The
  migrations create `vector`, `citext`, and `uuid-ossp` unqualified, so under
  per-site migration they land in whichever site migrated first. Every later
  site finds them already present, skips creation, and fails on the type. They
  are now installed in `public` up front — which is the reason `public` stays
  on each site's `search_path`.
- **Migration 000004** tested a column type with a scalar subquery over
  `information_schema` with no `table_schema` filter. With more than one schema
  holding `workspace_projects` that is an outright *"more than one row returned
  by a subquery"* error.
- **Migration 000009** looked up indexes in `pg_indexes` with no `schemaname`
  filter, so an index of the same name in another site's schema satisfied the
  check and the rename was skipped.
- **golang-migrate's `SchemaName` only decides where its version table lives.**
  It does not scope the migration SQL. The database-specific extras were worse:
  they ran straight on the pool with no schema at all. Both now run on one
  pinned connection with `search_path` set on it.

One question resolved along the way: `CREATE TABLE IF NOT EXISTS` resolves
against the *creation* schema, not against everything visible on the
`search_path`. So site tables do land correctly even when `public` already
holds a single-tenant install. That is verified rather than assumed, because
the cost of it being wrong is every site sharing one set of rows with no error
anywhere.

#### What booting the whole thing exposed

The unit and integration tests covered each piece; starting the real server
against the shipped example config found five more, three of which predate this
work:

- **`/api/v2/documents/` was registered twice**, by `DocumentHandler` and by
  `SimilarDocumentsHandler`. `http.ServeMux` panics on a duplicate pattern, so
  this was not a shadowed handler — it crashed the server at startup, in every
  configuration. `TestEndpointPatternsAreUnique` reads the patterns out of the
  source with `go/ast` and registers them, so the check cannot drift from the
  list it checks.
- **`/api/v2/web/config` panicked** on any config with no `feature_flags` block
  and again on any without an `algolia` block. `gin.New()` installs no recovery
  middleware, and this is the one endpoint the frontend must reach before it can
  do anything else.
- **`registerProducts` panicked** on a config with no `products` block.
- **Instance identity keyed on the global `base_url`**, so every site
  registered itself under the first site's hostname — two rows, one identity.
- **Startup document indexing** walked the process-wide workspace and wrote to
  the unprefixed indexes, so each site's search was built from the wrong files.

The last one generalises: constructing a provider has side effects (a directory
is created, indexes are created), so an unscoped provider in a multi-site
deployment leaves an empty workspace and a set of unprefixed indexes that look
like a tenant nobody serves. The process-wide providers are now built for the
primary site instead, matching how the fallback database pool is chosen.

### API / Schema Changes

No HTTP API surface changes. Configuration gains:

| Block / field | Meaning |
|---|---|
| `site "<hostname>" { … }` | One tenant. Repeatable. |
| `site.aliases` | Additional hostnames for the same tenant. |
| `site.base_url`, `site.schema_name`, `site.workspace_path` | Overrides for the derived defaults. |
| `site.database { … }` | Puts the site on a different server or database. Unset fields inherit from the global `postgres` block. |
| `postgres.sslmode` | libpq sslmode; defaults to `disable`, which suits loopback only. |
| `HERMES_SITE_<SLUG>_POSTGRES_PASSWORD` | Per-site database password. |
| `site.disabled` | Skip without deleting. |
| `default_site` | Where unmatched hostnames go; default is to reject with 421. |
| `trusted_proxies` | CIDRs (or bare IPs) whose `X-Forwarded-Host` is believed. |
| `session { key, ttl }` | Session signing secret and lifetime. |
| `HERMES_SESSION_KEY` | Environment override for `session.key`. |

Database: each site gets its own PostgreSQL schema, named `site_<encoded hostname>`. The schema layout within a site is unchanged.

#### Two levels of tenancy, not one

Schema separation is how isolation is *enforced*. It is not, by itself, how
isolation stays *checkable*, and the two are worth keeping apart.

A schema is only a namespace. Restore a dump into the wrong one, point
`schema_name` at a schema already in use, or recover a backup into the wrong
environment, and nothing in the data objects — the rows are equally at home
anywhere. So each site schema now also carries:

- a `site_identity` table naming its owner, which makes a bare dump traceable;
- a `domain` column on every top-level object table, with a per-schema
  `DEFAULT` and a `CHECK` constraint fixing it to that owner.

The column is deliberately invisible to the application. The models do not
declare it and GORM never names it, so it is populated entirely by the default
and there is no code path that can write the wrong value — which is the
difference between a stamp that means something and one that drifts. A `CHECK`
turns a mis-restore into an error at the moment it happens.

This is not the row-level tenancy rejected under Alternatives. That proposal
made the `WHERE` clause responsible for isolation, so a forgotten predicate was
a cross-tenant read. Here the schema still does the isolating; the column only
records what the schema already implies, and enforces that the record and the
schema agree.

Deployments with no `site` blocks get neither the column nor the identity
table.

#### Sites may live in different databases

`site.database` overrides the connection per site, inheriting anything it does
not set from the global `postgres` block, so the common case — schema
separation inside one database — still needs no configuration at all. Setting
`host` or `dbname` moves a site onto its own server or database, so one
tenant's data need not share a backup, a failover, or a blast radius with
another's.

Two consequences worth stating: every database a site uses needs the shared
extensions installed, not just the first; and `hermes-migrate -config` now
takes its connections from the config rather than a single `-dsn`, running the
same code path the server runs so the two cannot produce different schemas.

### Migration Plan

| Phase | Content | State |
|---|---|---|
| 0 | Guardrails: clean-checkout build, hermetic test suite, working lint, `make verify` | Done |
| 1 | `local/` configuration area with committed, tested example | Done |
| 2 | `pkg/domain` canonical type | Done |
| 3 | `internal/sites` registry and `internal/middleware` host routing | Done |
| 4a | Per-domain local workspace scoping | Done |
| 4b | Per-domain PostgreSQL schema; `srv.ForDomain(ctx)` | Done |
| 4c | Per-domain search namespace, workspace, and outbox relays | Done |
| 4d | Per-site database backends and row-level tenancy stamping | Done |
| 5 | Signed sessions | Done |
| 6 | Deployment artifacts: systemd unit, nginx vhost, deploy script | Done |
| 7 | Run it on jrepp.com | Not started |

Backward compatibility is total for phases 0–5: with no `site` blocks configured, the registry is empty, the resolver is not installed, and sessions are issued with no site scope. Existing deployments need no configuration change.

The one behavioural change for existing deployments is that session cookies issued before this change stop working, because they are not signed. Users log in again once. There is no compatibility window for the old format, deliberately: accepting an unsigned cookie is the vulnerability, so a grace period would be a grace period on the bypass.

Rollback is per-phase and independent. Removing `site` blocks disables multi-tenancy without touching sessions; reverting `internal/session` restores single-tenant behaviour without touching routing.

### Security / Privacy Considerations

The threat model that drives the domain binding in the token is worth stating explicitly, because the obvious objection — "the cookie is host-only, so it never reaches the other subdomain" — is wrong.

Cookies do not obey the same-origin policy. Any host under `jrepp.com` may set a cookie scoped to `Domain=jrepp.com`, and the browser will then send it to every sibling subdomain. That host does not have to be one Hermes serves; it does not have to be one the operator controls. Without the site name inside the signed payload, a token minted for `docs.jrepp.com` would authenticate its bearer on `notes.jrepp.com`. The host-only attribute and the embedded domain defend against different attacks, and both are needed.

Two further items:

- **`Secure` cannot be derived from the connection.** In the target topology nginx terminates TLS and proxies to loopback, so `r.TLS` is nil on every request even though the browser is on HTTPS. Deriving the flag from the connection would ship every production session cookie without `Secure`, exposing it to anyone who can force one plaintext request. It is derived from the site's configured `base_url` scheme instead, falling back to the connection only when no site is configured.
- **An unset signing key generates an ephemeral one and warns.** This fails closed — sessions end at restart — where a fixed default would be a signing secret published in this repository. Placeholder values such as the `change-me` in the shipped example are rejected for the same reason, and a test asserts it.

Not addressed here: `internal/auth/microsoft` sets a `user_email` cookie and its `extractTokenFromRequest` will accept any cookie longer than 100 characters as a bearer token. That path does validate the token against Microsoft Graph, so it is not the same trivial bypass, but it deserves its own review.

## Alternatives Considered

- **Separate process per subdomain** — the status quo generalized. Rejected: N processes, N configs, N database instances, and N TLS setups for what is one small deployment. It also does not make hostnames canonical, so the same normalization bugs remain, just distributed.
- **Row-level tenancy as the isolation mechanism (a `domain` column plus a `WHERE` clause on every query)** — one schema, one connection pool, filtering by tenant. Rejected in favour of schema-per-domain: every query becomes a place where a forgotten `WHERE domain = ?` is a silent cross-tenant read, and there is no way to make the compiler or the database enforce it. Schema separation makes isolation the default and leakage the thing that requires effort. Note that the `domain` column described above is *not* this: it records ownership and is enforced by a constraint, but nothing reads it to decide what a query returns.
- **Opaque session IDs backed by a server-side store** — a standard design with a real advantage: instant revocation. Rejected for now because it requires a session table or cache on the request path, and the deployment does not yet need per-session revocation. Rotating `HERMES_SESSION_KEY` provides global revocation. The nonce in the payload is the handle a future revocation list would key on.
- **JWTs via a library** — same shape, more surface. `alg=none` and algorithm-confusion bugs are the two most common JWT vulnerabilities, and both come from the format's flexibility about which algorithm to use. A fixed-version, fixed-algorithm token has neither.
- **Deriving `Secure` from `X-Forwarded-Proto`** — workable, but it makes cookie security depend on a header, and therefore on the trusted-proxy list being right. The site's own `base_url` is operator-configured, not request-derived, so it cannot be influenced by a client at all.

## Open Questions

- Algolia remains unsupported for multi-site: index count is a billing dimension, and more importantly the frontend queries it through a proxy handler that is not site-aware. Startup refuses the combination rather than serving one tenant's index to another. Google Workspace and SharePoint are refused for the same class of reason on the workspace side.
- Project configuration and indexer registration tokens remain process-level rather than per-site.
- The indexer now consumes per-site topics (`<topic>.<site-slug>`); whether it should instead carry the site in the event payload is open.
- Does the indexer need domain awareness, or is it sufficient for it to be pointed at one site at a time? It submits via API only (ADR-020), so the API's domain scope may be enough.

## References

- [ADR-012: Multi-Provider Auth Architecture](../adr/adr-012-multi-provider-auth-architecture.md) — binding rule on OIDC session cookies.
- [ADR-014: Dex Authentication Implementation](../adr/adr-014-dex-authentication-implementation.md) — binding rule on the Dex session path.
- [ADR-007: Local File Workspace System](../adr/adr-007-local-file-workspace-system.md) — the workspace layout being scoped per domain.
- [ADR-009: Provider Abstraction Architecture](../adr/adr-009-provider-abstraction-architecture.md) — provider model the session signer plugs into.
- Code: `pkg/domain/`, `internal/sites/`, `internal/middleware/domain.go`, `internal/session/`, `pkg/workspace/adapters/local/config.go`.
- Configuration: `local/config.example.hcl`, `local/readme.md`.
