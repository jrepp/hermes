package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/pkg/indexer"
)

// CalculateHashCommand calculates a SHA-256 hash of the document content.
// This hash is used for change detection and conflict resolution during migration.
// The command normalizes the content before hashing to ensure consistency.
type CalculateHashCommand struct {
	Logger hclog.Logger
}

// Name returns the command name.
func (c *CalculateHashCommand) Name() string {
	return "calculate-hash"
}

// Execute calculates the content hash for a document.
func (c *CalculateHashCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
	if c.Logger == nil {
		c.Logger = hclog.NewNullLogger()
	}

	// Ensure content is available
	if doc.Content == "" {
		return fmt.Errorf("document content is empty, cannot calculate hash")
	}

	// Normalize content for consistent hashing
	normalized := normalizeContent(doc.Content)

	// Calculate SHA-256 hash
	hash := sha256.Sum256([]byte(normalized))
	hashStr := hex.EncodeToString(hash[:])

	doc.ContentHash = hashStr

	c.Logger.Debug("calculated content hash",
		"document_id", doc.Document.ID,
		"name", doc.Document.Name,
		"hash", hashStr,
		"content_length", len(doc.Content),
	)

	return nil
}

// ExecuteBatch implements BatchCommand for parallel hash calculation.
func (c *CalculateHashCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	// Hash calculation is CPU-bound and can be done in parallel
	return indexer.ParallelProcess(ctx, docs, c.Execute, 10)
}

// normalizeContent normalizes text for consistent hashing.
// It removes extra whitespace and ensures consistent line endings.
func normalizeContent(content string) string {
	// Normalize line endings to \n
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	// Trim leading/trailing whitespace
	normalized = strings.TrimSpace(normalized)

	return normalized
}
