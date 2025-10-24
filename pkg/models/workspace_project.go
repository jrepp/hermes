package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WorkspaceProject represents a workspace project configuration stored in the database.
// This model serves as the source of truth for project configurations at runtime,
// initially populated from HCL config files on startup, but can be updated dynamically
// including from remote Hermes instances.
type WorkspaceProject struct {
	gorm.Model

	// Instance relationship - links this project to a specific Hermes instance
	InstanceUUID *uuid.UUID `gorm:"type:uuid;index:idx_workspace_projects_instance" json:"instanceUuid,omitempty"`

	// ProjectUUID is an auto-generated globally unique identifier for this project
	// Nullable initially for backward compatibility with existing workspace_projects
	ProjectUUID *uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_workspace_projects_uuid" json:"projectUuid,omitempty"`

	// GlobalProjectID is a computed composite identifier (instance_uuid/name)
	// This is generated automatically by the BeforeCreate hook
	GlobalProjectID *string `gorm:"type:varchar(512);index:idx_workspace_projects_global_id" json:"globalProjectId,omitempty"`

	// ConfigHash is a SHA-256 hash of the project configuration (detects drift)
	ConfigHash *string `gorm:"type:varchar(64);index:idx_workspace_projects_config_hash" json:"configHash,omitempty"`

	// Name is the unique identifier for the project within this instance (e.g., "example-project")
	// Combined with InstanceUUID, this forms a globally unique composite key
	Name string `gorm:"index:idx_workspace_projects_instance_name;not null" json:"name"`

	// Title is the display title for the project
	Title string `gorm:"not null"`

	// FriendlyName is a human-readable name for the project
	FriendlyName string `gorm:"not null"`

	// ShortName is an abbreviated name for the project
	ShortName string `gorm:"not null"`

	// Description is an optional description of the project
	Description *string

	// Status is the project status (active, archived, completed)
	Status string `gorm:"not null;default:'active'"`

	// ProvidersJSON stores the serialized provider configurations
	// This is a JSON blob containing provider type, configuration, and state
	ProvidersJSON string `gorm:"type:jsonb"`

	// MetadataJSON stores additional metadata as JSON
	// This includes created_at, owner, tags, notes, etc.
	MetadataJSON string `gorm:"type:jsonb"`

	// SourceType indicates where this project config came from
	// Values: "hcl_file", "remote_hermes", "api"
	SourceType string `gorm:"not null;default:'hcl_file'"`

	// SourceIdentifier is the source location (file path, remote URL, etc.)
	SourceIdentifier *string

	// LastSyncedAt tracks when this project was last synced from its source
	LastSyncedAt *time.Time

	// ConfigVersion tracks the version of the configuration schema
	ConfigVersion string `gorm:"not null;default:'1.0'"`
}

// WorkspaceProjectStatus constants
const (
	WorkspaceProjectStatusActive    = "active"
	WorkspaceProjectStatusArchived  = "archived"
	WorkspaceProjectStatusCompleted = "completed"
)

// WorkspaceProjectSourceType constants
const (
	WorkspaceProjectSourceHCLFile      = "hcl_file"
	WorkspaceProjectSourceRemoteHermes = "remote_hermes"
	WorkspaceProjectSourceAPI          = "api"
)

// BeforeCreate hook to generate project UUID and calculate config hash
func (wp *WorkspaceProject) BeforeCreate(tx *gorm.DB) error {
	// Generate ProjectUUID if not set
	if wp.ProjectUUID == nil {
		newUUID := uuid.New()
		wp.ProjectUUID = &newUUID
	}

	// Calculate config hash
	if wp.ConfigHash == nil {
		hash := wp.CalculateConfigHash()
		wp.ConfigHash = &hash
	}

	// Generate global project ID if instance is set
	if wp.InstanceUUID != nil && wp.ProjectUUID != nil {
		globalID := fmt.Sprintf("%s/%s", wp.InstanceUUID.String(), wp.Name)
		wp.GlobalProjectID = &globalID
	}

	return nil
}

// CalculateConfigHash computes SHA-256 hash of the project configuration
func (wp *WorkspaceProject) CalculateConfigHash() string {
	// Concatenate relevant config fields for hashing
	configData := fmt.Sprintf("%s|%s|%s|%s", wp.ProvidersJSON, wp.MetadataJSON, wp.Status, wp.SourceType)
	hash := sha256.Sum256([]byte(configData))
	return hex.EncodeToString(hash[:])
}

// Create creates a new workspace project in the database.
func (wp *WorkspaceProject) Create(db *gorm.DB) error {
	// Validate required fields
	if err := validation.ValidateStruct(wp,
		validation.Field(&wp.Name, validation.Required),
		validation.Field(&wp.Title, validation.Required),
		validation.Field(&wp.FriendlyName, validation.Required),
		validation.Field(&wp.ShortName, validation.Required),
		validation.Field(&wp.Status, validation.Required),
		validation.Field(&wp.SourceType, validation.Required),
	); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	return db.
		Omit(clause.Associations).
		Create(&wp).
		Error
}

// Get retrieves a workspace project by ID.
func (wp *WorkspaceProject) Get(db *gorm.DB, id uint) error {
	if err := validation.Validate(id, validation.Required); err != nil {
		return err
	}

	return db.
		Preload(clause.Associations).
		First(&wp, id).
		Error
}

// GetByName retrieves a workspace project by name.
func (wp *WorkspaceProject) GetByName(db *gorm.DB, name string) error {
	if err := validation.Validate(name, validation.Required); err != nil {
		return err
	}

	return db.
		Where("name = ?", name).
		First(&wp).
		Error
}

// GetByInstanceAndName retrieves a workspace project by instance UUID and name (composite key).
func GetWorkspaceProjectByInstanceAndName(db *gorm.DB, instanceUUID uuid.UUID, name string) (*WorkspaceProject, error) {
	var project WorkspaceProject
	err := db.Where("instance_uuid = ? AND name = ?", instanceUUID, name).
		First(&project).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// GetByProjectUUID retrieves a workspace project by its project UUID.
func GetWorkspaceProjectByUUID(db *gorm.DB, projectUUID uuid.UUID) (*WorkspaceProject, error) {
	var project WorkspaceProject
	err := db.Where("project_uuid = ?", projectUUID).
		First(&project).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// GetByGlobalProjectID retrieves a workspace project by global project ID.
func GetWorkspaceProjectByGlobalID(db *gorm.DB, globalProjectID string) (*WorkspaceProject, error) {
	var project WorkspaceProject
	err := db.Where("global_project_id = ?", globalProjectID).
		First(&project).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// Update updates a workspace project.
func (wp *WorkspaceProject) Update(db *gorm.DB) error {
	if err := validation.ValidateStruct(wp,
		validation.Field(&wp.ID, validation.Required),
	); err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Model(&wp).
			Select("*").
			Updates(wp).
			Error; err != nil {
			return err
		}

		if err := wp.Get(tx, wp.ID); err != nil {
			return fmt.Errorf("error getting workspace project after update: %w", err)
		}

		return nil
	})
}

// Upsert creates or updates a workspace project by name.
func (wp *WorkspaceProject) Upsert(db *gorm.DB) error {
	// Validate required fields
	if err := validation.ValidateStruct(wp,
		validation.Field(&wp.Name, validation.Required),
		validation.Field(&wp.Title, validation.Required),
		validation.Field(&wp.FriendlyName, validation.Required),
		validation.Field(&wp.ShortName, validation.Required),
		validation.Field(&wp.Status, validation.Required),
		validation.Field(&wp.SourceType, validation.Required),
	); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	// Try to find existing project
	existing := &WorkspaceProject{}
	err := existing.GetByName(db, wp.Name)
	if err == nil {
		// Update existing
		wp.ID = existing.ID
		wp.CreatedAt = existing.CreatedAt
		return wp.Update(db)
	} else if err == gorm.ErrRecordNotFound {
		// Create new
		return wp.Create(db)
	}

	return fmt.Errorf("error checking for existing project: %w", err)
}

// GetAllActive retrieves all active workspace projects.
func GetAllActiveWorkspaceProjects(db *gorm.DB) ([]WorkspaceProject, error) {
	var projects []WorkspaceProject
	err := db.
		Where("status = ?", WorkspaceProjectStatusActive).
		Order("name ASC").
		Find(&projects).
		Error
	return projects, err
}

// GetAllWorkspaceProjects retrieves all workspace projects.
func GetAllWorkspaceProjects(db *gorm.DB) ([]WorkspaceProject, error) {
	var projects []WorkspaceProject
	err := db.
		Order("name ASC").
		Find(&projects).
		Error
	return projects, err
}

// ProvidersData is a helper struct for serializing/deserializing provider configurations.
type ProvidersData struct {
	Providers []ProviderData `json:"providers"`
}

// ProviderData represents a single provider configuration.
type ProviderData struct {
	Type            string                 `json:"type"`
	MigrationStatus string                 `json:"migration_status,omitempty"`
	Config          map[string]interface{} `json:"config"`
}

// GetProviders deserializes the ProvidersJSON field.
func (wp *WorkspaceProject) GetProviders() (*ProvidersData, error) {
	if wp.ProvidersJSON == "" {
		return &ProvidersData{Providers: []ProviderData{}}, nil
	}

	var data ProvidersData
	if err := json.Unmarshal([]byte(wp.ProvidersJSON), &data); err != nil {
		return nil, fmt.Errorf("error unmarshaling providers JSON: %w", err)
	}
	return &data, nil
}

// SetProviders serializes provider data to ProvidersJSON field.
func (wp *WorkspaceProject) SetProviders(data *ProvidersData) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling providers to JSON: %w", err)
	}
	wp.ProvidersJSON = string(jsonBytes)
	return nil
}

// MetadataData is a helper struct for serializing/deserializing metadata.
type MetadataData struct {
	CreatedAt *time.Time `json:"created_at,omitempty"`
	Owner     string     `json:"owner,omitempty"`
	Tags      []string   `json:"tags,omitempty"`
	Notes     string     `json:"notes,omitempty"`
}

// GetMetadata deserializes the MetadataJSON field.
func (wp *WorkspaceProject) GetMetadata() (*MetadataData, error) {
	if wp.MetadataJSON == "" {
		return &MetadataData{}, nil
	}

	var data MetadataData
	if err := json.Unmarshal([]byte(wp.MetadataJSON), &data); err != nil {
		return nil, fmt.Errorf("error unmarshaling metadata JSON: %w", err)
	}
	return &data, nil
}

// SetMetadata serializes metadata to MetadataJSON field.
func (wp *WorkspaceProject) SetMetadata(data *MetadataData) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling metadata to JSON: %w", err)
	}
	wp.MetadataJSON = string(jsonBytes)
	return nil
}
