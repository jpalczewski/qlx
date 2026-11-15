//go:build usb

package transport

import "testing"

func TestAutoDetachDefault(t *testing.T) {
	tests := []struct {
		goos string
		want bool
	}{
		{"linux", true},
		{"darwin", false}, // no kernel driver on the printer interface; detach needs root there
		{"windows", false},
		{"freebsd", false},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			if got := autoDetachDefault(tt.goos); got != tt.want {
				t.Errorf("autoDetachDefault(%q) = %v, want %v", tt.goos, got, tt.want)
			}
		})
	}
}
