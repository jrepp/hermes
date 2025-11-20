package bleve

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"

	hermessearch "github.com/hashicorp-forge/hermes/pkg/search"
)

// Adapter implements search.Provider for Bleve (embedded full-text search).
type Adapter struct {
	docsIndex     bleve.Index
	draftsIndex   bleve.Index
	projectsIndex bleve.Index
	linksIndex    bleve.Index

	docsPath     string
	draftsPath   string
	projectsPath string
	linksPath    string
}

// Config contains Bleve configuration.
type Config struct {
	IndexPath string // Base path for all indexes (e.g., "./docs-cms/data/fts.index")
}

// NewAdapter creates a new Bleve search adapter.
func NewAdapter(cfg *Config) (*Adapter, error) {
	if cfg.IndexPath == "" {
		return nil, fmt.Errorf("bleve index path required")
	}

	// Create index directory
	if err := os.MkdirAll(cfg.IndexPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create index directory: %w", err)
	}

	adapter := &Adapter{
		docsPath:     filepath.Join(cfg.IndexPath, "documents.bleve"),
		draftsPath:   filepath.Join(cfg.IndexPath, "drafts.bleve"),
		projectsPath: filepath.Join(cfg.IndexPath, "projects.bleve"),
		linksPath:    filepath.Join(cfg.IndexPath, "links.bleve"),
	}

	// Initialize indexes
	if err := adapter.initializeIndexes(); err != nil {
		return nil, fmt.Errorf("failed to initialize indexes: %w", err)
	}

	return adapter, nil
}

// initializeIndexes opens or creates Bleve indexes.
func (a *Adapter) initializeIndexes() error {
	var err error

	// Create document index mapping
	docMapping := createDocumentMapping()

	// Open or create documents index
	a.docsIndex, err = openOrCreateIndex(a.docsPath, docMapping)
	if err != nil {
		return fmt.Errorf("failed to open docs index: %w", err)
	}

	// Open or create drafts index
	a.draftsIndex, err = openOrCreateIndex(a.draftsPath, docMapping)
	if err != nil {
		return fmt.Errorf("failed to open drafts index: %w", err)
	}

	// Open or create projects index
	projectMapping := createProjectMapping()
	a.projectsIndex, err = openOrCreateIndex(a.projectsPath, projectMapping)
	if err != nil {
		return fmt.Errorf("failed to open projects index: %w", err)
	}

	// Open or create links index
	linkMapping := createLinkMapping()
	a.linksIndex, err = openOrCreateIndex(a.linksPath, linkMapping)
	if err != nil {
		return fmt.Errorf("failed to open links index: %w", err)
	}

	return nil
}

// openOrCreateIndex opens an existing Bleve index or creates a new one.
func openOrCreateIndex(path string, indexMapping mapping.IndexMapping) (bleve.Index, error) {
	idx, err := bleve.Open(path)
	if err == bleve.ErrorIndexPathDoesNotExist {
		// Index doesn't exist, create it
		return bleve.New(path, indexMapping)
	}
	return idx, err
}

// createDocumentMapping creates the index mapping for documents.
func createDocumentMapping() mapping.IndexMapping {
	indexMapping := bleve.NewIndexMapping()

	// Define text field mappings with appropriate analyzers
	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Analyzer = "en" // English analyzer with stemming

	keywordFieldMapping := bleve.NewKeywordFieldMapping()

	dateFieldMapping := bleve.NewDateTimeFieldMapping()

	// Define document mapping
	docMapping := bleve.NewDocumentMapping()

	// Searchable text fields
	docMapping.AddFieldMappingsAt("title", textFieldMapping)
	docMapping.AddFieldMappingsAt("docNumber", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("summary", textFieldMapping)
	docMapping.AddFieldMappingsAt("content", textFieldMapping)

	// Keyword fields for exact matching and faceting
	docMapping.AddFieldMappingsAt("docType", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("product", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("status", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("owners", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("contributors", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("approvers", keywordFieldMapping)

	// Date fields
	docMapping.AddFieldMappingsAt("createdTime", dateFieldMapping)
	docMapping.AddFieldMappingsAt("modifiedTime", dateFieldMapping)

	indexMapping.AddDocumentMapping("_default", docMapping)

	return indexMapping
}

// createProjectMapping creates the index mapping for projects.
func createProjectMapping() mapping.IndexMapping {
	indexMapping := bleve.NewIndexMapping()

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Analyzer = "en"

	keywordFieldMapping := bleve.NewKeywordFieldMapping()
	dateFieldMapping := bleve.NewDateTimeFieldMapping()

	projectMapping := bleve.NewDocumentMapping()

	projectMapping.AddFieldMappingsAt("title", textFieldMapping)
	projectMapping.AddFieldMappingsAt("description", textFieldMapping)
	projectMapping.AddFieldMappingsAt("jiraIssueID", keywordFieldMapping)
	projectMapping.AddFieldMappingsAt("status", keywordFieldMapping)
	projectMapping.AddFieldMappingsAt("createdTime", dateFieldMapping)
	projectMapping.AddFieldMappingsAt("modifiedTime", dateFieldMapping)

	indexMapping.AddDocumentMapping("_default", projectMapping)

	return indexMapping
}

// createLinkMapping creates the index mapping for links.
func createLinkMapping() mapping.IndexMapping {
	indexMapping := bleve.NewIndexMapping()

	keywordFieldMapping := bleve.NewKeywordFieldMapping()

	linkMapping := bleve.NewDocumentMapping()

	linkMapping.AddFieldMappingsAt("objectID", keywordFieldMapping)
	linkMapping.AddFieldMappingsAt("documentID", keywordFieldMapping)

	indexMapping.AddDocumentMapping("_default", linkMapping)

	return indexMapping
}

// Name returns the provider name.
func (a *Adapter) Name() string {
	return "bleve"
}

// Healthy checks if the search backend is accessible.
func (a *Adapter) Healthy(ctx context.Context) error {
	// Check if all indexes are accessible
	if a.docsIndex == nil || a.draftsIndex == nil || a.projectsIndex == nil || a.linksIndex == nil {
		return fmt.Errorf("one or more indexes are not initialized")
	}

	// Try a simple operation to verify indexes are healthy
	_, err := a.docsIndex.DocCount()
	if err != nil {
		return fmt.Errorf("docs index unhealthy: %w", err)
	}

	return nil
}

// DocumentIndex returns the document search interface.
func (a *Adapter) DocumentIndex() hermessearch.DocumentIndex {
	return &documentIndex{adapter: a, index: a.docsIndex}
}

// DraftIndex returns the draft document search interface.
func (a *Adapter) DraftIndex() hermessearch.DraftIndex {
	return &draftIndex{adapter: a, index: a.draftsIndex}
}

// ProjectIndex returns the project search interface.
func (a *Adapter) ProjectIndex() hermessearch.ProjectIndex {
	return &projectIndex{adapter: a, index: a.projectsIndex}
}

// LinksIndex returns the links/redirect search interface.
func (a *Adapter) LinksIndex() hermessearch.LinksIndex {
	return &linksIndex{adapter: a, index: a.linksIndex}
}

// Close closes all Bleve indexes.
func (a *Adapter) Close() error {
	var errs []error

	if err := a.docsIndex.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close docs index: %w", err))
	}

	if err := a.draftsIndex.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close drafts index: %w", err))
	}

	if err := a.projectsIndex.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close projects index: %w", err))
	}

	if err := a.linksIndex.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close links index: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing indexes: %v", errs)
	}

	return nil
}

// documentIndex implements hermessearch.DocumentIndex.
type documentIndex struct {
	adapter *Adapter
	index   bleve.Index
}

// Index adds or updates a document in the search index.
func (d *documentIndex) Index(ctx context.Context, doc *hermessearch.Document) error {
	return d.index.Index(doc.ObjectID, doc)
}

// IndexBatch adds or updates multiple documents.
func (d *documentIndex) IndexBatch(ctx context.Context, docs []*hermessearch.Document) error {
	batch := d.index.NewBatch()

	for _, doc := range docs {
		if err := batch.Index(doc.ObjectID, doc); err != nil {
			return fmt.Errorf("failed to add document to batch: %w", err)
		}
	}

	return d.index.Batch(batch)
}

// Delete removes a document from the search index.
func (d *documentIndex) Delete(ctx context.Context, docID string) error {
	return d.index.Delete(docID)
}

// DeleteBatch removes multiple documents.
func (d *documentIndex) DeleteBatch(ctx context.Context, docIDs []string) error {
	batch := d.index.NewBatch()

	for _, id := range docIDs {
		batch.Delete(id)
	}

	return d.index.Batch(batch)
}

// Search performs a search query.
func (d *documentIndex) Search(ctx context.Context, searchQuery *hermessearch.SearchQuery) (*hermessearch.SearchResult, error) {
	return performSearch(d.index, searchQuery)
}

// GetObject retrieves a single document by ID from the search index.
func (d *documentIndex) GetObject(ctx context.Context, docID string) (*hermessearch.Document, error) {
	doc, err := d.index.Document(docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	if doc == nil {
		return nil, fmt.Errorf("document not found: %s", docID)
	}

	// Convert Bleve document back to hermessearch.Document
	// This is a simplified conversion - in production you'd need proper deserialization
	result := &hermessearch.Document{
		ObjectID: docID,
	}

	return result, nil
}

// GetFacets retrieves available facets for filtering.
func (d *documentIndex) GetFacets(ctx context.Context, facetNames []string) (*hermessearch.Facets, error) {
	// Create a match-all query to get facet counts
	matchAllQuery := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(matchAllQuery)
	searchRequest.Size = 0 // We only want facets, not results

	// Add facet requests
	for _, facetName := range facetNames {
		searchRequest.AddFacet(facetName, bleve.NewFacetRequest(facetName, 100))
	}

	searchResult, err := d.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to get facets: %w", err)
	}

	// Convert Bleve facets to hermessearch.Facets
	facets := &hermessearch.Facets{
		Products: make(map[string]int),
		DocTypes: make(map[string]int),
		Statuses: make(map[string]int),
		Owners:   make(map[string]int),
	}

	if productFacet := searchResult.Facets["product"]; productFacet != nil {
		for _, term := range productFacet.Terms.Terms() {
			facets.Products[term.Term] = term.Count
		}
	}

	if docTypeFacet := searchResult.Facets["docType"]; docTypeFacet != nil {
		for _, term := range docTypeFacet.Terms.Terms() {
			facets.DocTypes[term.Term] = term.Count
		}
	}

	if statusFacet := searchResult.Facets["status"]; statusFacet != nil {
		for _, term := range statusFacet.Terms.Terms() {
			facets.Statuses[term.Term] = term.Count
		}
	}

	if ownersFacet := searchResult.Facets["owners"]; ownersFacet != nil {
		for _, term := range ownersFacet.Terms.Terms() {
			facets.Owners[term.Term] = term.Count
		}
	}

	return facets, nil
}

// Clear removes all documents from the index.
func (d *documentIndex) Clear(ctx context.Context) error {
	// Close and delete the index, then recreate it
	indexPath := d.adapter.docsPath

	if err := d.index.Close(); err != nil {
		return fmt.Errorf("failed to close index: %w", err)
	}

	if err := os.RemoveAll(indexPath); err != nil {
		return fmt.Errorf("failed to remove index: %w", err)
	}

	// Recreate the index
	newIndex, err := bleve.New(indexPath, createDocumentMapping())
	if err != nil {
		return fmt.Errorf("failed to recreate index: %w", err)
	}

	d.index = newIndex
	d.adapter.docsIndex = newIndex

	return nil
}

// draftIndex implements hermessearch.DraftIndex.
// It reuses the documentIndex implementation.
type draftIndex struct {
	adapter *Adapter
	index   bleve.Index
}

func (d *draftIndex) Index(ctx context.Context, doc *hermessearch.Document) error {
	return d.index.Index(doc.ObjectID, doc)
}

func (d *draftIndex) IndexBatch(ctx context.Context, docs []*hermessearch.Document) error {
	batch := d.index.NewBatch()
	for _, doc := range docs {
		if err := batch.Index(doc.ObjectID, doc); err != nil {
			return err
		}
	}
	return d.index.Batch(batch)
}

func (d *draftIndex) Delete(ctx context.Context, docID string) error {
	return d.index.Delete(docID)
}

func (d *draftIndex) DeleteBatch(ctx context.Context, docIDs []string) error {
	batch := d.index.NewBatch()
	for _, id := range docIDs {
		batch.Delete(id)
	}
	return d.index.Batch(batch)
}

func (d *draftIndex) Search(ctx context.Context, searchQuery *hermessearch.SearchQuery) (*hermessearch.SearchResult, error) {
	return performSearch(d.index, searchQuery)
}

func (d *draftIndex) GetObject(ctx context.Context, docID string) (*hermessearch.Document, error) {
	doc, err := d.index.Document(docID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found: %s", docID)
	}
	return &hermessearch.Document{ObjectID: docID}, nil
}

func (d *draftIndex) GetFacets(ctx context.Context, facetNames []string) (*hermessearch.Facets, error) {
	// Same implementation as documentIndex
	matchAllQuery := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(matchAllQuery)
	searchRequest.Size = 0

	for _, facetName := range facetNames {
		searchRequest.AddFacet(facetName, bleve.NewFacetRequest(facetName, 100))
	}

	searchResult, err := d.index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	facets := &hermessearch.Facets{
		Products: make(map[string]int),
		DocTypes: make(map[string]int),
		Statuses: make(map[string]int),
		Owners:   make(map[string]int),
	}

	if productFacet := searchResult.Facets["product"]; productFacet != nil {
		for _, term := range productFacet.Terms.Terms() {
			facets.Products[term.Term] = term.Count
		}
	}

	if docTypeFacet := searchResult.Facets["docType"]; docTypeFacet != nil {
		for _, term := range docTypeFacet.Terms.Terms() {
			facets.DocTypes[term.Term] = term.Count
		}
	}

	if statusFacet := searchResult.Facets["status"]; statusFacet != nil {
		for _, term := range statusFacet.Terms.Terms() {
			facets.Statuses[term.Term] = term.Count
		}
	}

	if ownersFacet := searchResult.Facets["owners"]; ownersFacet != nil {
		for _, term := range ownersFacet.Terms.Terms() {
			facets.Owners[term.Term] = term.Count
		}
	}

	return facets, nil
}

func (d *draftIndex) Clear(ctx context.Context) error {
	indexPath := d.adapter.draftsPath

	if err := d.index.Close(); err != nil {
		return err
	}

	if err := os.RemoveAll(indexPath); err != nil {
		return err
	}

	newIndex, err := bleve.New(indexPath, createDocumentMapping())
	if err != nil {
		return err
	}

	d.index = newIndex
	d.adapter.draftsIndex = newIndex

	return nil
}

// projectIndex implements hermessearch.ProjectIndex.
type projectIndex struct {
	adapter *Adapter
	index   bleve.Index
}

func (p *projectIndex) Index(ctx context.Context, project map[string]any) error {
	objectID, ok := project["objectID"].(string)
	if !ok {
		return fmt.Errorf("project missing objectID")
	}
	return p.index.Index(objectID, project)
}

func (p *projectIndex) Delete(ctx context.Context, projectID string) error {
	return p.index.Delete(projectID)
}

func (p *projectIndex) Search(ctx context.Context, searchQuery *hermessearch.SearchQuery) (*hermessearch.SearchResult, error) {
	return performSearch(p.index, searchQuery)
}

func (p *projectIndex) GetObject(ctx context.Context, projectID string) (map[string]any, error) {
	doc, err := p.index.Document(projectID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}
	// Simplified return - in production you'd need proper deserialization
	return map[string]any{"objectID": projectID}, nil
}

func (p *projectIndex) Clear(ctx context.Context) error {
	indexPath := p.adapter.projectsPath

	if err := p.index.Close(); err != nil {
		return err
	}

	if err := os.RemoveAll(indexPath); err != nil {
		return err
	}

	newIndex, err := bleve.New(indexPath, createProjectMapping())
	if err != nil {
		return err
	}

	p.index = newIndex
	p.adapter.projectsIndex = newIndex

	return nil
}

// linksIndex implements hermessearch.LinksIndex.
type linksIndex struct {
	adapter *Adapter
	index   bleve.Index
}

func (l *linksIndex) SaveLink(ctx context.Context, link map[string]string) error {
	objectID := link["objectID"]
	if objectID == "" {
		return fmt.Errorf("link missing objectID")
	}
	return l.index.Index(objectID, link)
}

func (l *linksIndex) DeleteLink(ctx context.Context, objectID string) error {
	return l.index.Delete(objectID)
}

func (l *linksIndex) GetLink(ctx context.Context, objectID string) (map[string]string, error) {
	doc, err := l.index.Document(objectID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("link not found: %s", objectID)
	}
	// Simplified return - in production you'd need proper deserialization
	return map[string]string{"objectID": objectID}, nil
}

func (l *linksIndex) Clear(ctx context.Context) error {
	indexPath := l.adapter.linksPath

	if err := l.index.Close(); err != nil {
		return err
	}

	if err := os.RemoveAll(indexPath); err != nil {
		return err
	}

	newIndex, err := bleve.New(indexPath, createLinkMapping())
	if err != nil {
		return err
	}

	l.index = newIndex
	l.adapter.linksIndex = newIndex

	return nil
}

// performSearch executes a search query on a Bleve index.
func performSearch(index bleve.Index, searchQuery *hermessearch.SearchQuery) (*hermessearch.SearchResult, error) {
	startTime := time.Now()

	// Build Bleve query
	var q query.Query

	if searchQuery.Query == "" {
		q = bleve.NewMatchAllQuery()
	} else {
		// Use match query for text search
		q = bleve.NewMatchQuery(searchQuery.Query)
	}

	// Build filter queries
	var filterQueries []query.Query

	for field, values := range searchQuery.Filters {
		if len(values) == 0 {
			continue
		}

		// Create disjunction (OR) for multiple values in same field
		disjunction := bleve.NewDisjunctionQuery(nil)
		for _, value := range values {
			matchQuery := bleve.NewMatchPhraseQuery(value)
			matchQuery.SetField(field)
			disjunction.AddQuery(matchQuery)
		}

		filterQueries = append(filterQueries, disjunction)
	}

	// Combine query and filters with AND
	if len(filterQueries) > 0 {
		conjunction := bleve.NewConjunctionQuery(append([]query.Query{q}, filterQueries...)...)
		q = conjunction
	}

	// Create search request
	searchRequest := bleve.NewSearchRequest(q)

	// Pagination
	perPage := searchQuery.PerPage
	if perPage <= 0 {
		perPage = 20 // Default
	}
	page := searchQuery.Page
	if page < 0 {
		page = 0
	}

	searchRequest.From = page * perPage
	searchRequest.Size = perPage

	// Sorting
	if searchQuery.SortBy != "" {
		sortOrder := strings.ToLower(searchQuery.SortOrder) == "desc"
		searchRequest.SortBy([]string{
			fmt.Sprintf("%s%s", map[bool]string{true: "-", false: ""}[sortOrder], searchQuery.SortBy),
		})
	}

	// Highlighting
	if searchQuery.HighlightPreTag != "" {
		searchRequest.Highlight = bleve.NewHighlightWithStyle("html")
		searchRequest.Highlight.AddField("title")
		searchRequest.Highlight.AddField("summary")
		searchRequest.Highlight.AddField("content")
	}

	// Add facets
	for _, facetName := range searchQuery.Facets {
		searchRequest.AddFacet(facetName, bleve.NewFacetRequest(facetName, 100))
	}

	// Execute search
	searchResult, err := index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert results
	hits := make([]*hermessearch.Document, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		doc := &hermessearch.Document{
			ObjectID: hit.ID,
		}

		// Extract fields from hit.Fields
		if title, ok := hit.Fields["title"].(string); ok {
			doc.Title = title
		}
		if docNumber, ok := hit.Fields["docNumber"].(string); ok {
			doc.DocNumber = docNumber
		}
		if docType, ok := hit.Fields["docType"].(string); ok {
			doc.DocType = docType
		}
		if product, ok := hit.Fields["product"].(string); ok {
			doc.Product = product
		}
		if status, ok := hit.Fields["status"].(string); ok {
			doc.Status = status
		}
		if summary, ok := hit.Fields["summary"].(string); ok {
			doc.Summary = summary
		}

		// Extract timestamps
		if createdTime, ok := hit.Fields["createdTime"].(string); ok {
			if t, err := time.Parse(time.RFC3339, createdTime); err == nil {
				doc.CreatedTime = t.Unix()
			} else if i, err := strconv.ParseInt(createdTime, 10, 64); err == nil {
				doc.CreatedTime = i
			}
		}
		if modifiedTime, ok := hit.Fields["modifiedTime"].(string); ok {
			if t, err := time.Parse(time.RFC3339, modifiedTime); err == nil {
				doc.ModifiedTime = t.Unix()
			} else if i, err := strconv.ParseInt(modifiedTime, 10, 64); err == nil {
				doc.ModifiedTime = i
			}
		}

		hits = append(hits, doc)
	}

	// Build facets
	facets := &hermessearch.Facets{
		Products: make(map[string]int),
		DocTypes: make(map[string]int),
		Statuses: make(map[string]int),
		Owners:   make(map[string]int),
	}

	if productFacet := searchResult.Facets["product"]; productFacet != nil {
		for _, term := range productFacet.Terms.Terms() {
			facets.Products[term.Term] = term.Count
		}
	}

	if docTypeFacet := searchResult.Facets["docType"]; docTypeFacet != nil {
		for _, term := range docTypeFacet.Terms.Terms() {
			facets.DocTypes[term.Term] = term.Count
		}
	}

	if statusFacet := searchResult.Facets["status"]; statusFacet != nil {
		for _, term := range statusFacet.Terms.Terms() {
			facets.Statuses[term.Term] = term.Count
		}
	}

	if ownersFacet := searchResult.Facets["owners"]; ownersFacet != nil {
		for _, term := range ownersFacet.Terms.Terms() {
			facets.Owners[term.Term] = term.Count
		}
	}

	totalPages := int(searchResult.Total) / perPage
	if int(searchResult.Total)%perPage > 0 {
		totalPages++
	}

	return &hermessearch.SearchResult{
		Hits:       hits,
		TotalHits:  int(searchResult.Total),
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
		Facets:     facets,
		QueryTime:  time.Since(startTime),
	}, nil
}
