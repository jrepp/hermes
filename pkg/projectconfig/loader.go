// Package projectconfig provides projectconfig functionality.
package projectconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type projectsBlock struct {
	Defaults          *Defaults `hcl:"defaults,block"`
	Version           string    `hcl:"version"`
	ConfigDir         string    `hcl:"config_dir,optional"`
	WorkspaceBasePath string    `hcl:"workspace_base_path"`
}

type projectFile struct {
	Projects []*projectHCL `hcl:"project,block"`
}

type projectHCL struct {
	Metadata     *metadataHCL `hcl:"metadata,block"`
	Name         string       `hcl:"name,label"`
	Title        string       `hcl:"title"`
	FriendlyName string       `hcl:"friendly_name"`
	ShortName    string       `hcl:"short_name"`
	Description  string       `hcl:"description,optional"`
	Status       string       `hcl:"status"`
	Providers    []*Provider  `hcl:"provider,block"`
	Lanes        []*laneHCL   `hcl:"lane,block"`
}

type laneHCL struct {
	Name                   string   `hcl:"name,label"`
	Schema                 string   `hcl:"schema,optional"`
	FilenamePattern        string   `hcl:"filename_pattern,optional"`
	Roots                  []string `hcl:"roots,optional"`
	Folders                []string `hcl:"folders,optional"`
	AllowedExtensions      []string `hcl:"allowed_extensions,optional"`
	SkipTemplates          []string `hcl:"skip_templates,optional"`
	EnforceFilenamePattern bool     `hcl:"enforce_filename_pattern,optional"`
	RequireFrontmatter     bool     `hcl:"require_frontmatter,optional"`
}

type metadataHCL struct {
	CreatedAt string   `hcl:"created_at,optional"`
	Owner     string   `hcl:"owner,optional"`
	Notes     string   `hcl:"notes,optional"`
	Tags      []string `hcl:"tags,optional"`
}

// LoadConfig loads and parses a projects configuration.
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config file path is required")
	}

	root, err := parseHCLFile(configPath)
	if err != nil {
		return nil, err
	}
	imports, err := parseImportLines(configPath)
	if err != nil {
		return nil, err
	}
	body, ok := root.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("unexpected HCL body type in %s", configPath)
	}

	config := &Config{
		Projects: make(map[string]*Project),
	}

	for _, block := range body.Blocks {
		if block.Type != "projects" {
			continue
		}
		var decoded projectsBlock
		if diags := gohcl.DecodeBody(block.Body, nil, &decoded); diags.HasErrors() {
			return nil, fmt.Errorf("failed to decode projects block in %s: %w", configPath, diags)
		}
		config.Version = decoded.Version
		config.ConfigDir = decoded.ConfigDir
		config.WorkspaceBasePath = decoded.WorkspaceBasePath
		config.Defaults = decoded.Defaults
	}

	if config.Defaults == nil {
		config.Defaults = &Defaults{}
	}
	if config.ConfigDir == "" {
		config.ConfigDir = "./projects"
	}

	configDir := filepath.Dir(configPath)
	for _, importPath := range imports {
		projectPath := importPath
		if !filepath.IsAbs(projectPath) {
			projectPath = filepath.Join(configDir, projectPath)
		}
		if strings.HasPrefix(filepath.Base(projectPath), "_template-") {
			continue
		}

		projects, err := loadProjectFile(projectPath)
		if err != nil {
			return nil, err
		}
		for _, project := range projects {
			config.Projects[project.Name] = project
		}
	}

	return config, nil
}

func loadProjectFile(filename string) ([]*Project, error) {
	file, err := parseHCLFile(filename)
	if err != nil {
		return nil, err
	}

	var decoded projectFile
	if diags := gohcl.DecodeBody(file.Body, nil, &decoded); diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode project file %s: %w", filename, diags)
	}

	projects := make([]*Project, 0, len(decoded.Projects))
	for _, raw := range decoded.Projects {
		project, err := raw.toProject(filename)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, nil
}

func (p *projectHCL) toProject(filename string) (*Project, error) {
	lanes := make([]*Lane, 0, len(p.Lanes))
	for _, rawLane := range p.Lanes {
		lanes = append(lanes, &Lane{
			Name:                   rawLane.Name,
			Schema:                 rawLane.Schema,
			FilenamePattern:        rawLane.FilenamePattern,
			Roots:                  rawLane.Roots,
			Folders:                rawLane.Folders,
			AllowedExtensions:      rawLane.AllowedExtensions,
			SkipTemplates:          rawLane.SkipTemplates,
			EnforceFilenamePattern: rawLane.EnforceFilenamePattern,
			RequireFrontmatter:     rawLane.RequireFrontmatter,
		})
	}

	project := &Project{
		Name:         p.Name,
		Title:        p.Title,
		FriendlyName: p.FriendlyName,
		ShortName:    p.ShortName,
		Description:  p.Description,
		Status:       p.Status,
		Providers:    p.Providers,
		Lanes:        lanes,
	}

	if p.Metadata != nil {
		metadata := &Metadata{
			Owner: p.Metadata.Owner,
			Notes: p.Metadata.Notes,
			Tags:  p.Metadata.Tags,
		}
		if p.Metadata.CreatedAt != "" {
			createdAt, err := parseTime(p.Metadata.CreatedAt)
			if err != nil {
				return nil, fmt.Errorf("failed to parse metadata.created_at for project %q in %s: %w", p.Name, filename, err)
			}
			metadata.CreatedAt = createdAt
		}
		project.Metadata = metadata
	}

	markLaneAttributePresence(project, filename)
	for _, lane := range project.Lanes {
		applyLaneDefaults(lane)
	}

	return project, nil
}

func markLaneAttributePresence(project *Project, filename string) {
	file, err := parseHCLFile(filename)
	if err != nil {
		return
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return
	}

	lanesByName := make(map[string]*Lane, len(project.Lanes))
	for _, lane := range project.Lanes {
		lanesByName[lane.Name] = lane
	}

	for _, block := range body.Blocks {
		if block.Type != "project" || len(block.Labels) != 1 || block.Labels[0] != project.Name {
			continue
		}
		projectBody := block.Body
		for _, laneBlock := range projectBody.Blocks {
			if laneBlock.Type != "lane" || len(laneBlock.Labels) != 1 {
				continue
			}
			lane := lanesByName[laneBlock.Labels[0]]
			if lane == nil {
				continue
			}
			laneBody := laneBlock.Body
			_, lane.RequireFrontmatterSet = laneBody.Attributes["require_frontmatter"]
			_, lane.EnforceFilenamePatternSet = laneBody.Attributes["enforce_filename_pattern"]
		}
	}
}

func applyLaneDefaults(lane *Lane) {
	if lane.Schema == "" {
		lane.Schema = "generic"
	}
	if len(lane.AllowedExtensions) == 0 {
		lane.AllowedExtensions = []string{"md"}
	}
	if len(lane.Roots) == 0 {
		lane.Roots = []string{"."}
	}
	if !lane.RequireFrontmatterSet {
		lane.RequireFrontmatter = lane.Schema != "generic"
	}
}

func parseHCLFile(filename string) (*hcl.File, error) {
	//nolint:gosec // G304: filename is provided by config/CLI callers.
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read HCL file %s: %w", filename, err)
	}

	file, diags := hclsyntax.ParseConfig(stripImportLines(src), filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse HCL file %s: %w", filename, diags)
	}
	return file, nil
}

func parseImportLines(filename string) ([]string, error) {
	//nolint:gosec // G304: filename is provided by config/CLI callers.
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read HCL file %s: %w", filename, err)
	}

	imports := make([]string, 0)
	for _, line := range strings.Split(string(src), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(trimmed, "import "))
		path = strings.Trim(path, `"`)
		if path == "" {
			return nil, fmt.Errorf("empty import path in %s", filename)
		}
		imports = append(imports, path)
	}
	return imports, nil
}

func stripImportLines(src []byte) []byte {
	lines := strings.Split(string(src), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "import ") {
			lines[i] = ""
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// Helper functions

func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}

	for _, format := range formats {
		t, err := time.Parse(format, s)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time %q", s)
}

func mustParseTime(s string) time.Time {
	t, err := parseTime(s)
	if err != nil {
		panic(err)
	}
	return t
}

// LoadConfigFromEnv loads config from environment variable or default path.
func LoadConfigFromEnv() (*Config, error) {
	configPath := os.Getenv("HERMES_PROJECTS_CONFIG")
	if configPath == "" {
		configPath = "./testing/projects.hcl"
	}

	//nolint:gosec // G703: configPath is from env/config, not direct user input.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	return LoadConfig(configPath)
}

// ResolveEnvVars resolves environment variable references in the config.
// This handles the env("VAR_NAME") syntax.
func ResolveEnvVars(value string) string {
	if strings.HasPrefix(value, "env(") && strings.HasSuffix(value, ")") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(value, "env(\""), "\")")
		return os.Getenv(envVar)
	}
	return value
}
