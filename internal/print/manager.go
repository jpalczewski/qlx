package print

import (
	"fmt"
	"sync"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// TransportFactory creates a Transport by name.
type TransportFactory func(name string) transport.Transport

// PrinterStatusEvent is sent to SSE subscribers.
type PrinterStatusEvent struct {
	PrinterID string        `json:"printer_id"`
	Status    PrinterStatus `json:"status"`
}

// PrinterManager manages persistent printer sessions with heartbeat.
type PrinterManager struct {
	store      *store.Store
	encoders   map[string]encoder.Encoder
	sessions   map[string]*PrinterSession
	mu         sync.RWMutex
	sseMu      sync.Mutex
	sseClients map[chan PrinterStatusEvent]struct{}
	transportFn TransportFactory
}

func NewPrinterManager(s *store.Store) *PrinterManager {
	m := &PrinterManager{
		store:      s,
		encoders:   make(map[string]encoder.Encoder),
		sessions:   make(map[string]*PrinterSession),
		sseClients: make(map[chan PrinterStatusEvent]struct{}),
	}
	m.transportFn = m.defaultTransportFactory
	return m
}

func (m *PrinterManager) RegisterEncoder(enc encoder.Encoder) {
	m.encoders[enc.Name()] = enc
}

func (m *PrinterManager) AvailableEncoders() map[string]encoder.Encoder {
	return m.encoders
}

// Start connects to all configured printers.
func (m *PrinterManager) Start() {
	for _, p := range m.store.AllPrinters() {
		if err := m.ConnectPrinter(p.ID); err != nil {
			webutil.LogError("auto-connect %s failed: %v", p.Name, err)
		}
	}
}

// Stop disconnects all printers.
func (m *PrinterManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, session := range m.sessions {
		webutil.LogInfo("disconnecting %s", session.config.Name)
		session.Stop()
		delete(m.sessions, id)
	}
}

// ConnectPrinter creates a session and connects to the printer.
func (m *PrinterManager) ConnectPrinter(printerID string) error {
	cfg := m.store.GetPrinter(printerID)
	if cfg == nil {
		return fmt.Errorf("printer not found: %s", printerID)
	}

	enc, ok := m.encoders[cfg.Encoder]
	if !ok {
		return fmt.Errorf("encoder not found: %s", cfg.Encoder)
	}

	tr := m.transportFn(cfg.Transport)
	if tr == nil {
		return fmt.Errorf("unknown transport: %s", cfg.Transport)
	}
	if webutil.TraceEnabled {
		tr = &transport.TraceTransport{Inner: tr}
	}

	// Stop existing session if any
	m.DisconnectPrinter(printerID)

	session := NewSession(*cfg, tr, enc, m.onStatusUpdate)

	m.mu.Lock()
	m.sessions[printerID] = session
	m.mu.Unlock()

	webutil.LogInfo("connecting to %s (%s/%s via %s)", cfg.Name, cfg.Encoder, cfg.Model, cfg.Transport)
	if err := session.Start(); err != nil {
		return fmt.Errorf("connect %s: %w", cfg.Name, err)
	}

	return nil
}

// DisconnectPrinter stops and removes a session.
func (m *PrinterManager) DisconnectPrinter(printerID string) {
	m.mu.Lock()
	session, ok := m.sessions[printerID]
	if ok {
		delete(m.sessions, printerID)
	}
	m.mu.Unlock()

	if ok {
		session.Stop()
	}
}

// GetStatus returns status for a single printer.
func (m *PrinterManager) GetStatus(printerID string) PrinterStatus {
	m.mu.RLock()
	session, ok := m.sessions[printerID]
	m.mu.RUnlock()
	if !ok {
		return PrinterStatus{Battery: -1, TotalLabels: -1, UsedLabels: -1}
	}
	return session.Status()
}

// AllStatuses returns statuses for all configured printers.
func (m *PrinterManager) AllStatuses() map[string]PrinterStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]PrinterStatus)
	for id, session := range m.sessions {
		result[id] = session.Status()
	}
	return result
}

// Print renders a label and sends it to the printer.
func (m *PrinterManager) Print(printerID string, data label.LabelData, templateName string) error {
	cfg := m.store.GetPrinter(printerID)
	if cfg == nil {
		return fmt.Errorf("printer not found: %s", printerID)
	}

	enc, ok := m.encoders[cfg.Encoder]
	if !ok {
		return fmt.Errorf("encoder not found: %s", cfg.Encoder)
	}

	var modelInfo *encoder.ModelInfo
	for _, mi := range enc.Models() {
		if mi.ID == cfg.Model {
			info := mi
			modelInfo = &info
			break
		}
	}
	if modelInfo == nil {
		return fmt.Errorf("model not found: %s", cfg.Model)
	}

	img, err := label.Render(data, templateName, modelInfo.PrintWidthPx, modelInfo.DPI)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	m.mu.RLock()
	session, ok := m.sessions[printerID]
	m.mu.RUnlock()

	if !ok || !session.Status().Connected {
		// No active session — try to connect first
		if err := m.ConnectPrinter(printerID); err != nil {
			return fmt.Errorf("connect for print: %w", err)
		}
		m.mu.RLock()
		session = m.sessions[printerID]
		m.mu.RUnlock()
	}

	webutil.LogInfo("printing on %s (%s/%s)", cfg.Name, cfg.Encoder, cfg.Model)
	opts := encoder.PrintOpts{
		Density:  modelInfo.DensityDefault,
		AutoCut:  true,
		Quantity: 1,
	}
	if err := session.Print(img, cfg.Model, opts); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	webutil.LogInfo("print complete on %s", cfg.Name)
	return nil
}

// SubscribeSSE returns a channel that receives status events.
func (m *PrinterManager) SubscribeSSE() chan PrinterStatusEvent {
	ch := make(chan PrinterStatusEvent, 16)
	m.sseMu.Lock()
	m.sseClients[ch] = struct{}{}
	m.sseMu.Unlock()
	return ch
}

// UnsubscribeSSE removes a SSE subscriber.
func (m *PrinterManager) UnsubscribeSSE(ch chan PrinterStatusEvent) {
	m.sseMu.Lock()
	delete(m.sseClients, ch)
	m.sseMu.Unlock()
	close(ch)
}

// onStatusUpdate is called by sessions when status changes.
func (m *PrinterManager) onStatusUpdate(printerID string, status PrinterStatus) {
	evt := PrinterStatusEvent{PrinterID: printerID, Status: status}
	m.sseMu.Lock()
	for ch := range m.sseClients {
		select {
		case ch <- evt:
		default:
			// drop if client is slow
		}
	}
	m.sseMu.Unlock()
}

func (m *PrinterManager) defaultTransportFactory(name string) transport.Transport {
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
