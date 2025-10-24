# Indexer Refactor Plan: Command Pattern & Provider Architecture

**Version**: 2.0.0  
**Status**: 🚧 Design Phase - Updated with Document Revisions & Vector Search  
**Last Updated**: October 22, 2025  
**Related**: `DOCUMENT_REVISIONS_AND_MIGRATION.md`, `DISTRIBUTED_PROJECTS_ARCHITECTURE.md`

## Executive Summary

Refactor the Hermes indexer from a Google Workspace-specific service into a provider-agnostic document processing pipeline using the Command pattern and visitor pattern. This will enable:

1. **Multi-provider support**: Work with Google Workspace, Local filesystem, and future providers
2. **Document migration**: Move documents between providers with revision tracking (e.g., Google → Local for testing)
3. **Document revision tracking**: Track multiple versions of documents across providers during migration (see `DOCUMENT_REVISIONS_AND_MIGRATION.md`)
4. **Vector embeddings**: Generate and store embeddings using AWS Bedrock (Claude Sonnet 3.7) for semantic search
5. **Local integration testing**: Run full indexer validation in `./testing` infrastructure
6. **Composable operations**: Chain document processing commands (index, migrate, transform, validate, embed)

## Current Architecture (Problems)

### Hard Dependencies on Google Workspace

```go
// internal/indexer/indexer.go
type Indexer struct {
    GoogleWorkspaceService *gw.Service  // ❌ Hardcoded to Google
    AlgoliaClient *algolia.Client       // ❌ Hardcoded to Algolia
    DocumentsFolderID string             // ❌ Google Drive specific
    DraftsFolderID string                // ❌ Google Drive specific
}

func (idx *Indexer) Run() error {
    // ❌ Direct calls to Google Drive API
    docFiles, err := gwSvc.GetUpdatedDocsBetween(
        idx.DocumentsFolderID, lastIndexedAtStr, currentTimeStr)
    
    // ❌ Direct export from Google Docs
    exp, err := gwSvc.Drive.Files.Export(file.Id, "text/plain").Download()
}
```

### Monolithic Processing Loop

- Single 60-second loop that does everything
- No separation of concerns (discovery, transformation, indexing)
- No way to test individual operations
- No way to chain or compose operations

### Inability to Test Locally

- Requires live Google Workspace connection
- Can't validate indexer behavior in `./testing` environment
- No way to create reproducible test scenarios

## New Architecture: Command Pipeline with Revisions & Vector Search

### Core Abstraction Layers

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Indexer Orchestrator                              │
│  (Schedules runs, manages state, executes pipelines)                    │
│  • Document discovery across providers                                  │
│  • Revision tracking & conflict detection                               │
│  • Content hash calculation                                             │
│  • Migration coordination                                               │
└───────────────────────┬─────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           Document Pipeline                              │
│  (Chain of commands that process documents)                             │
│                                                                          │
│  [Discover] → [AssignUUID] → [ExtractContent] → [CalcHash] →           │
│  → [TrackRevision] → [Transform] → [SummarizeReview] →                │
│  → [GenerateEmbedding] → [IndexFullText] → [IndexVector] →            │
│  → [DetectConflicts] → [Notify]                                        │
└───────────────────────┬─────────────────────────────────────────────────┘
                        │
        ┌───────────────┼───────────────┬──────────────────┬──────────────────┐
        ▼               ▼               ▼                  ▼                  ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│   Workspace  │ │  Search      │ │  Indexer API │ │  Embeddings  │ │  Project     │
│   Provider   │ │  Provider    │ │  (HTTP/REST) │ │  Provider    │ │  Config      │
│              │ │              │ │              │ │              │ │              │
│ • Google     │ │ • Algolia    │ │ • Documents  │ │ • Bedrock    │ │ • HCL        │
│ • Local      │ │ • Meilisearch│ │ • Revisions  │ │   (Sonnet)   │ │ • Projects   │
│ • GitHub     │ │ • Vector DB  │ │ • Summaries  │ │ • Ollama     │ │ • Workspaces │
│ • Remote     │ │ • Mock       │ │ • Embeddings │ │ • Mock       │ │              │
│ • Mock       │ │              │ │ • Auth       │ │              │ │              │
└──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘
```

### Document Lifecycle with Revisions

```
1. Discovery Phase:
   ┌─────────────┐
   │  Workspace  │ ──(discover)──> [Document Found]
   │  Provider   │                        │
   └─────────────┘                        │
                                          ▼
2. UUID Assignment:                 [Check UUID]
   ┌──────────────────────────────────────┐
   │ Has UUID in metadata?                │
   │  YES: Use existing                   │
   │  NO:  Generate & write back          │
   └──────────────────────────────────────┘
                     │
                     ▼
3. Content & Hash:        [Extract Content] → [Calculate Hash]
                                    │
                                    ▼
4. Revision Tracking:         [Create/Update Revision]
   ┌──────────────────────────────────────┐
   │ document_revisions table:            │
   │  - document_uuid (stable)            │
   │  - project_id                        │
   │  - provider_type                     │
   │  - provider_document_id              │
   │  - content_hash                      │
   │  - status (active/source/target)     │
   └──────────────────────────────────────┘
                     │
                     ▼
5. Processing:    [Transform] → [Summarize & Review]
                     │              │
                     │              ▼
                     │         [AI Summary]
                     │         [Review Status]
                     │         [Key Points]
                     │              │
                     ▼              ▼
6. AI Enhancement: [Generate Embedding (Bedrock)]
                     │
                     ▼
7. Indexing:   [Full-Text]    [Vector DB]
                     │              │
                     ▼              ▼
8. Validation: [Detect Conflicts] → [Alert if needed]
```

## Design Patterns

### 1. Command Pattern

Each indexer operation becomes a command that can be executed, undone, and chained:

```go
// Command interface for document operations
type Command interface {
    Execute(ctx context.Context, doc *DocumentContext) error
    Name() string
}

// Example commands
type DiscoverDocumentsCommand struct {
    provider workspace.DocumentStorage
    folderID string
    since    time.Time
}

type ExtractContentCommand struct {
    provider workspace.DocumentStorage
    maxSize  int
}

type IndexDocumentCommand struct {
    searchProvider search.Provider
    indexType      IndexType // docs, drafts
}

type UpdateHeaderCommand struct {
    provider     workspace.DocumentStorage
    documentTypes []*config.DocumentType
}

type MigrateDocumentCommand struct {
    source workspace.DocumentStorage
    target workspace.DocumentStorage
}

type SummarizeReviewCommand struct {
    aiProvider    AI.Provider // Bedrock, OpenAI, etc.
    model         string      // "claude-3-7-sonnet", etc.
    extractTopics bool
    generateTags  bool
}

type GenerateEmbeddingCommand struct {
    aiProvider    AI.Provider
    model         string
    vectorDB      search.VectorIndex
    chunkSize     int
    chunkOverlap  int
}
```

### 2. Pipeline Pattern

Commands are composed into pipelines for different operations:

```go
type Pipeline struct {
    name     string
    commands []Command
    filter   DocumentFilter // Skip certain documents
}

// Standard pipelines
var (
    IndexPublishedPipeline = Pipeline{
        name: "index-published",
        commands: []Command{
            &DiscoverDocumentsCommand{},
            &AssignUUIDCommand{},
            &ExtractContentCommand{},
            &CalculateHashCommand{},
            &TrackRevisionCommand{},
            &TransformToSearchDocCommand{},
            &IndexDocumentCommand{},
            &UpdateTrackingCommand{},
        },
    }
    
    AIEnhancedIndexPipeline = Pipeline{
        name: "ai-enhanced-index",
        commands: []Command{
            &DiscoverDocumentsCommand{},
            &AssignUUIDCommand{},
            &ExtractContentCommand{},
            &CalculateHashCommand{},
            &TrackRevisionCommand{},
            &TransformToSearchDocCommand{},
            &SummarizeReviewCommand{},      // AI summarization
            &GenerateEmbeddingCommand{},     // Vector embeddings
            &IndexDocumentCommand{},         // Full-text search
            &IndexVectorCommand{},           // Vector search
            &UpdateTrackingCommand{},
        },
    }
    
    RefreshHeadersPipeline = Pipeline{
        name: "refresh-headers",
        commands: []Command{
            &DiscoverDocumentsCommand{},
            &LoadMetadataCommand{},
            &UpdateHeaderCommand{},
            &UpdateTrackingCommand{},
        },
        filter: RecentlyModifiedFilter(30 * time.Minute),
    }
    
    MigrationPipeline = Pipeline{
        name: "migrate",
        commands: []Command{
            &DiscoverDocumentsCommand{},
            &AssignUUIDCommand{},           // Ensure UUID exists
            &ExtractContentCommand{},
            &CalculateHashCommand{},        // For conflict detection
            &LoadMetadataCommand{},
            &MigrateDocumentCommand{},
            &TrackRevisionCommand{},        // Track in target
            &DetectConflictsCommand{},      // Check for conflicts
            &IndexDocumentCommand{},        // Index in target
            &UpdateTrackingCommand{},
        },
    }
    
    SemanticSearchBootstrapPipeline = Pipeline{
        name: "semantic-search-bootstrap",
        description: "Generate embeddings for existing documents",
        commands: []Command{
            &DiscoverDocumentsCommand{},
            &ExtractContentCommand{},
            &SummarizeReviewCommand{},      // Extract key points
            &GenerateEmbeddingCommand{},     // Generate embeddings
            &IndexVectorCommand{},           // Store in vector DB
            &UpdateTrackingCommand{},
        },
        filter: MissingEmbeddingFilter(),
    }
)
```

### 3. Document Visitor Pattern

Process documents without knowing their concrete provider:

```go
type DocumentVisitor interface {
    VisitDocument(ctx context.Context, doc *workspace.Document) error
}

type DocumentContext struct {
    // Source document from provider
    Document *workspace.Document
    
    // UUID and Revision Tracking
    DocumentUUID UUID                     // Stable identifier across providers
    ContentHash  string                   // SHA-256 hash for change detection
    Revision     *models.DocumentRevision // Current revision info
    
    // Hermes-specific metadata
    Metadata     *models.Document
    Reviews      models.DocumentReviews
    GroupReviews models.DocumentGroupReviews
    
    // Processing state
    Content     string
    Transformed *document.Document // For full-text search indexing
    
    // AI-generated content (external structures, optional)
    AISummary  *ai.DocumentSummary    // AI-generated summary and analysis
    Embeddings *ai.DocumentEmbeddings // Vector embeddings for semantic search
    VectorDoc  *search.VectorDocument // Prepared vector document for indexing
    
    // Provider information
    SourceProvider workspace.DocumentStorage
    TargetProvider workspace.DocumentStorage
    
    // Migration tracking
    MigrationStatus string // "none", "source", "target", "conflict", "canonical"
    ConflictInfo    *ConflictInfo
    
    // Tracking
    StartTime time.Time
    Errors    []error
}

// ConflictInfo tracks migration conflicts
type ConflictInfo struct {
    DetectedAt    time.Time
    ConflictType  string // "concurrent-edit", "content-divergence", etc.
    SourceHash    string
    TargetHash    string
    SourceModTime time.Time
    TargetModTime time.Time
    Resolution    string // "pending", "source-wins", "target-wins", "manual"
}
```

## New Package Structure

```
pkg/
  indexer/                    # New provider-agnostic indexer
    command.go                # Command interface
    pipeline.go               # Pipeline composition
    context.go                # DocumentContext with revisions & AI fields
    orchestrator.go           # Main orchestrator
    
    commands/                 # Individual commands
      discover.go             # Discover documents in a provider
      assign_uuid.go          # Assign/discover document UUIDs
      extract.go              # Extract content
      hash.go                 # Calculate content hash
      revision.go             # Track document revisions
      transform.go            # Transform to search format
      summarize.go            # AI summarization & review (NEW)
      embedding.go            # Generate vector embeddings (NEW)
      index.go                # Index in search provider (full-text)
      index_vector.go         # Index in vector database (NEW)
      header.go               # Update headers
      migrate.go              # Migrate between providers
      conflict.go             # Detect and resolve conflicts (NEW)
      tracking.go             # Update database tracking
      
    filters/                  # Document filters
      time.go                 # Time-based filtering
      type.go                 # Document type filtering
      status.go               # Status filtering
      embedding.go            # Filter by embedding status (NEW)
      
    config.go                 # Configuration structures
  
  ai/                         # AI provider abstraction (NEW)
    provider.go               # AI provider interface
    document_summary.go       # DocumentSummary struct (external)
    document_embeddings.go    # DocumentEmbeddings struct (external)
    bedrock/                  # AWS Bedrock implementation
      client.go               # Bedrock client
      summarize.go            # Summarization using Claude
      embedding.go            # Embeddings using Titan
    openai/                   # OpenAI implementation (optional)
      client.go
      summarize.go
      embedding.go
    mock/                     # Mock for testing
      provider.go
    
internal/
  indexer/                    # Legacy - will be deprecated
    [current files]
    
  cmd/
    commands/
      indexer/
        indexer.go            # Updated CLI using new pkg/indexer
        
testing/
  indexer/                    # Local testing infrastructure
    test-data/                # Sample documents
      docs/
        RFC-001.md
        PRD-002.md
      drafts/
        DRAFT-003.md
    fixtures.go               # Test data generation
    integration_test.go       # Full integration tests
```

## AI Provider & Vector Search Architecture

### Design Philosophy: External Data Structures

AI-generated content (`DocumentSummary`, `DocumentEmbeddings`) follows the same pattern as `document.Document`:

**✅ External Structures (Reusable, Cacheable)**:
- `ai.DocumentSummary` - AI-generated analysis
- `ai.DocumentEmbeddings` - Vector embeddings
- `search.VectorDocument` - Prepared for vector indexing
- `document.Document` - Transformed for full-text search

**✅ Benefits**:
1. **Optional Dependencies**: Commands can reference or ignore these structures
2. **Caching**: Store in database, reuse across pipelines
3. **Independent Lifecycle**: Generate once, use many times
4. **Type Safety**: Strong typing with clear ownership
5. **Testability**: Mock or provide real instances easily

**Example Usage**:
```go
// Pipeline 1: Generate summary only
doc.AISummary = &ai.DocumentSummary{...}
doc.Embeddings = nil  // Not needed

// Pipeline 2: Use existing summary, generate embeddings
doc.AISummary = loadFromDB(docID)  // Reuse cached
doc.Embeddings = generateNew()     // Fresh embeddings

// Pipeline 3: Full AI enhancement
doc.AISummary = generateNew()
doc.Embeddings = generateNew()
doc.VectorDoc = prepareForIndexing(doc.AISummary, doc.Embeddings)
```

### AI Provider Interface

```go
// pkg/ai/provider.go
package ai

import (
    "context"
)

// Provider defines the interface for AI operations
type Provider interface {
    // Summarize generates a summary and extracts key information
    // Returns DocumentSummary as an external, reusable structure
    Summarize(ctx context.Context, req *SummarizeRequest) (*SummarizeResponse, error)
    
    // GenerateEmbedding creates vector embeddings for text
    // Returns DocumentEmbeddings as an external, reusable structure
    GenerateEmbedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)
    
    // Name returns the provider name
    Name() string
}

// SummarizeRequest contains the document to summarize
type SummarizeRequest struct {
    Content      string
    Title        string
    DocType      string
    MaxSummaryLength int // Token limit for summary
    ExtractTopics    bool
    ExtractKeyPoints bool
    SuggestTags      bool
    AnalyzeStatus    bool // Suggest document status from content
}

// SummarizeResponse contains the AI-generated analysis
type SummarizeResponse struct {
    Summary     *DocumentSummary
    Model       string
    TokensUsed  int
}

// DocumentSummary represents AI-generated analysis of a document
// This is stored independently and can be referenced by DocumentContext
type DocumentSummary struct {
    DocumentID       string    // Reference to source document
    ExecutiveSummary string    // Brief overview (2-3 sentences)
    KeyPoints        []string  // Main takeaways
    Topics           []string  // Extracted topics
    Tags             []string  // Generated tags for categorization
    SuggestedStatus  string    // e.g., "In Review", "Approved"
    Confidence       float64   // AI confidence in analysis (0.0-1.0)
    GeneratedAt      time.Time
    Model            string    // e.g., "claude-3-7-sonnet"
    TokensUsed       int       // Tokens consumed for generation
}

// EmbeddingRequest contains text to embed
type EmbeddingRequest struct {
    Texts        []string // Support batch embedding
    ChunkSize    int      // Max tokens per chunk
    ChunkOverlap int      // Overlap between chunks
}

// EmbeddingResponse contains the generated embeddings
type EmbeddingResponse struct {
    Embeddings *DocumentEmbeddings
    Model      string
    Dimensions int
    TokensUsed int
}

// DocumentEmbeddings represents vector embeddings for a document
// This is stored independently and can be referenced by DocumentContext
type DocumentEmbeddings struct {
    DocumentID       string           // Reference to source document
    ContentEmbedding []float32        // Full document embedding
    Chunks           []ChunkEmbedding // Individual chunk embeddings
    Model            string           // e.g., "amazon.titan-embed-text-v2"
    Dimensions       int              // Embedding dimensions (e.g., 1024)
    GeneratedAt      time.Time
    TokensUsed       int              // Tokens consumed for generation
}

// ChunkEmbedding represents an embedding for a text chunk
type ChunkEmbedding struct {
    ChunkIndex int       // Sequential chunk number
    StartPos   int       // Character position in original text
    EndPos     int       // End character position
    Text       string    // Actual text content of chunk
    Embedding  []float32 // Vector embedding for this chunk
}
```

### AWS Bedrock Implementation

```go
// pkg/ai/bedrock/client.go
package bedrock

import (
    "context"
    "encoding/json"
    
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
    "github.com/hashicorp/hermes/pkg/ai"
)

// Provider implements ai.Provider using AWS Bedrock
type Provider struct {
    client        *bedrockruntime.Client
    summarizeModel string // "anthropic.claude-3-5-sonnet-20241022-v2:0"
    embeddingModel string // "amazon.titan-embed-text-v2:0"
}

// NewProvider creates a new Bedrock AI provider
func NewProvider(cfg *Config) (*Provider, error) {
    // Initialize AWS SDK client
    // ...
}

// Summarize uses Claude Sonnet 3.7 for document analysis
func (p *Provider) Summarize(ctx context.Context, req *ai.SummarizeRequest) (*ai.SummarizeResponse, error) {
    // Build prompt for Claude
    prompt := buildSummarizePrompt(req)
    
    // Call Bedrock InvokeModel API
    input := &bedrockruntime.InvokeModelInput{
        ModelId:     aws.String(p.summarizeModel),
        ContentType: aws.String("application/json"),
        Body:        []byte(prompt),
    }
    
    output, err := p.client.InvokeModel(ctx, input)
    if err != nil {
        return nil, fmt.Errorf("bedrock invoke failed: %w", err)
    }
    
    // Parse response into DocumentSummary
    summary, tokensUsed, err := parseSummarizeResponse(output.Body)
    if err != nil {
        return nil, err
    }
    
    return &ai.SummarizeResponse{
        Summary:    summary,
        Model:      p.summarizeModel,
        TokensUsed: tokensUsed,
    }, nil
}

// GenerateEmbedding uses Titan Embeddings V2
func (p *Provider) GenerateEmbedding(ctx context.Context, req *ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
    // Chunk text if needed
    chunks := chunkTexts(req.Texts, req.ChunkSize, req.ChunkOverlap)
    
    docEmbeddings := &ai.DocumentEmbeddings{
        Chunks:      make([]ai.ChunkEmbedding, 0, len(chunks)),
        Model:       p.embeddingModel,
        Dimensions:  1024, // Titan V2 dimensions
        GeneratedAt: time.Now(),
    }
    
    totalTokens := 0
    
    for _, chunk := range chunks {
        // Call Bedrock for each chunk
        input := &bedrockruntime.InvokeModelInput{
            ModelId:     aws.String(p.embeddingModel),
            ContentType: aws.String("application/json"),
            Body:        buildEmbeddingRequest(chunk.Text),
        }
        
        output, err := p.client.InvokeModel(ctx, input)
        if err != nil {
            return nil, fmt.Errorf("bedrock embedding failed: %w", err)
        }
        
        vector, tokens, err := parseEmbeddingResponse(output.Body)
        if err != nil {
            return nil, err
        }
        
        totalTokens += tokens
        
        docEmbeddings.Chunks = append(docEmbeddings.Chunks, ai.ChunkEmbedding{
            ChunkIndex: chunk.Index,
            StartPos:   chunk.StartPos,
            EndPos:     chunk.EndPos,
            Text:       chunk.Text,
            Embedding:  vector,
        })
        
        // Use first chunk as content embedding
        if chunk.Index == 0 {
            docEmbeddings.ContentEmbedding = vector
        }
    }
    
    docEmbeddings.TokensUsed = totalTokens
    
    return &ai.EmbeddingResponse{
        Embeddings: docEmbeddings,
        Model:      p.embeddingModel,
        Dimensions: 1024,
        TokensUsed: totalTokens,
    }, nil
}

func buildSummarizePrompt(req *ai.SummarizeRequest) string {
    return fmt.Sprintf(`You are analyzing a %s document titled "%s".

Please provide:
1. A concise executive summary (2-3 sentences)
2. 3-5 key points or takeaways
3. Main topics covered
4. Suggested tags for categorization
5. Recommended document status based on content maturity

Document content:
%s

Respond in JSON format:
{
  "executive_summary": "...",
  "key_points": ["...", "..."],
  "topics": ["...", "..."],
  "suggested_tags": ["...", "..."],
  "suggested_status": "...",
  "confidence": 0.95
}`, req.DocType, req.Title, req.Content)
}
```

### Extended Search Provider Interface

```go
// pkg/search/search.go - Add VectorIndex interface

// VectorIndex handles vector similarity search operations
type VectorIndex interface {
    // IndexEmbedding stores a document's vector embedding
    IndexEmbedding(ctx context.Context, doc *VectorDocument) error
    
    // IndexEmbeddingBatch stores multiple document embeddings
    IndexEmbeddingBatch(ctx context.Context, docs []*VectorDocument) error
    
    // SearchSimilar finds documents similar to the query embedding
    SearchSimilar(ctx context.Context, query *VectorSearchQuery) (*VectorSearchResult, error)
    
    // SearchHybrid combines vector similarity with keyword search
    SearchHybrid(ctx context.Context, query *HybridSearchQuery) (*SearchResult, error)
    
    // Delete removes a document's embeddings
    Delete(ctx context.Context, docID string) error
    
    // DeleteBatch removes multiple documents' embeddings
    DeleteBatch(ctx context.Context, docIDs []string) error
    
    // GetEmbedding retrieves stored embedding for a document
    GetEmbedding(ctx context.Context, docID string) (*VectorDocument, error)
    
    // Clear removes all vector data (use with caution)
    Clear(ctx context.Context) error
}

// VectorDocument represents a document with embeddings
type VectorDocument struct {
    ObjectID    string
    DocID       string
    Title       string
    DocType     string
    
    // Vector embeddings
    ContentEmbedding []float32        // Full document embedding
    ChunkEmbeddings  []ChunkEmbedding // Individual chunk embeddings
    
    // Metadata for hybrid search
    Summary      string
    KeyPoints    []string
    Topics       []string
    Tags         []string
    
    // Embedding info
    Model        string
    Dimensions   int
    EmbeddedAt   time.Time
}

// ChunkEmbedding represents an embedding for a text chunk
type ChunkEmbedding struct {
    ChunkIndex int
    Text       string
    Embedding  []float32
    StartPos   int
    EndPos     int
}

// VectorSearchQuery for similarity search
type VectorSearchQuery struct {
    QueryEmbedding []float32
    Limit          int
    Threshold      float64 // Minimum similarity score
    Filters        map[string]interface{} // Filter by docType, status, etc.
}

// HybridSearchQuery combines vector and keyword search
type HybridSearchQuery struct {
    QueryText      string
    QueryEmbedding []float32
    VectorWeight   float64 // 0.0-1.0, weight for vector search
    KeywordWeight  float64 // 0.0-1.0, weight for keyword search
    Limit          int
    Filters        map[string]interface{}
}

// VectorSearchResult contains similar documents
type VectorSearchResult struct {
    Hits  []VectorHit
    Total int
    Took  time.Duration
}

// VectorHit represents a search result with similarity score
type VectorHit struct {
    Document   *VectorDocument
    Score      float64 // Similarity score (0.0-1.0)
    MatchedChunks []int // Which chunks matched for chunked embeddings
}
```

### Meilisearch Vector Search Extension

```go
// pkg/search/meilisearch/vector.go
package meilisearch

import (
    "context"
    "fmt"
    
    "github.com/hashicorp/hermes/pkg/search"
)

// VectorIndex implements search.VectorIndex for Meilisearch
type VectorIndex struct {
    client    *meilisearch.Client
    indexName string
}

// NewVectorIndex creates a Meilisearch vector index
func NewVectorIndex(client *meilisearch.Client, indexName string) (*VectorIndex, error) {
    // Configure index for vector search
    // Meilisearch 1.11+ supports vector search
    _, err := client.Index(indexName).UpdateSettings(&meilisearch.Settings{
        SearchableAttributes: []string{"title", "summary", "content"},
        FilterableAttributes: []string{"docType", "status", "owners", "docNumber"},
        SortableAttributes:   []string{"modifiedTime", "createdTime"},
        // Enable vector search (Meilisearch 1.11+)
        Embedders: map[string]meilisearch.Embedder{
            "default": {
                Source: "userProvided",
                Dimensions: 1024, // Titan V2 dimensions
            },
        },
    })
    
    if err != nil {
        return nil, fmt.Errorf("failed to configure vector index: %w", err)
    }
    
    return &VectorIndex{
        client:    client,
        indexName: indexName,
    }, nil
}

// IndexEmbedding stores a document with its vector embedding
func (v *VectorIndex) IndexEmbedding(ctx context.Context, doc *search.VectorDocument) error {
    // Convert to Meilisearch document format
    msDoc := map[string]interface{}{
        "id":              doc.ObjectID,
        "docID":           doc.DocID,
        "title":           doc.Title,
        "docType":         doc.DocType,
        "summary":         doc.Summary,
        "keyPoints":       doc.KeyPoints,
        "topics":          doc.Topics,
        "tags":            doc.Tags,
        "_vectors": map[string][]float32{
            "default": doc.ContentEmbedding,
        },
    }
    
    // Add chunk embeddings if present
    if len(doc.ChunkEmbeddings) > 0 {
        chunks := make([]map[string]interface{}, len(doc.ChunkEmbeddings))
        for i, chunk := range doc.ChunkEmbeddings {
            chunks[i] = map[string]interface{}{
                "index":     chunk.ChunkIndex,
                "text":      chunk.Text,
                "embedding": chunk.Embedding,
            }
        }
        msDoc["chunks"] = chunks
    }
    
    _, err := v.client.Index(v.indexName).AddDocuments([]map[string]interface{}{msDoc})
    return err
}

// SearchSimilar performs vector similarity search
func (v *VectorIndex) SearchSimilar(ctx context.Context, query *search.VectorSearchQuery) (*search.VectorSearchResult, error) {
    // Use Meilisearch vector search
    req := &meilisearch.SearchRequest{
        Vector:  query.QueryEmbedding,
        Limit:   int64(query.Limit),
        Filter:  buildMeilisearchFilter(query.Filters),
        Hybrid: &meilisearch.Hybrid{
            SemanticRatio: 1.0, // Pure vector search
        },
    }
    
    resp, err := v.client.Index(v.indexName).Search("", req)
    if err != nil {
        return nil, fmt.Errorf("vector search failed: %w", err)
    }
    
    // Convert results
    return convertMeilisearchVectorResults(resp, query.Threshold)
}

// SearchHybrid combines vector and keyword search
func (v *VectorIndex) SearchHybrid(ctx context.Context, query *search.HybridSearchQuery) (*search.SearchResult, error) {
    req := &meilisearch.SearchRequest{
        Query:   query.QueryText,
        Vector:  query.QueryEmbedding,
        Limit:   int64(query.Limit),
        Filter:  buildMeilisearchFilter(query.Filters),
        Hybrid: &meilisearch.Hybrid{
            SemanticRatio: query.VectorWeight, // Balance between vector and keyword
        },
    }
    
    resp, err := v.client.Index(v.indexName).Search(query.QueryText, req)
    if err != nil {
        return nil, fmt.Errorf("hybrid search failed: %w", err)
    }
    
    return convertMeilisearchResults(resp)
}
```

### Persistence & Caching Strategy

**Database Schema for AI Structures**:

```sql
-- Option 1: JSONB columns in documents table (simple)
ALTER TABLE documents ADD COLUMN ai_summary JSONB;
ALTER TABLE documents ADD COLUMN ai_embeddings JSONB;

-- Option 2: Dedicated tables (recommended for querying/indexing)
CREATE TABLE document_summaries (
    id SERIAL PRIMARY KEY,
    document_id VARCHAR(500) NOT NULL,
    document_uuid UUID,
    executive_summary TEXT,
    key_points JSONB,
    topics JSONB,
    tags JSONB,
    suggested_status VARCHAR(50),
    confidence FLOAT,
    generated_at TIMESTAMP NOT NULL,
    model VARCHAR(100),
    tokens_used INTEGER,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(document_id, model)  -- One summary per model per doc
);

CREATE INDEX idx_summaries_doc_id ON document_summaries(document_id);
CREATE INDEX idx_summaries_uuid ON document_summaries(document_uuid);
CREATE INDEX idx_summaries_generated ON document_summaries(generated_at);

CREATE TABLE document_embeddings (
    id SERIAL PRIMARY KEY,
    document_id VARCHAR(500) NOT NULL,
    document_uuid UUID,
    content_embedding VECTOR(1024),  -- pgvector extension
    chunks JSONB,  -- Store chunk embeddings as JSONB
    model VARCHAR(100),
    dimensions INTEGER,
    generated_at TIMESTAMP NOT NULL,
    tokens_used INTEGER,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(document_id, model)  -- One embedding per model per doc
);

CREATE INDEX idx_embeddings_doc_id ON document_embeddings(document_id);
CREATE INDEX idx_embeddings_uuid ON document_embeddings(document_uuid);
CREATE INDEX idx_embeddings_vector ON document_embeddings 
    USING ivfflat (content_embedding vector_cosine_ops);  -- pgvector index
```

**Loading Cached AI Data**:

```go
// pkg/indexer/commands/load_ai_data.go
type LoadAIDataCommand struct {
    DB *gorm.DB
    MaxAge time.Duration  // e.g., 30 days
}

func (c *LoadAIDataCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    // Load cached summary if recent enough
    var summary models.DocumentSummary
    err := c.DB.Where("document_id = ? AND generated_at > ?", 
        doc.Document.ID, 
        time.Now().Add(-c.MaxAge),
    ).Order("generated_at DESC").First(&summary).Error
    
    if err == nil {
        // Found recent summary, use it
        doc.AISummary = &ai.DocumentSummary{
            DocumentID:       summary.DocumentID,
            ExecutiveSummary: summary.ExecutiveSummary,
            KeyPoints:        summary.KeyPoints,
            Topics:           summary.Topics,
            Tags:             summary.Tags,
            SuggestedStatus:  summary.SuggestedStatus,
            Confidence:       summary.Confidence,
            GeneratedAt:      summary.GeneratedAt,
            Model:            summary.Model,
            TokensUsed:       summary.TokensUsed,
        }
    }
    
    // Load cached embeddings if recent enough
    var embeddings models.DocumentEmbeddings
    err = c.DB.Where("document_id = ? AND generated_at > ?", 
        doc.Document.ID, 
        time.Now().Add(-c.MaxAge),
    ).Order("generated_at DESC").First(&embeddings).Error
    
    if err == nil {
        // Found recent embeddings, use them
        doc.Embeddings = &ai.DocumentEmbeddings{
            DocumentID:       embeddings.DocumentID,
            ContentEmbedding: embeddings.ContentEmbedding,
            Chunks:           parseChunks(embeddings.ChunksJSON),
            Model:            embeddings.Model,
            Dimensions:       embeddings.Dimensions,
            GeneratedAt:      embeddings.GeneratedAt,
            TokensUsed:       embeddings.TokensUsed,
        }
    }
    
    return nil
}
```

**Pipeline with Caching**:

```yaml
# config/indexer/plans/ai-with-caching.yaml
pipelines:
  - name: ai-enhanced-with-cache
    commands:
      - type: discover
      - type: assign-uuid
      - type: extract
      - type: load-ai-data      # Load cached AI data first
        config:
          max_age: 720h         # 30 days
      - type: summarize
        config:
          skip_if_cached: true  # Only generate if not loaded
      - type: embedding
        config:
          skip_if_cached: true  # Only generate if not loaded
      - type: index
      - type: index-vector
      - type: track
```

**Cost Optimization**:

```go
// Track AI usage for cost control
type AIUsageTracker struct {
    DailyLimit   int
    CurrentUsage int
    ResetAt      time.Time
}

func (t *AIUsageTracker) CanUseAI(tokensNeeded int) bool {
    if time.Now().After(t.ResetAt) {
        t.CurrentUsage = 0
        t.ResetAt = time.Now().Add(24 * time.Hour)
    }
    
    return t.CurrentUsage + tokensNeeded <= t.DailyLimit
}

func (t *AIUsageTracker) RecordUsage(tokensUsed int) {
    t.CurrentUsage += tokensUsed
}
```

### Database Schema for Document Summaries

**SQL Schema**:

```sql
-- Document Summaries Table
CREATE TABLE document_summaries (
    id BIGSERIAL PRIMARY KEY,
    
    -- Document identification
    document_id VARCHAR(500) NOT NULL,        -- Provider-specific document ID (e.g., Google Drive ID)
    document_uuid UUID,                        -- Stable UUID across providers
    
    -- Summary content
    executive_summary TEXT NOT NULL,           -- 2-3 sentence overview
    key_points JSONB,                          -- Array of key takeaways
    topics JSONB,                              -- Array of main topics
    tags JSONB,                                -- Array of suggested tags
    
    -- AI analysis
    suggested_status VARCHAR(50),              -- e.g., "In Review", "Approved", "Draft"
    confidence DOUBLE PRECISION,               -- AI confidence score (0.0-1.0)
    
    -- Metadata
    model VARCHAR(100) NOT NULL,               -- e.g., "claude-3-5-sonnet-20241022-v2:0"
    provider VARCHAR(50) NOT NULL,             -- e.g., "bedrock", "openai"
    tokens_used INTEGER,                       -- Tokens consumed for this summary
    generation_time_ms INTEGER,                -- Time taken to generate (milliseconds)
    
    -- Document context at time of generation
    document_title VARCHAR(500),
    document_type VARCHAR(50),                 -- e.g., "RFC", "PRD", "FRD"
    content_hash VARCHAR(64),                  -- SHA-256 of content used for summary
    content_length INTEGER,                    -- Character count of source document
    
    -- Timestamps
    generated_at TIMESTAMP NOT NULL,           -- When AI generated this
    created_at TIMESTAMP DEFAULT NOW(),        -- When record was created
    updated_at TIMESTAMP DEFAULT NOW(),        -- When record was last updated
    
    -- Constraints
    CONSTRAINT unique_doc_model UNIQUE (document_id, model),
    CONSTRAINT unique_uuid_model UNIQUE (document_uuid, model)
);

-- Indexes for efficient querying
CREATE INDEX idx_doc_summaries_doc_id ON document_summaries(document_id);
CREATE INDEX idx_doc_summaries_uuid ON document_summaries(document_uuid);
CREATE INDEX idx_doc_summaries_generated ON document_summaries(generated_at DESC);
CREATE INDEX idx_doc_summaries_model ON document_summaries(model);
CREATE INDEX idx_doc_summaries_content_hash ON document_summaries(content_hash);
CREATE INDEX idx_doc_summaries_doc_type ON document_summaries(document_type);

-- GIN indexes for JSONB fields (for querying within arrays)
CREATE INDEX idx_doc_summaries_topics ON document_summaries USING GIN (topics);
CREATE INDEX idx_doc_summaries_tags ON document_summaries USING GIN (tags);
CREATE INDEX idx_doc_summaries_key_points ON document_summaries USING GIN (key_points);

-- Trigger to update updated_at
CREATE OR REPLACE FUNCTION update_document_summaries_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_document_summaries_updated_at
    BEFORE UPDATE ON document_summaries
    FOR EACH ROW
    EXECUTE FUNCTION update_document_summaries_updated_at();
```

**GORM Model**:

```go
// pkg/models/document_summary.go
package models

import (
    "database/sql/driver"
    "encoding/json"
    "time"
    
    "github.com/google/uuid"
    "gorm.io/gorm"
)

// DocumentSummary stores AI-generated summaries and analysis of documents
type DocumentSummary struct {
    ID uint `gorm:"primaryKey" json:"id"`
    
    // Document identification
    DocumentID   string     `gorm:"type:varchar(500);not null;index" json:"documentId"`
    DocumentUUID *uuid.UUID `gorm:"type:uuid;index" json:"documentUuid,omitempty"`
    
    // Summary content
    ExecutiveSummary string         `gorm:"type:text;not null" json:"executiveSummary"`
    KeyPoints        StringArray    `gorm:"type:jsonb" json:"keyPoints"`
    Topics           StringArray    `gorm:"type:jsonb" json:"topics"`
    Tags             StringArray    `gorm:"type:jsonb" json:"tags"`
    
    // AI analysis
    SuggestedStatus string   `gorm:"type:varchar(50)" json:"suggestedStatus,omitempty"`
    Confidence      *float64 `gorm:"type:double precision" json:"confidence,omitempty"`
    
    // Metadata
    Model            string `gorm:"type:varchar(100);not null;index" json:"model"`
    Provider         string `gorm:"type:varchar(50);not null" json:"provider"`
    TokensUsed       *int   `gorm:"type:integer" json:"tokensUsed,omitempty"`
    GenerationTimeMs *int   `gorm:"type:integer" json:"generationTimeMs,omitempty"`
    
    // Document context at time of generation
    DocumentTitle  string `gorm:"type:varchar(500)" json:"documentTitle,omitempty"`
    DocumentType   string `gorm:"type:varchar(50);index" json:"documentType,omitempty"`
    ContentHash    string `gorm:"type:varchar(64);index" json:"contentHash,omitempty"`
    ContentLength  *int   `gorm:"type:integer" json:"contentLength,omitempty"`
    
    // Timestamps
    GeneratedAt time.Time `gorm:"not null;index:,sort:desc" json:"generatedAt"`
    CreatedAt   time.Time `gorm:"autoCreateTime" json:"createdAt"`
    UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName specifies the table name
func (DocumentSummary) TableName() string {
    return "document_summaries"
}

// StringArray is a custom type for storing string arrays in JSONB
type StringArray []string

// Scan implements the sql.Scanner interface
func (s *StringArray) Scan(value interface{}) error {
    if value == nil {
        *s = StringArray{}
        return nil
    }
    
    bytes, ok := value.([]byte)
    if !ok {
        return fmt.Errorf("failed to unmarshal JSONB value: %v", value)
    }
    
    var arr []string
    if err := json.Unmarshal(bytes, &arr); err != nil {
        return err
    }
    
    *s = StringArray(arr)
    return nil
}

// Value implements the driver.Valuer interface
func (s StringArray) Value() (driver.Value, error) {
    if s == nil {
        return nil, nil
    }
    return json.Marshal(s)
}

// BeforeCreate hook to ensure required fields
func (ds *DocumentSummary) BeforeCreate(tx *gorm.DB) error {
    if ds.GeneratedAt.IsZero() {
        ds.GeneratedAt = time.Now()
    }
    if ds.DocumentID == "" {
        return fmt.Errorf("document_id is required")
    }
    if ds.ExecutiveSummary == "" {
        return fmt.Errorf("executive_summary is required")
    }
    if ds.Model == "" {
        return fmt.Errorf("model is required")
    }
    if ds.Provider == "" {
        return fmt.Errorf("provider is required")
    }
    return nil
}

// GetLatestByDocumentID retrieves the most recent summary for a document
func (ds *DocumentSummary) GetLatestByDocumentID(db *gorm.DB, documentID string) error {
    return db.Where("document_id = ?", documentID).
        Order("generated_at DESC").
        First(ds).Error
}

// GetLatestByUUID retrieves the most recent summary for a document UUID
func (ds *DocumentSummary) GetLatestByUUID(db *gorm.DB, documentUUID uuid.UUID) error {
    return db.Where("document_uuid = ?", documentUUID).
        Order("generated_at DESC").
        First(ds).Error
}

// GetByDocumentIDAndModel retrieves a summary for a specific document and model
func (ds *DocumentSummary) GetByDocumentIDAndModel(db *gorm.DB, documentID, model string) error {
    return db.Where("document_id = ? AND model = ?", documentID, model).
        First(ds).Error
}

// IsStale checks if the summary is older than the specified duration
func (ds *DocumentSummary) IsStale(maxAge time.Duration) bool {
    return time.Since(ds.GeneratedAt) > maxAge
}

// MatchesContentHash checks if the summary was generated from the same content
func (ds *DocumentSummary) MatchesContentHash(hash string) bool {
    return ds.ContentHash == hash
}

// List retrieves summaries with optional filters
func ListDocumentSummaries(db *gorm.DB, filters *SummaryFilters) ([]DocumentSummary, error) {
    query := db.Model(&DocumentSummary{})
    
    if filters != nil {
        if filters.DocumentID != "" {
            query = query.Where("document_id = ?", filters.DocumentID)
        }
        if filters.DocumentUUID != nil {
            query = query.Where("document_uuid = ?", filters.DocumentUUID)
        }
        if filters.DocumentType != "" {
            query = query.Where("document_type = ?", filters.DocumentType)
        }
        if filters.Model != "" {
            query = query.Where("model = ?", filters.Model)
        }
        if filters.Provider != "" {
            query = query.Where("provider = ?", filters.Provider)
        }
        if !filters.GeneratedAfter.IsZero() {
            query = query.Where("generated_at > ?", filters.GeneratedAfter)
        }
        if filters.HasTopic != "" {
            query = query.Where("topics @> ?", fmt.Sprintf(`["%s"]`, filters.HasTopic))
        }
        if filters.HasTag != "" {
            query = query.Where("tags @> ?", fmt.Sprintf(`["%s"]`, filters.HasTag))
        }
    }
    
    var summaries []DocumentSummary
    err := query.Order("generated_at DESC").Find(&summaries).Error
    return summaries, err
}

// SummaryFilters defines filters for querying summaries
type SummaryFilters struct {
    DocumentID     string
    DocumentUUID   *uuid.UUID
    DocumentType   string
    Model          string
    Provider       string
    GeneratedAfter time.Time
    HasTopic       string // Find summaries containing this topic
    HasTag         string // Find summaries containing this tag
}

// DeleteOldSummaries removes summaries older than the specified age
func DeleteOldSummaries(db *gorm.DB, olderThan time.Duration) (int64, error) {
    cutoff := time.Now().Add(-olderThan)
    result := db.Where("generated_at < ?", cutoff).Delete(&DocumentSummary{})
    return result.RowsAffected, result.Error
}

// GetSummaryStats returns statistics about stored summaries
func GetSummaryStats(db *gorm.DB) (*SummaryStats, error) {
    var stats SummaryStats
    
    // Total count
    db.Model(&DocumentSummary{}).Count(&stats.TotalCount)
    
    // Count by provider
    var providerCounts []struct {
        Provider string
        Count    int64
    }
    db.Model(&DocumentSummary{}).
        Select("provider, COUNT(*) as count").
        Group("provider").
        Scan(&providerCounts)
    stats.ByProvider = make(map[string]int64)
    for _, pc := range providerCounts {
        stats.ByProvider[pc.Provider] = pc.Count
    }
    
    // Count by model
    var modelCounts []struct {
        Model string
        Count int64
    }
    db.Model(&DocumentSummary{}).
        Select("model, COUNT(*) as count").
        Group("model").
        Scan(&modelCounts)
    stats.ByModel = make(map[string]int64)
    for _, mc := range modelCounts {
        stats.ByModel[mc.Model] = mc.Count
    }
    
    // Total tokens used
    var totalTokens sql.NullInt64
    db.Model(&DocumentSummary{}).
        Select("SUM(tokens_used)").
        Scan(&totalTokens)
    if totalTokens.Valid {
        stats.TotalTokensUsed = totalTokens.Int64
    }
    
    // Average generation time
    var avgTime sql.NullFloat64
    db.Model(&DocumentSummary{}).
        Select("AVG(generation_time_ms)").
        Scan(&avgTime)
    if avgTime.Valid {
        stats.AvgGenerationTimeMs = avgTime.Float64
    }
    
    return &stats, nil
}

// SummaryStats contains statistics about document summaries
type SummaryStats struct {
    TotalCount          int64
    ByProvider          map[string]int64
    ByModel             map[string]int64
    TotalTokensUsed     int64
    AvgGenerationTimeMs float64
}
```

**Migration File**:

```go
// pkg/models/migrations/YYYYMMDDHHMMSS_create_document_summaries.go
package migrations

import (
    "github.com/hashicorp/hermes/pkg/models"
    "gorm.io/gorm"
)

func CreateDocumentSummariesTable(db *gorm.DB) error {
    return db.AutoMigrate(&models.DocumentSummary{})
}

func DropDocumentSummariesTable(db *gorm.DB) error {
    return db.Migrator().DropTable(&models.DocumentSummary{})
}
```

**Usage Example in Summarize Command**:

```go
// pkg/indexer/commands/summarize.go
func (c *SummarizeReviewCommand) storeSummary(doc *DocumentContext) error {
    if c.DB == nil {
        return nil // Skip storage if no DB configured
    }
    
    startTime := time.Now()
    
    // Create GORM model from ai.DocumentSummary
    dbSummary := &models.DocumentSummary{
        DocumentID:       doc.Document.ID,
        DocumentUUID:     doc.DocumentUUID,
        ExecutiveSummary: doc.AISummary.ExecutiveSummary,
        KeyPoints:        models.StringArray(doc.AISummary.KeyPoints),
        Topics:           models.StringArray(doc.AISummary.Topics),
        Tags:             models.StringArray(doc.AISummary.Tags),
        SuggestedStatus:  doc.AISummary.SuggestedStatus,
        Confidence:       &doc.AISummary.Confidence,
        Model:            doc.AISummary.Model,
        Provider:         c.AIProvider.Name(),
        TokensUsed:       &doc.AISummary.TokensUsed,
        DocumentTitle:    doc.Document.Name,
        DocumentType:     doc.Metadata.DocType,
        ContentHash:      doc.ContentHash,
        GeneratedAt:      doc.AISummary.GeneratedAt,
    }
    
    if doc.Content != "" {
        contentLen := len(doc.Content)
        dbSummary.ContentLength = &contentLen
    }
    
    // Upsert: create or update based on unique constraint
    result := c.DB.Where(models.DocumentSummary{
        DocumentID: doc.Document.ID,
        Model:      doc.AISummary.Model,
    }).Assign(dbSummary).FirstOrCreate(dbSummary)
    
    if result.Error != nil {
        return fmt.Errorf("failed to store summary: %w", result.Error)
    }
    
    generationTime := int(time.Since(startTime).Milliseconds())
    dbSummary.GenerationTimeMs = &generationTime
    c.DB.Save(dbSummary)
    
    return nil
}
```

**Loading Cached Summaries**:

```go
// pkg/indexer/commands/load_ai_data.go
func (c *LoadAIDataCommand) loadSummary(ctx context.Context, doc *DocumentContext) error {
    var summary models.DocumentSummary
    
    // Try to load by UUID first (stable across providers)
    if doc.DocumentUUID != nil {
        err := summary.GetLatestByUUID(c.DB, *doc.DocumentUUID)
        if err == nil && !summary.IsStale(c.MaxAge) {
            doc.AISummary = c.convertToAISummary(&summary)
            return nil
        }
    }
    
    // Fall back to document ID
    err := summary.GetLatestByDocumentID(c.DB, doc.Document.ID)
    if err != nil {
        return err // Not found or error
    }
    
    // Check if stale or content changed
    if summary.IsStale(c.MaxAge) {
        return fmt.Errorf("summary is stale")
    }
    
    if doc.ContentHash != "" && !summary.MatchesContentHash(doc.ContentHash) {
        return fmt.Errorf("content has changed since summary was generated")
    }
    
    doc.AISummary = c.convertToAISummary(&summary)
    return nil
}

func (c *LoadAIDataCommand) convertToAISummary(dbSummary *models.DocumentSummary) *ai.DocumentSummary {
    summary := &ai.DocumentSummary{
        DocumentID:       dbSummary.DocumentID,
        ExecutiveSummary: dbSummary.ExecutiveSummary,
        KeyPoints:        []string(dbSummary.KeyPoints),
        Topics:           []string(dbSummary.Topics),
        Tags:             []string(dbSummary.Tags),
        SuggestedStatus:  dbSummary.SuggestedStatus,
        GeneratedAt:      dbSummary.GeneratedAt,
        Model:            dbSummary.Model,
    }
    
    if dbSummary.Confidence != nil {
        summary.Confidence = *dbSummary.Confidence
    }
    
    if dbSummary.TokensUsed != nil {
        summary.TokensUsed = *dbSummary.TokensUsed
    }
    
    return summary
}
```

## Implementation Phases

### Phase 1: Core Abstractions (Week 1)

**Goal**: Create command interfaces and basic pipeline

```go
// pkg/indexer/command.go
type Command interface {
    Execute(ctx context.Context, doc *DocumentContext) error
    Name() string
}

// Optional: Commands that can process multiple documents efficiently
type BatchCommand interface {
    Command
    ExecuteBatch(ctx context.Context, docs []*DocumentContext) error
}

// pkg/indexer/pipeline.go
type Pipeline struct {
    name     string
    commands []Command
}

func (p *Pipeline) Execute(ctx context.Context, docs []*DocumentContext) error

// pkg/indexer/context.go
type DocumentContext struct {
    Document *workspace.Document
    Metadata *models.Document
    Content  string
    // ...
}
```

**Deliverables**:
- [ ] `pkg/indexer/command.go` - Command interface (no rollback needed)
- [ ] `pkg/indexer/pipeline.go` - Pipeline execution
- [ ] `pkg/indexer/context.go` - Document context
- [ ] Unit tests for pipeline execution

### Phase 2: Basic Commands (Week 2)

**Goal**: Implement essential commands

```go
// pkg/indexer/commands/discover.go
type DiscoverCommand struct {
    Provider  workspace.DocumentStorage
    FolderID  string
    Since     time.Time
    Until     time.Time
}

func (c *DiscoverCommand) Execute(ctx context.Context, _ *DocumentContext) ([]*DocumentContext, error) {
    opts := &workspace.ListOptions{
        ModifiedAfter: &c.Since,
    }
    docs, err := c.Provider.ListDocuments(ctx, c.FolderID, opts)
    // Convert to DocumentContext slice
}

// pkg/indexer/commands/extract.go
type ExtractContentCommand struct {
    Provider workspace.DocumentStorage
    MaxSize  int
}

func (c *ExtractContentCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    content, err := c.Provider.GetDocumentContent(ctx, doc.Document.ID)
    if len(content) > c.MaxSize {
        content = content[:c.MaxSize]
    }
    doc.Content = content
    return nil
}
```

**Deliverables**:
- [ ] `pkg/indexer/commands/discover.go` - Document discovery
- [ ] `pkg/indexer/commands/extract.go` - Content extraction
- [ ] `pkg/indexer/commands/transform.go` - Transform to search format
- [ ] `pkg/indexer/commands/index.go` - Index in search provider
- [ ] Integration tests with mock providers

### Phase 3: Orchestrator (Week 3)

**Goal**: Create the main orchestrator that replaces current `Indexer.Run()`

```go
// pkg/indexer/orchestrator.go
type Orchestrator struct {
    db             *gorm.DB
    logger         hclog.Logger
    
    // Providers
    workspaceProvider workspace.StorageProvider
    searchProvider    search.Provider
    
    // Configuration
    config *Config
    
    // Pipelines
    pipelines map[string]*Pipeline
}

func (o *Orchestrator) Run(ctx context.Context) error {
    ticker := time.NewTicker(o.config.RunInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := o.runCycle(ctx); err != nil {
                o.logger.Error("indexer cycle failed", "error", err)
            }
        }
    }
}

func (o *Orchestrator) runCycle(ctx context.Context) error {
    // Get last run metadata
    md := models.IndexerMetadata{}
    if err := md.Get(o.db); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
        return err
    }
    
    // Execute configured pipelines
    for _, pipelineName := range o.config.EnabledPipelines {
        pipeline := o.pipelines[pipelineName]
        if err := o.executePipeline(ctx, pipeline); err != nil {
            return fmt.Errorf("pipeline %s failed: %w", pipelineName, err)
        }
    }
    
    return nil
}
```

**Deliverables**:
- [ ] `pkg/indexer/orchestrator.go` - Main orchestration logic
- [ ] `pkg/indexer/config.go` - Configuration structures
- [ ] Pipeline registration and discovery
- [ ] Integration with existing database tracking

### Phase 4: Migration Commands (Week 4)

**Goal**: Enable document migration between providers

```go
// pkg/indexer/commands/migrate.go
type MigrateCommand struct {
    Source workspace.DocumentStorage
    Target workspace.DocumentStorage
    
    // Options
    SkipExisting bool
    DryRun       bool
}

func (c *MigrateCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    // Check if document exists in target
    if c.SkipExisting {
        existing, err := c.Target.GetDocument(ctx, doc.Document.ID)
        if err == nil && existing != nil {
            return nil // Skip
        }
    }
    
    if c.DryRun {
        log.Info("would migrate", "id", doc.Document.ID, "name", doc.Document.Name)
        return nil
    }
    
    // Create document in target
    createOpts := &workspace.DocumentCreate{
        Name:           doc.Document.Name,
        ParentFolderID: doc.TargetFolderID,
        Content:        doc.Content,
        Owner:          doc.Document.Owner,
        Metadata:       doc.Document.Metadata,
    }
    
    targetDoc, err := c.Target.CreateDocument(ctx, createOpts)
    if err != nil {
        return fmt.Errorf("failed to create document in target: %w", err)
    }
    
    doc.TargetDocument = targetDoc
    return nil
}
```

**Deliverables**:
- [ ] `pkg/indexer/commands/migrate.go` - Document migration
- [ ] Support for dry-run mode
- [ ] Conflict resolution strategies
- [ ] Migration progress tracking

### Phase 5: Local Testing Infrastructure (Week 5)

**Goal**: Enable full integration testing with local workspace

```go
// testing/indexer/integration_test.go
func TestLocalIndexerIntegration(t *testing.T) {
    ctx := context.Background()
    
    // Setup local workspace
    localProvider := setupLocalWorkspace(t)
    
    // Create test documents
    docs := createTestDocuments(t, localProvider)
    
    // Setup search provider (Meilisearch in docker)
    searchProvider := setupSearchProvider(t)
    
    // Setup database
    db := setupDatabase(t)
    
    // Create orchestrator
    orchestrator := indexer.NewOrchestrator(
        indexer.WithDatabase(db),
        indexer.WithWorkspaceProvider(localProvider),
        indexer.WithSearchProvider(searchProvider),
        indexer.WithLogger(testLogger),
        indexer.WithConfig(&indexer.Config{
            RunInterval: 5 * time.Second,
            EnabledPipelines: []string{"index-published"},
            MaxParallelDocs: 2,
        }),
    )
    
    // Run one cycle
    err := orchestrator.ExecutePipeline(ctx, "index-published")
    require.NoError(t, err)
    
    // Verify documents are indexed
    for _, doc := range docs {
        result, err := searchProvider.DocumentIndex().GetObject(ctx, doc.ID)
        require.NoError(t, err)
        assert.Equal(t, doc.Name, result.Title)
    }
}

// testing/indexer/fixtures.go
func setupLocalWorkspace(t *testing.T) workspace.StorageProvider {
    basePath := t.TempDir()
    cfg := &local.Config{
        BasePath:   basePath,
        DocsPath:   filepath.Join(basePath, "docs"),
        DraftsPath: filepath.Join(basePath, "drafts"),
    }
    
    adapter, err := local.NewAdapter(cfg)
    require.NoError(t, err)
    return adapter
}

func createTestDocuments(t *testing.T, provider workspace.StorageProvider) []*workspace.Document {
    ctx := context.Background()
    docs := []*workspace.Document{}
    
    // Create RFC
    rfc, err := provider.DocumentStorage().CreateDocument(ctx, &workspace.DocumentCreate{
        Name:           "RFC-001: Test Document",
        ParentFolderID: "docs",
        Content:        "# RFC-001\n\nThis is a test RFC document.",
        Metadata: map[string]any{
            "docType":   "RFC",
            "docNumber": "RFC-001",
            "status":    "In Review",
        },
    })
    require.NoError(t, err)
    docs = append(docs, rfc)
    
    // Create PRD
    prd, err := provider.DocumentStorage().CreateDocument(ctx, &workspace.DocumentCreate{
        Name:           "PRD-002: Test Product",
        ParentFolderID: "docs",
        Content:        "# PRD-002\n\nThis is a test PRD document.",
        Metadata: map[string]any{
            "docType":   "PRD",
            "docNumber": "PRD-002",
            "status":    "Approved",
        },
    })
    require.NoError(t, err)
    docs = append(docs, prd)
    
    return docs
}
```

**Test Scenarios**:
1. **Basic indexing**: Create documents, run indexer, verify in search
2. **Incremental updates**: Modify documents, run indexer, verify changes
3. **Header refresh**: Update metadata, run refresh, verify document updated
4. **Migration**: Migrate from one local folder to another
5. **Error handling**: Simulate failures, verify recovery
6. **Concurrent processing**: Process multiple documents in parallel

**Deliverables**:
- [ ] `testing/indexer/integration_test.go` - Integration tests
- [ ] `testing/indexer/fixtures.go` - Test data generation
- [ ] `testing/indexer/test-data/` - Sample documents
- [ ] Makefile targets for running local indexer tests
- [ ] Documentation for local testing setup

### Phase 6: CLI Integration (Week 6)

**Goal**: Update CLI to support new architecture

```go
// internal/cmd/commands/indexer/indexer.go
func (c *Command) Run(args []string) int {
    // ... flag parsing ...
    
    cfg, err := config.NewConfig(c.flagConfig, "")
    if err != nil {
        ui.Error(fmt.Sprintf("error parsing configuration: %v", err))
        return 1
    }
    
    // Initialize workspace provider (based on config)
    var workspaceProvider workspace.StorageProvider
    if cfg.WorkspaceProvider == "local" {
        localCfg := &local.Config{
            BasePath:   cfg.LocalWorkspace.BasePath,
            DocsPath:   cfg.LocalWorkspace.DocsPath,
            DraftsPath: cfg.LocalWorkspace.DraftsPath,
        }
        workspaceProvider, err = local.NewAdapter(localCfg)
    } else {
        // Default to Google Workspace
        workspaceProvider = google.NewFromConfig(cfg.GoogleWorkspace.Auth)
    }
    
    // Initialize search provider
    searchProvider, err := createSearchProvider(cfg)
    if err != nil {
        ui.Error(fmt.Sprintf("error initializing search: %v", err))
        return 1
    }
    
    // Create orchestrator
    orchestrator, err := indexer.NewOrchestrator(
        indexer.WithDatabase(db),
        indexer.WithWorkspaceProvider(workspaceProvider),
        indexer.WithSearchProvider(searchProvider),
        indexer.WithLogger(log),
        indexer.WithConfig(cfg.Indexer),
    )
    
    // Run based on mode
    if c.flagMigrate {
        return c.runMigration(orchestrator)
    } else {
        return c.runContinuous(orchestrator)
    }
}
```

**New CLI Flags**:
```bash
# Run with specific plan
./hermes indexer -config=config.hcl -plan=testing/indexer/plans/local-integration-test.yaml

# Run production plan
./hermes indexer -config=config.hcl -plan=config/indexer/plans/production.yaml

# Migrate from Google to Local (using migration plan)
./hermes indexer -config=config.hcl -plan=config/indexer/plans/migrate-google-to-local.yaml

# List available plans
./hermes indexer -config=config.hcl -list-plans

# Validate plan without running
./hermes indexer -config=config.hcl -plan=testing/indexer/plans/local.yaml -validate

# Dry run mode (override plan setting)
./hermes indexer -config=config.hcl -plan=production.yaml -dry-run
```

**Deliverables**:
- [ ] Update CLI command implementation
- [ ] Add plan loading and validation
- [ ] Add plan listing functionality
- [ ] Build orchestrator from plan configuration
- [ ] Update help text and examples

### Phase 7: Document Revisions & Migration (Week 7-8)

**Goal**: Implement UUID-based revision tracking and migration support

```go
// pkg/indexer/commands/assign_uuid.go
type AssignUUIDCommand struct {
    Provider workspace.DocumentStorage
}

func (c *AssignUUIDCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    // Check if document already has UUID in metadata
    if uuid, exists := doc.Document.Metadata["hermesUuid"]; exists && uuid != "" {
        doc.DocumentUUID = UUID(uuid.(string))
        return nil
    }
    
    // Generate new UUID
    doc.DocumentUUID = uuid.New()
    
    // Write UUID back to document metadata
    return c.Provider.UpdateMetadata(ctx, doc.Document.ID, map[string]any{
        "hermesUuid": doc.DocumentUUID.String(),
    })
}

// pkg/indexer/commands/hash.go
type CalculateHashCommand struct{}

func (c *CalculateHashCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    // Normalize content for hashing
    normalized := normalizeContent(doc.Content)
    
    // Include critical metadata
    hashInput := fmt.Sprintf("%s|%s|%s",
        normalized,
        doc.Document.Name,
        doc.Document.ModifiedTime.Format(time.RFC3339),
    )
    
    // SHA-256 hash
    hash := sha256.Sum256([]byte(hashInput))
    doc.ContentHash = fmt.Sprintf("sha256:%x", hash)
    return nil
}

// pkg/indexer/commands/revision.go
type TrackRevisionCommand struct {
    DB         *gorm.DB
    ProjectID  string
    ProviderType string
}

func (c *TrackRevisionCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    revision := &models.DocumentRevision{
        DocumentUUID:       doc.DocumentUUID,
        ProjectID:          c.ProjectID,
        ProviderType:       c.ProviderType,
        ProviderDocumentID: doc.Document.ID,
        ContentHash:        doc.ContentHash,
        LastModified:       doc.Document.ModifiedTime,
        Status:             "active",
        IndexedAt:          time.Now(),
    }
    
    // Upsert revision
    result := c.DB.Where(
        "document_uuid = ? AND project_id = ? AND provider_type = ?",
        doc.DocumentUUID, c.ProjectID, c.ProviderType,
    ).Assign(revision).FirstOrCreate(&revision)
    
    doc.Revision = revision
    return result.Error
}

// pkg/indexer/commands/conflict.go
type DetectConflictsCommand struct {
    DB *gorm.DB
}

func (c *DetectConflictsCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    // Find all revisions for this document
    var revisions []models.DocumentRevision
    err := c.DB.Where("document_uuid = ?", doc.DocumentUUID).Find(&revisions).Error
    if err != nil {
        return err
    }
    
    // Check for conflicts (different content hashes)
    hashes := make(map[string]bool)
    for _, rev := range revisions {
        if rev.Status == "active" || rev.Status == "source" || rev.Status == "target" {
            hashes[rev.ContentHash] = true
        }
    }
    
    if len(hashes) > 1 {
        // Conflict detected!
        doc.MigrationStatus = "conflict"
        doc.ConflictInfo = &ConflictInfo{
            DetectedAt:   time.Now(),
            ConflictType: "content-divergence",
            Resolution:   "pending",
        }
        
        // Update revisions to conflict status
        c.DB.Model(&models.DocumentRevision{}).
            Where("document_uuid = ? AND status IN (?)", doc.DocumentUUID, []string{"source", "target"}).
            Update("status", "conflict")
    }
    
    return nil
}
```

**Test Scenarios**:
1. **UUID assignment**: Create documents without UUIDs, verify UUIDs are generated and stored
2. **Content hashing**: Modify document, verify hash changes
3. **Revision tracking**: Index same document from multiple providers, verify revisions created
4. **Conflict detection**: Edit document in two providers, verify conflict detected
5. **Migration workflow**: Migrate documents, track revisions, detect conflicts

**Deliverables**:
- [ ] `pkg/indexer/commands/assign_uuid.go` - UUID assignment
- [ ] `pkg/indexer/commands/hash.go` - Content hash calculation
- [ ] `pkg/indexer/commands/revision.go` - Revision tracking
- [ ] `pkg/indexer/commands/conflict.go` - Conflict detection
- [ ] Database migration for `document_revisions` table
- [ ] Integration tests for migration workflow
- [ ] Documentation for UUID and revision system

### Phase 8: AI Summarization & Review (Week 9-10)

**Goal**: Implement AI-powered document summarization using AWS Bedrock

```go
// pkg/ai/provider.go - Interface (see earlier section for full definition)

// pkg/ai/bedrock/client.go - Bedrock implementation
type Provider struct {
    client         *bedrockruntime.Client
    summarizeModel string
    region         string
}

// pkg/indexer/commands/summarize.go
type SummarizeReviewCommand struct {
    AIProvider    ai.Provider
    ExtractTopics bool
    GenerateTags  bool
    AnalyzeStatus bool
}

func (c *SummarizeReviewCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    // Skip if document is too short
    if len(doc.Content) < 100 {
        return nil
    }
    
    // Skip if summary already exists and is recent
    if doc.AISummary != nil && 
       doc.AISummary.GeneratedAt.After(time.Now().Add(-30 * 24 * time.Hour)) {
        return nil // Already has recent summary
    }
    
    // Call AI provider
    req := &ai.SummarizeRequest{
        Content:          doc.Content,
        Title:            doc.Document.Name,
        DocType:          doc.Metadata.DocType,
        MaxSummaryLength: 500,
        ExtractTopics:    c.ExtractTopics,
        ExtractKeyPoints: true,
        SuggestTags:      c.GenerateTags,
        AnalyzeStatus:    c.AnalyzeStatus,
    }
    
    resp, err := c.AIProvider.Summarize(ctx, req)
    if err != nil {
        return fmt.Errorf("AI summarization failed: %w", err)
    }
    
    // Store DocumentSummary in context (external structure)
    resp.Summary.DocumentID = doc.Document.ID
    doc.AISummary = resp.Summary
    
    // Store in database for future reference
    return c.storeSummary(doc)
}

func (c *SummarizeReviewCommand) storeSummary(doc *DocumentContext) error {
    // Store AI summary in database (new table or JSONB column)
    // Option 1: Store as JSONB in documents table
    summaryData := map[string]interface{}{
        "executive_summary": doc.AISummary.ExecutiveSummary,
        "key_points":        doc.AISummary.KeyPoints,
        "topics":            doc.AISummary.Topics,
        "tags":              doc.AISummary.Tags,
        "suggested_status":  doc.AISummary.SuggestedStatus,
        "confidence":        doc.AISummary.Confidence,
        "generated_at":      doc.AISummary.GeneratedAt,
        "model":             doc.AISummary.Model,
        "tokens_used":       doc.AISummary.TokensUsed,
    }
    
    // Store in document metadata
    doc.Metadata.CustomFields["ai_summary"] = summaryData
    
    // Option 2: Store in dedicated table (recommended for querying)
    // db.Create(&models.DocumentSummary{...})
    
    return nil
}
```

**AWS Bedrock Configuration**:
```hcl
// config.hcl
ai_provider {
  type = "bedrock"
  
  bedrock {
    region           = "us-west-2"
    summarize_model  = "anthropic.claude-3-5-sonnet-20241022-v2:0"
    embedding_model  = "amazon.titan-embed-text-v2:0"
    
    // AWS credentials from environment or IAM role
    // AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN
  }
}

indexer {
  ai_enabled = true
  ai_summarize_on_index = true
  ai_generate_embeddings = true
}
```

**Test Scenarios**:
1. **Basic summarization**: Index document, verify summary generated
2. **Topic extraction**: Verify topics identified correctly
3. **Tag generation**: Check generated tags are relevant
4. **Status analysis**: Verify document status suggestions
5. **Error handling**: Test with malformed content, API failures
6. **Cost control**: Mock Bedrock to avoid API costs in tests

**Deliverables**:
- [ ] `pkg/ai/provider.go` - AI provider interface
- [ ] `pkg/ai/bedrock/client.go` - AWS Bedrock implementation
- [ ] `pkg/ai/bedrock/summarize.go` - Summarization logic
- [ ] `pkg/ai/mock/provider.go` - Mock for testing
- [ ] `pkg/indexer/commands/summarize.go` - Summarize command
- [ ] Integration tests with mocked AI provider
- [ ] Configuration support for AI provider
- [ ] Documentation for AI features

### Phase 9: Vector Embeddings & Semantic Search (Week 11-13)

**Goal**: Implement vector embeddings and semantic search using Bedrock + Meilisearch

```go
// pkg/indexer/commands/embedding.go
type GenerateEmbeddingCommand struct {
    AIProvider   ai.Provider
    VectorDB     search.VectorIndex
    ChunkSize    int
    ChunkOverlap int
    Enabled      bool
}

func (c *GenerateEmbeddingCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    if !c.Enabled {
        return nil
    }
    
    // Skip if already has recent embeddings
    if doc.Embeddings != nil && 
       doc.Embeddings.GeneratedAt.After(time.Now().Add(-7 * 24 * time.Hour)) {
        return nil
    }
    
    // Prepare text for embedding
    texts := []string{doc.Content}
    
    // Generate embeddings via Bedrock
    req := &ai.EmbeddingRequest{
        Texts:        texts,
        ChunkSize:    c.ChunkSize,
        ChunkOverlap: c.ChunkOverlap,
    }
    
    resp, err := c.AIProvider.GenerateEmbedding(ctx, req)
    if err != nil {
        return fmt.Errorf("embedding generation failed: %w", err)
    }
    
    // Store DocumentEmbeddings in context (external structure)
    resp.Embeddings.DocumentID = doc.Document.ID
    doc.Embeddings = resp.Embeddings
    
    return nil
}

// pkg/indexer/commands/index_vector.go
type IndexVectorCommand struct {
    VectorDB search.VectorIndex
}

func (c *IndexVectorCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    // Skip if no embeddings
    if doc.Embeddings == nil || len(doc.Embeddings.ContentEmbedding) == 0 {
        return nil
    }
    
    // Prepare vector document from external structures
    vectorDoc := &search.VectorDocument{
        ObjectID:         doc.Document.ID,
        DocID:            doc.Metadata.GoogleFileID,
        Title:            doc.Document.Name,
        DocType:          doc.Metadata.DocType,
        ContentEmbedding: doc.Embeddings.ContentEmbedding,
        Model:            doc.Embeddings.Model,
        Dimensions:       doc.Embeddings.Dimensions,
        EmbeddedAt:       doc.Embeddings.GeneratedAt,
    }
    
    // Add AI summary if available
    if doc.AISummary != nil {
        vectorDoc.Summary = doc.AISummary.ExecutiveSummary
        vectorDoc.KeyPoints = doc.AISummary.KeyPoints
        vectorDoc.Topics = doc.AISummary.Topics
        vectorDoc.Tags = doc.AISummary.Tags
    }
    
    // Add chunk embeddings
    for _, chunk := range doc.Embeddings.Chunks {
        vectorDoc.ChunkEmbeddings = append(vectorDoc.ChunkEmbeddings, search.ChunkEmbedding{
            ChunkIndex: chunk.ChunkIndex,
            Text:       chunk.Text,
            Embedding:  chunk.Embedding,
            StartPos:   chunk.StartPos,
            EndPos:     chunk.EndPos,
        })
    }
    
    // Store prepared vector document in context for reuse
    doc.VectorDoc = vectorDoc
    
    // Index in vector database
    return c.VectorDB.IndexEmbedding(ctx, vectorDoc)
}
```

**Extended Search Provider**:
```go
// pkg/search/provider.go - Add to Provider interface
type Provider interface {
    // ... existing methods ...
    
    // VectorIndex returns the vector search interface (optional)
    VectorIndex() VectorIndex
}

// pkg/search/meilisearch/provider.go - Implement vector support
func (p *Provider) VectorIndex() search.VectorIndex {
    if p.vectorIndex == nil {
        p.vectorIndex, _ = NewVectorIndex(p.client, "documents-vectors")
    }
    return p.vectorIndex
}
```

**API Endpoints for Semantic Search**:
```go
// internal/api/v2/search.go - Add semantic search endpoint
func (s *SearchHandler) SemanticSearch(c *gin.Context) {
    var req struct {
        Query         string                 `json:"query"`
        Limit         int                    `json:"limit"`
        VectorWeight  float64                `json:"vectorWeight"`  // 0.0-1.0
        KeywordWeight float64                `json:"keywordWeight"` // 0.0-1.0
        Filters       map[string]interface{} `json:"filters"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // Generate query embedding
    embResp, err := s.aiProvider.GenerateEmbedding(c.Request.Context(), &ai.EmbeddingRequest{
        Texts: []string{req.Query},
    })
    if err != nil {
        c.JSON(500, gin.H{"error": "failed to generate query embedding"})
        return
    }
    
    // Perform hybrid search
    results, err := s.searchProvider.VectorIndex().SearchHybrid(c.Request.Context(), &search.HybridSearchQuery{
        QueryText:      req.Query,
        QueryEmbedding: embResp.Embeddings[0].Vector,
        VectorWeight:   req.VectorWeight,
        KeywordWeight:  req.KeywordWeight,
        Limit:          req.Limit,
        Filters:        req.Filters,
    })
    
    c.JSON(200, results)
}
```

**Test Scenarios**:
1. **Embedding generation**: Generate embeddings for documents
2. **Vector indexing**: Store embeddings in Meilisearch
3. **Similarity search**: Find similar documents by embedding
4. **Hybrid search**: Combine keyword + vector search
5. **Chunked embeddings**: Test with long documents
6. **Performance**: Benchmark embedding generation and search

**Deliverables**:
- [ ] `pkg/ai/bedrock/embedding.go` - Bedrock embedding implementation
- [ ] `pkg/indexer/commands/embedding.go` - Generate embedding command
- [ ] `pkg/indexer/commands/index_vector.go` - Index vector command
- [ ] `pkg/search/vector.go` - Vector search interface
- [ ] `pkg/search/meilisearch/vector.go` - Meilisearch vector implementation
- [ ] `internal/api/v2/search.go` - Semantic search API endpoints
- [ ] Database schema for storing embedding metadata
- [ ] Integration tests for vector search
- [ ] Performance benchmarks
- [ ] Documentation for semantic search

## Configuration Changes

### Indexer Plans (Declarative Configuration)

Instead of hardcoded pipelines, use declarative YAML plans that define the complete indexer execution:

```yaml
# testing/indexer/plans/local-integration-test.yaml
name: local-integration-test
description: Full integration test using local workspace provider

workspace_provider: local
search_provider: meilisearch

folders:
  - id: docs
    type: published
    pipeline: index-published
  
  - id: drafts
    type: drafts
    pipeline: index-drafts

pipelines:
  - name: index-published
    description: Index published documents
    commands:
      - type: discover
        config:
          folder_id: docs
          since_last_run: true
      - type: extract
        config:
          max_size: 85000
      - type: load-metadata
        config:
          source: database
      - type: transform
        config:
          format: search-document
      - type: index
        config:
          index_type: published
          batch_size: 10
      - type: track
        config:
          update_folder_timestamp: true

execution:
  run_interval: 10s
  max_parallel_docs: 3
  dry_run: false
```

### Production Plan Example (AI-Enhanced)

```yaml
# config/indexer/plans/production.yaml
name: production
description: Production indexer for Google Workspace with AI enhancements

workspace_provider: google
search_provider: meilisearch  # Supports both full-text and vector search
ai_provider: bedrock

folders:
  - id: "{{ .GoogleWorkspace.DocsFolder }}"
    type: published
    pipeline: ai-enhanced-index-published

pipelines:
  - name: ai-enhanced-index-published
    commands:
      - type: discover
      - type: assign-uuid
      - type: extract
      - type: hash
      - type: revision
      - type: load-metadata
      - type: transform
      - type: summarize
        config:
          extract_topics: true
          generate_tags: true
          analyze_status: true
          max_summary_length: 500
      - type: embedding
        config:
          chunk_size: 512
          chunk_overlap: 50
          enabled: true
      - type: index
        config:
          index_type: published
          batch_size: 10
      - type: index-vector
        config:
          enabled: true
      - type: update-header
        config:
          enabled: "{{ .Indexer.UpdateDocHeaders }}"
      - type: track
    filter:
      skip_recently_modified: 30m

execution:
  run_interval: 60s
  max_parallel_docs: 5
```

### Migration Plan Example

```yaml
# config/indexer/plans/migrate-google-to-local.yaml
name: migrate-google-to-local
description: Migrate documents from Google Workspace to local Git with conflict detection

source_workspace_provider: google
target_workspace_provider: local
search_provider: meilisearch
ai_provider: bedrock

folders:
  - source_id: "{{ .GoogleWorkspace.DocsFolder }}"
    target_id: "docs"
    type: published
    pipeline: migration-with-conflict-detection

pipelines:
  - name: migration-with-conflict-detection
    commands:
      - type: discover
        config:
          provider: source
      - type: assign-uuid
        config:
          provider: source
      - type: extract
        config:
          provider: source
      - type: hash
      - type: load-metadata
      - type: migrate
        config:
          skip_existing: false
          dry_run: false
      - type: revision
        config:
          status: target
      - type: detect-conflicts
      - type: summarize
        config:
          enabled: true
      - type: embedding
        config:
          enabled: true
      - type: index
        config:
          index_type: published
      - type: index-vector
      - type: track

execution:
  run_interval: 0  # Run once
  max_parallel_docs: 3
  on_conflict: alert

execution:
  run_interval: 60s
  max_parallel_docs: 5
```

### Updated `config.hcl` Structure

```hcl
// Workspace provider selection
workspace_provider = "google" // or "local"

// Local workspace configuration (when provider = "local")
local_workspace {
  base_path   = "/tmp/hermes-workspace"
  docs_path   = "/tmp/hermes-workspace/docs"
  drafts_path = "/tmp/hermes-workspace/drafts"
}

// AI Provider configuration (NEW)
ai_provider {
  type = "bedrock"  // or "openai", "mock"
  
  bedrock {
    region           = env("AWS_REGION")          // Default: "us-west-2"
    summarize_model  = "anthropic.claude-3-5-sonnet-20241022-v2:0"
    embedding_model  = "amazon.titan-embed-text-v2:0"
    
    // AWS credentials from environment variables or IAM role
    // Required: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
    // Optional: AWS_SESSION_TOKEN (for temporary credentials)
    
    // Rate limiting
    max_requests_per_second = 10
    max_concurrent_requests = 5
    
    // Cost controls
    max_tokens_per_request = 100000
    daily_token_limit      = 10000000  // 10M tokens/day
  }
}

// Indexer configuration
indexer {
  // Plan to use (optional, can be specified via CLI flag)
  plan = "config/indexer/plans/production.yaml"
  
  // AI features
  ai_enabled                = true
  ai_summarize_on_index     = true
  ai_generate_embeddings    = true
  ai_update_existing_summaries = false  // Only generate for new docs
  
  // Document revision tracking
  track_revisions           = true
  enable_uuid_assignment    = true
  detect_conflicts          = true
  conflict_notification_webhook = env("HERMES_CONFLICT_WEBHOOK_URL")
  
  // Legacy options (for backward compatibility)
  max_parallel_docs            = 5
  update_doc_headers           = true
  update_draft_headers         = true
  use_database_for_document_data = false
}
```

## Testing Strategy

### Unit Tests

Each command is independently testable:

```go
func TestExtractContentCommand(t *testing.T) {
    mockProvider := &mock.DocumentStorage{}
    mockProvider.On("GetDocumentContent", mock.Anything, "doc-123").
        Return("Document content here", nil)
    
    cmd := &commands.ExtractContentCommand{
        Provider: mockProvider,
        MaxSize:  100,
    }
    
    doc := &indexer.DocumentContext{
        Document: &workspace.Document{ID: "doc-123"},
    }
    
    err := cmd.Execute(context.Background(), doc)
    require.NoError(t, err)
    assert.Equal(t, "Document content here", doc.Content)
}
```

### Integration Tests (in `./testing`)

```bash
# Run local integration tests
cd testing
make indexer/test

# Or via root Makefile
make test/indexer/integration
```

**Test Scenarios**:
1. Index documents from local filesystem
2. Migrate documents between providers
3. Refresh headers on local documents
4. Concurrent document processing
5. Error recovery and retry logic
6. Database tracking verification

### E2E Tests (with Docker)

```yaml
# testing/docker-compose.yml (add to existing)
services:
  indexer-test:
    build:
      context: ..
      dockerfile: testing/Dockerfile.hermes
    command: >
      indexer
      -config=/config/config.hcl
      -provider=local
    volumes:
      - ./workspace_data:/workspace
      - ./config.hcl:/config/config.hcl
    depends_on:
      - postgres
      - meilisearch
    environment:
      HERMES_INDEXER_RUN_ONCE: "true"
```

## Migration Path

### Backward Compatibility

Phase 1-3: Old indexer continues to work alongside new implementation

```go
// internal/cmd/commands/indexer/indexer.go
func (c *Command) Run(args []string) int {
    if c.flagUseLegacy {
        return c.runLegacyIndexer()
    }
    return c.runNewIndexer()
}
```

### Deprecation Timeline

- **Weeks 1-6**: Implement new architecture in `pkg/indexer`
- **Week 7**: Feature flag to enable new indexer
- **Week 8**: Run both in parallel, compare results
- **Week 9**: Default to new indexer, keep legacy as fallback
- **Week 10**: Remove legacy indexer code from `internal/indexer`

## Benefits

### 1. Provider Agnostic

```go
// Same code works with any provider
orchestrator := indexer.NewOrchestrator(
    indexer.WithWorkspaceProvider(googleProvider), // or localProvider
    // ...
)
```

### 2. Composable Operations

```go
// Create custom pipelines
customPipeline := &indexer.Pipeline{
    name: "custom",
    commands: []indexer.Command{
        &commands.DiscoverCommand{},
        &commands.ValidateCommand{},
        &commands.TransformCommand{},
        &myCustomCommand{},
    },
}
```

### 3. Testable Locally

```bash
# Full integration test without Google Workspace
make test/indexer/integration

# Test specific pipeline
./hermes indexer -config=testing/config.hcl \
  -provider=local \
  -pipeline=index-published \
  -dry-run
```

### 4. Migration Support

```bash
# Migrate from Google to Local for testing
./hermes indexer -config=config.hcl -migrate \
  -source=google -target=local \
  -source-folder=1abc... -target-folder=/tmp/docs

# Migrate between local folders
./hermes indexer -config=config.hcl -migrate \
  -source=local -target=local \
  -source-folder=./old -target-folder=./new
```

### 5. Extensibility

Easy to add new commands:

```go
// Add custom validation command
type ValidateLinksCommand struct {
    httpClient *http.Client
}

func (c *ValidateLinksCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    // Extract links from doc.Content
    // Validate each link
    // Add results to doc.Errors
}

// Use in pipeline
pipeline.AddCommand(&ValidateLinksCommand{})
```

## Success Criteria

### Phase 1-3 (Foundation)
- [ ] All existing indexer functionality works with new architecture
- [ ] Unit tests for all commands (>80% coverage)
- [ ] Integration tests pass with mock providers

### Phase 4-5 (Local Testing)
- [ ] Can index documents from local filesystem
- [ ] Can migrate documents between providers
- [ ] Full integration tests run in `./testing` without external dependencies
- [ ] Documentation for local testing workflow

### Phase 6 (Production Ready)
- [ ] CLI supports new architecture
- [ ] Backward compatibility maintained
- [ ] Performance equals or exceeds legacy indexer
- [ ] Deployed and running in production

## Risks & Mitigation

### Risk 1: Workspace Provider Interface Gaps

**Risk**: `workspace.DocumentStorage` interface may not have all operations needed

**Mitigation**: 
- Audit current indexer for all workspace operations
- Extend interface as needed before Phase 2
- Add `GetUpdatedDocsBetween` equivalent to interface

### Risk 2: Performance Regression

**Risk**: Abstraction layers may slow down indexing

**Mitigation**:
- Benchmark each phase against legacy
- Optimize hot paths (content extraction, API calls)
- Maintain parallel processing capability

### Risk 3: Breaking Changes

**Risk**: Refactor may break existing deployments

**Mitigation**:
- Feature flag for new vs legacy
- Run both in parallel initially
- Comprehensive integration tests

### Risk 4: AI/Bedrock Costs

**Risk**: AI operations (summarization, embeddings) may incur significant AWS costs

**Mitigation**:
- Implement daily token limits and rate limiting
- Cache AI results (don't regenerate for unchanged documents)
- Make AI features optional with feature flags
- Monitor costs with AWS Cost Explorer
- Start with subset of documents for testing
- Use mock AI provider in development/testing

### Risk 5: Vector Search Performance

**Risk**: Vector search may be slow for large document collections

**Mitigation**:
- Use Meilisearch 1.11+ with native vector support
- Implement chunked embeddings for long documents
- Add caching layer for frequent queries
- Benchmark with realistic data volumes
- Consider dedicated vector database if Meilisearch insufficient

## Next Steps

### Immediate Actions (Week 1)

1. **Review & Approve**: Get team feedback on this updated plan
2. **Extend Workspace Interface**: Add missing operations (`GetUpdatedDocsBetween`, `UpdateMetadata` for UUIDs)
3. **Extend Search Provider Interface**: Add `VectorIndex()` method to `Provider` interface
4. **Create Feature Branch**: `feat/indexer-refactor-with-ai`
5. **Set up AWS Bedrock**: Configure IAM roles, test API access
6. **Database Schema**: Design `document_revisions` table and embedding storage

### Phase-by-Phase Rollout (Weeks 2-13)

**Phases 1-3 (Weeks 2-4)**: Core command pipeline without AI
- Get basic refactored indexer working
- Validate with local workspace provider
- Ensure backward compatibility

**Phases 4-6 (Weeks 5-8)**: Migration & revision tracking
- Implement UUID assignment and revision tracking
- Build migration commands and conflict detection
- Test Google → Local migration workflow

**Phases 7-8 (Weeks 9-11)**: AI summarization
- Integrate AWS Bedrock for summarization
- Test with mock provider first
- Roll out to subset of documents

**Phase 9 (Weeks 12-13)**: Vector embeddings & semantic search
- Generate embeddings with Bedrock Titan
- Implement Meilisearch vector index
- Add semantic search API endpoints
- Performance testing and optimization

### Success Metrics

**Phase 1-3 (Foundation)**:
- [ ] All existing indexer functionality works with new architecture
- [ ] Unit tests for all commands (>80% coverage)
- [ ] Integration tests pass with mock providers
- [ ] Performance within 10% of legacy indexer

**Phase 4-6 (Revisions & Migration)**:
- [ ] Can track document revisions across providers
- [ ] Can detect conflicts during migration
- [ ] Can migrate 1000+ documents without data loss
- [ ] Conflict detection accuracy >95%

**Phase 7-8 (AI Summarization)**:
- [ ] Successfully generate summaries for 90%+ of documents
- [ ] Summary quality validated by human review (sample of 50 docs)
- [ ] AI costs under $50/month for 10K documents
- [ ] Response time <5s per document

**Phase 9 (Vector Search)**:
- [ ] Generate embeddings for 100% of documents
- [ ] Semantic search returns relevant results (>80% relevance in blind tests)
- [ ] Hybrid search improves relevance over keyword-only by 20%+
- [ ] Search latency <500ms for 95th percentile

## API-Based Architecture

### Overview

The indexer operates as an **external client** that communicates with Hermes via REST API instead of direct database access. This architectural shift enables:

1. **Separation of concerns**: Indexer discovers/processes, API handles persistence
2. **External document sources**: Index from GitHub, local files, remote Hermes instances
3. **Project-based workspaces**: Use project config to resolve providers
4. **Service isolation**: Indexer can run independently, scale separately
5. **Authentication & authorization**: Leverage existing API security

### Architecture Flow

```
┌──────────────────────────────────────────────────────────────────────┐
│                         Indexer Service                               │
│  (Discovers documents, generates content, coordinates processing)    │
│                                                                       │
│  ┌────────────┐  ┌──────────────┐  ┌─────────────┐  ┌────────────┐ │
│  │  Discover  │→ │ Extract      │→ │ Calculate   │→ │ Summarize  │ │
│  │ (Project)  │  │ Content/Meta │  │ Hash        │  │ (AI)       │ │
│  └────────────┘  └──────────────┘  └─────────────┘  └────────────┘ │
│         │                                                      │      │
└─────────┼──────────────────────────────────────────────────────┼─────┘
          │                                                      │
          │ HTTP POST/PUT                                        │
          ▼                                                      ▼
┌──────────────────────────────────────────────────────────────────────┐
│                       Hermes API Server                               │
│                  POST /api/v2/indexer/documents                       │
│              POST /api/v2/indexer/documents/:uuid/revisions          │
│              PUT /api/v2/indexer/documents/:uuid/summary             │
│              PUT /api/v2/indexer/documents/:uuid/embeddings          │
│                                                                       │
│  ┌──────────────────┐  ┌───────────────┐  ┌────────────────┐       │
│  │ Authentication   │  │ Validation    │  │ Business Logic │       │
│  │ (Service Token)  │  │ (Schema)      │  │ (Dedup, etc.)  │       │
│  └──────────────────┘  └───────────────┘  └────────────────┘       │
└───────────────────────────────────┬───────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────┐
│                        Database (PostgreSQL)                          │
│  • documents (uuid, title, metadata, workspace_provider)             │
│  • document_revisions (content_hash, commit_sha, summary)            │
│  • document_embeddings (vectors for semantic search)                 │
└──────────────────────────────────────────────────────────────────────┘
```

### Key API Endpoints

See `INDEXER_IMPLEMENTATION_GUIDE.md` for complete API specification.

**Document Management**:
- `POST /api/v2/indexer/documents` - Create/upsert document reference
- `GET /api/v2/indexer/documents/:uuid` - Get document by UUID

**Revision Tracking**:
- `POST /api/v2/indexer/documents/:uuid/revisions` - Create new revision
- `PUT /api/v2/indexer/documents/:uuid/summary` - Update AI summary

**Vector Search**:
- `PUT /api/v2/indexer/documents/:uuid/embeddings` - Store embeddings

### Workspace Provider Resolution

The indexer uses **project configuration** to resolve workspace providers:

```hcl
# testing/projects/docs-internal.hcl
project "docs-internal" {
  short_name  = "DOCS"
  description = "Internal documentation and RFCs"
  status      = "active"
  
  workspace "local" {
    type = "local"
    root = "./docs-internal"
    
    folders {
      docs   = "."
      drafts = ".drafts"
    }
  }
  
  # Alternative: GitHub workspace
  # workspace "github" {
  #   type       = "github"
  #   repository = "hashicorp/hermes"
  #   branch     = "main"
  #   path       = "docs-internal"
  # }
}
```

**Discovery Flow**:
```go
// Load project config
cfg, err := projectconfig.LoadConfig("testing/projects.hcl")

// Get project by ID
project := cfg.GetProject("docs-internal")

// Resolve workspace provider
provider, err := workspace.NewProvider(project.Workspace)

// Discover documents
docs, err := provider.ListDocuments(ctx, project.Workspace.Folders.Docs, nil)

// For each document, POST to API
for _, doc := range docs {
    req := &CreateDocumentRequest{
        UUID:  doc.UUID,
        Title: doc.Title,
        WorkspaceProvider: WorkspaceProviderMetadata{
            Type:       "local",
            Path:       doc.Path,
            ProjectID:  "docs-internal",
        },
    }
    
    resp, err := apiClient.CreateDocument(ctx, req)
    // Handle response...
}
```

### Advantages Over Direct DB Access

| Aspect | Direct DB | API-Based |
|--------|-----------|-----------|
| **Coupling** | Tight coupling to schema | Loose coupling via contracts |
| **Testing** | Requires DB setup | Can mock API responses |
| **Security** | Full DB access | Scoped permissions via API |
| **Validation** | Manual | API enforces rules |
| **Audit** | Manual logging | Built-in API logs |
| **Deployment** | Must share DB | Can deploy independently |
| **Scaling** | Single process | Indexer can scale horizontally |
| **External Sources** | Difficult (schema assumptions) | Natural (API accepts metadata) |

### Migration Strategy

**Phase 1**: Keep existing direct DB commands, add API client parameter
```go
type TrackRevisionCommand struct {
    DB        *gorm.DB        // Legacy (deprecated)
    APIClient *IndexerAPIClient // New (preferred)
}

func (c *TrackRevisionCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    if c.APIClient != nil {
        // New: Use API
        return c.createRevisionViaAPI(ctx, doc)
    }
    // Legacy: Direct DB (for backward compatibility)
    return c.createRevisionDirectDB(ctx, doc)
}
```

**Phase 2**: Deprecate DB parameter, log warnings
```go
if c.DB != nil {
    log.Warn("Direct DB access is deprecated, use APIClient instead")
}
```

**Phase 3**: Remove DB parameter entirely
```go
type TrackRevisionCommand struct {
    APIClient *IndexerAPIClient // Required
}
```

### Authentication

**Service Token Approach** (recommended for production):
```bash
# Generate service token
./hermes admin create-service-token --name="indexer-service" --scopes="indexer:write"

# Use in indexer
export HERMES_INDEXER_TOKEN="svc_abc123..."
./hermes indexer -config=config.hcl
```

**OIDC Approach** (for testing with Dex):
```go
// Get auth token from Dex
token, err := auth.GetOIDCToken(ctx, dexURL, clientID, clientSecret)

// Create API client
apiClient := &IndexerAPIClient{
    BaseURL:   "http://localhost:8001",
    AuthToken: token,
}
```

### Project Config Integration

The indexer becomes **project-aware**:

```bash
# Index specific project
./hermes indexer -config=config.hcl -project=docs-internal

# Index all active projects
./hermes indexer -config=config.hcl -all-projects

# List projects
./hermes indexer -config=config.hcl -list-projects
```

**CLI Flow**:
1. Load config: `projectconfig.LoadConfig("projects.hcl")`
2. Select project: `cfg.GetProject(projectID)`
3. Resolve workspace: `workspace.NewProvider(project.Workspace)`
4. Create API client: `NewIndexerAPIClient(apiURL, token)`
5. Execute pipeline with API client

## Related Documentation

- [Document Revisions and Migration](./DOCUMENT_REVISIONS_AND_MIGRATION.md) - UUID-based revision tracking
- [Indexer README](./README-indexer.md) - Current indexer implementation
- [Indexer Implementation Guide](./INDEXER_IMPLEMENTATION_GUIDE.md) - **Complete API specification and examples**
- [Workspace Provider Architecture](../pkg/workspace/README.md) - Multi-provider abstraction
- [Search Provider Architecture](../pkg/search/README.md) - Search abstraction layer
- [Testing Infrastructure](../testing/README.md) - Local testing environment
- [Distributed Projects Architecture](./DISTRIBUTED_PROJECTS_ARCHITECTURE.md) - Multi-provider project management

## Appendix: AI Provider Comparison

| Provider | Summarization Model | Embedding Model | Pros | Cons |
|----------|-------------------|-----------------|------|------|
| **AWS Bedrock** | Claude 3.7 Sonnet | Titan Embed V2 | • Native AWS integration<br>• Pay-per-use<br>• High quality | • Requires AWS setup<br>• Regional availability |
| **OpenAI** | GPT-4 | text-embedding-3 | • Best-in-class quality<br>• Well-documented API | • Higher cost<br>• Rate limits<br>• External dependency |
| **Local (Ollama)** | Llama 3 | nomic-embed-text | • No API costs<br>• Data stays local<br>• No rate limits | • Requires GPU<br>• Lower quality<br>• Self-hosted complexity |

**Recommendation**: Start with **AWS Bedrock** for production (Claude 3.7 + Titan Embed V2), use **Mock** provider for development/testing.

## Appendix: Meilisearch Vector Search Setup

```bash
# Start Meilisearch 1.11+ with vector search support
docker run -d \
  --name meilisearch \
  -p 7700:7700 \
  -e MEILI_MASTER_KEY="your-master-key" \
  -v $(pwd)/meili_data:/meili_data \
  getmeili/meilisearch:v1.11
```

**Configuration**:
```json
{
  "filterableAttributes": ["docType", "status", "owners", "product"],
  "sortableAttributes": ["modifiedTime", "createdTime"],
  "searchableAttributes": ["title", "summary", "content"],
  "embedders": {
    "default": {
      "source": "userProvided",
      "dimensions": 1024
    }
  }
}
```

**Index with Vector**:
```bash
curl -X POST 'http://localhost:7700/indexes/documents/documents' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer your-master-key' \
  --data-binary '[
    {
      "id": "rfc-001",
      "title": "API Gateway Design",
      "summary": "Design for new API gateway...",
      "_vectors": {
        "default": [0.1, 0.2, ..., 0.9]
      }
    }
  ]'
```

**Hybrid Search**:
```bash
curl -X POST 'http://localhost:7700/indexes/documents/search' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer your-master-key' \
  --data-binary '{
    "q": "authentication",
    "vector": [0.1, 0.2, ..., 0.9],
    "hybrid": {
      "semanticRatio": 0.7
    },
    "limit": 10
  }'
```

---

**Author**: GitHub Copilot  
**Date**: October 22, 2025  
**Status**: 🚧 Design Phase - Ready for Review  
**Version**: 2.0.0 (Updated with Document Revisions, AI Summarization, Vector Embeddings)
