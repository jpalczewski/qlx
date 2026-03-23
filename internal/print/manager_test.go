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

func newManagerWithMock(t *testing.T) (*PrinterManager, *store.MemoryStore, *transport.MockTransport) {
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
	return mgr, s, mockTr
}

func TestPrinterManager_Print(t *testing.T) {
	mgr, s, mockTr := newManagerWithMock(t)

	printer := s.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")

	data := label.LabelData{
		Name:        "Test Item",
		Description: "A test label",
	}

	if err := mgr.Print(printer.ID, data, "simple", label.RenderOpts{}); err != nil {
		t.Fatalf("Print() returned unexpected error: %v", err)
	}

	if len(mockTr.Written) == 0 {
		t.Error("expected data to be written to transport, got none")
	}
}

func TestPrinterManager_PrintUnknownPrinter(t *testing.T) {
	s := store.NewMemoryStore()
	mgr := NewPrinterManager(s)

	err := mgr.Print("nonexistent-id", label.LabelData{Name: "x"}, "simple", label.RenderOpts{})
	if err == nil {
		t.Fatal("expected error for unknown printer, got nil")
	}
}

func TestPrinterManager_ConnectDisconnect(t *testing.T) {
	mgr, s, _ := newManagerWithMock(t)

	printer := s.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")

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
	mgr, s, mockTr := newManagerWithMock(t)

	printer := s.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")

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

func TestApplyCalibrationOffset_NoOffset(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	cfg := &store.PrinterConfig{Name: "test"}
	result := applyCalibrationOffset(img, cfg, 100)
	// With zero offsets, should return original image
	if result != img {
		t.Error("expected same image when offsets are zero")
	}
}

func TestApplyCalibrationOffset_PositiveX(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	// Set a pixel to verify it moves
	img.Set(0, 0, image.Black)

	cfg := &store.PrinterConfig{Name: "test", OffsetX: 10}
	result := applyCalibrationOffset(img, cfg, 100)

	bounds := result.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 50 {
		t.Fatalf("expected 100x50, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestApplyCalibrationOffset_NegativeX(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	cfg := &store.PrinterConfig{Name: "test", OffsetX: -5}
	result := applyCalibrationOffset(img, cfg, 100)

	bounds := result.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 50 {
		t.Fatalf("expected 100x50, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestApplyCalibrationOffset_PositiveY(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	cfg := &store.PrinterConfig{Name: "test", OffsetY: 10}
	result := applyCalibrationOffset(img, cfg, 100)

	bounds := result.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 50 {
		t.Fatalf("expected 100x50, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestApplyCalibrationOffset_BothAxes(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	cfg := &store.PrinterConfig{Name: "test", OffsetX: 5, OffsetY: -3}
	result := applyCalibrationOffset(img, cfg, 200)

	bounds := result.Bounds()
	if bounds.Dx() != 200 {
		t.Fatalf("expected width 200 (printhead), got %d", bounds.Dx())
	}
	if bounds.Dy() != 50 {
		t.Fatalf("expected height 50, got %d", bounds.Dy())
	}
}
