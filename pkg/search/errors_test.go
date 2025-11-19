package search

import (
	"errors"
	"testing"
)

func TestError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected string
	}{
		{
			name: "error with message",
			err: &Error{
				Op:  "Index",
				Err: ErrIndexingFailed,
				Msg: "document validation failed",
			},
			expected: "Index: document validation failed: failed to index document",
		},
		{
			name: "error without message",
			err: &Error{
				Op:  "Search",
				Err: ErrBackendUnavailable,
			},
			expected: "Search: search backend unavailable",
		},
		{
			name: "error with empty operation",
			err: &Error{
				Op:  "",
				Err: ErrNotFound,
			},
			expected: ": document not found in search index",
		},
		{
			name: "error with nested error",
			err: &Error{
				Op:  "Delete",
				Err: errors.New("connection timeout"),
				Msg: "failed to connect to Algolia",
			},
			expected: "Delete: failed to connect to Algolia: connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected error
	}{
		{
			name: "unwrap sentinel error",
			err: &Error{
				Op:  "Index",
				Err: ErrIndexingFailed,
			},
			expected: ErrIndexingFailed,
		},
		{
			name: "unwrap custom error",
			err: &Error{
				Op:  "Search",
				Err: errors.New("custom error"),
			},
			expected: errors.New("custom error"),
		},
		{
			name: "unwrap nested search error",
			err: &Error{
				Op: "Batch",
				Err: &Error{
					Op:  "Index",
					Err: ErrBackendUnavailable,
				},
			},
			expected: &Error{
				Op:  "Index",
				Err: ErrBackendUnavailable,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Unwrap()
			if got == nil {
				t.Fatal("Unwrap() returned nil")
			}

			// For custom errors, compare error messages
			if tt.expected.Error() != got.Error() {
				t.Errorf("Unwrap() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestError_Is(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{
			name: "wrapped ErrNotFound matches",
			err: &Error{
				Op:  "GetObject",
				Err: ErrNotFound,
			},
			target: ErrNotFound,
			want:   true,
		},
		{
			name: "wrapped ErrBackendUnavailable matches",
			err: &Error{
				Op:  "Search",
				Err: ErrBackendUnavailable,
			},
			target: ErrBackendUnavailable,
			want:   true,
		},
		{
			name: "wrapped ErrInvalidQuery matches",
			err: &Error{
				Op:  "Search",
				Err: ErrInvalidQuery,
			},
			target: ErrInvalidQuery,
			want:   true,
		},
		{
			name: "wrapped ErrIndexingFailed matches",
			err: &Error{
				Op:  "Index",
				Err: ErrIndexingFailed,
			},
			target: ErrIndexingFailed,
			want:   true,
		},
		{
			name: "double wrapped error matches",
			err: &Error{
				Op: "BatchIndex",
				Err: &Error{
					Op:  "Index",
					Err: ErrIndexingFailed,
				},
			},
			target: ErrIndexingFailed,
			want:   true,
		},
		{
			name: "different error does not match",
			err: &Error{
				Op:  "Index",
				Err: ErrIndexingFailed,
			},
			target: ErrNotFound,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errors.Is(tt.err, tt.target)
			if got != tt.want {
				t.Errorf("errors.Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrNotFound",
			err:      ErrNotFound,
			expected: "document not found in search index",
		},
		{
			name:     "ErrInvalidQuery",
			err:      ErrInvalidQuery,
			expected: "invalid search query",
		},
		{
			name:     "ErrBackendUnavailable",
			err:      ErrBackendUnavailable,
			expected: "search backend unavailable",
		},
		{
			name:     "ErrIndexingFailed",
			err:      ErrIndexingFailed,
			expected: "failed to index document",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestError_ErrorChaining(t *testing.T) {
	// Test complex error chaining scenario
	originalErr := errors.New("network timeout")

	wrappedOnce := &Error{
		Op:  "Connect",
		Err: originalErr,
		Msg: "failed to connect to backend",
	}

	wrappedTwice := &Error{
		Op:  "Initialize",
		Err: wrappedOnce,
		Msg: "initialization failed",
	}

	// Verify the full error message includes all context
	expected := "Initialize: initialization failed: Connect: failed to connect to backend: network timeout"
	got := wrappedTwice.Error()
	if got != expected {
		t.Errorf("chained error message = %q, want %q", got, expected)
	}

	// Verify we can unwrap to the original error
	var current error = wrappedTwice
	depth := 0
	maxDepth := 10 // Safety limit

	for current != nil && depth < maxDepth {
		if current.Error() == originalErr.Error() {
			return // Found original error
		}
		current = errors.Unwrap(current)
		depth++
	}

	t.Error("failed to unwrap to original error")
}

func TestError_NilError(t *testing.T) {
	// Test behavior with nil underlying error
	err := &Error{
		Op:  "Test",
		Err: nil,
		Msg: "test message",
	}

	// This will panic in the Error() method, which is expected behavior
	// We're just documenting this edge case
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when Err is nil")
		}
	}()

	_ = err.Error()
}

func TestError_EmptyFields(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
	}{
		{
			name: "empty Op and Msg",
			err: &Error{
				Op:  "",
				Err: ErrNotFound,
				Msg: "",
			},
		},
		{
			name: "empty Msg only",
			err: &Error{
				Op:  "Search",
				Err: ErrNotFound,
				Msg: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			got := tt.err.Error()
			if got == "" {
				t.Error("Error() returned empty string")
			}

			// Should be able to unwrap
			unwrapped := tt.err.Unwrap()
			if unwrapped == nil {
				t.Error("Unwrap() returned nil")
			}
		})
	}
}

func TestError_AsUsage(t *testing.T) {
	// Test errors.As usage pattern
	originalErr := &Error{
		Op:  "Index",
		Err: ErrIndexingFailed,
		Msg: "validation failed",
	}

	wrappedErr := &Error{
		Op:  "BatchIndex",
		Err: originalErr,
	}

	var searchErr *Error
	if !errors.As(wrappedErr, &searchErr) {
		t.Error("errors.As failed to match *Error type")
	}

	if searchErr.Op != "BatchIndex" {
		t.Errorf("errors.As returned wrong error: got Op=%q, want %q", searchErr.Op, "BatchIndex")
	}
}

func TestProviderType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		expected string
	}{
		{
			name:     "Algolia provider",
			provider: ProviderTypeAlgolia,
			expected: "algolia",
		},
		{
			name:     "Meilisearch provider",
			provider: ProviderTypeMeilisearch,
			expected: "meilisearch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.provider) != tt.expected {
				t.Errorf("ProviderType = %q, want %q", tt.provider, tt.expected)
			}
		})
	}
}

func TestProviderType_StringConversion(t *testing.T) {
	// Test that provider types can be used as strings
	providers := map[ProviderType]bool{
		ProviderTypeAlgolia:     true,
		ProviderTypeMeilisearch: true,
	}

	for provider := range providers {
		if provider == "" {
			t.Errorf("provider type is empty string")
		}
	}
}
