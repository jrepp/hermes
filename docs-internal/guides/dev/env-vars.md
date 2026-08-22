---
created: 2026-04-24
author: Hermes Team
project_id: hermes
doc_uuid: bd259e44-a5d8-4a7b-a7b1-b4df75161021
status: Reference
title: Environment Variables Setup Guide
tags: []
type: Guide
---

# Environment Variables Setup Guide

## Quick Start

1. **Copy the template**:

   ```bash
   cp .env.template .env

   ```

2. **Fill in required credentials** in `.env`:
   - Google OAuth 2.0 Client ID
   - Google Service Account credentials file path
   - Algolia App ID and API keys

3. **Never commit** `.env` files with real credentials (already in `.gitignore`)

## Required Credentials

### Google Workspace Setup

1. **Google Cloud Console** (https://console.cloud.google.com)
   - Create a project or use existing
   - Enable these APIs:
     - Google Drive API
     - Google Docs API
     - Gmail API
     - Google People API

2. **OAuth 2.0 Client ID** (for web frontend):
   - Go to: APIs & Services > Credentials
   - Create OAuth 2.0 Client ID (Web application)
   - Add authorized redirect URIs (e.g., `http://localhost:4200`)
   - Copy Client ID to `HERMES_WEB_GOOGLE_OAUTH2_CLIENT_ID`

3. **Service Account** (for backend):
   - Go to: IAM & Admin > Service Accounts
   - Create service account with appropriate permissions
   - Generate JSON key and save as `credentials.json` in repo root
   - Set `GOOGLE_APPLICATION_CREDENTIALS=./credentials.json`

### Algolia Setup

1. **Algolia Account** (https://www.algolia.com)
   - Create account and application
   - Get App ID from Dashboard
   - Get API keys from API Keys section:
     - Search API Key (public, for frontend)
     - Admin API Key (secret, for backend)

2. **Create indices** (or use existing):
   - `hermes_docs` (or `hermes_docs_dev` for development)
   - `hermes_drafts`
   - `hermes_internal`
   - `hermes_projects`

## Environment Variables Reference

Hermes is configured primarily through **HCL**, not through the environment.
The variables below are the complete set the code actually reads; anything not
listed here has no effect, whatever it is named after.

The HCL loader decodes with a nil `hcl.EvalContext`, so config files have no
functions — no `env()`, no interpolation. That is why secrets arrive as
environment overrides rather than as substitutions inside the file.

> Verified by `TestDocumentedEnvVarsAreReal` in `internal/config`, which fails
> if this list drifts from the source.

### Secrets

Set these in the environment, never in a committed config file.

| Variable | Read by | Purpose |
|---|---|---|
| `HERMES_SESSION_KEY` | `internal/session/key.go` | Signs session cookies. At least 32 characters. Unset or placeholder means an ephemeral key: safe, but everyone is logged out on restart. Must match across every instance serving the same sites. |
| `HERMES_SERVER_POSTGRES_PASSWORD` | `internal/cmd/commands/server/server.go` | Overrides `postgres.password`. |

### Server

Each of these also has a `hermes server` flag, shown in `hermes server -h`.
The flag wins over the variable, and both win over the config file.

| Variable | Flag | Purpose |
|---|---|---|
| `HERMES_SERVER_ADDR` | `-addr` | Listen address. Default `127.0.0.1:8000`. |
| `HERMES_BASE_URL` | `-base-url` | Fallback origin for generated links. Per-site links use that site's own `base_url`. |
| `HERMES_SERVER_PROFILE` | `-profile` | Config profile to load. |
| `HERMES_AUTH_PROVIDER` | `-auth-provider` | `dex`, `okta`, or `google`; disables the others. |
| `HERMES_SEARCH_PROVIDER` | `-search-provider` | `algolia`, `meilisearch`, or `bleve`. |
| `HERMES_WORKSPACE_PROVIDER` | `-workspace-provider` | `google`, `local`, or `sharepoint`. |
| `HERMES_SERVER_TLS_ENABLED` | `-tls-enabled` | Terminate TLS in Hermes. Leave off when nginx does it. |
| `HERMES_SERVER_TLS_CERT` | `-tls-cert` | Certificate path. |
| `HERMES_SERVER_TLS_KEY` | `-tls-key` | Private key path. |
| `HERMES_SERVER_OKTA_AUTH_SERVER_URL` | `-okta-auth-server-url` | Okta authorization server. |
| `HERMES_SERVER_OKTA_CLIENT_ID` | `-okta-client-id` | Okta client ID. |
| `HERMES_SERVER_OKTA_DISABLED` | — | Disables Okta. |
| `HERMES_SERVER_OKTA_JWT_SIGNER` | — | Okta JWT signer. |

There is deliberately no environment override for the site list. Sites are
structural configuration, and the server refuses to start on an ambiguous one
rather than resolving it silently — see
[Multi-Domain Deployment](../deploy/multi-domain.md).

### Indexer agent

| Variable | Purpose |
|---|---|
| `HERMES_CENTRAL_URL` | Hermes API the agent submits to. |
| `HERMES_API_TOKEN` | Bearer token for that API. |
| `HERMES_INDEXER_TYPE` | Indexer type to register as. |
| `HERMES_INDEXER_TOKEN_PATH` | Where the server writes a registration token at startup. |
| `HERMES_WORKSPACE_PATH` | Workspace root the agent scans. |
| `HERMES_PROJECTS_CONFIG` | Path to the projects config. |

### Embeddings and vector search

| Variable | Purpose |
|---|---|
| `HERMES_OLLAMA_URL` | Ollama endpoint for embeddings. |
| `HERMES_EMBEDDING_MODEL` | Embedding model name. |
| `HERMES_EMBEDDING_DIMENSIONS` | Embedding width; must match the model. |
| `HERMES_QDRANT_URL` | Qdrant endpoint. |
| `HERMES_QDRANT_API_KEY` | Qdrant API key. |
| `HERMES_QDRANT_COLLECTION` | Qdrant collection name. |

### Documents

| Variable | Purpose |
|---|---|
| `HERMES_DOCUMENT` | Document path used by the header-replacement tool. |

### Tests

None of these are read by the server; they select or skip integration tests.

| Variable | Effect |
|---|---|
| `HERMES_TEST_POSTGRESQL_DSN` | Use an existing PostgreSQL instead of a container. |
| `HERMES_TEST_MEILISEARCH_HOST` | Use an existing Meilisearch. |
| `HERMES_TEST_OLLAMA_URL` | Use an existing Ollama. |
| `HERMES_TEST_OLLAMA_EMBED_MODEL` | Embedding model for those tests. |
| `HERMES_TEST_QDRANT_URL` | Use an existing Qdrant. |
| `HERMES_TEST_QDRANT_API_KEY` | API key for it. |
| `HERMES_TEST_NTFY` | Opt in to posting to the public ntfy.sh service. Off by default so neither CI nor a local run sends traffic to a third party. |
| `HERMES_REPO_ROOT` | Repo root for tests that shell out. |
| `HERMES_NFR_BACKEND` | Backend under test in the NFR harness. |
| `HERMES_NFR_PROFILE` | NFR load profile. |

### Frontend build

Read by the Ember build and the Compose files, not by any Go code.

| Variable | Purpose |
|---|---|
| `HERMES_WEB_GOOGLE_OAUTH2_CLIENT_ID` | Google OAuth client ID for the frontend. |
| `HERMES_WEB_ALGOLIA_APP_ID` | Algolia application ID. |
| `HERMES_WEB_ALGOLIA_SEARCH_API_KEY` | Algolia search-only key. |

### Third-party

| Variable | Purpose |
|---|---|
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to the Google service-account JSON. Read by the Google SDK, not by Hermes. |

## Configured in HCL, not the environment

Dex, Algolia, Meilisearch, Jira, Datadog, and the local workspace are
configured **only** through the config file. Earlier revisions of this guide
listed 28 variables in the shape HERMES_DEX_ISSUER_URL, HERMES_MEILISEARCH_HOST,
HERMES_SERVER_POSTGRES_HOST, and HERMES_JIRA_API_TOKEN. None of them were ever
read by any code. They are removed rather than deprecated, because a variable
that silently does nothing is worse than no variable at all: you set it, the
setting is ignored, and nothing tells you why.

(Those names appear here without backticks on purpose. The test that checks
this guide treats a backticked name as a claim that the variable works.)

Use the config file for those settings:

```hcl
dex {
  issuer_url    = "https://auth.example.com/dex"
  client_id     = "hermes"
  client_secret = "..."
}

meilisearch {
  host    = "http://127.0.0.1:7700"
  api_key = "..."
}

postgres {
  host   = "localhost"
  port   = 5432
  dbname = "hermes"
  user   = "hermes"
  // password comes from HERMES_SERVER_POSTGRES_PASSWORD
}
```

See [`local/config.example.hcl`](../../../local/config.example.hcl) for a
complete, tested example.
