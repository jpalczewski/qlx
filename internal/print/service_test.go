package print

import (
	"image"
	"testing"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/store"
)

// mockEncoder is a minimal Encoder implementation for tests.
type mockEncoder struct{}

func (m *mockEncoder) Name() string { return "mock" }

func (m *mockEncoder) Models() []encoder.ModelInfo {
	return []encoder.ModelInfo{
		{
			ID:             "mock-model",
			Name:           "Mock Model",
			DPI:            203,
			PrintWidthPx:   696,
			DensityDefault: 3,
		},
	}
}

func (m *mockEncoder) Encode(img image.Image, model string, opts encoder.PrintOpts, tr transport.Transport) error {
	// Write a trivial byte to satisfy the transport.
	_, err := tr.Write([]byte{0x01})
	return err
}

func TestPrintService_Print(t *testing.T) {
	s := store.NewMemoryStore()

	ps := NewPrintService(s)

	// Register mock encoder.
	ps.RegisterEncoder(&mockEncoder{})

	// Override transport factory to return MockTransport for "mock".
	mockTr := &transport.MockTransport{}
	ps.transportFactory = func(name string) transport.Transport {
		if name == "mock" {
			return mockTr
		}
		return nil
	}

	// Add a printer using the mock encoder, model, and transport.
	printer := s.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")

	data := label.LabelData{
		Name:        "Test Item",
		Description: "A test label",
	}

	if err := ps.Print(printer.ID, data, "simple"); err != nil {
		t.Fatalf("Print() returned unexpected error: %v", err)
	}

	// Verify that data was written to the transport.
	if len(mockTr.Written) == 0 {
		t.Error("expected data to be written to transport, got none")
	}
}

func TestPrintService_PrintUnknownPrinter(t *testing.T) {
	s := store.NewMemoryStore()
	ps := NewPrintService(s)

	err := ps.Print("nonexistent-id", label.LabelData{Name: "x"}, "simple")
	if err == nil {
		t.Fatal("expected error for unknown printer, got nil")
	}
}

func TestPrintService_PrintUnknownEncoder(t *testing.T) {
	s := store.NewMemoryStore()
	ps := NewPrintService(s)

	printer := s.AddPrinter("P", "noenc", "model", "mock", "/dev/null")

	err := ps.Print(printer.ID, label.LabelData{Name: "x"}, "simple")
	if err == nil {
		t.Fatal("expected error for unknown encoder, got nil")
	}
}

func TestPrintService_PrintUnknownModel(t *testing.T) {
	s := store.NewMemoryStore()
	ps := NewPrintService(s)
	ps.RegisterEncoder(&mockEncoder{})

	printer := s.AddPrinter("P", "mock", "bad-model", "mock", "/dev/null")

	err := ps.Print(printer.ID, label.LabelData{Name: "x"}, "simple")
	if err == nil {
		t.Fatal("expected error for unknown model, got nil")
	}
}

func TestPrintService_PrintUnknownTransport(t *testing.T) {
	s := store.NewMemoryStore()
	ps := NewPrintService(s)
	ps.RegisterEncoder(&mockEncoder{})

	printer := s.AddPrinter("P", "mock", "mock-model", "bad-transport", "/dev/null")

	err := ps.Print(printer.ID, label.LabelData{Name: "x"}, "simple")
	if err == nil {
		t.Fatal("expected error for unknown transport, got nil")
	}
}

func TestPrintService_AvailableEncoders(t *testing.T) {
	s := store.NewMemoryStore()
	ps := NewPrintService(s)
	ps.RegisterEncoder(&mockEncoder{})

	encs := ps.AvailableEncoders()
	if _, ok := encs["mock"]; !ok {
		t.Error("expected 'mock' encoder to be present in AvailableEncoders")
	}
}
