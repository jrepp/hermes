package ruleset

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp-forge/hermes/pkg/models"
)

// Ruleset defines when and how to process a document revision.
type Ruleset struct {
	Name       string                 `hcl:"name,label"`
	Conditions map[string]string      `hcl:"conditions,optional"`
	Pipeline   []string               `hcl:"pipeline"`
	Config     map[string]interface{} `hcl:"config,optional"`
}

// Rulesets is a collection of rulesets.
type Rulesets []Ruleset

// Matcher matches document revisions against rulesets.
type Matcher struct {
	rulesets Rulesets
}

// NewMatcher creates a new ruleset matcher.
func NewMatcher(rulesets Rulesets) *Matcher {
	return &Matcher{
		rulesets: rulesets,
	}
}

// Match returns all rulesets that match the given revision and metadata.
// Rulesets are evaluated in order, and multiple rulesets can match.
func (m *Matcher) Match(revision *models.DocumentRevision, metadata map[string]interface{}) []Ruleset {
	var matched []Ruleset

	for _, ruleset := range m.rulesets {
		if ruleset.Matches(revision, metadata) {
			matched = append(matched, ruleset)
		}
	}

	return matched
}

// Matches checks if this ruleset matches the given revision and metadata.
func (r *Ruleset) Matches(revision *models.DocumentRevision, metadata map[string]interface{}) bool {
	// If no conditions, match all (default ruleset)
	if len(r.Conditions) == 0 {
		return true
	}

	// All conditions must match (AND logic)
	for key, expected := range r.Conditions {
		if !r.matchCondition(key, expected, revision, metadata) {
			return false
		}
	}

	return true
}

// matchCondition checks if a single condition matches.
func (r *Ruleset) matchCondition(key, expected string, revision *models.DocumentRevision, metadata map[string]interface{}) bool {
	// Get actual value from revision or metadata
	actual := r.getValue(key, revision, metadata)

	// Handle different condition operators
	if strings.Contains(key, "_gt") {
		// Greater than comparison (e.g., content_length_gt: "5000")
		return r.compareGreaterThan(actual, expected)
	}

	if strings.Contains(key, "_lt") {
		// Less than comparison (e.g., content_length_lt: "1000")
		return r.compareLessThan(actual, expected)
	}

	if strings.Contains(key, "_contains") {
		// Contains comparison (e.g., title_contains: "RFC")
		return r.compareContains(actual, expected)
	}

	// Default: exact match or IN operator (comma-separated values)
	return r.compareEquals(actual, expected)
}

// getValue extracts the value for a given key from revision or metadata.
func (r *Ruleset) getValue(key string, revision *models.DocumentRevision, metadata map[string]interface{}) interface{} {
	// Strip operator suffixes
	key = strings.TrimSuffix(key, "_gt")
	key = strings.TrimSuffix(key, "_lt")
	key = strings.TrimSuffix(key, "_contains")

	// Check revision fields first
	switch key {
	case "provider_type":
		return revision.ProviderType
	case "status":
		return revision.Status
	case "document_id":
		return revision.DocumentID
	case "document_uuid":
		return revision.DocumentUUID.String()
	case "title":
		return revision.Title
	case "content_hash":
		return revision.ContentHash
	}

	// Check metadata
	if val, ok := metadata[key]; ok {
		return val
	}

	return nil
}

// compareEquals checks if actual equals expected (or is in comma-separated list).
func (r *Ruleset) compareEquals(actual interface{}, expected string) bool {
	if actual == nil {
		return false
	}

	actualStr := fmt.Sprintf("%v", actual)

	// Check if expected is a comma-separated list (IN operator)
	if strings.Contains(expected, ",") {
		values := strings.Split(expected, ",")
		for _, val := range values {
			if strings.TrimSpace(val) == actualStr {
				return true
			}
		}
		return false
	}

	// Exact match
	return actualStr == expected
}

// compareGreaterThan checks if actual > expected (numeric comparison).
func (r *Ruleset) compareGreaterThan(actual interface{}, expected string) bool {
	if actual == nil {
		return false
	}

	actualNum, err := r.toNumber(actual)
	if err != nil {
		return false
	}

	expectedNum, err := strconv.ParseFloat(expected, 64)
	if err != nil {
		return false
	}

	return actualNum > expectedNum
}

// compareLessThan checks if actual < expected (numeric comparison).
func (r *Ruleset) compareLessThan(actual interface{}, expected string) bool {
	if actual == nil {
		return false
	}

	actualNum, err := r.toNumber(actual)
	if err != nil {
		return false
	}

	expectedNum, err := strconv.ParseFloat(expected, 64)
	if err != nil {
		return false
	}

	return actualNum < expectedNum
}

// compareContains checks if actual contains expected (string comparison).
func (r *Ruleset) compareContains(actual interface{}, expected string) bool {
	if actual == nil {
		return false
	}

	actualStr := fmt.Sprintf("%v", actual)
	return strings.Contains(strings.ToLower(actualStr), strings.ToLower(expected))
}

// toNumber converts a value to a float64.
func (r *Ruleset) toNumber(val interface{}) (float64, error) {
	switch v := val.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to number", val)
	}
}

// GetStepConfig returns the configuration for a specific pipeline step.
func (r *Ruleset) GetStepConfig(stepName string) map[string]interface{} {
	if r.Config == nil {
		return nil
	}

	if stepConfig, ok := r.Config[stepName]; ok {
		if configMap, ok := stepConfig.(map[string]interface{}); ok {
			return configMap
		}
	}

	return nil
}

// Validate checks if the ruleset configuration is valid.
func (r *Ruleset) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("ruleset name is required")
	}

	if len(r.Pipeline) == 0 {
		return fmt.Errorf("ruleset %s: pipeline steps are required", r.Name)
	}

	// Validate pipeline step names (basic check)
	validSteps := map[string]bool{
		"search_index":     true,
		"embeddings":       true,
		"llm_summary":      true,
		"validation":       true,
		"llm_validation":   true,
		"link_extraction":  true,
		"metadata_extract": true,
	}

	for _, step := range r.Pipeline {
		if !validSteps[step] {
			return fmt.Errorf("ruleset %s: unknown pipeline step %q", r.Name, step)
		}
	}

	return nil
}

// ValidateAll validates all rulesets in the collection.
func (rs Rulesets) ValidateAll() error {
	if len(rs) == 0 {
		return fmt.Errorf("at least one ruleset is required")
	}

	for _, ruleset := range rs {
		if err := ruleset.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Example ruleset configurations:
//
// Ruleset 1: Published RFCs get full processing
// {
//   Name: "published-rfcs",
//   Conditions: {
//     "document_type": "RFC",
//     "status": "In-Review,Approved",
//     "provider_type": "google",
//   },
//   Pipeline: ["search_index", "embeddings", "llm_summary"],
//   Config: {
//     "embeddings": {
//       "model": "text-embedding-3-small",
//       "dimensions": 1536,
//     },
//     "llm_summary": {
//       "model": "gpt-4o-mini",
//       "max_tokens": 500,
//     },
//   },
// }
//
// Ruleset 2: All documents get search indexing
// {
//   Name: "all-documents",
//   Conditions: {}, // Matches all
//   Pipeline: ["search_index"],
// }
//
// Ruleset 3: Long design docs get deep analysis
// {
//   Name: "design-docs-deep",
//   Conditions: {
//     "document_type": "PRD,RFC",
//     "content_length_gt": "5000",
//   },
//   Pipeline: ["search_index", "embeddings", "llm_summary", "llm_validation"],
//   Config: {
//     "llm_validation": {
//       "checks": ["has_motivation", "has_alternatives"],
//     },
//   },
// }
