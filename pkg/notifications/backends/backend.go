package backends

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
)

// Backend defines the interface for notification backends
type Backend interface {
	// Name returns the backend identifier
	Name() string

	// Handle processes a notification message
	Handle(ctx context.Context, msg *notifications.NotificationMessage) error

	// SupportsBackend checks if this backend should process the message
	SupportsBackend(backend string) bool
}

// BackendError represents an error from a specific backend
// RFC-087-ADDENDUM Section 9: Backend Error Handling
type BackendError struct {
	Backend   string // Backend name (e.g., "mail", "slack")
	Operation string // Operation that failed (e.g., "send", "connect")
	Retryable bool   // Whether the error is retryable
	Err       error  // Underlying error
}

func (e *BackendError) Error() string {
	retryability := "permanent"
	if e.Retryable {
		retryability = "retryable"
	}
	return fmt.Sprintf("%s backend error (%s, %s): %v", e.Backend, e.Operation, retryability, e.Err)
}

func (e *BackendError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error is retryable
func (e *BackendError) IsRetryable() bool {
	return e.Retryable
}

// MultiBackendError represents errors from multiple backends
type MultiBackendError struct {
	Errors []*BackendError
}

func (e *MultiBackendError) Error() string {
	if len(e.Errors) == 0 {
		return "no backend errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("multiple backend errors: %s", strings.Join(msgs, "; "))
}

// HasRetryableErrors returns true if any of the errors are retryable
func (e *MultiBackendError) HasRetryableErrors() bool {
	for _, err := range e.Errors {
		if err.Retryable {
			return true
		}
	}
	return false
}

// AllRetryable returns true if all errors are retryable
func (e *MultiBackendError) AllRetryable() bool {
	if len(e.Errors) == 0 {
		return false
	}
	for _, err := range e.Errors {
		if !err.Retryable {
			return false
		}
	}
	return true
}

// NewBackendError creates a new backend error
func NewBackendError(backend, operation string, retryable bool, err error) *BackendError {
	return &BackendError{
		Backend:   backend,
		Operation: operation,
		Retryable: retryable,
		Err:       err,
	}
}
