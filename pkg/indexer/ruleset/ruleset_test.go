package ruleset

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/pkg/models"
)

// Helper function to create a test revision
func createTestRevision() *models.DocumentRevision {
	return &models.DocumentRevision{
		DocumentUUID: uuid.New(),
		DocumentID:   "doc-123",
		ProviderType: "google",
		Title:        "RFC-001: Test Document",
		ContentHash:  "hash123",
		Status:       "active",
		ModifiedTime: time.Now(),
	}
}

func TestMatcher_Match_NoConditions(t *testing.T) {
	rulesets := Rulesets{
		{
			Name:       "match-all",
			Conditions: map[string]string{}, // No conditions = match all
			Pipeline:   []string{"search_index"},
		},
	}

	matcher := NewMatcher(rulesets)
	revision := createTestRevision()

	matched := matcher.Match(revision, nil)

	assert.Len(t, matched, 1)
	assert.Equal(t, "match-all", matched[0].Name)
}

func TestMatcher_Match_MultipleRulesets(t *testing.T) {
	rulesets := Rulesets{
		{
			Name: "google-docs",
			Conditions: map[string]string{
				"provider_type": "google",
			},
			Pipeline: []string{"search_index"},
		},
		{
			Name: "active-docs",
			Conditions: map[string]string{
				"status": "active",
			},
			Pipeline: []string{"embeddings"},
		},
		{
			Name: "github-docs",
			Conditions: map[string]string{
				"provider_type": "github",
			},
			Pipeline: []string{"validation"},
		},
	}

	matcher := NewMatcher(rulesets)
	revision := createTestRevision()
	revision.ProviderType = "google"
	revision.Status = "active"

	matched := matcher.Match(revision, nil)

	// Should match "google-docs" and "active-docs" but not "github-docs"
	assert.Len(t, matched, 2)
	assert.Equal(t, "google-docs", matched[0].Name)
	assert.Equal(t, "active-docs", matched[1].Name)
}

func TestMatcher_Match_NoMatches(t *testing.T) {
	rulesets := Rulesets{
		{
			Name: "github-only",
			Conditions: map[string]string{
				"provider_type": "github",
			},
			Pipeline: []string{"search_index"},
		},
	}

	matcher := NewMatcher(rulesets)
	revision := createTestRevision()
	revision.ProviderType = "google"

	matched := matcher.Match(revision, nil)

	assert.Len(t, matched, 0)
}

func TestRuleset_Matches_ExactMatch(t *testing.T) {
	ruleset := Ruleset{
		Name: "test",
		Conditions: map[string]string{
			"provider_type": "google",
			"status":        "active",
		},
		Pipeline: []string{"search_index"},
	}

	revision := createTestRevision()
	revision.ProviderType = "google"
	revision.Status = "active"

	assert.True(t, ruleset.Matches(revision, nil))
}

func TestRuleset_Matches_PartialMatch_ShouldFail(t *testing.T) {
	ruleset := Ruleset{
		Name: "test",
		Conditions: map[string]string{
			"provider_type": "google",
			"status":        "active",
		},
		Pipeline: []string{"search_index"},
	}

	revision := createTestRevision()
	revision.ProviderType = "google"
	revision.Status = "draft" // Doesn't match

	assert.False(t, ruleset.Matches(revision, nil))
}

func TestRuleset_Matches_WithMetadata(t *testing.T) {
	ruleset := Ruleset{
		Name: "test",
		Conditions: map[string]string{
			"document_type": "RFC",
		},
		Pipeline: []string{"search_index"},
	}

	revision := createTestRevision()
	metadata := map[string]interface{}{
		"document_type": "RFC",
	}

	assert.True(t, ruleset.Matches(revision, metadata))
}

func TestRuleset_CompareEquals_InOperator(t *testing.T) {
	ruleset := Ruleset{
		Name: "test",
		Conditions: map[string]string{
			"status": "active,draft,archived", // IN operator
		},
		Pipeline: []string{"search_index"},
	}

	tests := []struct {
		status      string
		shouldMatch bool
	}{
		{"active", true},
		{"draft", true},
		{"archived", true},
		{"deleted", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			revision := createTestRevision()
			revision.Status = tt.status

			result := ruleset.Matches(revision, nil)
			assert.Equal(t, tt.shouldMatch, result)
		})
	}
}

func TestRuleset_CompareContains(t *testing.T) {
	ruleset := Ruleset{
		Name: "test",
		Conditions: map[string]string{
			"title_contains": "RFC",
		},
		Pipeline: []string{"search_index"},
	}

	tests := []struct {
		title       string
		shouldMatch bool
	}{
		{"RFC-001: Test", true},
		{"Test RFC Document", true},
		{"rfc-002: lowercase", true}, // Case-insensitive
		{"Design Document", false},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			revision := createTestRevision()
			revision.Title = tt.title

			result := ruleset.Matches(revision, nil)
			assert.Equal(t, tt.shouldMatch, result)
		})
	}
}

func TestRuleset_CompareGreaterThan(t *testing.T) {
	ruleset := Ruleset{
		Name: "test",
		Conditions: map[string]string{
			"content_length_gt": "5000",
		},
		Pipeline: []string{"search_index"},
	}

	tests := []struct {
		name        string
		length      interface{}
		shouldMatch bool
	}{
		{"larger int", 10000, true},
		{"larger int64", int64(6000), true},
		{"larger float", 5001.0, true},
		{"larger string", "7000", true},
		{"equal", 5000, false},
		{"smaller", 4000, false},
		{"invalid string", "not-a-number", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			revision := createTestRevision()
			metadata := map[string]interface{}{
				"content_length": tt.length,
			}

			result := ruleset.Matches(revision, metadata)
			assert.Equal(t, tt.shouldMatch, result)
		})
	}
}

func TestRuleset_CompareLessThan(t *testing.T) {
	ruleset := Ruleset{
		Name: "test",
		Conditions: map[string]string{
			"content_length_lt": "1000",
		},
		Pipeline: []string{"search_index"},
	}

	tests := []struct {
		name        string
		length      interface{}
		shouldMatch bool
	}{
		{"smaller int", 500, true},
		{"smaller int64", int64(800), true},
		{"smaller float", 999.5, true},
		{"smaller string", "100", true},
		{"equal", 1000, false},
		{"larger", 2000, false},
		{"invalid string", "not-a-number", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			revision := createTestRevision()
			metadata := map[string]interface{}{
				"content_length": tt.length,
			}

			result := ruleset.Matches(revision, metadata)
			assert.Equal(t, tt.shouldMatch, result)
		})
	}
}

func TestRuleset_GetValue_RevisionFields(t *testing.T) {
	ruleset := Ruleset{Name: "test"}
	revision := createTestRevision()
	revision.ProviderType = "google"
	revision.Status = "active"
	revision.DocumentID = "doc-123"
	revision.Title = "Test Doc"
	revision.ContentHash = "hash456"

	tests := []struct {
		key      string
		expected interface{}
	}{
		{"provider_type", "google"},
		{"status", "active"},
		{"document_id", "doc-123"},
		{"document_uuid", revision.DocumentUUID.String()},
		{"title", "Test Doc"},
		{"content_hash", "hash456"},
		{"nonexistent", nil},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			value := ruleset.getValue(tt.key, revision, nil)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRuleset_GetValue_MetadataFields(t *testing.T) {
	ruleset := Ruleset{Name: "test"}
	revision := createTestRevision()
	metadata := map[string]interface{}{
		"document_type":  "RFC",
		"author":         "john@example.com",
		"content_length": 5000,
	}

	tests := []struct {
		key      string
		expected interface{}
	}{
		{"document_type", "RFC"},
		{"author", "john@example.com"},
		{"content_length", 5000},
		{"nonexistent", nil},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			value := ruleset.getValue(tt.key, revision, metadata)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRuleset_GetValue_StripsOperatorSuffixes(t *testing.T) {
	ruleset := Ruleset{Name: "test"}
	revision := createTestRevision()
	metadata := map[string]interface{}{
		"content_length": 5000,
	}

	// Test that operator suffixes are stripped when getting value
	tests := []string{
		"content_length_gt",
		"content_length_lt",
		"content_length_contains",
	}

	for _, key := range tests {
		t.Run(key, func(t *testing.T) {
			value := ruleset.getValue(key, revision, metadata)
			assert.Equal(t, 5000, value)
		})
	}
}

func TestRuleset_CompareEquals_NilValue(t *testing.T) {
	ruleset := Ruleset{Name: "test"}

	result := ruleset.compareEquals(nil, "anything")
	assert.False(t, result)
}

func TestRuleset_CompareGreaterThan_NilValue(t *testing.T) {
	ruleset := Ruleset{Name: "test"}

	result := ruleset.compareGreaterThan(nil, "100")
	assert.False(t, result)
}

func TestRuleset_CompareLessThan_NilValue(t *testing.T) {
	ruleset := Ruleset{Name: "test"}

	result := ruleset.compareLessThan(nil, "100")
	assert.False(t, result)
}

func TestRuleset_CompareContains_NilValue(t *testing.T) {
	ruleset := Ruleset{Name: "test"}

	result := ruleset.compareContains(nil, "test")
	assert.False(t, result)
}

func TestRuleset_ToNumber_DifferentTypes(t *testing.T) {
	ruleset := Ruleset{Name: "test"}

	tests := []struct {
		name      string
		input     interface{}
		expected  float64
		shouldErr bool
	}{
		{"int", 42, 42.0, false},
		{"int64", int64(1000), 1000.0, false},
		{"float64", 3.14, 3.14, false},
		{"string valid", "123.45", 123.45, false},
		{"string invalid", "not-a-number", 0, true},
		{"bool", true, 0, true},
		{"struct", struct{}{}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ruleset.toNumber(tt.input)

			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRuleset_GetStepConfig(t *testing.T) {
	ruleset := Ruleset{
		Name: "test",
		Config: map[string]interface{}{
			"llm_summary": map[string]interface{}{
				"model":      "gpt-4o-mini",
				"max_tokens": 500,
			},
			"embeddings": map[string]interface{}{
				"model":      "text-embedding-3-small",
				"dimensions": 1536,
			},
		},
	}

	// Test valid step config
	llmConfig := ruleset.GetStepConfig("llm_summary")
	require.NotNil(t, llmConfig)
	assert.Equal(t, "gpt-4o-mini", llmConfig["model"])
	assert.Equal(t, 500, llmConfig["max_tokens"])

	// Test another valid step
	embeddingsConfig := ruleset.GetStepConfig("embeddings")
	require.NotNil(t, embeddingsConfig)
	assert.Equal(t, "text-embedding-3-small", embeddingsConfig["model"])

	// Test nonexistent step
	missingConfig := ruleset.GetStepConfig("nonexistent")
	assert.Nil(t, missingConfig)
}

func TestRuleset_GetStepConfig_NoConfig(t *testing.T) {
	ruleset := Ruleset{
		Name:   "test",
		Config: nil,
	}

	config := ruleset.GetStepConfig("llm_summary")
	assert.Nil(t, config)
}

func TestRuleset_GetStepConfig_InvalidType(t *testing.T) {
	ruleset := Ruleset{
		Name: "test",
		Config: map[string]interface{}{
			"llm_summary": "not-a-map", // Invalid type
		},
	}

	config := ruleset.GetStepConfig("llm_summary")
	assert.Nil(t, config)
}

func TestRuleset_Validate_Success(t *testing.T) {
	ruleset := Ruleset{
		Name: "test-ruleset",
		Conditions: map[string]string{
			"provider_type": "google",
		},
		Pipeline: []string{"search_index", "llm_summary"},
	}

	err := ruleset.Validate()
	assert.NoError(t, err)
}

func TestRuleset_Validate_MissingName(t *testing.T) {
	ruleset := Ruleset{
		Name:     "",
		Pipeline: []string{"search_index"},
	}

	err := ruleset.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestRuleset_Validate_MissingPipeline(t *testing.T) {
	ruleset := Ruleset{
		Name:     "test",
		Pipeline: []string{},
	}

	err := ruleset.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline steps are required")
}

func TestRuleset_Validate_InvalidStep(t *testing.T) {
	ruleset := Ruleset{
		Name:     "test",
		Pipeline: []string{"search_index", "invalid_step"},
	}

	err := ruleset.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown pipeline step")
	assert.Contains(t, err.Error(), "invalid_step")
}

func TestRuleset_Validate_AllValidSteps(t *testing.T) {
	validSteps := []string{
		"search_index",
		"embeddings",
		"llm_summary",
		"validation",
		"llm_validation",
		"link_extraction",
		"metadata_extract",
	}

	ruleset := Ruleset{
		Name:     "test",
		Pipeline: validSteps,
	}

	err := ruleset.Validate()
	assert.NoError(t, err)
}

func TestRulesets_ValidateAll_Success(t *testing.T) {
	rulesets := Rulesets{
		{
			Name:     "ruleset1",
			Pipeline: []string{"search_index"},
		},
		{
			Name:     "ruleset2",
			Pipeline: []string{"embeddings"},
		},
	}

	err := rulesets.ValidateAll()
	assert.NoError(t, err)
}

func TestRulesets_ValidateAll_EmptyRulesets(t *testing.T) {
	rulesets := Rulesets{}

	err := rulesets.ValidateAll()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one ruleset is required")
}

func TestRulesets_ValidateAll_OneInvalid(t *testing.T) {
	rulesets := Rulesets{
		{
			Name:     "valid",
			Pipeline: []string{"search_index"},
		},
		{
			Name:     "", // Invalid
			Pipeline: []string{"embeddings"},
		},
	}

	err := rulesets.ValidateAll()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestRuleset_ComplexConditions(t *testing.T) {
	ruleset := Ruleset{
		Name: "complex",
		Conditions: map[string]string{
			"provider_type":     "google",
			"status":            "active,published",
			"title_contains":    "RFC",
			"content_length_gt": "1000",
			"content_length_lt": "50000",
		},
		Pipeline: []string{"search_index", "embeddings", "llm_summary"},
	}

	revision := createTestRevision()
	revision.ProviderType = "google"
	revision.Status = "active"
	revision.Title = "RFC-001: Complex Test"

	metadata := map[string]interface{}{
		"content_length": 5000,
	}

	// Should match - all conditions satisfied
	assert.True(t, ruleset.Matches(revision, metadata))

	// Fail one condition
	revision.Status = "draft"
	assert.False(t, ruleset.Matches(revision, metadata))

	// Restore status, fail numeric condition
	revision.Status = "active"
	metadata["content_length"] = 500 // Too small
	assert.False(t, ruleset.Matches(revision, metadata))

	// Restore, fail other numeric condition
	metadata["content_length"] = 60000 // Too large
	assert.False(t, ruleset.Matches(revision, metadata))
}

func TestRuleset_CaseInsensitiveContains(t *testing.T) {
	ruleset := Ruleset{
		Name: "test",
		Conditions: map[string]string{
			"title_contains": "rfc",
		},
		Pipeline: []string{"search_index"},
	}

	tests := []struct {
		title       string
		shouldMatch bool
	}{
		{"RFC-001", true},
		{"rfc-002", true},
		{"Rfc-003", true},
		{"Document about rfc standards", true},
		{"Design Doc", false},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			revision := createTestRevision()
			revision.Title = tt.title

			result := ruleset.Matches(revision, nil)
			assert.Equal(t, tt.shouldMatch, result)
		})
	}
}

func TestRuleset_MultipleMatchers_Priority(t *testing.T) {
	// Test that multiple rulesets can match and are returned in order
	rulesets := Rulesets{
		{
			Name: "priority-1",
			Conditions: map[string]string{
				"provider_type": "google",
			},
			Pipeline: []string{"search_index"},
		},
		{
			Name: "priority-2",
			Conditions: map[string]string{
				"status": "active",
			},
			Pipeline: []string{"embeddings"},
		},
		{
			Name: "priority-3",
			Conditions: map[string]string{
				"provider_type": "google",
				"status":        "active",
			},
			Pipeline: []string{"llm_summary"},
		},
	}

	matcher := NewMatcher(rulesets)
	revision := createTestRevision()
	revision.ProviderType = "google"
	revision.Status = "active"

	matched := matcher.Match(revision, nil)

	// All three should match, in order
	require.Len(t, matched, 3)
	assert.Equal(t, "priority-1", matched[0].Name)
	assert.Equal(t, "priority-2", matched[1].Name)
	assert.Equal(t, "priority-3", matched[2].Name)
}
