package cmd

import (
	"bufio"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"

	"github.com/hashicorp-forge/hermes/internal/version"
)

// Main runs the CLI with the given arguments and returns the exit code.
func Main(args []string) int {
	cliName := args[0]

	log := hclog.New(&hclog.LoggerOptions{
		Name: cliName,
	})

	if len(args) == 2 &&
		(args[1] == "-version" ||
			args[1] == "-v") {
		args = []string{cliName, "version"}
	}

	// If no subcommand is provided, default to 'serve'
	if len(args) == 1 {
		args = append(args, "serve")
	}

	ui := &cli.BasicUi{
		Reader:      bufio.NewReader(os.Stdin),
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}

	initCommands(log, ui)

	c := &cli.CLI{
		Name:     cliName,
		Args:     args[1:],
		Version:  version.Version,
		Commands: Commands,
	}

	// Run the CLI
	exitCode, err := c.Run()
	if err != nil {
		panic(err)
	}

	return exitCode
}
