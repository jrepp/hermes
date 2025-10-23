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
	DocumentID       string    // Reference to source document
	ExecutiveSummary string    // Brief overview (2-3 sentences)
	KeyPoints        []string  // Main takeaways
	Topics           []string  // Extracted topics
	Tags             []string  // Generated tags for categorization
	SuggestedStatus  string    // e.g., "In Review", "Approved"
	Confidence       float64   // AI confidence in analysis (0.0-1.0)
	GeneratedAt      time.Time // When this was generated
	Model            string    // e.g., "claude-3-7-sonnet"
	TokensUsed       int       // Tokens consumed for generation
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
	DocumentID       string           // Reference to source document
	ContentEmbedding []float32        // Full document embedding
	Chunks           []ChunkEmbedding // Individual chunk embeddings
	Model            string           // e.g., "amazon.titan-embed-text-v2"
	Dimensions       int              // Embedding dimensions (e.g., 1024)
	GeneratedAt      time.Time        // When embeddings were generated
	TokensUsed       int              // Tokens consumed for generation
}

// ChunkEmbedding represents an embedding for a text chunk.
type ChunkEmbedding struct {
	ChunkIndex int       // Sequential chunk number
	StartPos   int       // Character position in original text
	EndPos     int       // End character position
	Text       string    // Actual text content of chunk
	Embedding  []float32 // Vector embedding for this chunk
}
