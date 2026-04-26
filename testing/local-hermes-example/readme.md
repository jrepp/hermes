# Local Hermes Example - Vending Hermes into a Workspace

This example demonstrates how to "vend" Hermes into a local workspace directory for document management.

## Overview

**Local Mode** allows developers to:
- Run Hermes locally on their machine
- Manage documents in a specific workspace directory (e.g., `~/projects/my-project/.hermes`)
- Sync documents to a central Hermes instance (optional)
- Work offline with full document management capabilities

## Directory Structure

```
~/projects/my-project/
├── .hermes/                          # Local Hermes workspace
│   ├── config.hcl                    # Local Hermes configuration
│   ├── workspace_data/               # Local document storage
│   │   ├── docs/                     # Published documents
│   │   ├── drafts/                   # Draft documents
│   │   └── templates/                # Document templates
│   └── data/                         # Local database (SQLite)
│       └── hermes.db
├── src/                              # Your project source code
└── README.md
```

## Quick Start

### 1. Initialize Local Hermes

```bash
cd ~/projects/my-project

# Create .hermes directory
mkdir -p .hermes/workspace_data/{docs,drafts,templates}

# Copy example config
cp /path/to/hermes/testing/local-hermes-example/config.hcl .hermes/config.hcl

# Edit config.hcl to customize settings
vim .hermes/config.hcl
```

### 2. Start Local Hermes

```bash
# Using the hermes binary
hermes server -config=.hermes/config.hcl

# Or using Docker
docker run -v $(pwd)/.hermes:/app/.hermes -p 8000:8000 hermes:latest server -config=/app/.hermes/config.hcl
```

### 3. Access Local Hermes

Open http://localhost:8000 in your browser.

## Configuration

See `config.hcl` for a fully annotated example configuration.

### Key Settings

- **Database**: Uses SQLite for local storage (no PostgreSQL required)
- **Search**: Local Meilisearch or in-memory search
- **Workspace**: Points to `.hermes/workspace_data`
- **Sync**: Optional - configure central Hermes URL for syncing

## Syncing to Central Hermes

If you want to sync local documents to a central Hermes instance:

1. Configure the indexer in `config.hcl`:
   ```hcl
   indexer {
     enabled      = true
     central_url  = "https://hermes.company.com"
     workspace_path = ".hermes/workspace_data"
   }
   ```

2. Start the indexer agent:
   ```bash
   hermes indexer-agent -config=.hermes/config.hcl
   ```

3. Documents will sync to central Hermes every 5 minutes

## Use Cases

### Solo Developer
- Document architecture decisions
- Track RFCs and design docs
- Manage project documentation
- No central server needed

### Team with Central Hermes
- Work on docs offline
- Sync to central server when online
- Central server aggregates all team docs
- Full-text search across all projects

### Multi-Project Workspace
- Each project has its own `.hermes` directory
- Run multiple local Hermes instances on different ports
- Each syncs to central Hermes with unique project ID

## Files in This Example

- `config.hcl` - Local Hermes configuration
- `projects.hcl` - Project definitions
- `users.json` - Local users (dev mode)
- `README.md` - This file

## Next Steps

1. Customize `config.hcl` for your workspace
2. Add document templates to `workspace_data/templates/`
3. Start creating documents!
4. (Optional) Configure sync to central Hermes

## Troubleshooting

### Port Already in Use
Change the port in `config.hcl`:
```hcl
server {
  addr = "127.0.0.1:8001"  # Use different port
}
```

### Database Locked
SQLite can only have one writer. Make sure only one Hermes instance is running per `.hermes` directory.

### Sync Not Working
Check the indexer logs:
```bash
hermes indexer-agent -config=.hermes/config.hcl -log-level=debug
```
