package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

// DetectConflictsCommand detects conflicts between different versions of a document
// across providers. Conflicts occur when the same document (identified by UUID)
// has different content hashes in different providers.
type DetectConflictsCommand struct {
	DB     *gorm.DB
	Logger hclog.Logger
}

// Name returns the command name.
func (c *DetectConflictsCommand) Name() string {
	return "detect-conflicts"
}

// Execute detects conflicts for a document.
func (c *DetectConflictsCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
	if c.Logger == nil {
		c.Logger = hclog.NewNullLogger()
	}

	if c.DB == nil {
		return fmt.Errorf("database connection is required")
	}

	// Skip if no revision tracking
	if doc.Revision == nil {
		c.Logger.Debug("no revision info, skipping conflict detection",
			"document_id", doc.Document.ID,
		)
		return nil
	}

	// Find all active revisions for this document UUID
	var allRevisions []models.DocumentRevision
	err := c.DB.Where("document_uuid = ? AND status = ?",
		doc.DocumentUUID, "active").
		Order("modified_time DESC").
		Find(&allRevisions).Error

	if err != nil {
		return fmt.Errorf("failed to query revisions: %w", err)
	}

	// If only one revision, no conflict possible
	if len(allRevisions) <= 1 {
		c.Logger.Debug("single revision, no conflicts",
			"document_id", doc.Document.ID,
			"uuid", doc.DocumentUUID.String(),
		)
		return nil
	}

	// Check for content divergence
	conflicts := make([]models.DocumentRevision, 0)
	for _, rev := range allRevisions {
		// Skip the current revision
		if rev.ID == doc.Revision.ID {
			continue
		}

		// Different content hash indicates conflict
		if rev.ContentHash != doc.Revision.ContentHash {
			conflicts = append(conflicts, rev)
		}
	}

	if len(conflicts) == 0 {
		c.Logger.Debug("no conflicts detected",
			"document_id", doc.Document.ID,
			"uuid", doc.DocumentUUID.String(),
			"revisions_checked", len(allRevisions)-1,
		)
		return nil
	}

	// Conflict detected - determine type and record details
	conflictType := determineConflictType(doc.Revision, conflicts)

	// Create conflict info
	doc.ConflictInfo = &indexer.ConflictInfo{
		DetectedAt:    time.Now(),
		ConflictType:  conflictType,
		SourceHash:    doc.Revision.ContentHash,
		SourceModTime: doc.Revision.ModifiedTime,
		Resolution:    "pending",
	}

	// Use the most recent conflict for comparison
	if len(conflicts) > 0 {
		doc.ConflictInfo.TargetHash = conflicts[0].ContentHash
		doc.ConflictInfo.TargetModTime = conflicts[0].ModifiedTime
	}

	doc.MigrationStatus = "conflict"

	c.Logger.Warn("conflict detected",
		"document_id", doc.Document.ID,
		"uuid", doc.DocumentUUID.String(),
		"conflict_type", conflictType,
		"conflicting_revisions", len(conflicts),
		"current_hash", doc.Revision.ContentHash,
	)

	// Mark revisions as conflicted
	for _, conflict := range conflicts {
		if err := c.DB.Model(&conflict).Update("status", "conflict").Error; err != nil {
			c.Logger.Error("failed to mark revision as conflicted",
				"revision_id", conflict.ID,
				"error", err,
			)
		}
	}

	// Mark current revision as conflicted
	if err := c.DB.Model(doc.Revision).Update("status", "conflict").Error; err != nil {
		c.Logger.Error("failed to mark current revision as conflicted",
			"revision_id", doc.Revision.ID,
			"error", err,
		)
	}

	return nil
}

// determineConflictType analyzes revisions to determine the type of conflict.
func determineConflictType(current *models.DocumentRevision, conflicts []models.DocumentRevision) string {
	// If all conflicts are from different providers, it's a migration conflict
	providers := make(map[string]bool)
	providers[current.ProviderType] = true

	for _, c := range conflicts {
		providers[c.ProviderType] = true
	}

	if len(providers) > 1 {
		return "migration-divergence"
	}

	// If same provider, check modification times
	for _, c := range conflicts {
		timeDiff := current.ModifiedTime.Sub(c.ModifiedTime)
		if timeDiff < 5*time.Minute && timeDiff > -5*time.Minute {
			return "concurrent-edit"
		}
	}

	return "content-divergence"
}

// ExecuteBatch implements BatchCommand for parallel conflict detection.
func (c *DetectConflictsCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	// Conflict detection involves database queries, process in parallel with moderate concurrency
	return indexer.ParallelProcess(ctx, docs, c.Execute, 5)
}
