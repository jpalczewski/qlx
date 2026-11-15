package print

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/print/transport"
)

var errNoDevice = errors.New("gousb write: libusb: no device [code -4]")

// idleLinkTransport simulates a USB link that went stale while idle: status
// exchanges (3-byte writes) work until failStatus is set, and the Nth job
// write (1-byte writes) fails with failErr. jobWrites counts job writes that
// reached the "device", including the failing one.
type idleLinkTransport struct {
	failAt     int32 // 1-based job write index that fails; 0 = never
	failErr    error
	failStatus atomic.Pointer[error] // heartbeat write error
	jobWrites  atomic.Int32
	closes     atomic.Int32
}

func (t *idleLinkTransport) Name() string                       { return "idle" }
func (t *idleLinkTransport) Open(context.Context, string) error { return nil }
func (t *idleLinkTransport) Close() error                       { t.closes.Add(1); return nil }
func (t *idleLinkTransport) Read(p []byte) (int, error)         { return copy(p, []byte{1}), nil }
func (t *idleLinkTransport) Write(p []byte) (int, error) {
	if len(p) == 3 { // status request
		if e := t.failStatus.Load(); e != nil {
			return 0, *e
		}
		return len(p), nil
	}
	n := t.jobWrites.Add(1)
	if t.failAt != 0 && n == t.failAt {
		return 0, t.failErr
	}
	return len(p), nil
}

// chunkEncoder issues `chunks` small writes per job, like the Brother raster
// encoder. With ignoreErrors it keeps writing after a failure, to prove the
// session aborts the job anyway.
type chunkEncoder struct {
	chunks       int
	ignoreErrors bool
	statusErr    error // returned from the job after the first write, if set
	hbStatus     error // returned from Heartbeat, if set
}

func (e *chunkEncoder) Name() string { return "mock" }
func (e *chunkEncoder) Models() []encoder.ModelInfo {
	return []encoder.ModelInfo{{ID: "mock-model", DPI: 203, PrintWidthPx: 696}}
}
func (e *chunkEncoder) Encode(_ image.Image, _ string, _ encoder.PrintOpts, tr transport.Transport) error {
	return e.EncodePages(nil, "", encoder.PrintOpts{}, tr)
}
func (e *chunkEncoder) EncodePages(_ []image.Image, _ string, _ encoder.PrintOpts, tr transport.Transport) error {
	var first error
	for i := 0; i < e.chunks; i++ {
		if _, err := tr.Write([]byte{0x01}); err != nil {
			if !e.ignoreErrors {
				return err
			}
			if first == nil {
				first = err
			}
		}
		if i == 0 && e.statusErr != nil {
			return e.statusErr
		}
	}
	return first
}
func (e *chunkEncoder) Connect(context.Context, transport.Transport) error { return nil }
func (e *chunkEncoder) RfidInfo(context.Context, transport.Transport) (encoder.RfidResult, error) {
	return encoder.RfidResult{}, nil
}
func (e *chunkEncoder) HeartbeatInterval() time.Duration { return time.Hour } // preflight only
func (e *chunkEncoder) Heartbeat(tr transport.Transport) (encoder.HeartbeatResult, error) {
	if _, err := tr.Write([]byte{0x1b, 0x69, 0x53}); err != nil {
		return encoder.HeartbeatResult{}, fmt.Errorf("heartbeat: %w", err)
	}
	var b [1]byte
	if _, err := tr.Read(b[:]); err != nil {
		return encoder.HeartbeatResult{}, fmt.Errorf("heartbeat: %w", err)
	}
	return encoder.HeartbeatResult{Battery: -1, LidClosed: true, PaperLoaded: true}, e.hbStatus
}

func startIdleSession(t *testing.T, tr *idleLinkTransport, enc *chunkEncoder) *PrinterSession {
	t.Helper()
	s := NewSession(testPrinterCfg("p1"), tr, enc, nil, nil)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(s.Stop)
	return s
}

// printWithin fails the test if PrintPages does not return in time.
func printWithin(t *testing.T, s *PrinterSession) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- s.PrintPages([]image.Image{image.NewRGBA(image.Rect(0, 0, 2, 2))}, "mock-model", encoder.PrintOpts{Copies: 1})
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("print hung")
		return nil
	}
}

func sessionFailed(s *PrinterSession) bool {
	select {
	case <-s.failed:
		return true
	default:
		return false
	}
}

func TestPrintPages_WriteFailureAbortsAndTearsDown(t *testing.T) {
	timeout := fmt.Errorf("gousb write: %w", transport.ErrTimeout)
	tests := []struct {
		name          string
		failAt        int32
		failErr       error
		wantRetryable bool
	}{
		{"first write timeout", 1, timeout, true},
		{"first write no device", 1, errNoDevice, true},
		{"mid-job write timeout", 5, timeout, false},
		{"mid-job no device", 5, errNoDevice, false},
	}
	for _, tt := range tests {
		for _, ignore := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/ignoreErrors=%v", tt.name, ignore), func(t *testing.T) {
				tr := &idleLinkTransport{failAt: tt.failAt, failErr: tt.failErr}
				enc := &chunkEncoder{chunks: 300, ignoreErrors: ignore}
				s := startIdleSession(t, tr, enc)

				err := printWithin(t, s)
				var le *LinkError
				if !errors.As(err, &le) {
					t.Fatalf("err = %v, want *LinkError", err)
				}
				if !errors.Is(err, ErrLinkLost) || !errors.Is(err, tt.failErr) {
					t.Errorf("err %v must match ErrLinkLost and the cause", err)
				}
				if le.Retryable != tt.wantRetryable {
					t.Errorf("Retryable = %v, want %v", le.Retryable, tt.wantRetryable)
				}
				if got := tr.jobWrites.Load(); got != tt.failAt {
					t.Errorf("device saw %d job writes, want job aborted at the failing write (%d)", got, tt.failAt)
				}
				if !sessionFailed(s) || !s.closed.Load() || tr.closes.Load() != 1 {
					t.Errorf("session must be failed and closed once: failed=%v closed=%v closes=%d",
						sessionFailed(s), s.closed.Load(), tr.closes.Load())
				}
				if st := s.Status(); st.Connected {
					t.Errorf("status still connected: %+v", st)
				}
				if err := printWithin(t, s); !errors.Is(err, ErrPrinterDisconnected) {
					t.Errorf("second print err = %v, want ErrPrinterDisconnected", err)
				}
			})
		}
	}
}

func TestPrintPages_PreflightNoDeviceTearsDownRetryable(t *testing.T) {
	tr := &idleLinkTransport{}
	s := startIdleSession(t, tr, &chunkEncoder{chunks: 10})
	tr.failStatus.Store(&errNoDevice)

	err := printWithin(t, s)
	var le *LinkError
	if !errors.As(err, &le) || !le.Retryable {
		t.Fatalf("err = %v, want retryable *LinkError", err)
	}
	if tr.jobWrites.Load() != 0 {
		t.Errorf("job writes = %d, want 0", tr.jobWrites.Load())
	}
	if !sessionFailed(s) || tr.closes.Load() != 1 {
		t.Errorf("session must be torn down: failed=%v closes=%d", sessionFailed(s), tr.closes.Load())
	}
}

func TestPrintPages_PreflightTimeoutKeepsLink(t *testing.T) {
	setUnresponsiveLimit(t, time.Hour)
	tr := &idleLinkTransport{}
	s := startIdleSession(t, tr, &chunkEncoder{chunks: 10})
	timeout := fmt.Errorf("write: %w", transport.ErrTimeout)
	tr.failStatus.Store(&timeout)

	err := printWithin(t, s)
	if !errors.Is(err, ErrPrinterNotResponding) {
		t.Fatalf("err = %v, want ErrPrinterNotResponding", err)
	}
	if sessionFailed(s) || tr.closes.Load() != 0 {
		t.Error("a single status timeout must not end the session (heartbeat decides)")
	}
}

func TestPrintPages_StatusConditionKeepsLink(t *testing.T) {
	cond := &encoder.StatusError{Source: "mock", Conditions: []string{"cover open"}}
	tests := []struct {
		name string
		enc  *chunkEncoder
	}{
		{"from encode", &chunkEncoder{chunks: 10, statusErr: cond}},
		{"from preflight", &chunkEncoder{chunks: 10, hbStatus: cond}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &idleLinkTransport{}
			s := startIdleSession(t, tr, tt.enc)
			err := printWithin(t, s)
			var se *encoder.StatusError
			if !errors.As(err, &se) {
				t.Fatalf("err = %v, want *StatusError", err)
			}
			if sessionFailed(s) || s.closed.Load() || tr.closes.Load() != 0 {
				t.Error("a printer condition must not tear down the link")
			}
		})
	}
}

// TestSession_StopInterruptsBlockedPrint: a write that never returns on its own
// is released by Stop closing the transport, so shutdown does not hang.
func TestSession_StopInterruptsBlockedPrint(t *testing.T) {
	setUnresponsiveLimit(t, time.Hour)
	tr := newStallTransport(time.Hour)
	s := startStallSession(t, tr, &ioEncoder{})
	waitFor(t, "connected", func() bool { return s.Status().Connected })

	tr.stalled.Store(true)
	printDone := make(chan error, 1)
	go func() {
		printDone <- s.Print(image.NewRGBA(image.Rect(0, 0, 2, 2)), "mock-model", encoder.PrintOpts{})
	}()
	select {
	case <-tr.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("print never reached the transport")
	}

	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop hung behind a blocked print")
	}
	select {
	case <-printDone:
	case <-time.After(3 * time.Second):
		t.Fatal("blocked print was not interrupted by Stop")
	}
}

// retryRig is a manager whose first connection has a stale link and whose
// reconnects are healthy.
type retryRig struct {
	mgr   *PrinterManager
	cm    *ConnectionManager
	made  atomic.Int32
	enc   *chunkEncoder
	pid   string
	mu    sync.Mutex
	links []*idleLinkTransport
}

func (r *retryRig) link(i int) *idleLinkTransport {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i < len(r.links) {
		return r.links[i]
	}
	return nil
}

func newRetryRig(t *testing.T, failAt int32, failErr error) *retryRig {
	t.Helper()
	oldQuick, oldWait, oldStable := quickRetryDelay, reconnectWait, stableConnectionThreshold
	quickRetryDelay, reconnectWait, stableConnectionThreshold = 5*time.Millisecond, 3*time.Second, 0
	t.Cleanup(func() { quickRetryDelay, reconnectWait, stableConnectionThreshold = oldQuick, oldWait, oldStable })

	r := &retryRig{enc: &chunkEncoder{chunks: 20}}
	cm := NewConnectionManager(func(string) transport.Transport {
		tr := &idleLinkTransport{}
		if r.made.Add(1) == 1 {
			tr.failAt, tr.failErr = failAt, failErr
		}
		r.mu.Lock()
		r.links = append(r.links, tr)
		r.mu.Unlock()
		return tr
	}, func(string) encoder.Encoder { return r.enc })
	ctx, cancel := context.WithCancel(context.Background())
	cm.Start(ctx)
	t.Cleanup(func() { cancel(); cm.Stop() })

	st := newTestPrintStore(t)
	mgr := NewPrinterManager(st, cm)
	mgr.RegisterEncoder(r.enc)
	mgr.renderer = &spyRenderer{}
	p := st.AddPrinter("P", "mock", "mock-model", "mock", "/dev/null")
	if err := cm.Add(*p); err != nil {
		t.Fatal(err)
	}
	if !waitForState(cm, p.ID, StateConnected, 2*time.Second) {
		t.Fatal("not connected")
	}
	r.mgr, r.cm, r.pid = mgr, cm, p.ID
	return r
}

func (r *retryRig) print(t *testing.T) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- r.mgr.Print(r.pid, label.LabelData{Name: "x"}, "simple", label.RenderOpts{}, encoder.PrintOpts{})
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("print hung")
		return nil
	}
}

func TestManagerPrint_RetriesOnceAfterStaleLinkOnFirstWrite(t *testing.T) {
	r := newRetryRig(t, 1, fmt.Errorf("gousb write: %w", transport.ErrTimeout))
	if err := r.print(t); err != nil {
		t.Fatalf("Print: %v", err)
	}
	first, second := r.link(0), r.link(1)
	if second == nil {
		t.Fatal("no reconnect happened")
	}
	if first.closes.Load() != 1 {
		t.Errorf("stale transport closes = %d, want 1", first.closes.Load())
	}
	if got := first.jobWrites.Load(); got != 1 {
		t.Errorf("stale link saw %d job writes, want 1", got)
	}
	if got := second.jobWrites.Load(); int(got) != r.enc.chunks {
		t.Errorf("retry wrote %d chunks on the new link, want %d", got, r.enc.chunks)
	}
}

func TestManagerPrint_NoRetryAfterDataMayHaveBeenConsumed(t *testing.T) {
	r := newRetryRig(t, 7, errNoDevice)
	err := r.print(t)
	var le *LinkError
	if !errors.As(err, &le) || le.Retryable {
		t.Fatalf("err = %v, want non-retryable *LinkError", err)
	}
	if !errors.Is(err, ErrLinkLost) {
		t.Errorf("err must match ErrLinkLost: %v", err)
	}
	// The session still reconnects for the next print, but the job was not repeated.
	waitFor(t, "reconnect", func() bool { return r.link(1) != nil })
	if got := r.link(1).jobWrites.Load(); got != 0 {
		t.Errorf("new link got %d job writes; the job must not be repeated", got)
	}
}

func TestManagerPrint_RetryGivesUpWhenNoReconnect(t *testing.T) {
	r := newRetryRig(t, 1, errNoDevice)
	reconnectWait = 50 * time.Millisecond
	r.cm.mu.Lock()
	r.cm.transportFn = func(string) transport.Transport { return nil } // reconnects fail
	r.cm.mu.Unlock()
	err := r.print(t)
	if !errors.Is(err, ErrLinkLost) {
		t.Fatalf("err = %v, want the link error returned to the caller", err)
	}
}
