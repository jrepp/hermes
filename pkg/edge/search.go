package edge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docdiscover"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
	"github.com/hashicorp-forge/hermes/pkg/search/bm25"
)

// IndexResult describes local cache indexing.
type IndexResult struct {
	IndexedAt time.Time       `json:"indexedAt"`
	Documents []IndexDocument `json:"documents"`
	Skipped   []IndexSkip     `json:"skipped,omitempty"`
}

// IndexDocument is local index metadata.
type IndexDocument struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Project   string `json:"project"`
	Lane      string `json:"lane"`
	Hash      string `json:"hash"`
	Provider  string `json:"provider"`
	Unchanged bool   `json:"unchanged,omitempty"`
}

// IndexSkip describes a skipped local document.
type IndexSkip struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// SearchResult describes local edge search results.
type SearchResult struct {
	Query   string      `json:"query"`
	Mode    string      `json:"mode"`
	Hits    []SearchHit `json:"hits"`
	Indexed int         `json:"indexed"`
}

// SearchHit is a local search hit.
type SearchHit struct {
	Path    string         `json:"path"`
	Title   string         `json:"title,omitempty"`
	Project string         `json:"project"`
	Lane    string         `json:"lane"`
	Score   float64        `json:"score"`
	Terms   []string       `json:"terms"`
	Debug   map[string]any `json:"debug,omitempty"`
}

// BuildIndex builds an in-memory local BM25 index from discovered docs.
func BuildIndex(opts Options) (*bm25.Index, IndexResult, error) {
	config, docs, err := load(opts)
	if err != nil {
		return nil, IndexResult{}, err
	}
	idx := bm25.New()
	result := IndexResult{IndexedAt: time.Now().UTC()}
	for _, doc := range docs {
		indexed, searchDoc, err := buildIndexDocument(config, doc)
		if err != nil {
			result.Skipped = append(result.Skipped, IndexSkip{Path: doc.RelPath, Reason: err.Error()})
			continue
		}
		idx.Add(searchDoc)
		result.Documents = append(result.Documents, indexed)
	}
	return idx, result, nil
}

// SearchLocal builds a local index and searches it.
func SearchLocal(opts Options, query string, limit int, debug bool) (SearchResult, error) {
	idx, indexResult, err := BuildIndex(opts)
	if err != nil {
		return SearchResult{}, err
	}
	results := idx.Search(query, limit)
	hits := make([]SearchHit, 0, len(results))
	for _, result := range results {
		hit := SearchHit{
			Path:    fmt.Sprint(result.Document.Metadata["path"]),
			Title:   result.Document.Title,
			Project: fmt.Sprint(result.Document.Metadata["project"]),
			Lane:    fmt.Sprint(result.Document.Metadata["lane"]),
			Score:   result.Score,
			Terms:   result.Terms,
		}
		if debug {
			hit.Debug = map[string]any{"score_components": "bm25", "document_id": result.Document.ID}
		}
		hits = append(hits, hit)
	}
	return SearchResult{Query: query, Mode: "bm25", Hits: hits, Indexed: len(indexResult.Documents)}, nil
}

func buildIndexDocument(config *projectconfig.Config, doc docdiscover.Document) (IndexDocument, bm25.Document, error) {
	data, err := os.ReadFile(doc.Path) //nolint:gosec // path comes from local discovery.
	if err != nil {
		return IndexDocument{}, bm25.Document{}, err
	}
	hash := sha256.Sum256(data)
	id := doc.RelPath
	if uuid := extractDocUUID(data); uuid != "" {
		id = uuid
	}
	title := extractTitle(data)
	provider := "local"
	if doc.Project != nil {
		if active, err := doc.Project.GetActiveProvider(); err == nil {
			provider = active.Type
		}
	}
	indexed := IndexDocument{ID: id, Path: doc.RelPath, Project: doc.ProjectName, Lane: doc.LaneName, Hash: hex.EncodeToString(hash[:]), Provider: provider}
	searchDoc := bm25.Document{
		ID:      id,
		Title:   title,
		Content: stripFrontmatter(string(data)),
		Metadata: map[string]any{
			"path":     doc.RelPath,
			"project":  doc.ProjectName,
			"lane":     doc.LaneName,
			"provider": provider,
			"root":     config.WorkspaceBasePath,
		},
	}
	return indexed, searchDoc, nil
}

func extractTitle(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "title:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "title:")), `"'`)
		}
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	parts := strings.SplitN(content, "\n---\n", 2)
	if len(parts) != 2 {
		return content
	}
	return parts[1]
}

// IndexCachePath returns a deterministic placeholder cache path for future persisted local indexes.
func IndexCachePath(root, project string) string {
	return filepath.Join(root, ".hermes", "edge", project, "bm25")
}
