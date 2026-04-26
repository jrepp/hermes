# AGENTS Instructions

## Project Grounding: Architectural Source of Truth

Hermes architecture is governed by ADRs in [`docs-internal/adr/`](docs-internal/adr/adr-002-readme.md). Before proposing or writing code that touches the areas below, **read the relevant ADR(s) and conform to them** unless the user explicitly asks to revisit a decision.

### Binding Cross-Cutting Principles

These are project-wide commitments. Cite the ADR when you invoke them; flag contradictions instead of silently working around them.

**Architecture & Provider Model**
- All external integrations go through provider interfaces: `auth.Provider`, `workspace.Provider`, `search.Provider`. Strategy + Adapter + Factory + DI. (ADR-009)
- Provider selection is configuration-driven via HCL `providers { auth, workspace, search }`; CLI flag `-auth-provider` and `HERMES_AUTH_PROVIDER` override. (ADR-009, ADR-013)
- V2 HTTP handlers use the single-parameter form `func Handler(srv *server.Server) http.Handler`; `server.Server` is the DI container. (ADR-017)
- Errors wrap typed sentinels and are matched with `errors.Is` (`search.ErrNotFound`, `workspace.ErrNotFound`). (ADR-009, ADR-017)

**Auth & Sessions**
- OIDC (Dex/Okta) is backend-centric (server-side token exchange, HttpOnly + Secure + SameSite=Lax session cookies); Google OAuth remains client-centric (popup, `Hermes-Google-Access-Token` header). (ADR-012, ADR-014, MEMO-062)
- Frontend selects the auth header at runtime from `/api/v2/web/config`; no Torii. (ADR-012, ADR-016)
- Local development uses Dex `v2.41.1` with static-password connector. (ADR-008)
- OAuth `redirect_uri` always points to the backend; the backend then redirects to the frontend via environment-agnostic `base_url`. (ADR-004)
- Never store auth tokens in `localStorage`. (ADR-012, ADR-014)

**Data, Search, Storage**
- Database is the source of truth; the search index is a cache. (ADR-007, ADR-017)
- All search calls go through backend `/1/indexes/*`; the frontend never holds Algolia credentials. (ADR-016)
- Local search uses Meilisearch `v1.11`; production uses Algolia; both behind `search.Provider`. (ADR-011)
- Workspace abstraction supports Google and a local Markdown+YAML-frontmatter filesystem (`pkg/workspace/local/`). (ADR-007, ADR-015)
- Document identity is the stable UUID in `pkg/docid` (`UUID` + `ProviderID` + project), surviving provider migrations. (ADR-018)

**Build, Binaries, Migrations**
- Server binary is pure-Go (`CGO_ENABLED=0`); migrations live in a separate binary `cmd/hermes-migrate` (PG + SQLite via `modernc.org/sqlite`). Do not pull SQLite drivers into `cmd/hermes`. (ADR-019, "Split Server and Migrate Binaries")
- Migrations follow the core+deltas layout: `00000N_<thing>_core.up.sql` plus small `_postgres` / `_sqlite` extras files. (ADR-020)
- The indexer is stateless and submits via API only; no direct DB access. (ADR-020)
- Configuration is HCL (not YAML/JSON); per-project files live under `testing/projects/*.hcl`; `_template-*` files are not loaded. (ADR-021)

**Frontend (Ember)**
- Build system: classic Ember CLI / Broccoli. Do not migrate to Embroider/Vite without revisiting ADR-001. (ADR-001)
- `locationType` is `history`. (MEMO-060)
- Every async API call is wrapped in `withTimeout()` with a graceful fallback; no infinite spinners. (ADR-005, "Frontend Async Timeout & Fallback Policy")
- Use `data-test-*` selectors for tests/automation. (ADR-010)
- `/me` and other singleton endpoints are fetched directly with `fetch()` and `credentials: "include"`, not via `store.findAll()`. (ADR-003, "Direct fetch() for Singleton API Endpoints")

**Testing & Local Iteration**
- Testing Docker Compose uses +1-offset ports: hermes 8001, postgres 5433, meilisearch 7701, dex 5557/5559, web 4201. (ADR-006)
- Integration tests use real PostgreSQL + Meilisearch via testcontainers, with per-test isolated schemas/indexes. (ADR-017)
- Playwright: `playwright-mcp` for interactive/agent exploration, headless Playwright for CI. **Never use `--headed` in automated contexts.** Use `--reporter=line --max-failures=1` for agent-friendly output. (ADR-010)

### When in Doubt
- Read [`docs-internal/adr/adr-002-readme.md`](docs-internal/adr/adr-002-readme.md) for the full index and the cross-cutting principles table.
- If a request appears to contradict an ADR, surface the conflict and ask before proceeding.
- Proposing a new architectural direction → write an RFC under `docs-internal/rfc/`. Recording an accepted decision → write an ADR under `docs-internal/adr/` and update the index.
- **Authoring any new ADR / RFC / memo, or demoting an ADR, starts from [`docs-internal/templates/readme.md`](docs-internal/templates/readme.md).** That guide pins the templates, frontmatter vocabularies, and the demotion / RFC-migration checklists.

## Documentation Source of Truth

- Public docs: `docs/`
- Internal notes and operational guides: `docs-internal/`
- Internal doc hierarchy:
  - `docs-internal/adr/` — architectural decisions (binding)
  - `docs-internal/rfc/` — proposals/architecture
  - `docs-internal/memo/` — operational/procedural notes
  - `docs-internal/plans/`
  - `docs-internal/archive/`

## Working Rules for Documentation Changes

- Read existing hub files first before adding or moving docs:
  - `docs/README.md`
  - `docs-internal/README.md`
  - `docs-internal/adr/adr-002-readme.md`
  - `docs-internal/memo/memo-020-readme.md`
  - `docs-internal/rfc/rfc-002-readme.md`
- Keep links valid after moves and update references in this order:
  1. `docs/`
  2. `docs-internal/`
  3. `docs-demo/`, `testing/`, `tests/`
  4. `internal/*`, `scripts/*`
- Prefer `docs-internal/memo/` for operational/procedural notes and `docs-internal/rfc/` for proposals/architecture.
- When an ADR is added, modified, or superseded, update `docs-internal/adr/adr-002-readme.md` (both the category table and, if applicable, the cross-cutting principles table).

## Validation Commands

- Run documentation validation with `uv`:

```bash
./scripts/docs-validate.sh            # default validation
./scripts/docs-validate.sh --quick    # quick validation
./scripts/docs-validate.sh fix        # auto-fix issues where supported
./scripts/docs-validate.sh migrate    # migrate docs metadata schema
./scripts/docs-validate.sh bootstrap  # run docuchango bootstrap workflow
```

- If `uv` is not installed, install it first:

```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

- Current behavior note:
  - Existing internal docs still use legacy uppercase filenames (`ADR-...`, `RFC-...`, `MEMO-...`).
  - `docuchango` expects new lowercase `adr-NNN-*` / `rfc-NNN-*` / `memo-NNN-*` naming.
  - Validation therefore reports filename-format violations until historical docs are normalized.

## If `docuchange` Is Not Installed

- `docuchange` is not required for repo tooling, but docs validation requires `docuchango`.
- Use the wrapper in `scripts/docs-validate.sh` so command differences stay consistent.
- If docuchango behavior differs locally, re-run with:

```bash
uv run docuchango validate --repo-root .
```

## Reporting

- For doc moves or deletions, add/update any impacted index files and related references in the same change.
- Prefer smaller, focused follow-up changes over broad refactors.
