package docid

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUUID(t *testing.T) {
	t.Run("generates valid non-zero UUID", func(t *testing.T) {
		u := NewUUID()
		assert.False(t, u.IsZero())
		assert.Len(t, u.String(), 36) // Standard UUID format
	})

	t.Run("generates unique UUIDs", func(t *testing.T) {
		u1 := NewUUID()
		u2 := NewUUID()
		assert.False(t, u1.Equal(u2))
	})
}

func TestMustParseUUID(t *testing.T) {
	t.Run("parses valid UUID", func(t *testing.T) {
		u := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", u.String())
	})

	t.Run("panics on invalid UUID", func(t *testing.T) {
		assert.Panics(t, func() {
			MustParseUUID("not-a-uuid")
		})
	})

	t.Run("panics on empty UUID", func(t *testing.T) {
		assert.Panics(t, func() {
			MustParseUUID("")
		})
	})
}

func TestParseUUID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid UUID with hyphens",
			input: "550e8400-e29b-41d4-a716-446655440000",
			want:  "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:  "valid UUID uppercase",
			input: "550E8400-E29B-41D4-A716-446655440000",
			want:  "550e8400-e29b-41d4-a716-446655440000", // Normalized to lowercase
		},
		{
			name:  "valid UUID without hyphens",
			input: "550e8400e29b41d4a716446655440000",
			want:  "550e8400-e29b-41d4-a716-446655440000", // Normalized with hyphens
		},
		{
			name:    "invalid UUID format",
			input:   "not-a-uuid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "too short",
			input:   "550e8400",
			wantErr: true,
		},
		{
			name:    "invalid characters",
			input:   "550e8400-e29b-41d4-a716-44665544000g",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := ParseUUID(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, u.String())
		})
	}
}

func TestUUID_IsZero(t *testing.T) {
	t.Run("zero UUID", func(t *testing.T) {
		var u UUID
		assert.True(t, u.IsZero())
	})

	t.Run("non-zero UUID", func(t *testing.T) {
		u := NewUUID()
		assert.False(t, u.IsZero())
	})

	t.Run("parsed nil UUID", func(t *testing.T) {
		u, err := ParseUUID("00000000-0000-0000-0000-000000000000")
		require.NoError(t, err)
		assert.True(t, u.IsZero())
	})
}

func TestUUID_Equal(t *testing.T) {
	t.Run("equal UUIDs", func(t *testing.T) {
		u1, _ := ParseUUID("550e8400-e29b-41d4-a716-446655440000")
		u2, _ := ParseUUID("550e8400-e29b-41d4-a716-446655440000")
		assert.True(t, u1.Equal(u2))
	})

	t.Run("different UUIDs", func(t *testing.T) {
		u1 := NewUUID()
		u2 := NewUUID()
		assert.False(t, u1.Equal(u2))
	})

	t.Run("zero UUIDs are equal", func(t *testing.T) {
		var u1, u2 UUID
		assert.True(t, u1.Equal(u2))
	})
}

func TestUUID_MarshalJSON(t *testing.T) {
	t.Run("valid UUID", func(t *testing.T) {
		u, _ := ParseUUID("550e8400-e29b-41d4-a716-446655440000")
		data, err := json.Marshal(u)
		require.NoError(t, err)
		assert.Equal(t, `"550e8400-e29b-41d4-a716-446655440000"`, string(data))
	})

	t.Run("zero UUID", func(t *testing.T) {
		var u UUID
		data, err := json.Marshal(u)
		require.NoError(t, err)
		assert.Equal(t, "null", string(data))
	})

	t.Run("in struct", func(t *testing.T) {
		type testStruct struct {
			ID   UUID   `json:"id"`
			Name string `json:"name"`
		}
		s := testStruct{
			ID:   MustParseUUID("550e8400-e29b-41d4-a716-446655440000"),
			Name: "test",
		}
		data, err := json.Marshal(s)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"id":"550e8400-e29b-41d4-a716-446655440000"`)
		assert.Contains(t, string(data), `"name":"test"`)
	})
}

func TestUUID_UnmarshalJSON(t *testing.T) {
	t.Run("valid UUID", func(t *testing.T) {
		var u UUID
		err := json.Unmarshal([]byte(`"550e8400-e29b-41d4-a716-446655440000"`), &u)
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", u.String())
	})

	t.Run("null value", func(t *testing.T) {
		var u UUID
		err := json.Unmarshal([]byte("null"), &u)
		require.NoError(t, err)
		assert.True(t, u.IsZero())
	})

	t.Run("empty string", func(t *testing.T) {
		var u UUID
		err := json.Unmarshal([]byte(`""`), &u)
		require.NoError(t, err)
		assert.True(t, u.IsZero())
	})

	t.Run("invalid UUID", func(t *testing.T) {
		var u UUID
		err := json.Unmarshal([]byte(`"not-a-uuid"`), &u)
		assert.Error(t, err)
	})

	t.Run("not a string", func(t *testing.T) {
		var u UUID
		err := json.Unmarshal([]byte(`123`), &u)
		assert.Error(t, err)
	})

	t.Run("in struct", func(t *testing.T) {
		type testStruct struct {
			ID   UUID   `json:"id"`
			Name string `json:"name"`
		}
		var s testStruct
		data := []byte(`{"id":"550e8400-e29b-41d4-a716-446655440000","name":"test"}`)
		err := json.Unmarshal(data, &s)
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", s.ID.String())
		assert.Equal(t, "test", s.Name)
	})
}

func TestUUID_Scan(t *testing.T) {
	t.Run("scan from string", func(t *testing.T) {
		var u UUID
		err := u.Scan("550e8400-e29b-41d4-a716-446655440000")
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", u.String())
	})

	t.Run("scan from bytes", func(t *testing.T) {
		var u UUID
		err := u.Scan([]byte("550e8400-e29b-41d4-a716-446655440000"))
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", u.String())
	})

	t.Run("scan from nil", func(t *testing.T) {
		var u UUID
		err := u.Scan(nil)
		require.NoError(t, err)
		assert.True(t, u.IsZero())
	})

	t.Run("scan from empty string", func(t *testing.T) {
		var u UUID
		err := u.Scan("")
		require.NoError(t, err)
		assert.True(t, u.IsZero())
	})

	t.Run("scan from empty bytes", func(t *testing.T) {
		var u UUID
		err := u.Scan([]byte{})
		require.NoError(t, err)
		assert.True(t, u.IsZero())
	})

	t.Run("scan invalid type", func(t *testing.T) {
		var u UUID
		err := u.Scan(123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot scan int")
	})

	t.Run("scan invalid UUID", func(t *testing.T) {
		var u UUID
		err := u.Scan("not-a-uuid")
		assert.Error(t, err)
	})
}

func TestUUID_Value(t *testing.T) {
	t.Run("valid UUID", func(t *testing.T) {
		u := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		val, err := u.Value()
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", val)
	})

	t.Run("zero UUID", func(t *testing.T) {
		var u UUID
		val, err := u.Value()
		require.NoError(t, err)
		assert.Nil(t, val)
	})
}

func TestUUID_DatabaseRoundTrip(t *testing.T) {
	t.Run("round trip non-zero UUID", func(t *testing.T) {
		original := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")

		// Simulate writing to database
		val, err := original.Value()
		require.NoError(t, err)

		// Simulate reading from database
		var scanned UUID
		err = scanned.Scan(val)
		require.NoError(t, err)

		assert.True(t, original.Equal(scanned))
	})

	t.Run("round trip zero UUID", func(t *testing.T) {
		var original UUID

		// Simulate writing to database
		val, err := original.Value()
		require.NoError(t, err)
		assert.Nil(t, val)

		// Simulate reading from database
		var scanned UUID
		err = scanned.Scan(val)
		require.NoError(t, err)

		assert.True(t, scanned.IsZero())
	})
}

// BenchmarkUUID_Parse benchmarks UUID parsing performance.
func BenchmarkUUID_Parse(b *testing.B) {
	uuidStr := "550e8400-e29b-41d4-a716-446655440000"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseUUID(uuidStr)
	}
}

// BenchmarkUUID_String benchmarks UUID string conversion.
func BenchmarkUUID_String(b *testing.B) {
	u := NewUUID()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = u.String()
	}
}

// BenchmarkUUID_MarshalJSON benchmarks JSON marshaling.
func BenchmarkUUID_MarshalJSON(b *testing.B) {
	u := NewUUID()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(u)
	}
}

// BenchmarkUUID_UnmarshalJSON benchmarks JSON unmarshaling.
func BenchmarkUUID_UnmarshalJSON(b *testing.B) {
	data := []byte(`"550e8400-e29b-41d4-a716-446655440000"`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var u UUID
		_ = json.Unmarshal(data, &u)
	}
}

// TestUUID_ThreadSafety verifies UUID operations are safe for concurrent use.
func TestUUID_ThreadSafety(t *testing.T) {
	t.Run("concurrent NewUUID calls", func(t *testing.T) {
		const goroutines = 100
		done := make(chan UUID, goroutines)

		for i := 0; i < goroutines; i++ {
			go func() {
				done <- NewUUID()
			}()
		}

		// Collect all UUIDs
		uuids := make(map[string]bool)
		for i := 0; i < goroutines; i++ {
			u := <-done
			uuids[u.String()] = true
		}

		// All should be unique
		assert.Len(t, uuids, goroutines)
	})
}

// TestUUID_Integration tests UUID with actual uuid.UUID behavior.
func TestUUID_Integration(t *testing.T) {
	t.Run("wraps google uuid correctly", func(t *testing.T) {
		// Create using google's uuid package directly
		googleUUID := uuid.New()

		// Parse using our wrapper
		ourUUID, err := ParseUUID(googleUUID.String())
		require.NoError(t, err)

		// Should be equal
		assert.Equal(t, googleUUID.String(), ourUUID.String())
	})

	t.Run("nil UUID behavior", func(t *testing.T) {
		nilUUID := uuid.Nil
		ourUUID, err := ParseUUID(nilUUID.String())
		require.NoError(t, err)
		assert.True(t, ourUUID.IsZero())
	})
}
