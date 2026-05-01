package edge

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
)

func TestEdgeCommandReturnsHelp(t *testing.T) {
	ui := cli.NewMockUi()
	c := &Command{Command: base.NewCommand(hclog.NewNullLogger(), ui)}

	if code := c.Run(nil); code != cli.RunResultHelp {
		t.Fatalf("expected help result, got %d", code)
	}
	if c.Synopsis() == "" {
		t.Fatal("expected synopsis")
	}
	if c.Help() == "" {
		t.Fatal("expected help")
	}
}

func TestEdgeDiscoverCommand(t *testing.T) {
	ui := cli.NewMockUi()
	c := &Command{Command: base.NewCommand(hclog.NewNullLogger(), ui)}
	root := filepath.Join("..", "..", "..", "..", "pkg", "edge", "testdata")

	code := c.Run([]string{"discover", "-config", filepath.Join(root, "projects.hcl"), "-root", root, "-project", "edge"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; errors: %s", code, ui.ErrorWriter.String())
	}
	if !strings.Contains(ui.OutputWriter.String(), "project edge") {
		t.Fatalf("expected edge project output, got %q", ui.OutputWriter.String())
	}
}

func TestEdgeSyncStatusCommand(t *testing.T) {
	ui := cli.NewMockUi()
	c := &Command{Command: base.NewCommand(hclog.NewNullLogger(), ui)}
	root := filepath.Join("..", "..", "..", "..", "pkg", "edge", "testdata")

	code := c.Run([]string{"sync", "status", "-config", filepath.Join(root, "projects.hcl"), "-root", root, "-project", "edge", "-probe-remote", "-timeout", "10ms"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; errors: %s", code, ui.ErrorWriter.String())
	}
	if !strings.Contains(ui.OutputWriter.String(), "remote-unavailable") {
		t.Fatalf("expected sync state output, got %q", ui.OutputWriter.String())
	}
}

func TestEdgeIndexCommand(t *testing.T) {
	ui := cli.NewMockUi()
	c := &Command{Command: base.NewCommand(hclog.NewNullLogger(), ui)}
	root := filepath.Join("..", "..", "..", "..", "pkg", "edge", "testdata")

	code := c.Run([]string{"index", "-config", filepath.Join(root, "projects.hcl"), "-root", root, "-project", "edge"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; errors: %s", code, ui.ErrorWriter.String())
	}
	if !strings.Contains(ui.OutputWriter.String(), "indexed 3 documents") {
		t.Fatalf("expected index output, got %q", ui.OutputWriter.String())
	}
}

func TestEdgeSearchCommand(t *testing.T) {
	ui := cli.NewMockUi()
	c := &Command{Command: base.NewCommand(hclog.NewNullLogger(), ui)}
	root := filepath.Join("..", "..", "..", "..", "pkg", "edge", "testdata")

	code := c.Run([]string{"search", "-config", filepath.Join(root, "projects.hcl"), "-root", root, "-project", "edge", "clean"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; errors: %s", code, ui.ErrorWriter.String())
	}
	if !strings.Contains(ui.OutputWriter.String(), "docs/clean.md") {
		t.Fatalf("expected search output, got %q", ui.OutputWriter.String())
	}
}
