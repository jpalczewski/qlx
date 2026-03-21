package webutil

import (
	"testing"

	"github.com/erxyi/qlx/internal/store"
)

func TestFormatContainerPath(t *testing.T) {
	tests := []struct {
		name string
		path []store.Container
		sep  string
		want string
	}{
		{
			name: "empty path",
			path: nil,
			sep:  " → ",
			want: "",
		},
		{
			name: "single container",
			path: []store.Container{{Name: "Box A"}},
			sep:  " → ",
			want: "Box A",
		},
		{
			name: "multiple containers unicode arrow",
			path: []store.Container{{Name: "Room"}, {Name: "Shelf"}, {Name: "Box"}},
			sep:  " → ",
			want: "Room → Shelf → Box",
		},
		{
			name: "multiple containers ASCII arrow for CSV",
			path: []store.Container{{Name: "Room"}, {Name: "Shelf"}},
			sep:  " -> ",
			want: "Room -> Shelf",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatContainerPath(tt.path, tt.sep)
			if got != tt.want {
				t.Errorf("FormatContainerPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
