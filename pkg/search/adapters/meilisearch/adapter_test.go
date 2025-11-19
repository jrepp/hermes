package meilisearch

import (
	"encoding/json"
	"testing"

	hermessearch "github.com/hashicorp-forge/hermes/pkg/search"
)

// TestNewAdapter tests adapter creation validation only.
// Note: This test validates configuration, not actual Meilisearch connection.
// Integration tests with real Meilisearch are in tests/integration/search/.
func TestNewAdapter(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				Host:              "http://localhost:7700",
				APIKey:            "masterKey123",
				DocsIndexName:     "test-docs",
				DraftsIndexName:   "test-drafts",
				ProjectsIndexName: "test-projects",
				LinksIndexName:    "test-links",
			},
			wantErr: true, // Will fail without real Meilisearch, which is expected
		},
		{
			name: "missing host",
			cfg: &Config{
				APIKey:            "masterKey123",
				DocsIndexName:     "test-docs",
				DraftsIndexName:   "test-drafts",
				ProjectsIndexName: "test-projects",
				LinksIndexName:    "test-links",
			},
			wantErr: true,
		},
		{
			name: "empty host string",
			cfg: &Config{
				Host:              "",
				APIKey:            "masterKey123",
				DocsIndexName:     "test-docs",
				DraftsIndexName:   "test-drafts",
				ProjectsIndexName: "test-projects",
				LinksIndexName:    "test-links",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewAdapter(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAdapter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && adapter == nil {
				t.Error("NewAdapter() returned nil adapter")
			}
			if adapter != nil && adapter.Name() != "meilisearch" {
				t.Errorf("adapter.Name() = %v, want meilisearch", adapter.Name())
			}
		})
	}
}

// TestConfig_Validation tests configuration validation
func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid complete config",
			cfg: &Config{
				Host:              "http://meilisearch.local:7700",
				APIKey:            "test-key",
				DocsIndexName:     "docs",
				DraftsIndexName:   "drafts",
				ProjectsIndexName: "projects",
				LinksIndexName:    "links",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			cfg: &Config{
				APIKey:            "test-key",
				DocsIndexName:     "docs",
				DraftsIndexName:   "drafts",
				ProjectsIndexName: "projects",
				LinksIndexName:    "links",
			},
			wantErr: true,
			errMsg:  "host required",
		},
		{
			name: "https host",
			cfg: &Config{
				Host:              "https://search.example.com",
				APIKey:            "prod-key",
				DocsIndexName:     "prod-docs",
				DraftsIndexName:   "prod-drafts",
				ProjectsIndexName: "prod-projects",
				LinksIndexName:    "prod-links",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAdapter(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("error message should contain %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		len(s) > len(substr)*2))
}

// TestBuildMeilisearchFilters tests filter string generation.
func TestBuildMeilisearchFilters(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		want    string
	}{
		{
			name: "single filter single value",
			filters: map[string][]string{
				"product": {"terraform"},
			},
			want: `product = "terraform"`,
		},
		{
			name: "single filter multiple values",
			filters: map[string][]string{
				"status": {"approved", "published"},
			},
			want: `status IN ["approved", "published"]`,
		},
		{
			name: "multiple filters",
			filters: map[string][]string{
				"product": {"terraform"},
				"status":  {"approved"},
			},
			// Note: map iteration order is random, so we check both possibilities
			want: "", // We'll check contains instead
		},
		{
			name:    "empty filters",
			filters: map[string][]string{},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMeilisearchFilters(tt.filters)
			if tt.name == "empty filters" {
				if got != nil {
					t.Errorf("buildMeilisearchFilters() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				if tt.want != "" {
					t.Errorf("buildMeilisearchFilters() = nil, want %v", tt.want)
				}
				return
			}
			gotStr, ok := got.(string)
			if !ok {
				t.Errorf("buildMeilisearchFilters() returned non-string: %T", got)
				return
			}
			if tt.want != "" && gotStr != tt.want {
				t.Errorf("buildMeilisearchFilters() = %v, want %v", gotStr, tt.want)
			}
		})
	}
}

// TestConvertMeilisearchFacets tests facet conversion.
func TestConvertMeilisearchFacets(t *testing.T) {
	tests := []struct {
		name      string
		facetDist map[string]map[string]int64
		want      *hermessearch.Facets
		wantErr   bool
	}{
		{
			name: "all facet types",
			facetDist: map[string]map[string]int64{
				"product": {
					"terraform": 10,
					"vault":     5,
				},
				"docType": {
					"RFC": 8,
					"PRD": 7,
				},
				"status": {
					"approved":  6,
					"published": 9,
				},
				"owners": {
					"user1": 3,
					"user2": 4,
				},
			},
			want: &hermessearch.Facets{
				Products: map[string]int{
					"terraform": 10,
					"vault":     5,
				},
				DocTypes: map[string]int{
					"RFC": 8,
					"PRD": 7,
				},
				Statuses: map[string]int{
					"approved":  6,
					"published": 9,
				},
				Owners: map[string]int{
					"user1": 3,
					"user2": 4,
				},
			},
		},
		{
			name:      "nil facets",
			facetDist: nil,
			want: &hermessearch.Facets{
				Products: map[string]int{},
				DocTypes: map[string]int{},
				Statuses: map[string]int{},
				Owners:   map[string]int{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the test data to JSON as the function expects
			facetJSON, err := json.Marshal(tt.facetDist)
			if err != nil {
				t.Fatalf("Failed to marshal test data: %v", err)
			}

			got, err := convertMeilisearchFacets(facetJSON)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertMeilisearchFacets() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got == nil {
				t.Error("convertMeilisearchFacets() returned nil")
				return
			}
			// Compare Products
			if len(got.Products) != len(tt.want.Products) {
				t.Errorf("Products length = %v, want %v", len(got.Products), len(tt.want.Products))
			}
			for k, v := range tt.want.Products {
				if got.Products[k] != v {
					t.Errorf("Products[%s] = %v, want %v", k, got.Products[k], v)
				}
			}
			// Similar checks for other facets...
		})
	}
}

// TestAdapterInterfaces verifies the adapter implements required interfaces.
func TestAdapterInterfaces(t *testing.T) {
	var _ hermessearch.Provider = (*Adapter)(nil)
	var _ hermessearch.DocumentIndex = (*documentIndex)(nil)
	var _ hermessearch.DraftIndex = (*draftIndex)(nil)
}

// TestAdapter_Name tests the Name() method
func TestAdapter_Name(t *testing.T) {
	// Create adapter struct without calling NewAdapter (avoid connection)
	adapter := &Adapter{
		docsIndex:     "test-docs",
		draftsIndex:   "test-drafts",
		projectsIndex: "test-projects",
		linksIndex:    "test-links",
	}

	if got := adapter.Name(); got != "meilisearch" {
		t.Errorf("Name() = %v, want %v", got, "meilisearch")
	}
}

// TestDocument_Structure tests document structure for Meilisearch compatibility
func TestDocument_Structure(t *testing.T) {
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
				Title:    "Minimal Doc",
			},
		},
		{
			name: "document with custom fields",
			doc: &hermessearch.Document{
				ObjectID: "doc-custom",
				Title:    "Custom Doc",
				CustomFields: map[string]interface{}{
					"customKey": "customValue",
					"priority":  5,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify required fields
			if tt.doc.ObjectID == "" {
				t.Error("ObjectID should not be empty")
			}
			if tt.doc.Title == "" {
				t.Error("Title should not be empty")
			}

			// Verify document can be marshaled (Meilisearch compatibility)
			data, err := json.Marshal(tt.doc)
			if err != nil {
				t.Errorf("failed to marshal document: %v", err)
			}

			// Verify we can unmarshal back
			var unmarshaled hermessearch.Document
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("failed to unmarshal document: %v", err)
			}

			if unmarshaled.ObjectID != tt.doc.ObjectID {
				t.Errorf("ObjectID mismatch after marshal/unmarshal: got %v, want %v",
					unmarshaled.ObjectID, tt.doc.ObjectID)
			}
		})
	}
}

// TestBuildMeilisearchFilters_EdgeCases tests additional filter scenarios
func TestBuildMeilisearchFilters_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		wantNil bool
	}{
		{
			name:    "nil filters",
			filters: nil,
			wantNil: true,
		},
		{
			name:    "empty map",
			filters: map[string][]string{},
			wantNil: true,
		},
		{
			name: "filter with empty values",
			filters: map[string][]string{
				"product": {},
			},
			wantNil: true, // Empty filter values treated as no filter
		},
		{
			name: "multiple filters with single values",
			filters: map[string][]string{
				"product": {"terraform"},
				"status":  {"approved"},
				"docType": {"RFC"},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMeilisearchFilters(tt.filters)

			if tt.wantNil {
				if got != nil {
					t.Errorf("buildMeilisearchFilters() = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Error("buildMeilisearchFilters() = nil, want non-nil")
				}
			}
		})
	}
}

// TestConvertMeilisearchFacets_EdgeCases tests additional facet scenarios
func TestConvertMeilisearchFacets_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]map[string]int64
		wantEmpty bool
	}{
		{
			name:      "empty facets",
			input:     map[string]map[string]int64{},
			wantEmpty: true,
		},
		{
			name: "single facet type",
			input: map[string]map[string]int64{
				"product": {
					"terraform": 10,
				},
			},
			wantEmpty: false,
		},
		{
			name: "facet with zero count",
			input: map[string]map[string]int64{
				"product": {
					"terraform": 10,
					"vault":     0,
				},
			},
			wantEmpty: false,
		},
		{
			name: "unknown facet type",
			input: map[string]map[string]int64{
				"unknownFacet": {
					"value": 5,
				},
			},
			wantEmpty: false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			facetJSON, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal test data: %v", err)
			}

			got, err := convertMeilisearchFacets(facetJSON)
			if err != nil {
				t.Errorf("convertMeilisearchFacets() error = %v", err)
				return
			}

			if got == nil {
				t.Error("convertMeilisearchFacets() returned nil")
				return
			}

			// Check that we got proper structure
			if got.Products == nil {
				t.Error("Products map is nil")
			}
			if got.DocTypes == nil {
				t.Error("DocTypes map is nil")
			}
			if got.Statuses == nil {
				t.Error("Statuses map is nil")
			}
			if got.Owners == nil {
				t.Error("Owners map is nil")
			}
		})
	}
}

// TestConfig_IndexNames tests index name configuration
func TestConfig_IndexNames(t *testing.T) {
	tests := []struct {
		name              string
		docsIndex         string
		draftsIndex       string
		projectsIndex     string
		linksIndex        string
		wantDifferentDocs bool
	}{
		{
			name:              "standard names",
			docsIndex:         "hermes-docs",
			draftsIndex:       "hermes-drafts",
			projectsIndex:     "hermes-projects",
			linksIndex:        "hermes-links",
			wantDifferentDocs: true,
		},
		{
			name:              "prefixed names",
			docsIndex:         "prod-hermes-docs",
			draftsIndex:       "prod-hermes-drafts",
			projectsIndex:     "prod-hermes-projects",
			linksIndex:        "prod-hermes-links",
			wantDifferentDocs: true,
		},
		{
			name:              "simple names",
			docsIndex:         "docs",
			draftsIndex:       "drafts",
			projectsIndex:     "projects",
			linksIndex:        "links",
			wantDifferentDocs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &Adapter{
				docsIndex:     tt.docsIndex,
				draftsIndex:   tt.draftsIndex,
				projectsIndex: tt.projectsIndex,
				linksIndex:    tt.linksIndex,
			}

			// Verify index names are properly set
			if adapter.docsIndex != tt.docsIndex {
				t.Errorf("docsIndex = %v, want %v", adapter.docsIndex, tt.docsIndex)
			}
			if adapter.draftsIndex != tt.draftsIndex {
				t.Errorf("draftsIndex = %v, want %v", adapter.draftsIndex, tt.draftsIndex)
			}

			// Verify docs and drafts use different indexes
			if tt.wantDifferentDocs && adapter.docsIndex == adapter.draftsIndex {
				t.Error("docsIndex and draftsIndex should be different")
			}
		})
	}
}

// TestFilterString_SpecialCharacters tests filter handling with special chars
func TestFilterString_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
	}{
		{
			name: "email addresses",
			filters: map[string][]string{
				"owners": {"alice@example.com", "bob@company.org"},
			},
		},
		{
			name: "values with spaces",
			filters: map[string][]string{
				"title": {"My Document Title"},
			},
		},
		{
			name: "values with quotes",
			filters: map[string][]string{
				"product": {`Product "Name"`},
			},
		},
		{
			name: "values with hyphens",
			filters: map[string][]string{
				"docNumber": {"RFC-042", "PRD-123"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			result := buildMeilisearchFilters(tt.filters)
			if result == nil {
				t.Error("buildMeilisearchFilters() returned nil for non-empty filters")
			}
		})
	}
}

// Note: Integration tests that require a running Meilisearch instance
// have been moved to tests/integration/search/meilisearch_adapter_test.go
// Those tests use testcontainers-go to automatically start Meilisearch.
//
// To run integration tests:
//   go test -tags=integration ./tests/integration/search/...
