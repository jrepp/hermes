//go:build integration
// +build integration

package indexer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/pkg/ai"
	"github.com/hashicorp-forge/hermes/pkg/ai/ollama"
)

// TestOllamaProvider_Simple tests the Ollama provider without database or containers
func TestOllamaProvider_Simple(t *testing.T) {
	// Check if Ollama is available
	if !checkOllamaAvailable("http://localhost:11434") {
		t.Skip("Ollama not available at http://localhost:11434")
	}

	// Create Ollama provider
	ollamaCfg := &ollama.Config{
		BaseURL:        "http://localhost:11434",
		SummarizeModel: "llama3.2",
		EmbeddingModel: "nomic-embed-text",
		Timeout:        5 * time.Minute,
	}
	aiProvider, err := ollama.NewProvider(ollamaCfg)
	require.NoError(t, err, "failed to create Ollama provider")

	t.Run("Summarization", func(t *testing.T) {
		// Build summarization request
		req := &ai.SummarizeRequest{
			Title:            "Test Document",
			DocType:          "RFC",
			Content:          sampleRFCContent,
			ExtractKeyPoints: true,
			ExtractTopics:    true,
			SuggestTags:      true,
			AnalyzeStatus:    true,
		}

		// Generate summary
		ctx := context.Background()
		resp, err := aiProvider.Summarize(ctx, req)
		require.NoError(t, err, "summarization failed")
		require.NotNil(t, resp, "response is nil")
		require.NotNil(t, resp.Summary, "summary is nil")

		// Validate summary fields
		summary := resp.Summary
		assert.NotEmpty(t, summary.ExecutiveSummary, "executive summary should not be empty")
		assert.Greater(t, len(summary.KeyPoints), 0, "should have key points")
		assert.Greater(t, len(summary.Topics), 0, "should have topics")
		assert.Greater(t, len(summary.Tags), 0, "should have tags")
		// Note: SuggestedStatus and Confidence are optional fields that may not always be populated
		// by the AI model, so we log them but don't assert on them
		if summary.SuggestedStatus == "" {
			t.Logf("  Note: SuggestedStatus not populated by AI model")
		}
		if summary.Confidence == 0.0 {
			t.Logf("  Note: Confidence not populated by AI model")
		}

		t.Logf("✓ Summary generated successfully")
		t.Logf("  Model: %s", resp.Model)
		t.Logf("  Tokens: %d", resp.TokensUsed)
		t.Logf("  Executive Summary: %s", summary.ExecutiveSummary[:min(100, len(summary.ExecutiveSummary))])
		t.Logf("  Key Points (%d): %v", len(summary.KeyPoints), summary.KeyPoints)
		t.Logf("  Topics (%d): %v", len(summary.Topics), summary.Topics)
		t.Logf("  Tags (%d): %v", len(summary.Tags), summary.Tags)
	})

	t.Run("Embedding", func(t *testing.T) {
		// Build embedding request
		req := &ai.EmbeddingRequest{
			Texts: []string{"This is a test document about AI and document processing."},
		}

		// Generate embedding
		ctx := context.Background()
		resp, err := aiProvider.GenerateEmbedding(ctx, req)
		require.NoError(t, err, "embedding generation failed")
		require.NotNil(t, resp, "response is nil")
		require.NotNil(t, resp.Embeddings, "embeddings is nil")

		// Validate embedding
		embeddings := resp.Embeddings
		assert.Equal(t, 768, resp.Dimensions, "nomic-embed-text should have 768 dimensions")
		assert.NotNil(t, embeddings.ContentEmbedding, "content embedding should exist")
		assert.Equal(t, 768, len(embeddings.ContentEmbedding), "content embedding should have 768 dimensions")

		t.Logf("✓ Embedding generated successfully")
		t.Logf("  Model: %s", resp.Model)
		t.Logf("  Dimensions: %d", resp.Dimensions)
		t.Logf("  First 5 values: %v", embeddings.ContentEmbedding[:5])
	})

	t.Run("ChunkedEmbedding", func(t *testing.T) {
		// Build embedding request with chunking
		req := &ai.EmbeddingRequest{
			Texts:        []string{sampleRFCContent},
			ChunkSize:    200, // Words per chunk
			ChunkOverlap: 50,  // Words of overlap
		}

		// Generate chunked embeddings
		ctx := context.Background()
		resp, err := aiProvider.GenerateEmbedding(ctx, req)
		require.NoError(t, err, "embedding generation failed")
		require.NotNil(t, resp.Embeddings, "embeddings is nil")

		// Validate chunking
		embeddings := resp.Embeddings
		assert.Greater(t, len(embeddings.Chunks), 0, "should have chunks")
		assert.NotNil(t, embeddings.ContentEmbedding, "should have content embedding")

		t.Logf("✓ Generated %d chunks for document", len(embeddings.Chunks))
		for i, chunk := range embeddings.Chunks {
			t.Logf("  Chunk %d: %d-%d chars", i, chunk.StartPos, chunk.EndPos)
		}
	})
}

// Sample content for testing
const sampleRFCContent = `
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
`

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
