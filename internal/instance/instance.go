package instance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

var (
	currentInstance   *models.HermesInstance
	currentInstanceMu sync.RWMutex
)

// Initialize sets up the Hermes instance identity.
// This should be called once at startup before any other database operations.
func Initialize(ctx context.Context, db *gorm.DB, cfg *config.Config, logger hclog.Logger) error {
	currentInstanceMu.Lock()
	defer currentInstanceMu.Unlock()

	// Check if instance already exists
	var instance models.HermesInstance
	err := db.First(&instance).Error

	if err == nil {
		// Instance exists - update if config changed
		needsUpdate := false

		if instance.InstanceID != cfg.BaseURL {
			logger.Warn("Instance ID changed in config",
				"old", instance.InstanceID,
				"new", cfg.BaseURL)
			instance.InstanceID = cfg.BaseURL
			needsUpdate = true
		}

		if instance.BaseURL != cfg.BaseURL {
			instance.BaseURL = cfg.BaseURL
			needsUpdate = true
		}

		// Determine environment (always development for now)
		deploymentEnv := "development"
		if instance.DeploymentEnv != deploymentEnv {
			instance.DeploymentEnv = deploymentEnv
			needsUpdate = true
		}

		if needsUpdate {
			if err := db.Save(&instance).Error; err != nil {
				return fmt.Errorf("failed to update instance: %w", err)
			}
			logger.Info("Instance configuration updated")
		}

		// Update heartbeat
		instance.LastHeartbeat = time.Now()
		if err := db.Model(&instance).Update("last_heartbeat", instance.LastHeartbeat).Error; err != nil {
			logger.Warn("Failed to update heartbeat", "error", err)
		}

		currentInstance = &instance
		logger.Info("Instance initialized",
			"instance_id", instance.InstanceID,
			"instance_uuid", instance.InstanceUUID,
			"environment", instance.DeploymentEnv)

		return nil
	}

	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to query instance: %w", err)
	}

	// No instance exists - create new
	instanceName := cfg.BaseURL
	if instanceName == "" {
		instanceName = "Hermes Instance"
	}

	deploymentEnv := "development"

	instance = models.HermesInstance{
		InstanceUUID:  uuid.New(),
		InstanceID:    cfg.BaseURL,
		InstanceName:  instanceName,
		BaseURL:       cfg.BaseURL,
		DeploymentEnv: deploymentEnv,
		InitializedAt: time.Now(),
		LastHeartbeat: time.Now(),
	}

	if err := db.Create(&instance).Error; err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	currentInstance = &instance

	logger.Info("Hermes instance initialized",
		"instance_id", instance.InstanceID,
		"instance_uuid", instance.InstanceUUID,
		"environment", instance.DeploymentEnv)

	return nil
}

// GetCurrentInstance returns the current instance (must call Initialize first)
func GetCurrentInstance() *models.HermesInstance {
	currentInstanceMu.RLock()
	defer currentInstanceMu.RUnlock()
	return currentInstance
}

// StartHeartbeat starts a background goroutine that updates last_heartbeat
func StartHeartbeat(ctx context.Context, db *gorm.DB, interval time.Duration, logger hclog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			instance := GetCurrentInstance()
			if instance == nil {
				continue
			}

			if err := instance.UpdateHeartbeat(db); err != nil {
				logger.Warn("Failed to update instance heartbeat", "error", err)
			}
		}
	}
}

// ResetForTesting resets the global instance state for testing purposes.
// This should only be called from test code.
func ResetForTesting() {
	currentInstanceMu.Lock()
	defer currentInstanceMu.Unlock()
	currentInstance = nil
}
