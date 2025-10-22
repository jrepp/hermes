package docid

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// UUID is a stable, globally unique document identifier.
// This persists across provider migrations and represents the logical document.
//
// UUIDs are discovered in three ways:
//  1. Document declares UUID (in frontmatter/metadata)
//  2. Auto-assigned during indexing (written to document)
//  3. Explicit assignment during migration
//
// UUIDs are stored in:
//   - Database: documents.document_uuid (nullable initially, unique when set)
//   - Google Docs: customProperties.hermesUuid
//   - Markdown: frontmatter field "hermes-uuid"
//   - Local metadata: metadata.json
type UUID struct {
	value uuid.UUID
}

// NewUUID generates a new random UUID (v4).
func NewUUID() UUID {
	return UUID{value: uuid.New()}
}

// MustParseUUID parses a UUID from string, panicking on error.
// This is useful for test fixtures and constants where the UUID is known valid.
func MustParseUUID(s string) UUID {
	u, err := ParseUUID(s)
	if err != nil {
		panic(fmt.Sprintf("invalid UUID: %s: %v", s, err))
	}
	return u
}

// ParseUUID parses a UUID from string (e.g., "550e8400-e29b-41d4-a716-446655440000").
// Accepts standard UUID formats (with or without hyphens).
func ParseUUID(s string) (UUID, error) {
	if s == "" {
		return UUID{}, fmt.Errorf("UUID cannot be empty")
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return UUID{}, fmt.Errorf("invalid UUID format: %w", err)
	}
	return UUID{value: u}, nil
}

// String returns the canonical UUID string in lowercase with hyphens.
// Format: "550e8400-e29b-41d4-a716-446655440000"
func (u UUID) String() string {
	return u.value.String()
}

// IsZero returns true if this is the zero/nil UUID.
func (u UUID) IsZero() bool {
	return u.value == uuid.Nil
}

// Equal returns true if two UUIDs are equal.
func (u UUID) Equal(other UUID) bool {
	return u.value == other.value
}

// MarshalJSON implements json.Marshaler.
// UUIDs are serialized as strings: "550e8400-e29b-41d4-a716-446655440000"
func (u UUID) MarshalJSON() ([]byte, error) {
	if u.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(u.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (u *UUID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("UUID must be a string: %w", err)
	}
	if s == "" || s == "null" {
		*u = UUID{}
		return nil
	}
	parsed, err := ParseUUID(s)
	if err != nil {
		return err
	}
	*u = parsed
	return nil
}

// Scan implements sql.Scanner for database reading.
// Supports string and []byte input from database.
func (u *UUID) Scan(value interface{}) error {
	if value == nil {
		*u = UUID{}
		return nil
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			*u = UUID{}
			return nil
		}
		parsed, err := ParseUUID(v)
		if err != nil {
			return fmt.Errorf("cannot scan string into UUID: %w", err)
		}
		*u = parsed
		return nil
	case []byte:
		if len(v) == 0 {
			*u = UUID{}
			return nil
		}
		parsed, err := ParseUUID(string(v))
		if err != nil {
			return fmt.Errorf("cannot scan bytes into UUID: %w", err)
		}
		*u = parsed
		return nil
	default:
		return fmt.Errorf("cannot scan %T into UUID", value)
	}
}

// Value implements driver.Valuer for database writing.
// Returns nil for zero UUID, string for valid UUID.
func (u UUID) Value() (driver.Value, error) {
	if u.IsZero() {
		return nil, nil
	}
	return u.String(), nil
}
