package docid

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		want     bool
	}{
		{"google is valid", ProviderTypeGoogle, true},
		{"local is valid", ProviderTypeLocal, true},
		{"remote-hermes is valid", ProviderTypeRemoteHermes, true},
		{"empty is invalid", ProviderType(""), false},
		{"unknown is invalid", ProviderType("unknown"), false},
		{"s3 is invalid", ProviderType("s3"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.provider.IsValid())
		})
	}
}

func TestProviderType_String(t *testing.T) {
	assert.Equal(t, "google", ProviderTypeGoogle.String())
	assert.Equal(t, "local", ProviderTypeLocal.String())
	assert.Equal(t, "remote-hermes", ProviderTypeRemoteHermes.String())
}

func TestValidProviderTypes(t *testing.T) {
	types := ValidProviderTypes()
	assert.Len(t, types, 3)
	assert.Contains(t, types, ProviderTypeGoogle)
	assert.Contains(t, types, ProviderTypeLocal)
	assert.Contains(t, types, ProviderTypeRemoteHermes)
}

func TestNewProviderID(t *testing.T) {
	t.Run("valid Google provider", func(t *testing.T) {
		pid, err := NewProviderID(ProviderTypeGoogle, "1a2b3c4d5e6f7890")
		require.NoError(t, err)
		assert.Equal(t, ProviderTypeGoogle, pid.Provider())
		assert.Equal(t, "1a2b3c4d5e6f7890", pid.ID())
	})

	t.Run("valid Local provider", func(t *testing.T) {
		pid, err := NewProviderID(ProviderTypeLocal, "docs/rfc-001.md")
		require.NoError(t, err)
		assert.Equal(t, ProviderTypeLocal, pid.Provider())
		assert.Equal(t, "docs/rfc-001.md", pid.ID())
	})

	t.Run("valid RemoteHermes provider", func(t *testing.T) {
		pid, err := NewProviderID(ProviderTypeRemoteHermes, "https://hermes.example.com/docs/123")
		require.NoError(t, err)
		assert.Equal(t, ProviderTypeRemoteHermes, pid.Provider())
		assert.Equal(t, "https://hermes.example.com/docs/123", pid.ID())
	})

	t.Run("invalid provider type", func(t *testing.T) {
		_, err := NewProviderID(ProviderType("unknown"), "123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid provider type")
	})

	t.Run("empty ID", func(t *testing.T) {
		_, err := NewProviderID(ProviderTypeGoogle, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider ID cannot be empty")
	})
}

func TestGoogleFileID(t *testing.T) {
	t.Run("valid Google file ID", func(t *testing.T) {
		pid, err := GoogleFileID("1a2b3c4d5e6f7890")
		require.NoError(t, err)
		assert.Equal(t, ProviderTypeGoogle, pid.Provider())
		assert.Equal(t, "1a2b3c4d5e6f7890", pid.ID())
	})

	t.Run("typical Drive file ID length", func(t *testing.T) {
		// Real Google Drive IDs are typically 33-44 characters
		pid, err := GoogleFileID("1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms")
		require.NoError(t, err)
		assert.Equal(t, "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms", pid.ID())
	})

	t.Run("empty file ID", func(t *testing.T) {
		_, err := GoogleFileID("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Google file ID cannot be empty")
	})

	t.Run("too short file ID", func(t *testing.T) {
		_, err := GoogleFileID("short")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Google file ID too short")
	})

	t.Run("minimum valid length", func(t *testing.T) {
		pid, err := GoogleFileID("1234567890")
		require.NoError(t, err)
		assert.Equal(t, "1234567890", pid.ID())
	})
}

func TestLocalFileID(t *testing.T) {
	t.Run("valid file path", func(t *testing.T) {
		pid, err := LocalFileID("docs/rfc-001.md")
		require.NoError(t, err)
		assert.Equal(t, ProviderTypeLocal, pid.Provider())
		assert.Equal(t, "docs/rfc-001.md", pid.ID())
	})

	t.Run("path with backslashes normalized", func(t *testing.T) {
		pid, err := LocalFileID("docs\\rfc-001.md")
		require.NoError(t, err)
		assert.Equal(t, "docs/rfc-001.md", pid.ID())
	})

	t.Run("absolute path", func(t *testing.T) {
		pid, err := LocalFileID("/home/user/docs/rfc-001.md")
		require.NoError(t, err)
		assert.Equal(t, "/home/user/docs/rfc-001.md", pid.ID())
	})

	t.Run("Windows path normalized", func(t *testing.T) {
		pid, err := LocalFileID("C:\\Users\\docs\\rfc-001.md")
		require.NoError(t, err)
		assert.Equal(t, "C:/Users/docs/rfc-001.md", pid.ID())
	})

	t.Run("empty path", func(t *testing.T) {
		_, err := LocalFileID("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "local file path cannot be empty")
	})
}

func TestRemoteHermesID(t *testing.T) {
	t.Run("URL format", func(t *testing.T) {
		pid, err := RemoteHermesID("https://hermes.example.com/api/v2/documents/550e8400-e29b-41d4-a716-446655440000")
		require.NoError(t, err)
		assert.Equal(t, ProviderTypeRemoteHermes, pid.Provider())
		assert.Contains(t, pid.ID(), "hermes.example.com")
	})

	t.Run("UUID format", func(t *testing.T) {
		pid, err := RemoteHermesID("550e8400-e29b-41d4-a716-446655440000")
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", pid.ID())
	})

	t.Run("empty ID", func(t *testing.T) {
		_, err := RemoteHermesID("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote Hermes ID cannot be empty")
	})
}

func TestProviderID_IsZero(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var pid ProviderID
		assert.True(t, pid.IsZero())
	})

	t.Run("non-zero value", func(t *testing.T) {
		pid, _ := GoogleFileID("1a2b3c4d5e6f7890")
		assert.False(t, pid.IsZero())
	})
}

func TestProviderID_Equal(t *testing.T) {
	t.Run("equal provider IDs", func(t *testing.T) {
		pid1, _ := GoogleFileID("1a2b3c4d5e6f7890")
		pid2, _ := GoogleFileID("1a2b3c4d5e6f7890")
		assert.True(t, pid1.Equal(pid2))
	})

	t.Run("different IDs same provider", func(t *testing.T) {
		pid1, _ := GoogleFileID("1a2b3c4d5e6f7890")
		pid2, _ := GoogleFileID("5e6f7890")
		assert.False(t, pid1.Equal(pid2))
	})

	t.Run("same ID different providers", func(t *testing.T) {
		pid1, _ := GoogleFileID("1234567890")
		pid2, _ := LocalFileID("1234567890")
		assert.False(t, pid1.Equal(pid2))
	})

	t.Run("zero values are equal", func(t *testing.T) {
		var pid1, pid2 ProviderID
		assert.True(t, pid1.Equal(pid2))
	})
}

func TestProviderID_String(t *testing.T) {
	t.Run("Google provider", func(t *testing.T) {
		pid, _ := GoogleFileID("1a2b3c4d5e6f7890")
		assert.Equal(t, "google:1a2b3c4d5e6f7890", pid.String())
	})

	t.Run("Local provider", func(t *testing.T) {
		pid, _ := LocalFileID("docs/rfc-001.md")
		assert.Equal(t, "local:docs/rfc-001.md", pid.String())
	})

	t.Run("RemoteHermes provider", func(t *testing.T) {
		pid, _ := RemoteHermesID("https://hermes.example.com/docs/123")
		assert.Equal(t, "remote-hermes:https://hermes.example.com/docs/123", pid.String())
	})

	t.Run("zero value", func(t *testing.T) {
		var pid ProviderID
		assert.Equal(t, "", pid.String())
	})
}

func TestParseProviderID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType ProviderType
		wantID   string
		wantErr  bool
	}{
		{
			name:     "valid Google ID",
			input:    "google:1a2b3c4d5e6f7890",
			wantType: ProviderTypeGoogle,
			wantID:   "1a2b3c4d5e6f7890",
		},
		{
			name:     "valid Local ID",
			input:    "local:docs/rfc-001.md",
			wantType: ProviderTypeLocal,
			wantID:   "docs/rfc-001.md",
		},
		{
			name:     "valid RemoteHermes ID",
			input:    "remote-hermes:https://hermes.example.com/docs/123",
			wantType: ProviderTypeRemoteHermes,
			wantID:   "https://hermes.example.com/docs/123",
		},
		{
			name:     "ID with colons",
			input:    "google:prefix:suffix",
			wantType: ProviderTypeGoogle,
			wantID:   "prefix:suffix",
		},
		{
			name:    "missing colon",
			input:   "google1a2b3c4d",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "only provider",
			input:   "google:",
			wantErr: true, // Empty ID
		},
		{
			name:    "invalid provider",
			input:   "s3:bucket/key",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pid, err := ParseProviderID(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, pid.Provider())
			assert.Equal(t, tt.wantID, pid.ID())
		})
	}
}

func TestProviderID_MarshalJSON(t *testing.T) {
	t.Run("valid provider ID", func(t *testing.T) {
		pid, _ := GoogleFileID("1a2b3c4d5e6f7890")
		data, err := json.Marshal(pid)
		require.NoError(t, err)
		assert.JSONEq(t, `{"provider":"google","id":"1a2b3c4d5e6f7890"}`, string(data))
	})

	t.Run("zero provider ID", func(t *testing.T) {
		var pid ProviderID
		data, err := json.Marshal(pid)
		require.NoError(t, err)
		assert.Equal(t, "null", string(data))
	})

	t.Run("in struct", func(t *testing.T) {
		type testStruct struct {
			Provider ProviderID `json:"provider"`
			Name     string     `json:"name"`
		}
		s := testStruct{
			Provider: mustGoogleFileID("1a2b3c4d5e6f7890"),
			Name:     "test-doc",
		}
		data, err := json.Marshal(s)
		require.NoError(t, err)
		// Use JSONEq to compare regardless of field order
		assert.JSONEq(t, `{"provider":{"provider":"google","id":"1a2b3c4d5e6f7890"},"name":"test-doc"}`, string(data))
	})

	t.Run("Local provider", func(t *testing.T) {
		pid, _ := LocalFileID("docs/rfc-001.md")
		data, err := json.Marshal(pid)
		require.NoError(t, err)
		assert.JSONEq(t, `{"provider":"local","id":"docs/rfc-001.md"}`, string(data))
	})
}

func TestProviderID_UnmarshalJSON(t *testing.T) {
	t.Run("valid Google provider", func(t *testing.T) {
		var pid ProviderID
		err := json.Unmarshal([]byte(`{"provider":"google","id":"1a2b3c4d5e6f7890"}`), &pid)
		require.NoError(t, err)
		assert.Equal(t, ProviderTypeGoogle, pid.Provider())
		assert.Equal(t, "1a2b3c4d5e6f7890", pid.ID())
	})

	t.Run("valid Local provider", func(t *testing.T) {
		var pid ProviderID
		err := json.Unmarshal([]byte(`{"provider":"local","id":"docs/rfc-001.md"}`), &pid)
		require.NoError(t, err)
		assert.Equal(t, ProviderTypeLocal, pid.Provider())
		assert.Equal(t, "docs/rfc-001.md", pid.ID())
	})

	t.Run("null value", func(t *testing.T) {
		var pid ProviderID
		err := json.Unmarshal([]byte("null"), &pid)
		require.NoError(t, err)
		assert.True(t, pid.IsZero())
	})

	t.Run("empty object", func(t *testing.T) {
		var pid ProviderID
		err := json.Unmarshal([]byte(`{"provider":"","id":""}`), &pid)
		require.NoError(t, err)
		assert.True(t, pid.IsZero())
	})

	t.Run("invalid JSON", func(t *testing.T) {
		var pid ProviderID
		err := json.Unmarshal([]byte(`not json`), &pid)
		assert.Error(t, err)
	})

	t.Run("invalid provider type", func(t *testing.T) {
		var pid ProviderID
		err := json.Unmarshal([]byte(`{"provider":"unknown","id":"123"}`), &pid)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid provider type")
	})

	t.Run("missing ID field", func(t *testing.T) {
		var pid ProviderID
		err := json.Unmarshal([]byte(`{"provider":"google"}`), &pid)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider ID cannot be empty")
	})

	t.Run("in struct", func(t *testing.T) {
		type testStruct struct {
			Provider ProviderID `json:"provider"`
			Name     string     `json:"name"`
		}
		var s testStruct
		data := []byte(`{"provider":{"provider":"google","id":"1a2b3c4d5e6f7890"},"name":"test"}`)
		err := json.Unmarshal(data, &s)
		require.NoError(t, err)
		assert.Equal(t, ProviderTypeGoogle, s.Provider.Provider())
		assert.Equal(t, "1a2b3c4d5e6f7890", s.Provider.ID())
		assert.Equal(t, "test", s.Name)
	})
}

func TestProviderID_JSONRoundTrip(t *testing.T) {
	t.Run("Google provider round trip", func(t *testing.T) {
		original, _ := GoogleFileID("1a2b3c4d5e6f7890")

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var unmarshaled ProviderID
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.True(t, original.Equal(unmarshaled))
	})

	t.Run("Local provider round trip", func(t *testing.T) {
		original, _ := LocalFileID("docs/rfc-001.md")

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var unmarshaled ProviderID
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.True(t, original.Equal(unmarshaled))
	})

	t.Run("zero value round trip", func(t *testing.T) {
		var original ProviderID

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var unmarshaled ProviderID
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.True(t, unmarshaled.IsZero())
	})
}

// Helper function for tests - bypasses length validation
func mustGoogleFileID(id string) ProviderID {
	// Use NewProviderID directly to bypass length check for tests
	pid, err := NewProviderID(ProviderTypeGoogle, id)
	if err != nil {
		panic(err)
	}
	return pid
}
