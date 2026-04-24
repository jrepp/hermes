// Package structs provides structs functionality.
package structs

// ProductDocTypeData contains data for each document type.
type ProductDocTypeData struct {
	FolderID        string `json:"folderID"`
	LatestDocNumber int    `json:"latestDocNumber"`
}

// ProductData is the data associated with a product or area.
// This may include product abbreviation, etc.
type ProductData struct {
	PerDocTypeData map[string]ProductDocTypeData `json:"perDocTypeData"`
	Abbreviation   string                        `json:"abbreviation"`
}

// Products is the slice of product data.
type Products struct {
	Data     map[string]ProductData `json:"data"`
	ObjectID string                 `json:"objectID,omitempty"`
}
