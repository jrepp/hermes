// Package docrepair plans and applies safe documentation repairs.
package docrepair

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/hashicorp-forge/hermes/pkg/docdiscover"
	"github.com/hashicorp-forge/hermes/pkg/docschema"
)

// Operation identifies a repair operation.
type Operation string

const (
	// OperationMigrate normalizes frontmatter fields.
	OperationMigrate Operation = "migrate"
	// OperationBulkUpdate sets or removes one frontmatter field.
	OperationBulkUpdate Operation = "bulk_update"
	// OperationTimestamps backfills missing created timestamps.
	OperationTimestamps Operation = "timestamps"
	// OperationCompressIDs compresses lane-local document IDs.
	OperationCompressIDs Operation = "compress_ids"
	// OperationLinks checks relative Markdown links.
	OperationLinks Operation = "links"
)

// Options controls repair planning and application.
type Options struct {
	Operation Operation
	Apply     bool
	Field     string
	Value     string
	Remove    bool
	StartID   int
	Now       time.Time
}

// Plan describes planned or applied documentation changes.
type Plan struct {
	Operation Operation `json:"operation"`
	DryRun    bool      `json:"dryRun"`
	Changes   []Change  `json:"changes"`
	Warnings  []Warning `json:"warnings,omitempty"`
}

// Change describes one file mutation or check result.
type Change struct {
	File    string `json:"file"`
	Action  string `json:"action"`
	Field   string `json:"field,omitempty"`
	Before  string `json:"before,omitempty"`
	After   string `json:"after,omitempty"`
	Applied bool   `json:"applied"`
}

// Warning describes a degraded or non-mutating repair finding.
type Warning struct {
	File    string `json:"file,omitempty"`
	Message string `json:"message"`
}

// Execute plans and optionally applies a repair operation.
func Execute(documents []docdiscover.Document, opts Options) (Plan, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	plan := Plan{Operation: opts.Operation, DryRun: !opts.Apply}

	switch opts.Operation {
	case OperationMigrate:
		return executeFrontmatter(documents, opts, migrateDocument)
	case OperationBulkUpdate:
		return executeFrontmatter(documents, opts, bulkUpdateDocument)
	case OperationTimestamps:
		plan.Warnings = append(plan.Warnings, Warning{Message: "git history timestamp derivation is not implemented in this phase; using current timestamp for missing created fields"})
		return executeFrontmatter(documents, opts, timestampDocumentWithCurrentTime)
	case OperationCompressIDs:
		return executeCompressIDs(documents, opts)
	case OperationLinks:
		return checkLinks(documents, opts), nil
	default:
		return plan, fmt.Errorf("unsupported repair operation %q", opts.Operation)
	}
}

type frontmatterMutator func(doc docdiscover.Document, fm map[string]any, opts Options) ([]Change, bool)

func executeFrontmatter(documents []docdiscover.Document, opts Options, mutator frontmatterMutator) (Plan, error) {
	plan := Plan{Operation: opts.Operation, DryRun: !opts.Apply}
	for _, doc := range documents {
		parsed, err := readDocument(doc)
		if err != nil {
			plan.Warnings = append(plan.Warnings, Warning{File: doc.RelPath, Message: err.Error()})
			continue
		}
		changes, changed := mutator(doc, parsed.Frontmatter, opts)
		plan.Changes = append(plan.Changes, changes...)
		if !changed || !opts.Apply {
			continue
		}
		if err := writeDocument(doc.Path, parsed.Frontmatter, parsed.Body, parsed.HadFrontmatter); err != nil {
			return plan, err
		}
		markApplied(plan.Changes[len(plan.Changes)-len(changes):])
	}
	return plan, nil
}

func migrateDocument(doc docdiscover.Document, fm map[string]any, _ Options) ([]Change, bool) {
	changes := make([]Change, 0)
	changed := false

	if !hasField(fm, "title") {
		title := strings.TrimSuffix(filepath.Base(doc.Path), filepath.Ext(doc.Path))
		fm["title"] = title
		changes = append(changes, Change{File: doc.RelPath, Action: "set", Field: "title", After: title})
		changed = true
	}
	if !hasField(fm, "type") && doc.Schema != "generic" {
		docType := canonicalType(doc.Schema)
		fm["type"] = docType
		changes = append(changes, Change{File: doc.RelPath, Action: "set", Field: "type", After: docType})
		changed = true
	}
	if !hasField(fm, "status") && (doc.Schema == "adr" || doc.Schema == "rfc" || doc.Schema == "prd") {
		fm["status"] = "Draft"
		changes = append(changes, Change{File: doc.RelPath, Action: "set", Field: "status", After: "Draft"})
		changed = true
	}
	if !hasField(fm, "project_id") && doc.ProjectName != "" {
		fm["project_id"] = doc.ProjectName
		changes = append(changes, Change{File: doc.RelPath, Action: "set", Field: "project_id", After: doc.ProjectName})
		changed = true
	}
	if !hasField(fm, "created") {
		created := time.Now().UTC().Format("2006-01-02")
		fm["created"] = created
		changes = append(changes, Change{File: doc.RelPath, Action: "set", Field: "created", After: created})
		changed = true
	}
	if !hasField(fm, "doc_uuid") {
		uuid := pseudoUUID()
		fm["doc_uuid"] = uuid
		changes = append(changes, Change{File: doc.RelPath, Action: "set", Field: "doc_uuid", After: uuid})
		changed = true
	}
	if value, ok := fm["date"]; ok && !hasField(fm, "created") {
		fm["created"] = value
		changes = append(changes, Change{File: doc.RelPath, Action: "rename", Field: "date", Before: fmt.Sprint(value), After: fmt.Sprint(value)})
		changed = true
	}

	return changes, changed
}

func bulkUpdateDocument(doc docdiscover.Document, fm map[string]any, opts Options) ([]Change, bool) {
	if opts.Field == "" {
		return nil, false
	}
	before, had := fm[opts.Field]
	if opts.Remove {
		if !had {
			return nil, false
		}
		delete(fm, opts.Field)
		return []Change{{File: doc.RelPath, Action: "remove", Field: opts.Field, Before: fmt.Sprint(before)}}, true
	}
	if had && fmt.Sprint(before) == opts.Value {
		return nil, false
	}
	fm[opts.Field] = opts.Value
	return []Change{{File: doc.RelPath, Action: "set", Field: opts.Field, Before: fmt.Sprint(before), After: opts.Value}}, true
}

func timestampDocumentWithCurrentTime(doc docdiscover.Document, fm map[string]any, opts Options) ([]Change, bool) {
	if hasField(fm, "created") {
		return nil, false
	}
	created := opts.Now.UTC().Format("2006-01-02")
	fm["created"] = created
	return []Change{{File: doc.RelPath, Action: "set", Field: "created", After: created}}, true
}

func executeCompressIDs(documents []docdiscover.Document, opts Options) (Plan, error) {
	plan := Plan{Operation: opts.Operation, DryRun: !opts.Apply}
	if opts.StartID <= 0 {
		opts.StartID = 1
	}

	sorted := append([]docdiscover.Document(nil), documents...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].RelPath < sorted[j].RelPath })

	for i, doc := range sorted {
		parsed, err := readDocument(doc)
		if err != nil {
			plan.Warnings = append(plan.Warnings, Warning{File: doc.RelPath, Message: err.Error()})
			continue
		}
		id := fmt.Sprintf("%03d", opts.StartID+i)
		before := fmt.Sprint(parsed.Frontmatter["id"])
		if before == id {
			continue
		}
		parsed.Frontmatter["id"] = id
		change := Change{File: doc.RelPath, Action: "set", Field: "id", Before: before, After: id}
		if opts.Apply {
			if err := writeDocument(doc.Path, parsed.Frontmatter, parsed.Body, parsed.HadFrontmatter); err != nil {
				return plan, err
			}
			change.Applied = true
		}
		plan.Changes = append(plan.Changes, change)
	}
	return plan, nil
}

func checkLinks(documents []docdiscover.Document, opts Options) Plan {
	plan := Plan{Operation: opts.Operation, DryRun: !opts.Apply}
	linkPattern := regexp.MustCompile(`\[[^\]]+\]\(([^)#][^)]+\.md(?:#[^)]+)?)\)`)
	known := make(map[string]bool, len(documents))
	for _, doc := range documents {
		known[filepath.Clean(doc.Path)] = true
	}

	for _, doc := range documents {
		data, err := os.ReadFile(doc.Path) //nolint:gosec // G304: path comes from local discovery.
		if err != nil {
			plan.Warnings = append(plan.Warnings, Warning{File: doc.RelPath, Message: err.Error()})
			continue
		}
		matches := linkPattern.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			target := strings.Split(match[1], "#")[0]
			targetPath := filepath.Clean(filepath.Join(filepath.Dir(doc.Path), target))
			if !known[targetPath] {
				plan.Warnings = append(plan.Warnings, Warning{File: doc.RelPath, Message: fmt.Sprintf("missing relative link target %s", target)})
			}
		}
	}
	return plan
}

type parsedDocument struct {
	Frontmatter    map[string]any
	Body           string
	HadFrontmatter bool
}

func readDocument(doc docdiscover.Document) (parsedDocument, error) {
	data, err := os.ReadFile(doc.Path) //nolint:gosec // G304: path comes from local discovery.
	if err != nil {
		return parsedDocument{}, err
	}
	fm, body, had, err := splitFrontmatter(data)
	if err != nil {
		return parsedDocument{}, err
	}
	if !had && doc.Lane != nil && doc.Lane.RequireFrontmatter {
		fm = map[string]any{}
	}
	return parsedDocument{Frontmatter: fm, Body: body, HadFrontmatter: had}, nil
}

func splitFrontmatter(data []byte) (map[string]any, string, bool, error) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return map[string]any{}, string(data), false, nil
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
		return nil, "", true, fmt.Errorf("unterminated frontmatter")
	}
	fm := map[string]any{}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &fm); err != nil {
		return nil, "", true, fmt.Errorf("invalid frontmatter YAML: %w", err)
	}
	body := strings.Join(lines[end+1:], "\n")
	return fm, body, true, nil
}

func writeDocument(path string, fm map[string]any, body string, _ bool) error {
	data, err := marshalFrontmatter(fm)
	if err != nil {
		return err
	}
	if body != "" && !strings.HasPrefix(body, "\n") {
		data = append(data, '\n')
	}
	data = append(data, []byte(body)...)
	return os.WriteFile(path, data, 0o644) //nolint:gosec // permissions are normal markdown file permissions.
}

func marshalFrontmatter(fm map[string]any) ([]byte, error) {
	keys := make([]string, 0, len(fm))
	for key := range fm {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("---\n")
	for _, key := range keys {
		value := fm[key]
		b.WriteString(key)
		b.WriteString(": ")
		switch v := value.(type) {
		case []any, []string, map[string]any:
			encoded, err := yaml.Marshal(v)
			if err != nil {
				return nil, err
			}
			b.WriteString(strings.TrimSpace(string(encoded)))
		default:
			_, _ = fmt.Fprint(&b, value)
		}
		b.WriteString("\n")
	}
	b.WriteString("---\n")
	return []byte(b.String()), nil
}

func hasField(fm map[string]any, field string) bool {
	value, ok := fm[field]
	return ok && strings.TrimSpace(fmt.Sprint(value)) != ""
}

func canonicalType(schema string) string {
	switch strings.ToLower(schema) {
	case "adr":
		return "ADR"
	case "rfc":
		return "RFC"
	case "memo":
		return "Memo"
	case "prd":
		return "PRD"
	default:
		return schema
	}
}

func pseudoUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-4000-8000-" + strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	hexed := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexed[0:8], hexed[8:12], hexed[12:16], hexed[16:20], hexed[20:32])
}

func markApplied(changes []Change) {
	for i := range changes {
		changes[i].Applied = true
	}
}

// ValidateAfterApply validates repaired documents.
func ValidateAfterApply(documents []docdiscover.Document) docschema.Result {
	return docschema.ValidateDocuments(documents)
}
