// Package mcpserver implements the Hermes Model Context Protocol server.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"github.com/hashicorp-forge/hermes/internal/version"
)

const (
	defaultMaxLogEntries    = 100
	defaultMaxLogFieldBytes = 2048
	callStatusError         = "error"
)

// Options configures the MCP server.
type Options struct {
	Logger           hclog.Logger
	ProjectsConfig   string
	RootDir          string
	MaxLogEntries    int
	MaxLogFieldBytes int
}

// Server is the Hermes MCP server wrapper.
type Server struct {
	mcp *mcpgo.MCPServer
	log hclog.Logger

	maxLogEntries    int
	maxLogFieldBytes int
	projectsConfig   string
	rootDir          string

	mu         sync.RWMutex
	handlers   map[string]mcpgo.ToolHandlerFunc
	callLog    []CallLogEntry
	initEvents []InitializeEvent
}

// CallLogEntry records a bounded tool call summary.
type CallLogEntry struct {
	Tool      string    `json:"tool"`
	StartedAt time.Time `json:"startedAt"`
	Duration  string    `json:"duration"`
	Status    string    `json:"status"`
	Arguments string    `json:"arguments,omitempty"`
	Result    string    `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// InitializeEvent records a client initialize event.
type InitializeEvent struct {
	ClientName    string    `json:"clientName,omitempty"`
	ClientVersion string    `json:"clientVersion,omitempty"`
	ObservedAt    time.Time `json:"observedAt"`
}

// New creates a Hermes MCP server with Phase 0 tools registered.
func New(opts Options) *Server {
	if opts.Logger == nil {
		opts.Logger = hclog.NewNullLogger()
	}
	if opts.MaxLogEntries <= 0 {
		opts.MaxLogEntries = defaultMaxLogEntries
	}
	if opts.MaxLogFieldBytes <= 0 {
		opts.MaxLogFieldBytes = defaultMaxLogFieldBytes
	}

	s := &Server{
		log:              opts.Logger,
		maxLogEntries:    opts.MaxLogEntries,
		maxLogFieldBytes: opts.MaxLogFieldBytes,
		projectsConfig:   opts.ProjectsConfig,
		rootDir:          opts.RootDir,
		handlers:         make(map[string]mcpgo.ToolHandlerFunc),
	}
	if s.projectsConfig == "" {
		s.projectsConfig = "./testing/projects.hcl"
	}
	if s.rootDir == "" {
		s.rootDir = "."
	}

	hooks := &mcpgo.Hooks{}
	hooks.AddAfterInitialize(func(_ context.Context, _ any, message *mcp.InitializeRequest, _ *mcp.InitializeResult) {
		s.recordInitialize(message)
	})

	s.mcp = mcpgo.NewMCPServer(
		"hermes",
		version.Version,
		mcpgo.WithToolCapabilities(false),
		mcpgo.WithRecovery(),
		mcpgo.WithHooks(hooks),
		mcpgo.WithInstructions("Use Hermes tools to inspect project, document, and search context. Phase 0 tools are read-only placeholders."),
	)

	s.registerTools()

	return s
}

// AddTool registers a tool and wraps its handler with recovery and call logging.
func (s *Server) AddTool(tool mcp.Tool, handler mcpgo.ToolHandlerFunc) {
	wrapped := s.wrapToolHandler(tool.Name, handler)

	s.mu.Lock()
	s.handlers[tool.Name] = wrapped
	s.mu.Unlock()

	s.mcp.AddTool(tool, wrapped)
}

// ServeStdio serves MCP over stdin/stdout until the context is canceled.
func (s *Server) ServeStdio(ctx context.Context) error {
	stdio := mcpgo.NewStdioServer(s.mcp)
	return stdio.Listen(ctx, os.Stdin, os.Stdout)
}

// HTTPHandler returns an HTTP handler serving streamable MCP at endpoint.
func (s *Server) HTTPHandler(endpoint string) http.Handler {
	if endpoint == "" {
		endpoint = "/mcp"
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}

	streamable := mcpgo.NewStreamableHTTPServer(s.mcp, mcpgo.WithEndpointPath(endpoint))
	mux := http.NewServeMux()
	mux.Handle(endpoint, streamable)
	return mux
}

// CallTool calls a registered tool directly without transport.
func (s *Server) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	s.mu.RLock()
	handler, ok := s.handlers[name]
	s.mu.RUnlock()
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("unknown tool %q", name)), nil
	}

	return handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
}

// CallLog returns a copy of recent bounded tool call records.
func (s *Server) CallLog() []CallLogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]CallLogEntry, len(s.callLog))
	copy(entries, s.callLog)
	return entries
}

// InitializeEvents returns a copy of recorded initialize events.
func (s *Server) InitializeEvents() []InitializeEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]InitializeEvent, len(s.initEvents))
	copy(events, s.initEvents)
	return events
}

func (s *Server) wrapToolHandler(name string, handler mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
		started := time.Now()
		entry := CallLogEntry{
			Tool:      name,
			StartedAt: started.UTC(),
			Arguments: s.truncate(redactSecrets(mustJSON(request.Params.Arguments))),
		}

		defer func() {
			entry.Duration = time.Since(started).String()
			if r := recover(); r != nil {
				result = mcp.NewToolResultError(fmt.Sprintf("tool %q panicked: %v", name, r))
				err = nil
				entry.Status = "panic"
				entry.Error = s.truncate(fmt.Sprint(r))
			} else {
				switch {
				case err != nil:
					result = mcp.NewToolResultErrorFromErr(fmt.Sprintf("tool %q failed", name), err)
					err = nil
					entry.Status = callStatusError
					entry.Error = s.truncate(resultText(result))
				case result != nil && result.IsError:
					entry.Status = callStatusError
					entry.Error = s.truncate(resultText(result))
				default:
					entry.Status = "ok"
					entry.Result = s.truncate(resultText(result))
				}
			}
			s.recordCall(entry)
		}()

		return handler(ctx, request)
	}
}

func (s *Server) recordCall(entry CallLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.callLog = append(s.callLog, entry)
	if len(s.callLog) > s.maxLogEntries {
		s.callLog = s.callLog[len(s.callLog)-s.maxLogEntries:]
	}
}

func (s *Server) recordInitialize(message *mcp.InitializeRequest) {
	event := InitializeEvent{ObservedAt: time.Now().UTC()}
	if message != nil {
		event.ClientName = message.Params.ClientInfo.Name
		event.ClientVersion = message.Params.ClientInfo.Version
	}

	s.mu.Lock()
	s.initEvents = append(s.initEvents, event)
	if len(s.initEvents) > s.maxLogEntries {
		s.initEvents = s.initEvents[len(s.initEvents)-s.maxLogEntries:]
	}
	s.mu.Unlock()
}

func (s *Server) truncate(value string) string {
	if len(value) <= s.maxLogFieldBytes {
		return value
	}
	return value[:s.maxLogFieldBytes] + "..."
}

func mustJSON(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

func redactSecrets(value string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return value
	}
	redactMap(data)
	return mustJSON(data)
}

func redactMap(data map[string]any) {
	for key, value := range data {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "credential") {
			data[key] = "[redacted]"
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			redactMap(nested)
		}
	}
}

func resultText(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	if result.StructuredContent != nil {
		return mustJSON(result.StructuredContent)
	}
	parts := make([]string, 0, len(result.Content))
	for _, content := range result.Content {
		if text, ok := content.(mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func readOnlyTool(name, description string) mcp.Tool {
	return mcp.NewTool(
		name,
		mcp.WithDescription(description),
		mcp.WithString("action", mcp.Description("Read-only action to perform"), mcp.Required()),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
}
