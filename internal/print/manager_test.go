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

func newManagerWithMock(t *testing.T) (*PrinterManager, *transport.MockTransport) {
	t.Helper()
	s := store.NewMemoryStore()
	mgr := NewPrinterManager(s)
	mgr.RegisterEncoder(&mockEncoder{})

	mockTr := &transport.MockTransport{}
	mgr.transportFn = func(name string) transport.Transport {
		if name == "mock" {
			return mockTr
		}
		return nil
	}
	return mgr, mockTr
}

func TestPrinterManager_Print(t *testing.T) {
	mgr, mockTr := newManagerWithMock(t)

	printer := mgr.store.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")

	data := label.LabelData{
		Name:        "Test Item",
		Description: "A test label",
	}

	if err := mgr.Print(printer.ID, data, "simple"); err != nil {
		t.Fatalf("Print() returned unexpected error: %v", err)
	}

	if len(mockTr.Written) == 0 {
		t.Error("expected data to be written to transport, got none")
	}
}

func TestPrinterManager_PrintUnknownPrinter(t *testing.T) {
	s := store.NewMemoryStore()
	mgr := NewPrinterManager(s)

	err := mgr.Print("nonexistent-id", label.LabelData{Name: "x"}, "simple")
	if err == nil {
		t.Fatal("expected error for unknown printer, got nil")
	}
}

func TestPrinterManager_ConnectDisconnect(t *testing.T) {
	mgr, _ := newManagerWithMock(t)

	printer := mgr.store.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")

	if err := mgr.ConnectPrinter(printer.ID); err != nil {
		t.Fatalf("ConnectPrinter() returned unexpected error: %v", err)
	}

	status := mgr.GetStatus(printer.ID)
	if !status.Connected {
		t.Error("expected printer to be connected after ConnectPrinter")
	}

	mgr.DisconnectPrinter(printer.ID)

	status = mgr.GetStatus(printer.ID)
	if status.Connected {
		t.Error("expected printer to be disconnected after DisconnectPrinter")
	}
}

func TestPrinterManager_PrintImage(t *testing.T) {
	mgr, mockTr := newManagerWithMock(t)

	printer := mgr.store.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")

	img := image.NewRGBA(image.Rect(0, 0, 100, 50))

	if err := mgr.PrintImage(printer.ID, img); err != nil {
		t.Fatalf("PrintImage() returned unexpected error: %v", err)
	}

	if len(mockTr.Written) == 0 {
		t.Error("expected data to be written to transport, got none")
	}
}

func TestPrinterManager_PrintImageUnknownPrinter(t *testing.T) {
	s := store.NewMemoryStore()
	mgr := NewPrinterManager(s)

	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	err := mgr.PrintImage("nonexistent-id", img)
	if err == nil {
		t.Fatal("expected error for unknown printer, got nil")
	}
}

func TestPrinterManager_AvailableEncoders(t *testing.T) {
	s := store.NewMemoryStore()
	mgr := NewPrinterManager(s)
	mgr.RegisterEncoder(&mockEncoder{})

	encs := mgr.AvailableEncoders()
	if _, ok := encs["mock"]; !ok {
		t.Error("expected 'mock' encoder to be present in AvailableEncoders")
	}
}
