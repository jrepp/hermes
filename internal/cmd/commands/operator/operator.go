// Package operator provides CLI commands for operational tasks.
package operator

import (
	"github.com/mitchellh/cli"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
)

// Command implements the operator CLI command.
type Command struct {
	*base.Command
}

// Synopsis returns a short description.
func (c *Command) Synopsis() string {
	return "Perform operator-specific tasks"
}

// Help returns the full help text.
func (c *Command) Help() string {
	return `Usage: hermes operator <subcommand> [options] [args]

  This command groups subcommands for operators interacting with Hermes.`
}

// Run displays help for the operator command group.
func (c *Command) Run(_ []string) int {
	return cli.RunResultHelp
}
