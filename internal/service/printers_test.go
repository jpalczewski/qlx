package service

import (
	"errors"
	"testing"

	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/store"
)

type mockPrinterStore struct {
	store.PrinterStore
	added []string
}

func (m *mockPrinterStore) AddPrinter(name, encoder, model, transportName, address string) *store.PrinterConfig {
	m.added = append(m.added, transportName)
	return &store.PrinterConfig{Name: name, Transport: transportName}
}

func TestAddPrinter_TransportValidation(t *testing.T) {
	// Pick one registered transport that is available and, if any, one that is not.
	var available, unavailable string
	for _, in := range transport.List() {
		if in.Available && available == "" {
			available = in.Name
		}
		if !in.Available && unavailable == "" {
			unavailable = in.Name
		}
	}
	if available == "" {
		t.Fatal("no available transport registered")
	}

	tests := []struct {
		name      string
		transport string
		wantErr   error
	}{
		{"available transport", available, nil},
		{"unknown transport", "carrier-pigeon", ErrUnknownTransport},
		{"empty transport", "", ErrUnknownTransport},
		{"mock is not user selectable", "mock", ErrUnknownTransport},
	}
	if unavailable != "" {
		// Registered but unavailable in this build: stored, with a warning.
		tests = append(tests, struct {
			name      string
			transport string
			wantErr   error
		}{"registered but unavailable", unavailable, nil})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockPrinterStore{}
			svc := NewPrinterService(ms)
			p, err := svc.AddPrinter("P", "niimbot", "B1", tt.transport, "addr")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if p != nil || len(ms.added) != 0 {
					t.Errorf("printer stored despite error: %v %v", p, ms.added)
				}
				return
			}
			if len(ms.added) != 1 || ms.added[0] != tt.transport {
				t.Errorf("stored transports = %v", ms.added)
			}
		})
	}
}
