package print

import (
	"fmt"
	"image"
	"image/color"
	imagedraw "image/draw"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
	"golang.org/x/image/draw"
)

// PrinterConfigStore provides read-only access to printer configuration.
type PrinterConfigStore interface {
	GetPrinter(id string) *store.PrinterConfig
	AllPrinters() []store.PrinterConfig
}

// PrinterManager manages persistent printer sessions with heartbeat.
type PrinterManager struct {
	store    PrinterConfigStore
	encoders map[string]encoder.Encoder
	cm       *ConnectionManager
}

// NewPrinterManager creates a PrinterManager backed by the given store and ConnectionManager.
func NewPrinterManager(s PrinterConfigStore, cm *ConnectionManager) *PrinterManager {
	return &PrinterManager{
		store:    s,
		cm:       cm,
		encoders: make(map[string]encoder.Encoder),
	}
}

// DefaultTransportFactory returns a TransportFactoryFn that creates transports by name.
func DefaultTransportFactory() TransportFactoryFn {
	return func(name string) transport.Transport {
		switch name {
		case "usb":
			return &transport.FileTransport{}
		case "gousb":
			return &transport.GoUSBTransport{}
		case "serial":
			return &transport.SerialTransport{}
		case "remote":
			return &transport.RemoteTransport{}
		case "ble":
			return &transport.BLETransport{}
		default:
			return nil
		}
	}
}

// findModel returns the ModelInfo matching modelID, or nil if not found.
func findModel(enc encoder.Encoder, modelID string) *encoder.ModelInfo {
	for _, mi := range enc.Models() {
		if mi.ID == modelID {
			info := mi
			return &info
		}
	}
	return nil
}

// RegisterEncoder adds an encoder. Must be called before use.
func (m *PrinterManager) RegisterEncoder(enc encoder.Encoder) {
	m.encoders[enc.Name()] = enc
}

// SetConnectionManager wires the ConnectionManager after construction.
// Used when CM needs PM's encoder lookup (breaks the initialization cycle).
func (m *PrinterManager) SetConnectionManager(cm *ConnectionManager) {
	m.cm = cm
}

// Encoder returns a registered encoder by name, or nil if not found.
func (m *PrinterManager) Encoder(name string) encoder.Encoder {
	return m.encoders[name]
}

// AvailableEncoders returns a snapshot of registered encoders.
func (m *PrinterManager) AvailableEncoders() map[string]encoder.Encoder {
	cp := make(map[string]encoder.Encoder, len(m.encoders))
	for k, v := range m.encoders {
		cp[k] = v
	}
	return cp
}

// GetStatus returns status for a single printer.
func (m *PrinterManager) GetStatus(printerID string) PrinterStatus {
	if m.cm == nil {
		return PrinterStatus{Battery: -1, TotalLabels: -1, UsedLabels: -1}
	}
	session := m.cm.Session(printerID)
	if session == nil {
		return PrinterStatus{Battery: -1, TotalLabels: -1, UsedLabels: -1}
	}
	return session.Status()
}

// AllStatuses returns statuses for all managed printers.
func (m *PrinterManager) AllStatuses() map[string]PrinterStatus {
	result := make(map[string]PrinterStatus)
	if m.cm == nil {
		return result
	}
	for id, state := range m.cm.States() {
		if state == StateConnected {
			if session := m.cm.Session(id); session != nil {
				result[id] = session.Status()
				continue
			}
		}
		result[id] = PrinterStatus{
			Connected:   state == StateConnected,
			Battery:     -1,
			TotalLabels: -1,
			UsedLabels:  -1,
		}
	}
	return result
}

// Print renders a label and sends it to the printer.
func (m *PrinterManager) Print(printerID string, data label.LabelData, templateName string, opts label.RenderOpts) error {
	cfg := m.store.GetPrinter(printerID)
	if cfg == nil {
		return fmt.Errorf("printer not found: %s", printerID)
	}

	enc, ok := m.encoders[cfg.Encoder]
	if !ok {
		return fmt.Errorf("encoder not found: %s", cfg.Encoder)
	}

	modelInfo := findModel(enc, cfg.Model)
	if modelInfo == nil {
		return fmt.Errorf("model not found: %s", cfg.Model)
	}

	img, err := label.Render(data, templateName, modelInfo.PrintWidthPx, modelInfo.DPI, opts)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	if m.cm.State(printerID) != StateConnected {
		return fmt.Errorf("printer %s not connected", printerID)
	}
	session := m.cm.Session(printerID)
	if session == nil {
		return fmt.Errorf("printer %s: no active session", printerID)
	}

	img = applyCalibrationOffset(img, cfg, modelInfo.PrintWidthPx)

	webutil.LogInfo("printing on %s (%s/%s)", cfg.Name, cfg.Encoder, cfg.Model)
	printOpts := encoder.PrintOpts{
		Density:  modelInfo.DensityDefault,
		AutoCut:  true,
		Quantity: 1,
	}
	if err := session.Print(img, cfg.Model, printOpts); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	webutil.LogInfo("print complete on %s", cfg.Name)
	return nil
}

// PrintImage sends a pre-rendered image directly to the printer, bypassing label rendering.
func (m *PrinterManager) PrintImage(printerID string, img image.Image) error {
	cfg := m.store.GetPrinter(printerID)
	if cfg == nil {
		return fmt.Errorf("printer not found: %s", printerID)
	}

	enc, ok := m.encoders[cfg.Encoder]
	if !ok {
		return fmt.Errorf("encoder not found: %s", cfg.Encoder)
	}

	modelInfo := findModel(enc, cfg.Model)
	if modelInfo == nil {
		return fmt.Errorf("model not found: %s", cfg.Model)
	}

	if m.cm.State(printerID) != StateConnected {
		return fmt.Errorf("printer %s not connected", printerID)
	}
	session := m.cm.Session(printerID)
	if session == nil {
		return fmt.Errorf("printer %s: no active session", printerID)
	}

	// Scale image to match printer printhead width if needed
	imgWidth := img.Bounds().Dx()
	if imgWidth != modelInfo.PrintWidthPx && modelInfo.PrintWidthPx > 0 {
		scale := float64(modelInfo.PrintWidthPx) / float64(imgWidth)
		newH := int(float64(img.Bounds().Dy()) * scale)
		dst := image.NewRGBA(image.Rect(0, 0, modelInfo.PrintWidthPx, newH))
		imagedraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, imagedraw.Src)
		draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
		img = dst
		webutil.LogInfo("scaled image %d→%dpx width for %s", imgWidth, modelInfo.PrintWidthPx, cfg.Model)
	}

	img = applyCalibrationOffset(img, cfg, modelInfo.PrintWidthPx)

	webutil.LogInfo("printing image on %s (%s/%s)", cfg.Name, cfg.Encoder, cfg.Model)
	printOpts := encoder.PrintOpts{
		Density:  modelInfo.DensityDefault,
		AutoCut:  true,
		Quantity: 1,
	}
	if err := session.Print(img, cfg.Model, printOpts); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	webutil.LogInfo("print image complete on %s", cfg.Name)
	return nil
}

// applyCalibrationOffset shifts the image content to compensate for printhead misalignment.
// Positive X = content moves right on paper, negative X = content moves left.
// The output image keeps the same dimensions (printhead width × original height).
// Content that shifts outside the canvas is clipped; exposed areas are white.
func applyCalibrationOffset(img image.Image, cfg *store.PrinterConfig, printheadPx int) image.Image {
	if cfg.OffsetX == 0 && cfg.OffsetY == 0 {
		return img
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	outW := w
	if printheadPx > 0 {
		outW = printheadPx
	}
	outH := h

	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	imagedraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, imagedraw.Src)

	// Source point: where in the original image to start reading.
	// If offset is negative (shift left), we skip pixels from the left of the source.
	srcPt := bounds.Min
	dstX := cfg.OffsetX
	if dstX < 0 {
		srcPt.X -= dstX // skip -dstX pixels from source left
		dstX = 0
	}
	dstY := cfg.OffsetY
	if dstY < 0 {
		srcPt.Y -= dstY
		dstY = 0
	}

	dstRect := image.Rect(dstX, dstY, dstX+w, dstY+h)
	imagedraw.Draw(dst, dstRect, img, srcPt, imagedraw.Src)

	webutil.LogInfo("applied calibration offset (%+d, %+d) for %s", cfg.OffsetX, cfg.OffsetY, cfg.Name)
	return dst
}
