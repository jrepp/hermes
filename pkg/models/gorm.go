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
