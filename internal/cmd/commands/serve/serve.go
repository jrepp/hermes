package serve

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
	"github.com/hashicorp-forge/hermes/internal/cmd/commands/server"
	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/workspace"
)

type Command struct {
	*base.Command

	// Inherit all server command fields
	serverCmd *server.Command
}

func (c *Command) Synopsis() string {
	return "Run the server (zero-config simplified mode or traditional server)"
}

func (c *Command) Help() string {
	return `Usage: hermes serve [path]
       hermes serve -config=config.hcl

  Run Hermes in simplified mode (zero-config) or traditional server mode.

  Simplified Mode (Zero-Config):
    ./hermes                  - Uses ./docs-cms/ in current directory
    ./hermes /path/to/docs    - Uses specified path for docs-cms

  Traditional Mode:
    ./hermes serve -config=config.hcl  - Uses explicit config file

  In simplified mode, Hermes will:
    - Auto-create ./docs-cms/ directory structure if not exists
    - Use embedded SQLite database (no PostgreSQL required)
    - Use local filesystem for document storage
    - Start web server on http://localhost:8000
    - Auto-open browser (use --browser=false to disable)

` + c.Flags().Help()
}

func (c *Command) Flags() *base.FlagSet {
	// Use server command's flags
	if c.serverCmd == nil {
		c.serverCmd = &server.Command{Command: c.Command}
	}
	return c.serverCmd.Flags()
}

func (c *Command) Run(args []string) int {
	// Initialize server command
	c.serverCmd = &server.Command{Command: c.Command}

	f := c.Flags()
	if err := f.Parse(args); err != nil {
		c.UI.Error(fmt.Sprintf("error parsing flags: %v", err))
		return 1
	}

	// Check if explicit config file provided
	// FlagSet embeds *flag.FlagSet, so we can access its methods directly
	configPath := ""
	if configFlag := f.FlagSet.Lookup("config"); configFlag != nil {
		configPath = configFlag.Value.String()
	}

	// If explicit config provided, use traditional server mode
	if configPath != "" {
		c.UI.Info("Running in traditional server mode (config file specified)")
		return c.serverCmd.Run(args)
	} // Simplified mode: determine workspace path
	var workspacePath string
	remainingArgs := f.Args()

	if len(remainingArgs) > 0 {
		// Explicit path provided
		workspacePath = remainingArgs[0]
	} else {
		// Check for ./docs-cms/ in current directory
		cwd, err := os.Getwd()
		if err != nil {
			c.UI.Error(fmt.Sprintf("error getting current directory: %v", err))
			return 1
		}
		workspacePath = filepath.Join(cwd, "docs-cms")
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		c.UI.Error(fmt.Sprintf("error resolving workspace path: %v", err))
		return 1
	}
	workspacePath = absPath

	// Check if workspace exists, initialize if not
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		c.UI.Info(fmt.Sprintf("Initializing new Hermes workspace at %s", workspacePath))
		if err := workspace.InitializeWorkspace(workspacePath); err != nil {
			c.UI.Error(fmt.Sprintf("error initializing workspace: %v", err))
			return 1
		}
		c.UI.Info("✓ Workspace initialized successfully")
	} else {
		c.UI.Info(fmt.Sprintf("Using existing workspace at %s", workspacePath))
	}

	// Generate simplified config
	cfg := config.GenerateSimplifiedConfig(workspacePath)

	// Write temporary config file (so server command can load it)
	tmpConfigPath := filepath.Join(workspacePath, ".hermes-config-temp.hcl")
	if err := config.WriteConfig(cfg, tmpConfigPath); err != nil {
		c.UI.Error(fmt.Sprintf("error writing config: %v", err))
		return 1
	}
	defer os.Remove(tmpConfigPath)

	c.UI.Info(fmt.Sprintf("Starting Hermes in simplified mode..."))
	c.UI.Info(fmt.Sprintf("  Workspace: %s", workspacePath))
	c.UI.Info(fmt.Sprintf("  Database: SQLite (embedded)"))
	c.UI.Info(fmt.Sprintf("  Web UI: http://localhost:8000"))
	c.UI.Info("")

	// Run server with generated config
	return c.serverCmd.Run([]string{"-config", tmpConfigPath})
}
