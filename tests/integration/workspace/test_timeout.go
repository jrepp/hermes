//go:build integration
// +build integration

// Package workspace provides integration tests for workspace operations.
package workspace

import (
	"context"
	"testing"
	"time"
)

// WithTimeout wraps a test with timeout protection.
func WithTimeout(t *testing.T, timeout, _ time.Duration, testFunc func(ctx context.Context, progress func(string))) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	progress := func(message string) {
		if message != "" {
			t.Logf("Progress: %s", message)
		}
	}

	testFunc(ctx, progress)
}
