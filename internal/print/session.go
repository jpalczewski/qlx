package print

import (
	"context"
	"image"
	"sync"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// PrinterSession manages a persistent connection to a printer with heartbeat.
type PrinterSession struct {
	config   store.PrinterConfig
	tr       transport.Transport
	enc      encoder.Encoder
	querier  encoder.StatusQuerier // nil if encoder doesn't support status
	status   PrinterStatus
	mu       sync.Mutex
	stop     chan struct{}
	stopped  chan struct{}
	onUpdate func(printerID string, status PrinterStatus)
}

// NewSession creates a session. onUpdate is called when status changes.
// modelInfo provides DPI and print width for display.
func NewSession(cfg store.PrinterConfig, tr transport.Transport, enc encoder.Encoder, modelInfo *encoder.ModelInfo, onUpdate func(string, PrinterStatus)) *PrinterSession {
	var querier encoder.StatusQuerier
	if q, ok := enc.(encoder.StatusQuerier); ok {
		querier = q
	}
	st := PrinterStatus{Battery: -1, TotalLabels: -1, UsedLabels: -1}
	if modelInfo != nil {
		st.DPI = modelInfo.DPI
		st.PrintWidthMm = modelInfo.PrintWidthPx * 254 / (modelInfo.DPI * 10) // px * 25.4 / DPI
	}
	return &PrinterSession{
		config:   cfg,
		tr:       tr,
		enc:      enc,
		querier:  querier,
		status:   st,
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
		onUpdate: onUpdate,
	}
}

// Start opens the transport, sends connect, reads RFID, starts heartbeat goroutine.
func (s *PrinterSession) Start() error {
	if err := s.tr.Open(context.TODO(), s.config.Address); err != nil {
		s.updateStatus(func(st *PrinterStatus) {
			st.Connected = false
			st.LastError = err.Error()
		})
		close(s.stopped) // ensure Stop() never blocks on <-s.stopped
		return err
	}

	s.updateStatus(func(st *PrinterStatus) {
		st.Connected = true
		st.LastError = ""
	})

	if s.querier != nil {
		// Initial connect handshake
		if err := s.querier.Connect(context.TODO(), s.tr); err != nil {
			webutil.LogError("session %s: connect handshake failed: %v", s.config.Name, err)
		}

		// Read RFID info
		s.mu.Lock()
		rfid, err := s.querier.RfidInfo(context.TODO(), s.tr)
		s.mu.Unlock()
		if err != nil {
			webutil.LogTrace("session %s: rfid read failed: %v", s.config.Name, err)
		} else {
			s.updateStatus(func(st *PrinterStatus) {
				st.LabelType = rfid.LabelType
				st.TotalLabels = rfid.TotalLabels
				st.UsedLabels = rfid.UsedLabels
				st.LabelWidthMm = rfid.LabelWidthMm
				st.LabelHeightMm = rfid.LabelHeightMm
			})
		}

		// Start heartbeat
		go s.heartbeatLoop()
	} else {
		close(s.stopped)
	}

	return nil
}

// Stop stops heartbeat and closes transport.
func (s *PrinterSession) Stop() {
	select {
	case <-s.stop:
		return // already stopped
	default:
		close(s.stop)
	}
	// Close transport first to unblock any in-flight BLE I/O,
	// otherwise the heartbeat goroutine may hang on Read/Write
	// and <-s.stopped would block forever.
	_ = s.tr.Close()
	<-s.stopped
	s.updateStatus(func(st *PrinterStatus) {
		st.Connected = false
	})
}

// Status returns current printer status.
func (s *PrinterSession) Status() PrinterStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Print encodes and sends an image to the printer (locks mutex to pause heartbeat).
func (s *PrinterSession) Print(img image.Image, model string, opts encoder.PrintOpts) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enc.Encode(img, model, opts, s.tr)
}

func (s *PrinterSession) heartbeatLoop() {
	defer close(s.stopped)

	interval := 2 * time.Second
	if hc, ok := s.querier.(interface{ HeartbeatInterval() time.Duration }); ok {
		interval = hc.HeartbeatInterval()
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.mu.Lock()
			result, err := s.querier.Heartbeat(s.tr)
			s.mu.Unlock()

			if err != nil {
				webutil.LogTrace("session %s: heartbeat error: %v", s.config.Name, err)
				s.updateStatus(func(st *PrinterStatus) {
					st.Connected = false
					st.LastError = err.Error()
				})
				// Could add reconnect logic here in the future
				return
			}

			s.updateStatus(func(st *PrinterStatus) {
				st.Connected = true
				st.Battery = result.Battery
				st.LidClosed = result.LidClosed
				st.PaperLoaded = result.PaperLoaded
				st.LastError = ""
			})
		}
	}
}

func (s *PrinterSession) updateStatus(fn func(*PrinterStatus)) {
	s.mu.Lock()
	fn(&s.status)
	s.status.LastUpdated = time.Now()
	status := s.status
	s.mu.Unlock()

	if s.onUpdate != nil {
		s.onUpdate(s.config.ID, status)
	}
}
