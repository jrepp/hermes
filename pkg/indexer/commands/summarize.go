package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/ai"
	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

// SummarizeCommand uses AI to generate document summaries and analysis.
// It supports caching to avoid redundant API calls and respects cost limits.
type SummarizeCommand struct {
	AIProvider       ai.Provider
	DB               *gorm.DB // Optional: for caching summaries
	Logger           hclog.Logger
	MaxAge           time.Duration // How long to cache summaries (0 = always regenerate)
	MinContentLength int           // Minimum content length to summarize
	ExtractTopics    bool
	ExtractKeyPoints bool
	SuggestTags      bool
	AnalyzeStatus    bool
}

// Name returns the command name.
func (c *SummarizeCommand) Name() string {
	return "summarize"
}

// Execute generates an AI summary for the document.
func (c *SummarizeCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
	if c.Logger == nil {
		c.Logger = hclog.NewNullLogger()
	}

	if c.AIProvider == nil {
		return fmt.Errorf("AI provider is required")
	}

	// Skip if document is too short
	if doc.Content == "" {
		c.Logger.Debug("skipping summarization: no content",
			"document_id", doc.Document.ID,
		)
		return nil
	}

	if c.MinContentLength > 0 && len(doc.Content) < c.MinContentLength {
		c.Logger.Debug("skipping summarization: content too short",
			"document_id", doc.Document.ID,
			"content_length", len(doc.Content),
			"min_length", c.MinContentLength,
		)
		return nil
	}

	// Check for cached summary if database is available
	if c.DB != nil && c.MaxAge > 0 {
		cached, err := c.loadCachedSummary(doc)
		if err == nil && cached != nil {
			c.Logger.Debug("using cached summary",
				"document_id", doc.Document.ID,
				"age", time.Since(cached.GeneratedAt),
			)
			doc.SetCustom("ai_summary", cached)
			return nil
		}
	}

	// Build summarization request
	req := &ai.SummarizeRequest{
		Content:          doc.Content,
		Title:            doc.Document.Name,
		MaxSummaryLength: 0, // Use provider default
		ExtractTopics:    c.ExtractTopics,
		ExtractKeyPoints: c.ExtractKeyPoints,
		SuggestTags:      c.SuggestTags,
		AnalyzeStatus:    c.AnalyzeStatus,
	}

	// Get document type if available
	if doc.Metadata != nil && doc.Metadata.DocumentType.Name != "" {
		req.DocType = doc.Metadata.DocumentType.Name
	}

	c.Logger.Info("generating AI summary",
		"document_id", doc.Document.ID,
		"title", doc.Document.Name,
		"content_length", len(doc.Content),
		"provider", c.AIProvider.Name(),
	)

	// Generate summary
	startTime := time.Now()
	resp, err := c.AIProvider.Summarize(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}
	generationTime := time.Since(startTime)

	// Set document ID on summary
	resp.Summary.DocumentID = doc.Document.ID

	c.Logger.Info("AI summary generated",
		"document_id", doc.Document.ID,
		"model", resp.Model,
		"tokens_used", resp.TokensUsed,
		"generation_time", generationTime,
	)

	// Store summary in context
	doc.SetCustom("ai_summary", resp.Summary)

	// Store in database if available
	if c.DB != nil {
		if err := c.storeSummary(doc, resp, generationTime); err != nil {
			c.Logger.Warn("failed to store summary in database",
				"document_id", doc.Document.ID,
				"error", err,
			)
			// Don't fail the command if storage fails
		}
	}

	return nil
}

// loadCachedSummary attempts to load a cached summary from the database.
func (c *SummarizeCommand) loadCachedSummary(doc *indexer.DocumentContext) (*ai.DocumentSummary, error) {
	var dbSummary models.DocumentSummary
	err := c.DB.Where("document_id = ?", doc.Document.ID).
		Order("generated_at DESC").
		First(&dbSummary).Error

	if err != nil {
		return nil, err
	}

	// Check if cached summary is too old
	if c.MaxAge > 0 && dbSummary.IsStale(c.MaxAge) {
		return nil, fmt.Errorf("cached summary is stale")
	}

	// Check if content has changed (if hash available)
	if doc.ContentHash != "" && !dbSummary.MatchesContentHash(doc.ContentHash) {
		return nil, fmt.Errorf("content has changed since summary was generated")
	}

	// Convert database model to AI summary
	aiSummary := &ai.DocumentSummary{
		DocumentID:       dbSummary.DocumentID,
		ExecutiveSummary: dbSummary.ExecutiveSummary,
		KeyPoints:        []string(dbSummary.KeyPoints),
		Topics:           []string(dbSummary.Topics),
		Tags:             []string(dbSummary.Tags),
		SuggestedStatus:  dbSummary.SuggestedStatus,
		GeneratedAt:      dbSummary.GeneratedAt,
		Model:            dbSummary.Model,
		TokensUsed:       0,
	}

	if dbSummary.Confidence != nil {
		aiSummary.Confidence = *dbSummary.Confidence
	}

	if dbSummary.TokensUsed != nil {
		aiSummary.TokensUsed = *dbSummary.TokensUsed
	}

	return aiSummary, nil
}

// storeSummary stores the AI summary in the database.
func (c *SummarizeCommand) storeSummary(doc *indexer.DocumentContext, resp *ai.SummarizeResponse, generationTime time.Duration) error {
	generationTimeMs := int(generationTime.Milliseconds())
	tokensUsed := resp.TokensUsed
	contentLength := len(doc.Content)
	confidence := resp.Summary.Confidence

	dbSummary := &models.DocumentSummary{
		DocumentID:       doc.Document.ID,
		ExecutiveSummary: resp.Summary.ExecutiveSummary,
		KeyPoints:        models.StringArray(resp.Summary.KeyPoints),
		Topics:           models.StringArray(resp.Summary.Topics),
		Tags:             models.StringArray(resp.Summary.Tags),
		SuggestedStatus:  resp.Summary.SuggestedStatus,
		Confidence:       &confidence,
		Model:            resp.Model,
		Provider:         c.AIProvider.Name(),
		TokensUsed:       &tokensUsed,
		GenerationTimeMs: &generationTimeMs,
		DocumentTitle:    doc.Document.Name,
		ContentHash:      doc.ContentHash,
		ContentLength:    &contentLength,
		GeneratedAt:      resp.Summary.GeneratedAt,
	}

	// Set document UUID if available
	if doc.DocumentUUID.String() != "00000000-0000-0000-0000-000000000000" {
		dbSummary.DocumentUUID = &doc.DocumentUUID
	}

	// Set document type if available
	if doc.Metadata != nil && doc.Metadata.DocumentType.Name != "" {
		dbSummary.DocumentType = doc.Metadata.DocumentType.Name
	}

	return c.DB.Create(dbSummary).Error
}

// ExecuteBatch implements BatchCommand for parallel summarization.
func (c *SummarizeCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	// AI operations are expensive, limit concurrency
	return indexer.ParallelProcess(ctx, docs, c.Execute, 2)
}
