package cmd

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
)

func TestCommandRegistryIncludesEdgeCommands(t *testing.T) {
	initCommands(hclog.NewNullLogger(), cli.NewMockUi())

	for _, name := range []string{"docs", "edge", "mcp"} {
		factory, ok := Commands[name]
		if !ok {
			t.Fatalf("expected command %q to be registered", name)
		}
		command, err := factory()
		if err != nil {
			t.Fatalf("factory for %q returned error: %v", name, err)
		}
		if command.Help() == "" {
			t.Fatalf("expected help for %q", name)
		}
	}
}
