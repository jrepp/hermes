# Hermes

[![CI](https://github.com/hashicorp-forge/hermes/workflows/ci/badge.svg?branch=main)](https://github.com/hashicorp-forge/hermes/actions/workflows/ci.yml?query=branch%3Amain)

> **Note**: Hermes is not an official HashiCorp project. The repository contains software which is under active development and is in the alpha stage. Please read the [Project Status](#project-status) section for more information.

Hermes is an open source document management system created by HashiCorp to help scale the writing and document process. Read the release blog post [here](https://hashicorp.com/blog/introducing-hermes-an-open-source-document-management-system).

**Security**: If you think that you've found a security issue, please contact us via email at security@hashicorp.com instead of filing a GitHub issue.

## 🚀 Quick Start

Get Hermes running locally in **one command**:

\`\`\`bash
cd testing && docker compose up -d
\`\`\`

This starts a complete testing environment with:
- ✅ Backend (Go) on http://localhost:8001
- ✅ Frontend (Ember.js) on http://localhost:4201
- ✅ PostgreSQL database
- ✅ Meilisearch search engine
- ✅ Dex OIDC provider

**Login**: `test@hermes.local` / `password`

### Alternative: Native Development

For faster iteration when developing backend or frontend code:

\`\`\`bash
# 1. Copy example configuration
cp config-example.hcl config.hcl

# 2. Start infrastructure services only
cd testing && docker compose up -d postgres meilisearch dex && cd ..

# 3. Terminal 1: Backend
make bin
./hermes server -config=config.hcl

# 4. Terminal 2: Frontend
cd web && yarn install
yarn start:proxy  # Auto-detects backend on port 8000

# 5. Open http://localhost:4200
\`\`\`

**Next Steps**:
- 📖 [Testing Environment Guide](testing/readme.md) - Detailed setup and troubleshooting
- 🔧 [Configuration Guide](local/config.example.hcl) - Customize your setup
- 🧪 [Makefile Targets](docs-internal/guides/dev/makefile-targets.md) - Common development commands

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Users / Browsers                         │
└───────────────────────────┬─────────────────────────────────────┘
                            │ HTTPS
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Frontend (Ember.js)                          │
│  • TypeScript + Tailwind CSS + HashiCorp Design System         │
│  • Document editor, search UI, approval workflows              │
└───────────────────────────┬─────────────────────────────────────┘
                            │ API Calls
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Backend (Go Server)                          │
│  • REST API (v1 + v2)                                          │
│  • Authentication & Authorization                               │
│  • Document lifecycle management                               │
│  • Search proxy                                                 │
└─────┬──────────┬──────────┬────────────┬────────────────────────┘
      │          │          │            │
      ▼          ▼          ▼            ▼
┌──────────┐ ┌────────┐ ┌────────┐ ┌─────────────┐
│PostgreSQL│ │Workspace│ │ Search │ │    Auth     │
│          │ │Provider │ │Provider│ │  Provider   │
│  (GORM)  │ │         │ │        │ │             │
└──────────┘ └────────┘ └────────┘ └─────────────┘
      │          │          │            │
      │     ┌────┴────┐ ┌───┴────┐  ┌────┴─────┐
      │     │ Google  │ │Algolia │  │  Google  │
      │     │Workspace│ │   or   │  │  OAuth   │
      │     │   or    │ │Meili   │  │    or    │
      │     │  Local  │ │ search │  │ Okta/Dex │
      │     └─────────┘ └────────┘  └──────────┘
      │
      ▼
┌─────────────────┐
│   Indexer       │
│ (Background)    │
│ • Syncs docs    │
│ • Updates index │
│ • Updates headers│
└─────────────────┘
```

### Components

**Frontend**: Ember.js 6.7 TypeScript application with HDS components  
**Backend**: Go 1.25+ server with modular provider architecture  
**Database**: PostgreSQL 15+ (source of truth for all data)  
**Search**: Algolia (managed) or Meilisearch (self-hosted)  
**Workspace**: Google Workspace (production) or Local (development)  
**Auth**: Google OAuth, Okta OIDC, or Dex (local)

See [Architecture Documentation](docs-internal/) for details.

## 📋 Requirements

- **Go**: 1.25 or later
- **Node.js**: 20 or later
- **Yarn**: 4.10+ ([install with corepack](https://yarnpkg.com/getting-started/install))
- **Docker & Docker Compose**: For local services (PostgreSQL, Dex, Meilisearch)

## 🔌 Provider Configuration

Hermes uses a modular provider architecture. Configure providers in `config.hcl`:

```hcl
providers {
  auth      = "dex"          # or "google", "okta"
  workspace = "local"        # or "google"
  search    = "meilisearch"  # or "algolia"
}
```

### Provider Guides

**Authentication Providers**:
- 🔐 [Dex (Local)](docs-internal/guides/auth/dex.md) - Recommended for development
- 🔐 [Google OAuth](docs-internal/guides/auth/google-workspace.md) - Production with Workspace
- 🔐 [Okta](docs-internal/guides/auth/providers.md) - Enterprise SSO

**Workspace Providers** (document storage):
- 📁 [Local Workspace](docs-internal/guides/workspace/local.md) - Filesystem-based, for development
- 📁 [Google Workspace](docs-internal/guides/auth/google-workspace.md) - Google Docs integration

**Search Providers**:
- 🔍 [Meilisearch](docs-internal/guides/search/meilisearch.md) - Self-hosted, open-source
- 🔍 [Algolia](docs-internal/guides/search/algolia.md) - Managed, cloud-hosted

**Infrastructure**:
- 🗄️ [PostgreSQL](docs-internal/guides/storage/postgresql.md) - Primary database
- 🎫 [Jira Integration](docs-internal/guides/integrations/jira.md) - Optional project linking

## 🛠️ Development

### Build Commands

```bash
# Backend only (fast)
make bin

# Backend tests
make go/test

# Frontend (in web/)
cd web
yarn install
yarn test:types      # TypeScript checking
yarn lint:hbs        # Template linting
yarn build           # Production build

# Full build (backend + frontend)
make build
```

### Development Modes

**Option 1: Native Backend + Native Frontend** (fastest iteration)
```bash
# Terminal 1: Backend
make bin && ./hermes server -config=config.hcl

# Terminal 2: Frontend
cd web && yarn start:proxy
```

**Option 2: Docker Backend + Native Frontend** (stable backend, fast frontend)
```bash
# Start backend in Docker
cd testing && docker compose up -d

# Frontend in another terminal
cd web && yarn start:proxy:testing
```

**Option 3: Fully Containerized** (complete integration)
```bash
# Everything in containers
cd testing && docker compose up -d

# Access at http://localhost:4201
```

See [Makefile Targets Guide](docs-internal/guides/dev/makefile-targets.md) for all available commands.

### Configuration

The `config-example.hcl` file contains comprehensive documentation for all options:

```bash
# Copy and customize
cp config-example.hcl config.hcl
nano config.hcl

# Start with your config
./hermes server -config=config.hcl
```

See [Configuration Documentation](local/config.example.hcl) for details.

## 🧪 Testing

### End-to-End Tests

```bash
# Start services first
cd testing && docker compose up -d

# Run Playwright tests
cd tests/e2e-playwright
npx playwright test --reporter=line
```

See [Playwright Guide](docs-internal/guides/dev/playwright-agent.md) for comprehensive testing instructions.

### Unit Tests

```bash
# Backend tests (no DB required)
make go/test

# Frontend tests
cd web && yarn test:types
```

## 📚 Documentation

### Getting Started
- [Testing Environment](testing/readme.md) - Complete local setup
- [Configuration Guide](local/config.example.hcl) - All config options
- [Makefile Targets](docs-internal/guides/dev/makefile-targets.md) - Development workflows

### Provider Setup
- [Dex Authentication](docs-internal/guides/auth/dex.md) - Local auth for development
- [Google Workspace](docs-internal/guides/auth/google-workspace.md) - Production document storage
- [Local Workspace](docs-internal/guides/workspace/local.md) - Filesystem-based storage
- [Meilisearch](docs-internal/guides/search/meilisearch.md) - Self-hosted search
- [Algolia](docs-internal/guides/search/algolia.md) - Managed search
- [PostgreSQL](docs-internal/guides/storage/postgresql.md) - Database setup
- [Jira Integration](docs-internal/guides/integrations/jira.md) - Project management integration

### Architecture & Development
- [Auth Providers Overview](docs-internal/guides/auth/providers.md) - All authentication options
- [Architecture Diagrams](docs-internal/guides/auth/providers.md) - System design
- [ADRs](docs-internal/adr/adr-002-readme.md) - Architecture decisions
- [RFCs](docs-internal/rfc/rfc-002-readme.md) - Technical proposals
- [Agent Instructions](.github/copilot-instructions.md) - AI-assisted development

## 🚢 Production Deployment

### Typical Production Stack

```hcl
providers {
  auth      = "google"    # or "okta"
  workspace = "google"    # Google Workspace
  search    = "algolia"   # or self-hosted Meilisearch
}

# Use managed PostgreSQL (RDS, Cloud SQL, etc.)
# Configure SSL/TLS for all connections
# Use environment variables for secrets
# Enable structured logging
```

### Serving several subdomains

One process can serve many hostnames as isolated tenants — each with its own
PostgreSQL schema, workspace directory, and session scope:

```hcl
site "docs.example.com" {
  aliases  = ["www.example.com"]
  base_url = "https://docs.example.com"
}

site "notes.example.com" {
  base_url = "https://notes.example.com"
}
```

Full procedure — PostgreSQL extensions, per-site migrations, systemd, nginx,
TLS, adding and removing sites —
[Multi-Domain Deployment](docs-internal/guides/deploy/multi-domain.md).

### Production Checklist

- [ ] Configure authentication provider (Google/Okta/Dex)
- [ ] Register every site's `/auth/callback` on the OIDC client
- [ ] Configure search provider (Algolia or Meilisearch)
- [ ] Deploy managed PostgreSQL with backups, and the `vector`, `citext`, and `uuid-ossp` extensions
- [ ] Set `HERMES_SESSION_KEY` to a stable 32+ character secret — without it, every restart logs everyone out
- [ ] Set `log_format = "json"` in config.hcl
- [ ] Use environment variables for secrets (see [env vars](docs-internal/guides/dev/env-vars.md))
- [ ] Configure `base_url` to your public URL, with the `https://` scheme — it is what marks session cookies `Secure` behind a TLS-terminating proxy
- [ ] Set up SSL/TLS certificates
- [ ] Enable monitoring and alerting against `/health`
- [ ] Run indexer as background service

## 📊 Project Status

This project is under active development and in the **alpha stage**. There may be breaking changes to:
- API endpoints
- Configuration file format
- Database schema
- Provider interfaces

We recommend:
- ✅ Using for internal tools and testing
- ✅ Following releases for updates
- ❌ Avoid production use until beta/stable
- ❌ Don't install builds from `main` branch

See [GitHub Releases](https://github.com/hashicorp-forge/hermes/releases) for stable versions.

## 🤝 Contributing

Before submitting a PR, please [create a GitHub issue](https://github.com/hashicorp-forge/hermes/issues/new) to discuss your proposed changes. This ensures alignment with project direction and avoids conflicts with planned work.

**Note**: Response time may be up to one week as we continue active development.

## 💬 Feedback

- **Security issues**: Email security@hashicorp.com (do not file public issues)
- **Bugs & features**: [Open a GitHub issue](https://github.com/hashicorp-forge/hermes/issues/new)
- **Questions**: Check [documentation](docs-internal/) or open a discussion

## 📜 License

See [LICENSE](LICENSE) file for details.

---

**Maintained by**: HashiCorp Labs (Office of the CTO)  
**Status**: Alpha - Active Development  
**Website**: https://hashicorp.com/blog/introducing-hermes-an-open-source-document-management-system
