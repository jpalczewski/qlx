package print

import (
	"fmt"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// TransportFactory creates a Transport by name.
type TransportFactory func(name string) transport.Transport

// PrintService orchestrates label rendering, encoding, and transport.
type PrintService struct {
	store            *store.Store
	encoders         map[string]encoder.Encoder
	transportFactory TransportFactory
}

// NewPrintService creates a PrintService with the default transport factory.
func NewPrintService(s *store.Store) *PrintService {
	ps := &PrintService{
		store:    s,
		encoders: make(map[string]encoder.Encoder),
	}
	ps.transportFactory = ps.defaultTransportFactory
	return ps
}

// RegisterEncoder registers an encoder under its name.
func (ps *PrintService) RegisterEncoder(enc encoder.Encoder) {
	ps.encoders[enc.Name()] = enc
}

// AvailableEncoders returns all registered encoders.
func (ps *PrintService) AvailableEncoders() map[string]encoder.Encoder {
	return ps.encoders
}

// Print renders and sends a label to the named printer.
func (ps *PrintService) Print(printerID string, data label.LabelData, templateName string) error {
	// 1. Get printer config from store
	cfg := ps.store.GetPrinter(printerID)
	if cfg == nil {
		return fmt.Errorf("printer not found: %s", printerID)
	}

	// 2. Get encoder
	enc, ok := ps.encoders[cfg.Encoder]
	if !ok {
		return fmt.Errorf("encoder not found: %s", cfg.Encoder)
	}

	// 3. Find model info for DPI and width
	var modelInfo *encoder.ModelInfo
	for _, m := range enc.Models() {
		if m.ID == cfg.Model {
			mi := m
			modelInfo = &mi
			break
		}
	}
	if modelInfo == nil {
		return fmt.Errorf("model not found: %s", cfg.Model)
	}

	// 4. Render label
	img, err := label.Render(data, templateName, modelInfo.PrintWidthPx, modelInfo.DPI)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	// 5. Create transport (with trace wrapper if enabled)
	tr := ps.transportFactory(cfg.Transport)
	if tr == nil {
		return fmt.Errorf("unknown transport: %s", cfg.Transport)
	}
	if webutil.TraceEnabled {
		tr = &transport.TraceTransport{Inner: tr}
	}

	// 6. Open transport
	if err := tr.Open(cfg.Address); err != nil {
		return fmt.Errorf("open %s: %w", cfg.Address, err)
	}
	defer tr.Close()

	// 7. Encode and send
	webutil.LogInfo("printing on %s (%s/%s via %s)", cfg.Name, cfg.Encoder, cfg.Model, cfg.Transport)
	opts := encoder.PrintOpts{
		Density:  modelInfo.DensityDefault,
		AutoCut:  true,
		Quantity: 1,
	}
	if err := enc.Encode(img, cfg.Model, opts, tr); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	webutil.LogInfo("print complete on %s", cfg.Name)
	return nil
}

// defaultTransportFactory returns a new Transport instance by transport name.
func (ps *PrintService) defaultTransportFactory(name string) transport.Transport {
	switch name {
	case "usb":
		return &transport.FileTransport{}
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
