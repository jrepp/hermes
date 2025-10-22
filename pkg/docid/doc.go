// Package docid provides type-safe document identification for Hermes.
//
// This package implements the composite document ID system described in
// docs-internal/DISTRIBUTED_PROJECTS_ARCHITECTURE.md, enabling documents to
// be tracked across multiple storage providers (Google Workspace, local Git,
// remote Hermes instances) with stable UUIDs.
//
// # Core Concepts
//
//  1. UUID: Stable, globally unique document identifier that persists across
//     provider migrations and represents the logical document.
//
//  2. ProviderID: Backend-specific identifier (e.g., Google Drive file ID,
//     local file path, remote Hermes URI).
//
//  3. CompositeID: Fully-qualified document reference containing UUID,
//     provider type, provider-specific ID, and optional project context.
//
// # Usage Examples
//
//	// Create a new document with UUID
//	uuid := docid.NewUUID()
//	googleID, _ := docid.GoogleFileID("1a2b3c4d5e6f7890")
//	compositeID := docid.NewCompositeID(uuid, googleID, "rfc-archive")
//
//	// Parse from string
//	id, err := docid.ParseCompositeID("uuid/550e8400-e29b-41d4-a716-446655440000")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Serialize for API
//	urlPath := id.ShortString()  // "uuid/550e8400-..."
//	fullString := id.String()    // "uuid:...:provider:...:id:..."
//
// # Database Integration
//
// UUID and CompositeID implement sql.Scanner and driver.Valuer for direct
// database integration:
//
//	type Document struct {
//	    gorm.Model
//	    DocumentUUID  docid.UUID `gorm:"type:uuid;uniqueIndex"`
//	    GoogleFileID  string     // Legacy field
//	    ProviderType  string
//	    ProviderDocID string
//	}
//
// # Migration Strategy
//
// This package supports gradual migration from GoogleFileID-based
// identification:
//
// Phase 1: Add nullable UUID column, populate via background job
// Phase 2: Update APIs to accept both GoogleFileID and UUID
// Phase 3: Migrate frontend to use UUIDs
// Phase 4: Deprecate GoogleFileID-only lookups
//
// See docs-internal/DOCID_PACKAGE_ANALYSIS.md for detailed integration plan.
package docid
