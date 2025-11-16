package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp-forge/hermes/internal/server"
	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"
	"github.com/hashicorp-forge/hermes/pkg/search"
)

// SemanticSearchRequest represents a semantic search query request.
type SemanticSearchRequest struct {
	Query         string   `json:"query"`                   // Search query text
	Limit         int      `json:"limit,omitempty"`         // Maximum results (default: 10)
	MinSimilarity float64  `json:"minSimilarity,omitempty"` // Minimum similarity threshold (0-1)
	DocumentIDs   []string `json:"documentIds,omitempty"`   // Filter by specific document IDs
	DocumentTypes []string `json:"documentTypes,omitempty"` // Filter by document types
}

// HybridSearchRequest represents a hybrid (keyword + semantic) search request.
type HybridSearchRequest struct {
	Query          string  `json:"query"`                    // Search query text
	Limit          int     `json:"limit,omitempty"`          // Maximum results (default: 10)
	KeywordWeight  float64 `json:"keywordWeight,omitempty"`  // Weight for keyword search (0-1, default: 0.4)
	SemanticWeight float64 `json:"semanticWeight,omitempty"` // Weight for semantic search (0-1, default: 0.4)
	BoostBoth      float64 `json:"boostBoth,omitempty"`      // Bonus for docs in both results (0-1, default: 0.2)
	MinSimilarity  float64 `json:"minSimilarity,omitempty"`  // Minimum similarity threshold
}

// SemanticSearchResponse represents the response from semantic search.
type SemanticSearchResponse struct {
	Results []SemanticSearchResult `json:"results"`
	Query   string                 `json:"query"`
	Count   int                    `json:"count"`
}

// SemanticSearchResult represents a single semantic search result.
type SemanticSearchResult struct {
	DocumentID   string  `json:"documentId"`
	DocumentUUID string  `json:"documentUuid,omitempty"`
	Title        string  `json:"title,omitempty"`
	Excerpt      string  `json:"excerpt,omitempty"`
	Similarity   float64 `json:"similarity"` // Cosine similarity score (0-1)
	ChunkIndex   *int    `json:"chunkIndex,omitempty"`
	ChunkText    string  `json:"chunkText,omitempty"`
}

// HybridSearchResponse represents the response from hybrid search.
type HybridSearchResponse struct {
	Results []HybridSearchResult `json:"results"`
	Query   string               `json:"query"`
	Count   int                  `json:"count"`
}

// HybridSearchResult represents a single hybrid search result.
type HybridSearchResult struct {
	DocumentID    string  `json:"documentId"`
	DocumentUUID  string  `json:"documentUuid,omitempty"`
	Title         string  `json:"title,omitempty"`
	Excerpt       string  `json:"excerpt,omitempty"`
	HybridScore   float64 `json:"hybridScore"`   // Combined score (0-1)
	KeywordScore  float64 `json:"keywordScore"`  // Score from keyword search
	SemanticScore float64 `json:"semanticScore"` // Score from semantic search
	MatchedInBoth bool    `json:"matchedInBoth"` // True if in both keyword and semantic results
}

// SemanticSearchHandler handles semantic (vector) search requests.
//
// Endpoint: POST /api/v2/search/semantic
//
// Uses OpenAI embeddings and pgvector to find semantically similar documents.
func SemanticSearchHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Authorize request
		userEmail, ok := pkgauth.GetUserEmail(r.Context())
		if !ok || userEmail == "" {
			srv.Logger.Error("no user email found in request context",
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if semantic search is configured
		if srv.SemanticSearch == nil {
			srv.Logger.Error("semantic search not configured",
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "Semantic search not available", http.StatusServiceUnavailable)
			return
		}

		// Parse request
		var req SemanticSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			srv.Logger.Error("error decoding semantic search request",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate query
		if strings.TrimSpace(req.Query) == "" {
			http.Error(w, "Query cannot be empty", http.StatusBadRequest)
			return
		}

		// Set defaults
		if req.Limit <= 0 {
			req.Limit = 10
		}
		if req.Limit > 100 {
			req.Limit = 100 // Cap at 100 results
		}

		// Perform semantic search with filters if provided
		var results []search.SemanticSearchResult
		var err error

		if len(req.DocumentIDs) > 0 || req.MinSimilarity > 0 {
			// Use filtered search
			filter := search.SearchFilter{
				DocumentIDs:   req.DocumentIDs,
				DocumentTypes: req.DocumentTypes,
				MinSimilarity: req.MinSimilarity,
			}
			results, err = srv.SemanticSearch.SearchWithFilters(r.Context(), req.Query, req.Limit, filter)
		} else {
			// Use basic search
			results, err = srv.SemanticSearch.Search(r.Context(), req.Query, req.Limit)
		}

		if err != nil {
			srv.Logger.Error("semantic search failed",
				"error", err,
				"query", req.Query,
				"user", userEmail,
			)
			http.Error(w, "Search failed", http.StatusInternalServerError)
			return
		}

		// Convert to response format
		respResults := make([]SemanticSearchResult, len(results))
		for i, r := range results {
			respResults[i] = SemanticSearchResult{
				DocumentID:   r.DocumentID,
				DocumentUUID: r.DocumentUUID,
				Similarity:   r.Similarity,
				ChunkIndex:   r.ChunkIndex,
				ChunkText:    r.ChunkText,
			}

			// TODO: Fetch document title and excerpt from database
			// For now, use chunk text as excerpt
			if r.ChunkText != "" {
				excerpt := r.ChunkText
				if len(excerpt) > 200 {
					excerpt = excerpt[:200] + "..."
				}
				respResults[i].Excerpt = excerpt
			}
		}

		response := SemanticSearchResponse{
			Results: respResults,
			Query:   req.Query,
			Count:   len(respResults),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			srv.Logger.Error("error encoding semantic search response",
				"error", err,
			)
		}

		srv.Logger.Info("semantic search completed",
			"query", req.Query,
			"results", len(respResults),
			"user", userEmail,
		)
	})
}

// HybridSearchHandler handles hybrid (keyword + semantic) search requests.
//
// Endpoint: POST /api/v2/search/hybrid
//
// Combines traditional keyword search (Meilisearch) with semantic search (pgvector)
// using configurable weights.
func HybridSearchHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Authorize request
		userEmail, ok := pkgauth.GetUserEmail(r.Context())
		if !ok || userEmail == "" {
			srv.Logger.Error("no user email found in request context",
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if hybrid search is configured
		if srv.HybridSearch == nil {
			srv.Logger.Error("hybrid search not configured",
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "Hybrid search not available", http.StatusServiceUnavailable)
			return
		}

		// Parse request
		var req HybridSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			srv.Logger.Error("error decoding hybrid search request",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate query
		if strings.TrimSpace(req.Query) == "" {
			http.Error(w, "Query cannot be empty", http.StatusBadRequest)
			return
		}

		// Set defaults
		if req.Limit <= 0 {
			req.Limit = 10
		}
		if req.Limit > 100 {
			req.Limit = 100
		}

		// Configure search weights
		weights := search.DefaultWeights()
		if req.KeywordWeight > 0 || req.SemanticWeight > 0 || req.BoostBoth > 0 {
			weights = search.SearchWeights{
				KeywordWeight:  req.KeywordWeight,
				SemanticWeight: req.SemanticWeight,
				BoostBoth:      req.BoostBoth,
			}
			// Default to balanced if not specified
			if weights.KeywordWeight == 0 {
				weights.KeywordWeight = 0.4
			}
			if weights.SemanticWeight == 0 {
				weights.SemanticWeight = 0.4
			}
			if weights.BoostBoth == 0 {
				weights.BoostBoth = 0.2
			}
		}

		// Perform hybrid search
		results, err := srv.HybridSearch.Search(r.Context(), req.Query, req.Limit, weights)
		if err != nil {
			srv.Logger.Error("hybrid search failed",
				"error", err,
				"query", req.Query,
				"user", userEmail,
			)
			http.Error(w, "Search failed", http.StatusInternalServerError)
			return
		}

		// Convert to response format
		respResults := make([]HybridSearchResult, len(results))
		for i, r := range results {
			respResults[i] = HybridSearchResult{
				DocumentID:    r.DocumentID,
				DocumentUUID:  r.DocumentUUID,
				Title:         r.Title,
				HybridScore:   r.HybridScore,
				KeywordScore:  r.KeywordScore,
				SemanticScore: r.SemanticScore,
				MatchedInBoth: r.MatchedInBoth,
			}

			// TODO: Fetch excerpt from database or search highlight
		}

		response := HybridSearchResponse{
			Results: respResults,
			Query:   req.Query,
			Count:   len(respResults),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			srv.Logger.Error("error encoding hybrid search response",
				"error", err,
			)
		}

		srv.Logger.Info("hybrid search completed",
			"query", req.Query,
			"results", len(respResults),
			"keyword_weight", weights.KeywordWeight,
			"semantic_weight", weights.SemanticWeight,
			"user", userEmail,
		)
	})
}

// SimilarDocumentsHandler finds documents similar to a given document.
//
// Endpoint: GET /api/v2/documents/{documentID}/similar?limit=10
//
// Uses the document's existing embeddings to find similar documents via vector similarity.
func SimilarDocumentsHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Authorize request
		userEmail, ok := pkgauth.GetUserEmail(r.Context())
		if !ok || userEmail == "" {
			srv.Logger.Error("no user email found in request context",
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if semantic search is configured
		if srv.SemanticSearch == nil {
			srv.Logger.Error("semantic search not configured",
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "Semantic search not available", http.StatusServiceUnavailable)
			return
		}

		// Extract document ID from URL path
		// Expected format: /api/v2/documents/{documentID}/similar
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) < 5 || pathParts[4] != "similar" {
			http.Error(w, "Invalid path format", http.StatusBadRequest)
			return
		}
		documentID := pathParts[3]

		if documentID == "" {
			http.Error(w, "Document ID required", http.StatusBadRequest)
			return
		}

		// Parse query parameters
		limitStr := r.URL.Query().Get("limit")
		limit := 10
		if limitStr != "" {
			if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
				limit = parsedLimit
				if limit > 100 {
					limit = 100
				}
			}
		}

		// Find similar documents
		results, err := srv.SemanticSearch.FindSimilarDocuments(r.Context(), documentID, limit)
		if err != nil {
			srv.Logger.Error("failed to find similar documents",
				"error", err,
				"documentID", documentID,
				"user", userEmail,
			)
			http.Error(w, "Failed to find similar documents", http.StatusInternalServerError)
			return
		}

		// Convert to response format
		respResults := make([]SemanticSearchResult, len(results))
		for i, r := range results {
			respResults[i] = SemanticSearchResult{
				DocumentID:   r.DocumentID,
				DocumentUUID: r.DocumentUUID,
				Similarity:   r.Similarity,
				ChunkIndex:   r.ChunkIndex,
				ChunkText:    r.ChunkText,
			}

			// Use chunk text as excerpt
			if r.ChunkText != "" {
				excerpt := r.ChunkText
				if len(excerpt) > 200 {
					excerpt = excerpt[:200] + "..."
				}
				respResults[i].Excerpt = excerpt
			}
		}

		response := SemanticSearchResponse{
			Results: respResults,
			Query:   "similar to " + documentID,
			Count:   len(respResults),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			srv.Logger.Error("error encoding similar documents response",
				"error", err,
			)
		}

		srv.Logger.Info("similar documents search completed",
			"documentID", documentID,
			"results", len(respResults),
			"user", userEmail,
		)
	})
}
