package print

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/store"
	"github.com/erxyi/qlx/internal/store/sqlite"
)

// mockEncoder is a minimal Encoder + StatusQuerier implementation for tests.
// Implementing StatusQuerier keeps the session heartbeat alive so that
// ConnectionManager maintains a stable StateConnected.
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

func (m *mockEncoder) Connect(_ context.Context, _ transport.Transport) error {
	return nil
}

func (m *mockEncoder) Heartbeat(_ transport.Transport) (encoder.HeartbeatResult, error) {
	return encoder.HeartbeatResult{Battery: 100, LidClosed: true, PaperLoaded: true}, nil
}

func (m *mockEncoder) RfidInfo(_ context.Context, _ transport.Transport) (encoder.RfidResult, error) {
	return encoder.RfidResult{}, nil
}

func newTestPrintStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	db, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// newManagerWithMock creates a PrinterManager backed by a ConnectionManager using MockTransport.
// It returns the manager, store, mock transport, and the ConnectionManager.
func newManagerWithMock(t *testing.T) (*PrinterManager, *sqlite.SQLiteStore, *transport.MockTransport, *ConnectionManager) {
	t.Helper()
	s := newTestPrintStore(t)

	mockTr := &transport.MockTransport{}
	mockEnc := &mockEncoder{}

	cm := NewConnectionManager(
		func(name string) transport.Transport {
			if name == "mock" {
				return mockTr
			}
			return nil
		},
		func(name string) encoder.Encoder {
			if name == "mock" {
				return mockEnc
			}
			return nil
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(cm.Stop)
	cm.Start(ctx)

	mgr := NewPrinterManager(s, cm)
	mgr.RegisterEncoder(mockEnc)

	return mgr, s, mockTr, cm
}

// waitForState polls until the printer reaches the desired state or timeout.
func waitForState(cm *ConnectionManager, printerID string, desired ConnState, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cm.State(printerID) == desired {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func TestPrinterManager_Print(t *testing.T) {
	mgr, s, mockTr, cm := newManagerWithMock(t)

	printer := s.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")

	if err := cm.Add(*printer); err != nil {
		t.Fatalf("cm.Add: %v", err)
	}
	if !waitForState(cm, printer.ID, StateConnected, 2*time.Second) {
		t.Fatalf("printer did not reach connected state, got %s", cm.State(printer.ID))
	}

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
	s := newTestPrintStore(t)
	mgr := NewPrinterManager(s, nil)

	err := mgr.Print("nonexistent-id", label.LabelData{Name: "x"}, "simple", label.RenderOpts{})
	if err == nil {
		t.Fatal("expected error for unknown printer, got nil")
	}
}

func TestPrinterManager_ConnectDisconnect(t *testing.T) {
	_, s, _, cm := newManagerWithMock(t)

	printer := s.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")

	if err := cm.Add(*printer); err != nil {
		t.Fatalf("cm.Add: %v", err)
	}
	if !waitForState(cm, printer.ID, StateConnected, 2*time.Second) {
		t.Fatalf("printer did not reach connected state")
	}

	state := cm.State(printer.ID)
	if state != StateConnected {
		t.Errorf("expected connected state after Add, got %s", state)
	}

	if err := cm.Remove(printer.ID); err != nil {
		t.Fatalf("cm.Remove: %v", err)
	}

	state = cm.State(printer.ID)
	if state == StateConnected {
		t.Error("expected printer to not be connected after Remove")
	}
}

func TestPrinterManager_PrintImage(t *testing.T) {
	mgr, s, mockTr, cm := newManagerWithMock(t)

	printer := s.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")

	if err := cm.Add(*printer); err != nil {
		t.Fatalf("cm.Add: %v", err)
	}
	if !waitForState(cm, printer.ID, StateConnected, 2*time.Second) {
		t.Fatalf("printer did not reach connected state")
	}

	img := image.NewRGBA(image.Rect(0, 0, 100, 50))

	if err := mgr.PrintImage(printer.ID, img); err != nil {
		t.Fatalf("PrintImage() returned unexpected error: %v", err)
	}

	if len(mockTr.Written) == 0 {
		t.Error("expected data to be written to transport, got none")
	}
}

func TestPrinterManager_PrintImageUnknownPrinter(t *testing.T) {
	s := newTestPrintStore(t)
	mgr := NewPrinterManager(s, nil)

	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	err := mgr.PrintImage("nonexistent-id", img)
	if err == nil {
		t.Fatal("expected error for unknown printer, got nil")
	}
}

func TestPrinterManager_AvailableEncoders(t *testing.T) {
	s := newTestPrintStore(t)
	mgr := NewPrinterManager(s, nil)
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
