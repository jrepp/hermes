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
	ModifiedAt       time.Time
	EmbeddedAt       time.Time
	Model            string
	DocID            string
	Title            string
	DocType          string
	ObjectID         string
	Summary          string
	ChunkEmbeddings  []ChunkEmbedding
	Topics           []string
	Tags             []string
	KeyPoints        []string
	ContentEmbedding []float32
	Dimensions       int
}

// ChunkEmbedding represents an embedding for a text chunk.
type ChunkEmbedding struct {
	Text       string
	Embedding  []float32
	ChunkIndex int
	StartPos   int
	EndPos     int
}

// VectorSearchQuery for similarity search.
type VectorSearchQuery struct {
	Filters        map[string]interface{}
	QueryEmbedding []float32
	Limit          int
	Threshold      float64
}

// HybridSearchQuery combines vector and keyword search.
type HybridSearchQuery struct {
	Filters        map[string]interface{}
	QueryText      string
	QueryEmbedding []float32
	VectorWeight   float64
	KeywordWeight  float64
	Limit          int
}

// VectorSearchResult contains similar documents.
type VectorSearchResult struct {
	Hits  []VectorHit   // Matching documents
	Total int           // Total matches found
	Took  time.Duration // Time taken for search
}

// VectorHit represents a search result with similarity score.
type VectorHit struct {
	Document      *VectorDocument
	MatchedChunks []int
	Score         float64
}
