// Package docdiscover discovers local documentation files from HCL project lanes.
package docdiscover

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
)

// Options configures documentation discovery.
type Options struct {
	RootDir string
	Project string
}

// Document is a discovered local documentation file.
type Document struct {
	ProjectName string                 `json:"projectName"`
	LaneName    string                 `json:"laneName"`
	Schema      string                 `json:"schema"`
	Path        string                 `json:"path"`
	RelPath     string                 `json:"relPath"`
	Lane        *projectconfig.Lane    `json:"-"`
	Project     *projectconfig.Project `json:"-"`
}

// Discover returns documents from all configured project lanes.
func Discover(config *projectconfig.Config, opts Options) ([]Document, error) {
	if config == nil {
		return nil, fmt.Errorf("project config is required")
	}
	rootDir := opts.RootDir
	if rootDir == "" {
		rootDir = "."
	}

	docs := make([]Document, 0)
	for _, projectName := range config.ListProjects() {
		if opts.Project != "" && projectName != opts.Project {
			continue
		}
		project := config.Projects[projectName]
		for _, lane := range project.Lanes {
			discovered, err := discoverLane(rootDir, project, lane)
			if err != nil {
				return nil, err
			}
			docs = append(docs, discovered...)
		}
	}

	sort.Slice(docs, func(i, j int) bool {
		if docs[i].ProjectName != docs[j].ProjectName {
			return docs[i].ProjectName < docs[j].ProjectName
		}
		if docs[i].LaneName != docs[j].LaneName {
			return docs[i].LaneName < docs[j].LaneName
		}
		return docs[i].RelPath < docs[j].RelPath
	})

	return docs, nil
}

func discoverLane(rootDir string, project *projectconfig.Project, lane *projectconfig.Lane) ([]Document, error) {
	allowed := make(map[string]bool, len(lane.AllowedExtensions))
	for _, ext := range lane.AllowedExtensions {
		ext = strings.TrimPrefix(strings.ToLower(ext), ".")
		allowed[ext] = true
	}

	docs := make([]Document, 0)
	for _, root := range lane.Roots {
		folders := lane.Folders
		if len(folders) == 0 {
			folders = []string{"."}
		}
		for _, folder := range folders {
			discovered, err := walkLaneFolder(rootDir, filepath.Join(rootDir, root, folder), project, lane, allowed)
			if err != nil {
				return nil, fmt.Errorf("discover lane %q for project %q: %w", lane.Name, project.Name, err)
			}
			docs = append(docs, discovered...)
		}
	}

	return docs, nil
}

func walkLaneFolder(rootDir, scanRoot string, project *projectconfig.Project, lane *projectconfig.Lane, allowed map[string]bool) ([]Document, error) {
	docs := make([]Document, 0)
	err := filepath.WalkDir(scanRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		doc, skip, err := laneDocument(rootDir, path, entry, project, lane, allowed)
		if err != nil {
			return err
		}
		if skip {
			return nil
		}
		docs = append(docs, doc)
		return nil
	})
	return docs, err
}

func laneDocument(rootDir, path string, entry fs.DirEntry, project *projectconfig.Project, lane *projectconfig.Lane, allowed map[string]bool) (Document, bool, error) {
	rel, err := filepath.Rel(rootDir, path)
	if err != nil {
		return Document{}, true, err
	}
	if shouldSkip(rel, entry, lane.SkipTemplates) {
		if entry.IsDir() {
			return Document{}, true, filepath.SkipDir
		}
		return Document{}, true, nil
	}
	if entry.IsDir() {
		return Document{}, true, nil
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if !allowed[ext] {
		return Document{}, true, nil
	}
	return Document{
		ProjectName: project.Name,
		LaneName:    lane.Name,
		Schema:      lane.Schema,
		Path:        path,
		RelPath:     filepath.ToSlash(rel),
		Lane:        lane,
		Project:     project,
	}, false, nil
}

func shouldSkip(rel string, entry fs.DirEntry, skipTemplates []string) bool {
	base := entry.Name()
	if strings.HasPrefix(base, "_template-") {
		return true
	}
	for _, pattern := range skipTemplates {
		if pattern == "" {
			continue
		}
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		if matched, _ := filepath.Match(filepath.ToSlash(pattern), filepath.ToSlash(rel)); matched {
			return true
		}
	}
	return false
}
