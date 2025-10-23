package search

import (
	"context"
	"time"
)

// VectorIndex handles vector similarity search operations.
// This extends the search provider with semantic/vector search capabilities.
type VectorIndex interface {
	// IndexEmbedding stores a document's vector embedding.
	IndexEmbedding(ctx context.Context, doc *VectorDocument) error

	// IndexEmbeddingBatch stores multiple document embeddings.
	IndexEmbeddingBatch(ctx context.Context, docs []*VectorDocument) error

	// SearchSimilar finds documents similar to the query embedding.
	SearchSimilar(ctx context.Context, query *VectorSearchQuery) (*VectorSearchResult, error)

	// SearchHybrid combines vector similarity with keyword search.
	SearchHybrid(ctx context.Context, query *HybridSearchQuery) (*SearchResult, error)

	// Delete removes a document's embeddings.
	Delete(ctx context.Context, docID string) error

	// DeleteBatch removes multiple documents' embeddings.
	DeleteBatch(ctx context.Context, docIDs []string) error

	// GetEmbedding retrieves stored embedding for a document.
	GetEmbedding(ctx context.Context, docID string) (*VectorDocument, error)

	// Clear removes all vector data (use with caution).
	Clear(ctx context.Context) error
}

// VectorDocument represents a document with embeddings.
type VectorDocument struct {
	ObjectID   string    // Unique document identifier
	DocID      string    // Document ID in the source system
	Title      string    // Document title
	DocType    string    // Document type (RFC, PRD, etc.)
	ModifiedAt time.Time // Last modification time

	// Vector embeddings
	ContentEmbedding []float32        // Full document embedding
	ChunkEmbeddings  []ChunkEmbedding // Individual chunk embeddings

	// Metadata for hybrid search
	Summary   string   // AI-generated summary
	KeyPoints []string // Main takeaways
	Topics    []string // Extracted topics
	Tags      []string // Categorization tags

	// Embedding info
	Model      string    // e.g., "amazon.titan-embed-text-v2"
	Dimensions int       // Embedding dimensions (e.g., 1024)
	EmbeddedAt time.Time // When embeddings were generated
}

// ChunkEmbedding represents an embedding for a text chunk.
type ChunkEmbedding struct {
	ChunkIndex int       // Sequential chunk number
	Text       string    // Actual text content of chunk
	Embedding  []float32 // Vector embedding for this chunk
	StartPos   int       // Character position in original text
	EndPos     int       // End character position
}

// VectorSearchQuery for similarity search.
type VectorSearchQuery struct {
	QueryEmbedding []float32              // Query vector
	Limit          int                    // Maximum results to return
	Threshold      float64                // Minimum similarity score (0.0-1.0)
	Filters        map[string]interface{} // Filter by docType, status, etc.
}

// HybridSearchQuery combines vector and keyword search.
type HybridSearchQuery struct {
	QueryText      string                 // Text query for keyword search
	QueryEmbedding []float32              // Query vector for semantic search
	VectorWeight   float64                // Weight for vector search (0.0-1.0)
	KeywordWeight  float64                // Weight for keyword search (0.0-1.0)
	Limit          int                    // Maximum results to return
	Filters        map[string]interface{} // Filter criteria
}

// VectorSearchResult contains similar documents.
type VectorSearchResult struct {
	Hits  []VectorHit   // Matching documents
	Total int           // Total matches found
	Took  time.Duration // Time taken for search
}

// VectorHit represents a search result with similarity score.
type VectorHit struct {
	Document      *VectorDocument // Matching document
	Score         float64         // Similarity score (0.0-1.0)
	MatchedChunks []int           // Which chunks matched (for chunked embeddings)
}
