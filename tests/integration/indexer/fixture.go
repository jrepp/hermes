//go:build integration
// +build integration

package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/pkg/workspace"
	"github.com/hashicorp-forge/hermes/pkg/workspace/adapters/local"
)

// LocalWorkspaceFixture provides a temporary local workspace for testing
type LocalWorkspaceFixture struct {
	t         *testing.T
	tempDir   string
	adapter   *local.Adapter
	provider  workspace.WorkspaceProvider
	documents map[string]*workspace.Document // document name -> document
}

// NewLocalWorkspaceFixture creates a new local workspace fixture
func NewLocalWorkspaceFixture(t *testing.T) *LocalWorkspaceFixture {
	t.Helper()

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "hermes-test-*")
	require.NoError(t, err, "failed to create temp directory")

	// Create local adapter configuration
	cfg := &local.Config{
		BasePath: tempDir,
	}

	// Create local adapter
	adapter, err := local.NewAdapter(cfg)
	require.NoError(t, err, "failed to create local adapter")

	// Create provider adapter
	provider := local.NewProviderAdapter(adapter)

	fixture := &LocalWorkspaceFixture{
		t:         t,
		tempDir:   tempDir,
		adapter:   adapter,
		provider:  provider,
		documents: make(map[string]*workspace.Document),
	}

	// Create sample documents
	fixture.createSampleDocuments()

	return fixture
}

// Cleanup removes temporary directory
func (f *LocalWorkspaceFixture) Cleanup() {
	if f.tempDir != "" {
		os.RemoveAll(f.tempDir)
	}
}

// GetAdapter returns the local adapter
func (f *LocalWorkspaceFixture) GetAdapter() *local.Adapter {
	return f.adapter
}

// GetProvider returns the provider interface
func (f *LocalWorkspaceFixture) GetProvider() workspace.WorkspaceProvider {
	return f.provider
}

// GetDocument returns a document by name
func (f *LocalWorkspaceFixture) GetDocument(name string) *workspace.Document {
	return f.documents[name]
}

// GetAllDocuments returns all documents
func (f *LocalWorkspaceFixture) GetAllDocuments() []*workspace.Document {
	docs := make([]*workspace.Document, 0, len(f.documents))
	for _, doc := range f.documents {
		docs = append(docs, doc)
	}
	return docs
}

// createSampleDocuments creates test documents in the workspace
func (f *LocalWorkspaceFixture) createSampleDocuments() {
	ctx := context.Background()
	storage := f.adapter.DocumentStorage()

	// RFC: Indexer Refactor
	rfc := f.createDocument(ctx, storage, "rfc-indexer-refactor", "RFC: Indexer Refactor with AI Enhancement", `
# RFC: Indexer Refactor with AI Enhancement

## Summary
This RFC proposes a comprehensive refactor of the Hermes indexer to support provider-agnostic document processing with AI capabilities.

## Goals
- **Provider Agnostic**: Support Google Workspace, local filesystem, and future providers
- **AI Integration**: Add document summarization and semantic search via embeddings
- **UUID Tracking**: Stable document identity across providers
- **Migration Support**: Track document revisions and detect conflicts

## Architecture
The new indexer uses a Command Pattern for composable operations:
1. Discovery - Find documents in provider
2. UUID Assignment - Assign stable identifiers
3. Hashing - Calculate content fingerprints
4. Summarization - Generate AI summaries
5. Embeddings - Create vector representations
6. Indexing - Store in search backend

## Implementation
Phase 1: Core abstractions (Command, Pipeline, Context)
Phase 2: Basic commands (Discover, Assign UUID, Hash)
Phase 3: AI commands (Summarize, Generate Embeddings)
Phase 4: Vector search integration

## Benefits
- Testable: Each command independently testable
- Composable: Build custom pipelines from commands
- Extensible: Easy to add new providers and operations
- Cost-effective: Use local Ollama instead of cloud APIs
`)
	f.documents["rfc-indexer-refactor"] = rfc

	// PRD: Semantic Search Feature
	prd := f.createDocument(ctx, storage, "prd-semantic-search", "PRD: Semantic Search Feature", `
# PRD: Semantic Search Feature

## Overview
This PRD describes the semantic search feature that enables users to find documents using natural language queries instead of keyword matching.

## User Stories
1. **As a user**, I want to search for "documents about API design" and find relevant RFCs even if they don't contain those exact words
2. **As a user**, I want to see why a document matched my query (highlight relevant sections)
3. **As a user**, I want search results ranked by relevance, not just keyword frequency

## Requirements
### Functional Requirements
- FR1: Support natural language queries
- FR2: Return semantically similar documents
- FR3: Highlight relevant sections in results
- FR4: Rank by semantic similarity score
- FR5: Support filtering by document type

### Non-Functional Requirements
- NFR1: Search latency < 500ms for 95th percentile
- NFR2: Support at least 10,000 documents
- NFR3: Accuracy: Top 5 results should include relevant docs 80% of time

## Technical Approach
- Use vector embeddings (768 dimensions)
- Meilisearch for vector search backend
- Ollama with nomic-embed-text for embeddings
- Chunk documents into 200-word segments
- Hybrid search: combine keyword + semantic

## Success Metrics
- User satisfaction: 4+ stars on feedback
- Reduced search time: 30% faster to find documents
- Increased engagement: 20% more documents discovered
`)
	f.documents["prd-semantic-search"] = prd

	// FRD: Document Migration System
	frd := f.createDocument(ctx, storage, "frd-migration-system", "FRD: Document Migration System", `
# FRD: Document Migration System

## Purpose
Define the functional requirements for migrating documents between workspace providers (Google Workspace ↔ Local Filesystem).

## Use Cases
### UC1: Migrate Document to Local
**Actor**: System Administrator
**Preconditions**: Document exists in Google Drive
**Flow**:
1. User selects document in Google Drive
2. System reads document content and metadata
3. System creates equivalent document in local filesystem
4. System tracks migration in database (source/target revisions)
5. System verifies content integrity (hash comparison)
**Postconditions**: Document exists in both providers, migration tracked

### UC2: Detect Migration Conflicts
**Actor**: Indexer System
**Preconditions**: Document migrated, then modified in both providers
**Flow**:
1. Indexer calculates content hash for Google version
2. Indexer calculates content hash for local version
3. Hashes differ → conflict detected
4. System marks document with conflict status
5. System notifies administrator
**Postconditions**: Conflict recorded, requires manual resolution

## Functional Requirements
### FR1: Content Migration
System shall copy document content byte-for-byte between providers

### FR2: Metadata Preservation
System shall preserve document metadata:
- Title
- Creation date
- Last modified date
- Owner information
- Custom metadata fields

### FR3: Revision Tracking
System shall track document revisions in database:
- UUID (stable identifier)
- Provider (google, local)
- Content hash (SHA-256)
- Revision timestamp
- Status (active, archived, source, target, conflict)

### FR4: Conflict Detection
System shall detect conflicts when:
- Same UUID exists in multiple providers with different hashes
- Document modified after migration in both providers
- Migration target already exists with different content

## Validation Rules
### VR1: UUID Uniqueness
Each document shall have exactly one UUID across all providers

### VR2: Content Integrity
Content hash shall match after migration (before any modifications)

### VR3: Metadata Completeness
Required metadata fields shall not be null after migration
`)
	f.documents["frd-migration-system"] = frd
}

// createDocument creates a document in the local workspace
func (f *LocalWorkspaceFixture) createDocument(
	ctx context.Context,
	storage workspace.DocumentStorage,
	id, title, content string,
) *workspace.Document {
	f.t.Helper()

	// Create document file
	docPath := filepath.Join(f.tempDir, id+".md")
	err := os.WriteFile(docPath, []byte(content), 0644)
	require.NoError(f.t, err, "failed to write document file")

	// Create document metadata
	modTime, err := time.Parse(time.RFC3339, "2025-10-22T12:00:00Z")
	if err != nil {
		// This should never fail with a valid constant, use current time as fallback
		modTime = time.Now()
	}
	doc := &workspace.Document{
		ID:           id,
		Name:         title,
		MimeType:     "text/markdown",
		ModifiedTime: modTime,
		Metadata:     make(map[string]interface{}),
	}

	// Store in adapter's document registry (if it has one)
	// For local adapter, documents are discovered via filesystem

	return doc
}

// CreateDocument creates a new document in the workspace
func (f *LocalWorkspaceFixture) CreateDocument(title, content string) *workspace.Document {
	f.t.Helper()

	ctx := context.Background()
	storage := f.adapter.DocumentStorage()

	// Generate ID from title
	id := fmt.Sprintf("test-doc-%d", len(f.documents))

	doc := f.createDocument(ctx, storage, id, title, content)
	f.documents[title] = doc

	return doc
}

// UpdateDocument updates a document's content
func (f *LocalWorkspaceFixture) UpdateDocument(doc *workspace.Document, newContent string) {
	f.t.Helper()

	// Update file
	docPath := filepath.Join(f.tempDir, doc.ID+".md")
	err := os.WriteFile(docPath, []byte(newContent), 0644)
	require.NoError(f.t, err, "failed to update document file")
}

// GetDocumentContent reads a document's content
func (f *LocalWorkspaceFixture) GetDocumentContent(doc *workspace.Document) string {
	f.t.Helper()

	docPath := filepath.Join(f.tempDir, doc.ID+".md")
	content, err := os.ReadFile(docPath)
	require.NoError(f.t, err, "failed to read document file")

	return string(content)
}

// ListDocuments discovers all documents in the workspace
func (f *LocalWorkspaceFixture) ListDocuments() []*workspace.Document {
	f.t.Helper()

	ctx := context.Background()
	storage := f.adapter.DocumentStorage()

	// List all documents in docs and drafts folders
	docs, err := storage.ListDocuments(ctx, "", nil)
	require.NoError(f.t, err, "failed to list documents")

	return docs
}
