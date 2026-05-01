package operator

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/db"
	searchoutbox "github.com/hashicorp-forge/hermes/pkg/search/outbox"
)

// SearchOutboxCommand provides operator controls for failed search projection events.
type SearchOutboxCommand struct {
	*base.Command
	flagConfig string
	flagLimit  int
	flagNote   string
}

// Synopsis returns a short command description.
func (c *SearchOutboxCommand) Synopsis() string {
	return "Inspect and operate search outbox failures"
}

// Help returns command usage text.
func (c *SearchOutboxCommand) Help() string {
	return `Usage: hermes operator search-outbox [options] <list|retry|skip|rebuild-current> [event-id]

  list                       List failed and DLQ search outbox events.
  retry <event-id>           Return a failed, DLQ, or skipped event to pending.
  skip <event-id>            Mark a failed or DLQ event skipped so later same-aggregate events can proceed.
  rebuild-current <event-id> Enqueue a fresh projection from current database truth.` + c.Flags().Help()
}

// Flags returns command flags.
func (c *SearchOutboxCommand) Flags() *base.FlagSet {
	f := base.NewFlagSet(flag.NewFlagSet("search-outbox", flag.ExitOnError))
	f.StringVar(&c.flagConfig, "config", "", "(Required) Path to Hermes config file")
	f.IntVar(&c.flagLimit, "limit", 50, "Maximum number of events to list")
	f.StringVar(&c.flagNote, "note", "", "Required operator note for skip")
	return f
}

// Run executes the selected search-outbox operator subcommand.
func (c *SearchOutboxCommand) Run(args []string) int {
	flags := c.Flags()
	if err := flags.Parse(args); err != nil {
		c.UI.Error(fmt.Sprintf("error parsing flags: %v", err))
		return 1
	}
	if c.flagConfig == "" {
		c.UI.Error("config flag is required")
		return 1
	}

	remaining := flags.Args()
	if len(remaining) == 0 {
		c.UI.Error("subcommand is required: list, retry, or skip")
		return 1
	}

	cfg, err := config.NewConfig(c.flagConfig, "")
	if err != nil {
		c.UI.Error(fmt.Sprintf("error parsing config file: %v", err))
		return 1
	}
	database, err := db.NewDB(*cfg.Postgres)
	if err != nil {
		c.UI.Error(fmt.Sprintf("error initializing database: %v", err))
		return 1
	}

	return c.runSubcommand(database, remaining)
}

func (c *SearchOutboxCommand) runSubcommand(database *gorm.DB, remaining []string) int {
	switch remaining[0] {
	case "list":
		return c.runList(database)
	case "retry":
		return c.runRetry(database, remaining)
	case "skip":
		return c.runSkip(database, remaining)
	case "rebuild-current":
		return c.runRebuildCurrent(database, remaining)
	default:
		c.UI.Error(fmt.Sprintf("unknown search-outbox subcommand %q", remaining[0]))
		return 1
	}
}

func (c *SearchOutboxCommand) runList(database *gorm.DB) int {
	events, err := searchoutbox.ListFailedEvents(database, c.flagLimit)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	if len(events) == 0 {
		c.UI.Info("No failed or DLQ search outbox events found")
		return 0
	}
	for i := range events {
		event := &events[i]
		c.UI.Info(fmt.Sprintf(
			"id=%d status=%s aggregate=%s/%s sequence=%d attempts=%d event=%s error=%q",
			event.ID,
			event.Status,
			event.AggregateType,
			event.AggregateID,
			event.Sequence,
			event.AttemptCount,
			event.EventType,
			strings.TrimSpace(event.ErrorMessage),
		))
	}
	return 0
}

func (c *SearchOutboxCommand) runRetry(database *gorm.DB, remaining []string) int {
	id, ok := parseSearchOutboxEventID(c, remaining)
	if !ok {
		return 1
	}
	if err := searchoutbox.RetryEvent(database, id, time.Now().UTC()); err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	c.UI.Info(fmt.Sprintf("Queued search outbox event %d for retry", id))
	return 0
}

func (c *SearchOutboxCommand) runSkip(database *gorm.DB, remaining []string) int {
	id, ok := parseSearchOutboxEventID(c, remaining)
	if !ok {
		return 1
	}
	if err := searchoutbox.SkipEvent(database, id, c.flagNote, time.Now().UTC()); err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	c.UI.Info(fmt.Sprintf("Skipped search outbox event %d", id))
	return 0
}

func (c *SearchOutboxCommand) runRebuildCurrent(database *gorm.DB, remaining []string) int {
	id, ok := parseSearchOutboxEventID(c, remaining)
	if !ok {
		return 1
	}
	event, err := searchoutbox.RebuildCurrent(database, id)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	c.UI.Info(fmt.Sprintf(
		"Enqueued rebuilt search projection event %d for %s/%s sequence=%d",
		event.ID,
		event.AggregateType,
		event.AggregateID,
		event.Sequence,
	))
	return 0
}

func parseSearchOutboxEventID(c *SearchOutboxCommand, args []string) (uint, bool) {
	if len(args) < 2 {
		c.UI.Error("event-id is required")
		return 0, false
	}
	id, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil || id == 0 {
		c.UI.Error("event-id must be a positive integer")
		return 0, false
	}
	return uint(id), true
}
