package docdiscover

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
)

func TestDiscoverMonorepoMixedLanes(t *testing.T) {
	root := filepath.Join("testdata", "monorepo")
	config, err := projectconfig.LoadConfig(filepath.Join(root, "projects.hcl"))
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	docs, err := Discover(config, Options{RootDir: root})
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	got := make([]string, 0, len(docs))
	for _, doc := range docs {
		got = append(got, doc.ProjectName+":"+doc.LaneName+":"+doc.RelPath)
	}
	want := []string{
		"alpha:adr:alpha/adr/adr-001-alpha.md",
		"alpha:guides:alpha/guides/plain.md",
		"beta:memo:beta/memo/memo-001-beta.md",
		"beta:memo:shared/memo/memo-002-shared.md",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d docs %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("doc %d = %q, want %q (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestDiscoverFiltersProject(t *testing.T) {
	root := filepath.Join("testdata", "monorepo")
	config, err := projectconfig.LoadConfig(filepath.Join(root, "projects.hcl"))
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	docs, err := Discover(config, Options{RootDir: root, Project: "alpha"})
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("got %d docs, want 2", len(docs))
	}
	for _, doc := range docs {
		if doc.ProjectName != "alpha" {
			t.Fatalf("expected only alpha docs, got %q", doc.ProjectName)
		}
	}
}
