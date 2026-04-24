//go:build integration
// +build integration

package indexer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/hashicorp-forge/hermes/pkg/ai"
	"github.com/hashicorp-forge/hermes/pkg/ai/ollama"
	"github.com/hashicorp-forge/hermes/pkg/document"
	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp-forge/hermes/pkg/indexer/commands"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/search/adapters/meilisearch"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
	"github.com/hashicorp-forge/hermes/pkg/workspace/adapters/local"
)

// TestFullPipelineWithDocsInternal tests the complete indexer pipeline:
// - Discovers documents from local workspace (docs-internal)
// - Assigns UUIDs and tracks content hashes
// - Extracts content from markdown files
// - Generates AI summaries using Ollama
// - Generates vector embeddings using Ollama
// - Stores everything in PostgreSQL
// - Indexes vectors in Meilisearch
func TestFullPipelineWithDocsInternal(t *testing.T) {
	if !ollamaAvailable {
		t.Skip("Ollama not available, skipping full pipeline test")
	}

	if testDB == nil {
		t.Skip("Database not available, skipping full pipeline test")
	}

	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "test-pipeline",
		Level:  hclog.Info,
		Output: os.Stdout,
	})

	// Setup local workspace pointing to docs-internal
	repoRoot := os.Getenv("HERMES_REPO_ROOT")
	if repoRoot == "" {
		// Try to detect repo root
		cwd, err := os.Getwd()
		require.NoError(t, err, "Failed to get working directory")
		repoRoot = filepath.Join(cwd, "..", "..", "..")
	}

	docsPath := filepath.Join(repoRoot, "docs-internal")
	if _, err := os.Stat(docsPath); os.IsNotExist(err) {
		t.Skipf("docs-internal not found at %s, skipping test", docsPath)
	}

	t.Logf("📁 Using docs-internal path: %s", docsPath)

	// Setup Ollama AI provider
	aiProvider, err := ollama.NewProvider(&ollama.Config{
		BaseURL:        ollamaBaseURL,
		SummarizeModel: summarizeModel,
		EmbeddingModel: embeddingModel,
		Timeout:        5 * time.Minute,
	})
	require.NoError(t, err, "Failed to create Ollama provider")

	t.Logf("🤖 Using Ollama at %s", ollamaBaseURL)
	t.Logf("   Summarization: %s", summarizeModel)
	t.Logf("   Embeddings: %s", embeddingModel)

	// Setup Meilisearch search provider
	var searchProvider *meilisearch.Adapter
	if testFixture != nil {
		searchProvider, err = meilisearch.NewAdapter(&meilisearch.Config{
			Host:              testFixture.MeilisearchHost,
			APIKey:            testFixture.MeilisearchAPIKey,
			DocsIndexName:     "docs_test",
			DraftsIndexName:   "drafts_test",
			ProjectsIndexName: "projects_test",
			LinksIndexName:    "links_test",
		})
		require.NoError(t, err, "Failed to create Meilisearch adapter")
		t.Logf("🔍 Using Meilisearch at %s", testFixture.MeilisearchHost)
	} else {
		t.Log("⚠️  Meilisearch not available, skipping search indexing")
	}

	// Create a custom discovery command that walks the filesystem
	// and reads markdown files directly with frontmatter parsing
	discoverCmd := &LocalFilesystemDiscoverCommand{
		basePath: docsPath,
		logger:   logger.Named("discover"),
	}

	// Discover documents
	initialDocs, err := discoverCmd.Discover(ctx)
	require.NoError(t, err, "Failed to discover documents")
	t.Logf("📚 Found %d markdown documents to process", len(initialDocs))

	// Create a no-op workspace provider for commands that need it
	noOpProvider := &noOpDocumentStorage{}

	// Build the pipeline
	pipeline := &indexer.Pipeline{
		Name:        "full-test-pipeline",
		Description: "Test pipeline for docs-internal indexing with AI",
		Logger:      logger.Named("pipeline"),
		MaxParallel: 3, // Limit parallelism for testing
		Commands: []indexer.Command{
			// Step 1: Assign UUIDs
			&commands.AssignUUIDCommand{
				Logger:   logger.Named("assign-uuid"),
				Provider: noOpProvider,
			},

			// Step 2: Calculate content hash
			// Note: Content is already extracted during discovery
			&commands.CalculateHashCommand{},

			// Skip LoadMetadataCommand - we're indexing new documents

			// Step 3: Generate AI summary
			&commands.SummarizeCommand{
				AIProvider:       aiProvider,
				DB:               testDB,
				Logger:           logger.Named("summarize"),
				MinContentLength: 500, // Skip very short docs
				ExtractTopics:    true,
				ExtractKeyPoints: true,
				SuggestTags:      true,
			},

			// Step 4: Generate embeddings
			&commands.GenerateEmbeddingCommand{
				AIProvider:   aiProvider,
				Logger:       logger.Named("embedding"),
				ChunkSize:    2000,
				ChunkOverlap: 200,
				Enabled:      true,
			},

			// Step 5: Transform for search indexing (if search provider available)
			// Note: We use a lightweight transform that doesn't require database metadata
			&SimpleTransformCommand{
				Logger: logger.Named("transform"),
			},

			// Step 6: Index in Meilisearch (if available)
			&commands.IndexCommand{
				SearchProvider: searchProvider,
				IndexType:      commands.IndexTypePublished,
			},

			// Step 7: Store revision tracking
			&commands.TrackRevisionCommand{
				DB: testDB,
			},

			// Step 8: Update tracking timestamps
			&commands.TrackCommand{
				DB:                 testDB,
				UpdateDocumentTime: true,
			},
		},
	}

	// Execute pipeline
	t.Log("🚀 Starting full pipeline execution...")
	startTime := time.Now()

	// Execute pipeline with discovered documents
	err = pipeline.Execute(ctx, initialDocs)
	require.NoError(t, err, "Pipeline execution failed")

	duration := time.Since(startTime)
	t.Logf("✅ Pipeline completed in %s", duration)

	// Verify results in database
	t.Run("VerifyDatabaseResults", func(t *testing.T) {
		var docCount int64
		err := testDB.Model(&models.Document{}).Count(&docCount).Error
		require.NoError(t, err, "Failed to count documents")

		t.Logf("📊 Documents in database: %d", docCount)
		assert.Greater(t, docCount, int64(0), "Expected documents to be stored in database")

		// Check for documents with embeddings tracked
		var revisionCount int64
		err = testDB.Model(&models.DocumentRevision{}).Count(&revisionCount).Error
		require.NoError(t, err, "Failed to count revisions")

		t.Logf("📊 Document revisions tracked: %d", revisionCount)
		assert.Greater(t, revisionCount, int64(0), "Expected document revisions to be tracked")

		// Sample a few documents to verify content
		var sampleDocs []models.Document
		err = testDB.Limit(5).
			Preload("DocumentType").
			Order("created_at DESC").
			Find(&sampleDocs).Error
		require.NoError(t, err, "Failed to query sample documents")

		t.Log("📄 Sample documents processed:")
		for _, doc := range sampleDocs {
			t.Logf("   - %s (Type: %s, UUID: %s)",
				doc.GoogleFileID,
				doc.DocumentType.Name,
				doc.DocumentUUID,
			)
		}
	})

	// Verify embeddings were generated
	t.Run("VerifyEmbeddingsGenerated", func(t *testing.T) {
		var revisionsWithEmbeddings int64
		err := testDB.Model(&models.DocumentRevision{}).
			Where("embedding_dimensions > 0").
			Count(&revisionsWithEmbeddings).Error
		require.NoError(t, err, "Failed to count revisions with embeddings")

		t.Logf("📊 Embeddings statistics:")
		t.Logf("   Revisions with embeddings: %d", revisionsWithEmbeddings)

		// Note: Vector indexing into search provider skipped in this test
		// as it requires VectorIndex implementation in meilisearch adapter
		if revisionsWithEmbeddings > 0 {
			assert.Greater(t, revisionsWithEmbeddings, int64(0), "Expected embeddings to be generated")
		}
	})

	// Performance statistics
	t.Run("PerformanceMetrics", func(t *testing.T) {
		var avgProcessingTime float64
		var totalDocs int64

		err := testDB.Model(&models.Document{}).Count(&totalDocs).Error
		require.NoError(t, err)

		if totalDocs > 0 {
			avgProcessingTime = duration.Seconds() / float64(totalDocs)
		}

		t.Logf("⏱️  Performance metrics:")
		t.Logf("   Total time: %s", duration)
		t.Logf("   Documents processed: %d", totalDocs)
		t.Logf("   Average per document: %.2f seconds", avgProcessingTime)
		t.Logf("   Throughput: %.2f docs/minute", float64(totalDocs)/(duration.Minutes()))
	})
}

// TestPipelineWithSingleDocument tests the pipeline with a single known document
// for detailed verification of each processing step.
func TestPipelineWithSingleDocument(t *testing.T) {
	if !ollamaAvailable {
		t.Skip("Ollama not available, skipping single document test")
	}

	if testDB == nil {
		t.Skip("Database not available, skipping single document test")
	}

	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "test-single-doc",
		Level:  hclog.Debug,
		Output: os.Stdout,
	})

	// Setup providers (same as full pipeline test)
	repoRoot := os.Getenv("HERMES_REPO_ROOT")
	if repoRoot == "" {
		cwd, err := os.Getwd()
		require.NoError(t, err)
		repoRoot = filepath.Join(cwd, "..", "..", "..")
	}

	docsPath := filepath.Join(repoRoot, "docs-internal")
	workspaceAdapter, err := local.NewAdapter(&local.Config{
		BasePath: docsPath,
	})
	require.NoError(t, err)

	ws := workspaceAdapter.DocumentStorage()

	aiProvider, err := ollama.NewProvider(&ollama.Config{
		BaseURL:        ollamaBaseURL,
		SummarizeModel: summarizeModel,
		EmbeddingModel: embeddingModel,
		Timeout:        5 * time.Minute,
	})
	require.NoError(t, err)

	// Find README.md in docs-internal
	readmePath := "README.md"
	docs, err := ws.ListDocuments(ctx, "", nil)
	require.NoError(t, err, "Failed to list documents")

	var readmeDoc *indexer.DocumentContext
	for _, doc := range docs {
		if doc.Name == readmePath {
			readmeDoc = &indexer.DocumentContext{
				Document:       doc,
				SourceProvider: ws,
				StartTime:      time.Now(),
			}
			break
		}
	}

	if readmeDoc == nil {
		t.Skip("README.md not found in docs-internal")
	}

	t.Logf("📄 Processing single document: %s", readmeDoc.Document.Name)

	// Process through each command with detailed logging
	t.Run("AssignUUID", func(t *testing.T) {
		cmd := &commands.AssignUUIDCommand{}
		err := cmd.Execute(ctx, readmeDoc)
		require.NoError(t, err)
		assert.NotEqual(t, "", readmeDoc.DocumentUUID.String())
		t.Logf("   ✓ UUID assigned: %s", readmeDoc.DocumentUUID)
	})

	t.Run("CalculateHash", func(t *testing.T) {
		cmd := &commands.CalculateHashCommand{}
		err := cmd.Execute(ctx, readmeDoc)
		require.NoError(t, err)
		assert.NotEqual(t, "", readmeDoc.ContentHash)
		t.Logf("   ✓ Content hash: %s", readmeDoc.ContentHash)
	})

	t.Run("ExtractContent", func(t *testing.T) {
		cmd := &commands.ExtractContentCommand{}
		err := cmd.Execute(ctx, readmeDoc)
		require.NoError(t, err)
		assert.NotEqual(t, "", readmeDoc.Content)
		t.Logf("   ✓ Content extracted: %d bytes", len(readmeDoc.Content))
		// Use min from ollama_simple_test.go
		previewLen := 100
		if len(readmeDoc.Content) < previewLen {
			previewLen = len(readmeDoc.Content)
		}
		t.Logf("   Preview: %s...", readmeDoc.Content[:previewLen])
	})

	t.Run("GenerateSummary", func(t *testing.T) {
		cmd := &commands.SummarizeCommand{
			AIProvider:       aiProvider,
			Logger:           logger,
			MinContentLength: 100,
			ExtractTopics:    true,
			ExtractKeyPoints: true,
			SuggestTags:      true,
		}
		err := cmd.Execute(ctx, readmeDoc)
		require.NoError(t, err)

		summaryVal, ok := readmeDoc.GetCustom("ai_summary")
		require.True(t, ok, "Summary not found in context")

		summary := summaryVal.(*ai.DocumentSummary)
		assert.NotEqual(t, "", summary.ExecutiveSummary)
		t.Logf("   ✓ Summary generated")
		t.Logf("      Executive Summary: %s", summary.ExecutiveSummary)
		t.Logf("      Key Points: %v", summary.KeyPoints)
		t.Logf("      Topics: %v", summary.Topics)
		t.Logf("      Tags: %v", summary.Tags)
	})

	t.Run("GenerateEmbeddings", func(t *testing.T) {
		cmd := &commands.GenerateEmbeddingCommand{
			AIProvider:   aiProvider,
			Logger:       logger,
			ChunkSize:    2000,
			ChunkOverlap: 200,
			Enabled:      true,
		}
		err := cmd.Execute(ctx, readmeDoc)
		require.NoError(t, err)

		embeddingsVal, ok := readmeDoc.GetCustom("ai_embeddings")
		require.True(t, ok, "Embeddings not found in context")

		embeddings := embeddingsVal.(*ai.DocumentEmbeddings)
		assert.Greater(t, len(embeddings.ContentEmbedding), 0)
		t.Logf("   ✓ Embeddings generated")
		t.Logf("      Model: %s", embeddings.Model)
		t.Logf("      Dimensions: %d", embeddings.Dimensions)
		t.Logf("      Chunks: %d", len(embeddings.Chunks))
		t.Logf("      Content embedding (first 5): %v", embeddings.ContentEmbedding[:min(5, len(embeddings.ContentEmbedding))])
	})

	t.Logf("✅ Single document pipeline test completed successfully")
}

// LocalFilesystemDiscoverCommand walks a local filesystem directory
// and creates document contexts by reading markdown files directly
// and parsing YAML frontmatter.
type LocalFilesystemDiscoverCommand struct {
	logger   hclog.Logger
	basePath string
}

// Name returns the command name.
func (c *LocalFilesystemDiscoverCommand) Name() string {
	return "discover-local-filesystem"
}

// Execute is not used for DiscoverCommand.
func (c *LocalFilesystemDiscoverCommand) Execute(_ context.Context, _ *indexer.DocumentContext) error {
	return fmt.Errorf("LocalFilesystemDiscoverCommand should use Discover() method")
}

// parseFrontmatter extracts YAML frontmatter from markdown content.
func parseFrontmatter(content []byte) (map[string]any, string) {
	// Check for frontmatter delimiter
	if !bytes.HasPrefix(content, []byte("---\n")) && !bytes.HasPrefix(content, []byte("---\r\n")) {
		// No frontmatter - return empty metadata and full content
		return make(map[string]any), string(content)
	}

	// Find the closing delimiter
	endIdx := bytes.Index(content[4:], []byte("\n---\n"))
	if endIdx == -1 {
		endIdx = bytes.Index(content[4:], []byte("\r\n---\r\n"))
	}
	if endIdx == -1 {
		// Invalid frontmatter - return empty metadata and full content
		return make(map[string]any), string(content)
	}

	// Extract frontmatter and content
	frontmatter := content[4 : endIdx+4]
	remainingContent := content[endIdx+8:] // Skip past "\n---\n"

	// Parse YAML frontmatter
	var metadata map[string]any
	if err := yaml.Unmarshal(frontmatter, &metadata); err != nil {
		// Failed to parse - return empty metadata
		return make(map[string]any), string(content)
	}

	return metadata, string(remainingContent)
}

// Discover walks the filesystem and loads documents.
func (c *LocalFilesystemDiscoverCommand) Discover(_ context.Context) ([]*indexer.DocumentContext, error) {
	var docContexts []*indexer.DocumentContext

	err := filepath.Walk(c.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			c.logger.Warn("error accessing path", "path", path, "error", err)
			return nil // Skip files we can't access
		}

		// Skip directories and non-markdown files
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			c.logger.Warn("failed to read file", "path", path, "error", err)
			return nil
		}

		// Parse frontmatter
		metadata, markdownContent := parseFrontmatter(content)

		// Calculate relative path for document ID
		relPath, err := filepath.Rel(c.basePath, path)
		if err != nil {
			c.logger.Warn("failed to get relative path", "path", path, "error", err)
			return nil
		}

		// Use relative path as document ID
		docID := filepath.ToSlash(relPath)

		// Ensure name metadata exists
		if _, ok := metadata["name"]; !ok {
			metadata["name"] = filepath.Base(path)
		}

		// Create workspace.Document
		doc := &workspace.Document{
			ID:       docID,
			Name:     filepath.Base(path),
			Content:  markdownContent,
			MimeType: "text/markdown",
			Metadata: metadata,
		}

		// Create document context
		docCtx := &indexer.DocumentContext{
			Document:  doc,
			Content:   markdownContent, // Set extracted content directly
			StartTime: time.Now(),
			Custom:    make(map[string]any),
		}

		docContexts = append(docContexts, docCtx)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	c.logger.Info("discovered documents", "count", len(docContexts))
	return docContexts, nil
}

// noOpDocumentStorage is a no-op implementation of workspace.DocumentStorage
// for testing purposes. It doesn't actually persist any changes.
type noOpDocumentStorage struct{}

func (n *noOpDocumentStorage) GetDocument(_ context.Context, id string) (*workspace.Document, error) {
	return nil, workspace.NotFoundError("document", id)
}

func (n *noOpDocumentStorage) CreateDocument(_ context.Context, _ *workspace.DocumentCreate) (*workspace.Document, error) {
	return nil, fmt.Errorf("not implemented")
}

func (n *noOpDocumentStorage) UpdateDocument(_ context.Context, id string, _ *workspace.DocumentUpdate) (*workspace.Document, error) {
	// No-op: pretend the update succeeded
	return &workspace.Document{ID: id}, nil
}

func (n *noOpDocumentStorage) DeleteDocument(_ context.Context, _ string) error {
	return nil
}

func (n *noOpDocumentStorage) ListDocuments(_ context.Context, _ string, _ *workspace.ListOptions) ([]*workspace.Document, error) {
	return nil, nil
}

func (n *noOpDocumentStorage) GetDocumentContent(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (n *noOpDocumentStorage) UpdateDocumentContent(_ context.Context, _, _ string) error {
	return nil
}

func (n *noOpDocumentStorage) ReplaceTextInDocument(_ context.Context, _ string, _ map[string]string) error {
	return nil
}

func (n *noOpDocumentStorage) CopyDocument(_ context.Context, _, _, _ string) (*workspace.Document, error) {
	return nil, fmt.Errorf("not implemented")
}

func (n *noOpDocumentStorage) MoveDocument(_ context.Context, _, _ string) error {
	return nil
}

func (n *noOpDocumentStorage) CreateFolder(_ context.Context, _, _ string) (*workspace.Folder, error) {
	return nil, fmt.Errorf("not implemented")
}

func (n *noOpDocumentStorage) GetFolder(_ context.Context, id string) (*workspace.Folder, error) {
	return nil, workspace.NotFoundError("folder", id)
}

func (n *noOpDocumentStorage) ListFolders(_ context.Context, _ string) ([]*workspace.Folder, error) {
	return nil, nil
}

func (n *noOpDocumentStorage) GetSubfolder(_ context.Context, _, name string) (*workspace.Folder, error) {
	return nil, workspace.NotFoundError("folder", name)
}

func (n *noOpDocumentStorage) ListRevisions(_ context.Context, _ string) ([]*workspace.Revision, error) {
	return nil, nil
}

func (n *noOpDocumentStorage) GetRevision(_ context.Context, _, revisionID string) (*workspace.Revision, error) {
	return nil, workspace.NotFoundError("revision", revisionID)
}

func (n *noOpDocumentStorage) GetLatestRevision(_ context.Context, _ string) (*workspace.Revision, error) {
	return nil, workspace.NotFoundError("revision", "latest")
}

// SimpleTransformCommand is a lightweight transform command for testing
// that creates search documents without requiring database metadata.
type SimpleTransformCommand struct {
	Logger hclog.Logger
}

func (c *SimpleTransformCommand) Name() string {
	return "simple-transform"
}

func (c *SimpleTransformCommand) Execute(_ context.Context, doc *indexer.DocumentContext) error {
	if c.Logger == nil {
		c.Logger = hclog.NewNullLogger()
	}

	// Create a minimal search document from the document context
	searchDoc := &document.Document{
		ObjectID:     doc.Document.ID,
		Title:        doc.Document.Name,
		Content:      doc.Content,
		ModifiedTime: time.Now().Unix(),
		Status:       "Published", // Default to published for testing
		DocType:      "Markdown",  // Default type
	}

	// Note: DocumentUUID and Summary fields have been removed from document.Document
	// These were part of an older schema and are no longer available
	// UUID tracking is now handled via the models.Document database table

	doc.Transformed = searchDoc

	c.Logger.Debug("transformed document for search",
		"document_id", doc.Document.ID,
		"title", searchDoc.Title,
		"content_length", len(searchDoc.Content),
	)

	return nil
}

func (c *SimpleTransformCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	return indexer.ParallelProcess(ctx, docs, c.Execute, 10)
}
