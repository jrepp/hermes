package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultRFCTemplate = `---
title: "RFC-XXX: Your RFC Title"
doc_type: "RFC"
status: "WIP"
contributors: []
approved_by: []
---

# RFC-XXX: Your RFC Title

## Context

Describe the problem or opportunity this RFC addresses.

## Proposal

Describe your proposed solution.

## Benefits

List the key benefits of this approach.

## Risks

Identify potential risks and mitigation strategies.

## References

- Link to related documents
- Link to external resources
`

	defaultPRDTemplate = `---
title: "PRD-XXX: Your Product Title"
doc_type: "PRD"
status: "WIP"
contributors: []
approved_by: []
---

# PRD-XXX: Your Product Title

## Problem Statement

What problem are we solving?

## Goals

What are we trying to achieve?

## Requirements

### Functional Requirements

### Non-Functional Requirements

## Success Metrics

How will we measure success?
`

	defaultFRDTemplate = `---
title: "FRD-XXX: Your Feature Title"
doc_type: "FRD"
status: "WIP"
contributors: []
approved_by: []
---

# FRD-XXX: Your Feature Title

## Overview

Describe the feature at a high level.

## User Stories

- As a [user type], I want [goal] so that [benefit]

## Design

Describe the design approach.

## Implementation Notes

Technical considerations for implementation.
`

	defaultConfigYAML = `# Hermes Simplified Mode Configuration
# This file is auto-generated. Modify only if needed.

server:
  addr: "127.0.0.1:8000"
  auto_open_browser: true

auth:
  mode: "single-user"  # Options: single-user, local-network

# Advanced options (optional)
# Uncomment and modify as needed:

# database:
#   type: "sqlite"  # default for simplified mode

# search:
#   provider: "bleve"  # default for simplified mode
`
)

// InitializeWorkspace creates the standard Hermes workspace directory structure
// and default files for simplified mode.
func InitializeWorkspace(basePath string) error {
	// Create directory structure
	dirs := []string{
		filepath.Join(basePath, "data"),
		filepath.Join(basePath, "documents"),
		filepath.Join(basePath, "drafts"),
		filepath.Join(basePath, "attachments"),
		filepath.Join(basePath, "templates"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create default templates
	templates := map[string]string{
		"rfc.md": defaultRFCTemplate,
		"prd.md": defaultPRDTemplate,
		"frd.md": defaultFRDTemplate,
	}

	for name, content := range templates {
		path := filepath.Join(basePath, "templates", name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create template %s: %w", name, err)
		}
	}

	// Write config.yaml
	configPath := filepath.Join(basePath, "config.yaml")
	if err := os.WriteFile(configPath, []byte(defaultConfigYAML), 0644); err != nil {
		return fmt.Errorf("failed to create config.yaml: %w", err)
	}

	// Create README
	readmePath := filepath.Join(basePath, "README.md")
	readmeContent := fmt.Sprintf(`# Hermes Workspace

This directory contains your Hermes document management system data.

## Structure

- **documents/** - Published documents
- **drafts/** - Draft documents
- **attachments/** - Binary attachments (images, PDFs)
- **templates/** - Document templates (RFC, PRD, FRD)
- **data/** - Database and search index (do not edit)
- **config.yaml** - Configuration overrides (optional)

## Getting Started

1. Start Hermes: %s
2. Open browser to http://localhost:8000
3. Create your first document using one of the templates

## Data Files

- %s - SQLite database
- %s - Full-text search index

## Backup

To backup your workspace, simply copy this entire directory.

For more information, visit: https://github.com/hashicorp/hermes
`, "`hermes serve`", "`data/hermes.db`", "`data/fts.index`")

	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}

	return nil
}
