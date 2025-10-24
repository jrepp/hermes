package models

func ModelsToAutoMigrate() []interface{} {
	return []interface{}{
		&HermesInstance{}, // Must be first - other tables reference it
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
		&Indexer{},      // NEW: Indexer registration
		&IndexerToken{}, // NEW: Indexer authentication tokens
		&IndexerFolder{},
		&IndexerMetadata{},
		&Product{},
		&ProductLatestDocumentNumber{},
		&Project{},
		&ProjectRelatedResource{},
		&ProjectRelatedResourceExternalLink{},
		&ProjectRelatedResourceHermesDocument{},
		&User{},
		&WorkspaceProject{},
	}
}
