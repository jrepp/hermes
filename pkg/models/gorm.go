package models

func ModelsToAutoMigrate() []interface{} {
	// TEMPORARY: Re-enabling AutoMigrate for models with incomplete migrations
	// TODO: Complete the SQL migrations for all models and remove this entirely
	//
	// Core schema: internal/db/migrations/000001_core_schema.up.sql
	// Indexer schema: internal/db/migrations/000002_indexer_core.up.sql
	// Database-specific enhancements: internal/db/migrations/db-specific/*.sql
	//
	// Known incomplete tables (missing columns in migrations):
	// - document_types: missing flight_icon, more_info_link_text, more_info_link_url, checks
	// - (likely others - needs full audit)
	return []interface{}{
		&DocumentType{},
		&Document{},
		&DocumentCustomField{},
		&DocumentFileRevision{},
		&DocumentRevision{},
		DocumentGroupReview{},
		&DocumentRelatedResource{},
		&DocumentRelatedResourceExternalLink{},
		&DocumentRelatedResourceHermesDocument{},
		&DocumentReview{},
		&DocumentTypeCustomField{},
		&Group{},
		// &IndexerFolder{}, // Commented out - causing GORM constraint rename bug
		&IndexerMetadata{},
		&Product{},
		&ProductLatestDocumentNumber{},
		&Project{},
		&ProjectRelatedResource{},
		&ProjectRelatedResourceExternalLink{},
		&ProjectRelatedResourceHermesDocument{},
		&User{},
		&WorkspaceProject{},
		// Do NOT include: HermesInstance, Indexer, IndexerToken (fully in migrations)
	}
}
