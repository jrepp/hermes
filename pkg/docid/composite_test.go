package docid

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCompositeID(t *testing.T) {
	t.Run("complete composite ID", func(t *testing.T) {
		uuid := NewUUID()
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "rfc-archive")

		assert.Equal(t, uuid, cid.UUID())
		assert.True(t, cid.ProviderID().Equal(providerID))
		assert.Equal(t, "rfc-archive", cid.Project())
	})

	t.Run("partial composite ID - UUID only", func(t *testing.T) {
		uuid := NewUUID()
		cid := NewCompositeID(uuid, ProviderID{}, "")

		assert.Equal(t, uuid, cid.UUID())
		assert.True(t, cid.ProviderID().IsZero())
		assert.Equal(t, "", cid.Project())
	})

	t.Run("partial composite ID - with provider no project", func(t *testing.T) {
		uuid := NewUUID()
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "")

		assert.False(t, cid.IsComplete())
		assert.True(t, cid.HasProvider())
		assert.False(t, cid.HasProject())
	})
}

func TestNewCompositeIDFromUUID(t *testing.T) {
	t.Run("creates UUID-only composite ID", func(t *testing.T) {
		uuid := NewUUID()
		cid := NewCompositeIDFromUUID(uuid)

		assert.Equal(t, uuid, cid.UUID())
		assert.True(t, cid.ProviderID().IsZero())
		assert.Equal(t, "", cid.Project())
	})

	t.Run("zero UUID", func(t *testing.T) {
		var uuid UUID
		cid := NewCompositeIDFromUUID(uuid)

		assert.True(t, cid.UUID().IsZero())
	})
}

func TestCompositeID_HasProvider(t *testing.T) {
	t.Run("with provider", func(t *testing.T) {
		uuid := NewUUID()
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "")

		assert.True(t, cid.HasProvider())
	})

	t.Run("without provider", func(t *testing.T) {
		uuid := NewUUID()
		cid := NewCompositeIDFromUUID(uuid)

		assert.False(t, cid.HasProvider())
	})
}

func TestCompositeID_HasProject(t *testing.T) {
	t.Run("with project", func(t *testing.T) {
		uuid := NewUUID()
		cid := NewCompositeID(uuid, ProviderID{}, "rfc-archive")

		assert.True(t, cid.HasProject())
	})

	t.Run("without project", func(t *testing.T) {
		uuid := NewUUID()
		cid := NewCompositeIDFromUUID(uuid)

		assert.False(t, cid.HasProject())
	})
}

func TestCompositeID_IsZero(t *testing.T) {
	t.Run("zero composite ID", func(t *testing.T) {
		var cid CompositeID
		assert.True(t, cid.IsZero())
	})

	t.Run("with UUID only", func(t *testing.T) {
		uuid := NewUUID()
		cid := NewCompositeIDFromUUID(uuid)

		assert.False(t, cid.IsZero())
	})

	t.Run("with all fields", func(t *testing.T) {
		uuid := NewUUID()
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "rfcs")

		assert.False(t, cid.IsZero())
	})
}

func TestCompositeID_IsComplete(t *testing.T) {
	t.Run("complete - all fields set", func(t *testing.T) {
		uuid := NewUUID()
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "rfc-archive")

		assert.True(t, cid.IsComplete())
	})

	t.Run("incomplete - missing provider", func(t *testing.T) {
		uuid := NewUUID()
		cid := NewCompositeID(uuid, ProviderID{}, "rfcs")

		assert.False(t, cid.IsComplete())
	})

	t.Run("incomplete - missing project", func(t *testing.T) {
		uuid := NewUUID()
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "")

		assert.False(t, cid.IsComplete())
	})

	t.Run("incomplete - UUID only", func(t *testing.T) {
		uuid := NewUUID()
		cid := NewCompositeIDFromUUID(uuid)

		assert.False(t, cid.IsComplete())
	})

	t.Run("incomplete - zero UUID", func(t *testing.T) {
		var uuid UUID
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "rfcs")

		assert.False(t, cid.IsComplete())
	})
}

func TestCompositeID_Equal(t *testing.T) {
	t.Run("equal complete IDs", func(t *testing.T) {
		uuid, _ := ParseUUID("550e8400-e29b-41d4-a716-446655440000")
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid1 := NewCompositeID(uuid, providerID, "rfcs")
		cid2 := NewCompositeID(uuid, providerID, "rfcs")

		assert.True(t, cid1.Equal(cid2))
	})

	t.Run("different UUIDs", func(t *testing.T) {
		uuid1 := NewUUID()
		uuid2 := NewUUID()
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid1 := NewCompositeID(uuid1, providerID, "rfcs")
		cid2 := NewCompositeID(uuid2, providerID, "rfcs")

		assert.False(t, cid1.Equal(cid2))
	})

	t.Run("different providers", func(t *testing.T) {
		uuid := NewUUID()
		googleID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		localID, _ := LocalFileID("docs/rfc.md")
		cid1 := NewCompositeID(uuid, googleID, "rfcs")
		cid2 := NewCompositeID(uuid, localID, "rfcs")

		assert.False(t, cid1.Equal(cid2))
	})

	t.Run("different projects", func(t *testing.T) {
		uuid := NewUUID()
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid1 := NewCompositeID(uuid, providerID, "rfcs")
		cid2 := NewCompositeID(uuid, providerID, "prds")

		assert.False(t, cid1.Equal(cid2))
	})

	t.Run("zero values are equal", func(t *testing.T) {
		var cid1, cid2 CompositeID
		assert.True(t, cid1.Equal(cid2))
	})
}

func TestCompositeID_String(t *testing.T) {
	t.Run("complete ID", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "rfc-archive")

		str := cid.String()
		assert.Contains(t, str, "uuid:550e8400-e29b-41d4-a716-446655440000")
		assert.Contains(t, str, "provider:google")
		assert.Contains(t, str, "id:1a2b3c4d5e6f7890")
		assert.Contains(t, str, "project:rfc-archive")
	})

	t.Run("UUID only", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		cid := NewCompositeIDFromUUID(uuid)

		str := cid.String()
		assert.Equal(t, "uuid:550e8400-e29b-41d4-a716-446655440000", str)
	})

	t.Run("with provider no project", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "")

		str := cid.String()
		assert.Contains(t, str, "uuid:550e8400-e29b-41d4-a716-446655440000")
		assert.Contains(t, str, "provider:google")
		assert.Contains(t, str, "id:1a2b3c4d5e6f7890")
		assert.NotContains(t, str, "project:")
	})

	t.Run("zero value", func(t *testing.T) {
		var cid CompositeID
		assert.Equal(t, "", cid.String())
	})
}

func TestCompositeID_ShortString(t *testing.T) {
	t.Run("with UUID", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		cid := NewCompositeIDFromUUID(uuid)

		assert.Equal(t, "uuid/550e8400-e29b-41d4-a716-446655440000", cid.ShortString())
	})

	t.Run("complete ID uses short format", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "rfcs")

		// ShortString only includes UUID, not provider/project
		assert.Equal(t, "uuid/550e8400-e29b-41d4-a716-446655440000", cid.ShortString())
	})

	t.Run("zero UUID falls back to String", func(t *testing.T) {
		var uuid UUID
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "rfcs")

		// Should fall back to full string representation
		assert.Contains(t, cid.ShortString(), "provider:")
	})
}

func TestCompositeID_URIString(t *testing.T) {
	t.Run("complete ID", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "rfc-archive")

		uri := cid.URIString()
		assert.Contains(t, uri, "uuid/550e8400-e29b-41d4-a716-446655440000")
		assert.Contains(t, uri, "provider=google")
		assert.Contains(t, uri, "id=1a2b3c4d5e6f7890")
		assert.Contains(t, uri, "project=rfc-archive")
	})

	t.Run("UUID only", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		cid := NewCompositeIDFromUUID(uuid)

		uri := cid.URIString()
		assert.Equal(t, "uuid/550e8400-e29b-41d4-a716-446655440000", uri)
		assert.NotContains(t, uri, "?")
	})

	t.Run("with provider no project", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "")

		uri := cid.URIString()
		assert.Contains(t, uri, "provider=google")
		assert.Contains(t, uri, "id=1a2b3c4d5e6f7890")
		assert.NotContains(t, uri, "project=")
	})

	t.Run("zero value", func(t *testing.T) {
		var cid CompositeID
		assert.Equal(t, "", cid.URIString())
	})
}

func TestParseCompositeID(t *testing.T) {
	t.Run("short format - uuid/...", func(t *testing.T) {
		cid, err := ParseCompositeID("uuid/550e8400-e29b-41d4-a716-446655440000")
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", cid.UUID().String())
		assert.False(t, cid.HasProvider())
		assert.False(t, cid.HasProject())
	})

	t.Run("bare UUID", func(t *testing.T) {
		cid, err := ParseCompositeID("550e8400-e29b-41d4-a716-446655440000")
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", cid.UUID().String())
	})

	t.Run("URI format with query params", func(t *testing.T) {
		cid, err := ParseCompositeID("uuid/550e8400-e29b-41d4-a716-446655440000?provider=google&id=1a2b3c4d5e6f7890&project=rfcs")
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", cid.UUID().String())
		assert.Equal(t, ProviderTypeGoogle, cid.ProviderID().Provider())
		assert.Equal(t, "1a2b3c4d5e6f7890", cid.ProviderID().ID())
		assert.Equal(t, "rfcs", cid.Project())
	})

	t.Run("URI format with provider only", func(t *testing.T) {
		cid, err := ParseCompositeID("uuid/550e8400-e29b-41d4-a716-446655440000?provider=local&id=docs/rfc.md")
		require.NoError(t, err)
		assert.Equal(t, ProviderTypeLocal, cid.ProviderID().Provider())
		assert.Equal(t, "docs/rfc.md", cid.ProviderID().ID())
		assert.False(t, cid.HasProject())
	})

	t.Run("full colon-separated format", func(t *testing.T) {
		cid, err := ParseCompositeID("uuid:550e8400-e29b-41d4-a716-446655440000:provider:google:id:1a2b3c4d5e6f7890:project:rfcs")
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", cid.UUID().String())
		assert.Equal(t, ProviderTypeGoogle, cid.ProviderID().Provider())
		assert.Equal(t, "1a2b3c4d5e6f7890", cid.ProviderID().ID())
		assert.Equal(t, "rfcs", cid.Project())
	})

	t.Run("full format without project", func(t *testing.T) {
		cid, err := ParseCompositeID("uuid:550e8400-e29b-41d4-a716-446655440000:provider:google:id:1a2b3c4d5e6f7890")
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", cid.UUID().String())
		assert.True(t, cid.HasProvider())
		assert.False(t, cid.HasProject())
	})

	t.Run("provider-only format", func(t *testing.T) {
		cid, err := ParseCompositeID("provider:google:id:1a2b3c4d5e6f7890")
		require.NoError(t, err)
		assert.True(t, cid.UUID().IsZero())
		assert.Equal(t, ProviderTypeGoogle, cid.ProviderID().Provider())
		assert.Equal(t, "1a2b3c4d5e6f7890", cid.ProviderID().ID())
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := ParseCompositeID("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("invalid UUID", func(t *testing.T) {
		_, err := ParseCompositeID("uuid/not-a-uuid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid UUID")
	})

	t.Run("unrecognized format", func(t *testing.T) {
		_, err := ParseCompositeID("random-string-format")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unrecognized")
	})

	t.Run("URI with invalid UUID", func(t *testing.T) {
		_, err := ParseCompositeID("uuid/invalid?provider=google&id=1a2b3c4d5e6f7890")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid UUID")
	})

	t.Run("URI with provider but no id", func(t *testing.T) {
		_, err := ParseCompositeID("uuid/550e8400-e29b-41d4-a716-446655440000?provider=google")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider specified without id")
	})

	t.Run("full format with provider but no id", func(t *testing.T) {
		_, err := ParseCompositeID("uuid:550e8400-e29b-41d4-a716-446655440000:provider:google")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider specified without id")
	})

	t.Run("full format with invalid provider", func(t *testing.T) {
		_, err := ParseCompositeID("uuid:550e8400-e29b-41d4-a716-446655440000:provider:s3:id:bucket/key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid provider")
	})

	t.Run("full format with no valid fields", func(t *testing.T) {
		_, err := ParseCompositeID("foo:bar")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no valid fields")
	})
}

func TestCompositeID_MarshalJSON(t *testing.T) {
	t.Run("complete composite ID", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "rfc-archive")

		data, err := json.Marshal(cid)
		require.NoError(t, err)

		var obj map[string]interface{}
		err = json.Unmarshal(data, &obj)
		require.NoError(t, err)

		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", obj["uuid"])
		assert.Equal(t, "rfc-archive", obj["project"])

		provider, ok := obj["provider"].(map[string]interface{})
		require.True(t, ok, "provider should be a map")
		assert.Equal(t, "google", provider["provider"])
		assert.Equal(t, "1a2b3c4d5e6f7890", provider["id"])
	})

	t.Run("UUID only", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		cid := NewCompositeIDFromUUID(uuid)

		data, err := json.Marshal(cid)
		require.NoError(t, err)

		var obj map[string]interface{}
		err = json.Unmarshal(data, &obj)
		require.NoError(t, err)

		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", obj["uuid"])
		assert.Nil(t, obj["provider"])
		assert.Nil(t, obj["project"])
	})

	t.Run("zero composite ID", func(t *testing.T) {
		var cid CompositeID
		data, err := json.Marshal(cid)
		require.NoError(t, err)
		assert.Equal(t, "null", string(data))
	})

	t.Run("with provider no project", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		providerID, _ := LocalFileID("docs/rfc.md")
		cid := NewCompositeID(uuid, providerID, "")

		data, err := json.Marshal(cid)
		require.NoError(t, err)

		var obj map[string]interface{}
		err = json.Unmarshal(data, &obj)
		require.NoError(t, err)

		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", obj["uuid"])
		provider, ok := obj["provider"].(map[string]interface{})
		require.True(t, ok, "provider should be a map")
		assert.Equal(t, "local", provider["provider"])
		assert.Nil(t, obj["project"])
	})
}

func TestCompositeID_UnmarshalJSON(t *testing.T) {
	t.Run("complete composite ID", func(t *testing.T) {
		data := []byte(`{
			"uuid": "550e8400-e29b-41d4-a716-446655440000",
			"provider": {"provider": "google", "id": "1a2b3c4d5e6f7890"},
			"project": "rfc-archive"
		}`)

		var cid CompositeID
		err := json.Unmarshal(data, &cid)
		require.NoError(t, err)

		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", cid.UUID().String())
		assert.Equal(t, ProviderTypeGoogle, cid.ProviderID().Provider())
		assert.Equal(t, "1a2b3c4d5e6f7890", cid.ProviderID().ID())
		assert.Equal(t, "rfc-archive", cid.Project())
	})

	t.Run("UUID only", func(t *testing.T) {
		data := []byte(`{"uuid": "550e8400-e29b-41d4-a716-446655440000"}`)

		var cid CompositeID
		err := json.Unmarshal(data, &cid)
		require.NoError(t, err)

		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", cid.UUID().String())
		assert.False(t, cid.HasProvider())
		assert.False(t, cid.HasProject())
	})

	t.Run("null value", func(t *testing.T) {
		var cid CompositeID
		err := json.Unmarshal([]byte("null"), &cid)
		require.NoError(t, err)
		assert.True(t, cid.IsZero())
	})

	t.Run("empty object", func(t *testing.T) {
		var cid CompositeID
		err := json.Unmarshal([]byte("{}"), &cid)
		require.NoError(t, err)
		assert.True(t, cid.UUID().IsZero())
	})

	t.Run("invalid JSON", func(t *testing.T) {
		var cid CompositeID
		err := json.Unmarshal([]byte("not json"), &cid)
		assert.Error(t, err)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		data := []byte(`{"uuid": "not-a-uuid"}`)
		var cid CompositeID
		err := json.Unmarshal(data, &cid)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid UUID")
	})

	t.Run("invalid provider", func(t *testing.T) {
		data := []byte(`{
			"uuid": "550e8400-e29b-41d4-a716-446655440000",
			"provider": {"provider": "unknown", "id": "123"}
		}`)
		var cid CompositeID
		err := json.Unmarshal(data, &cid)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid provider")
	})
}

func TestCompositeID_JSONRoundTrip(t *testing.T) {
	t.Run("complete composite ID", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		original := NewCompositeID(uuid, providerID, "rfc-archive")

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var unmarshaled CompositeID
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.True(t, original.Equal(unmarshaled))
	})

	t.Run("UUID only", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		original := NewCompositeIDFromUUID(uuid)

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var unmarshaled CompositeID
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.True(t, original.Equal(unmarshaled))
	})

	t.Run("with Local provider", func(t *testing.T) {
		uuid := NewUUID()
		providerID, _ := LocalFileID("docs/rfc-001.md")
		original := NewCompositeID(uuid, providerID, "rfcs-new")

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var unmarshaled CompositeID
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.True(t, original.Equal(unmarshaled))
	})
}

func TestCompositeID_StringFormatRoundTrip(t *testing.T) {
	t.Run("short format round trip", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		original := NewCompositeIDFromUUID(uuid)

		shortStr := original.ShortString()
		parsed, err := ParseCompositeID(shortStr)
		require.NoError(t, err)

		assert.True(t, original.UUID().Equal(parsed.UUID()))
	})

	t.Run("full format round trip", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		original := NewCompositeID(uuid, providerID, "rfcs")

		fullStr := original.String()
		parsed, err := ParseCompositeID(fullStr)
		require.NoError(t, err)

		assert.True(t, original.Equal(parsed))
	})

	t.Run("URI format round trip", func(t *testing.T) {
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		original := NewCompositeID(uuid, providerID, "rfc-archive")

		uriStr := original.URIString()
		parsed, err := ParseCompositeID(uriStr)
		require.NoError(t, err)

		assert.True(t, original.Equal(parsed))
	})
}

// TestCompositeID_Integration tests realistic usage scenarios.
func TestCompositeID_Integration(t *testing.T) {
	t.Run("cross-provider document tracking", func(t *testing.T) {
		// Same document in both Google and Local
		uuid := MustParseUUID("550e8400-e29b-41d4-a716-446655440000")

		// Google revision
		googleID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		googleComposite := NewCompositeID(uuid, googleID, "rfcs-old")

		// Local revision
		localID, _ := LocalFileID("docs/rfc-001.md")
		localComposite := NewCompositeID(uuid, localID, "rfcs-new")

		// Both share same UUID but different providers
		assert.True(t, googleComposite.UUID().Equal(localComposite.UUID()))
		assert.False(t, googleComposite.Equal(localComposite)) // Different providers
		assert.True(t, googleComposite.HasProvider())
		assert.True(t, localComposite.HasProvider())
	})

	t.Run("API URL generation", func(t *testing.T) {
		uuid := NewUUID()
		cid := NewCompositeIDFromUUID(uuid)

		// Generate API URL
		apiURL := "/api/v2/documents/" + cid.ShortString()
		assert.Contains(t, apiURL, "uuid/")

		// Parse back from URL
		pathPart := apiURL[len("/api/v2/documents/"):]
		parsed, err := ParseCompositeID(pathPart)
		require.NoError(t, err)
		assert.True(t, cid.UUID().Equal(parsed.UUID()))
	})

	t.Run("database storage", func(t *testing.T) {
		// Simulate document storage
		type DocumentRecord struct {
			UUID         string
			ProviderType string
			ProviderID   string
			ProjectID    string
		}

		uuid := NewUUID()
		providerID, _ := GoogleFileID("1a2b3c4d5e6f7890")
		cid := NewCompositeID(uuid, providerID, "rfc-archive")

		// Store in database
		record := DocumentRecord{
			UUID:         cid.UUID().String(),
			ProviderType: string(cid.ProviderID().Provider()),
			ProviderID:   cid.ProviderID().ID(),
			ProjectID:    cid.Project(),
		}

		// Reconstruct from database
		reconstructedUUID, _ := ParseUUID(record.UUID)
		reconstructedProvider, _ := NewProviderID(ProviderType(record.ProviderType), record.ProviderID)
		reconstructed := NewCompositeID(reconstructedUUID, reconstructedProvider, record.ProjectID)

		assert.True(t, cid.Equal(reconstructed))
	})
}
