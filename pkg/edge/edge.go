// Package edge provides local edge discovery and sync status planning.
package edge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docdiscover"
	"github.com/hashicorp-forge/hermes/pkg/docschema"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
)

const remoteStateUnavailable = "unavailable"

// Options configures edge discovery/status.
type Options struct {
	ConfigPath          string
	RootDir             string
	Project             string
	QdrantURL           string
	QdrantAPIKey        string
	QdrantCollection    string
	EmbeddingModel      string
	OllamaURL           string
	OpenAIAPIKey        string
	ProbeRemote         bool
	Timeout             time.Duration
	EmbeddingDimensions int
	HTTPClient          *http.Client
	VectorIndex         VectorIndex
	EmbeddingsGenerator EmbeddingsGenerator
}

// Discovery describes effective edge projects, lanes, and providers.
type Discovery struct {
	Projects []ProjectDiscovery `json:"projects"`
}

// ProjectDiscovery describes one project.
type ProjectDiscovery struct {
	Name           string              `json:"name"`
	Title          string              `json:"title"`
	Status         string              `json:"status"`
	ActiveProvider string              `json:"activeProvider,omitempty"`
	LocalPath      string              `json:"localPath,omitempty"`
	Lanes          []LaneDiscovery     `json:"lanes"`
	Providers      []ProviderDiscovery `json:"providers"`
	DocumentCount  int                 `json:"documentCount"`
}

// LaneDiscovery describes one documentation lane.
type LaneDiscovery struct {
	Name      string   `json:"name"`
	Schema    string   `json:"schema"`
	Roots     []string `json:"roots"`
	Folders   []string `json:"folders"`
	Documents int      `json:"documents"`
}

// ProviderDiscovery describes one configured provider.
type ProviderDiscovery struct {
	Type              string          `json:"type"`
	State             string          `json:"state"`
	Role              string          `json:"role"`
	WorkspacePath     string          `json:"workspacePath,omitempty"`
	RemoteURL         string          `json:"remoteUrl,omitempty"`
	Capabilities      map[string]bool `json:"capabilities"`
	Available         bool            `json:"available"`
	UnavailableReason string          `json:"unavailableReason,omitempty"`
}

// SyncStatus describes local/remote sync status.
type SyncStatus struct {
	Projects []ProjectSyncStatus `json:"projects"`
}

// ProjectSyncStatus describes sync state for one project.
type ProjectSyncStatus struct {
	Name      string               `json:"name"`
	Documents []DocumentSyncStatus `json:"documents"`
	Remote    *RemoteStatus        `json:"remote,omitempty"`
	Summary   map[string]int       `json:"summary"`
	Warnings  []string             `json:"warnings,omitempty"`
}

// DocumentSyncStatus describes one local document's status.
type DocumentSyncStatus struct {
	Path       string `json:"path"`
	State      string `json:"state"`
	DocUUID    string `json:"docUuid,omitempty"`
	Hash       string `json:"hash,omitempty"`
	Diagnostic string `json:"diagnostic,omitempty"`
}

// RemoteStatus describes remote provider availability.
type RemoteStatus struct {
	Provider string `json:"provider"`
	URL      string `json:"url,omitempty"`
	State    string `json:"state"`
	Error    string `json:"error,omitempty"`
}

// Discover loads project HCL and returns effective edge discovery.
func Discover(opts Options) (Discovery, error) {
	config, docs, err := load(opts)
	if err != nil {
		return Discovery{}, err
	}
	docsByProjectLane := groupDocsByProjectLane(docs)

	discovery := Discovery{Projects: make([]ProjectDiscovery, 0, len(config.Projects))}
	for _, name := range config.ListProjects() {
		if opts.Project != "" && name != opts.Project {
			continue
		}
		project := config.Projects[name]
		projectDocs := documentsForProject(docs, name)
		pd := ProjectDiscovery{
			Name:          project.Name,
			Title:         project.Title,
			Status:        project.Status,
			Lanes:         make([]LaneDiscovery, 0, len(project.Lanes)),
			Providers:     make([]ProviderDiscovery, 0, len(project.Providers)),
			DocumentCount: len(projectDocs),
		}
		if provider, err := project.GetActiveProvider(); err == nil {
			pd.ActiveProvider = provider.Type
			if provider.IsLocal() {
				pd.LocalPath = provider.ResolveWorkspacePath(config.WorkspaceBasePath)
			}
		}
		for _, lane := range project.Lanes {
			pd.Lanes = append(pd.Lanes, LaneDiscovery{
				Name:      lane.Name,
				Schema:    lane.Schema,
				Roots:     lane.Roots,
				Folders:   lane.Folders,
				Documents: len(docsByProjectLane[name+"/"+lane.Name]),
			})
		}
		for _, provider := range project.Providers {
			pd.Providers = append(pd.Providers, providerDiscovery(provider, opts, config.WorkspaceBasePath))
		}
		discovery.Projects = append(discovery.Projects, pd)
	}
	return discovery, nil
}

// Status returns local sync status and optional remote availability.
func Status(ctx context.Context, opts Options) (SyncStatus, error) {
	config, docs, err := load(opts)
	if err != nil {
		return SyncStatus{}, err
	}
	status := SyncStatus{Projects: make([]ProjectSyncStatus, 0, len(config.Projects))}
	validation := docschema.ValidateDocuments(docs)
	diagnosticsByFile := make(map[string][]docschema.Diagnostic)
	for _, diagnostic := range validation.Diagnostics {
		diagnosticsByFile[diagnostic.File] = append(diagnosticsByFile[diagnostic.File], diagnostic)
	}

	for _, name := range config.ListProjects() {
		if opts.Project != "" && name != opts.Project {
			continue
		}
		project := config.Projects[name]
		ps := ProjectSyncStatus{Name: project.Name, Summary: map[string]int{}}
		remoteProvider := firstRemoteProvider(project)
		if remoteProvider != nil {
			remote := probeRemote(ctx, remoteProvider, opts)
			ps.Remote = &remote
			if remote.State == remoteStateUnavailable {
				ps.Warnings = append(ps.Warnings, remote.Error)
			}
		}

		for _, doc := range documentsForProject(docs, name) {
			docStatus := documentStatus(doc, diagnosticsByFile[doc.RelPath])
			if ps.Remote != nil && ps.Remote.State == remoteStateUnavailable && docStatus.State == "clean" {
				docStatus.State = "remote-unavailable"
			}
			ps.Documents = append(ps.Documents, docStatus)
			ps.Summary[docStatus.State]++
		}
		status.Projects = append(status.Projects, ps)
	}
	return status, nil
}

func load(opts Options) (*projectconfig.Config, []docdiscover.Document, error) {
	config, err := projectconfig.LoadConfig(opts.ConfigPath)
	if err != nil {
		return nil, nil, err
	}
	docs, err := docdiscover.Discover(config, docdiscover.Options{RootDir: opts.RootDir, Project: opts.Project})
	if err != nil {
		return nil, nil, err
	}
	return config, docs, nil
}

func providerDiscovery(provider *projectconfig.Provider, opts Options, workspaceBasePath string) ProviderDiscovery {
	pd := ProviderDiscovery{
		Type:         provider.Type,
		State:        provider.GetState(),
		Role:         provider.GetRole(),
		Capabilities: capabilities(provider),
		Available:    true,
	}
	if provider.IsLocal() {
		pd.WorkspacePath = provider.ResolveWorkspacePath(workspaceBasePath)
		if _, err := os.Stat(pd.WorkspacePath); err != nil {
			pd.Available = false
			pd.UnavailableReason = err.Error()
		}
	}
	if provider.IsRemoteHermes() {
		pd.RemoteURL = provider.HermesURL
		pd.Available = false
		pd.UnavailableReason = "remote probing disabled"
		if opts.ProbeRemote {
			remote := probeRemote(context.Background(), provider, opts)
			pd.Available = remote.State == "available"
			pd.UnavailableReason = remote.Error
		}
	}
	return pd
}

func capabilities(provider *projectconfig.Provider) map[string]bool {
	switch provider.Type {
	case projectconfig.ProviderTypeLocal:
		return map[string]bool{"content": true, "permissions": false, "remote_sync": false}
	case projectconfig.ProviderTypeRemoteHermes:
		return map[string]bool{"content": true, "permissions": true, "remote_sync": true}
	case projectconfig.ProviderTypeGoogle:
		return map[string]bool{"content": true, "permissions": true, "remote_sync": false}
	default:
		return map[string]bool{"unknown": true}
	}
}

func probeRemote(ctx context.Context, provider *projectconfig.Provider, opts Options) RemoteStatus {
	remote := RemoteStatus{Provider: provider.Type, URL: provider.HermesURL, State: remoteStateUnavailable}
	if !provider.IsRemoteHermes() {
		remote.Error = "provider is not remote-hermes"
		return remote
	}
	if provider.HermesURL == "" {
		remote.Error = "remote Hermes URL is not configured"
		return remote
	}
	if !opts.ProbeRemote {
		remote.Error = "remote probing disabled"
		return remote
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, strings.TrimRight(provider.HermesURL, "/")+"/health", http.NoBody)
	if err != nil {
		remote.Error = err.Error()
		return remote
	}
	resp, err := client.Do(req)
	if err != nil {
		remote.Error = err.Error()
		return remote
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		remote.Error = fmt.Sprintf("remote returned status %d: %s", resp.StatusCode, string(body))
		return remote
	}
	remote.State = "available"
	remote.Error = ""
	return remote
}

func documentStatus(doc docdiscover.Document, diagnostics []docschema.Diagnostic) DocumentSyncStatus {
	status := DocumentSyncStatus{Path: doc.RelPath, State: "clean"}
	data, err := os.ReadFile(doc.Path) //nolint:gosec // path is from local discovery.
	if err != nil {
		status.State = "local-error"
		status.Diagnostic = err.Error()
		return status
	}
	hash := sha256.Sum256(data)
	status.Hash = hex.EncodeToString(hash[:])
	status.DocUUID = extractDocUUID(data)
	if status.DocUUID == "" {
		status.State = "missing-doc-uuid"
		status.Diagnostic = "missing doc_uuid frontmatter"
	}
	if len(diagnostics) > 0 && status.State == "clean" {
		status.State = "local-dirty"
		status.Diagnostic = diagnostics[0].Message
	}
	return status
}

func extractDocUUID(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == "doc_uuid" {
			return strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		}
	}
	return ""
}

func documentsForProject(docs []docdiscover.Document, project string) []docdiscover.Document {
	filtered := make([]docdiscover.Document, 0)
	for _, doc := range docs {
		if doc.ProjectName == project {
			filtered = append(filtered, doc)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].RelPath < filtered[j].RelPath })
	return filtered
}

func groupDocsByProjectLane(docs []docdiscover.Document) map[string][]docdiscover.Document {
	grouped := make(map[string][]docdiscover.Document)
	for _, doc := range docs {
		grouped[doc.ProjectName+"/"+doc.LaneName] = append(grouped[doc.ProjectName+"/"+doc.LaneName], doc)
	}
	return grouped
}

func firstRemoteProvider(project *projectconfig.Project) *projectconfig.Provider {
	for _, provider := range project.Providers {
		if provider.IsRemoteHermes() {
			return provider
		}
	}
	return nil
}

// MarshalJSONStable returns indented JSON for CLI output.
func MarshalJSONStable(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}
