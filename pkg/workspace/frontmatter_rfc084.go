package workspace

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
)

// FrontmatterParser parses YAML/TOML frontmatter into RFC-084 DocumentMetadata.
//
// Design Philosophy:
// - Core attributes: Parsed into type-safe struct fields (uuid, name, tags, etc.)
// - Extensible attributes: Stored in ExtendedMetadata map for document-type-specific fields
//
// Example frontmatter:
//
//	---
//	id: rfc-010
//	uuid: 550e8400-e29b-41d4-a716-446655440000
//	title: RFC-010: Diff Classification System
//	tags: [rfc, classification, diff]
//	project: agf-iac-remediation-poc
//	owning_team: Platform Team
//	workflow_status: Draft
//	created: 2025-11-08
//	updated: 2025-11-08
//	sidebar_position: 10
//	document_type: rfc
//	---
type FrontmatterParser struct {
	// ProviderType is the provider type for documents parsed (e.g., "local", "github")
	ProviderType string

	// CoreFields defines which frontmatter fields map to core DocumentMetadata attributes
	// vs ExtendedMetadata. If not specified, uses default mapping.
	CoreFields map[string]bool
}

// NewFrontmatterParser creates a parser with default core field mapping.
func NewFrontmatterParser(providerType string) *FrontmatterParser {
	return &FrontmatterParser{
		ProviderType: providerType,
		CoreFields:   DefaultCoreFields(),
	}
}

// DefaultCoreFields returns the default mapping of frontmatter fields to core attributes.
// Any field not in this set will be stored in ExtendedMetadata.
func DefaultCoreFields() map[string]bool {
	return map[string]bool{
		// Identity
		"uuid":          true,
		"hermes-uuid":   true, // Alternative UUID field name
		"provider_type": true,
		"provider_id":   true,

		// Core metadata
		"title":        true,
		"name":         true,
		"mime_type":    true,
		"created":      true,
		"created_time": true,
		"updated":      true,
		"modified":     true,
		"modified_time": true,

		// Ownership
		"owner":        true,
		"owning_team":  true,
		"author":       true,
		"contributors": true,

		// Organization
		"parents":  true,
		"project":  true,
		"project_id": true,
		"tags":     true,

		// Lifecycle
		"sync_status":     true,
		"workflow_status": true,
		"status":          true,
		"content_hash":    true,
	}
}

// ParseFrontmatter extracts RFC-084 DocumentMetadata and content from a document.
//
// Format: ---\n<yaml>\n---\n<content>
//
// Returns:
//   - DocumentMetadata with core attributes populated
//   - Document content (body after frontmatter)
//   - Error if parsing fails
func (p *FrontmatterParser) ParseFrontmatter(data []byte, providerID string) (*DocumentMetadata, string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	// Check for opening ---
	if !scanner.Scan() || scanner.Text() != "---" {
		return nil, "", fmt.Errorf("missing frontmatter opening '---'")
	}

	// Initialize metadata
	meta := &DocumentMetadata{
		ProviderType:     p.ProviderType,
		ProviderID:       providerID,
		ExtendedMetadata: make(map[string]any),
		Tags:             []string{},
		Contributors:     []UserIdentity{},
	}

	// Track fields for smart defaults
	hasTitle := false
	hasCreated := false
	hasModified := false

	// Parse YAML frontmatter
	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" {
			// End of frontmatter
			break
		}

		// Parse YAML key-value pairs
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, `"'`)

		// Parse based on field type
		if err := p.parseField(meta, key, value, &hasTitle, &hasCreated, &hasModified); err != nil {
			// Log warning but don't fail - just store in extended metadata
			meta.ExtendedMetadata[key] = value
		}
	}

	// Apply smart defaults
	p.applyDefaults(meta, hasTitle, hasCreated, hasModified)

	// Read remaining content
	var contentBuf bytes.Buffer
	for scanner.Scan() {
		contentBuf.WriteString(scanner.Text())
		contentBuf.WriteString("\n")
	}

	content := strings.TrimSpace(contentBuf.String())

	// Calculate content hash
	if meta.ContentHash == "" {
		hash := sha256.Sum256([]byte(content))
		meta.ContentHash = "sha256:" + hex.EncodeToString(hash[:])
	}

	return meta, content, nil
}

// parseField parses a single frontmatter field into DocumentMetadata.
func (p *FrontmatterParser) parseField(meta *DocumentMetadata, key, value string, hasTitle, hasCreated, hasModified *bool) error {
	// Normalize key for comparison
	normalizedKey := strings.ToLower(strings.ReplaceAll(key, "-", "_"))

	// Check if this is a core field
	isCoreField := p.CoreFields[key] || p.CoreFields[normalizedKey]

	if !isCoreField {
		// Store in ExtendedMetadata
		meta.ExtendedMetadata[key] = p.parseValue(value)
		return nil
	}

	// Parse core fields
	switch normalizedKey {
	case "uuid", "hermes_uuid":
		if value != "" {
			uuid, err := docid.ParseUUID(value)
			if err != nil {
				return fmt.Errorf("invalid UUID: %w", err)
			}
			meta.UUID = uuid
		}

	case "provider_type":
		meta.ProviderType = value

	case "provider_id":
		meta.ProviderID = value

	case "title", "name":
		meta.Name = value
		*hasTitle = true

	case "mime_type":
		meta.MimeType = value

	case "created", "created_time":
		if t, err := p.parseTime(value); err == nil {
			meta.CreatedTime = t
			*hasCreated = true
		}

	case "updated", "modified", "modified_time":
		if t, err := p.parseTime(value); err == nil {
			meta.ModifiedTime = t
			*hasModified = true
		}

	case "owner", "author":
		// Simple string owner for now - can be enhanced to parse email
		meta.Owner = &UserIdentity{
			Email:       value,
			DisplayName: value,
		}

	case "owning_team":
		meta.OwningTeam = value

	case "project", "project_id":
		meta.Project = value

	case "tags":
		meta.Tags = p.parseArray(value)

	case "sync_status":
		meta.SyncStatus = value

	case "workflow_status", "status":
		meta.WorkflowStatus = value

	case "content_hash":
		meta.ContentHash = value

	default:
		// Unknown core field - store in extended metadata
		meta.ExtendedMetadata[key] = p.parseValue(value)
	}

	return nil
}

// parseTime parses time strings in various formats.
func (p *FrontmatterParser) parseTime(value string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported time format: %s", value)
}

// parseArray parses YAML array syntax: [item1, item2, item3]
func (p *FrontmatterParser) parseArray(value string) []string {
	// Remove brackets
	value = strings.Trim(value, "[]")

	// Split by comma
	items := strings.Split(value, ",")

	var result []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, `"'`)
		if item != "" {
			result = append(result, item)
		}
	}

	return result
}

// parseValue attempts to parse a value into appropriate type.
func (p *FrontmatterParser) parseValue(value string) any {
	// Try boolean
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}

	// Try integer
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i
	}

	// Try float
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}

	// Try array
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return p.parseArray(value)
	}

	// Default to string
	return value
}

// applyDefaults applies smart defaults to metadata fields.
func (p *FrontmatterParser) applyDefaults(meta *DocumentMetadata, hasTitle, hasCreated, hasModified bool) {
	// Default UUID if not present
	if meta.UUID.IsZero() {
		meta.UUID = docid.NewUUID()
	}

	// Default sync status
	if meta.SyncStatus == "" {
		meta.SyncStatus = "canonical" // Documents authored locally are canonical
	}

	// Default timestamps
	now := time.Now().UTC()
	if !hasCreated {
		meta.CreatedTime = now
	}
	if !hasModified {
		meta.ModifiedTime = now
	}

	// Default MIME type based on file extension
	if meta.MimeType == "" {
		meta.MimeType = "text/markdown"
	}
}

// SerializeFrontmatter creates YAML frontmatter from RFC-084 DocumentMetadata.
//
// Returns: Document bytes with frontmatter and content
func (p *FrontmatterParser) SerializeFrontmatter(meta *DocumentMetadata, content string) []byte {
	var buf bytes.Buffer

	buf.WriteString("---\n")

	// Write core fields
	if !meta.UUID.IsZero() {
		buf.WriteString(fmt.Sprintf("uuid: %s\n", meta.UUID.String()))
	}
	if meta.Name != "" {
		buf.WriteString(fmt.Sprintf("title: %s\n", meta.Name))
	}
	if len(meta.Tags) > 0 {
		buf.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(meta.Tags, ", ")))
	}
	if meta.Project != "" {
		buf.WriteString(fmt.Sprintf("project: %s\n", meta.Project))
	}
	if meta.OwningTeam != "" {
		buf.WriteString(fmt.Sprintf("owning_team: %s\n", meta.OwningTeam))
	}
	if meta.WorkflowStatus != "" {
		buf.WriteString(fmt.Sprintf("workflow_status: %s\n", meta.WorkflowStatus))
	}
	if !meta.CreatedTime.IsZero() {
		buf.WriteString(fmt.Sprintf("created: %s\n", meta.CreatedTime.Format("2006-01-02")))
	}
	if !meta.ModifiedTime.IsZero() {
		buf.WriteString(fmt.Sprintf("updated: %s\n", meta.ModifiedTime.Format("2006-01-02")))
	}
	if meta.Owner != nil && meta.Owner.Email != "" {
		buf.WriteString(fmt.Sprintf("author: %s\n", meta.Owner.Email))
	}

	// Write extended metadata
	for key, value := range meta.ExtendedMetadata {
		buf.WriteString(fmt.Sprintf("%s: %v\n", key, value))
	}

	buf.WriteString("---\n\n")
	buf.WriteString(content)

	return buf.Bytes()
}
