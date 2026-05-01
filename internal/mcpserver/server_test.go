package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestCallToolProjectSuccess(t *testing.T) {
	srv := testServer()

	result, err := srv.CallTool(context.Background(), "project", map[string]any{"action": "list"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result: %#v", result)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map, got %T", result.StructuredContent)
	}
	if structured["action"] != "list" {
		t.Fatalf("unexpected action: %#v", structured["action"])
	}
}

func TestCallToolUnknownTool(t *testing.T) {
	srv := testServer()

	result, err := srv.CallTool(context.Background(), "missing", map[string]any{})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result")
	}
	if !strings.Contains(textResult(t, result), "unknown tool") {
		t.Fatalf("expected unknown tool message, got %q", textResult(t, result))
	}
}

func TestCallToolHandlerErrorRecoveredAsToolError(t *testing.T) {
	srv := testServer()

	result, err := srv.CallTool(context.Background(), "project", map[string]any{"action": "error"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result")
	}
	if !strings.Contains(textResult(t, result), "test handler error") {
		t.Fatalf("expected handler error text, got %q", textResult(t, result))
	}
}

func TestCallToolPanicRecoveredAsToolError(t *testing.T) {
	srv := testServer()

	result, err := srv.CallTool(context.Background(), "project", map[string]any{"action": "panic"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result")
	}
	if !strings.Contains(textResult(t, result), "panicked") {
		t.Fatalf("expected panic text, got %q", textResult(t, result))
	}
}

func TestCallLogIsBoundedAndTruncated(t *testing.T) {
	srv := New(Options{ProjectsConfig: testProjectsConfig(), RootDir: testRoot(), MaxLogEntries: 2, MaxLogFieldBytes: 16})

	for i := 0; i < 3; i++ {
		_, err := srv.CallTool(context.Background(), "project", map[string]any{
			"action": "get",
			"name":   "a-very-long-project-name",
		})
		if err != nil {
			t.Fatalf("CallTool returned error: %v", err)
		}
	}

	entries := srv.CallLog()
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(entries))
	}
	if !strings.HasSuffix(entries[0].Arguments, "...") {
		t.Fatalf("expected truncated arguments, got %q", entries[0].Arguments)
	}
}

func TestContextCallsToolIncludesLog(t *testing.T) {
	srv := testServer()
	_, err := srv.CallTool(context.Background(), "project", map[string]any{"action": "list"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}

	result, err := srv.CallTool(context.Background(), "context", map[string]any{"action": "calls"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map, got %T", result.StructuredContent)
	}
	if structured["calls"] == nil {
		t.Fatal("expected calls in structured content")
	}
}

func TestContextValidateReturnsDiagnostics(t *testing.T) {
	srv := testServer()

	result, err := srv.CallTool(context.Background(), "context", map[string]any{"action": "validate"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result: %#v", result)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map, got %T", result.StructuredContent)
	}
	if structured["documents"] == nil || structured["diagnostics"] == nil {
		t.Fatalf("expected validation fields, got %#v", structured)
	}
}

func TestRepairPlanDryRun(t *testing.T) {
	srv := New(Options{ProjectsConfig: repairProjectsConfig(), RootDir: repairRoot()})

	result, err := srv.CallTool(context.Background(), "repair", map[string]any{"action": "plan"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result: %#v", result)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map, got %T", result.StructuredContent)
	}
	if structured["plan"] == nil {
		t.Fatalf("expected plan content, got %#v", structured)
	}
}

func TestSyncStatusTool(t *testing.T) {
	srv := New(Options{ProjectsConfig: edgeProjectsConfig(), RootDir: edgeRoot()})

	result, err := srv.CallTool(context.Background(), "sync", map[string]any{"action": "status", "project": "edge"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result: %#v", result)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map, got %T", result.StructuredContent)
	}
	if structured["status"] == nil {
		t.Fatalf("expected status content, got %#v", structured)
	}
}

func TestSearchQueryTool(t *testing.T) {
	srv := New(Options{ProjectsConfig: edgeProjectsConfig(), RootDir: edgeRoot()})

	result, err := srv.CallTool(context.Background(), "search", map[string]any{"action": "query", "project": "edge", "query": "clean"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result: %#v", result)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map, got %T", result.StructuredContent)
	}
	if structured["result"] == nil {
		t.Fatalf("expected search result content, got %#v", structured)
	}
}

func TestDocumentToolActions(t *testing.T) {
	srv := New(Options{ProjectsConfig: edgeProjectsConfig(), RootDir: edgeRoot()})

	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{name: "list", args: map[string]any{"action": "list", "project": "edge", "detail": "compact"}},
		{name: "get", args: map[string]any{"action": "get", "project": "edge", "path": "docs/clean.md"}},
		{name: "content", args: map[string]any{"action": "content", "project": "edge", "path": "docs/clean.md", "detail": "minimal"}},
		{name: "validate", args: map[string]any{"action": "validate", "project": "edge", "path": "docs/dirty.md"}},
		{name: "links", args: map[string]any{"action": "links", "project": "edge", "path": "docs/clean.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := srv.CallTool(context.Background(), "document", tc.args)
			if err != nil {
				t.Fatalf("CallTool returned error: %v", err)
			}
			if result.IsError {
				t.Fatalf("expected success result: %#v", result)
			}
		})
	}
}

func TestDocumentUpdateDryRunDoesNotMutate(t *testing.T) {
	root := copyEdgeFixture(t)
	srv := New(Options{ProjectsConfig: filepath.Join(root, "projects.hcl"), RootDir: root})
	path := filepath.Join(root, "docs", "clean.md")
	before := readTestFile(t, path)

	result, err := srv.CallTool(context.Background(), "document", map[string]any{"action": "update", "project": "edge", "path": "docs/clean.md", "content": "# Changed", "dry_run": true})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result: %#v", result)
	}
	if after := readTestFile(t, path); after != before {
		t.Fatal("dry-run update mutated file")
	}
}

func TestDocumentUpdateApplyMutates(t *testing.T) {
	root := copyEdgeFixture(t)
	srv := New(Options{ProjectsConfig: filepath.Join(root, "projects.hcl"), RootDir: root})
	path := filepath.Join(root, "docs", "clean.md")

	result, err := srv.CallTool(context.Background(), "document", map[string]any{"action": "update", "project": "edge", "path": "docs/clean.md", "content": "# Changed", "dry_run": false})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result: %#v", result)
	}
	if after := readTestFile(t, path); after != "# Changed" {
		t.Fatalf("expected applied content, got %q", after)
	}
}

func TestToolActionFailures(t *testing.T) {
	srv := New(Options{ProjectsConfig: edgeProjectsConfig(), RootDir: edgeRoot()})
	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{tool: "document", args: map[string]any{"action": "missing"}},
		{tool: "search", args: map[string]any{"action": "query"}},
		{tool: "sync", args: map[string]any{"action": "push"}},
		{tool: "repair", args: map[string]any{"action": "unknown"}},
	} {
		result, err := srv.CallTool(context.Background(), tc.tool, tc.args)
		if err != nil {
			t.Fatalf("CallTool returned error for %s: %v", tc.tool, err)
		}
		if !result.IsError {
			t.Fatalf("expected error result for %s %#v", tc.tool, tc.args)
		}
	}
}

func TestCallLogRedactsSecrets(t *testing.T) {
	srv := New(Options{ProjectsConfig: edgeProjectsConfig(), RootDir: edgeRoot()})
	_, err := srv.CallTool(context.Background(), "search", map[string]any{"action": "query", "query": "clean", "auth_token": "secret-value"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	entries := srv.CallLog()
	if len(entries) == 0 {
		t.Fatal("expected call log entry")
	}
	if strings.Contains(entries[len(entries)-1].Arguments, "secret-value") {
		t.Fatalf("secret was not redacted: %s", entries[len(entries)-1].Arguments)
	}
}

func testServer() *Server {
	return New(Options{ProjectsConfig: testProjectsConfig(), RootDir: testRoot()})
}

func testRoot() string {
	return filepath.Join("..", "..", "pkg", "docdiscover", "testdata", "monorepo")
}

func testProjectsConfig() string {
	return filepath.Join(testRoot(), "projects.hcl")
}

func repairRoot() string {
	return filepath.Join("..", "..", "pkg", "docrepair", "testdata", "repair")
}

func repairProjectsConfig() string {
	return filepath.Join(repairRoot(), "projects.hcl")
}

func edgeRoot() string {
	return filepath.Join("..", "..", "pkg", "edge", "testdata")
}

func edgeProjectsConfig() string {
	return filepath.Join(edgeRoot(), "projects.hcl")
}

func textResult(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		return ""
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	return text.Text
}

func copyEdgeFixture(t *testing.T) string {
	t.Helper()
	src := edgeRoot()
	dst := t.TempDir()
	copyTree(t, src, dst)
	return dst
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("ReadDir %s: %v", src, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				t.Fatalf("MkdirAll %s: %v", dstPath, err)
			}
			copyTree(t, srcPath, dstPath)
			continue
		}
		data, err := os.ReadFile(srcPath) //nolint:gosec // test fixture copy.
		if err != nil {
			t.Fatalf("ReadFile %s: %v", srcPath, err)
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", dstPath, err)
		}
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test fixture read.
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	return string(data)
}
