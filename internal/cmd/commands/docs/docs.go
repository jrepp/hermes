// Package docs provides CLI commands for documentation workflows.
package docs

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/mitchellh/cli"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
	"github.com/hashicorp-forge/hermes/pkg/docdiscover"
	"github.com/hashicorp-forge/hermes/pkg/docrepair"
	"github.com/hashicorp-forge/hermes/pkg/docschema"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
)

// Command implements the docs CLI command group.
type Command struct {
	*base.Command
}

// Synopsis returns a short description.
func (c *Command) Synopsis() string {
	return "Inspect and maintain documentation projects"
}

// Help returns the full help text.
func (c *Command) Help() string {
	helpText := `Usage: hermes docs <subcommand> [options] [args]

  This command groups documentation validation, repair, migration, bootstrap,
  and bulk update workflows. Subcommands will be added as the edge documentation
  client is implemented.

  Available subcommands:

    validate            Discover configured documentation lanes and report diagnostics.
    migrate             Plan or apply frontmatter migration.
    bulk update         Plan or apply a bulk frontmatter field update.
    bulk timestamps     Plan or apply missing created timestamp backfills.
    bulk compress-ids   Plan or apply lane-local id renumbering.
    repair links        Check relative Markdown links.`

	return helpText + c.Flags().Help()
}

// Flags returns the command flag set.
func (c *Command) Flags() *base.FlagSet {
	fs := flag.NewFlagSet("docs", flag.ContinueOnError)
	fs.String("config", "./testing/projects.hcl", "Path to HCL projects configuration")
	fs.String("root", ".", "Repository root used for local discovery")
	fs.String("project", "", "Optional project name to validate")
	fs.String("format", "text", "Output format: text or json")
	fs.Bool("apply", false, "Apply planned documentation changes")
	fs.String("field", "", "Frontmatter field for bulk update")
	fs.String("value", "", "Frontmatter value for bulk update")
	fs.Bool("remove", false, "Remove the selected bulk update field")
	fs.Int("start-id", 1, "Starting numeric ID for bulk compress-ids")

	return base.NewFlagSet(fs)
}

// Run displays help for the docs command group.
func (c *Command) Run(args []string) int {
	if len(args) == 0 {
		return cli.RunResultHelp
	}

	subcommand := args[0]
	switch subcommand {
	case "validate":
		return c.runValidate(args[1:])
	case "migrate":
		return c.runRepair(args[1:], docrepair.OperationMigrate)
	case "bulk":
		return c.runBulk(args[1:])
	case "repair":
		return c.runRepairGroup(args[1:])
	default:
		c.UI.Error(fmt.Sprintf("unknown docs subcommand %q", subcommand))
		return 1
	}
}

func (c *Command) runBulk(args []string) int {
	if len(args) == 0 {
		c.UI.Error("missing bulk subcommand")
		return 1
	}
	switch args[0] {
	case "update":
		return c.runRepair(args[1:], docrepair.OperationBulkUpdate)
	case "timestamps":
		return c.runRepair(args[1:], docrepair.OperationTimestamps)
	case "compress-ids":
		return c.runRepair(args[1:], docrepair.OperationCompressIDs)
	default:
		c.UI.Error(fmt.Sprintf("unknown docs bulk subcommand %q", args[0]))
		return 1
	}
}

func (c *Command) runRepairGroup(args []string) int {
	if len(args) == 0 {
		c.UI.Error("missing repair subcommand")
		return 1
	}
	switch args[0] {
	case "links":
		return c.runRepair(args[1:], docrepair.OperationLinks)
	default:
		c.UI.Error(fmt.Sprintf("unknown docs repair subcommand %q", args[0]))
		return 1
	}
}

func (c *Command) runValidate(args []string) int {
	flags := c.Flags()
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	configPath := flags.Lookup("config").Value.String()
	root := flags.Lookup("root").Value.String()
	project := flags.Lookup("project").Value.String()
	format := flags.Lookup("format").Value.String()

	config, err := projectconfig.LoadConfig(configPath)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	documents, err := docdiscover.Discover(config, docdiscover.Options{RootDir: root, Project: project})
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	result := docschema.ValidateDocuments(documents)

	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			c.UI.Error(err.Error())
			return 1
		}
	case "text":
		c.UI.Output(fmt.Sprintf("Discovered %d documents", len(result.Documents)))
		if len(result.Diagnostics) == 0 {
			c.UI.Output("No diagnostics")
			return 0
		}
		for _, diagnostic := range result.Diagnostics {
			location := diagnostic.File
			if diagnostic.Line > 0 {
				location = fmt.Sprintf("%s:%d", location, diagnostic.Line)
			}
			field := diagnostic.Field
			if field == "" {
				field = "document"
			}
			c.UI.Output(fmt.Sprintf("%s [%s] %s: %s", location, diagnostic.Severity, field, diagnostic.Message))
		}
	default:
		c.UI.Error(fmt.Sprintf("unsupported output format %q", format))
		return 1
	}

	if len(result.Diagnostics) > 0 {
		return 2
	}
	return 0
}

func (c *Command) runRepair(args []string, operation docrepair.Operation) int {
	flags := c.Flags()
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	configPath := flags.Lookup("config").Value.String()
	root := flags.Lookup("root").Value.String()
	project := flags.Lookup("project").Value.String()
	format := flags.Lookup("format").Value.String()
	apply := flags.Lookup("apply").Value.String() == "true"
	field := flags.Lookup("field").Value.String()
	value := flags.Lookup("value").Value.String()
	remove := flags.Lookup("remove").Value.String() == "true"
	startID, _ := strconv.Atoi(flags.Lookup("start-id").Value.String())

	config, err := projectconfig.LoadConfig(configPath)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	documents, err := docdiscover.Discover(config, docdiscover.Options{RootDir: root, Project: project})
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	plan, err := docrepair.Execute(documents, docrepair.Options{
		Operation: operation,
		Apply:     apply,
		Field:     field,
		Value:     value,
		Remove:    remove,
		StartID:   startID,
	})
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(plan); err != nil {
			c.UI.Error(err.Error())
			return 1
		}
	case "text":
		mode := "dry-run"
		if apply {
			mode = "apply"
		}
		c.UI.Output(fmt.Sprintf("%s %s: %d changes, %d warnings", operation, mode, len(plan.Changes), len(plan.Warnings)))
		for _, change := range plan.Changes {
			c.UI.Output(fmt.Sprintf("%s %s %s: %s -> %s", change.File, change.Action, change.Field, change.Before, change.After))
		}
		for _, warning := range plan.Warnings {
			location := warning.File
			if location == "" {
				location = "-"
			}
			c.UI.Output(fmt.Sprintf("warning %s: %s", location, warning.Message))
		}
	default:
		c.UI.Error(fmt.Sprintf("unsupported output format %q", format))
		return 1
	}

	return 0
}
