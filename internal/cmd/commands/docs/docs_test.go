package docs

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
)

func TestDocsCommandReturnsHelp(t *testing.T) {
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

func TestDocsValidateCommand(t *testing.T) {
	ui := cli.NewMockUi()
	c := &Command{Command: base.NewCommand(hclog.NewNullLogger(), ui)}
	root := filepath.Join("..", "..", "..", "..", "pkg", "docdiscover", "testdata", "monorepo")

	code := c.Run([]string{"validate", "-config", filepath.Join(root, "projects.hcl"), "-root", root, "-project", "alpha"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; errors: %s; output: %s", code, ui.ErrorWriter.String(), ui.OutputWriter.String())
	}
}

func TestDocsMigrateDryRunCommand(t *testing.T) {
	ui := cli.NewMockUi()
	c := &Command{Command: base.NewCommand(hclog.NewNullLogger(), ui)}
	root := filepath.Join("..", "..", "..", "..", "pkg", "docrepair", "testdata", "repair")

	code := c.Run([]string{"migrate", "-config", filepath.Join(root, "projects.hcl"), "-root", root})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; errors: %s; output: %s", code, ui.ErrorWriter.String(), ui.OutputWriter.String())
	}
	if !strings.Contains(ui.OutputWriter.String(), "dry-run") {
		t.Fatalf("expected dry-run output, got %q", ui.OutputWriter.String())
	}
}
