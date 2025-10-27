package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// HermesInstance represents this Hermes deployment's identity.
// This is a singleton table - only one instance per database.
type HermesInstance struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// Instance identifiers
	InstanceUUID uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"instanceUuid"`
	InstanceID   string    `gorm:"uniqueIndex;not null;size:255" json:"instanceId"`

	// Instance metadata
	InstanceName  string `gorm:"not null;size:255" json:"instanceName"`
	BaseURL       string `gorm:"size:255" json:"baseUrl,omitempty"`
	DeploymentEnv string `gorm:"not null;default:development;size:50" json:"deploymentEnv"`

	// Tracking
	InitializedAt time.Time `gorm:"not null" json:"initializedAt"`
	LastHeartbeat time.Time `gorm:"not null" json:"lastHeartbeat"`

	// Metadata
	Metadata JSON `gorm:"type:jsonb" json:"metadata,omitempty"`

	// Timestamps
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
}

// TableName returns the table name for GORM
func (HermesInstance) TableName() string {
	return "hermes_instances"
}

// BeforeCreate hook to generate instance UUID if not set
func (h *HermesInstance) BeforeCreate(tx *gorm.DB) error {
	if h.InstanceUUID == uuid.Nil {
		h.InstanceUUID = uuid.New()
	}
	if h.InitializedAt.IsZero() {
		h.InitializedAt = time.Now()
	}
	if h.LastHeartbeat.IsZero() {
		h.LastHeartbeat = time.Now()
	}
	return nil
}

// GetInstance retrieves the singleton instance record.
// Returns nil if no instance exists (should be initialized at startup).
func GetInstance(db *gorm.DB) (*HermesInstance, error) {
	var instance HermesInstance

	// Try to get existing instance
	if err := db.First(&instance).Error; err != nil {
		return nil, err
	}

	return &instance, nil
}

// UpdateHeartbeat updates the last_heartbeat timestamp
func (h *HermesInstance) UpdateHeartbeat(db *gorm.DB) error {
	return db.Model(h).Update("last_heartbeat", time.Now()).Error
}
