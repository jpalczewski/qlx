package dto

import (
	"testing"
)

func TestSplitBulkIDs(t *testing.T) {
	tests := []struct {
		name         string
		entries      []BulkIDEntry
		wantItems    []string
		wantContains []string
	}{
		{
			name:         "empty",
			entries:      nil,
			wantItems:    nil,
			wantContains: nil,
		},
		{
			name: "items only",
			entries: []BulkIDEntry{
				{ID: "i1", Type: "item"},
				{ID: "i2", Type: "item"},
			},
			wantItems:    []string{"i1", "i2"},
			wantContains: nil,
		},
		{
			name: "containers only",
			entries: []BulkIDEntry{
				{ID: "c1", Type: "container"},
			},
			wantItems:    nil,
			wantContains: []string{"c1"},
		},
		{
			name: "mixed",
			entries: []BulkIDEntry{
				{ID: "i1", Type: "item"},
				{ID: "c1", Type: "container"},
				{ID: "i2", Type: "item"},
				{ID: "c2", Type: "container"},
			},
			wantItems:    []string{"i1", "i2"},
			wantContains: []string{"c1", "c2"},
		},
		{
			name: "unknown type ignored",
			entries: []BulkIDEntry{
				{ID: "x1", Type: "unknown"},
				{ID: "i1", Type: "item"},
			},
			wantItems:    []string{"i1"},
			wantContains: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotItems, gotContains := SplitBulkIDs(tt.entries)
			if !sliceEqual(gotItems, tt.wantItems) {
				t.Errorf("items = %v, want %v", gotItems, tt.wantItems)
			}
			if !sliceEqual(gotContains, tt.wantContains) {
				t.Errorf("containers = %v, want %v", gotContains, tt.wantContains)
			}
		})
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
