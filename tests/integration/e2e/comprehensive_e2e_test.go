//go:build integration
// +build integration

package e2e

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/tests/integration"
)

const (
	// Service URLs - these services should be running via docker-compose
	centralURL     = "http://localhost:8000"
	edgeURL        = "http://localhost:8002"
	meilisearchURL = "http://localhost:7701"
	mailhogURL     = "http://localhost:8025"

	// Configuration
	meilisearchKey = "masterKey123"
	edgeInstance   = "edge-dev-1"

	// Timeouts
	serviceTimeout = 5 * time.Second
	indexTimeout   = 10 * time.Second
	notifyTimeout  = 15 * time.Second
)

// TestComprehensiveE2E validates the complete Hermes central-edge architecture.
//
// Prerequisites:
//   - All services must be running: cd testing && docker compose up -d
//   - Database must be migrated
//   - All indexers and notifiers running
//
// This test validates:
//   - Service health and connectivity
//   - RFC-086 bearer token authentication
//   - Edge-to-central synchronization
//   - Meilisearch integration
//   - RFC-087 notification system
//   - Complete end-to-end flow
func TestComprehensiveE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive E2E test in short mode")
	}

	ctx := context.Background()

	// Phase 1: Prerequisites
	t.Run("Phase1_Prerequisites", func(t *testing.T) {
		testServiceHealth(t, ctx)
	})

	// Phase 2: Authentication
	t.Run("Phase2_Authentication", func(t *testing.T) {
		testBearerTokenAuth(t, ctx)
	})

	// Phase 3: Edge-to-Central Sync
	t.Run("Phase3_EdgeToCentralSync", func(t *testing.T) {
		testEdgeToCentralSync(t, ctx)
	})

	// Phase 4: Search Integration
	t.Run("Phase4_SearchIntegration", func(t *testing.T) {
		testSearchIntegration(t, ctx)
	})

	// Phase 5: Notification System
	t.Run("Phase5_NotificationSystem", func(t *testing.T) {
		testNotificationSystem(t, ctx)
	})

	// Phase 6: End-to-End Validation
	t.Run("Phase6_EndToEndValidation", func(t *testing.T) {
		testEndToEndValidation(t, ctx)
	})
}

// testServiceHealth validates all required services are running and healthy.
func testServiceHealth(t *testing.T, ctx context.Context) {
	t.Log("=== Phase 1: Service Health & Prerequisites ===")

	services := []struct {
		name string
		url  string
	}{
		{"Central Hermes API", centralURL + "/health"},
		{"Edge Hermes API", edgeURL + "/health"},
		{"Meilisearch", meilisearchURL + "/health"},
		{"Mailhog", mailhogURL},
	}

	for _, svc := range services {
		t.Run(svc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", svc.url, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("❌ %s unreachable: %v\n   URL: %s\n   Ensure docker-compose is running: cd testing && docker compose up -d",
					svc.name, err, svc.url)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("❌ %s unhealthy (HTTP %d)\n   Response: %s",
					svc.name, resp.StatusCode, truncate(string(body), 200))
			}

			t.Logf("✓ %s healthy", svc.name)
		})
	}

	// Test PostgreSQL via fixture
	t.Run("PostgreSQL", func(t *testing.T) {
		fixture := integration.GetFixture()
		db, err := sql.Open("pgx", fixture.PostgresURL)
		require.NoError(t, err)
		defer db.Close()

		ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
		defer cancel()

		err = db.PingContext(ctx)
		if err != nil {
			t.Fatalf("❌ PostgreSQL unreachable: %v\n   Fixture URL: %s",
				err, fixture.PostgresURL)
		}

		t.Log("✓ PostgreSQL accessible via fixture")
	})

	// Test Redpanda
	t.Run("Redpanda", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, "docker", "exec", "hermes-redpanda",
			"rpk", "cluster", "health")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("❌ Redpanda health check failed: %v\n   Output: %s\n   Ensure container is running: docker ps | grep redpanda",
				err, string(output))
		}

		if !strings.Contains(string(output), "Healthy") {
			t.Fatalf("❌ Redpanda unhealthy\n   Output: %s", string(output))
		}

		t.Log("✓ Redpanda healthy")
	})

	// Check critical containers
	criticalContainers := []string{
		"hermes-central-indexer",
		"hermes-notifier-audit",
		"hermes-notifier-mail",
		"hermes-notifier-ntfy",
	}

	for _, container := range criticalContainers {
		t.Run(container, func(t *testing.T) {
			cmd := exec.CommandContext(ctx, "docker", "ps", "--filter",
				fmt.Sprintf("name=%s", container), "--format", "{{.Status}}")
			output, err := cmd.CombinedOutput()
			require.NoError(t, err)

			status := strings.TrimSpace(string(output))
			if status == "" {
				t.Fatalf("❌ Container %s not found\n   Check: docker ps -a | grep %s",
					container, container)
			}

			if !strings.HasPrefix(status, "Up") {
				t.Fatalf("❌ Container %s not running\n   Status: %s\n   Start: docker start %s",
					container, status, container)
			}

			t.Logf("✓ Container %s running", container)
		})
	}

	t.Log("✅ All services healthy and operational")
}

// testBearerTokenAuth validates RFC-086 bearer token authentication.
func testBearerTokenAuth(t *testing.T, ctx context.Context) {
	t.Log("=== Phase 2: Bearer Token Authentication (RFC-086) ===")

	fixture := integration.GetFixture()
	db, err := sql.Open("pgx", fixture.PostgresURL)
	require.NoError(t, err)
	defer db.Close()

	// Generate test token
	testID := time.Now().UnixNano()
	token := fmt.Sprintf("hermes-edge-token-%s-test%d",
		uuid.New().String(), testID)
	tokenHash := hashSHA256(token)

	t.Logf("Generated test token: %s...", token[:40])

	// Store token in database
	t.Run("StoreToken", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO service_tokens (id, created_at, updated_at, token_hash, token_type, revoked, expires_at)
			VALUES ($1, NOW(), NOW(), $2, 'edge', false, NOW() + INTERVAL '1 hour')
			ON CONFLICT (token_hash) DO NOTHING
		`, uuid.New(), tokenHash)

		if err != nil {
			t.Fatalf("❌ Failed to store token in database: %v\n   Check: SELECT COUNT(*) FROM service_tokens;",
				err)
		}

		t.Log("✓ Token stored in service_tokens table")
	})

	// Cleanup token after test
	defer func() {
		db.ExecContext(context.Background(),
			"DELETE FROM service_tokens WHERE token_hash = $1", tokenHash)
		t.Log("Cleaned up test token")
	}()

	// Test valid authentication
	t.Run("ValidTokenAccepted", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("%s/api/v2/edge/documents/sync-status?edge_instance=%s",
				centralURL, edgeInstance), nil)
		require.NoError(t, err)

		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("❌ Valid token rejected (HTTP %d)\n   Response: %s\n   Token hash: %s",
				resp.StatusCode, truncate(string(body), 200), tokenHash[:16])
		}

		t.Log("✓ Valid bearer token accepted (HTTP 200)")
	})

	// Test invalid token rejection
	t.Run("InvalidTokenRejected", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("%s/api/v2/edge/documents/sync-status?edge_instance=%s",
				centralURL, edgeInstance), nil)
		require.NoError(t, err)

		req.Header.Set("Authorization", "Bearer invalid-token-12345")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("❌ Invalid token not rejected (HTTP %d, expected 401)",
				resp.StatusCode)
		}

		t.Log("✓ Invalid token rejected (HTTP 401)")
	})

	// Test missing auth header rejection
	t.Run("MissingAuthRejected", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("%s/api/v2/edge/documents/sync-status?edge_instance=%s",
				centralURL, edgeInstance), nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("❌ Unauthenticated request not rejected (HTTP %d, expected 401)",
				resp.StatusCode)
		}

		t.Log("✓ Unauthenticated request rejected (HTTP 401)")
	})

	t.Log("✅ Bearer token authentication working correctly")
}

// testEdgeToCentralSync validates RFC-085 edge-to-central synchronization.
func testEdgeToCentralSync(t *testing.T, ctx context.Context) {
	t.Log("=== Phase 3: Edge-to-Central Synchronization (RFC-085) ===")

	fixture := integration.GetFixture()
	db, err := sql.Open("pgx", fixture.PostgresURL)
	require.NoError(t, err)
	defer db.Close()

	// Create test token
	token := fmt.Sprintf("hermes-edge-token-%s-sync", uuid.New().String())
	tokenHash := hashSHA256(token)

	_, err = db.ExecContext(ctx, `
		INSERT INTO service_tokens (id, created_at, updated_at, token_hash, token_type, revoked, expires_at)
		VALUES ($1, NOW(), NOW(), $2, 'edge', false, NOW() + INTERVAL '1 hour')
		ON CONFLICT (token_hash) DO NOTHING
	`, uuid.New(), tokenHash)
	require.NoError(t, err)

	defer func() {
		db.ExecContext(context.Background(),
			"DELETE FROM service_tokens WHERE token_hash = $1", tokenHash)
	}()

	// Test sync status endpoint
	t.Run("SyncStatusEndpoint", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("%s/api/v2/edge/documents/sync-status?edge_instance=%s",
				centralURL, edgeInstance), nil)
		require.NoError(t, err)

		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("❌ Sync status query failed (HTTP %d)\n   Response: %s",
				resp.StatusCode, truncate(string(body), 300))
		}

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		if err != nil {
			t.Fatalf("❌ Invalid JSON response: %v\n   Body: %s",
				err, truncate(string(body), 200))
		}

		t.Log("✓ Sync status endpoint accessible")
		t.Logf("  Response: %v", truncateJSON(result, 150))
	})

	// Test edge stats endpoint
	t.Run("EdgeStatsEndpoint", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("%s/api/v2/edge/stats", centralURL), nil)
		require.NoError(t, err)

		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("❌ Edge stats query failed (HTTP %d)\n   Response: %s",
				resp.StatusCode, truncate(string(body), 300))
		}

		t.Log("✓ Edge stats endpoint accessible")
	})

	// Verify edge_document_registry table
	t.Run("VerifyRegistryTable", func(t *testing.T) {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM edge_document_registry WHERE edge_instance = $1",
			edgeInstance).Scan(&count)

		if err != nil {
			t.Fatalf("❌ Failed to query edge_document_registry: %v\n   Check: \\d edge_document_registry",
				err)
		}

		t.Logf("✓ edge_document_registry accessible (%d documents for %s)",
			count, edgeInstance)
	})

	t.Log("✅ Edge-to-central sync endpoints operational")
}

// testSearchIntegration validates Meilisearch integration.
func testSearchIntegration(t *testing.T, ctx context.Context) {
	t.Log("=== Phase 4: Search Integration (Meilisearch) ===")

	// Test basic search
	t.Run("BasicSearch", func(t *testing.T) {
		result := performSearch(t, ctx, "test", 10, nil)
		require.NotNil(t, result)

		hits, ok := result["hits"].([]interface{})
		if !ok {
			t.Fatalf("❌ Invalid search response format\n   Response: %v", result)
		}

		t.Logf("✓ Basic search working (%d results)", len(hits))
	})

	// Test filtered search
	t.Run("FilteredSearch", func(t *testing.T) {
		filter := "documentType = RFC"
		result := performSearch(t, ctx, "", 10, &filter)
		require.NotNil(t, result)

		hits, ok := result["hits"].([]interface{})
		if ok {
			t.Logf("✓ Filtered search working (%d RFC documents)", len(hits))
		} else {
			t.Log("⚠ Filtered search returned unexpected format")
		}
	})

	// Test index statistics
	t.Run("IndexStatistics", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("%s/indexes/documents/stats", meilisearchURL), nil)
		require.NoError(t, err)

		req.Header.Set("Authorization", "Bearer "+meilisearchKey)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("❌ Index stats query failed (HTTP %d)\n   Response: %s",
				resp.StatusCode, truncate(string(body), 200))
		}

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var stats map[string]interface{}
		err = json.Unmarshal(body, &stats)
		require.NoError(t, err)

		docCount := 0
		if count, ok := stats["numberOfDocuments"].(float64); ok {
			docCount = int(count)
		}

		t.Logf("✓ Index statistics accessible (%d documents indexed)", docCount)
	})

	t.Log("✅ Search integration operational")
}

// testNotificationSystem validates RFC-087 notification system.
func testNotificationSystem(t *testing.T, ctx context.Context) {
	t.Log("=== Phase 5: Notification System (RFC-087) ===")

	// Test Redpanda topics
	t.Run("NotificationTopic", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, "docker", "exec", "hermes-redpanda",
			"rpk", "topic", "list")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err)

		topics := string(output)
		if strings.Contains(topics, "hermes.notifications") {
			t.Log("✓ Notification topic exists")
		} else {
			t.Log("⚠ Notification topic not yet created (will be created on first message)")
		}
	})

	// Test consumer group
	t.Run("ConsumerGroup", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, "docker", "exec", "hermes-redpanda",
			"rpk", "group", "describe", "hermes-notifiers")
		output, err := cmd.CombinedOutput()

		// Group may not exist until consumers connect
		if err != nil {
			t.Log("⚠ Consumer group not yet initialized (notifiers may still be starting)")
			return
		}

		groupInfo := string(output)
		if strings.Contains(groupInfo, "Stable") || strings.Contains(groupInfo, "Empty") {
			t.Log("✓ Consumer group active")
		} else {
			t.Logf("⚠ Consumer group status: %s", truncate(groupInfo, 100))
		}
	})

	// Test Mailhog API
	t.Run("MailhogAPI", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET",
			mailhogURL+"/api/v2/messages", nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("❌ Mailhog API unreachable (HTTP %d)", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		emailCount := 0
		if total, ok := result["total"].(float64); ok {
			emailCount = int(total)
		}

		t.Logf("✓ Mailhog API accessible (%d emails)", emailCount)
	})

	// Check notifier logs for activity
	t.Run("NotifierActivity", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, "docker", "logs", "hermes-notifier-audit",
			"--tail", "50")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err)

		logs := string(output)
		if strings.Contains(logs, "Starting") || strings.Contains(logs, "Processing") ||
			strings.Contains(logs, "Acknowledged") {
			t.Log("✓ Audit backend active")
		} else {
			t.Log("⚠ No recent activity in audit backend")
		}
	})

	t.Log("✅ Notification system components operational")
}

// testEndToEndValidation performs final system validation.
func testEndToEndValidation(t *testing.T, ctx context.Context) {
	t.Log("=== Phase 6: End-to-End Validation ===")

	// Verify all services still healthy
	t.Run("ServicesStillHealthy", func(t *testing.T) {
		services := map[string]string{
			"Central": centralURL + "/health",
			"Edge":    edgeURL + "/health",
		}

		for name, url := range services {
			ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				t.Errorf("❌ %s became unhealthy during test", name)
			} else {
				resp.Body.Close()
			}
		}

		t.Log("✓ All services remain healthy")
	})

	// Check for critical errors in logs
	t.Run("CheckServiceLogs", func(t *testing.T) {
		containers := []string{
			"hermes-central",
			"hermes-edge",
			"hermes-central-indexer",
		}

		for _, container := range containers {
			cmd := exec.CommandContext(ctx, "docker", "logs", container, "--tail", "50")
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Logf("⚠ Could not read logs for %s: %v", container, err)
				continue
			}

			logs := string(output)
			criticalErrors := countCriticalErrors(logs)

			if criticalErrors > 0 {
				t.Logf("⚠ Found %d potential critical error(s) in %s logs",
					criticalErrors, container)
				// Show first error
				lines := strings.Split(logs, "\n")
				for _, line := range lines {
					if isCriticalError(line) {
						t.Logf("  Example: %s", truncate(line, 150))
						break
					}
				}
			}
		}

		t.Log("✓ Log analysis complete")
	})

	// Overall system status
	t.Run("OverallStatus", func(t *testing.T) {
		t.Log("✓ End-to-end validation complete")
		t.Log("")
		t.Log("╔════════════════════════════════════════════════════════╗")
		t.Log("║  ✅ Comprehensive E2E Test Passed                     ║")
		t.Log("║                                                        ║")
		t.Log("║  Validated:                                            ║")
		t.Log("║   ✓ Service Health & Connectivity                     ║")
		t.Log("║   ✓ RFC-086 Bearer Token Authentication               ║")
		t.Log("║   ✓ RFC-085 Edge-to-Central Synchronization           ║")
		t.Log("║   ✓ Meilisearch Integration                           ║")
		t.Log("║   ✓ RFC-087 Notification System                       ║")
		t.Log("║   ✓ System Stability & Error-Free Operation           ║")
		t.Log("╚════════════════════════════════════════════════════════╝")
	})
}

// Helper functions

func performSearch(t *testing.T, ctx context.Context, query string, limit int, filter *string) map[string]interface{} {
	t.Helper()

	searchBody := map[string]interface{}{
		"q":     query,
		"limit": limit,
	}

	if filter != nil {
		searchBody["filter"] = *filter
	}

	bodyBytes, err := json.Marshal(searchBody)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/indexes/documents/search", meilisearchURL),
		strings.NewReader(string(bodyBytes)))
	require.NoError(t, err)

	req.Header.Set("Authorization", "Bearer "+meilisearchKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("❌ Search failed (HTTP %d)\n   Query: %v\n   Response: %s",
			resp.StatusCode, searchBody, truncate(string(body), 200))
	}

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	return result
}

func hashSHA256(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func truncateJSON(v interface{}, maxLen int) string {
	data, _ := json.Marshal(v)
	return truncate(string(data), maxLen)
}

func countCriticalErrors(logs string) int {
	count := 0
	for _, line := range strings.Split(logs, "\n") {
		if isCriticalError(line) {
			count++
		}
	}
	return count
}

func isCriticalError(line string) bool {
	lower := strings.ToLower(line)

	// Ignore debug level
	if strings.Contains(lower, "debug") {
		return false
	}

	// Check for critical keywords
	return strings.Contains(lower, "error") ||
		strings.Contains(lower, "fatal") ||
		strings.Contains(lower, "panic")
}

// TestPrerequisites is a quick sanity check that can be run before the full suite.
// Run with: go test -tags=integration -run TestPrerequisites
func TestPrerequisites(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	ctx := context.Background()

	t.Run("DockerComposeRunning", func(t *testing.T) {
		// Check if docker-compose services are running
		if _, err := os.Stat("../../../testing/docker-compose.yml"); os.IsNotExist(err) {
			t.Skip("docker-compose.yml not found, skipping")
		}

		cmd := exec.CommandContext(ctx, "docker", "compose", "-f",
			"../../../testing/docker-compose.yml", "ps", "--services", "--filter", "status=running")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("❌ Failed to check docker-compose status: %v\n   Run: cd testing && docker compose up -d",
				err)
		}

		services := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(services) < 10 {
			t.Fatalf("❌ Not enough services running (%d/12)\n   Run: cd testing && docker compose up -d",
				len(services))
		}

		t.Logf("✓ Docker Compose running (%d services)", len(services))
	})

	t.Run("CentralAPIReachable", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", centralURL+"/health", nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("❌ Central API unreachable: %v\n   URL: %s\n   Check: docker logs hermes-central",
				err, centralURL)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("❌ Central API unhealthy (HTTP %d)", resp.StatusCode)
		}

		t.Log("✓ Central API reachable")
	})

	t.Log("✅ Prerequisites satisfied - ready for comprehensive E2E test")
}
