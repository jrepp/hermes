package steps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

// EmbeddingsStep generates vector embeddings for document revisions.
// Embeddings are stored in the document_embeddings table and enable semantic search.
type EmbeddingsStep struct {
	db                *gorm.DB
	embeddingsClient  EmbeddingsClient
	workspaceProvider WorkspaceContentProvider
	logger            hclog.Logger
}

// EmbeddingsClient is the interface for embeddings API clients.
type EmbeddingsClient interface {
	// GenerateEmbeddings generates embeddings for the given text.
	GenerateEmbeddings(ctx context.Context, text string, model string, dimensions int) ([]float64, error)

	// GenerateEmbeddingsBatch generates embeddings for multiple texts.
	GenerateEmbeddingsBatch(ctx context.Context, texts []string, model string, dimensions int) ([][]float64, error)
}

// EmbeddingsOptions holds options for embedding generation.
type EmbeddingsOptions struct {
	Model      string // e.g., "text-embedding-3-small", "text-embedding-3-large"
	Dimensions int    // Vector dimensions (e.g., 1536, 3072)
	ChunkSize  int    // Characters per chunk (0 = no chunking)
	Provider   string // "openai", "bedrock", etc.
}

// NewEmbeddingsStep creates a new embeddings step.
func NewEmbeddingsStep(db *gorm.DB, embeddingsClient EmbeddingsClient, workspaceProvider WorkspaceContentProvider, logger hclog.Logger) *EmbeddingsStep {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	return &EmbeddingsStep{
		db:                db,
		embeddingsClient:  embeddingsClient,
		workspaceProvider: workspaceProvider,
		logger:            logger.Named("embeddings-step"),
	}
}

// Name returns the step name.
func (s *EmbeddingsStep) Name() string {
	return "embeddings"
}

// Execute generates embeddings for the given revision.
func (s *EmbeddingsStep) Execute(ctx context.Context, revision *models.DocumentRevision, config map[string]interface{}) error {
	s.logger.Debug("executing embeddings step",
		"document_uuid", revision.DocumentUUID,
		"revision_id", revision.ID,
		"content_hash", revision.ContentHash,
	)

	// Parse configuration
	opts := s.parseOptions(config)

	// Check if embeddings already exist for this content hash
	existing, err := models.GetEmbeddingByDocumentIDAndModel(s.db, revision.DocumentID, opts.Model, nil)
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check for existing embeddings: %w", err)
	}

	if existing != nil && existing.MatchesContentHash(revision.ContentHash) {
		s.logger.Debug("embeddings already exist for this content hash, skipping",
			"document_uuid", revision.DocumentUUID,
			"content_hash", revision.ContentHash,
		)
		return nil
	}

	// Fetch document content
	content, err := s.fetchDocumentContent(revision)
	if err != nil {
		return fmt.Errorf("failed to fetch document content: %w", err)
	}

	// Check content length
	if len(content) == 0 {
		s.logger.Warn("document has no content, skipping embeddings",
			"document_uuid", revision.DocumentUUID,
		)
		return nil
	}

	// Clean content
	content = s.cleanContent(content)

	// Determine if chunking is needed
	if opts.ChunkSize > 0 && len(content) > opts.ChunkSize {
		return s.generateChunkedEmbeddings(ctx, revision, content, opts)
	}

	// Generate embeddings for whole document
	return s.generateSingleEmbedding(ctx, revision, content, opts)
}

// generateSingleEmbedding generates embeddings for the entire document.
func (s *EmbeddingsStep) generateSingleEmbedding(ctx context.Context, revision *models.DocumentRevision, content string, opts EmbeddingsOptions) error {
	startTime := time.Now()

	// Generate embeddings
	embedding, err := s.embeddingsClient.GenerateEmbeddings(ctx, content, opts.Model, opts.Dimensions)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	generationTime := int(time.Since(startTime).Milliseconds())

	// Create embedding record
	revisionID := int(revision.ID)
	docEmbedding := &models.DocumentEmbedding{
		DocumentID:       revision.DocumentID,
		DocumentUUID:     &revision.DocumentUUID,
		RevisionID:       &revisionID,
		Embedding:        embedding,
		Dimensions:       len(embedding),
		Model:            opts.Model,
		Provider:         opts.Provider,
		GenerationTimeMs: &generationTime,
		ContentHash:      revision.ContentHash,
		ContentLength:    intPtr(len(content)),
		GeneratedAt:      time.Now(),
	}

	// Save to database
	if err := s.db.Create(docEmbedding).Error; err != nil {
		return fmt.Errorf("failed to save embeddings: %w", err)
	}

	s.logger.Info("generated embeddings for document",
		"document_uuid", revision.DocumentUUID,
		"model", opts.Model,
		"dimensions", len(embedding),
		"generation_time_ms", generationTime,
	)

	return nil
}

// generateChunkedEmbeddings generates embeddings for a document split into chunks.
func (s *EmbeddingsStep) generateChunkedEmbeddings(ctx context.Context, revision *models.DocumentRevision, content string, opts EmbeddingsOptions) error {
	// Split content into chunks
	chunks := s.chunkContent(content, opts.ChunkSize)

	s.logger.Debug("generating embeddings for chunks",
		"document_uuid", revision.DocumentUUID,
		"num_chunks", len(chunks),
		"chunk_size", opts.ChunkSize,
	)

	startTime := time.Now()

	// Generate embeddings for all chunks in batch
	embeddings, err := s.embeddingsClient.GenerateEmbeddingsBatch(ctx, chunks, opts.Model, opts.Dimensions)
	if err != nil {
		return fmt.Errorf("failed to generate batch embeddings: %w", err)
	}

	generationTime := int(time.Since(startTime).Milliseconds())

	// Save each chunk's embedding
	revisionID := int(revision.ID)
	for i, embedding := range embeddings {
		chunkIndex := i
		docEmbedding := &models.DocumentEmbedding{
			DocumentID:       revision.DocumentID,
			DocumentUUID:     &revision.DocumentUUID,
			RevisionID:       &revisionID,
			Embedding:        embedding,
			Dimensions:       len(embedding),
			Model:            opts.Model,
			Provider:         opts.Provider,
			GenerationTimeMs: &generationTime,
			ContentHash:      revision.ContentHash,
			ContentLength:    intPtr(len(chunks[i])),
			ChunkIndex:       &chunkIndex,
			ChunkText:        chunks[i],
			GeneratedAt:      time.Now(),
		}

		if err := s.db.Create(docEmbedding).Error; err != nil {
			return fmt.Errorf("failed to save chunk %d embeddings: %w", i, err)
		}
	}

	s.logger.Info("generated chunked embeddings for document",
		"document_uuid", revision.DocumentUUID,
		"model", opts.Model,
		"num_chunks", len(chunks),
		"dimensions", opts.Dimensions,
		"total_generation_time_ms", generationTime,
	)

	return nil
}

// fetchDocumentContent fetches the document content from the workspace provider.
func (s *EmbeddingsStep) fetchDocumentContent(revision *models.DocumentRevision) (string, error) {
	// Use workspace provider to fetch content
	content, err := s.workspaceProvider.GetDocumentContent(revision.DocumentID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch content: %w", err)
	}

	return content, nil
}

// cleanContent normalizes and cleans document content.
func (s *EmbeddingsStep) cleanContent(content string) string {
	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Trim whitespace
	content = strings.TrimSpace(content)

	return content
}

// chunkContent splits content into chunks of approximately the specified size.
func (s *EmbeddingsStep) chunkContent(content string, chunkSize int) []string {
	var chunks []string

	// Split by paragraphs first to maintain context
	paragraphs := strings.Split(content, "\n\n")

	currentChunk := ""
	for _, para := range paragraphs {
		// If paragraph itself is too long, force split it
		if len(para) > chunkSize {
			// Save current chunk if exists
			if len(currentChunk) > 0 {
				chunks = append(chunks, strings.TrimSpace(currentChunk))
				currentChunk = ""
			}

			// Force split the long paragraph
			for i := 0; i < len(para); i += chunkSize {
				end := i + chunkSize
				if end > len(para) {
					end = len(para)
				}
				chunks = append(chunks, para[i:end])
			}
			continue
		}

		// If adding this paragraph would exceed chunk size
		if len(currentChunk)+len(para)+2 > chunkSize && len(currentChunk) > 0 {
			// Save current chunk and start new one
			chunks = append(chunks, strings.TrimSpace(currentChunk))
			currentChunk = para
		} else {
			// Add to current chunk
			if len(currentChunk) > 0 {
				currentChunk += "\n\n" + para
			} else {
				currentChunk = para
			}
		}
	}

	// Add final chunk
	if len(currentChunk) > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk))
	}

	return chunks
}

// parseOptions extracts embeddings options from config map.
func (s *EmbeddingsStep) parseOptions(config map[string]interface{}) EmbeddingsOptions {
	opts := EmbeddingsOptions{
		Model:      "text-embedding-3-small", // Default
		Dimensions: 1536,                     // Default for text-embedding-3-small
		ChunkSize:  0,                        // No chunking by default
		Provider:   "openai",                 // Default
	}

	if model, ok := config["model"].(string); ok {
		opts.Model = model
	}

	if dimensions, ok := config["dimensions"].(int); ok {
		opts.Dimensions = dimensions
	} else if dimensions, ok := config["dimensions"].(float64); ok {
		opts.Dimensions = int(dimensions)
	}

	if chunkSize, ok := config["chunk_size"].(int); ok {
		opts.ChunkSize = chunkSize
	} else if chunkSize, ok := config["chunk_size"].(float64); ok {
		opts.ChunkSize = int(chunkSize)
	}

	if provider, ok := config["provider"].(string); ok {
		opts.Provider = provider
	}

	return opts
}

// intPtr returns a pointer to an int.
func intPtr(i int) *int {
	return &i
}
