package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// JSON is a custom JSON type that implements driver.Valuer and sql.Scanner.
// This replaces gorm.io/datatypes.JSON to avoid pulling in gorm.io/driver/sqlite
// as a transitive dependency (which causes SQLite driver conflicts).
//
// It works with both PostgreSQL JSONB and SQLite JSON columns.
type JSON json.RawMessage

// Value implements driver.Valuer interface for database writes.
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	// Validate JSON before storing
	var tmp interface{}
	if err := json.Unmarshal(j, &tmp); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return []byte(j), nil
}

// Scan implements sql.Scanner interface for database reads.
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = JSON("null")
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("failed to unmarshal JSON value: unsupported type")
	}

	// Validate JSON
	var tmp interface{}
	if err := json.Unmarshal(bytes, &tmp); err != nil {
		return fmt.Errorf("invalid JSON in database: %w", err)
	}

	*j = JSON(bytes)
	return nil
}

// MarshalJSON implements json.Marshaler interface.
func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (j *JSON) UnmarshalJSON(data []byte) error {
	if j == nil {
		return errors.New("JSON: UnmarshalJSON on nil pointer")
	}
	*j = append((*j)[0:0], data...)
	return nil
}

// String returns the JSON as a string.
func (j JSON) String() string {
	return string(j)
}
