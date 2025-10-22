package docid

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// CompositeID is a fully-qualified document identifier containing:
//   - UUID: Stable global identifier
//   - ProviderID: Backend-specific identifier (optional for UUID-only lookups)
//   - Project: Project configuration context (optional)
//
// CompositeIDs support multiple serialization formats:
//   - Short: "uuid/{uuid}" - Most common, for APIs and UIs
//   - Full: "uuid:{uuid}:provider:{provider}:id:{id}:project:{project}"
//   - URI: "uuid/{uuid}?provider={provider}&id={id}&project={project}"
//
// A CompositeID can be partial (UUID-only) or complete (with provider and project).
type CompositeID struct {
	uuid       UUID
	providerID ProviderID
	project    string // Optional project ID
}

// NewCompositeID creates a new composite ID.
// All fields are optional but at least UUID should be set for meaningful IDs.
func NewCompositeID(uuid UUID, providerID ProviderID, project string) CompositeID {
	return CompositeID{
		uuid:       uuid,
		providerID: providerID,
		project:    project,
	}
}

// NewCompositeIDFromUUID creates a composite ID with only UUID set.
// This is the most common use case for document lookups.
func NewCompositeIDFromUUID(uuid UUID) CompositeID {
	return CompositeID{uuid: uuid}
}

// UUID returns the stable document UUID.
func (c CompositeID) UUID() UUID {
	return c.uuid
}

// ProviderID returns the provider-specific identifier.
// Returns zero ProviderID if not set.
func (c CompositeID) ProviderID() ProviderID {
	return c.providerID
}

// Project returns the project ID.
// Returns empty string if not set.
func (c CompositeID) Project() string {
	return c.project
}

// HasProvider returns true if this composite ID has provider information.
func (c CompositeID) HasProvider() bool {
	return !c.providerID.IsZero()
}

// HasProject returns true if this composite ID has project context.
func (c CompositeID) HasProject() bool {
	return c.project != ""
}

// IsZero returns true if this is a zero CompositeID (no fields set).
func (c CompositeID) IsZero() bool {
	return c.uuid.IsZero() && c.providerID.IsZero() && c.project == ""
}

// IsComplete returns true if this composite ID has all fields set (UUID, provider, project).
func (c CompositeID) IsComplete() bool {
	return !c.uuid.IsZero() && !c.providerID.IsZero() && c.project != ""
}

// Equal returns true if two CompositeIDs are equal.
func (c CompositeID) Equal(other CompositeID) bool {
	return c.uuid.Equal(other.uuid) &&
		c.providerID.Equal(other.providerID) &&
		c.project == other.project
}

// String returns a canonical string representation.
// Format: "uuid:{uuid}:provider:{provider}:id:{id}:project:{project}"
// Omits empty fields.
func (c CompositeID) String() string {
	if c.IsZero() {
		return ""
	}

	var parts []string

	if !c.uuid.IsZero() {
		parts = append(parts, fmt.Sprintf("uuid:%s", c.uuid.String()))
	}

	if !c.providerID.IsZero() {
		parts = append(parts, fmt.Sprintf("provider:%s", c.providerID.Provider()))
		parts = append(parts, fmt.Sprintf("id:%s", c.providerID.ID()))
	}

	if c.project != "" {
		parts = append(parts, fmt.Sprintf("project:%s", c.project))
	}

	return strings.Join(parts, ":")
}

// ShortString returns a human-readable short form.
// Format: "uuid/{uuid}" - Most common for APIs and UIs
// Falls back to full string if UUID is not set.
func (c CompositeID) ShortString() string {
	if c.uuid.IsZero() {
		return c.String()
	}
	return fmt.Sprintf("uuid/%s", c.uuid.String())
}

// URIString returns a URI-safe format for URLs with query parameters.
// Format: "uuid/{uuid}?provider={provider}&id={id}&project={project}"
// This format is easier to parse in web contexts.
func (c CompositeID) URIString() string {
	if c.IsZero() {
		return ""
	}

	u := url.URL{
		Path: c.ShortString(),
	}

	if !c.providerID.IsZero() || c.project != "" {
		q := url.Values{}
		if !c.providerID.IsZero() {
			q.Set("provider", string(c.providerID.Provider()))
			q.Set("id", c.providerID.ID())
		}
		if c.project != "" {
			q.Set("project", c.project)
		}
		u.RawQuery = q.Encode()
	}

	return u.String()
}

// ParseCompositeID parses a composite ID from various formats.
// Supports:
//   - Short format: "uuid/{uuid}" or just "{uuid}"
//   - Full format: "uuid:{uuid}:provider:{provider}:id:{id}:project:{project}"
//   - URI format: "uuid/{uuid}?provider={provider}&id={id}"
//   - Provider-only: "provider:{provider}:id:{id}"
func ParseCompositeID(s string) (CompositeID, error) {
	if s == "" {
		return CompositeID{}, fmt.Errorf("composite ID string cannot be empty")
	}

	// Try UUID-only short format first (most common)
	if strings.HasPrefix(s, "uuid/") {
		return parseUUIDOnlyFormat(s)
	}

	// Try full colon-separated format
	if strings.Contains(s, ":") && !strings.Contains(s, "://") {
		return parseFullFormat(s)
	}

	// Try bare UUID string
	uuid, err := ParseUUID(s)
	if err == nil {
		return NewCompositeIDFromUUID(uuid), nil
	}

	return CompositeID{}, fmt.Errorf("unrecognized composite ID format: %s", s)
}

// parseUUIDOnlyFormat parses "uuid/{uuid}" or "uuid/{uuid}?params"
func parseUUIDOnlyFormat(s string) (CompositeID, error) {
	// Check for query parameters
	if strings.Contains(s, "?") {
		u, err := url.Parse(s)
		if err != nil {
			return CompositeID{}, fmt.Errorf("invalid URI format: %w", err)
		}

		// Extract UUID from path
		uuidStr := strings.TrimPrefix(u.Path, "uuid/")
		uuid, err := ParseUUID(uuidStr)
		if err != nil {
			return CompositeID{}, fmt.Errorf("invalid UUID in URI: %w", err)
		}

		// Extract optional query params
		var providerID ProviderID
		q := u.Query()
		if provider := q.Get("provider"); provider != "" {
			id := q.Get("id")
			if id == "" {
				return CompositeID{}, fmt.Errorf("provider specified without id")
			}
			providerID, err = NewProviderID(ProviderType(provider), id)
			if err != nil {
				return CompositeID{}, fmt.Errorf("invalid provider ID in URI: %w", err)
			}
		}

		project := q.Get("project")

		return NewCompositeID(uuid, providerID, project), nil
	}

	// Simple "uuid/{uuid}" format
	uuidStr := strings.TrimPrefix(s, "uuid/")
	uuid, err := ParseUUID(uuidStr)
	if err != nil {
		return CompositeID{}, fmt.Errorf("invalid UUID: %w", err)
	}

	return NewCompositeIDFromUUID(uuid), nil
}

// parseFullFormat parses "uuid:{uuid}:provider:{provider}:id:{id}:project:{project}"
func parseFullFormat(s string) (CompositeID, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return CompositeID{}, fmt.Errorf("invalid format (expected key:value pairs): %s", s)
	}

	// Parse key:value pairs
	kv := make(map[string]string)
	for i := 0; i < len(parts)-1; i += 2 {
		key := parts[i]
		value := parts[i+1]
		kv[key] = value
	}

	var cid CompositeID

	// Parse UUID (optional)
	if uuidStr, ok := kv["uuid"]; ok {
		uuid, err := ParseUUID(uuidStr)
		if err != nil {
			return CompositeID{}, fmt.Errorf("invalid UUID: %w", err)
		}
		cid.uuid = uuid
	}

	// Parse provider ID (optional, but requires both provider and id)
	if provider, hasProvider := kv["provider"]; hasProvider {
		id, hasID := kv["id"]
		if !hasID {
			return CompositeID{}, fmt.Errorf("provider specified without id")
		}
		providerID, err := NewProviderID(ProviderType(provider), id)
		if err != nil {
			return CompositeID{}, fmt.Errorf("invalid provider ID: %w", err)
		}
		cid.providerID = providerID
	}

	// Parse project (optional)
	if project, ok := kv["project"]; ok {
		cid.project = project
	}

	// Validate that at least something was parsed
	if cid.IsZero() {
		return CompositeID{}, fmt.Errorf("no valid fields found in: %s", s)
	}

	return cid, nil
}

// MarshalJSON implements json.Marshaler.
// Serializes as: {"uuid": "...", "provider": {...}, "project": "..."}
func (c CompositeID) MarshalJSON() ([]byte, error) {
	if c.IsZero() {
		return []byte("null"), nil
	}

	obj := make(map[string]interface{})

	if !c.uuid.IsZero() {
		obj["uuid"] = c.uuid.String()
	}

	if !c.providerID.IsZero() {
		// Marshal provider ID as nested object
		providerJSON, err := c.providerID.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal provider ID: %w", err)
		}
		var providerObj map[string]string
		if err := json.Unmarshal(providerJSON, &providerObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal provider JSON: %w", err)
		}
		obj["provider"] = providerObj
	}

	if c.project != "" {
		obj["project"] = c.project
	}

	return json.Marshal(obj)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *CompositeID) UnmarshalJSON(data []byte) error {
	var obj struct {
		UUID     *string `json:"uuid"`
		Provider *struct {
			Provider string `json:"provider"`
			ID       string `json:"id"`
		} `json:"provider"`
		Project *string `json:"project"`
	}

	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid CompositeID JSON: %w", err)
	}

	// Parse UUID
	if obj.UUID != nil && *obj.UUID != "" {
		uuid, err := ParseUUID(*obj.UUID)
		if err != nil {
			return fmt.Errorf("invalid UUID in JSON: %w", err)
		}
		c.uuid = uuid
	}

	// Parse provider ID
	if obj.Provider != nil {
		providerID, err := NewProviderID(
			ProviderType(obj.Provider.Provider),
			obj.Provider.ID)
		if err != nil {
			return fmt.Errorf("invalid provider in JSON: %w", err)
		}
		c.providerID = providerID
	}

	// Parse project
	if obj.Project != nil {
		c.project = *obj.Project
	}

	return nil
}
