package sharepoint

import (
	"errors"
	"testing"

	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

func TestScopedProviderID(t *testing.T) {
	got := scopedProviderID("site-1", "drive-1", "item-1")
	want := "sharepoint:site:site-1:drive:drive-1:item:item-1"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestParseProviderID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    providerParts
		wantErr bool
	}{
		{
			name:  "raw item ID uses configured scope",
			input: "item-1",
			want:  providerParts{siteID: "site-1", driveID: "drive-1", itemID: "item-1"},
		},
		{
			name:  "legacy short sharepoint ID uses configured scope",
			input: "sharepoint:item-1",
			want:  providerParts{siteID: "site-1", driveID: "drive-1", itemID: "item-1"},
		},
		{
			name:  "scoped sharepoint ID preserves scope",
			input: "sharepoint:site:site-1:drive:drive-1:item:item-1",
			want:  providerParts{siteID: "site-1", driveID: "drive-1", itemID: "item-1"},
		},
		{
			name:    "different drive is rejected",
			input:   "sharepoint:site:site-1:drive:drive-2:item:item-1",
			wantErr: true,
		},
		{
			name:    "malformed scoped ID is rejected",
			input:   "sharepoint:site:site-1:item:item-1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProviderID(tt.input, "site-1", "drive-1")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !errors.Is(err, workspace.ErrInvalidInput) {
					t.Fatalf("expected invalid input error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %#v, got %#v", tt.want, got)
			}
		})
	}
}
