package hashicorpdocs

// BaseDoc contains common document metadata fields used by Hermes.
type BaseDoc struct {
	CustomEditableFields map[string]CustomDocTypeField `json:"customEditableFields,omitempty"`
	FileRevisions        map[string]string             `json:"fileRevisions,omitempty"`
	Title                string                        `json:"title,omitempty"`
	DocType              string                        `json:"docType,omitempty"`
	DocNumber            string                        `json:"docNumber,omitempty"`
	ThumbnailLink        string                        `json:"thumbnailLink,omitempty"`
	Status               string                        `json:"status,omitempty"`
	ObjectID             string                        `json:"objectID,omitempty"`
	Summary              string                        `json:"summary,omitempty"`
	Product              string                        `json:"product,omitempty"`
	Content              string                        `json:"content,omitempty"`
	Created              string                        `json:"created,omitempty"`
	Approvers            []string                      `json:"approvers,omitempty"`
	LinkedDocs           []string                      `json:"linkedDocs,omitempty"`
	MetaTags             []string                      `json:"_tags,omitempty"`
	Owners               []string                      `json:"owners,omitempty"`
	OwnerPhotos          []string                      `json:"ownerPhotos,omitempty"`
	Contributors         []string                      `json:"contributors,omitempty"`
	ChangesRequestedBy   []string                      `json:"changesRequestedBy,omitempty"`
	ApprovedBy           []string                      `json:"approvedBy,omitempty"`
	Tags                 []string                      `json:"tags,omitempty"`
	CreatedTime          int64                         `json:"createdTime,omitempty"`
	ModifiedTime         int64                         `json:"modifiedTime,omitempty"`
	Locked               bool                          `json:"locked,omitempty"`
	AppCreated           bool                          `json:"appCreated,omitempty"`
}

// DeleteFileRevision removes a file revision by its ID.
func (d *BaseDoc) DeleteFileRevision(revisionID string) {
	delete(d.FileRevisions, revisionID)
}

// GetApprovedBy returns the list of users who approved the document.
func (d BaseDoc) GetApprovedBy() []string {
	return d.ApprovedBy
}

// GetApprovers returns the list of document approvers.
func (d BaseDoc) GetApprovers() []string {
	return d.Approvers
}

// GetChangesRequestedBy returns the list of users who requested changes.
func (d BaseDoc) GetChangesRequestedBy() []string {
	return d.ChangesRequestedBy
}

// GetContributors returns the list of document contributors.
func (d BaseDoc) GetContributors() []string {
	return d.Contributors
}

// GetCreatedTime returns the document creation time as a Unix timestamp.
func (d BaseDoc) GetCreatedTime() int64 {
	return d.CreatedTime
}

// GetDocNumber returns the document number.
func (d BaseDoc) GetDocNumber() string {
	return d.DocNumber
}

// GetDocType returns the document type.
func (d BaseDoc) GetDocType() string {
	return d.DocType
}

// GetMetaTags returns the document meta tags.
func (d BaseDoc) GetMetaTags() []string {
	return d.MetaTags
}

// GetObjectID returns the document object ID.
func (d BaseDoc) GetObjectID() string {
	return d.ObjectID
}

// GetOwners returns the list of document owners.
func (d BaseDoc) GetOwners() []string {
	return d.Owners
}

// GetModifiedTime returns the document modification time as a Unix timestamp.
func (d BaseDoc) GetModifiedTime() int64 {
	return d.ModifiedTime
}

// GetProduct returns the product associated with the document.
func (d BaseDoc) GetProduct() string {
	return d.Product
}

// GetStatus returns the document status.
func (d BaseDoc) GetStatus() string {
	return d.Status
}

// GetSummary returns the document summary.
func (d BaseDoc) GetSummary() string {
	return d.Summary
}

// GetTitle returns the document title.
func (d BaseDoc) GetTitle() string {
	return d.Title
}

// SetApprovedBy sets the list of users who approved the document.
func (d *BaseDoc) SetApprovedBy(s []string) {
	d.ApprovedBy = s
}

// SetChangesRequestedBy sets the list of users who requested changes.
func (d *BaseDoc) SetChangesRequestedBy(s []string) {
	d.ChangesRequestedBy = s
}

// SetContent sets the document content.
func (d *BaseDoc) SetContent(s string) {
	d.Content = s
}

// SetDocNumber sets the document number.
func (d *BaseDoc) SetDocNumber(s string) {
	d.DocNumber = s
}

// SetFileRevision adds or updates a file revision.
func (d *BaseDoc) SetFileRevision(revisionID, revisionName string) {
	if d.FileRevisions == nil {
		d.FileRevisions = map[string]string{
			revisionID: revisionName,
		}
	} else {
		d.FileRevisions[revisionID] = revisionName
	}
}

// SetLocked sets the document locked state.
func (d *BaseDoc) SetLocked(l bool) {
	d.Locked = l
}

// SetModifiedTime sets the document modification time.
func (d *BaseDoc) SetModifiedTime(i int64) {
	d.ModifiedTime = i
}

// SetStatus sets the document status.
func (d *BaseDoc) SetStatus(s string) {
	d.Status = s
}
