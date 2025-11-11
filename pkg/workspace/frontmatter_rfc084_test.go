package workspace

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrontmatterParser_ParseFrontmatter_RFC(t *testing.T) {
	parser := NewFrontmatterParser("local")

	input := `---
id: rfc-010
uuid: 550e8400-e29b-41d4-a716-446655440000
title: RFC-010: Diff Classification System
tags: [rfc, classification, diff, observability]
project: agf-iac-remediation-poc
owning_team: Platform Team
workflow_status: Draft
created: 2025-11-08
updated: 2025-11-08
sidebar_position: 10
document_type: rfc
---

# RFC-010

## Summary

This RFC proposes a diff classification system.

## Motivation

...
`

	meta, content, err := parser.ParseFrontmatter([]byte(input), "local:docs/rfc-010.md")
	require.NoError(t, err)

	// Verify UUID
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", meta.UUID.String())

	// Verify core metadata
	assert.Equal(t, "RFC-010: Diff Classification System", meta.Name)
	assert.Equal(t, []string{"rfc", "classification", "diff", "observability"}, meta.Tags)
	assert.Equal(t, "agf-iac-remediation-poc", meta.Project)
	assert.Equal(t, "Platform Team", meta.OwningTeam)
	assert.Equal(t, "Draft", meta.WorkflowStatus)

	// Verify provider info
	assert.Equal(t, "local", meta.ProviderType)
	assert.Equal(t, "local:docs/rfc-010.md", meta.ProviderID)

	// Verify timestamps
	assert.Equal(t, 2025, meta.CreatedTime.Year())
	assert.Equal(t, time.November, meta.CreatedTime.Month())
	assert.Equal(t, 8, meta.CreatedTime.Day())

	// Verify extended metadata
	assert.Equal(t, "rfc-010", meta.ExtendedMetadata["id"])
	assert.Equal(t, int64(10), meta.ExtendedMetadata["sidebar_position"])
	assert.Equal(t, "rfc", meta.ExtendedMetadata["document_type"])

	// Verify content
	assert.True(t, strings.Contains(content, "# RFC-010"))
	assert.True(t, strings.Contains(content, "## Summary"))
	assert.True(t, strings.Contains(content, "## Motivation"))

	// Verify content hash was calculated
	assert.NotEmpty(t, meta.ContentHash)
	assert.True(t, strings.HasPrefix(meta.ContentHash, "sha256:"))
}

func TestFrontmatterParser_ParseFrontmatter_Minimal(t *testing.T) {
	parser := NewFrontmatterParser("local")

	input := `---
title: Simple Document
---

Content here.
`

	meta, content, err := parser.ParseFrontmatter([]byte(input), "local:docs/simple.md")
	require.NoError(t, err)

	// UUID should be auto-generated
	assert.False(t, meta.UUID.IsZero())

	// Verify core fields
	assert.Equal(t, "Simple Document", meta.Name)
	assert.Equal(t, "local", meta.ProviderType)
	assert.Equal(t, "local:docs/simple.md", meta.ProviderID)

	// Default sync status
	assert.Equal(t, "canonical", meta.SyncStatus)

	// Default timestamps should be set
	assert.False(t, meta.CreatedTime.IsZero())
	assert.False(t, meta.ModifiedTime.IsZero())

	// Default MIME type
	assert.Equal(t, "text/markdown", meta.MimeType)

	// Content
	assert.Equal(t, "Content here.", content)
}

func TestFrontmatterParser_ParseFrontmatter_WithOwner(t *testing.T) {
	parser := NewFrontmatterParser("local")

	input := `---
title: Team Document
author: jacob.repp@hashicorp.com
owning_team: Engineering Team
tags: [api, documentation]
---

Content.
`

	meta, _, err := parser.ParseFrontmatter([]byte(input), "local:docs/doc.md")
	require.NoError(t, err)

	// Verify owner
	require.NotNil(t, meta.Owner)
	assert.Equal(t, "jacob.repp@hashicorp.com", meta.Owner.Email)
	assert.Equal(t, "jacob.repp@hashicorp.com", meta.Owner.DisplayName)

	// Verify team
	assert.Equal(t, "Engineering Team", meta.OwningTeam)
}

func TestFrontmatterParser_ParseFrontmatter_MultipleTimeFormats(t *testing.T) {
	testCases := []struct {
		name     string
		created  string
		expected time.Time
	}{
		{
			name:     "RFC3339",
			created:  "2025-11-08T14:30:00Z",
			expected: time.Date(2025, 11, 8, 14, 30, 0, 0, time.UTC),
		},
		{
			name:     "Date only",
			created:  "2025-11-08",
			expected: time.Date(2025, 11, 8, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "DateTime space",
			created:  "2025-11-08 14:30:00",
			expected: time.Date(2025, 11, 8, 14, 30, 0, 0, time.UTC),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewFrontmatterParser("local")

			input := "---\ntitle: Test\ncreated: " + tc.created + "\n---\n\nContent."

			meta, _, err := parser.ParseFrontmatter([]byte(input), "local:test.md")
			require.NoError(t, err)

			assert.Equal(t, tc.expected.Year(), meta.CreatedTime.Year())
			assert.Equal(t, tc.expected.Month(), meta.CreatedTime.Month())
			assert.Equal(t, tc.expected.Day(), meta.CreatedTime.Day())
		})
	}
}

func TestFrontmatterParser_ParseFrontmatter_ExtendedMetadata(t *testing.T) {
	parser := NewFrontmatterParser("local")

	input := `---
title: API Documentation
tags: [api, openapi]
api_version: 2.1
deprecated: false
sidebar_position: 5
custom_field: custom value
---

Content.
`

	meta, _, err := parser.ParseFrontmatter([]byte(input), "local:docs/api.md")
	require.NoError(t, err)

	// Core fields
	assert.Equal(t, "API Documentation", meta.Name)
	assert.Equal(t, []string{"api", "openapi"}, meta.Tags)

	// Extended metadata
	assert.Equal(t, 2.1, meta.ExtendedMetadata["api_version"])
	assert.Equal(t, false, meta.ExtendedMetadata["deprecated"])
	assert.Equal(t, int64(5), meta.ExtendedMetadata["sidebar_position"])
	assert.Equal(t, "custom value", meta.ExtendedMetadata["custom_field"])
}

func TestFrontmatterParser_SerializeFrontmatter(t *testing.T) {
	parser := NewFrontmatterParser("local")

	uuid := docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
	meta := &DocumentMetadata{
		UUID:           uuid,
		ProviderType:   "local",
		ProviderID:     "local:docs/rfc-010.md",
		Name:           "RFC-010: Test RFC",
		Tags:           []string{"rfc", "test"},
		Project:        "test-project",
		OwningTeam:     "Test Team",
		WorkflowStatus: "Draft",
		CreatedTime:    time.Date(2025, 11, 8, 0, 0, 0, 0, time.UTC),
		ModifiedTime:   time.Date(2025, 11, 8, 0, 0, 0, 0, time.UTC),
		Owner: &UserIdentity{
			Email:       "test@example.com",
			DisplayName: "Test User",
		},
		ExtendedMetadata: map[string]any{
			"id":               "rfc-010",
			"sidebar_position": 10,
			"document_type":    "rfc",
		},
	}

	content := "# RFC-010\n\n## Summary\n\nTest content."

	data := parser.SerializeFrontmatter(meta, content)

	// Parse back
	parsedMeta, parsedContent, err := parser.ParseFrontmatter(data, "local:docs/rfc-010.md")
	require.NoError(t, err)

	// Verify round-trip
	assert.Equal(t, uuid, parsedMeta.UUID)
	assert.Equal(t, "RFC-010: Test RFC", parsedMeta.Name)
	assert.Equal(t, []string{"rfc", "test"}, parsedMeta.Tags)
	assert.Equal(t, "test-project", parsedMeta.Project)
	assert.Equal(t, "Test Team", parsedMeta.OwningTeam)
	assert.Equal(t, "Draft", parsedMeta.WorkflowStatus)
	assert.Equal(t, "rfc-010", parsedMeta.ExtendedMetadata["id"])
	assert.Equal(t, content, parsedContent)
}

func TestFrontmatterParser_ParseFrontmatter_NoFrontmatter(t *testing.T) {
	parser := NewFrontmatterParser("local")

	input := `# Just Content

No frontmatter here.
`

	_, _, err := parser.ParseFrontmatter([]byte(input), "local:docs/plain.md")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing frontmatter opening")
}

func TestFrontmatterParser_ParseFrontmatter_InvalidUUID(t *testing.T) {
	parser := NewFrontmatterParser("local")

	input := `---
uuid: not-a-valid-uuid
title: Test
---

Content.
`

	_, _, err := parser.ParseFrontmatter([]byte(input), "local:test.md")
	// Should not error - just generate new UUID
	require.NoError(t, err)
}

func TestFrontmatterParser_ParseArray_Formats(t *testing.T) {
	parser := NewFrontmatterParser("local")

	testCases := []struct {
		input    string
		expected []string
	}{
		{"[a, b, c]", []string{"a", "b", "c"}},
		{"[tag1,tag2,tag3]", []string{"tag1", "tag2", "tag3"}},
		{"['quoted', 'values']", []string{"quoted", "values"}},
	}

	for _, tc := range testCases {
		result := parser.parseArray(tc.input)
		assert.Equal(t, tc.expected, result, "Input: %s", tc.input)
	}

	// Test empty array separately (nil vs empty slice)
	emptyResult := parser.parseArray("[]")
	assert.Empty(t, emptyResult, "Empty array should be empty")
}

func TestFrontmatterParser_ParseValue_TypeInference(t *testing.T) {
	parser := NewFrontmatterParser("local")

	testCases := []struct {
		input    string
		expected any
	}{
		{"true", true},
		{"false", false},
		{"42", int64(42)},
		{"3.14", 3.14},
		{"hello", "hello"},
		{"[a, b]", []string{"a", "b"}},
	}

	for _, tc := range testCases {
		result := parser.parseValue(tc.input)
		assert.Equal(t, tc.expected, result, "Input: %s", tc.input)
	}
}

func TestDefaultCoreFields(t *testing.T) {
	coreFields := DefaultCoreFields()

	// Verify key core fields are present
	assert.True(t, coreFields["uuid"])
	assert.True(t, coreFields["title"])
	assert.True(t, coreFields["tags"])
	assert.True(t, coreFields["project"])
	assert.True(t, coreFields["workflow_status"])

	// Verify non-core fields are not present
	assert.False(t, coreFields["id"])
	assert.False(t, coreFields["sidebar_position"])
	assert.False(t, coreFields["document_type"])
}
