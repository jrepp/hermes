package mock

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/ai"
)

// Provider is a mock AI provider for testing.
// It generates predictable responses without calling external APIs.
type Provider struct {
	name           string
	simulateErrors bool
	delayMS        int
}

// NewProvider creates a new mock AI provider.
func NewProvider() *Provider {
	return &Provider{
		name: "mock",
	}
}

// WithName sets a custom name for the provider.
func (p *Provider) WithName(name string) *Provider {
	p.name = name
	return p
}

// WithSimulateErrors enables error simulation for testing error handling.
func (p *Provider) WithSimulateErrors(enable bool) *Provider {
	p.simulateErrors = enable
	return p
}

// WithDelay adds artificial delay to simulate API latency.
func (p *Provider) WithDelay(ms int) *Provider {
	p.delayMS = ms
	return p
}

// Summarize generates a mock summary.
func (p *Provider) Summarize(ctx context.Context, req *ai.SummarizeRequest) (*ai.SummarizeResponse, error) {
	if p.simulateErrors {
		return nil, fmt.Errorf("mock error: summarization failed")
	}

	if p.delayMS > 0 {
		time.Sleep(time.Duration(p.delayMS) * time.Millisecond)
	}

	// Generate mock summary based on content
	summary := &ai.DocumentSummary{
		DocumentID:       "", // Will be set by caller
		ExecutiveSummary: p.generateMockSummary(req.Content, req.Title),
		GeneratedAt:      time.Now(),
		Model:            "mock-v1",
		TokensUsed:       len(req.Content) / 4, // Rough estimate
		Confidence:       0.85,
	}

	if req.ExtractKeyPoints {
		summary.KeyPoints = p.generateMockKeyPoints(req.Content)
	}

	if req.ExtractTopics {
		summary.Topics = p.generateMockTopics(req.Content, req.DocType)
	}

	if req.SuggestTags {
		summary.Tags = p.generateMockTags(req.DocType)
	}

	if req.AnalyzeStatus {
		summary.SuggestedStatus = p.analyzeStatus(req.Content)
	}

	return &ai.SummarizeResponse{
		Summary:    summary,
		Model:      "mock-v1",
		TokensUsed: summary.TokensUsed,
	}, nil
}

// GenerateEmbedding generates mock embeddings.
func (p *Provider) GenerateEmbedding(ctx context.Context, req *ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
	if p.simulateErrors {
		return nil, fmt.Errorf("mock error: embedding generation failed")
	}

	if p.delayMS > 0 {
		time.Sleep(time.Duration(p.delayMS) * time.Millisecond)
	}

	dimensions := 1024 // Match AWS Titan dimensions

	embeddings := &ai.DocumentEmbeddings{
		DocumentID:       "", // Will be set by caller
		ContentEmbedding: p.generateMockEmbedding(dimensions),
		Model:            "mock-embed-v1",
		Dimensions:       dimensions,
		GeneratedAt:      time.Now(),
		TokensUsed:       0,
	}

	// Generate chunk embeddings if requested
	if req.ChunkSize > 0 && len(req.Texts) > 0 {
		embeddings.Chunks = make([]ai.ChunkEmbedding, 0)
		pos := 0
		chunkIndex := 0

		for _, text := range req.Texts {
			chunks := p.chunkText(text, req.ChunkSize, req.ChunkOverlap)
			for _, chunk := range chunks {
				embeddings.Chunks = append(embeddings.Chunks, ai.ChunkEmbedding{
					ChunkIndex: chunkIndex,
					StartPos:   pos,
					EndPos:     pos + len(chunk),
					Text:       chunk,
					Embedding:  p.generateMockEmbedding(dimensions),
				})
				pos += len(chunk)
				chunkIndex++
			}
		}

		embeddings.TokensUsed = len(embeddings.Chunks) * 100 // Mock token count
	}

	return &ai.EmbeddingResponse{
		Embeddings: embeddings,
		Model:      "mock-embed-v1",
		Dimensions: dimensions,
		TokensUsed: embeddings.TokensUsed,
	}, nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return p.name
}

// generateMockSummary creates a simple mock summary.
func (p *Provider) generateMockSummary(content, title string) string {
	if title != "" {
		return fmt.Sprintf("This is a mock summary of '%s'. The document contains %d characters of content.",
			title, len(content))
	}
	return fmt.Sprintf("This is a mock summary. The document contains %d characters of content.", len(content))
}

// generateMockKeyPoints creates mock key points.
func (p *Provider) generateMockKeyPoints(content string) []string {
	return []string{
		"Mock key point 1: Document overview",
		"Mock key point 2: Main findings",
		"Mock key point 3: Recommendations",
	}
}

// generateMockTopics creates mock topics based on document type.
func (p *Provider) generateMockTopics(content, docType string) []string {
	topics := []string{"documentation", "analysis"}
	if docType != "" {
		topics = append(topics, strings.ToLower(docType))
	}
	return topics
}

// generateMockTags creates mock tags.
func (p *Provider) generateMockTags(docType string) []string {
	tags := []string{"mock", "automated"}
	if docType != "" {
		tags = append(tags, strings.ToLower(docType))
	}
	return tags
}

// analyzeStatus provides a mock status analysis.
func (p *Provider) analyzeStatus(content string) string {
	// Simple heuristic based on content length
	if len(content) < 500 {
		return "Draft"
	} else if len(content) < 2000 {
		return "In Review"
	}
	return "Approved"
}

// generateMockEmbedding creates a deterministic mock embedding vector.
func (p *Provider) generateMockEmbedding(dimensions int) []float32 {
	embedding := make([]float32, dimensions)
	// Generate deterministic but varied values
	for i := range embedding {
		// Use a simple pattern that varies across dimensions
		embedding[i] = float32(i%100) / 100.0
	}
	return embedding
}

// chunkText splits text into chunks (simple mock implementation).
func (p *Provider) chunkText(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		return []string{text}
	}

	chunks := make([]string, 0)
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

	return chunks
}
