//go:build !minimal

package transport

import (
	"context"
	"testing"
)

// Sessions close the transport after a failed Open, so Close must be safe on a
// transport that never opened.
func TestCloseAfterFailedOpen(t *testing.T) {
	tests := []struct {
		name string
		tr   Transport
	}{
		{"serial", &SerialTransport{}},
		{"file", &FileTransport{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.tr.Open(context.Background(), "/nonexistent/device"); err == nil {
				t.Fatal("Open on a missing device succeeded")
			}
			if err := tt.tr.Close(); err != nil {
				t.Fatalf("Close after failed Open: %v", err)
			}
		})
	}
}
