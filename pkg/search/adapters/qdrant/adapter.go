// Package qdrant provides an opt-in local vector search adapter.
package qdrant

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/search"
)

// Config configures the Qdrant adapter.
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// Result is a vector search result placeholder for the future Qdrant-backed index.
type Result struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

// Adapter is a minimal Qdrant client used for local vector-search readiness checks.
type Adapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// Name returns the provider name for diagnostics.
func (a *Adapter) Name() string {
	return string(search.ProviderTypeQdrant)
}

// New creates an adapter.
func New(cfg Config) (*Adapter, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("qdrant base URL is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	return &Adapter{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		client:  &http.Client{Timeout: cfg.Timeout},
	}, nil
}

// Healthy verifies the Qdrant service endpoint is reachable.
func (a *Adapter) Healthy(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/healthz", http.NoBody)
	if err != nil {
		return err
	}
	if a.apiKey != "" {
		req.Header.Set("api-key", a.apiKey)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant health returned status %d", resp.StatusCode)
	}
	return nil
}

// Search is intentionally not implemented until vector indexing is wired.
func (a *Adapter) Search(_ context.Context, _ string, _ []float32, _ int) ([]Result, error) {
	return nil, fmt.Errorf("qdrant vector search is not implemented yet")
}
