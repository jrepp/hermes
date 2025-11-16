package search

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/go-hclog"
)

// KeywordSearcher provides keyword-based search (e.g., Meilisearch).
type KeywordSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]KeywordSearchResult, error)
}

// KeywordSearchResult represents a keyword search result.
type KeywordSearchResult struct {
	DocumentID   string
	DocumentUUID string
	Title        string
	Type         string
	Score        float64 // Relevance score from keyword search
}

// HybridSearch combines keyword and semantic search.
type HybridSearch struct {
	keywordSearch  *KeywordSearcher
	semanticSearch *SemanticSearch
	logger         hclog.Logger
}

// HybridSearchConfig holds configuration for hybrid search.
type HybridSearchConfig struct {
	KeywordSearch  KeywordSearcher
	SemanticSearch *SemanticSearch
	Logger         hclog.Logger
}

// HybridSearchResult represents a combined search result.
type HybridSearchResult struct {
	DocumentID    string
	DocumentUUID  string
	Title         string
	Type          string
	KeywordScore  float64 // Score from keyword search (0-1)
	SemanticScore float64 // Score from semantic search (0-1)
	HybridScore   float64 // Combined score (0-1)
	MatchedInBoth bool    // True if document appeared in both keyword and semantic results
}

// NewHybridSearch creates a new hybrid search instance.
func NewHybridSearch(config HybridSearchConfig) (*HybridSearch, error) {
	if config.Logger == nil {
		config.Logger = hclog.NewNullLogger()
	}

	return &HybridSearch{
		keywordSearch:  &config.KeywordSearch,
		semanticSearch: config.SemanticSearch,
		logger:         config.Logger.Named("hybrid-search"),
	}, nil
}

// Search performs hybrid search combining keyword and semantic approaches.
// The algorithm:
// 1. Perform keyword search (Meilisearch)
// 2. Perform semantic search (pgvector)
// 3. Merge and rank results using a weighted combination
func (h *HybridSearch) Search(ctx context.Context, query string, limit int, weights SearchWeights) ([]HybridSearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if limit <= 0 {
		limit = 10
	}

	h.logger.Debug("performing hybrid search",
		"query", query,
		"limit", limit,
		"keyword_weight", weights.KeywordWeight,
		"semantic_weight", weights.SemanticWeight,
	)

	// Perform both searches in parallel
	keywordResults, keywordErr := h.performKeywordSearch(ctx, query, limit*2) // Fetch more for merging
	semanticResults, semanticErr := h.performSemanticSearch(ctx, query, limit*2)

	// Handle errors (allow partial results if one fails)
	if keywordErr != nil && semanticErr != nil {
		return nil, fmt.Errorf("both keyword and semantic search failed: keyword=%w, semantic=%w", keywordErr, semanticErr)
	}

	// If only keyword search succeeded, return those results
	if semanticErr != nil {
		h.logger.Warn("semantic search failed, using keyword results only", "error", semanticErr)
		return h.convertKeywordResults(keywordResults, limit), nil
	}

	// If only semantic search succeeded, return those results
	if keywordErr != nil {
		h.logger.Warn("keyword search failed, using semantic results only", "error", keywordErr)
		return h.convertSemanticResults(semanticResults, limit), nil
	}

	// Merge and rank results
	merged := h.mergeResults(keywordResults, semanticResults, weights)

	// Limit results
	if len(merged) > limit {
		merged = merged[:limit]
	}

	h.logger.Info("hybrid search completed",
		"query", query,
		"keyword_results", len(keywordResults),
		"semantic_results", len(semanticResults),
		"merged_results", len(merged),
	)

	return merged, nil
}

// SearchWeights defines the weights for combining keyword and semantic scores.
type SearchWeights struct {
	KeywordWeight  float64 // Weight for keyword search (0-1)
	SemanticWeight float64 // Weight for semantic search (0-1)
	BoostBoth      float64 // Bonus for documents appearing in both (0-1)
}

// DefaultWeights returns balanced weights for hybrid search.
func DefaultWeights() SearchWeights {
	return SearchWeights{
		KeywordWeight:  0.4,
		SemanticWeight: 0.4,
		BoostBoth:      0.2,
	}
}

// KeywordFocusedWeights returns weights favoring keyword search.
func KeywordFocusedWeights() SearchWeights {
	return SearchWeights{
		KeywordWeight:  0.7,
		SemanticWeight: 0.2,
		BoostBoth:      0.1,
	}
}

// SemanticFocusedWeights returns weights favoring semantic search.
func SemanticFocusedWeights() SearchWeights {
	return SearchWeights{
		KeywordWeight:  0.2,
		SemanticWeight: 0.7,
		BoostBoth:      0.1,
	}
}

// performKeywordSearch executes keyword search.
func (h *HybridSearch) performKeywordSearch(ctx context.Context, query string, limit int) ([]KeywordSearchResult, error) {
	if h.keywordSearch == nil || *h.keywordSearch == nil {
		return []KeywordSearchResult{}, nil // No keyword search available
	}

	return (*h.keywordSearch).Search(ctx, query, limit)
}

// performSemanticSearch executes semantic search.
func (h *HybridSearch) performSemanticSearch(ctx context.Context, query string, limit int) ([]SemanticSearchResult, error) {
	if h.semanticSearch == nil {
		return []SemanticSearchResult{}, nil // No semantic search available
	}

	return h.semanticSearch.Search(ctx, query, limit)
}

// mergeResults combines keyword and semantic results with weighted scoring.
func (h *HybridSearch) mergeResults(
	keywordResults []KeywordSearchResult,
	semanticResults []SemanticSearchResult,
	weights SearchWeights,
) []HybridSearchResult {
	// Create a map to track all documents
	resultsMap := make(map[string]*HybridSearchResult)

	// Add keyword results
	for _, kr := range keywordResults {
		resultsMap[kr.DocumentID] = &HybridSearchResult{
			DocumentID:    kr.DocumentID,
			DocumentUUID:  kr.DocumentUUID,
			Title:         kr.Title,
			Type:          kr.Type,
			KeywordScore:  kr.Score,
			SemanticScore: 0,
			MatchedInBoth: false,
		}
	}

	// Add/merge semantic results
	for _, sr := range semanticResults {
		if existing, ok := resultsMap[sr.DocumentID]; ok {
			// Document appeared in both searches
			existing.SemanticScore = sr.Similarity
			existing.MatchedInBoth = true
		} else {
			// Document only in semantic results
			resultsMap[sr.DocumentID] = &HybridSearchResult{
				DocumentID:    sr.DocumentID,
				DocumentUUID:  sr.DocumentUUID,
				SemanticScore: sr.Similarity,
				KeywordScore:  0,
				MatchedInBoth: false,
			}
		}
	}

	// Calculate hybrid scores
	results := make([]HybridSearchResult, 0, len(resultsMap))
	for _, r := range resultsMap {
		// Weighted combination
		score := (r.KeywordScore * weights.KeywordWeight) +
			(r.SemanticScore * weights.SemanticWeight)

		// Boost for appearing in both
		if r.MatchedInBoth {
			score += weights.BoostBoth
		}

		// Normalize to 0-1 range
		if score > 1.0 {
			score = 1.0
		}

		r.HybridScore = score
		results = append(results, *r)
	}

	// Sort by hybrid score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].HybridScore > results[j].HybridScore
	})

	return results
}

// convertKeywordResults converts keyword results to hybrid results.
func (h *HybridSearch) convertKeywordResults(results []KeywordSearchResult, limit int) []HybridSearchResult {
	hybrid := make([]HybridSearchResult, 0, len(results))
	for i, r := range results {
		if i >= limit {
			break
		}
		hybrid = append(hybrid, HybridSearchResult{
			DocumentID:    r.DocumentID,
			DocumentUUID:  r.DocumentUUID,
			Title:         r.Title,
			Type:          r.Type,
			KeywordScore:  r.Score,
			SemanticScore: 0,
			HybridScore:   r.Score,
			MatchedInBoth: false,
		})
	}
	return hybrid
}

// convertSemanticResults converts semantic results to hybrid results.
func (h *HybridSearch) convertSemanticResults(results []SemanticSearchResult, limit int) []HybridSearchResult {
	hybrid := make([]HybridSearchResult, 0, len(results))
	for i, r := range results {
		if i >= limit {
			break
		}
		hybrid = append(hybrid, HybridSearchResult{
			DocumentID:    r.DocumentID,
			DocumentUUID:  r.DocumentUUID,
			KeywordScore:  0,
			SemanticScore: r.Similarity,
			HybridScore:   r.Similarity,
			MatchedInBoth: false,
		})
	}
	return hybrid
}

// SearchKeywordOnly performs only keyword search.
func (h *HybridSearch) SearchKeywordOnly(ctx context.Context, query string, limit int) ([]HybridSearchResult, error) {
	results, err := h.performKeywordSearch(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	return h.convertKeywordResults(results, limit), nil
}

// SearchSemanticOnly performs only semantic search.
func (h *HybridSearch) SearchSemanticOnly(ctx context.Context, query string, limit int) ([]HybridSearchResult, error) {
	results, err := h.performSemanticSearch(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	return h.convertSemanticResults(results, limit), nil
}
