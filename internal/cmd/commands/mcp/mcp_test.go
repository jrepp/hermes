package mcp

import (
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
)

func TestMCPCommandRejectsUnknownTransport(t *testing.T) {
	ui := cli.NewMockUi()
	c := &Command{Command: base.NewCommand(hclog.NewNullLogger(), ui)}

	if code := c.Run([]string{"-transport", "nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(ui.ErrorWriter.String(), "unsupported MCP transport") {
		t.Fatalf("expected unsupported transport error, got %q", ui.ErrorWriter.String())
	}
}

func TestMCPCommandHelpIncludesTransports(t *testing.T) {
	ui := cli.NewMockUi()
	c := &Command{Command: base.NewCommand(hclog.NewNullLogger(), ui)}

	help := c.Help()
	if !strings.Contains(help, "-transport") || !strings.Contains(help, "stdio") || !strings.Contains(help, "http") {
		t.Fatalf("expected transport help, got %q", help)
	}
}
