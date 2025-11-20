// Package ollama provides an Ollama implementation of the AI provider interface.
// Ollama runs Llama and other open-source models locally on macOS, Linux, and Windows.
// Install: https://ollama.ai/download
// Usage: ollama pull llama3.2 && ollama pull nomic-embed-text
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/ai"
)

// Config contains Ollama configuration.
type Config struct {
	BaseURL        string // Ollama API URL (default: http://localhost:11434)
	SummarizeModel string // Model for summarization (e.g., "llama3.2", "llama3.1")
	EmbeddingModel string // Model for embeddings (e.g., "nomic-embed-text", "mxbai-embed-large")
	Timeout        time.Duration
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() *Config {
	return &Config{
		BaseURL:        "http://localhost:11434",
		SummarizeModel: "llama3.2",         // Llama 3.2 (3B) - good balance of speed/quality
		EmbeddingModel: "nomic-embed-text", // 768-dim embeddings, optimized for semantic search
		Timeout:        5 * time.Minute,
	}
}

// Provider implements ai.Provider using Ollama.
type Provider struct {
	cfg    *Config
	client *http.Client
}

// NewProvider creates a new Ollama AI provider.
func NewProvider(cfg *Config) (*Provider, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	return &Provider{
		cfg:    cfg,
		client: client,
	}, nil
}

// Summarize uses Llama to generate document summaries.
func (p *Provider) Summarize(ctx context.Context, req *ai.SummarizeRequest) (*ai.SummarizeResponse, error) {
	// Build prompt for Llama
	prompt := p.buildSummarizePrompt(req)

	// Call Ollama generate API
	ollamaReq := map[string]interface{}{
		"model":  p.cfg.SummarizeModel,
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.7,
			"top_p":       0.9,
		},
	}

	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		p.cfg.BaseURL+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("ollama returned status %d (unable to read body: %w)", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp struct {
		Response string `json:"response"`
		Context  []int  `json:"context"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Parse the JSON response from Llama
	summary, err := p.parseSummarizeResponse(ollamaResp.Response, req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse summary: %w", err)
	}

	// Estimate tokens (rough approximation)
	tokensUsed := (len(req.Content) + len(ollamaResp.Response)) / 4

	return &ai.SummarizeResponse{
		Summary:    summary,
		Model:      p.cfg.SummarizeModel,
		TokensUsed: tokensUsed,
	}, nil
}

// GenerateEmbedding uses Ollama to create vector embeddings.
func (p *Provider) GenerateEmbedding(ctx context.Context, req *ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
	embeddings := &ai.DocumentEmbeddings{
		Model:       p.cfg.EmbeddingModel,
		GeneratedAt: time.Now(),
	}

	// If chunking requested, process chunks
	if req.ChunkSize > 0 && len(req.Texts) > 0 {
		chunks := p.chunkTexts(req.Texts, req.ChunkSize, req.ChunkOverlap)
		embeddings.Chunks = make([]ai.ChunkEmbedding, 0, len(chunks))

		pos := 0
		for i, chunk := range chunks {
			embedding, err := p.generateSingleEmbedding(ctx, chunk)
			if err != nil {
				return nil, fmt.Errorf("failed to generate embedding for chunk %d: %w", i, err)
			}

			if embeddings.Dimensions == 0 {
				embeddings.Dimensions = len(embedding)
			}

			embeddings.Chunks = append(embeddings.Chunks, ai.ChunkEmbedding{
				ChunkIndex: i,
				StartPos:   pos,
				EndPos:     pos + len(chunk),
				Text:       chunk,
				Embedding:  embedding,
			})
			pos += len(chunk)
		}

		// Use first chunk as content embedding
		if len(embeddings.Chunks) > 0 {
			embeddings.ContentEmbedding = embeddings.Chunks[0].Embedding
		}
	} else if len(req.Texts) > 0 {
		// Generate single embedding for combined text
		text := strings.Join(req.Texts, " ")
		embedding, err := p.generateSingleEmbedding(ctx, text)
		if err != nil {
			return nil, err
		}

		embeddings.ContentEmbedding = embedding
		embeddings.Dimensions = len(embedding)
	}

	return &ai.EmbeddingResponse{
		Embeddings: embeddings,
		Model:      p.cfg.EmbeddingModel,
		Dimensions: embeddings.Dimensions,
		TokensUsed: 0, // Ollama doesn't report token usage for embeddings
	}, nil
}

// generateSingleEmbedding generates an embedding for a single text.
func (p *Provider) generateSingleEmbedding(ctx context.Context, text string) ([]float32, error) {
	ollamaReq := map[string]interface{}{
		"model":  p.cfg.EmbeddingModel,
		"prompt": text,
	}

	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		p.cfg.BaseURL+"/api/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("ollama returned status %d (unable to read body: %w)", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp struct {
		Embedding []float64 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert float64 to float32
	embedding := make([]float32, len(ollamaResp.Embedding))
	for i, v := range ollamaResp.Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "ollama"
}

// buildSummarizePrompt creates a Llama-optimized prompt.
func (p *Provider) buildSummarizePrompt(req *ai.SummarizeRequest) string {
	var builder strings.Builder

	builder.WriteString("You are analyzing a document. Please provide a structured analysis.\n\n")

	if req.DocType != "" {
		builder.WriteString(fmt.Sprintf("Document Type: %s\n", req.DocType))
	}
	if req.Title != "" {
		builder.WriteString(fmt.Sprintf("Document Title: %s\n", req.Title))
	}

	builder.WriteString("\nPlease provide:\n")
	builder.WriteString("1. A concise executive summary (2-3 sentences)\n")

	if req.ExtractKeyPoints {
		builder.WriteString("2. 3-5 key points or takeaways\n")
	}
	if req.ExtractTopics {
		builder.WriteString("3. Main topics covered\n")
	}
	if req.SuggestTags {
		builder.WriteString("4. Suggested tags for categorization\n")
	}
	if req.AnalyzeStatus {
		builder.WriteString("5. Recommended document status (Draft, In Review, or Approved)\n")
	}

	builder.WriteString("\nDocument content:\n")
	builder.WriteString(req.Content)
	builder.WriteString("\n\nRespond ONLY with valid JSON in this exact format:\n")
	builder.WriteString("{\n")
	builder.WriteString("  \"executive_summary\": \"...\",\n")
	builder.WriteString("  \"key_points\": [\"...\", \"...\"],\n")
	builder.WriteString("  \"topics\": [\"...\", \"...\"],\n")
	builder.WriteString("  \"suggested_tags\": [\"...\", \"...\"],\n")
	builder.WriteString("  \"suggested_status\": \"...\",\n")
	builder.WriteString("  \"confidence\": 0.85\n")
	builder.WriteString("}")

	return builder.String()
}

// parseSummarizeResponse parses Llama's JSON response.
func (p *Provider) parseSummarizeResponse(responseText string, req *ai.SummarizeRequest) (*ai.DocumentSummary, error) {
	// Try to extract JSON from the response (Llama might add explanatory text)
	jsonStart := strings.Index(responseText, "{")
	jsonEnd := strings.LastIndex(responseText, "}")

	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonText := responseText[jsonStart : jsonEnd+1]

	var response struct {
		ExecutiveSummary string   `json:"executive_summary"`
		KeyPoints        []string `json:"key_points"`
		Topics           []string `json:"topics"`
		SuggestedTags    []string `json:"suggested_tags"`
		SuggestedStatus  string   `json:"suggested_status"`
		Confidence       float64  `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(jsonText), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	summary := &ai.DocumentSummary{
		ExecutiveSummary: response.ExecutiveSummary,
		KeyPoints:        response.KeyPoints,
		Topics:           response.Topics,
		Tags:             response.SuggestedTags,
		SuggestedStatus:  response.SuggestedStatus,
		Confidence:       response.Confidence,
		GeneratedAt:      time.Now(),
		Model:            p.cfg.SummarizeModel,
	}

	return summary, nil
}

// chunkTexts splits texts into chunks (simple word-based implementation).
func (p *Provider) chunkTexts(texts []string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		return texts
	}

	chunks := make([]string, 0)
	for _, text := range texts {
		words := strings.Fields(text)

		for i := 0; i < len(words); i += chunkSize - overlap {
			end := i + chunkSize
			if end > len(words) {
				end = len(words)
			}
			chunk := strings.Join(words[i:end], " ")
			chunks = append(chunks, chunk)

			if end >= len(words) {
				break
			}
		}
	}

	return chunks
}
