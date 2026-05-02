// Package edge provides CLI commands for edge-capable Hermes workflows.
package edge

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mitchellh/cli"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
	edgepkg "github.com/hashicorp-forge/hermes/pkg/edge"
)

const (
	searchModeHybrid = "hybrid"
	searchModeVector = "vector"
)

// Command implements the edge CLI command group.
type Command struct {
	*base.Command
}

// Synopsis returns a short description.
func (c *Command) Synopsis() string {
	return "Run local and remote edge document workflows"
}

// Help returns the full help text.
func (c *Command) Help() string {
	helpText := `Usage: hermes edge <subcommand> [options] [args]

  This command groups local and remote discovery, indexing, search, and sync
  workflows for edge-capable Hermes clients.

  Available subcommands:

    discover       Show effective projects, lanes, providers, and local paths.
    sync status    Show local document state and remote availability.`

	return helpText + c.Flags().Help()
}

// Flags returns the command flag set.
func (c *Command) Flags() *base.FlagSet {
	fs := flag.NewFlagSet("edge", flag.ContinueOnError)
	fs.String("config", "./testing/projects.hcl", "Path to HCL projects configuration")
	fs.String("root", ".", "Repository root used for local discovery")
	fs.String("project", "", "Optional project name")
	fs.String("format", "text", "Output format: text or json")
	fs.String("query", "", "Search query for edge search")
	fs.String("mode", "bm25", "Search mode: bm25, vector, or hybrid")
	fs.Int("limit", 10, "Maximum search results")
	fs.Bool("debug", false, "Include search score debug output")
	fs.Bool("probe-remote", false, "Probe remote Hermes providers with a bounded health check")
	fs.Duration("timeout", 2*time.Second, "Remote probe timeout")
	fs.String("qdrant-url", "", "Qdrant URL for vector search")
	fs.String("qdrant-api-key", "", "Qdrant API key for vector search")
	fs.String("qdrant-collection", "", "Qdrant collection for vector search")
	fs.String("embedding-model", "", "Embedding model for vector search")
	fs.String("ollama-url", "", "Ollama URL for local embeddings")
	fs.Int("embedding-dimensions", 0, "Embedding dimensions for vector search")

	return base.NewFlagSet(fs)
}

// Run executes an edge subcommand.
func (c *Command) Run(args []string) int {
	if len(args) == 0 {
		return cli.RunResultHelp
	}
	switch args[0] {
	case "discover":
		return c.runDiscover(args[1:])
	case "index":
		return c.runIndex(args[1:])
	case "search":
		return c.runSearch(args[1:])
	case "sync":
		return c.runSync(args[1:])
	default:
		c.UI.Error(fmt.Sprintf("unknown edge subcommand %q", args[0]))
		return 1
	}
}

func (c *Command) runIndex(args []string) int {
	flags := c.Flags()
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	_, result, err := edgepkg.BuildIndex(edgeOptions(flags))
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	return c.render(flags.Lookup("format").Value.String(), result)
}

func (c *Command) runSearch(args []string) int {
	flags := c.Flags()
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	query := flags.Lookup("query").Value.String()
	if query == "" && flags.NArg() > 0 {
		query = flags.Arg(0)
	}
	if query == "" {
		c.UI.Error("search query is required")
		return 1
	}
	mode := flags.Lookup("mode").Value.String()
	if mode != "bm25" && mode != searchModeHybrid && mode != searchModeVector {
		c.UI.Error(fmt.Sprintf("unsupported search mode %q", mode))
		return 1
	}
	limit := intFlag(flags, "limit")
	debug := flags.Lookup("debug").Value.String() == "true"
	if mode == searchModeVector {
		result, err := edgepkg.SearchVector(context.Background(), edgeOptions(flags), query, limit)
		if err != nil {
			c.UI.Error(err.Error())
			return 1
		}
		return c.render(flags.Lookup("format").Value.String(), result)
	}
	if mode == searchModeHybrid {
		result, err := edgepkg.SearchHybrid(context.Background(), edgeOptions(flags), query, limit, debug)
		if err != nil {
			c.UI.Error(err.Error())
			return 1
		}
		return c.render(flags.Lookup("format").Value.String(), result)
	}
	result, err := edgepkg.SearchLocal(edgeOptions(flags), query, limit, debug || mode == searchModeHybrid)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	return c.render(flags.Lookup("format").Value.String(), result)
}

func (c *Command) runSync(args []string) int {
	if len(args) == 0 {
		c.UI.Error("missing edge sync subcommand")
		return 1
	}
	if args[0] != "status" {
		c.UI.Error(fmt.Sprintf("unknown edge sync subcommand %q", args[0]))
		return 1
	}
	return c.runStatus(args[1:])
}

func (c *Command) runDiscover(args []string) int {
	flags := c.Flags()
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	discovery, err := edgepkg.Discover(edgeOptions(flags))
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	return c.render(flags.Lookup("format").Value.String(), discovery)
}

func (c *Command) runStatus(args []string) int {
	flags := c.Flags()
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	status, err := edgepkg.Status(context.Background(), edgeOptions(flags))
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	return c.render(flags.Lookup("format").Value.String(), status)
}

func (c *Command) render(format string, value any) int {
	if format == "json" {
		return c.renderJSON(value)
	}
	if format != "text" {
		c.UI.Error(fmt.Sprintf("unsupported output format %q", format))
		return 1
	}

	switch typed := value.(type) {
	case edgepkg.Discovery:
		c.renderDiscovery(typed)
	case edgepkg.SyncStatus:
		c.renderSyncStatus(typed)
	case edgepkg.IndexResult:
		c.renderIndexResult(typed)
	case edgepkg.SearchResult:
		c.renderSearchResult(typed)
	default:
		c.UI.Error("unsupported render value")
		return 1
	}
	return 0
}

func (c *Command) renderJSON(value any) int {
	data, err := edgepkg.MarshalJSONStable(value)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	_, _ = os.Stdout.Write(data)
	_, _ = os.Stdout.WriteString("\n")
	return 0
}

func (c *Command) renderDiscovery(discovery edgepkg.Discovery) {
	for i := range discovery.Projects {
		project := discovery.Projects[i]
		c.UI.Output(fmt.Sprintf("project %s (%s): %d docs, active provider %s", project.Name, project.Status, project.DocumentCount, project.ActiveProvider))
		for _, lane := range project.Lanes {
			c.UI.Output(fmt.Sprintf("  lane %s [%s]: %d docs", lane.Name, lane.Schema, lane.Documents))
		}
		for _, provider := range project.Providers {
			state := "available"
			if !provider.Available {
				state = "unavailable: " + provider.UnavailableReason
			}
			c.UI.Output(fmt.Sprintf("  provider %s (%s): %s", provider.Type, provider.State, state))
		}
	}
}

func (c *Command) renderSyncStatus(status edgepkg.SyncStatus) {
	for _, project := range status.Projects {
		c.UI.Output(fmt.Sprintf("project %s", project.Name))
		for state, count := range project.Summary {
			c.UI.Output(fmt.Sprintf("  %s: %d", state, count))
		}
		if project.Remote != nil {
			c.UI.Output(fmt.Sprintf("  remote %s: %s", project.Remote.URL, project.Remote.State))
		}
	}
}

func (c *Command) renderIndexResult(result edgepkg.IndexResult) {
	c.UI.Output(fmt.Sprintf("indexed %d documents", len(result.Documents)))
	for _, skipped := range result.Skipped {
		c.UI.Output(fmt.Sprintf("  skipped %s: %s", skipped.Path, skipped.Reason))
	}
}

func (c *Command) renderSearchResult(result edgepkg.SearchResult) {
	c.UI.Output(fmt.Sprintf("%s search %q: %d hits", result.Mode, result.Query, len(result.Hits)))
	for _, hit := range result.Hits {
		c.UI.Output(fmt.Sprintf("  %.4f %s", hit.Score, hit.Path))
	}
}

func intFlag(flags *base.FlagSet, name string) int {
	getter, ok := flags.Lookup(name).Value.(flag.Getter)
	if !ok {
		return 0
	}
	value, ok := getter.Get().(int)
	if !ok {
		return 0
	}
	return value
}

func edgeOptions(flags *base.FlagSet) edgepkg.Options {
	return edgepkg.Options{
		ConfigPath:          flags.Lookup("config").Value.String(),
		RootDir:             flags.Lookup("root").Value.String(),
		Project:             flags.Lookup("project").Value.String(),
		QdrantURL:           flags.Lookup("qdrant-url").Value.String(),
		QdrantAPIKey:        flags.Lookup("qdrant-api-key").Value.String(),
		QdrantCollection:    flags.Lookup("qdrant-collection").Value.String(),
		EmbeddingModel:      flags.Lookup("embedding-model").Value.String(),
		OllamaURL:           flags.Lookup("ollama-url").Value.String(),
		ProbeRemote:         flags.Lookup("probe-remote").Value.String() == "true",
		Timeout:             parseDuration(flags.Lookup("timeout").Value.String()),
		EmbeddingDimensions: intFlag(flags, "embedding-dimensions"),
		OpenAIAPIKey:        os.Getenv("OPENAI_API_KEY"),
	}
}

func parseDuration(value string) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 2 * time.Second
	}
	return duration
}
