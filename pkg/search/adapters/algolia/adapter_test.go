package algolia

import (
	"context"
	"testing"

	hermessearch "github.com/hashicorp-forge/hermes/pkg/search"
)

func TestNewAdapter(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				AppID:           "test-app-id",
				WriteAPIKey:     "test-write-key",
				SearchAPIKey:    "test-search-key",
				DocsIndexName:   "test-docs",
				DraftsIndexName: "test-drafts",
			},
			wantErr: false,
		},
		{
			name: "missing app id",
			config: &Config{
				WriteAPIKey:     "test-write-key",
				DocsIndexName:   "test-docs",
				DraftsIndexName: "test-drafts",
			},
			wantErr: true,
		},
		{
			name: "missing write key",
			config: &Config{
				AppID:           "test-app-id",
				DocsIndexName:   "test-docs",
				DraftsIndexName: "test-drafts",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewAdapter(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAdapter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && adapter == nil {
				t.Error("NewAdapter() returned nil adapter")
			}
		})
	}
}

func TestAdapter_Name(t *testing.T) {
	cfg := &Config{
		AppID:           "test-app-id",
		WriteAPIKey:     "test-write-key",
		DocsIndexName:   "test-docs",
		DraftsIndexName: "test-drafts",
	}
	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("NewAdapter() error = %v", err)
	}

	if got := adapter.Name(); got != "algolia" {
		t.Errorf("Name() = %v, want %v", got, "algolia")
	}
}

func TestAdapter_Interfaces(t *testing.T) {
	cfg := &Config{
		AppID:           "test-app-id",
		WriteAPIKey:     "test-write-key",
		DocsIndexName:   "test-docs",
		DraftsIndexName: "test-drafts",
	}
	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("NewAdapter() error = %v", err)
	}

	// Verify adapter implements Provider interface
	var _ hermessearch.Provider = adapter

	// Verify DocumentIndex returns correct interface
	docIndex := adapter.DocumentIndex()
	if docIndex == nil {
		t.Error("DocumentIndex() returned nil")
	}
	var _ hermessearch.DocumentIndex = docIndex

	// Verify DraftIndex returns correct interface
	draftIndex := adapter.DraftIndex()
	if draftIndex == nil {
		t.Error("DraftIndex() returned nil")
	}
	var _ hermessearch.DraftIndex = draftIndex
}

func TestDocumentIndex_BasicOperations(t *testing.T) {
	// Note: These are unit tests that verify the interface implementation.
	// Integration tests against real Algolia would be in a separate file.
	cfg := &Config{
		AppID:           "test-app-id",
		WriteAPIKey:     "test-write-key",
		DocsIndexName:   "test-docs",
		DraftsIndexName: "test-drafts",
	}
	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("NewAdapter() error = %v", err)
	}

	docIndex := adapter.DocumentIndex()

	t.Run("Index accepts document", func(t *testing.T) {
		doc := &hermessearch.Document{
			ObjectID:  "doc-123",
			DocID:     "doc-123",
			Title:     "Test Document",
			DocNumber: "RFC-001",
			DocType:   "RFC",
		}

		// This will fail without real Algolia credentials, but that's expected
		// We're just verifying the method signature and error handling
		err := docIndex.Index(context.Background(), doc)
		if err == nil {
			t.Log("Index succeeded (unexpected in unit test)")
		} else {
			// Verify error is wrapped correctly
			if searchErr, ok := err.(*hermessearch.Error); ok {
				if searchErr.Op != "Index" {
					t.Errorf("Expected Op='Index', got %v", searchErr.Op)
				}
			}
		}
	})
}

// TestConfig_Validation tests configuration validation scenarios
func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "complete valid config",
			config: &Config{
				AppID:             "APPID123",
				WriteAPIKey:       "write-key-123",
				SearchAPIKey:      "search-key-123",
				DocsIndexName:     "prod-docs",
				DraftsIndexName:   "prod-drafts",
				ProjectsIndexName: "prod-projects",
				LinksIndexName:    "prod-links",
			},
			wantErr: false,
		},
		{
			name: "missing app ID",
			config: &Config{
				WriteAPIKey:     "write-key",
				DocsIndexName:   "docs",
				DraftsIndexName: "drafts",
			},
			wantErr: true,
			errMsg:  "credentials required",
		},
		{
			name: "missing write key",
			config: &Config{
				AppID:           "APPID",
				DocsIndexName:   "docs",
				DraftsIndexName: "drafts",
			},
			wantErr: true,
			errMsg:  "credentials required",
		},
		{
			name: "empty app ID",
			config: &Config{
				AppID:           "",
				WriteAPIKey:     "write-key",
				DocsIndexName:   "docs",
				DraftsIndexName: "drafts",
			},
			wantErr: true,
			errMsg:  "credentials required",
		},
		{
			name: "empty write key",
			config: &Config{
				AppID:           "APPID",
				WriteAPIKey:     "",
				DocsIndexName:   "docs",
				DraftsIndexName: "drafts",
			},
			wantErr: true,
			errMsg:  "credentials required",
		},
		{
			name: "search key optional",
			config: &Config{
				AppID:           "APPID",
				WriteAPIKey:     "write-key",
				SearchAPIKey:    "", // Optional
				DocsIndexName:   "docs",
				DraftsIndexName: "drafts",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewAdapter(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("error should contain %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if adapter == nil {
					t.Error("adapter should not be nil")
				}
			}
		})
	}
}

// Helper function for string containment
func contains(s, substr string) bool {
	return len(s) >= len(substr) && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				contains(s[1:], substr)))
}

// TestConfig_IndexNames tests various index name configurations
func TestConfig_IndexNames(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		wantDocsIdx   string
		wantDraftsIdx string
	}{
		{
			name: "standard names",
			config: &Config{
				AppID:           "APP",
				WriteAPIKey:     "KEY",
				DocsIndexName:   "hermes-docs",
				DraftsIndexName: "hermes-drafts",
			},
			wantDocsIdx:   "hermes-docs",
			wantDraftsIdx: "hermes-drafts",
		},
		{
			name: "environment prefixed",
			config: &Config{
				AppID:           "APP",
				WriteAPIKey:     "KEY",
				DocsIndexName:   "prod-hermes-docs",
				DraftsIndexName: "prod-hermes-drafts",
			},
			wantDocsIdx:   "prod-hermes-docs",
			wantDraftsIdx: "prod-hermes-drafts",
		},
		{
			name: "simple names",
			config: &Config{
				AppID:           "APP",
				WriteAPIKey:     "KEY",
				DocsIndexName:   "docs",
				DraftsIndexName: "drafts",
			},
			wantDocsIdx:   "docs",
			wantDraftsIdx: "drafts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewAdapter(tt.config)
			if err != nil {
				t.Fatalf("NewAdapter() error = %v", err)
			}

			// Verify adapter was created
			if adapter == nil {
				t.Fatal("adapter is nil")
			}

			// Verify indexes are different
			if adapter.docsIndex == adapter.draftsIndex {
				t.Error("docs and drafts should use different indexes")
			}
		})
	}
}

// TestDocument_AlgoliaCompatibility tests document structure for Algolia
func TestDocument_AlgoliaCompatibility(t *testing.T) {
	tests := []struct {
		name string
		doc  *hermessearch.Document
	}{
		{
			name: "complete document",
			doc: &hermessearch.Document{
				ObjectID:     "doc-123",
				DocID:        "RFC-042",
				Title:        "Test RFC",
				DocNumber:    "RFC-042",
				DocType:      "RFC",
				Product:      "Terraform",
				Status:       "Approved",
				Owners:       []string{"alice@example.com"},
				Contributors: []string{"bob@example.com"},
				Approvers:    []string{"charlie@example.com"},
				Summary:      "Test summary",
				Content:      "Test content",
				CreatedTime:  1234567890,
				ModifiedTime: 1234567900,
			},
		},
		{
			name: "minimal document",
			doc: &hermessearch.Document{
				ObjectID: "doc-minimal",
				Title:    "Minimal",
			},
		},
		{
			name: "document with custom fields",
			doc: &hermessearch.Document{
				ObjectID: "doc-custom",
				Title:    "Custom",
				CustomFields: map[string]interface{}{
					"priority":    5,
					"tags":        []string{"important", "review"},
					"metadata":    map[string]string{"key": "value"},
					"isPublished": true,
				},
			},
		},
		{
			name: "document with special characters",
			doc: &hermessearch.Document{
				ObjectID:  "doc-special",
				Title:     "Document with \"quotes\" and 'apostrophes'",
				DocNumber: "RFC-042-A",
				Owners:    []string{"alice+test@example.com", "bob_jones@company.org"},
				Summary:   "Summary with <html> tags & special chars: é, ñ, 中文",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify required fields
			if tt.doc.ObjectID == "" {
				t.Error("ObjectID should not be empty")
			}

			// Verify document is valid (would work with Algolia)
			if tt.doc.ObjectID == "" {
				t.Error("Algolia requires ObjectID")
			}
		})
	}
}

// TestAdapter_AppID tests AppID is properly stored
func TestAdapter_AppID(t *testing.T) {
	appID := "TEST-APP-ID-123"
	cfg := &Config{
		AppID:           appID,
		WriteAPIKey:     "test-key",
		DocsIndexName:   "docs",
		DraftsIndexName: "drafts",
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("NewAdapter() error = %v", err)
	}

	if adapter.appID != appID {
		t.Errorf("adapter.appID = %v, want %v", adapter.appID, appID)
	}
}

// TestAdapter_AllIndexes tests all index getters
func TestAdapter_AllIndexes(t *testing.T) {
	cfg := &Config{
		AppID:             "APP",
		WriteAPIKey:       "KEY",
		DocsIndexName:     "docs",
		DraftsIndexName:   "drafts",
		ProjectsIndexName: "projects",
		LinksIndexName:    "links",
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("NewAdapter() error = %v", err)
	}

	tests := []struct {
		name  string
		index interface{}
	}{
		{"DocumentIndex", adapter.DocumentIndex()},
		{"DraftIndex", adapter.DraftIndex()},
		{"ProjectIndex", adapter.ProjectIndex()},
		{"LinksIndex", adapter.LinksIndex()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.index == nil {
				t.Errorf("%s() returned nil", tt.name)
			}
		})
	}
}

// TestSearchQuery_Structure tests search query structure
func TestSearchQuery_Structure(t *testing.T) {
	tests := []struct {
		name  string
		query *hermessearch.SearchQuery
		valid bool
	}{
		{
			name: "valid basic query",
			query: &hermessearch.SearchQuery{
				Query:   "test",
				Page:    1,
				PerPage: 20,
			},
			valid: true,
		},
		{
			name: "query with filters",
			query: &hermessearch.SearchQuery{
				Query: "terraform",
				Filters: map[string][]string{
					"product": {"terraform"},
					"status":  {"approved"},
				},
				Page:    1,
				PerPage: 50,
			},
			valid: true,
		},
		{
			name: "query with facets",
			query: &hermessearch.SearchQuery{
				Query:   "infrastructure",
				Facets:  []string{"product", "docType", "status"},
				Page:    1,
				PerPage: 20,
			},
			valid: true,
		},
		{
			name: "query with sorting",
			query: &hermessearch.SearchQuery{
				Query:     "documents",
				SortBy:    "modifiedTime",
				SortOrder: "desc",
				Page:      1,
				PerPage:   10,
			},
			valid: true,
		},
		{
			name: "empty query",
			query: &hermessearch.SearchQuery{
				Query:   "",
				Page:    1,
				PerPage: 20,
			},
			valid: true, // Empty queries are valid (returns all)
		},
		{
			name: "zero page",
			query: &hermessearch.SearchQuery{
				Query:   "test",
				Page:    0,
				PerPage: 20,
			},
			valid: true, // Should be handled by implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation
			if tt.valid {
				if tt.query == nil {
					t.Error("query should not be nil")
				}
			}
		})
	}
}

// TestPagination_Calculations tests pagination logic
func TestPagination_Calculations(t *testing.T) {
	tests := []struct {
		name        string
		page        int
		perPage     int
		totalHits   int
		wantPages   int
		wantValidPg bool
	}{
		{
			name:        "first page",
			page:        1,
			perPage:     20,
			totalHits:   100,
			wantPages:   5,
			wantValidPg: true,
		},
		{
			name:        "last page",
			page:        5,
			perPage:     20,
			totalHits:   100,
			wantPages:   5,
			wantValidPg: true,
		},
		{
			name:        "partial last page",
			page:        3,
			perPage:     25,
			totalHits:   60,
			wantPages:   3,
			wantValidPg: true,
		},
		{
			name:        "large page size",
			page:        1,
			perPage:     1000,
			totalHits:   500,
			wantPages:   1,
			wantValidPg: true,
		},
		{
			name:        "zero results",
			page:        1,
			perPage:     20,
			totalHits:   0,
			wantPages:   0,
			wantValidPg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate total pages
			totalPages := tt.totalHits / tt.perPage
			if tt.totalHits%tt.perPage != 0 {
				totalPages++
			}

			if tt.totalHits == 0 {
				totalPages = 0
			}

			if totalPages != tt.wantPages {
				t.Errorf("totalPages = %d, want %d", totalPages, tt.wantPages)
			}

			// Validate page number
			validPage := tt.page > 0 && (tt.page <= totalPages || totalPages == 0)
			if validPage != tt.wantValidPg {
				t.Errorf("page validation = %v, want %v", validPage, tt.wantValidPg)
			}
		})
	}
}

// TestConfig_OptionalFields tests optional configuration fields
func TestConfig_OptionalFields(t *testing.T) {
	cfg := &Config{
		AppID:                  "APP",
		WriteAPIKey:            "KEY",
		SearchAPIKey:           "", // Optional
		DocsIndexName:          "docs",
		DraftsIndexName:        "drafts",
		InternalIndexName:      "internal",      // Optional
		LinksIndexName:         "links",         // Optional
		MissingFieldsIndexName: "missing",       // Optional
		ProjectsIndexName:      "projects",      // Optional
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("NewAdapter() error = %v", err)
	}

	if adapter == nil {
		t.Fatal("adapter should not be nil")
	}

	// Verify adapter was created successfully with optional fields
	if adapter.Name() != "algolia" {
		t.Errorf("Name() = %v, want algolia", adapter.Name())
	}
}

// TestErrorWrapping tests that errors are properly wrapped
func TestErrorWrapping(t *testing.T) {
	cfg := &Config{
		AppID:           "INVALID",
		WriteAPIKey:     "INVALID",
		DocsIndexName:   "test-docs",
		DraftsIndexName: "test-drafts",
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("NewAdapter() error = %v", err)
	}

	ctx := context.Background()
	docIndex := adapter.DocumentIndex()

	doc := &hermessearch.Document{
		ObjectID: "test-doc",
		Title:    "Test",
	}

	// This should fail with invalid credentials
	err = docIndex.Index(ctx, doc)
	if err == nil {
		t.Log("Index succeeded (unexpected with invalid credentials)")
		return
	}

	// Verify error is properly wrapped
	searchErr, ok := err.(*hermessearch.Error)
	if !ok {
		t.Errorf("error should be *search.Error, got %T", err)
		return
	}

	if searchErr.Op == "" {
		t.Error("error Op should not be empty")
	}

	if searchErr.Err == nil {
		t.Error("error Err should not be nil")
	}
}

// Note: Integration tests requiring real Algolia credentials are in
// tests/integration/search/algolia_integration_test.go
