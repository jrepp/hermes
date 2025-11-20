package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/models"
)

// EmbeddingsGenerator generates embeddings for text.
type EmbeddingsGenerator interface {
	GenerateEmbeddings(ctx context.Context, text string, model string, dimensions int) ([]float64, error)
}

// SemanticSearch provides semantic/vector search capabilities.
type SemanticSearch struct {
	db         *gorm.DB
	embedGen   EmbeddingsGenerator
	model      string
	dimensions int
	logger     hclog.Logger
}

// SemanticSearchConfig holds configuration for semantic search.
type SemanticSearchConfig struct {
	DB         *gorm.DB
	EmbedGen   EmbeddingsGenerator
	Model      string // e.g., "text-embedding-3-small"
	Dimensions int    // e.g., 1536
	Logger     hclog.Logger
}

// SemanticSearchResult represents a search result with similarity score.
type SemanticSearchResult struct {
	DocumentID   string
	DocumentUUID string
	RevisionID   *int
	ChunkIndex   *int
	ChunkText    string
	Similarity   float64 // Cosine similarity score (0-1, higher is better)

	// Optional metadata
	ContentHash string
	Model       string
	Provider    string
}

// NewSemanticSearch creates a new semantic search instance.
func NewSemanticSearch(config SemanticSearchConfig) (*SemanticSearch, error) {
	if config.DB == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	if config.EmbedGen == nil {
		return nil, fmt.Errorf("embeddings generator is required")
	}
	if config.Model == "" {
		config.Model = "text-embedding-3-small"
	}
	if config.Dimensions == 0 {
		config.Dimensions = 1536
	}
	if config.Logger == nil {
		config.Logger = hclog.NewNullLogger()
	}

	return &SemanticSearch{
		db:         config.DB,
		embedGen:   config.EmbedGen,
		model:      config.Model,
		dimensions: config.Dimensions,
		logger:     config.Logger.Named("semantic-search"),
	}, nil
}

// Search performs semantic search using vector similarity.
// Returns documents most similar to the query text.
func (s *SemanticSearch) Search(ctx context.Context, query string, limit int) ([]SemanticSearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if limit <= 0 {
		limit = 10
	}

	s.logger.Debug("performing semantic search",
		"query", query,
		"limit", limit,
		"model", s.model,
	)

	// Generate embeddings for the query
	queryEmbedding, err := s.embedGen.GenerateEmbeddings(ctx, query, s.model, s.dimensions)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embeddings: %w", err)
	}

	// Search for similar embeddings using pgvector
	results, err := s.searchSimilar(ctx, queryEmbedding, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search similar embeddings: %w", err)
	}

	s.logger.Info("semantic search completed",
		"query", query,
		"results_count", len(results),
	)

	return results, nil
}

// searchSimilar uses pgvector to find similar embeddings.
func (s *SemanticSearch) searchSimilar(ctx context.Context, queryEmbedding []float64, limit int) ([]SemanticSearchResult, error) {
	// Convert embedding to pgvector format string
	vectorStr := s.formatVectorForPostgres(queryEmbedding)

	// Query using pgvector cosine distance operator (<=>)
	// cosine distance = 1 - cosine similarity
	// So we use (1 - distance) to get similarity score
	query := `
		SELECT
			document_id,
			document_uuid,
			revision_id,
			chunk_index,
			chunk_text,
			content_hash,
			model,
			provider,
			(1 - (embedding_vector <=> $1::vector)) as similarity
		FROM document_embeddings
		WHERE embedding_vector IS NOT NULL
		  AND model = $2
		ORDER BY embedding_vector <=> $1::vector
		LIMIT $3
	`

	type row struct {
		DocumentID   string
		DocumentUUID *string
		RevisionID   *int
		ChunkIndex   *int
		ChunkText    string
		ContentHash  string
		Model        string
		Provider     string
		Similarity   float64
	}

	var rows []row
	err := s.db.WithContext(ctx).Raw(query, vectorStr, s.model, limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query similar embeddings: %w", err)
	}

	// Convert to search results
	results := make([]SemanticSearchResult, len(rows))
	for i, r := range rows {
		results[i] = SemanticSearchResult{
			DocumentID:   r.DocumentID,
			DocumentUUID: stringPtrToString(r.DocumentUUID),
			RevisionID:   r.RevisionID,
			ChunkIndex:   r.ChunkIndex,
			ChunkText:    r.ChunkText,
			ContentHash:  r.ContentHash,
			Model:        r.Model,
			Provider:     r.Provider,
			Similarity:   r.Similarity,
		}
	}

	return results, nil
}

// SearchByEmbedding searches using a pre-generated embedding vector.
// Useful when you already have embeddings and want to find similar documents.
func (s *SemanticSearch) SearchByEmbedding(ctx context.Context, embedding []float64, limit int) ([]SemanticSearchResult, error) {
	if len(embedding) != s.dimensions {
		return nil, fmt.Errorf("embedding dimensions mismatch: expected %d, got %d", s.dimensions, len(embedding))
	}
	if limit <= 0 {
		limit = 10
	}

	return s.searchSimilar(ctx, embedding, limit)
}

// SearchWithFilter performs semantic search with additional filters.
type SearchFilter struct {
	DocumentTypes []string // Filter by document type
	DocumentIDs   []string // Filter by specific document IDs
	MinSimilarity float64  // Minimum similarity threshold (0-1)
}

// SearchWithFilters performs filtered semantic search.
func (s *SemanticSearch) SearchWithFilters(ctx context.Context, query string, limit int, filter SearchFilter) ([]SemanticSearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if limit <= 0 {
		limit = 10
	}

	// Generate embeddings for the query
	queryEmbedding, err := s.embedGen.GenerateEmbeddings(ctx, query, s.model, s.dimensions)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embeddings: %w", err)
	}

	// Build filtered query
	vectorStr := s.formatVectorForPostgres(queryEmbedding)

	queryStr := `
		SELECT
			document_id,
			document_uuid,
			revision_id,
			chunk_index,
			chunk_text,
			content_hash,
			model,
			provider,
			(1 - (embedding_vector <=> $1::vector)) as similarity
		FROM document_embeddings
		WHERE embedding_vector IS NOT NULL
		  AND model = $2
	`

	args := []interface{}{vectorStr, s.model}
	argIndex := 3

	// Add document ID filter
	if len(filter.DocumentIDs) > 0 {
		queryStr += fmt.Sprintf(" AND document_id = ANY($%d)", argIndex)
		args = append(args, filter.DocumentIDs)
		argIndex++
	}

	// Add minimum similarity filter
	if filter.MinSimilarity > 0 {
		queryStr += fmt.Sprintf(" AND (1 - (embedding_vector <=> $1::vector)) >= $%d", argIndex)
		args = append(args, filter.MinSimilarity)
		argIndex++
	}

	queryStr += fmt.Sprintf(" ORDER BY embedding_vector <=> $1::vector LIMIT $%d", argIndex)
	args = append(args, limit)

	type row struct {
		DocumentID   string
		DocumentUUID *string
		RevisionID   *int
		ChunkIndex   *int
		ChunkText    string
		ContentHash  string
		Model        string
		Provider     string
		Similarity   float64
	}

	var rows []row
	err = s.db.WithContext(ctx).Raw(queryStr, args...).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query similar embeddings: %w", err)
	}

	// Convert to search results
	results := make([]SemanticSearchResult, len(rows))
	for i, r := range rows {
		results[i] = SemanticSearchResult{
			DocumentID:   r.DocumentID,
			DocumentUUID: stringPtrToString(r.DocumentUUID),
			RevisionID:   r.RevisionID,
			ChunkIndex:   r.ChunkIndex,
			ChunkText:    r.ChunkText,
			ContentHash:  r.ContentHash,
			Model:        r.Model,
			Provider:     r.Provider,
			Similarity:   r.Similarity,
		}
	}

	return results, nil
}

// GetDocumentEmbedding retrieves the embedding for a specific document.
func (s *SemanticSearch) GetDocumentEmbedding(ctx context.Context, documentID string) (*models.DocumentEmbedding, error) {
	var embedding models.DocumentEmbedding
	err := s.db.WithContext(ctx).
		Where("document_id = ? AND model = ?", documentID, s.model).
		First(&embedding).Error
	if err != nil {
		return nil, err
	}
	return &embedding, nil
}

// FindSimilarDocuments finds documents similar to a given document.
func (s *SemanticSearch) FindSimilarDocuments(ctx context.Context, documentID string, limit int) ([]SemanticSearchResult, error) {
	// Get the embedding for the source document
	embedding, err := s.GetDocumentEmbedding(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document embedding: %w", err)
	}

	// Search for similar documents
	results, err := s.SearchByEmbedding(ctx, embedding.Embedding, limit+1)
	if err != nil {
		return nil, err
	}

	// Filter out the source document itself
	filtered := make([]SemanticSearchResult, 0, len(results))
	for _, r := range results {
		if r.DocumentID != documentID {
			filtered = append(filtered, r)
		}
	}

	// Limit to requested count
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

// formatVectorForPostgres converts a float64 slice to pgvector format string.
// Example: [0.1, 0.2, 0.3] -> "[0.1,0.2,0.3]"
func (s *SemanticSearch) formatVectorForPostgres(vec []float64) string {
	if len(vec) == 0 {
		return "[]"
	}

	strValues := make([]string, len(vec))
	for i, v := range vec {
		strValues[i] = fmt.Sprintf("%f", v)
	}

	return "[" + strings.Join(strValues, ",") + "]"
}

// stringPtrToString safely converts string pointer to string.
func stringPtrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
