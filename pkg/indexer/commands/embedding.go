package commands

import (
	"context"
	"fmt"

	"github.com/hashicorp-forge/hermes/pkg/ai"
	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp/go-hclog"
)

// GenerateEmbeddingCommand generates vector embeddings for document content.
// These embeddings enable semantic search and similarity-based document discovery.
type GenerateEmbeddingCommand struct {
	AIProvider   ai.Provider
	Logger       hclog.Logger
	ChunkSize    int  // Maximum tokens per chunk (0 = no chunking)
	ChunkOverlap int  // Overlap between chunks in tokens
	Enabled      bool // Whether embedding generation is enabled
}

// Name returns the command name.
func (c *GenerateEmbeddingCommand) Name() string {
	return "generate-embedding"
}

// Execute generates embeddings for a document.
func (c *GenerateEmbeddingCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
	if c.Logger == nil {
		c.Logger = hclog.NewNullLogger()
	}

	if !c.Enabled {
		c.Logger.Debug("embedding generation disabled",
			"document_id", doc.Document.ID,
		)
		return nil
	}

	if c.AIProvider == nil {
		return fmt.Errorf("AI provider is required for embedding generation")
	}

	// Skip if no content
	if doc.Content == "" {
		c.Logger.Debug("skipping embedding generation: no content",
			"document_id", doc.Document.ID,
		)
		return nil
	}

	c.Logger.Info("generating embeddings",
		"document_id", doc.Document.ID,
		"title", doc.Document.Name,
		"content_length", len(doc.Content),
		"chunk_size", c.ChunkSize,
		"provider", c.AIProvider.Name(),
	)

	// Build embedding request
	req := &ai.EmbeddingRequest{
		Texts:        []string{doc.Content},
		ChunkSize:    c.ChunkSize,
		ChunkOverlap: c.ChunkOverlap,
	}

	// Generate embeddings
	resp, err := c.AIProvider.GenerateEmbedding(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Set document ID on embeddings
	resp.Embeddings.DocumentID = doc.Document.ID

	c.Logger.Info("embeddings generated",
		"document_id", doc.Document.ID,
		"model", resp.Model,
		"dimensions", resp.Dimensions,
		"chunks", len(resp.Embeddings.Chunks),
		"tokens_used", resp.TokensUsed,
	)

	// Store embeddings in context for downstream commands
	doc.SetCustom("ai_embeddings", resp.Embeddings)
	doc.SetCustom("embedding_model", resp.Model)
	doc.SetCustom("embedding_dimensions", resp.Dimensions)

	return nil
}

// ExecuteBatch implements BatchCommand for parallel embedding generation.
func (c *GenerateEmbeddingCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	// AI operations are expensive, limit concurrency
	return indexer.ParallelProcess(ctx, docs, c.Execute, 2)
}
