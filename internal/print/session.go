package print

import (
	"context"
	"errors"
	"fmt"
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
	config  store.PrinterConfig
	tr      transport.Transport
	track   *failTrackingTransport // wraps the transport given to NewSession
	enc     encoder.Encoder
	querier encoder.StatusQuerier // nil if encoder doesn't support status
	status  PrinterStatus
	// mu serialises transport I/O (heartbeat, Print, RFID read). It may be held
	// for as long as a USB transfer takes; Status() never takes it.
	mu sync.Mutex
	// stMu guards status, condition and the timeout counters. It is held only
	// briefly and never during I/O, so status readers never wait for the printer.
	// Lock order: mu, then stMu.
	stMu         sync.Mutex
	condition    *encoder.StatusError // active printer status condition; nil when clean
	timeouts     int                  // consecutive heartbeat I/O timeouts
	firstTimeout time.Time            // start of the current run of timeouts
	stop         chan struct{}
	stopped      chan struct{}
	failed       chan struct{} // closed when the session ends by itself (see fail)
	failOnce     sync.Once
	stopOnce     sync.Once
	closeOnce    sync.Once
	closed       atomic.Bool // set once the transport is closed
	onUpdate     func(printerID string, status PrinterStatus)
}

// unresponsiveLimit is how long consecutive heartbeat I/O timeouts are tolerated
// (reported as ErrPrinterNotResponding, session kept) before the session is
// treated as lost so the ConnectionManager reconnects. A var so tests can
// shorten it. A single timeout never ends a session.
var unresponsiveLimit = 30 * time.Second

// ErrPrinterNotResponding is reported (as LastError, and by Print) while the
// printer does not answer status requests in time. The link is still open.
var ErrPrinterNotResponding = errors.New("printer not responding")

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

// releaseTransport closes the transport under the I/O lock s.mu, so it cannot run
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

// Status returns the current printer status. It only takes the short status
// lock, so it never waits for USB I/O.
func (s *PrinterSession) Status() PrinterStatus {
	s.stMu.Lock()
	defer s.stMu.Unlock()
	return s.status
}

// Print encodes and sends an image to the printer (holds the I/O lock to pause
// the heartbeat).
//
// For status-capable encoders it first takes a fresh status under the I/O
// lock. If a printer condition (cover open, media mismatch, ...) is reported,
// it re-initialises the printer when the encoder supports it (sticky error bits
// such as a media mismatch are only cleared that way) and queries again. Print
// refuses with the *encoder.StatusError only if the condition persists, and
// with ErrPrinterNotResponding if the printer does not answer the status query.
func (s *PrinterSession) Print(img image.Image, model string, opts encoder.PrintOpts) error {
	return s.PrintPages([]image.Image{img}, model, opts)
}

// PrintPages prints a sequence of pages as one set, opts.Copies times, with
// the pages of a set adjacent. opts.CutEvery counts sets. Encoders that
// implement encoder.PagedEncoder print it as one job (one cut per set);
// other encoders are driven page by page without cuts. It has the same
// preflight and failure semantics as Print.
func (s *PrinterSession) PrintPages(pages []image.Image, model string, opts encoder.PrintOpts) error {
	if len(pages) == 0 {
		return errors.New("no pages to print")
	}
	if s.notResponding() {
		return ErrPrinterNotResponding
	}
	s.mu.Lock()
	if s.closed.Load() {
		s.mu.Unlock()
		return ErrPrinterDisconnected
	}
	if s.querier != nil {
		if st, changed, err := s.preflightLocked(); err != nil {
			s.mu.Unlock()
			if changed {
				s.notify(st)
			}
			return err
		}
	}
	s.track.failed.Store(false)
	err := s.encodePagesLocked(pages, model, opts)
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

// encodePagesLocked sends pages to the encoder. s.mu must be held.
func (s *PrinterSession) encodePagesLocked(pages []image.Image, model string, opts encoder.PrintOpts) error {
	if pe, ok := s.enc.(encoder.PagedEncoder); ok {
		return pe.EncodePages(pages, model, opts, s.tr)
	}
	if len(pages) == 1 {
		return s.enc.Encode(pages[0], model, opts, s.tr)
	}
	// No paged job support (and so no cutter): one job per page, the set
	// repeated Copies times in order.
	single := opts
	single.Copies = 1
	single.CutEvery = 0
	for range max(opts.Copies, 1) {
		for _, pg := range pages {
			if err := s.enc.Encode(pg, model, single, s.tr); err != nil {
				return err
			}
		}
	}
	return nil
}

// notResponding reports whether the heartbeat is currently timing out.
func (s *PrinterSession) notResponding() bool {
	s.stMu.Lock()
	defer s.stMu.Unlock()
	return s.timeouts > 0
}

// preflightLocked takes a fresh printer status before printing. s.mu must be
// held. It returns the active status condition (after one re-initialisation
// attempt), ErrPrinterNotResponding on an I/O timeout, another transport error
// from the query itself, or nil when printing may proceed. The status snapshot
// and whether subscribers must be notified are returned for the caller to
// publish after releasing s.mu.
func (s *PrinterSession) preflightLocked() (PrinterStatus, bool, error) {
	result, err := s.querier.Heartbeat(s.tr)
	var statusErr *encoder.StatusError
	if errors.As(err, &statusErr) {
		if r, ok := s.enc.(encoder.Reinitializer); ok {
			if rerr := r.Reinitialize(s.tr); rerr != nil {
				return s.Status(), false, fmt.Errorf("reinitialize printer: %w", rerr)
			}
			result, err = s.querier.Heartbeat(s.tr)
		}
	}
	if err != nil && !errors.As(err, &statusErr) {
		if errors.Is(err, transport.ErrTimeout) {
			// The heartbeat owns the decision to give up on the session.
			st, changed, _ := s.recordTimeout()
			return st, changed, fmt.Errorf("%w (%v)", ErrPrinterNotResponding, err)
		}
		return s.Status(), false, fmt.Errorf("printer status: %w", err)
	}
	st, changed := s.recordHeartbeat(result, err)
	if cond := s.activeCondition(); cond != nil {
		return st, changed, cond
	}
	return st, changed, nil
}

func (s *PrinterSession) activeCondition() *encoder.StatusError {
	s.stMu.Lock()
	defer s.stMu.Unlock()
	return s.condition
}

// recordHeartbeat applies a successful heartbeat exchange to the status: err is
// nil (clean) or an *encoder.StatusError. It clears any timeout run. It reports
// whether subscribers must be notified (the status changed).
func (s *PrinterSession) recordHeartbeat(result encoder.HeartbeatResult, err error) (PrinterStatus, bool) {
	var statusErr *encoder.StatusError
	errors.As(err, &statusErr)
	lastError := ""
	if statusErr != nil {
		lastError = statusErr.Error()
	}

	s.stMu.Lock()
	defer s.stMu.Unlock()
	prev := s.status
	s.condition = statusErr
	s.timeouts, s.firstTimeout = 0, time.Time{}
	s.status.Connected = true
	s.status.Battery = result.Battery
	s.status.LidClosed = result.LidClosed
	s.status.PaperLoaded = result.PaperLoaded
	s.status.LastError = lastError
	return s.stampIfChangedLocked(prev, lastError)
}

// recordTimeout notes one I/O timeout of a status request. Until
// unresponsiveLimit has elapsed since the first timeout of the run (and at
// least two timeouts happened) the session stays Connected with LastError set
// to ErrPrinterNotResponding; fatal is true afterwards.
func (s *PrinterSession) recordTimeout() (st PrinterStatus, changed, fatal bool) {
	s.stMu.Lock()
	defer s.stMu.Unlock()
	prev := s.status
	now := time.Now()
	if s.timeouts == 0 {
		s.firstTimeout = now
	}
	s.timeouts++
	s.condition = nil
	fatal = s.timeouts >= 2 && now.Sub(s.firstTimeout) >= unresponsiveLimit
	if !fatal {
		s.status.Connected = true
		s.status.LastError = ErrPrinterNotResponding.Error()
	}
	st, changed = s.stampIfChangedLocked(prev, s.status.LastError)
	return st, changed, fatal
}

// stampIfChangedLocked refreshes LastUpdated and traces when the status differs
// from prev. s.stMu must be held.
func (s *PrinterSession) stampIfChangedLocked(prev PrinterStatus, lastError string) (PrinterStatus, bool) {
	if s.status == prev {
		return s.status, false
	}
	s.status.LastUpdated = time.Now()
	if lastError != prev.LastError {
		webutil.LogTrace("session %s: printer status: %q", s.config.Name, lastError)
	}
	return s.status, true
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
			if !s.heartbeatOnce() {
				return
			}
		}
	}
}

// heartbeatOnce runs one heartbeat and reports whether the session continues.
// A printer condition (*encoder.StatusError) or an isolated I/O timeout keeps
// the session alive; protocol errors and a run of timeouts longer than
// unresponsiveLimit end it.
func (s *PrinterSession) heartbeatOnce() bool {
	st, changed, fatal := s.heartbeatExchange()
	if changed {
		s.notify(st)
	}
	if fatal == nil {
		return true
	}

	webutil.LogTrace("session %s: heartbeat error: %v", s.config.Name, fatal)
	s.updateStatus(func(st *PrinterStatus) {
		st.Connected = false
		st.LastError = fatal.Error()
		s.condition = nil // a dead link supersedes any printer condition
	})
	// Release the transport now; the ConnectionManager owns reconnect.
	s.releaseTransport()
	s.fail()
	return false
}

// heartbeatExchange performs one status request under the I/O lock and records
// the outcome. The lock is released before it returns, also on a timeout, so a
// stalled printer never blocks Print or Stop beyond the transport's I/O timeout.
func (s *PrinterSession) heartbeatExchange() (PrinterStatus, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.querier.Heartbeat(s.tr)
	var statusErr *encoder.StatusError
	switch {
	case err == nil || errors.As(err, &statusErr):
		// Recorded before the I/O lock is released, so Print never sees a
		// condition that lags behind the printer.
		st, changed := s.recordHeartbeat(result, err)
		return st, changed, nil
	case errors.Is(err, transport.ErrTimeout):
		st, changed, fatal := s.recordTimeout()
		if fatal {
			return st, changed, fmt.Errorf("printer not responding: %w", err)
		}
		return st, changed, nil
	}
	return PrinterStatus{}, false, err
}

func (s *PrinterSession) updateStatus(fn func(*PrinterStatus)) {
	s.stMu.Lock()
	fn(&s.status)
	s.status.LastUpdated = time.Now()
	status := s.status
	s.stMu.Unlock()
	s.notify(status)
}

func (s *PrinterSession) notify(status PrinterStatus) {
	if s.onUpdate != nil {
		s.onUpdate(s.config.ID, status)
	}
}
