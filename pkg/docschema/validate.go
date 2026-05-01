// Package docschema validates discovered documentation files.
package docschema

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hashicorp-forge/hermes/pkg/docdiscover"
)

// Severity identifies diagnostic severity.
type Severity string

const (
	// SeverityError marks a validation failure.
	SeverityError Severity = "error"
	// SeverityWarning marks a non-blocking validation warning.
	SeverityWarning Severity = "warning"
)

// Diagnostic is a stable validation diagnostic.
type Diagnostic struct {
	File         string   `json:"file"`
	Line         int      `json:"line,omitempty"`
	Field        string   `json:"field,omitempty"`
	Severity     Severity `json:"severity"`
	Message      string   `json:"message"`
	SuggestedFix string   `json:"suggestedFix,omitempty"`
}

// Result contains discovered documents and diagnostics.
type Result struct {
	Documents   []docdiscover.Document `json:"documents"`
	Diagnostics []Diagnostic           `json:"diagnostics"`
}

// ValidateDocuments validates discovered documents.
func ValidateDocuments(documents []docdiscover.Document) Result {
	diagnostics := make([]Diagnostic, 0, len(documents))
	for _, doc := range documents {
		diagnostics = append(diagnostics, ValidateDocument(doc)...)
	}
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].File != diagnostics[j].File {
			return diagnostics[i].File < diagnostics[j].File
		}
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		return diagnostics[i].Field < diagnostics[j].Field
	})
	return Result{Documents: documents, Diagnostics: diagnostics}
}

// ValidateDocument validates a single discovered document.
func ValidateDocument(doc docdiscover.Document) []Diagnostic {
	data, err := os.ReadFile(doc.Path) //nolint:gosec // G304: path comes from local discovery.
	if err != nil {
		return []Diagnostic{{File: doc.RelPath, Severity: SeverityError, Message: fmt.Sprintf("read document: %v", err)}}
	}

	diagnostics := make([]Diagnostic, 0)
	if doc.Lane != nil && doc.Lane.FilenamePattern != "" {
		matched, err := regexp.MatchString(doc.Lane.FilenamePattern, filepath.Base(doc.Path))
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{File: doc.RelPath, Field: "filename_pattern", Severity: SeverityError, Message: fmt.Sprintf("invalid filename pattern: %v", err), SuggestedFix: "Fix the lane filename_pattern in project HCL."})
		} else if doc.Lane.EnforceFilenamePattern && !matched {
			diagnostics = append(diagnostics, Diagnostic{File: doc.RelPath, Field: "filename", Severity: SeverityError, Message: "filename does not match lane pattern", SuggestedFix: fmt.Sprintf("Rename the file to match %s.", doc.Lane.FilenamePattern)})
		}
	}

	frontmatter, _, hasFrontmatter, parseErr := parseFrontmatter(data)
	if parseErr != nil {
		diagnostics = append(diagnostics, Diagnostic{File: doc.RelPath, Line: 1, Field: "frontmatter", Severity: SeverityError, Message: parseErr.Error(), SuggestedFix: "Use YAML frontmatter delimited by --- lines."})
		return diagnostics
	}

	requireFrontmatter := doc.Lane != nil && doc.Lane.RequireFrontmatter
	if !hasFrontmatter {
		if requireFrontmatter {
			diagnostics = append(diagnostics, Diagnostic{File: doc.RelPath, Line: 1, Field: "frontmatter", Severity: SeverityError, Message: "missing required frontmatter", SuggestedFix: "Add YAML frontmatter with required schema fields."})
		}
		return diagnostics
	}

	required, unknownSchema := requiredFields(doc.Schema)
	if unknownSchema {
		diagnostics = append(diagnostics, Diagnostic{File: doc.RelPath, Field: "schema", Severity: SeverityError, Message: fmt.Sprintf("unknown schema %q", doc.Schema), SuggestedFix: "Use one of: adr, rfc, memo, prd, generic."})
		return diagnostics
	}

	for _, field := range required {
		if value, ok := frontmatter[field]; !ok || isEmpty(value) {
			diagnostics = append(diagnostics, Diagnostic{File: doc.RelPath, Field: field, Severity: SeverityError, Message: fmt.Sprintf("missing required field %q", field), SuggestedFix: fmt.Sprintf("Add %s to frontmatter.", field)})
		}
	}

	return diagnostics
}

func parseFrontmatter(data []byte) (map[string]any, map[string]int, bool, error) {
	if !bytes.HasPrefix(data, []byte("---\n")) && !bytes.Equal(bytes.TrimSpace(data), []byte("---")) {
		return nil, nil, false, nil
	}

	lines := strings.Split(string(data), "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, nil, true, fmt.Errorf("unterminated frontmatter")
	}

	frontmatterText := strings.Join(lines[1:end], "\n")
	fields := map[string]any{}
	if strings.TrimSpace(frontmatterText) != "" {
		if err := yaml.Unmarshal([]byte(frontmatterText), &fields); err != nil {
			return nil, nil, true, fmt.Errorf("invalid frontmatter YAML: %w", err)
		}
	}

	fieldLines := make(map[string]int, len(fields))
	for i := 1; i < end; i++ {
		line := lines[i]
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if key != "" {
			fieldLines[key] = i + 1
		}
	}

	return fields, fieldLines, true, nil
}

func requiredFields(schema string) ([]string, bool) {
	switch strings.ToLower(schema) {
	case "adr":
		return []string{"title", "type", "status", "created", "project_id", "doc_uuid"}, false
	case "rfc":
		return []string{"title", "type", "status", "created", "project_id", "doc_uuid"}, false
	case "memo":
		return []string{"title", "type", "created", "project_id", "doc_uuid"}, false
	case "prd":
		return []string{"title", "type", "status", "project_id", "doc_uuid"}, false
	case "generic":
		return nil, false
	default:
		return nil, true
	}
}

func isEmpty(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}
