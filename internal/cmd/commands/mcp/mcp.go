// Package mcp provides the Hermes MCP server CLI command.
package mcp

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
	"github.com/hashicorp-forge/hermes/internal/mcpserver"
)

const defaultEndpoint = "/mcp"

// Command implements the mcp CLI command.
type Command struct {
	*base.Command
}

// Synopsis returns a short description.
func (c *Command) Synopsis() string {
	return "Run the Hermes MCP server"
}

// Help returns the full help text.
func (c *Command) Help() string {
	helpText := `Usage: hermes mcp [options]

  Runs the Hermes Model Context Protocol server for agent workflows. The server
  uses stdio by default and can optionally serve streamable HTTP on /mcp.`

	return helpText + c.Flags().Help()
}

// Flags returns the command flag set.
func (c *Command) Flags() *base.FlagSet {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.String("transport", "stdio", "MCP transport to use: stdio or http")
	fs.String("http-addr", "127.0.0.1:8060", "Address used when -transport=http")
	fs.String("endpoint", defaultEndpoint, "HTTP endpoint path used when -transport=http")
	fs.String("projects-config", "./testing/projects.hcl", "Path to HCL projects configuration")
	fs.String("root", ".", "Repository root used for local discovery")

	return base.NewFlagSet(fs)
}

// Run starts the MCP server.
func (c *Command) Run(args []string) int {
	flags := c.Flags()
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	transport := flags.Lookup("transport").Value.String()
	httpAddr := flags.Lookup("http-addr").Value.String()
	endpoint := flags.Lookup("endpoint").Value.String()
	projectsConfig := flags.Lookup("projects-config").Value.String()
	root := flags.Lookup("root").Value.String()

	srv := mcpserver.New(mcpserver.Options{
		Logger:         c.Log,
		ProjectsConfig: projectsConfig,
		RootDir:        root,
	})

	switch transport {
	case "stdio":
		if err := srv.ServeStdio(c.Context); err != nil {
			c.UI.Error(fmt.Sprintf("mcp stdio server failed: %v", err))
			return 1
		}
		return 0
	case "http":
		httpServer := &http.Server{
			Addr:              httpAddr,
			Handler:           srv.HTTPHandler(endpoint),
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			<-c.Context.Done()
			_ = httpServer.Close()
		}()

		c.UI.Output(fmt.Sprintf("Hermes MCP server listening on http://%s%s", httpAddr, endpoint))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			c.UI.Error(fmt.Sprintf("mcp http server failed: %v", err))
			return 1
		}
		return 0
	default:
		c.UI.Error(fmt.Sprintf("unsupported MCP transport %q", transport))
		return 1
	}
}
