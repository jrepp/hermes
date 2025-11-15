package migration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
	s3adapter "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/s3"
	"github.com/hashicorp/go-hclog"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrationE2E tests the complete migration flow
// Requires: PostgreSQL, MinIO
func TestMigrationE2E(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run")
	}

	ctx := context.Background()

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", "host=localhost port=5433 user=postgres password=postgres dbname=hermes_testing sslmode=disable")
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()

	// Verify database connection
	err = db.Ping()
	require.NoError(t, err, "Failed to ping database")

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "migration-test",
		Level: hclog.Debug,
	})

	// Create mock source provider (simplified)
	sourceProvider := &mockProvider{
		documents: make(map[string]*workspace.DocumentMetadata),
		content:   make(map[string]*workspace.DocumentContent),
		logger:    logger.Named("mock-source"),
	}

	// Create S3 destination provider
	s3Config := &s3adapter.Config{
		Endpoint:          "http://localhost:9000",
		Region:            "us-east-1",
		Bucket:            "hermes-documents",
		Prefix:            "migration-test",
		AccessKey:         "minioadmin",
		SecretKey:         "minioadmin",
		VersioningEnabled: true,
		MetadataStore:     "manifest",
		UseSSL:            false,
	}
	destProvider, err := s3adapter.NewAdapter(s3Config, logger.Named("s3-dest"))
	require.NoError(t, err, "Failed to create S3 adapter")

	// Setup provider registry in database
	t.Run("SetupProviders", func(t *testing.T) {
		// Clean up any existing test providers
		_, _ = db.Exec("DELETE FROM migration_outbox WHERE migration_job_id IN (SELECT id FROM migration_jobs WHERE job_name LIKE 'test-%')")
		_, _ = db.Exec("DELETE FROM migration_items WHERE migration_job_id IN (SELECT id FROM migration_jobs WHERE job_name LIKE 'test-%')")
		_, _ = db.Exec("DELETE FROM migration_jobs WHERE job_name LIKE 'test-%'")
		_, _ = db.Exec("DELETE FROM provider_storage WHERE provider_name IN ('test-mock-source', 'test-s3-dest')")

		// Insert source provider
		_, err = db.Exec(`
			INSERT INTO provider_storage (provider_name, provider_type, config, status, is_primary, is_writable)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, "test-mock-source", "mock", "{}", "active", true, true)
		require.NoError(t, err, "Failed to insert source provider")

		// Insert dest provider
		_, err = db.Exec(`
			INSERT INTO provider_storage (provider_name, provider_type, config, status, is_primary, is_writable)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, "test-s3-dest", "s3", "{}", "active", false, true)
		require.NoError(t, err, "Failed to insert dest provider")
	})

	// Create test documents in source
	var testDocs []struct {
		uuid       docid.UUID
		providerID string
		name       string
		content    string
	}

	t.Run("CreateSourceDocuments", func(t *testing.T) {
		for i := 1; i <= 5; i++ {
			uuid := docid.NewUUID()
			providerID := uuid.String() // Mock uses UUID as provider ID
			name := fmt.Sprintf("Test Migration Document %d", i)
			content := fmt.Sprintf("# Document %d\n\nThis is test document %d for migration.", i, i)

			// Add to mock provider
			sourceProvider.documents[providerID] = &workspace.DocumentMetadata{
				UUID:         uuid,
				ProviderType: "mock",
				ProviderID:   providerID,
				Name:         name,
				MimeType:     "text/markdown",
				CreatedTime:  time.Now(),
				ModifiedTime: time.Now(),
				SyncStatus:   "canonical",
				ContentHash:  fmt.Sprintf("mock-hash-%d", i),
			}
			sourceProvider.content[providerID] = &workspace.DocumentContent{
				UUID:        uuid,
				ProviderID:  providerID,
				Title:       name,
				Body:        content,
				Format:      "markdown",
				ContentHash: fmt.Sprintf("mock-hash-%d", i),
			}

			testDocs = append(testDocs, struct {
				uuid       docid.UUID
				providerID string
				name       string
				content    string
			}{uuid, providerID, name, content})
		}

		t.Logf("Created %d test documents in source", len(testDocs))
	})

	// Create migration manager
	manager := NewManager(db, logger)

	var jobID int64

	t.Run("CreateMigrationJob", func(t *testing.T) {
		req := &CreateJobRequest{
			JobName:        "test-migration-e2e",
			SourceProvider: "test-mock-source",
			DestProvider:   "test-s3-dest",
			Strategy:       StrategyCopy,
			Concurrency:    2,
			BatchSize:      10,
			DryRun:         false,
			Validate:       true,
			CreatedBy:      "test-user",
		}

		job, err := manager.CreateJob(ctx, req)
		require.NoError(t, err, "Failed to create migration job")
		require.NotNil(t, job)
		assert.Equal(t, JobStatusPending, job.Status)
		jobID = job.ID

		t.Logf("Created migration job: ID=%d, UUID=%s", job.ID, job.JobUUID)
	})

	t.Run("QueueDocuments", func(t *testing.T) {
		var uuids []docid.UUID
		var providerIDs []string
		for _, doc := range testDocs {
			uuids = append(uuids, doc.uuid)
			providerIDs = append(providerIDs, doc.providerID)
		}

		err := manager.QueueDocuments(ctx, jobID, uuids, providerIDs)
		require.NoError(t, err, "Failed to queue documents")

		// Verify job was updated
		job, err := manager.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, len(testDocs), job.TotalDocuments)

		t.Logf("Queued %d documents for migration", len(testDocs))
	})

	t.Run("StartJob", func(t *testing.T) {
		err := manager.StartJob(ctx, jobID)
		require.NoError(t, err, "Failed to start job")

		job, err := manager.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusRunning, job.Status)
	})

	t.Run("ProcessMigration", func(t *testing.T) {
		// Create worker with provider map
		providerMap := map[string]workspace.WorkspaceProvider{
			"test-mock-source": sourceProvider,
			"test-s3-dest":     destProvider,
		}

		workerCfg := &WorkerConfig{
			PollInterval:   1 * time.Second,
			MaxConcurrency: 2,
		}
		worker := NewWorker(db, providerMap, logger, workerCfg)

		// Process tasks (run worker for a short time)
		workerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Process tasks manually instead of starting worker loop
		for i := 0; i < 5; i++ {
			err := worker.processPendingTasks(workerCtx)
			if err != nil {
				t.Logf("Process iteration %d: %v", i, err)
			}
			time.Sleep(500 * time.Millisecond)

			// Check progress
			progress, err := manager.GetProgress(ctx, jobID)
			if err == nil {
				t.Logf("Progress: %d/%d (%.1f%%), Failed: %d",
					progress.Migrated, progress.Total, progress.Percent, progress.Failed)

				if progress.Migrated+progress.Failed >= progress.Total {
					t.Log("All documents processed")
					break
				}
			}
		}

		// Check final progress
		progress, err := manager.GetProgress(ctx, jobID)
		require.NoError(t, err)

		t.Logf("Final progress: Migrated=%d, Failed=%d, Total=%d",
			progress.Migrated, progress.Failed, progress.Total)

		assert.Greater(t, progress.Migrated, 0, "Should have migrated at least one document")
		assert.Equal(t, progress.Total, progress.Migrated+progress.Failed,
			"All documents should be processed")
	})

	t.Run("VerifyMigratedDocuments", func(t *testing.T) {
		// Verify documents exist in S3
		for _, doc := range testDocs {
			// Try to get by UUID from S3
			migratedDoc, err := destProvider.GetDocumentByUUID(ctx, doc.uuid)
			if err != nil {
				t.Logf("Document %s not found in S3: %v", doc.uuid, err)
				continue
			}

			assert.Equal(t, doc.uuid, migratedDoc.UUID)
			t.Logf("Verified document %s migrated to S3: %s", doc.uuid, migratedDoc.ProviderID)

			// Verify content
			content, err := destProvider.GetContent(ctx, migratedDoc.ProviderID)
			if err == nil {
				assert.Contains(t, content.Body, fmt.Sprintf("# Document"))
				t.Logf("Content verified for %s", doc.uuid)
			}
		}
	})
}

// mockProvider is a simple mock provider for testing
type mockProvider struct {
	documents map[string]*workspace.DocumentMetadata
	content   map[string]*workspace.DocumentContent
	logger    hclog.Logger
}

func (m *mockProvider) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
	doc, ok := m.documents[providerID]
	if !ok {
		return nil, fmt.Errorf("document not found")
	}
	return doc, nil
}

func (m *mockProvider) GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	for _, doc := range m.documents {
		if doc.UUID == uuid {
			return doc, nil
		}
	}
	return nil, fmt.Errorf("document not found")
}

func (m *mockProvider) GetContent(ctx context.Context, providerID string) (*workspace.DocumentContent, error) {
	content, ok := m.content[providerID]
	if !ok {
		return nil, fmt.Errorf("content not found")
	}
	return content, nil
}

func (m *mockProvider) DeleteDocument(ctx context.Context, providerID string) error {
	delete(m.documents, providerID)
	delete(m.content, providerID)
	return nil
}

// Stub implementations for required interfaces
func (m *mockProvider) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) CreateDocumentWithUUID(ctx context.Context, uuid docid.UUID, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) RegisterDocument(ctx context.Context, doc *workspace.DocumentMetadata) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) CopyDocument(ctx context.Context, srcProviderID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) MoveDocument(ctx context.Context, providerID, destFolderID string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) RenameDocument(ctx context.Context, providerID, newName string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) CreateFolder(ctx context.Context, name, parentID string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetSubfolder(ctx context.Context, parentID, name string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockProvider) GetContentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) UpdateContent(ctx context.Context, providerID string, content string) (*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetContentBatch(ctx context.Context, providerIDs []string) ([]*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) CompareContent(ctx context.Context, providerID1, providerID2 string) (*workspace.ContentComparison, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetRevisionHistory(ctx context.Context, providerID string, limit int) ([]*workspace.BackendRevision, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetRevision(ctx context.Context, providerID, revisionID string) (*workspace.BackendRevision, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetRevisionContent(ctx context.Context, providerID, revisionID string) (*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) KeepRevisionForever(ctx context.Context, providerID, revisionID string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) GetAllDocumentRevisions(ctx context.Context, uuid docid.UUID) ([]*workspace.RevisionInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) ShareDocument(ctx context.Context, providerID, email, role string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) ShareDocumentWithDomain(ctx context.Context, providerID, domain, role string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) ListPermissions(ctx context.Context, providerID string) ([]*workspace.FilePermission, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) RemovePermission(ctx context.Context, providerID, permissionID string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) UpdatePermission(ctx context.Context, providerID, permissionID, newRole string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) SearchPeople(ctx context.Context, query string) ([]*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetPerson(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetPersonByUnifiedID(ctx context.Context, unifiedID string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) ResolveIdentity(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) ListTeams(ctx context.Context, domain, query string, maxResults int64) ([]*workspace.Team, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetTeam(ctx context.Context, teamID string) (*workspace.Team, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetUserTeams(ctx context.Context, userEmail string) ([]*workspace.Team, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetTeamMembers(ctx context.Context, teamID string) ([]*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error {
	return fmt.Errorf("not implemented")
}
