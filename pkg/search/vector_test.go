package search

import (
	"testing"
	"time"
)

func TestVectorDocument_Creation(t *testing.T) {
	now := time.Now()
	doc := &VectorDocument{
		ObjectID:         "doc-123",
		DocID:            "RFC-042",
		Title:            "Test Document",
		DocType:          "RFC",
		ModifiedAt:       now,
		ContentEmbedding: []float32{0.1, 0.2, 0.3, 0.4},
		Summary:          "Test summary",
		KeyPoints:        []string{"point 1", "point 2"},
		Topics:           []string{"infrastructure", "testing"},
		Tags:             []string{"rfc", "draft"},
		Model:            "amazon.titan-embed-text-v2",
		Dimensions:       1024,
		EmbeddedAt:       now,
	}

	if doc.ObjectID != "doc-123" {
		t.Errorf("ObjectID = %q, want %q", doc.ObjectID, "doc-123")
	}
	if len(doc.ContentEmbedding) != 4 {
		t.Errorf("ContentEmbedding length = %d, want 4", len(doc.ContentEmbedding))
	}
	if len(doc.KeyPoints) != 2 {
		t.Errorf("KeyPoints length = %d, want 2", len(doc.KeyPoints))
	}
}

func TestVectorDocument_EmptyFields(t *testing.T) {
	doc := &VectorDocument{
		ObjectID: "doc-empty",
		DocID:    "DOC-001",
	}

	// Should not panic with empty fields
	if doc.Title != "" {
		t.Error("expected empty title")
	}
	if doc.ContentEmbedding != nil {
		t.Error("expected nil ContentEmbedding")
	}
	if doc.ChunkEmbeddings != nil {
		t.Error("expected nil ChunkEmbeddings")
	}
	if len(doc.KeyPoints) != 0 {
		t.Error("expected empty KeyPoints")
	}
}

func TestChunkEmbedding_Creation(t *testing.T) {
	chunk := ChunkEmbedding{
		ChunkIndex: 0,
		Text:       "This is a test chunk of text.",
		Embedding:  []float32{0.1, 0.2, 0.3},
		StartPos:   0,
		EndPos:     30,
	}

	if chunk.ChunkIndex != 0 {
		t.Errorf("ChunkIndex = %d, want 0", chunk.ChunkIndex)
	}
	if chunk.Text != "This is a test chunk of text." {
		t.Errorf("Text = %q, want %q", chunk.Text, "This is a test chunk of text.")
	}
	if len(chunk.Embedding) != 3 {
		t.Errorf("Embedding length = %d, want 3", len(chunk.Embedding))
	}
	if chunk.EndPos-chunk.StartPos != 30 {
		t.Errorf("chunk length = %d, want 30", chunk.EndPos-chunk.StartPos)
	}
}

func TestVectorDocument_WithChunks(t *testing.T) {
	chunks := []ChunkEmbedding{
		{
			ChunkIndex: 0,
			Text:       "First chunk",
			Embedding:  []float32{0.1, 0.2},
			StartPos:   0,
			EndPos:     11,
		},
		{
			ChunkIndex: 1,
			Text:       "Second chunk",
			Embedding:  []float32{0.3, 0.4},
			StartPos:   11,
			EndPos:     23,
		},
		{
			ChunkIndex: 2,
			Text:       "Third chunk",
			Embedding:  []float32{0.5, 0.6},
			StartPos:   23,
			EndPos:     34,
		},
	}

	doc := &VectorDocument{
		ObjectID:        "chunked-doc",
		ChunkEmbeddings: chunks,
	}

	if len(doc.ChunkEmbeddings) != 3 {
		t.Errorf("ChunkEmbeddings length = %d, want 3", len(doc.ChunkEmbeddings))
	}

	// Verify chunks are sequential
	for i, chunk := range doc.ChunkEmbeddings {
		if chunk.ChunkIndex != i {
			t.Errorf("chunk %d has ChunkIndex = %d, want %d", i, chunk.ChunkIndex, i)
		}
	}
}

func TestVectorSearchQuery_Validation(t *testing.T) {
	tests := []struct {
		name    string
		query   *VectorSearchQuery
		isValid bool
	}{
		{
			name: "valid query",
			query: &VectorSearchQuery{
				QueryEmbedding: []float32{0.1, 0.2, 0.3},
				Limit:          10,
				Threshold:      0.5,
			},
			isValid: true,
		},
		{
			name: "empty embedding",
			query: &VectorSearchQuery{
				QueryEmbedding: []float32{},
				Limit:          10,
			},
			isValid: false,
		},
		{
			name: "nil embedding",
			query: &VectorSearchQuery{
				QueryEmbedding: nil,
				Limit:          10,
			},
			isValid: false,
		},
		{
			name: "zero limit",
			query: &VectorSearchQuery{
				QueryEmbedding: []float32{0.1, 0.2},
				Limit:          0,
			},
			isValid: false,
		},
		{
			name: "negative threshold",
			query: &VectorSearchQuery{
				QueryEmbedding: []float32{0.1, 0.2},
				Limit:          10,
				Threshold:      -0.5,
			},
			isValid: false,
		},
		{
			name: "threshold above 1.0",
			query: &VectorSearchQuery{
				QueryEmbedding: []float32{0.1, 0.2},
				Limit:          10,
				Threshold:      1.5,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateVectorQuery(tt.query)
			if valid != tt.isValid {
				t.Errorf("validation = %v, want %v", valid, tt.isValid)
			}
		})
	}
}

// Helper function for query validation (would be in production code)
func validateVectorQuery(q *VectorSearchQuery) bool {
	if q == nil {
		return false
	}
	if len(q.QueryEmbedding) == 0 {
		return false
	}
	if q.Limit <= 0 {
		return false
	}
	if q.Threshold < 0.0 || q.Threshold > 1.0 {
		return false
	}
	return true
}

func TestHybridSearchQuery_WeightValidation(t *testing.T) {
	tests := []struct {
		name          string
		vectorWeight  float64
		keywordWeight float64
		wantValid     bool
	}{
		{
			name:          "equal weights",
			vectorWeight:  0.5,
			keywordWeight: 0.5,
			wantValid:     true,
		},
		{
			name:          "vector-heavy",
			vectorWeight:  0.7,
			keywordWeight: 0.3,
			wantValid:     true,
		},
		{
			name:          "keyword-heavy",
			vectorWeight:  0.3,
			keywordWeight: 0.7,
			wantValid:     true,
		},
		{
			name:          "weights don't sum to 1.0",
			vectorWeight:  0.6,
			keywordWeight: 0.5,
			wantValid:     false,
		},
		{
			name:          "negative vector weight",
			vectorWeight:  -0.5,
			keywordWeight: 1.5,
			wantValid:     false,
		},
		{
			name:          "vector weight above 1.0",
			vectorWeight:  1.5,
			keywordWeight: -0.5,
			wantValid:     false,
		},
		{
			name:          "both weights zero",
			vectorWeight:  0.0,
			keywordWeight: 0.0,
			wantValid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &HybridSearchQuery{
				QueryText:      "test query",
				QueryEmbedding: []float32{0.1, 0.2},
				VectorWeight:   tt.vectorWeight,
				KeywordWeight:  tt.keywordWeight,
				Limit:          10,
			}

			valid := validateHybridQuery(query)
			if valid != tt.wantValid {
				t.Errorf("validation = %v, want %v (vw=%.1f, kw=%.1f)",
					valid, tt.wantValid, tt.vectorWeight, tt.keywordWeight)
			}
		})
	}
}

// Helper function for hybrid query validation
func validateHybridQuery(q *HybridSearchQuery) bool {
	if q == nil {
		return false
	}
	if q.QueryText == "" && len(q.QueryEmbedding) == 0 {
		return false
	}
	if q.Limit <= 0 {
		return false
	}
	if q.VectorWeight < 0 || q.VectorWeight > 1.0 {
		return false
	}
	if q.KeywordWeight < 0 || q.KeywordWeight > 1.0 {
		return false
	}
	// Weights should sum to approximately 1.0 (with small tolerance for float precision)
	sum := q.VectorWeight + q.KeywordWeight
	if sum < 0.99 || sum > 1.01 {
		return false
	}
	return true
}

func TestVectorSearchResult_Creation(t *testing.T) {
	hits := []VectorHit{
		{
			Document: &VectorDocument{
				ObjectID: "doc-1",
				Title:    "Document 1",
			},
			Score:         0.95,
			MatchedChunks: []int{0, 1},
		},
		{
			Document: &VectorDocument{
				ObjectID: "doc-2",
				Title:    "Document 2",
			},
			Score:         0.85,
			MatchedChunks: []int{2},
		},
	}

	result := &VectorSearchResult{
		Hits:  hits,
		Total: len(hits),
		Took:  50 * time.Millisecond,
	}

	if len(result.Hits) != 2 {
		t.Errorf("Hits length = %d, want 2", len(result.Hits))
	}
	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
	if result.Took != 50*time.Millisecond {
		t.Errorf("Took = %v, want 50ms", result.Took)
	}
}

func TestVectorSearchResult_ScoreOrdering(t *testing.T) {
	hits := []VectorHit{
		{Document: &VectorDocument{ObjectID: "doc-1"}, Score: 0.95},
		{Document: &VectorDocument{ObjectID: "doc-2"}, Score: 0.85},
		{Document: &VectorDocument{ObjectID: "doc-3"}, Score: 0.75},
	}

	// Verify hits are in descending score order
	for i := 1; i < len(hits); i++ {
		if hits[i].Score > hits[i-1].Score {
			t.Errorf("hit %d has higher score than hit %d: %.2f > %.2f",
				i, i-1, hits[i].Score, hits[i-1].Score)
		}
	}
}

func TestVectorHit_ScoreRange(t *testing.T) {
	tests := []struct {
		name      string
		score     float64
		wantValid bool
	}{
		{"perfect match", 1.0, true},
		{"high similarity", 0.9, true},
		{"medium similarity", 0.5, true},
		{"low similarity", 0.1, true},
		{"zero similarity", 0.0, true},
		{"negative score", -0.1, false},
		{"above 1.0", 1.1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit := VectorHit{
				Document: &VectorDocument{ObjectID: "test-doc"},
				Score:    tt.score,
			}

			valid := hit.Score >= 0.0 && hit.Score <= 1.0
			if valid != tt.wantValid {
				t.Errorf("score %.2f validity = %v, want %v", hit.Score, valid, tt.wantValid)
			}
		})
	}
}

func TestVectorDocument_EmbeddingDimensions(t *testing.T) {
	tests := []struct {
		name       string
		embedding  []float32
		dimensions int
		wantMatch  bool
	}{
		{
			name:       "matching dimensions",
			embedding:  make([]float32, 1024),
			dimensions: 1024,
			wantMatch:  true,
		},
		{
			name:       "mismatched dimensions",
			embedding:  make([]float32, 512),
			dimensions: 1024,
			wantMatch:  false,
		},
		{
			name:       "empty embedding",
			embedding:  []float32{},
			dimensions: 0,
			wantMatch:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &VectorDocument{
				ObjectID:         "test-doc",
				ContentEmbedding: tt.embedding,
				Dimensions:       tt.dimensions,
			}

			match := len(doc.ContentEmbedding) == doc.Dimensions
			if match != tt.wantMatch {
				t.Errorf("dimension match = %v, want %v (embedding len=%d, dimensions=%d)",
					match, tt.wantMatch, len(doc.ContentEmbedding), doc.Dimensions)
			}
		})
	}
}

func TestHybridSearchQuery_Creation(t *testing.T) {
	query := &HybridSearchQuery{
		QueryText:      "infrastructure best practices",
		QueryEmbedding: []float32{0.1, 0.2, 0.3},
		VectorWeight:   0.7,
		KeywordWeight:  0.3,
		Limit:          20,
		Filters: map[string]interface{}{
			"docType": "RFC",
			"status":  "Approved",
		},
	}

	if query.QueryText != "infrastructure best practices" {
		t.Errorf("QueryText = %q", query.QueryText)
	}
	if len(query.QueryEmbedding) != 3 {
		t.Errorf("QueryEmbedding length = %d, want 3", len(query.QueryEmbedding))
	}
	if query.VectorWeight+query.KeywordWeight != 1.0 {
		t.Errorf("weights sum = %.2f, want 1.0", query.VectorWeight+query.KeywordWeight)
	}
}

func TestVectorDocument_ModelInfo(t *testing.T) {
	models := []struct {
		name       string
		dimensions int
	}{
		{"amazon.titan-embed-text-v2", 1024},
		{"amazon.titan-embed-text-v1", 1536},
		{"text-embedding-ada-002", 1536},
		{"text-embedding-3-small", 1536},
		{"text-embedding-3-large", 3072},
	}

	for _, m := range models {
		t.Run(m.name, func(t *testing.T) {
			doc := &VectorDocument{
				ObjectID:         "test-doc",
				Model:            m.name,
				Dimensions:       m.dimensions,
				ContentEmbedding: make([]float32, m.dimensions),
			}

			if doc.Model != m.name {
				t.Errorf("Model = %q, want %q", doc.Model, m.name)
			}
			if doc.Dimensions != m.dimensions {
				t.Errorf("Dimensions = %d, want %d", doc.Dimensions, m.dimensions)
			}
			if len(doc.ContentEmbedding) != m.dimensions {
				t.Errorf("embedding length = %d, want %d", len(doc.ContentEmbedding), m.dimensions)
			}
		})
	}
}

func TestChunkEmbedding_PositionValidation(t *testing.T) {
	tests := []struct {
		name     string
		startPos int
		endPos   int
		wantOk   bool
	}{
		{"valid range", 0, 100, true},
		{"single char", 0, 1, true},
		{"same position", 10, 10, false},
		{"reversed range", 100, 0, false},
		{"negative start", -1, 100, false},
		{"negative end", 0, -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk := ChunkEmbedding{
				Text:      "test",
				StartPos:  tt.startPos,
				EndPos:    tt.endPos,
				Embedding: []float32{0.1},
			}

			valid := chunk.StartPos >= 0 && chunk.EndPos > chunk.StartPos
			if valid != tt.wantOk {
				t.Errorf("position validity = %v, want %v (start=%d, end=%d)",
					valid, tt.wantOk, tt.startPos, tt.endPos)
			}
		})
	}
}

func TestVectorSearchQuery_WithFilters(t *testing.T) {
	query := &VectorSearchQuery{
		QueryEmbedding: []float32{0.1, 0.2},
		Limit:          10,
		Threshold:      0.7,
		Filters: map[string]interface{}{
			"docType":  "RFC",
			"status":   []string{"Approved", "In-Review"},
			"modified": map[string]int64{"gte": 1234567890},
		},
	}

	if len(query.Filters) != 3 {
		t.Errorf("Filters length = %d, want 3", len(query.Filters))
	}

	docType, ok := query.Filters["docType"].(string)
	if !ok || docType != "RFC" {
		t.Errorf("docType filter = %v, want RFC", docType)
	}

	status, ok := query.Filters["status"].([]string)
	if !ok || len(status) != 2 {
		t.Errorf("status filter = %v, want 2 values", status)
	}
}

func TestVectorHit_MatchedChunks(t *testing.T) {
	hit := VectorHit{
		Document: &VectorDocument{
			ObjectID: "doc-with-chunks",
			ChunkEmbeddings: []ChunkEmbedding{
				{ChunkIndex: 0, Text: "chunk 0"},
				{ChunkIndex: 1, Text: "chunk 1"},
				{ChunkIndex: 2, Text: "chunk 2"},
				{ChunkIndex: 3, Text: "chunk 3"},
			},
		},
		Score:         0.88,
		MatchedChunks: []int{1, 3},
	}

	if len(hit.MatchedChunks) != 2 {
		t.Errorf("MatchedChunks length = %d, want 2", len(hit.MatchedChunks))
	}

	// Verify matched chunk indices exist in document
	for _, idx := range hit.MatchedChunks {
		if idx < 0 || idx >= len(hit.Document.ChunkEmbeddings) {
			t.Errorf("matched chunk index %d is out of range [0, %d)",
				idx, len(hit.Document.ChunkEmbeddings))
		}
	}
}

func TestVectorDocument_TimestampFields(t *testing.T) {
	modifiedAt := time.Date(2025, 11, 1, 12, 0, 0, 0, time.UTC)
	embeddedAt := time.Date(2025, 11, 2, 14, 30, 0, 0, time.UTC)

	doc := &VectorDocument{
		ObjectID:   "doc-with-times",
		ModifiedAt: modifiedAt,
		EmbeddedAt: embeddedAt,
	}

	if !doc.ModifiedAt.Equal(modifiedAt) {
		t.Errorf("ModifiedAt = %v, want %v", doc.ModifiedAt, modifiedAt)
	}
	if !doc.EmbeddedAt.Equal(embeddedAt) {
		t.Errorf("EmbeddedAt = %v, want %v", doc.EmbeddedAt, embeddedAt)
	}
	if !doc.EmbeddedAt.After(doc.ModifiedAt) {
		t.Error("EmbeddedAt should be after ModifiedAt")
	}
}
