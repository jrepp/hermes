// Package ai provides interfaces and types for AI-powered document analysis.
// This includes summarization, key point extraction, and vector embedding generation.
package ai

import (
	"context"
	"time"
)

// Provider defines the interface for AI operations on documents.
type Provider interface {
	// Summarize generates a summary and extracts key information from a document.
	// Returns DocumentSummary as an external, reusable structure.
	Summarize(ctx context.Context, req *SummarizeRequest) (*SummarizeResponse, error)

	// GenerateEmbedding creates vector embeddings for text.
	// Returns DocumentEmbeddings as an external, reusable structure.
	GenerateEmbedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)

	// Name returns the provider name (e.g., "bedrock", "openai", "mock").
	Name() string
}

// SummarizeRequest contains the document to summarize.
type SummarizeRequest struct {
	Content          string // Document content to summarize
	Title            string // Document title for context
	DocType          string // Document type (RFC, PRD, FRD, etc.)
	MaxSummaryLength int    // Token limit for summary (0 = provider default)
	ExtractTopics    bool   // Extract main topics
	ExtractKeyPoints bool   // Extract key takeaways
	SuggestTags      bool   // Generate categorization tags
	AnalyzeStatus    bool   // Suggest document status from content maturity
}

// SummarizeResponse contains the AI-generated analysis.
type SummarizeResponse struct {
	Summary    *DocumentSummary // Generated summary and analysis
	Model      string           // Model used (e.g., "claude-3-7-sonnet")
	TokensUsed int              // Tokens consumed for generation
}

// DocumentSummary represents AI-generated analysis of a document.
// This is stored independently and can be referenced by DocumentContext.
type DocumentSummary struct {
	GeneratedAt      time.Time
	DocumentID       string
	ExecutiveSummary string
	SuggestedStatus  string
	Model            string
	KeyPoints        []string
	Topics           []string
	Tags             []string
	Confidence       float64
	TokensUsed       int
}

// EmbeddingRequest contains text to embed.
type EmbeddingRequest struct {
	Texts        []string // Support batch embedding
	ChunkSize    int      // Max tokens per chunk (0 = no chunking)
	ChunkOverlap int      // Overlap between chunks in tokens
}

// EmbeddingResponse contains the generated embeddings.
type EmbeddingResponse struct {
	Embeddings *DocumentEmbeddings // Generated embeddings
	Model      string              // Model used
	Dimensions int                 // Embedding dimensions
	TokensUsed int                 // Tokens consumed
}

// DocumentEmbeddings represents vector embeddings for a document.
// This is stored independently and can be referenced by DocumentContext.
type DocumentEmbeddings struct {
	GeneratedAt      time.Time
	DocumentID       string
	Model            string
	ContentEmbedding []float32
	Chunks           []ChunkEmbedding
	Dimensions       int
	TokensUsed       int
}

// ChunkEmbedding represents an embedding for a text chunk.
type ChunkEmbedding struct {
	Text       string
	Embedding  []float32
	ChunkIndex int
	StartPos   int
	EndPos     int
}
