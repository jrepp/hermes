# Project Workspaces

This directory contains workspace data for individual projects in the testing environment.

## Structure

```
workspaces/
├── testing/          # TEST project workspace (hermes-testing)
├── docs/             # DOCS project workspace (hermes-docs)
└── README.md         # This file
```

## Container Mapping

These directories are mounted into the Hermes container at `/app/workspaces/`:

- Host: `./testing/workspaces/testing` → Container: `/app/workspaces/testing`
- Host: `./testing/workspaces/docs` → Container: `/app/workspaces/docs`

## Project Configuration

Projects in `testing/projects/*.hcl` use relative paths:

```hcl
project "testing" {
  provider "local" {
    workspace_path = "testing"  # Relative to workspace_base_path
  }
}
```

The global `workspace_base_path` is set in `testing/projects.hcl`:

```hcl
projects {
  workspace_base_path = "/app/workspaces"  # Container path
}
```

## Native Development

For native development (non-containerized), you can override the workspace base path:

```bash
# Option 1: Environment variable
export HERMES_WORKSPACE_BASE_PATH="./testing/workspaces"

# Option 2: Update projects.hcl locally (gitignored)
cp testing/projects.hcl testing/projects.local.hcl
# Edit workspace_base_path to "./testing/workspaces"
```

## Adding Documents

Add test documents to these directories:

```bash
# Testing workspace
echo "# Test Document" > workspaces/testing/test-doc-001.md

# Docs workspace
echo "# Documentation" > workspaces/docs/getting-started.md
```

The Hermes indexer will discover and index these documents automatically.

## Persistence

In containerized mode:
- These directories are mounted as bind mounts (data persists on host)
- Changes made in container are immediately visible on host
- Git integration can track changes if configured

## See Also

- `testing/projects.hcl` - Global projects configuration
- `testing/projects/testing.hcl` - TEST project configuration
- `testing/projects/docs.hcl` - DOCS project configuration
- `testing/projects/README.md` - Projects configuration guide
