package docid

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ProviderType identifies the storage backend for a document.
type ProviderType string

const (
	// ProviderTypeGoogle identifies Google Workspace (Google Drive/Docs).
	ProviderTypeGoogle ProviderType = "google"

	// ProviderTypeLocal identifies local filesystem storage (Git repos, markdown).
	ProviderTypeLocal ProviderType = "local"

	// ProviderTypeRemoteHermes identifies a remote Hermes instance.
	ProviderTypeRemoteHermes ProviderType = "remote-hermes"
)

// ValidProviderTypes returns all valid provider types.
func ValidProviderTypes() []ProviderType {
	return []ProviderType{
		ProviderTypeGoogle,
		ProviderTypeLocal,
		ProviderTypeRemoteHermes,
	}
}

// IsValid returns true if this is a recognized provider type.
func (pt ProviderType) IsValid() bool {
	switch pt {
	case ProviderTypeGoogle, ProviderTypeLocal, ProviderTypeRemoteHermes:
		return true
	default:
		return false
	}
}

// String returns the string representation of the provider type.
func (pt ProviderType) String() string {
	return string(pt)
}

// ProviderID represents a document's identifier within a specific provider.
//
// Each provider type has different ID formats:
//   - Google: Google Drive file ID (e.g., "1a2b3c4d5e6f7890")
//   - Local: Relative file path (e.g., "docs/rfc-001.md")
//   - Remote Hermes: URL or UUID (e.g., "https://hermes.example.com/api/v2/documents/{id}")
//
// ProviderIDs are immutable once created.
type ProviderID struct {
	provider ProviderType
	id       string
}

// NewProviderID creates a provider-specific ID.
// Returns error if provider type is invalid or ID is empty.
func NewProviderID(provider ProviderType, id string) (ProviderID, error) {
	if !provider.IsValid() {
		return ProviderID{}, fmt.Errorf("invalid provider type: %s (valid: %v)",
			provider, ValidProviderTypes())
	}
	if id == "" {
		return ProviderID{}, fmt.Errorf("provider ID cannot be empty")
	}
	return ProviderID{provider: provider, id: id}, nil
}

// GoogleFileID creates a Google Drive file ID.
// This is a convenience constructor for the most common use case.
func GoogleFileID(id string) (ProviderID, error) {
	if id == "" {
		return ProviderID{}, fmt.Errorf("Google file ID cannot be empty")
	}
	// Basic validation: Google file IDs are alphanumeric and hyphens
	// Typical length: 33-44 characters, but we don't enforce strict length
	if len(id) < 10 {
		return ProviderID{}, fmt.Errorf("Google file ID too short: %s", id)
	}
	return NewProviderID(ProviderTypeGoogle, id)
}

// LocalFileID creates a local filesystem ID (file path).
// Path should be relative to the workspace root.
func LocalFileID(path string) (ProviderID, error) {
	if path == "" {
		return ProviderID{}, fmt.Errorf("local file path cannot be empty")
	}
	// Normalize path separators for consistency
	normalizedPath := strings.ReplaceAll(path, "\\", "/")
	return NewProviderID(ProviderTypeLocal, normalizedPath)
}

// RemoteHermesID creates a remote Hermes instance ID.
// This can be a full URL or just the document UUID.
func RemoteHermesID(id string) (ProviderID, error) {
	if id == "" {
		return ProviderID{}, fmt.Errorf("remote Hermes ID cannot be empty")
	}
	return NewProviderID(ProviderTypeRemoteHermes, id)
}

// Provider returns the provider type.
func (p ProviderID) Provider() ProviderType {
	return p.provider
}

// ID returns the provider-specific identifier.
func (p ProviderID) ID() string {
	return p.id
}

// IsZero returns true if this is a zero ProviderID.
func (p ProviderID) IsZero() bool {
	return p.provider == "" && p.id == ""
}

// Equal returns true if two ProviderIDs are equal.
func (p ProviderID) Equal(other ProviderID) bool {
	return p.provider == other.provider && p.id == other.id
}

// String returns the canonical string representation.
// Format: "provider:id" (e.g., "google:1a2b3c4d5e6f7890")
func (p ProviderID) String() string {
	if p.IsZero() {
		return ""
	}
	return fmt.Sprintf("%s:%s", p.provider, p.id)
}

// ParseProviderID parses a provider ID from string.
// Expected format: "provider:id" (e.g., "google:1a2b3c4d5e6f7890")
func ParseProviderID(s string) (ProviderID, error) {
	if s == "" {
		return ProviderID{}, fmt.Errorf("provider ID string cannot be empty")
	}

	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return ProviderID{}, fmt.Errorf(
			"invalid provider ID format (expected 'provider:id'): %s", s)
	}

	return NewProviderID(ProviderType(parts[0]), parts[1])
}

// MarshalJSON implements json.Marshaler.
// Serializes as: {"provider": "google", "id": "1a2b3c4d"}
func (p ProviderID) MarshalJSON() ([]byte, error) {
	if p.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(map[string]string{
		"provider": string(p.provider),
		"id":       p.id,
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *ProviderID) UnmarshalJSON(data []byte) error {
	var obj struct {
		Provider string `json:"provider"`
		ID       string `json:"id"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid ProviderID JSON: %w", err)
	}
	if obj.Provider == "" && obj.ID == "" {
		*p = ProviderID{}
		return nil
	}
	parsed, err := NewProviderID(ProviderType(obj.Provider), obj.ID)
	if err != nil {
		return fmt.Errorf("invalid ProviderID: %w", err)
	}
	*p = parsed
	return nil
}
