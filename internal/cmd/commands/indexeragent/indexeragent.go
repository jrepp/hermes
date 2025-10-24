package indexeragent

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/internal/cmd/base"
	"github.com/hashicorp/go-hclog"
)

type Command struct {
	*base.Command

	flagTokenPath    string
	flagCentralURL   string
	flagWorkspace    string
	flagIndexerType  string
	flagPollInterval time.Duration
}

func (c *Command) Synopsis() string {
	return "Run the stateless indexer agent"
}

func (c *Command) Help() string {
	return `Usage: hermes indexer-agent

This command runs the stateless indexer agent that registers with central Hermes
and submits documents via API.` + c.Flags().Help()
}

func (c *Command) Flags() *base.FlagSet {
	f := base.NewFlagSet(flag.NewFlagSet("indexer-agent", flag.ExitOnError))

	f.StringVar(
		&c.flagTokenPath, "token-path", "",
		"[HERMES_INDEXER_TOKEN_PATH] Path to registration token file",
	)
	f.StringVar(
		&c.flagCentralURL, "central-url", "",
		"[HERMES_CENTRAL_URL] Central Hermes URL",
	)
	f.StringVar(
		&c.flagWorkspace, "workspace", "",
		"[HERMES_WORKSPACE_PATH] Workspace directory to index",
	)
	f.StringVar(
		&c.flagIndexerType, "indexer-type", "local-workspace",
		"[HERMES_INDEXER_TYPE] Indexer type (local-workspace, google-workspace, etc)",
	)
	f.DurationVar(
		&c.flagPollInterval, "poll-interval", 5*time.Minute,
		"Interval between indexing runs",
	)

	return f
}

func (c *Command) Run(args []string) int {
	f := c.Flags()
	if err := f.Parse(args); err != nil {
		c.UI.Error(fmt.Sprintf("error parsing flags: %v", err))
		return 1
	}

	// Get configuration from flags or environment
	tokenPath := c.flagTokenPath
	if val, ok := os.LookupEnv("HERMES_INDEXER_TOKEN_PATH"); ok && tokenPath == "" {
		tokenPath = val
	}

	centralURL := c.flagCentralURL
	if val, ok := os.LookupEnv("HERMES_CENTRAL_URL"); ok && centralURL == "" {
		centralURL = val
	}

	workspacePath := c.flagWorkspace
	if val, ok := os.LookupEnv("HERMES_WORKSPACE_PATH"); ok && workspacePath == "" {
		workspacePath = val
	}

	indexerType := c.flagIndexerType
	if val, ok := os.LookupEnv("HERMES_INDEXER_TYPE"); ok && indexerType == "" {
		indexerType = val
	}

	// Validate required parameters
	if tokenPath == "" {
		c.UI.Error("token path is required (--token-path or HERMES_INDEXER_TOKEN_PATH)")
		return 1
	}
	if centralURL == "" {
		c.UI.Error("central URL is required (--central-url or HERMES_CENTRAL_URL)")
		return 1
	}

	// Wait for token file to be created (server needs to start first)
	c.UI.Info(fmt.Sprintf("Waiting for registration token at: %s", tokenPath))
	var registrationToken string
	for i := 0; i < 60; i++ { // Wait up to 60 seconds
		tokenBytes, err := os.ReadFile(tokenPath)
		if err == nil {
			registrationToken = strings.TrimSpace(string(tokenBytes))
			break
		}
		if i%10 == 0 {
			c.UI.Info(fmt.Sprintf("Waiting for token file... (%d/60)", i))
		}
		time.Sleep(1 * time.Second)
	}

	if registrationToken == "" {
		c.UI.Error("failed to read registration token after 60 seconds")
		return 1
	}

	c.UI.Info(fmt.Sprintf("Found registration token: %s...", registrationToken[:30]))

	// Register with central Hermes
	c.UI.Info(fmt.Sprintf("Registering with central Hermes at: %s", centralURL))

	hostname, _ := os.Hostname()
	registerReq := map[string]interface{}{
		"token":          registrationToken,
		"indexer_type":   indexerType,
		"workspace_path": workspacePath,
		"metadata": map[string]interface{}{
			"hostname": hostname,
			"version":  "development",
		},
	}

	registerResp, err := makeRequest(http.MethodPost, centralURL+"/api/v2/indexer/register", registerReq, "")
	if err != nil {
		c.UI.Error(fmt.Sprintf("failed to register: %v", err))
		return 1
	}

	indexerID := registerResp["indexer_id"].(string)
	apiToken := registerResp["api_token"].(string)

	c.UI.Info(fmt.Sprintf("✓ Registered as indexer: %s", indexerID))
	c.UI.Info(fmt.Sprintf("✓ API token: %s...", apiToken[:30]))

	// Start heartbeat loop
	c.UI.Info(fmt.Sprintf("Starting heartbeat loop (interval: %v)", c.flagPollInterval))

	ticker := time.NewTicker(c.flagPollInterval)
	defer ticker.Stop()

	// Send initial heartbeat immediately
	if err := sendHeartbeat(centralURL, indexerID, apiToken, c.Log); err != nil {
		c.UI.Warn(fmt.Sprintf("Initial heartbeat failed: %v", err))
	}

	for {
		select {
		case <-ticker.C:
			if err := sendHeartbeat(centralURL, indexerID, apiToken, c.Log); err != nil {
				c.UI.Warn(fmt.Sprintf("Heartbeat failed: %v", err))
			}
			// TODO: Implement document indexing here
		}
	}
}

func sendHeartbeat(centralURL, indexerID, apiToken string, logger hclog.Logger) error {
	heartbeatReq := map[string]interface{}{
		"indexer_id":     indexerID,
		"status":         "healthy",
		"document_count": 0,
		"metrics": map[string]interface{}{
			"documents_processed": 0,
			"errors":              0,
		},
	}

	_, err := makeRequest(http.MethodPost, centralURL+"/api/v2/indexer/heartbeat", heartbeatReq, apiToken)
	if err != nil {
		return fmt.Errorf("heartbeat request failed: %w", err)
	}

	logger.Debug("heartbeat sent", "indexer_id", indexerID)
	return nil
}

func makeRequest(method, url string, body interface{}, bearerToken string) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error marshaling request: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}
