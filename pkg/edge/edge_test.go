package edge

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
)

func TestDiscoverShowsProjectsLanesAndProviders(t *testing.T) {
	discovery, err := Discover(testOptions(false))
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	if len(discovery.Projects) != 1 {
		t.Fatalf("expected 1 project, got %#v", discovery.Projects)
	}
	project := discovery.Projects[0]
	if project.Name != "edge" || project.DocumentCount != 3 {
		t.Fatalf("unexpected project discovery: %#v", project)
	}
	if len(project.Lanes) != 1 || project.Lanes[0].Documents != 3 {
		t.Fatalf("unexpected lanes: %#v", project.Lanes)
	}
	if len(project.Providers) != 2 {
		t.Fatalf("expected local and remote providers, got %#v", project.Providers)
	}
}

func TestStatusDistinguishesLocalStatesAndOfflineRemote(t *testing.T) {
	status, err := Status(context.Background(), testOptions(true))
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if len(status.Projects) != 1 {
		t.Fatalf("expected 1 project, got %#v", status.Projects)
	}
	project := status.Projects[0]
	if project.Summary["remote-unavailable"] != 1 {
		t.Fatalf("expected clean doc to become remote-unavailable, got summary %#v", project.Summary)
	}
	if project.Summary["missing-doc-uuid"] != 1 {
		t.Fatalf("expected missing-doc-uuid summary, got %#v", project.Summary)
	}
	if project.Summary["local-dirty"] != 1 {
		t.Fatalf("expected local-dirty summary, got %#v", project.Summary)
	}
	if project.Remote == nil || project.Remote.State != "unavailable" {
		t.Fatalf("expected unavailable remote, got %#v", project.Remote)
	}
}

func TestUnknownProviderCapability(t *testing.T) {
	provider := providerDiscovery(&mockUnknownProvider, Options{}, ".")
	if !provider.Capabilities["unknown"] {
		t.Fatalf("expected unknown capability marker, got %#v", provider.Capabilities)
	}
}

var mockUnknownProvider = projectconfig.Provider{Type: "mystery"}

func testOptions(probe bool) Options {
	root := "testdata"
	return Options{
		ConfigPath:  filepath.Join(root, "projects.hcl"),
		RootDir:     root,
		Project:     "edge",
		ProbeRemote: probe,
		Timeout:     10 * time.Millisecond,
	}
}
