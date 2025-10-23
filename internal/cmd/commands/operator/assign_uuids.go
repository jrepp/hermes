package operator

import (
	"flag"
	"fmt"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/db"
	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	gormlogger "gorm.io/gorm/logger"
)

type AssignUUIDsCommand struct {
	*base.Command

	flagConfig    string
	flagDryRun    bool
	flagBatchSize int
	flagVerbose   bool
}

func (c *AssignUUIDsCommand) Synopsis() string {
	return "Assign UUIDs to documents that don't have them"
}

func (c *AssignUUIDsCommand) Help() string {
	return `Usage: hermes operator assign-uuids

  This command assigns UUIDs to all documents that don't have one.
  Documents are processed in batches with progress logging.` +
		c.Flags().Help()
}

func (c *AssignUUIDsCommand) Flags() *base.FlagSet {
	f := base.NewFlagSet(
		flag.NewFlagSet("assign-uuids", flag.ExitOnError))

	f.StringVar(
		&c.flagConfig, "config", "", "(Required) Path to Hermes config file",
	)
	f.BoolVar(
		&c.flagDryRun, "dry-run", false,
		"Only print what would be done without making changes.",
	)
	f.IntVar(
		&c.flagBatchSize, "batch-size", 100,
		"Number of documents to process per batch.",
	)
	f.BoolVar(
		&c.flagVerbose, "verbose", false,
		"Print extra information including each document UUID assignment.",
	)

	return f
}

func (c *AssignUUIDsCommand) Run(args []string) int {
	logger, ui := c.Log, c.UI

	// Parse flags.
	flags := c.Flags()
	if err := flags.Parse(args); err != nil {
		ui.Error(fmt.Sprintf("error parsing flags: %v", err))
		return 1
	}

	// Validate flags.
	if c.flagConfig == "" {
		ui.Error("config flag is required")
		return 1
	}

	if c.flagBatchSize < 1 {
		ui.Error("batch-size must be at least 1")
		return 1
	}

	// Parse configuration.
	cfg, err := config.NewConfig(c.flagConfig, "") // No profile support in operator commands
	if err != nil {
		ui.Error(fmt.Sprintf("error parsing config file: %v", err))
		return 1
	}

	// Initialize database.
	database, err := db.NewDB(*cfg.Postgres)
	if err != nil {
		ui.Error(fmt.Sprintf("error initializing database: %v", err))
		return 1
	}

	// Create GORM-compatible logger.
	stdLogger := logger.StandardLogger(&hclog.StandardLoggerOptions{
		InferLevels: true,
	})
	logLevel := gormlogger.Silent
	if c.flagVerbose {
		logLevel = gormlogger.Info
	}
	database.Logger = gormlogger.New(
		stdLogger,
		gormlogger.Config{
			SlowThreshold:             0,
			IgnoreRecordNotFoundError: true,
			LogLevel:                  logLevel,
		},
	)

	// Count documents without UUIDs.
	var totalCount int64
	if err := database.Model(&models.Document{}).
		Where("document_uuid IS NULL").
		Count(&totalCount).Error; err != nil {
		ui.Error(fmt.Sprintf("error counting documents without UUIDs: %v", err))
		return 1
	}

	if totalCount == 0 {
		ui.Info("All documents already have UUIDs assigned")
		return 0
	}

	// Display summary.
	ui.Info(fmt.Sprintf("Found %d documents without UUIDs", totalCount))
	if c.flagDryRun {
		ui.Warn("DRY RUN mode enabled - no changes will be made")
	}
	ui.Info(fmt.Sprintf("Processing in batches of %d documents", c.flagBatchSize))

	// Process documents in batches.
	var processed int64
	var assigned int64
	var errors int64

	for offset := 0; int64(offset) < totalCount; offset += c.flagBatchSize {
		// Fetch batch of documents.
		var docs []models.Document
		if err := database.
			Where("document_uuid IS NULL").
			Limit(c.flagBatchSize).
			Offset(offset).
			Find(&docs).Error; err != nil {
			ui.Error(fmt.Sprintf("error fetching documents at offset %d: %v", offset, err))
			return 1
		}

		if len(docs) == 0 {
			break
		}

		// Process each document in the batch.
		for i := range docs {
			doc := &docs[i]
			processed++

			// Generate UUID.
			uuid := docid.NewUUID()

			if c.flagVerbose {
				ui.Info(fmt.Sprintf("[%d/%d] Document ID %d (GoogleFileID: %s) -> UUID: %s",
					processed, totalCount, doc.ID, doc.GoogleFileID, uuid.String()))
			}

			if !c.flagDryRun {
				// Assign UUID to document.
				doc.SetDocumentUUID(uuid)

				// Save to database.
				if err := database.Save(doc).Error; err != nil {
					ui.Error(fmt.Sprintf("error saving document ID %d: %v", doc.ID, err))
					errors++
					continue
				}
			}

			assigned++
		}

		// Progress update after each batch.
		if !c.flagVerbose {
			ui.Info(fmt.Sprintf("Progress: %d/%d documents processed (%.1f%%)",
				processed, totalCount, float64(processed)/float64(totalCount)*100))
		}
	}

	// Final summary.
	ui.Info("")
	ui.Info("=== Summary ===")
	ui.Info(fmt.Sprintf("Total documents processed: %d", processed))
	if c.flagDryRun {
		ui.Info(fmt.Sprintf("Would assign UUIDs to: %d documents", assigned))
	} else {
		ui.Info(fmt.Sprintf("UUIDs assigned: %d", assigned))
	}
	if errors > 0 {
		ui.Error(fmt.Sprintf("Errors encountered: %d", errors))
		return 1
	}

	if c.flagDryRun {
		ui.Warn("DRY RUN completed - no changes were made")
	} else {
		ui.Info("UUID assignment completed successfully")
	}

	return 0
}
