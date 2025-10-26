package serve

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
	"github.com/hashicorp-forge/hermes/internal/cmd/commands/server"
	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/workspace"
)

type Command struct {
	*base.Command

	// Inherit all server command fields
	serverCmd *server.Command

	// Browser launch settings
	FlagBrowser bool
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
	f := c.serverCmd.Flags()

	// Add simplified mode specific flags
	f.BoolVar(
		&c.FlagBrowser, "browser", true,
		"Automatically open browser (simplified mode only)",
	)

	return f
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

	// Display banner with server info
	dbPath := filepath.Join(workspacePath, "hermes.db")
	indexPath := filepath.Join(workspacePath, "search-index")
	serverURL := "http://localhost:8000"
	printBanner(workspacePath, dbPath, indexPath, serverURL)

	// Launch browser in background if enabled
	if c.FlagBrowser {
		go func() {
			// Wait for server to be ready (max 10 seconds)
			if err := waitForServer(serverURL, 10*time.Second); err != nil {
				c.UI.Warn(fmt.Sprintf("Server not ready, skipping browser launch: %v", err))
				return
			}

			// Open browser
			if err := openBrowser(serverURL); err != nil {
				c.UI.Warn(fmt.Sprintf("Could not open browser: %v", err))
			}
		}()
	}

	// Run server with generated config
	return c.serverCmd.Run([]string{"-config", tmpConfigPath})
}
