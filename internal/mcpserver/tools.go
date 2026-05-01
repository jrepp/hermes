package mcpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hashicorp-forge/hermes/pkg/docdiscover"
	"github.com/hashicorp-forge/hermes/pkg/docrepair"
	"github.com/hashicorp-forge/hermes/pkg/docschema"
	edgepkg "github.com/hashicorp-forge/hermes/pkg/edge"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
)

const searchActionHybrid = "hybrid"

func (s *Server) registerTools() {
	s.AddTool(readOnlyTool("project", "Inspect Hermes project configuration"), s.handleProjectTool)
	s.AddTool(readOnlyTool("context", "Inspect current Hermes agent context"), s.handleContextTool)
	s.AddTool(readOnlyTool("document", "Inspect and update local documentation files"), s.handleDocumentTool)
	s.AddTool(readOnlyTool("repair", "Plan and apply documentation repairs"), s.handleRepairTool)
	s.AddTool(readOnlyTool("sync", "Inspect edge sync status and plans"), s.handleSyncTool)
	s.AddTool(readOnlyTool("search", "Search local edge documentation indexes"), s.handleSearchTool)
}

func (s *Server) handleProjectTool(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := request.GetString("action", "")
	switch action {
	case "list", "discover":
		config, err := projectconfig.LoadConfig(s.projectsConfig)
		if err != nil {
			return nil, err
		}
		projects := make([]map[string]any, 0, len(config.Projects))
		for _, name := range config.ListProjects() {
			project := config.Projects[name]
			projects = append(projects, map[string]any{
				"name":      project.Name,
				"title":     project.Title,
				"shortName": project.ShortName,
				"status":    project.Status,
				"lanes":     len(project.Lanes),
			})
		}
		return structured(map[string]any{"action": action, "projects": projects})
	case "get":
		name := request.GetString("name", "")
		config, err := projectconfig.LoadConfig(s.projectsConfig)
		if err != nil {
			return nil, err
		}
		project, err := config.GetProject(name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"action": action, "name": name, "project": project.ToSummary()})
	case "providers":
		project, err := s.projectFromRequest(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"action": action, "project": project.Name, "providers": project.ToSummary().Providers})
	case "lanes":
		project, err := s.projectFromRequest(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"action": action, "project": project.Name, "lanes": project.Lanes})
	case "panic":
		panic("test panic")
	case "error":
		return nil, fmt.Errorf("test handler error")
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unsupported project action %q", action)), nil
	}
}

func (s *Server) handleContextTool(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := request.GetString("action", "")
	switch action {
	case "summary":
		config, err := projectconfig.LoadConfig(s.projectsConfig)
		if err != nil {
			return nil, err
		}
		documents, err := docdiscover.Discover(config, docdiscover.Options{RootDir: s.rootDir})
		if err != nil {
			return nil, err
		}
		return structured(map[string]any{
			"action": action,
			"summary": map[string]any{
				"phase":        "1",
				"capabilities": []string{"project", "context", "validation"},
				"writes":       false,
				"projects":     len(config.Projects),
				"documents":    len(documents),
			},
		})
	case "project":
		project, err := s.projectFromRequest(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"action": action, "project": project.ToSummary()})
	case "document":
		return structured(map[string]any{"action": action, "document": nil, "implemented": false, "message": "Document lookup is deferred until document tools are added."})
	case "search_context":
		return structured(map[string]any{"action": action, "search": nil, "implemented": false, "message": "Search context will be wired after edge search is connected."})
	case "validate":
		project := request.GetString("project", "")
		config, err := projectconfig.LoadConfig(s.projectsConfig)
		if err != nil {
			return nil, err
		}
		documents, err := docdiscover.Discover(config, docdiscover.Options{RootDir: s.rootDir, Project: project})
		if err != nil {
			return nil, err
		}
		result := docschema.ValidateDocuments(documents)
		return structured(map[string]any{"action": action, "documents": len(result.Documents), "diagnostics": result.Diagnostics})
	case "calls":
		return structured(map[string]any{"action": action, "calls": s.CallLog()})
	case "initialize_events":
		return structured(map[string]any{"action": action, "events": s.InitializeEvents()})
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unsupported context action %q", action)), nil
	}
}

func (s *Server) handleDocumentTool(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := request.GetString("action", "")
	switch action {
	case "list":
		return s.handleDocumentList(request)
	case "get":
		return s.handleDocumentGet(request)
	case "content":
		return s.handleDocumentContent(request)
	case "validate":
		return s.handleDocumentValidate(request)
	case "links":
		return s.handleDocumentLinks(request)
	case "update":
		return s.handleDocumentUpdate(request)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unsupported document action %q", action)), nil
	}
}

func (s *Server) handleDocumentList(request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	docs, err := s.discoverDocuments(request)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(docs))
	for _, doc := range docs {
		items = append(items, documentSummary(doc))
	}
	return structuredWithDetail(request, map[string]any{"action": "list", "documents": items, "count": len(items)})
}

func (s *Server) handleDocumentGet(request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	doc, err := s.documentFromRequest(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return structuredWithDetail(request, map[string]any{"action": "get", "document": documentSummary(doc)})
}

func (s *Server) handleDocumentContent(request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	doc, err := s.documentFromRequest(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	data, err := os.ReadFile(doc.Path) //nolint:gosec // path comes from local discovery.
	if err != nil {
		return nil, err
	}
	return structuredWithDetail(request, map[string]any{"action": "content", "document": documentSummary(doc), "content": string(data)})
}

func (s *Server) handleDocumentValidate(request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	doc, err := s.documentFromRequest(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	diagnostics := docschema.ValidateDocument(doc)
	return structuredWithDetail(request, map[string]any{"action": "validate", "document": documentSummary(doc), "diagnostics": diagnostics})
}

func (s *Server) handleDocumentLinks(request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	doc, err := s.documentFromRequest(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	plan, err := docrepair.Execute([]docdiscover.Document{doc}, docrepair.Options{Operation: docrepair.OperationLinks})
	if err != nil {
		return nil, err
	}
	return structuredWithDetail(request, map[string]any{"action": "links", "document": documentSummary(doc), "warnings": plan.Warnings})
}

func (s *Server) handleDocumentUpdate(request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	doc, err := s.documentFromRequest(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	content := request.GetString("content", "")
	if content == "" {
		return mcp.NewToolResultError("content is required for document update"), nil
	}
	dryRun := request.GetBool("dry_run", true)
	plan := map[string]any{"file": doc.RelPath, "dryRun": dryRun, "bytes": len(content), "action": "replace_content", "applied": false}
	if !dryRun {
		if err := os.WriteFile(doc.Path, []byte(content), 0o600); err != nil {
			return nil, err
		}
		plan["applied"] = true
	}
	return structuredWithDetail(request, map[string]any{"action": "update", "plan": plan})
}

func (s *Server) projectFromRequest(request mcp.CallToolRequest) (*projectconfig.Project, error) {
	name := request.GetString("name", "")
	if name == "" {
		return nil, fmt.Errorf("project name is required")
	}
	config, err := projectconfig.LoadConfig(s.projectsConfig)
	if err != nil {
		return nil, err
	}
	return config.GetProject(name)
}

func (s *Server) discoverDocuments(request mcp.CallToolRequest) ([]docdiscover.Document, error) {
	config, err := projectconfig.LoadConfig(s.projectsConfig)
	if err != nil {
		return nil, err
	}
	return docdiscover.Discover(config, docdiscover.Options{RootDir: s.rootDir, Project: request.GetString("project", "")})
}

func (s *Server) documentFromRequest(request mcp.CallToolRequest) (docdiscover.Document, error) {
	path := filepath.ToSlash(request.GetString("path", ""))
	if path == "" {
		return docdiscover.Document{}, fmt.Errorf("path is required")
	}
	docs, err := s.discoverDocuments(request)
	if err != nil {
		return docdiscover.Document{}, err
	}
	for _, doc := range docs {
		if doc.RelPath == path || filepath.ToSlash(doc.Path) == path {
			return doc, nil
		}
	}
	return docdiscover.Document{}, fmt.Errorf("document %q not found", path)
}

func documentSummary(doc docdiscover.Document) map[string]any {
	return map[string]any{
		"path":    doc.RelPath,
		"project": doc.ProjectName,
		"lane":    doc.LaneName,
		"schema":  doc.Schema,
	}
}

func (s *Server) handleRepairTool(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := request.GetString("action", "")
	operation, ok := map[string]docrepair.Operation{
		"plan":         docrepair.OperationMigrate,
		"apply":        docrepair.OperationMigrate,
		"migrate":      docrepair.OperationMigrate,
		"bulk_update":  docrepair.OperationBulkUpdate,
		"timestamps":   docrepair.OperationTimestamps,
		"compress_ids": docrepair.OperationCompressIDs,
		"links":        docrepair.OperationLinks,
	}[action]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("unsupported repair action %q", action)), nil
	}

	dryRun := request.GetBool("dry_run", true)
	if action == "plan" {
		dryRun = true
	}
	if action == "apply" {
		dryRun = false
	}

	project := request.GetString("project", "")
	config, err := projectconfig.LoadConfig(s.projectsConfig)
	if err != nil {
		return nil, err
	}
	documents, err := docdiscover.Discover(config, docdiscover.Options{RootDir: s.rootDir, Project: project})
	if err != nil {
		return nil, err
	}
	plan, err := docrepair.Execute(documents, docrepair.Options{
		Operation: operation,
		Apply:     !dryRun,
		Field:     request.GetString("field", ""),
		Value:     request.GetString("value", ""),
		Remove:    request.GetBool("remove", false),
		StartID:   request.GetInt("start_id", 1),
	})
	if err != nil {
		return nil, err
	}

	return structured(map[string]any{
		"action": action,
		"plan":   plan,
	})
}

func (s *Server) handleSyncTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := request.GetString("action", "")
	switch action {
	case "status", "plan_pull", "plan_push":
		status, err := edgepkg.Status(ctx, edgepkg.Options{
			ConfigPath:  s.projectsConfig,
			RootDir:     s.rootDir,
			Project:     request.GetString("project", ""),
			ProbeRemote: request.GetBool("probe_remote", false),
		})
		if err != nil {
			return nil, err
		}
		return structured(map[string]any{"action": action, "status": status, "dryRun": true})
	case "pull", "push":
		return mcp.NewToolResultError("sync pull/push mutation is not implemented; use plan_pull or plan_push"), nil
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unsupported sync action %q", action)), nil
	}
}

func (s *Server) handleSearchTool(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := request.GetString("action", "")
	switch action {
	case "query", searchActionHybrid:
		query := request.GetString("query", "")
		if query == "" {
			return mcp.NewToolResultError("query is required"), nil
		}
		result, err := edgepkg.SearchLocal(edgepkg.Options{ConfigPath: s.projectsConfig, RootDir: s.rootDir, Project: request.GetString("project", "")}, query, request.GetInt("limit", 10), request.GetBool("debug", false) || action == searchActionHybrid)
		if err != nil {
			return nil, err
		}
		if action == searchActionHybrid {
			result.Mode = searchActionHybrid
		}
		return structured(map[string]any{"action": action, "result": result})
	case "index_status":
		_, indexResult, err := edgepkg.BuildIndex(edgepkg.Options{ConfigPath: s.projectsConfig, RootDir: s.rootDir, Project: request.GetString("project", "")})
		if err != nil {
			return nil, err
		}
		return structured(map[string]any{"action": action, "index": indexResult})
	case "similar":
		return mcp.NewToolResultError("similar search requires vector indexing and is not enabled in this phase"), nil
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unsupported search action %q", action)), nil
	}
}

func structured(data map[string]any) (*mcp.CallToolResult, error) {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return mcp.NewToolResultStructured(data, fmt.Sprintf("Hermes MCP result fields: %v", keys)), nil
}

func structuredWithDetail(request mcp.CallToolRequest, data map[string]any) (*mcp.CallToolResult, error) {
	detail := request.GetString("detail", "full")
	if detail == "minimal" {
		minimal := map[string]any{"action": data["action"]}
		for _, key := range []string{"count", "plan", "diagnostics", "warnings"} {
			if value, ok := data[key]; ok {
				minimal[key] = value
			}
		}
		return structured(minimal)
	}
	if detail == "compact" {
		delete(data, "content")
		if docs, ok := data["documents"].([]map[string]any); ok && len(docs) > 20 {
			data["documents"] = docs[:20]
			data["truncated"] = true
		}
	}
	if detail != "full" && detail != "compact" && detail != "minimal" {
		return mcp.NewToolResultError(fmt.Sprintf("unsupported detail level %q", detail)), nil
	}
	return structured(data)
}
