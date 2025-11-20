// Package router provides multi-provider routing and failover for document storage.
// Implements RFC-089 Phase 2: Multi-Provider Router
package router

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ProviderConfig represents a configured storage provider
type ProviderConfig struct {
	ID              int64  // Database ID
	Name            string // Provider name (e.g., "local-primary", "s3-archive")
	Type            string // Provider type (e.g., "local", "s3", "google")
	IsPrimary       bool   // Is this the primary provider?
	IsWritable      bool   // Can this provider accept writes?
	Status          string // "active", "readonly", "disabled", "migrating"
	HealthStatus    string // "healthy", "degraded", "unhealthy"
	LastHealthCheck *time.Time
}

// WriteStrategy determines how writes are handled across providers
type WriteStrategy string

const (
	WriteStrategyPrimaryOnly WriteStrategy = "primary_only" // Write to primary only
	WriteStrategyAllWritable WriteStrategy = "all_writable" // Write to all writable providers
	WriteStrategyMirror      WriteStrategy = "mirror"       // Mirror writes to specific providers
)

// ReadStrategy determines how reads are handled
type ReadStrategy string

const (
	ReadStrategyPrimaryOnly         ReadStrategy = "primary_only"     // Read from primary only
	ReadStrategyPrimaryThenFallback ReadStrategy = "primary_fallback" // Try primary, fallback to others
	ReadStrategyLoadBalance         ReadStrategy = "load_balance"     // Load balance across healthy providers
)

// RouterConfig configures the multi-provider router
type RouterConfig struct {
	WriteStrategy       WriteStrategy
	ReadStrategy        ReadStrategy
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration
}

// Router manages multiple storage providers and routes requests appropriately
type Router struct {
	db        *sql.DB
	providers map[string]workspace.WorkspaceProvider // name -> provider
	configs   map[string]*ProviderConfig             // name -> config
	mu        sync.RWMutex
	logger    hclog.Logger
	config    *RouterConfig

	// Health check
	healthTicker *time.Ticker
	healthDone   chan struct{}
}

// NewRouter creates a new multi-provider router
func NewRouter(db *sql.DB, logger hclog.Logger, cfg *RouterConfig) *Router {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}
	if cfg == nil {
		cfg = &RouterConfig{
			WriteStrategy:       WriteStrategyPrimaryOnly,
			ReadStrategy:        ReadStrategyPrimaryOnly,
			HealthCheckInterval: 30 * time.Second,
			HealthCheckTimeout:  5 * time.Second,
		}
	}

	return &Router{
		db:         db,
		providers:  make(map[string]workspace.WorkspaceProvider),
		configs:    make(map[string]*ProviderConfig),
		logger:     logger.Named("provider-router"),
		config:     cfg,
		healthDone: make(chan struct{}),
	}
}

// RegisterProvider registers a provider with the router
func (r *Router) RegisterProvider(name string, provider workspace.WorkspaceProvider, config *ProviderConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	r.providers[name] = provider
	r.configs[name] = config

	r.logger.Info("provider registered",
		"name", name,
		"type", config.Type,
		"is_primary", config.IsPrimary,
		"is_writable", config.IsWritable,
		"status", config.Status)

	return nil
}

// UnregisterProvider removes a provider from the router
func (r *Router) UnregisterProvider(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("provider %s not registered", name)
	}

	delete(r.providers, name)
	delete(r.configs, name)

	r.logger.Info("provider unregistered", "name", name)
	return nil
}

// GetProviders returns a copy of the provider map for use by other services (e.g., migration worker)
func (r *Router) GetProviders() map[string]workspace.WorkspaceProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent external modification
	providersCopy := make(map[string]workspace.WorkspaceProvider, len(r.providers))
	for name, provider := range r.providers {
		providersCopy[name] = provider
	}
	return providersCopy
}

// GetProvider retrieves a provider by name
func (r *Router) GetProvider(name string) (workspace.WorkspaceProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

// GetPrimaryProvider returns the primary provider
func (r *Router) GetPrimaryProvider() (workspace.WorkspaceProvider, *ProviderConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, config := range r.configs {
		if config.IsPrimary && config.Status == "active" {
			return r.providers[name], config, nil
		}
	}

	return nil, nil, fmt.Errorf("no active primary provider found")
}

// GetWritableProviders returns all writable providers
func (r *Router) GetWritableProviders() []workspace.WorkspaceProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var writable []workspace.WorkspaceProvider
	for name, config := range r.configs {
		if config.IsWritable && config.Status == "active" {
			writable = append(writable, r.providers[name])
		}
	}

	return writable
}

// GetHealthyProviders returns all healthy providers
func (r *Router) GetHealthyProviders() []workspace.WorkspaceProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var healthy []workspace.WorkspaceProvider
	for name, config := range r.configs {
		if config.Status == "active" && config.HealthStatus == "healthy" {
			healthy = append(healthy, r.providers[name])
		}
	}

	return healthy
}

// RouteRead routes a read operation to the appropriate provider(s)
func (r *Router) RouteRead(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	switch r.config.ReadStrategy {
	case ReadStrategyPrimaryOnly:
		return r.readFromPrimary(ctx, uuid)

	case ReadStrategyPrimaryThenFallback:
		return r.readWithFallback(ctx, uuid)

	case ReadStrategyLoadBalance:
		return r.readLoadBalanced(ctx, uuid)

	default:
		return r.readFromPrimary(ctx, uuid)
	}
}

// readFromPrimary reads from the primary provider only
func (r *Router) readFromPrimary(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	primary, config, err := r.GetPrimaryProvider()
	if err != nil {
		return nil, fmt.Errorf("no primary provider: %w", err)
	}

	r.logger.Debug("reading from primary", "provider", config.Name, "uuid", uuid)

	doc, err := primary.GetDocumentByUUID(ctx, uuid)
	if err != nil {
		r.logger.Error("primary read failed",
			"provider", config.Name,
			"uuid", uuid,
			"error", err)
		return nil, fmt.Errorf("primary read failed: %w", err)
	}

	return doc, nil
}

// readWithFallback tries primary first, then falls back to other providers
func (r *Router) readWithFallback(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	// Try primary first
	primary, config, err := r.GetPrimaryProvider()
	if err == nil {
		doc, err := primary.GetDocumentByUUID(ctx, uuid)
		if err == nil {
			r.logger.Debug("read from primary succeeded", "provider", config.Name, "uuid", uuid)
			return doc, nil
		}
		r.logger.Warn("primary read failed, trying fallback",
			"provider", config.Name,
			"uuid", uuid,
			"error", err)
	}

	// Try other healthy providers
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, providerConfig := range r.configs {
		if providerConfig.IsPrimary || providerConfig.Status != "active" {
			continue
		}

		provider := r.providers[name]
		doc, err := provider.GetDocumentByUUID(ctx, uuid)
		if err == nil {
			r.logger.Info("fallback read succeeded",
				"provider", name,
				"uuid", uuid)
			return doc, nil
		}

		r.logger.Debug("fallback provider failed",
			"provider", name,
			"uuid", uuid,
			"error", err)
	}

	return nil, fmt.Errorf("document not found in any provider: %s", uuid)
}

// readLoadBalanced distributes reads across healthy providers (simplified round-robin)
func (r *Router) readLoadBalanced(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	healthy := r.GetHealthyProviders()
	if len(healthy) == 0 {
		return nil, fmt.Errorf("no healthy providers available")
	}

	// For now, just try all healthy providers (in future, implement true load balancing)
	for _, provider := range healthy {
		doc, err := provider.GetDocumentByUUID(ctx, uuid)
		if err == nil {
			return doc, nil
		}
	}

	return nil, fmt.Errorf("document not found in any healthy provider: %s", uuid)
}

// RouteWrite routes a write operation to the appropriate provider(s)
func (r *Router) RouteWrite(ctx context.Context, operation func(workspace.WorkspaceProvider) error) error {
	switch r.config.WriteStrategy {
	case WriteStrategyPrimaryOnly:
		return r.writeToPrimary(ctx, operation)

	case WriteStrategyAllWritable:
		return r.writeToAll(ctx, operation)

	case WriteStrategyMirror:
		return r.writeWithMirror(ctx, operation)

	default:
		return r.writeToPrimary(ctx, operation)
	}
}

// writeToPrimary writes to primary provider only
func (r *Router) writeToPrimary(ctx context.Context, operation func(workspace.WorkspaceProvider) error) error {
	primary, config, err := r.GetPrimaryProvider()
	if err != nil {
		return fmt.Errorf("no primary provider: %w", err)
	}

	if !config.IsWritable {
		return fmt.Errorf("primary provider %s is not writable", config.Name)
	}

	r.logger.Debug("writing to primary", "provider", config.Name)

	if err := operation(primary); err != nil {
		r.logger.Error("primary write failed",
			"provider", config.Name,
			"error", err)
		return fmt.Errorf("primary write failed: %w", err)
	}

	return nil
}

// writeToAll writes to all writable providers
func (r *Router) writeToAll(ctx context.Context, operation func(workspace.WorkspaceProvider) error) error {
	writable := r.GetWritableProviders()
	if len(writable) == 0 {
		return fmt.Errorf("no writable providers available")
	}

	r.logger.Debug("writing to all writable providers", "count", len(writable))

	// Execute in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(writable))

	for _, provider := range writable {
		wg.Add(1)
		go func(p workspace.WorkspaceProvider) {
			defer wg.Done()
			if err := operation(p); err != nil {
				errChan <- err
			}
		}(provider)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("write failed on %d providers: %v", len(errs), errs)
	}

	return nil
}

// writeWithMirror writes to primary and mirrors to configured providers
func (r *Router) writeWithMirror(ctx context.Context, operation func(workspace.WorkspaceProvider) error) error {
	// For now, implement as write to all writable (can be extended later)
	return r.writeToAll(ctx, operation)
}

// StartHealthChecks starts periodic health checks for all providers
func (r *Router) StartHealthChecks(ctx context.Context) {
	r.logger.Info("starting provider health checks",
		"interval", r.config.HealthCheckInterval)

	r.healthTicker = time.NewTicker(r.config.HealthCheckInterval)

	// Run initial health check
	r.performHealthChecks(ctx)

	go func() {
		for {
			select {
			case <-ctx.Done():
				r.logger.Info("stopping health checks")
				return
			case <-r.healthDone:
				return
			case <-r.healthTicker.C:
				r.performHealthChecks(ctx)
			}
		}
	}()
}

// StopHealthChecks stops health check routine
func (r *Router) StopHealthChecks() {
	if r.healthTicker != nil {
		r.healthTicker.Stop()
	}
	close(r.healthDone)
}

// performHealthChecks checks health of all providers
func (r *Router) performHealthChecks(ctx context.Context) {
	r.mu.RLock()
	providerNames := make([]string, 0, len(r.providers))
	for name := range r.providers {
		providerNames = append(providerNames, name)
	}
	r.mu.RUnlock()

	for _, name := range providerNames {
		go r.checkProviderHealth(ctx, name)
	}
}

// checkProviderHealth checks health of a single provider
func (r *Router) checkProviderHealth(ctx context.Context, name string) {
	r.mu.RLock()
	provider, providerExists := r.providers[name]
	config, configExists := r.configs[name]
	r.mu.RUnlock()

	if !providerExists || !configExists {
		return
	}

	// Create timeout context
	checkCtx, cancel := context.WithTimeout(ctx, r.config.HealthCheckTimeout)
	defer cancel()

	// Perform health check (try to create a test UUID and check if provider responds)
	testUUID := docid.NewUUID()
	_, err := provider.GetDocumentByUUID(checkCtx, testUUID)

	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	config.LastHealthCheck = &now

	if err == nil || err.Error() == "resource not found: document with id \""+testUUID.String()+"\"" {
		// Provider is healthy (either found or properly returned "not found")
		if config.HealthStatus != "healthy" {
			r.logger.Info("provider health recovered", "provider", name)
			config.HealthStatus = "healthy"
		}
	} else {
		// Provider is unhealthy
		if config.HealthStatus == "healthy" {
			r.logger.Warn("provider health degraded",
				"provider", name,
				"error", err)
		}
		config.HealthStatus = "unhealthy"
	}

	// Update database
	_, _ = r.db.Exec(`
		UPDATE provider_storage
		SET health_status = $1, last_health_check = $2, updated_at = NOW()
		WHERE provider_name = $3
	`, config.HealthStatus, now, name)
}

// ListProviders returns all registered providers with their configs
func (r *Router) ListProviders() map[string]*ProviderConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]*ProviderConfig, len(r.configs))
	for name, config := range r.configs {
		configCopy := *config
		result[name] = &configCopy
	}

	return result
}
