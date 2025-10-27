package serve

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
	"github.com/hashicorp-forge/hermes/internal/cmd/commands/server"
	"github.com/hashicorp-forge/hermes/internal/config"
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
	configPath := ""
	if configFlag := f.FlagSet.Lookup("config"); configFlag != nil {
		configPath = configFlag.Value.String()
	}

	// If explicit config provided, use traditional server mode
	if configPath != "" {
		c.UI.Info("Running in traditional server mode (config file specified)")
		return c.serverCmd.Run(args)
	}

	// Check for config.hcl in current directory
	cwd, err := os.Getwd()
	if err != nil {
		c.UI.Error(fmt.Sprintf("error getting current directory: %v", err))
		return 1
	}

	configPath = filepath.Join(cwd, "config.hcl")

	// If config.hcl exists, use it
	if _, err := os.Stat(configPath); err == nil {
		c.UI.Info("Found config.hcl, starting server...")
		return c.serverCmd.Run([]string{"-config", configPath})
	}

	// No config found - enter setup mode
	c.UI.Info("No configuration found. Starting setup wizard...")
	c.UI.Info("Open your browser to http://localhost:8000/setup to configure Hermes")
	c.UI.Info("")

	// Create minimal temporary config just to start the web server for setup
	tmpConfigPath := filepath.Join(cwd, ".hermes-setup-temp.hcl")
	if err := writeSetupConfig(tmpConfigPath, cwd); err != nil {
		c.UI.Error(fmt.Sprintf("error writing setup config: %v", err))
		return 1
	}
	defer os.Remove(tmpConfigPath)

	// Launch browser to setup page if enabled
	if c.FlagBrowser {
		setupURL := "http://localhost:8000/setup"
		go func() {
			// Wait for server to be ready (max 10 seconds)
			if err := waitForServer("http://localhost:8000", 10*time.Second); err != nil {
				c.UI.Warn(fmt.Sprintf("Server not ready, skipping browser launch: %v", err))
				return
			}

			// Open browser to setup page
			if err := openBrowser(setupURL); err != nil {
				c.UI.Warn(fmt.Sprintf("Could not open browser: %v", err))
			}
		}()
	}

	// Run server with setup config
	return c.serverCmd.Run([]string{"-config", tmpConfigPath})
}

// writeSetupConfig creates a minimal config for setup mode
func writeSetupConfig(configPath, workingDir string) error {
	// Create a minimal config using the config package
	// Use a temporary workspace path that won't conflict
	tmpWorkspace := filepath.Join(workingDir, ".hermes-setup-tmp")

	cfg := config.GenerateSimplifiedConfig(tmpWorkspace)

	return config.WriteConfig(cfg, configPath)
}
