package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// DocumentSyncService handles document synchronization from edge to central
type DocumentSyncService struct {
	db *gorm.DB
}

// NewDocumentSyncService creates a new document sync service
func NewDocumentSyncService(db *gorm.DB) *DocumentSyncService {
	return &DocumentSyncService{
		db: db,
	}
}

// EdgeDocumentRecord represents a document registered from an edge instance
type EdgeDocumentRecord struct {
	UUID           docid.UUID     `json:"uuid"`
	Title          string         `json:"title"`
	DocumentType   string         `json:"document_type"`
	Status         string         `json:"status"`
	Summary        string         `json:"summary"`
	Owners         []string       `json:"owners"`
	Contributors   []string       `json:"contributors"`
	EdgeInstance   string         `json:"edge_instance"`
	EdgeProviderID string         `json:"edge_provider_id"`
	Product        string         `json:"product"`
	Tags           []string       `json:"tags"`
	ParentFolders  []string       `json:"parent_folders"`
	Metadata       map[string]any `json:"metadata"`
	ContentHash    string         `json:"content_hash"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	SyncedAt       time.Time      `json:"synced_at"`
	LastSyncStatus string         `json:"last_sync_status"`
	SyncError      string         `json:"sync_error,omitempty"`
}

// RegisterDocument registers a document from an edge instance
func (s *DocumentSyncService) RegisterDocument(ctx context.Context, doc *workspace.DocumentMetadata, edgeInstance string) (*EdgeDocumentRecord, error) {
	// Get underlying sql.DB from gorm
	sqlDB, err := s.db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(doc.ExtendedMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Extract owners from Contributors
	var owners []string
	if doc.Owner != nil {
		owners = append(owners, doc.Owner.Email)
	}

	var contributors []string
	for _, c := range doc.Contributors {
		contributors = append(contributors, c.Email)
	}

	now := time.Now()

	// Insert or update document record
	query := `
		INSERT INTO edge_document_registry (
			uuid, title, document_type, status, summary,
			owners, contributors, edge_instance, edge_provider_id,
			product, tags, parent_folders, metadata, content_hash,
			created_at, updated_at, synced_at, last_sync_status
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13, $14,
			$15, $16, $17, $18
		)
		ON CONFLICT (uuid) DO UPDATE SET
			title = EXCLUDED.title,
			status = EXCLUDED.status,
			summary = EXCLUDED.summary,
			owners = EXCLUDED.owners,
			contributors = EXCLUDED.contributors,
			edge_provider_id = EXCLUDED.edge_provider_id,
			product = EXCLUDED.product,
			tags = EXCLUDED.tags,
			parent_folders = EXCLUDED.parent_folders,
			metadata = EXCLUDED.metadata,
			content_hash = EXCLUDED.content_hash,
			updated_at = EXCLUDED.updated_at,
			synced_at = EXCLUDED.synced_at,
			last_sync_status = EXCLUDED.last_sync_status
		RETURNING *
	`

	var record EdgeDocumentRecord
	var metadataBytes []byte

	err = sqlDB.QueryRowContext(ctx, query,
		doc.UUID, doc.Name, doc.ProviderType, doc.WorkflowStatus, "", // summary empty for now
		owners, contributors, edgeInstance, doc.ProviderID,
		doc.Project, doc.Tags, doc.Parents, metadataJSON, doc.ContentHash,
		doc.CreatedTime, doc.ModifiedTime, now, "synced",
	).Scan(
		&record.UUID, &record.Title, &record.DocumentType, &record.Status, &record.Summary,
		&record.Owners, &record.Contributors, &record.EdgeInstance, &record.EdgeProviderID,
		&record.Product, &record.Tags, &record.ParentFolders, &metadataBytes, &record.ContentHash,
		&record.CreatedAt, &record.UpdatedAt, &record.SyncedAt, &record.LastSyncStatus, &record.SyncError,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register document: %w", err)
	}

	// Unmarshal metadata
	if err := json.Unmarshal(metadataBytes, &record.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &record, nil
}

// UpdateDocumentMetadata updates document metadata from edge
func (s *DocumentSyncService) UpdateDocumentMetadata(ctx context.Context, uuid docid.UUID, updates map[string]any) (*EdgeDocumentRecord, error) {
	// Get underlying sql.DB from gorm
	sqlDB, err := s.db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Build dynamic UPDATE query based on provided fields
	now := time.Now()

	query := `
		UPDATE edge_document_registry
		SET
			title = COALESCE($2, title),
			status = COALESCE($3, status),
			summary = COALESCE($4, summary),
			product = COALESCE($5, product),
			content_hash = COALESCE($6, content_hash),
			updated_at = $7,
			synced_at = $8,
			last_sync_status = 'synced'
		WHERE uuid = $1
		RETURNING *
	`

	// Extract update fields (with nil for unchanged)
	var title, status, summary, product, contentHash *string
	if v, ok := updates["title"].(string); ok {
		title = &v
	}
	if v, ok := updates["status"].(string); ok {
		status = &v
	}
	if v, ok := updates["summary"].(string); ok {
		summary = &v
	}
	if v, ok := updates["product"].(string); ok {
		product = &v
	}
	if v, ok := updates["content_hash"].(string); ok {
		contentHash = &v
	}

	var record EdgeDocumentRecord
	var metadataBytes []byte

	err = sqlDB.QueryRowContext(ctx, query,
		uuid, title, status, summary, product, contentHash,
		now, now,
	).Scan(
		&record.UUID, &record.Title, &record.DocumentType, &record.Status, &record.Summary,
		&record.Owners, &record.Contributors, &record.EdgeInstance, &record.EdgeProviderID,
		&record.Product, &record.Tags, &record.ParentFolders, &metadataBytes, &record.ContentHash,
		&record.CreatedAt, &record.UpdatedAt, &record.SyncedAt, &record.LastSyncStatus, &record.SyncError,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update document metadata: %w", err)
	}

	// Unmarshal metadata
	if err := json.Unmarshal(metadataBytes, &record.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &record, nil
}

// GetSyncStatus gets synchronization status for documents from an edge instance
func (s *DocumentSyncService) GetSyncStatus(ctx context.Context, edgeInstance string, limit int) ([]*EdgeDocumentRecord, error) {
	// Get underlying sql.DB from gorm
	sqlDB, err := s.db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT
			uuid, title, document_type, status, summary,
			owners, contributors, edge_instance, edge_provider_id,
			product, tags, parent_folders, metadata, content_hash,
			created_at, updated_at, synced_at, last_sync_status, sync_error
		FROM edge_document_registry
		WHERE edge_instance = $1
		ORDER BY synced_at DESC
		LIMIT $2
	`

	rows, err := sqlDB.QueryContext(ctx, query, edgeInstance, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query sync status: %w", err)
	}
	defer rows.Close()

	var records []*EdgeDocumentRecord
	for rows.Next() {
		var record EdgeDocumentRecord
		var metadataBytes []byte

		err := rows.Scan(
			&record.UUID, &record.Title, &record.DocumentType, &record.Status, &record.Summary,
			&record.Owners, &record.Contributors, &record.EdgeInstance, &record.EdgeProviderID,
			&record.Product, &record.Tags, &record.ParentFolders, &metadataBytes, &record.ContentHash,
			&record.CreatedAt, &record.UpdatedAt, &record.SyncedAt, &record.LastSyncStatus, &record.SyncError,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}

		// Unmarshal metadata
		if err := json.Unmarshal(metadataBytes, &record.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		records = append(records, &record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return records, nil
}

// GetDocumentByUUID retrieves a synced document by UUID
func (s *DocumentSyncService) GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*EdgeDocumentRecord, error) {
	// Get underlying sql.DB from gorm
	sqlDB, err := s.db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	query := `
		SELECT
			uuid, title, document_type, status, summary,
			owners, contributors, edge_instance, edge_provider_id,
			product, tags, parent_folders, metadata, content_hash,
			created_at, updated_at, synced_at, last_sync_status, sync_error
		FROM edge_document_registry
		WHERE uuid = $1
	`

	var record EdgeDocumentRecord
	var metadataBytes []byte

	err = sqlDB.QueryRowContext(ctx, query, uuid).Scan(
		&record.UUID, &record.Title, &record.DocumentType, &record.Status, &record.Summary,
		&record.Owners, &record.Contributors, &record.EdgeInstance, &record.EdgeProviderID,
		&record.Product, &record.Tags, &record.ParentFolders, &metadataBytes, &record.ContentHash,
		&record.CreatedAt, &record.UpdatedAt, &record.SyncedAt, &record.LastSyncStatus, &record.SyncError,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("document not found: %s", uuid)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Unmarshal metadata
	if err := json.Unmarshal(metadataBytes, &record.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &record, nil
}

// SearchDocuments searches edge documents by various criteria
func (s *DocumentSyncService) SearchDocuments(ctx context.Context, query string, filters map[string]any, limit int) ([]*EdgeDocumentRecord, error) {
	// Get underlying sql.DB from gorm
	sqlDB, err := s.db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	if limit <= 0 {
		limit = 50
	}

	// Build search query
	sqlQuery := `
		SELECT
			uuid, title, document_type, status, summary,
			owners, contributors, edge_instance, edge_provider_id,
			product, tags, parent_folders, metadata, content_hash,
			created_at, updated_at, synced_at, last_sync_status, sync_error
		FROM edge_document_registry
		WHERE 1=1
	`

	args := []any{}
	argCount := 1

	// Add search filters
	if query != "" {
		sqlQuery += fmt.Sprintf(" AND (title ILIKE $%d OR summary ILIKE $%d)", argCount, argCount)
		args = append(args, "%"+query+"%")
		argCount++
	}

	if docType, ok := filters["document_type"].(string); ok && docType != "" {
		sqlQuery += fmt.Sprintf(" AND document_type = $%d", argCount)
		args = append(args, docType)
		argCount++
	}

	if status, ok := filters["status"].(string); ok && status != "" {
		sqlQuery += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	if product, ok := filters["product"].(string); ok && product != "" {
		sqlQuery += fmt.Sprintf(" AND product = $%d", argCount)
		args = append(args, product)
		argCount++
	}

	if edgeInstance, ok := filters["edge_instance"].(string); ok && edgeInstance != "" {
		sqlQuery += fmt.Sprintf(" AND edge_instance = $%d", argCount)
		args = append(args, edgeInstance)
		argCount++
	}

	sqlQuery += fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d", argCount)
	args = append(args, limit)

	rows, err := sqlDB.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}
	defer rows.Close()

	var records []*EdgeDocumentRecord
	for rows.Next() {
		var record EdgeDocumentRecord
		var metadataBytes []byte

		err := rows.Scan(
			&record.UUID, &record.Title, &record.DocumentType, &record.Status, &record.Summary,
			&record.Owners, &record.Contributors, &record.EdgeInstance, &record.EdgeProviderID,
			&record.Product, &record.Tags, &record.ParentFolders, &metadataBytes, &record.ContentHash,
			&record.CreatedAt, &record.UpdatedAt, &record.SyncedAt, &record.LastSyncStatus, &record.SyncError,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}

		// Unmarshal metadata
		if err := json.Unmarshal(metadataBytes, &record.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		records = append(records, &record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return records, nil
}

// DeleteDocument marks a document as deleted (soft delete)
func (s *DocumentSyncService) DeleteDocument(ctx context.Context, uuid docid.UUID) error {
	// Get underlying sql.DB from gorm
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}

	query := `
		DELETE FROM edge_document_registry
		WHERE uuid = $1
	`

	result, err := sqlDB.ExecContext(ctx, query, uuid)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("document not found: %s", uuid)
	}

	return nil
}

// GetEdgeInstanceStats returns statistics for an edge instance
func (s *DocumentSyncService) GetEdgeInstanceStats(ctx context.Context, edgeInstance string) (map[string]any, error) {
	// Get underlying sql.DB from gorm
	sqlDB, err := s.db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	query := `
		SELECT
			COUNT(*) as total_documents,
			COUNT(*) FILTER (WHERE last_sync_status = 'synced') as synced_documents,
			COUNT(*) FILTER (WHERE last_sync_status = 'pending') as pending_documents,
			COUNT(*) FILTER (WHERE last_sync_status = 'failed') as failed_documents,
			MAX(synced_at) as last_sync_time
		FROM edge_document_registry
		WHERE edge_instance = $1
	`

	var total, synced, pending, failed int
	var lastSyncTime sql.NullTime

	err = sqlDB.QueryRowContext(ctx, query, edgeInstance).Scan(
		&total, &synced, &pending, &failed, &lastSyncTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	stats := map[string]any{
		"edge_instance":     edgeInstance,
		"total_documents":   total,
		"synced_documents":  synced,
		"pending_documents": pending,
		"failed_documents":  failed,
		"last_sync_time":    nil,
	}

	if lastSyncTime.Valid {
		stats["last_sync_time"] = lastSyncTime.Time
	}

	return stats, nil
}
