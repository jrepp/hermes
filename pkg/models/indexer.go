package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Indexer represents a registered indexer instance that syncs documents to central Hermes.
type Indexer struct {
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	LastHeartbeatAt *time.Time     `json:"last_heartbeat_at,omitempty"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	IndexerType     string         `gorm:"type:varchar(50);not null" json:"indexer_type"`
	WorkspacePath   string         `gorm:"type:varchar(1024)" json:"workspace_path,omitempty"`
	Hostname        string         `gorm:"type:varchar(255)" json:"hostname,omitempty"`
	Version         string         `gorm:"type:varchar(50)" json:"version,omitempty"`
	Status          string         `gorm:"type:varchar(50);default:'active'" json:"status"`
	Metadata        string         `gorm:"type:text" json:"metadata,omitempty"`
	Tokens          []IndexerToken `gorm:"foreignKey:IndexerID" json:"-"`
	DocumentCount   int            `gorm:"default:0" json:"document_count"`
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
}

// BeforeCreate hook to generate UUID if not set.
func (i *Indexer) BeforeCreate(_ *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (Indexer) TableName() string {
	return "indexers"
}

// Indexers is a slice of indexers.
type Indexers []Indexer

// Get retrieves an indexer by ID.
func (i *Indexer) Get(db *gorm.DB) error {
	return db.First(i, "id = ?", i.ID).Error
}

// Create creates a new indexer in the database.
func (i *Indexer) Create(db *gorm.DB) error {
	return db.Create(i).Error
}

// Update updates an existing indexer.
func (i *Indexer) Update(db *gorm.DB) error {
	return db.Save(i).Error
}

// Delete soft-deletes an indexer.
func (i *Indexer) Delete(db *gorm.DB) error {
	return db.Delete(i).Error
}

// UpdateHeartbeat updates the last heartbeat timestamp and document count.
func (i *Indexer) UpdateHeartbeat(db *gorm.DB, documentCount int) error {
	now := time.Now()
	i.LastHeartbeatAt = &now
	i.DocumentCount = documentCount
	return db.Model(i).Updates(map[string]interface{}{
		"last_heartbeat_at": now,
		"document_count":    documentCount,
	}).Error
}

// FindAll retrieves all indexers (including deleted if unscoped).
func (is *Indexers) FindAll(db *gorm.DB) error {
	return db.Find(is).Error
}

// FindActive retrieves all active indexers.
func (is *Indexers) FindActive(db *gorm.DB) error {
	return db.Where("status = ?", "active").Find(is).Error
}
