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

func (d *BaseDoc) DeleteFileRevision(revisionID string) {
	delete(d.FileRevisions, revisionID)
}

func (d BaseDoc) GetApprovedBy() []string {
	return d.ApprovedBy
}

func (d BaseDoc) GetApprovers() []string {
	return d.Approvers
}

func (d BaseDoc) GetChangesRequestedBy() []string {
	return d.ChangesRequestedBy
}

func (d BaseDoc) GetContributors() []string {
	return d.Contributors
}

func (d BaseDoc) GetCreatedTime() int64 {
	return d.CreatedTime
}

func (d BaseDoc) GetDocNumber() string {
	return d.DocNumber
}

func (d BaseDoc) GetDocType() string {
	return d.DocType
}

func (d BaseDoc) GetMetaTags() []string {
	return d.MetaTags
}

func (d BaseDoc) GetObjectID() string {
	return d.ObjectID
}

func (d BaseDoc) GetOwners() []string {
	return d.Owners
}

func (d BaseDoc) GetModifiedTime() int64 {
	return d.ModifiedTime
}

func (d BaseDoc) GetProduct() string {
	return d.Product
}

func (d BaseDoc) GetStatus() string {
	return d.Status
}

func (d BaseDoc) GetSummary() string {
	return d.Summary
}

func (d BaseDoc) GetTitle() string {
	return d.Title
}

func (d *BaseDoc) SetApprovedBy(s []string) {
	d.ApprovedBy = s
}

func (d *BaseDoc) SetChangesRequestedBy(s []string) {
	d.ChangesRequestedBy = s
}

func (d *BaseDoc) SetContent(s string) {
	d.Content = s
}

func (d *BaseDoc) SetDocNumber(s string) {
	d.DocNumber = s
}

func (d *BaseDoc) SetFileRevision(revisionID, revisionName string) {
	if d.FileRevisions == nil {
		d.FileRevisions = map[string]string{
			revisionID: revisionName,
		}
	} else {
		d.FileRevisions[revisionID] = revisionName
	}
}

func (d *BaseDoc) SetLocked(l bool) {
	d.Locked = l
}

func (d *BaseDoc) SetModifiedTime(i int64) {
	d.ModifiedTime = i
}

func (d *BaseDoc) SetStatus(s string) {
	d.Status = s
}
