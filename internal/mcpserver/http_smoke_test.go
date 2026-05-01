package mcpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

func TestHTTPInitializeAndToolCallSmoke(t *testing.T) {
	srv := testServer()
	httpServer := httptest.NewServer(srv.HTTPHandler("/mcp"))
	defer httpServer.Close()

	initResp := postMCP(t, httpServer.URL+"/mcp", "", map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "hermes-test",
				"version": "0.0.0",
			},
		},
	})
	defer initResp.Body.Close()
	if initResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(initResp.Body)
		t.Fatalf("initialize status = %d body=%s", initResp.StatusCode, string(body))
	}
	sessionID := initResp.Header.Get(mcpgo.HeaderKeySessionID)
	if sessionID == "" {
		t.Fatal("expected session header")
	}

	toolResp := postMCP(t, httpServer.URL+"/mcp", sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "project",
			"arguments": map[string]any{
				"action": "list",
			},
		},
	})
	defer toolResp.Body.Close()
	if toolResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(toolResp.Body)
		t.Fatalf("tool call status = %d body=%s", toolResp.StatusCode, string(body))
	}

	var response struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error,omitempty"`
	}
	if err := json.NewDecoder(toolResp.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error != nil {
		t.Fatalf("unexpected tool error: %#v", response.Error)
	}
	if response.Result["isError"] == true {
		t.Fatalf("unexpected MCP tool error result: %#v", response.Result)
	}
}

func postMCP(t *testing.T, url, sessionID string, payload map[string]any) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if sessionID != "" {
		req.Header.Set(mcpgo.HeaderKeySessionID, sessionID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post MCP: %v", err)
	}
	return resp
}
