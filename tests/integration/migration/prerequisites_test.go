//go:build integration
// +build integration

package migration

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// PrerequisiteChecker validates all required services and configurations.
type PrerequisiteChecker struct {
	t                *testing.T
	postgresURL      string
	minioEndpoint    string
	minioBucket      string
	requiredTables   []string
	requiredServices []ServiceCheck
}

// ServiceCheck represents a service health check.
type ServiceCheck struct {
	Name     string
	URL      string
	Timeout  time.Duration
	Required bool
}

// NewPrerequisiteChecker creates a new prerequisite checker.
func NewPrerequisiteChecker(t *testing.T) *PrerequisiteChecker {
	return &PrerequisiteChecker{
		t:             t,
		postgresURL:   "postgres://postgres:postgres@localhost:5433/hermes_testing?sslmode=disable",
		minioEndpoint: "http://localhost:9000",
		minioBucket:   "hermes-documents",
		requiredTables: []string{
			"provider_storage",
			"migration_jobs",
			"migration_items",
			"migration_outbox",
		},
		requiredServices: []ServiceCheck{
			{Name: "PostgreSQL", URL: "localhost:5433", Timeout: 5 * time.Second, Required: true},
			{Name: "MinIO", URL: "http://localhost:9000/minio/health/live", Timeout: 5 * time.Second, Required: true},
		},
	}
}

// CheckAll runs all prerequisite checks and fails fast if any required check fails.
func (pc *PrerequisiteChecker) CheckAll(ctx context.Context) {
	pc.t.Helper()
	pc.t.Log("=== Checking Prerequisites ===")

	// Check 1: Docker is running
	pc.checkDockerRunning()

	// Check 2: Required containers are running
	pc.checkContainersRunning()

	// Check 3: Service connectivity
	pc.checkServiceConnectivity(ctx)

	// Check 4: Database connection
	db := pc.checkDatabaseConnection(ctx)
	defer db.Close()

	// Check 5: Migration tables exist
	pc.checkMigrationTables(ctx, db)

	// Check 6: MinIO bucket exists
	pc.checkMinioBucket(ctx)

	pc.t.Log("✅ All prerequisites met")
}

// checkDockerRunning verifies Docker daemon is accessible.
func (pc *PrerequisiteChecker) checkDockerRunning() {
	pc.t.Helper()
	pc.t.Log("Checking Docker daemon...")

	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		pc.t.Fatal("❌ Docker is not running or not accessible\n" +
			"   Please start Docker Desktop and try again\n" +
			"   Error: " + err.Error())
	}

	pc.t.Log("✓ Docker is running")
}

// checkContainersRunning verifies required containers are running.
func (pc *PrerequisiteChecker) checkContainersRunning() {
	pc.t.Helper()
	pc.t.Log("Checking required containers...")

	requiredContainers := map[string]string{
		"postgres": "hermes.*postgres",
		"minio":    "hermes.*minio",
	}

	for name, pattern := range requiredContainers {
		cmd := exec.Command("docker", "ps", "--filter", fmt.Sprintf("name=%s", pattern), "--format", "{{.Names}}\t{{.Status}}")
		output, err := cmd.Output()
		if err != nil {
			pc.t.Fatalf("❌ Failed to check %s container: %v\n"+
				"   Run: cd testing && docker compose up -d %s", name, err, name)
		}

		if len(output) == 0 {
			pc.t.Fatalf("❌ %s container is not running\n"+
				"   Run: cd testing && docker compose up -d %s", capitalize(name), name)
		}

		// Check if container is healthy
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) > 0 {
			status := lines[0]
			if !strings.Contains(status, "Up") {
				pc.t.Fatalf("❌ %s container is not running: %s\n"+
					"   Run: cd testing && docker compose up -d %s", capitalize(name), status, name)
			}
			pc.t.Logf("✓ %s container is running", capitalize(name))
		}
	}
}

// checkServiceConnectivity verifies services are accessible.
func (pc *PrerequisiteChecker) checkServiceConnectivity(ctx context.Context) {
	pc.t.Helper()
	pc.t.Log("Checking service connectivity...")

	for _, svc := range pc.requiredServices {
		if strings.Contains(svc.URL, "http") {
			// HTTP health check
			pc.checkHTTPService(ctx, svc)
		} else {
			// TCP port check
			pc.checkTCPService(svc)
		}
	}
}

// checkHTTPService checks if an HTTP service is healthy.
func (pc *PrerequisiteChecker) checkHTTPService(ctx context.Context, svc ServiceCheck) {
	pc.t.Helper()

	ctx, cancel := context.WithTimeout(ctx, svc.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", svc.URL, nil)
	if err != nil {
		if svc.Required {
			pc.t.Fatalf("❌ Failed to create request for %s: %v", svc.Name, err)
		}
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if svc.Required {
			pc.t.Fatalf("❌ %s is not accessible at %s\n"+
				"   Error: %v\n"+
				"   Run: cd testing && docker compose up -d", svc.Name, svc.URL, err)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if svc.Required {
			pc.t.Fatalf("❌ %s health check failed (HTTP %d)\n"+
				"   Run: docker compose logs %s", svc.Name, resp.StatusCode, strings.ToLower(svc.Name))
		}
		return
	}

	pc.t.Logf("✓ %s is accessible", svc.Name)
}

// checkTCPService checks if a TCP port is open.
func (pc *PrerequisiteChecker) checkTCPService(svc ServiceCheck) {
	pc.t.Helper()

	// Use netcat to check if port is open
	host := strings.Split(svc.URL, ":")[0]
	port := strings.Split(svc.URL, ":")[1]

	cmd := exec.Command("nc", "-z", host, port)
	err := cmd.Run()

	if err != nil {
		if svc.Required {
			pc.t.Fatalf("❌ %s is not accessible at %s\n"+
				"   Run: cd testing && docker compose up -d", svc.Name, svc.URL)
		}
		return
	}

	pc.t.Logf("✓ %s is accessible", svc.Name)
}

// checkDatabaseConnection verifies database is accessible and returns connection.
func (pc *PrerequisiteChecker) checkDatabaseConnection(ctx context.Context) *sql.DB {
	pc.t.Helper()
	pc.t.Log("Checking database connection...")

	db, err := sql.Open("pgx", pc.postgresURL)
	if err != nil {
		pc.t.Fatalf("❌ Failed to open database connection: %v\n"+
			"   URL: %s\n"+
			"   Run: cd testing && docker compose up -d postgres", err, pc.postgresURL)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		pc.t.Fatalf("❌ Failed to ping database: %v\n"+
			"   Run: docker compose logs postgres", err)
	}

	pc.t.Log("✓ Database connection successful")
	return db
}

// checkMigrationTables verifies RFC-089 migration tables exist.
func (pc *PrerequisiteChecker) checkMigrationTables(ctx context.Context, db *sql.DB) {
	pc.t.Helper()
	pc.t.Log("Checking migration tables...")

	for _, tableName := range pc.requiredTables {
		var exists bool
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public'
				AND table_name = $1
			)
		`, tableName).Scan(&exists)

		if err != nil {
			pc.t.Fatalf("❌ Failed to check table %s: %v", tableName, err)
		}

		if !exists {
			pc.t.Fatalf("❌ Required table '%s' does not exist\n"+
				"   Migration 000011_add_s3_migration_tables is not applied\n"+
				"   Run: make db-migrate", tableName)
		}

		pc.t.Logf("✓ Table %s exists", tableName)
	}

	// Check migration version
	var version int
	err := db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0)
		FROM schema_migrations
	`).Scan(&version)

	if err != nil {
		// Table might not exist in old schemas
		pc.t.Log("⚠️  Cannot verify schema_migrations table")
	} else if version < 11 {
		pc.t.Fatalf("❌ Database migrations are outdated (version %d, need >= 11)\n"+
			"   Run: make db-migrate", version)
	} else {
		pc.t.Logf("✓ Database migrations up to date (version %d)", version)
	}
}

// checkMinioBucket verifies MinIO bucket exists and versioning is enabled.
func (pc *PrerequisiteChecker) checkMinioBucket(ctx context.Context) {
	pc.t.Helper()
	pc.t.Log("Checking MinIO bucket...")

	// Check if bucket exists using Docker exec
	cmd := exec.Command("docker", "exec", "hermes-minio", "mc", "ls", fmt.Sprintf("myminio/%s", pc.minioBucket))
	_, err := cmd.CombinedOutput()

	if err != nil {
		pc.t.Logf("⚠️  Bucket '%s' does not exist, creating...", pc.minioBucket)

		// Create bucket
		cmd = exec.Command("docker", "exec", "hermes-minio", "mc", "mb", fmt.Sprintf("myminio/%s", pc.minioBucket), "--ignore-existing")
		if err := cmd.Run(); err != nil {
			pc.t.Fatalf("❌ Failed to create bucket: %v\n"+
				"   Run: cd testing && docker compose up -d minio-setup", err)
		}

		// Enable versioning
		cmd = exec.Command("docker", "exec", "hermes-minio", "mc", "version", "enable", fmt.Sprintf("myminio/%s", pc.minioBucket))
		if err := cmd.Run(); err != nil {
			pc.t.Fatalf("❌ Failed to enable versioning: %v", err)
		}

		pc.t.Logf("✓ Bucket '%s' created with versioning enabled", pc.minioBucket)
	} else {
		pc.t.Logf("✓ Bucket '%s' exists", pc.minioBucket)
	}

	// Verify versioning is enabled
	cmd = exec.Command("docker", "exec", "hermes-minio", "mc", "version", "info", fmt.Sprintf("myminio/%s", pc.minioBucket))
	output, err := cmd.CombinedOutput()

	if err != nil {
		pc.t.Logf("⚠️  Cannot verify versioning status: %v", err)
	} else if strings.Contains(string(output), "enabled") || strings.Contains(string(output), "Enabled") {
		pc.t.Log("✓ Bucket versioning is enabled")
	} else {
		// Try to enable it
		cmd = exec.Command("docker", "exec", "hermes-minio", "mc", "version", "enable", fmt.Sprintf("myminio/%s", pc.minioBucket))
		if err := cmd.Run(); err != nil {
			pc.t.Logf("⚠️  Failed to enable versioning: %v", err)
		} else {
			pc.t.Log("✓ Bucket versioning enabled")
		}
	}
}

// StartRequiredServices attempts to start required services if they're not running.
func (pc *PrerequisiteChecker) StartRequiredServices() {
	pc.t.Helper()
	pc.t.Log("Attempting to start required services...")

	cmd := exec.Command("docker", "compose", "up", "-d", "postgres", "minio")
	cmd.Dir = "../../../testing" // Relative to tests/integration/migration

	if output, err := cmd.CombinedOutput(); err != nil {
		pc.t.Logf("⚠️  Failed to start services: %v\n%s", err, string(output))
	} else {
		pc.t.Log("✓ Services starting... waiting for health checks")
		time.Sleep(5 * time.Second)
	}
}

// PrintServiceInfo prints helpful information about service endpoints.
func (pc *PrerequisiteChecker) PrintServiceInfo() {
	pc.t.Helper()

	pc.t.Log("=== Service Information ===")
	pc.t.Logf("PostgreSQL:    %s", pc.postgresURL)
	pc.t.Logf("MinIO S3 API:  %s", pc.minioEndpoint)
	pc.t.Logf("MinIO Console: http://localhost:9001 (minioadmin/minioadmin)")
	pc.t.Logf("Test Bucket:   %s", pc.minioBucket)
	pc.t.Log("===========================")
}

// capitalize returns the string with the first letter capitalized.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
