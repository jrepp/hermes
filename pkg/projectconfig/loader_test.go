package projectconfig

import (
	"path/filepath"
	"testing"
)

func TestLoadConfigImportsProjectsAndLanes(t *testing.T) {
	config, err := LoadConfig(filepath.Join("..", "docdiscover", "testdata", "monorepo", "projects.hcl"))
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	names := config.ListProjects()
	wantNames := []string{"alpha", "beta"}
	if len(names) != len(wantNames) {
		t.Fatalf("project names = %v, want %v", names, wantNames)
	}
	for i := range wantNames {
		if names[i] != wantNames[i] {
			t.Fatalf("project names = %v, want %v", names, wantNames)
		}
	}

	alpha := config.Projects["alpha"]
	if alpha == nil || len(alpha.Lanes) != 2 {
		t.Fatalf("expected alpha with 2 lanes, got %#v", alpha)
	}
	if !alpha.Lanes[0].RequireFrontmatter || !alpha.Lanes[0].EnforceFilenamePattern {
		t.Fatalf("expected lane booleans decoded: %#v", alpha.Lanes[0])
	}
}

func TestLoadConfigInvalidHCL(t *testing.T) {
	_, err := LoadConfig(filepath.Join("..", "docdiscover", "testdata", "invalid", "projects.hcl"))
	if err == nil {
		t.Fatal("expected missing import error")
	}
}
