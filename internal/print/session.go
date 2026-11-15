package print

import (
	"context"
	"errors"
	"image"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// ErrPrinterDisconnected is returned by Print when the session has ended and
// its transport is already closed.
var ErrPrinterDisconnected = errors.New("printer disconnected")

// failTrackingTransport remembers whether a Write or Read failed, so Print can
// tell a transport failure from an encoding error.
type failTrackingTransport struct {
	transport.Transport
	failed atomic.Bool
}

func (f *failTrackingTransport) Write(p []byte) (int, error) {
	n, err := f.Transport.Write(p)
	if err != nil {
		f.failed.Store(true)
	}
	return n, err
}

func (f *failTrackingTransport) Read(p []byte) (int, error) {
	n, err := f.Transport.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		f.failed.Store(true)
	}
	return n, err
}

// PrinterSession manages a persistent connection to a printer with heartbeat.
type PrinterSession struct {
	config    store.PrinterConfig
	tr        transport.Transport
	track     *failTrackingTransport // wraps the transport given to NewSession
	enc       encoder.Encoder
	querier   encoder.StatusQuerier // nil if encoder doesn't support status
	status    PrinterStatus
	mu        sync.Mutex
	stop      chan struct{}
	stopped   chan struct{}
	failed    chan struct{} // closed when the session ends by itself (see fail)
	failOnce  sync.Once
	stopOnce  sync.Once
	closeOnce sync.Once
	closed    atomic.Bool // set once the transport is closed
	onUpdate  func(printerID string, status PrinterStatus)
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
	track := &failTrackingTransport{Transport: tr}
	return &PrinterSession{
		config:   cfg,
		tr:       track,
		track:    track,
		failed:   make(chan struct{}),
		enc:      enc,
		querier:  querier,
		status:   st,
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
		onUpdate: onUpdate,
	}
}

// Start opens the transport, sends connect, reads RFID, starts heartbeat goroutine.
func (s *PrinterSession) Start(ctx context.Context) error {
	if err := s.tr.Open(ctx, s.config.Address); err != nil {
		s.updateStatus(func(st *PrinterStatus) {
			st.Connected = false
			st.LastError = err.Error()
		})
		// Open may have acquired resources before failing; release them.
		s.closeTransport()
		close(s.stopped) // ensure Stop() never blocks on <-s.stopped
		return err
	}

	s.updateStatus(func(st *PrinterStatus) {
		st.Connected = true
		st.LastError = ""
	})

	if s.querier != nil {
		// Initial connect handshake
		if err := s.querier.Connect(ctx, s.tr); err != nil {
			webutil.LogError("session %s: connect handshake failed: %v", s.config.Name, err)
		}

		// Read RFID info
		s.mu.Lock()
		rfid, err := s.querier.RfidInfo(ctx, s.tr)
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

// Stop stops heartbeat and closes transport. Safe to call concurrently.
func (s *PrinterSession) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)
		// Close transport first to unblock any in-flight BLE I/O,
		// otherwise the heartbeat goroutine may hang on Read/Write
		// and <-s.stopped would block forever.
		s.closeTransport()
		<-s.stopped
		s.updateStatus(func(st *PrinterStatus) {
			st.Connected = false
		})
	})
}

// closeTransport closes the underlying transport exactly once, whether the
// session ends via Stop() or via a fatal heartbeat error. Releasing it on both
// paths is what frees the claimed USB interface for the next session.
func (s *PrinterSession) closeTransport() {
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		_ = s.tr.Close()
	})
}

// releaseTransport closes the transport under s.mu, so it cannot run
// concurrently with an in-flight Print/Encode on the same transport.
func (s *PrinterSession) releaseTransport() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeTransport()
}

// fail signals the owner (ConnectionManager) that the session ended by itself.
func (s *PrinterSession) fail() {
	s.failOnce.Do(func() { close(s.failed) })
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
	if s.closed.Load() {
		s.mu.Unlock()
		return ErrPrinterDisconnected
	}
	s.track.failed.Store(false)
	err := s.enc.Encode(img, model, opts, s.tr)
	// Without a heartbeat nothing else notices a dead link, so a transport
	// failure while printing ends the session and the manager reconnects.
	broke := err != nil && s.querier == nil && s.track.failed.Load()
	if broke {
		s.closeTransport()
	}
	s.mu.Unlock()

	if broke {
		s.updateStatus(func(st *PrinterStatus) {
			st.Connected = false
			st.LastError = err.Error()
		})
		s.fail()
	}
	return err
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
				// Release the transport now; the ConnectionManager owns reconnect.
				s.releaseTransport()
				s.fail()
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
