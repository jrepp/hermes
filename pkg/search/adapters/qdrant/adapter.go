// Package qdrant provides an opt-in local vector search adapter.
package qdrant

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/search"
)

const (
	defaultCollection = "hermes_vectors"
	defaultDistance   = "Cosine"
	defaultTimeout    = 2 * time.Second
)

// Config configures the Qdrant adapter.
type Config struct {
	BaseURL    string
	APIKey     string
	Collection string
	Distance   string
	Timeout    time.Duration
	Dimensions int
	HTTPClient *http.Client
}

// Adapter is a Qdrant-backed vector index.
type Adapter struct {
	baseURL    string
	apiKey     string
	collection string
	distance   string
	dimensions int
	client     *http.Client
}

var _ search.VectorIndex = (*Adapter)(nil)

// Name returns the provider name for diagnostics.
func (a *Adapter) Name() string {
	return string(search.ProviderTypeQdrant)
}

// New creates an adapter.
func New(cfg Config) (*Adapter, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("qdrant base URL is required")
	}
	if cfg.Collection == "" {
		cfg.Collection = defaultCollection
	}
	if cfg.Distance == "" {
		cfg.Distance = defaultDistance
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	return &Adapter{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:     cfg.APIKey,
		collection: cfg.Collection,
		distance:   cfg.Distance,
		dimensions: cfg.Dimensions,
		client:     client,
	}, nil
}

// Healthy verifies the Qdrant service endpoint is reachable.
func (a *Adapter) Healthy(ctx context.Context) error {
	var out map[string]any
	if err := a.do(ctx, http.MethodGet, "/healthz", nil, &out); err != nil {
		return &search.Error{Op: "qdrant.health", Err: search.ErrBackendUnavailable, Msg: err.Error()}
	}
	return nil
}

// EnsureCollection creates or updates the configured Qdrant collection.
func (a *Adapter) EnsureCollection(ctx context.Context, dimensions int) error {
	if dimensions <= 0 {
		return &search.Error{Op: "qdrant.ensure_collection", Err: search.ErrInvalidQuery, Msg: "dimensions must be positive"}
	}
	body := map[string]any{
		"vectors": map[string]any{
			"size":     dimensions,
			"distance": a.distance,
		},
	}
	if err := a.do(ctx, http.MethodPut, "/collections/"+a.collection, body, nil); err != nil {
		return &search.Error{Op: "qdrant.ensure_collection", Err: search.ErrBackendUnavailable, Msg: err.Error()}
	}
	a.dimensions = dimensions
	return nil
}

// IndexEmbedding stores a document's vector embedding.
func (a *Adapter) IndexEmbedding(ctx context.Context, doc *search.VectorDocument) error {
	if doc == nil {
		return &search.Error{Op: "qdrant.index", Err: search.ErrInvalidQuery, Msg: "document is required"}
	}
	return a.IndexEmbeddingBatch(ctx, []*search.VectorDocument{doc})
}

// IndexEmbeddingBatch stores multiple document embeddings.
func (a *Adapter) IndexEmbeddingBatch(ctx context.Context, docs []*search.VectorDocument) error {
	points := make([]qdrantPoint, 0, len(docs))
	for _, doc := range docs {
		point, err := a.pointFromDocument(doc)
		if err != nil {
			return err
		}
		points = append(points, point)
	}
	if len(points) == 0 {
		return nil
	}
	body := map[string]any{"points": points}
	if err := a.do(ctx, http.MethodPut, "/collections/"+a.collection+"/points?wait=true", body, nil); err != nil {
		return &search.Error{Op: "qdrant.index", Err: search.ErrIndexingFailed, Msg: err.Error()}
	}
	return nil
}

// SearchSimilar finds documents similar to the query embedding.
func (a *Adapter) SearchSimilar(ctx context.Context, query *search.VectorSearchQuery) (*search.VectorSearchResult, error) {
	if query == nil || len(query.QueryEmbedding) == 0 {
		return nil, &search.Error{Op: "qdrant.search", Err: search.ErrInvalidQuery, Msg: "query embedding is required"}
	}
	if err := a.validateDimensions(len(query.QueryEmbedding)); err != nil {
		return nil, err
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}
	body := map[string]any{
		"vector":       query.QueryEmbedding,
		"limit":        limit,
		"with_payload": true,
		"with_vector":  true,
	}
	if query.Threshold > 0 {
		body["score_threshold"] = query.Threshold
	}
	if len(query.Filters) > 0 {
		body["filter"] = filterFromMap(query.Filters)
	}

	started := time.Now()
	var resp qdrantSearchResponse
	if err := a.do(ctx, http.MethodPost, "/collections/"+a.collection+"/points/search", body, &resp); err != nil {
		return nil, &search.Error{Op: "qdrant.search", Err: search.ErrBackendUnavailable, Msg: err.Error()}
	}
	return &search.VectorSearchResult{Hits: hitsFromQdrant(resp.Result), Total: len(resp.Result), Took: time.Since(started)}, nil
}

// SearchHybrid is intentionally left to a higher-level fusion layer.
func (a *Adapter) SearchHybrid(_ context.Context, _ *search.HybridSearchQuery) (*search.SearchResult, error) {
	return nil, &search.Error{Op: "qdrant.hybrid", Err: search.ErrInvalidQuery, Msg: "hybrid search requires a lexical search provider"}
}

// Delete removes a document's embeddings.
func (a *Adapter) Delete(ctx context.Context, docID string) error {
	if docID == "" {
		return &search.Error{Op: "qdrant.delete", Err: search.ErrInvalidQuery, Msg: "document id is required"}
	}
	body := map[string]any{"points": []string{pointID(docID)}}
	if err := a.do(ctx, http.MethodPost, "/collections/"+a.collection+"/points/delete?wait=true", body, nil); err != nil {
		return &search.Error{Op: "qdrant.delete", Err: search.ErrBackendUnavailable, Msg: err.Error()}
	}
	return nil
}

// DeleteBatch removes multiple documents' embeddings.
func (a *Adapter) DeleteBatch(ctx context.Context, docIDs []string) error {
	points := make([]string, 0, len(docIDs))
	for _, docID := range docIDs {
		if docID != "" {
			points = append(points, pointID(docID))
		}
	}
	if len(points) == 0 {
		return nil
	}
	body := map[string]any{"points": points}
	if err := a.do(ctx, http.MethodPost, "/collections/"+a.collection+"/points/delete?wait=true", body, nil); err != nil {
		return &search.Error{Op: "qdrant.delete_batch", Err: search.ErrBackendUnavailable, Msg: err.Error()}
	}
	return nil
}

// GetEmbedding retrieves stored embedding for a document.
func (a *Adapter) GetEmbedding(ctx context.Context, docID string) (*search.VectorDocument, error) {
	if docID == "" {
		return nil, &search.Error{Op: "qdrant.get", Err: search.ErrInvalidQuery, Msg: "document id is required"}
	}
	body := map[string]any{"ids": []string{pointID(docID)}, "with_payload": true, "with_vector": true}
	var resp qdrantRetrieveResponse
	if err := a.do(ctx, http.MethodPost, "/collections/"+a.collection+"/points", body, &resp); err != nil {
		return nil, &search.Error{Op: "qdrant.get", Err: search.ErrBackendUnavailable, Msg: err.Error()}
	}
	if len(resp.Result) == 0 {
		return nil, &search.Error{Op: "qdrant.get", Err: search.ErrNotFound, Msg: docID}
	}
	return vectorDocumentFromPoint(resp.Result[0]), nil
}

// Clear removes all vector data by deleting the configured collection.
func (a *Adapter) Clear(ctx context.Context) error {
	if err := a.do(ctx, http.MethodDelete, "/collections/"+a.collection, nil, nil); err != nil {
		return &search.Error{Op: "qdrant.clear", Err: search.ErrBackendUnavailable, Msg: err.Error()}
	}
	return nil
}

type qdrantPoint struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload"`
}

type qdrantPointResult struct {
	ID      string         `json:"id"`
	Score   float64        `json:"score,omitempty"`
	Vector  []float32      `json:"vector,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

type qdrantSearchResponse struct {
	Result []qdrantPointResult `json:"result"`
}

type qdrantRetrieveResponse struct {
	Result []qdrantPointResult `json:"result"`
}

func (a *Adapter) pointFromDocument(doc *search.VectorDocument) (qdrantPoint, error) {
	if doc == nil {
		return qdrantPoint{}, &search.Error{Op: "qdrant.point", Err: search.ErrInvalidQuery, Msg: "document is required"}
	}
	docID := doc.DocID
	if docID == "" {
		docID = doc.ObjectID
	}
	if docID == "" {
		return qdrantPoint{}, &search.Error{Op: "qdrant.point", Err: search.ErrInvalidQuery, Msg: "document id is required"}
	}
	if len(doc.ContentEmbedding) == 0 {
		return qdrantPoint{}, &search.Error{Op: "qdrant.point", Err: search.ErrInvalidQuery, Msg: "content embedding is required"}
	}
	if err := a.validateDimensions(len(doc.ContentEmbedding)); err != nil {
		return qdrantPoint{}, err
	}
	dimensions := doc.Dimensions
	if dimensions == 0 {
		dimensions = len(doc.ContentEmbedding)
	}
	return qdrantPoint{ID: pointID(docID), Vector: doc.ContentEmbedding, Payload: map[string]any{
		"doc_id":       docID,
		"object_id":    doc.ObjectID,
		"title":        doc.Title,
		"doc_type":     doc.DocType,
		"summary":      doc.Summary,
		"model":        doc.Model,
		"dimensions":   dimensions,
		"modified_at":  doc.ModifiedAt.Format(time.RFC3339Nano),
		"embedded_at":  doc.EmbeddedAt.Format(time.RFC3339Nano),
		"topics":       doc.Topics,
		"tags":         doc.Tags,
		"key_points":   doc.KeyPoints,
		"chunk_count":  len(doc.ChunkEmbeddings),
		"original_key": docID,
	}}, nil
}

func (a *Adapter) validateDimensions(got int) error {
	if a.dimensions > 0 && got != a.dimensions {
		return &search.Error{Op: "qdrant.dimensions", Err: search.ErrInvalidQuery, Msg: fmt.Sprintf("embedding dimensions mismatch: got %d, want %d", got, a.dimensions)}
	}
	return nil
}

func (a *Adapter) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a.apiKey != "" {
		req.Header.Set("api-key", a.apiKey)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("qdrant returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func pointID(docID string) string {
	sum := sha256.Sum256([]byte(docID))
	buf := append([]byte(nil), sum[:16]...)
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	hexed := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexed[0:8], hexed[8:12], hexed[12:16], hexed[16:20], hexed[20:32])
}

func filterFromMap(filters map[string]any) map[string]any {
	must := make([]map[string]any, 0, len(filters))
	for key, value := range filters {
		must = append(must, map[string]any{"key": key, "match": map[string]any{"value": value}})
	}
	return map[string]any{"must": must}
}

func hitsFromQdrant(points []qdrantPointResult) []search.VectorHit {
	hits := make([]search.VectorHit, 0, len(points))
	for _, point := range points {
		hits = append(hits, search.VectorHit{Document: vectorDocumentFromPoint(point), Score: point.Score})
	}
	return hits
}

func vectorDocumentFromPoint(point qdrantPointResult) *search.VectorDocument {
	payload := point.Payload
	docID := stringPayload(payload, "doc_id")
	if docID == "" {
		docID = stringPayload(payload, "original_key")
	}
	return &search.VectorDocument{
		ObjectID:         stringPayload(payload, "object_id"),
		DocID:            docID,
		Title:            stringPayload(payload, "title"),
		DocType:          stringPayload(payload, "doc_type"),
		Summary:          stringPayload(payload, "summary"),
		Model:            stringPayload(payload, "model"),
		Dimensions:       intPayload(payload, "dimensions"),
		ModifiedAt:       timePayload(payload, "modified_at"),
		EmbeddedAt:       timePayload(payload, "embedded_at"),
		Topics:           stringSlicePayload(payload, "topics"),
		Tags:             stringSlicePayload(payload, "tags"),
		KeyPoints:        stringSlicePayload(payload, "key_points"),
		ContentEmbedding: point.Vector,
	}
}

func stringPayload(payload map[string]any, key string) string {
	if value, ok := payload[key].(string); ok {
		return value
	}
	return ""
}

func intPayload(payload map[string]any, key string) int {
	switch value := payload[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func timePayload(payload map[string]any, key string) time.Time {
	value := stringPayload(payload, key)
	if value == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

func stringSlicePayload(payload map[string]any, key string) []string {
	values, ok := payload[key].([]any)
	if !ok {
		if typed, ok := payload[key].([]string); ok {
			return typed
		}
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}
