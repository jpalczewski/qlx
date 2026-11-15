package print

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

// stallTransport simulates a USB printer that may stop answering: while
// stalled, Write and Read block for stallFor and then fail with
// transport.ErrTimeout, like a transfer hitting its deadline.
type stallTransport struct {
	stalled  atomic.Bool
	stallFor time.Duration
	entered  chan struct{} // receives a token each time a call starts stalling
	closes   atomic.Int32
	closed   chan struct{}
}

func newStallTransport(stallFor time.Duration) *stallTransport {
	return &stallTransport{stallFor: stallFor, entered: make(chan struct{}, 64), closed: make(chan struct{})}
}

func (s *stallTransport) Name() string                       { return "stall" }
func (s *stallTransport) Open(context.Context, string) error { return nil }
func (s *stallTransport) maybeStall(op string) error {
	if !s.stalled.Load() {
		return nil
	}
	select {
	case s.entered <- struct{}{}:
	default:
	}
	select {
	case <-time.After(s.stallFor):
		return fmt.Errorf("%s: %w", op, transport.ErrTimeout)
	case <-s.closed:
		return errors.New(op + ": closed")
	}
}
func (s *stallTransport) Write(p []byte) (int, error) {
	if err := s.maybeStall("write"); err != nil {
		return 0, err
	}
	return len(p), nil
}
func (s *stallTransport) Read(p []byte) (int, error) {
	if err := s.maybeStall("read"); err != nil {
		return 0, err
	}
	return copy(p, []byte{0x01}), nil
}
func (s *stallTransport) Close() error {
	if s.closes.Add(1) == 1 {
		close(s.closed)
	}
	return nil
}

// ioEncoder does real transport I/O in Heartbeat and Encode.
type ioEncoder struct {
	encodes atomic.Int32
}

func (e *ioEncoder) Name() string { return "mock" }
func (e *ioEncoder) Models() []encoder.ModelInfo {
	return []encoder.ModelInfo{{ID: "mock-model", DPI: 203, PrintWidthPx: 696}}
}
func (e *ioEncoder) Encode(_ image.Image, _ string, _ encoder.PrintOpts, tr transport.Transport) error {
	e.encodes.Add(1)
	_, err := tr.Write([]byte{0x01})
	return err
}
func (e *ioEncoder) Connect(context.Context, transport.Transport) error { return nil }
func (e *ioEncoder) RfidInfo(context.Context, transport.Transport) (encoder.RfidResult, error) {
	return encoder.RfidResult{}, nil
}
func (e *ioEncoder) HeartbeatInterval() time.Duration { return 2 * time.Millisecond }
func (e *ioEncoder) Heartbeat(tr transport.Transport) (encoder.HeartbeatResult, error) {
	if _, err := tr.Write([]byte{0x1b, 0x69, 0x53}); err != nil {
		return encoder.HeartbeatResult{}, fmt.Errorf("heartbeat: %w", err)
	}
	var buf [1]byte
	if _, err := tr.Read(buf[:]); err != nil {
		return encoder.HeartbeatResult{}, fmt.Errorf("heartbeat: %w", err)
	}
	return encoder.HeartbeatResult{Battery: -1, LidClosed: true, PaperLoaded: true}, nil
}

func setUnresponsiveLimit(t *testing.T, d time.Duration) {
	t.Helper()
	orig := unresponsiveLimit
	unresponsiveLimit = d
	t.Cleanup(func() { unresponsiveLimit = orig })
}

func startStallSession(t *testing.T, tr *stallTransport, enc *ioEncoder) *PrinterSession {
	t.Helper()
	s := NewSession(testPrinterCfg("p1"), tr, enc, nil, nil)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(s.Stop)
	return s
}

func TestSession_StatusNeverWaitsForIO(t *testing.T) {
	setUnresponsiveLimit(t, time.Hour)
	tr := newStallTransport(500 * time.Millisecond) // one stalled transfer outlasts the checks
	s := startStallSession(t, tr, &ioEncoder{})
	waitFor(t, "connected", func() bool { return s.Status().Connected })

	tr.stalled.Store(true)
	select {
	case <-tr.entered: // a heartbeat is now blocked inside the transport, holding the I/O lock
	case <-time.After(3 * time.Second):
		t.Fatal("heartbeat never reached the stalled transport")
	}
	for i := 0; i < 20; i++ {
		start := time.Now()
		_ = s.Status()
		if d := time.Since(start); d > 100*time.Millisecond {
			t.Fatalf("Status() took %v while a heartbeat was stalled", d)
		}
	}
}

func TestSession_TimeoutIsNonFatalAndRecovers(t *testing.T) {
	setUnresponsiveLimit(t, time.Hour)
	tr := newStallTransport(3 * time.Millisecond)
	s := startStallSession(t, tr, &ioEncoder{})
	waitFor(t, "connected", func() bool { return s.Status().Connected })

	tr.stalled.Store(true)
	waitFor(t, "not responding", func() bool { return s.Status().LastError == ErrPrinterNotResponding.Error() })
	if st := s.Status(); !st.Connected {
		t.Fatalf("session must stay Connected while timing out: %+v", st)
	}

	tr.stalled.Store(false)
	waitFor(t, "recovery", func() bool { return s.Status().LastError == "" })
	if st := s.Status(); !st.Connected {
		t.Fatalf("status after recovery = %+v", st)
	}
	select {
	case <-s.failed:
		t.Fatal("session ended on recoverable timeouts")
	default:
	}
	if got := tr.closes.Load(); got != 0 {
		t.Fatalf("transport closed %d times without a reconnect", got)
	}
}

func TestSession_ConsecutiveTimeoutsEndSession(t *testing.T) {
	setUnresponsiveLimit(t, 40*time.Millisecond)
	tr := newStallTransport(3 * time.Millisecond)
	s := startStallSession(t, tr, &ioEncoder{})
	waitFor(t, "connected", func() bool { return s.Status().Connected })

	tr.stalled.Store(true)
	select {
	case <-s.failed:
	case <-time.After(3 * time.Second):
		t.Fatal("session did not end after repeated timeouts")
	}
	st := s.Status()
	if st.Connected || st.LastError == "" {
		t.Fatalf("status after giving up = %+v", st)
	}
	if got := tr.closes.Load(); got != 1 {
		t.Fatalf("transport closed %d times, want 1", got)
	}
}

func TestConnectionManager_ReconnectsAfterUnresponsivePrinter(t *testing.T) {
	setUnresponsiveLimit(t, 40*time.Millisecond)
	var made atomic.Int32
	first := make(chan *stallTransport, 1)
	g := newGatedSleep()
	cm := newManager(t, func(string) transport.Transport {
		tr := newStallTransport(3 * time.Millisecond)
		if made.Add(1) == 1 {
			first <- tr
		}
		return tr
	}, &ioEncoder{})
	cm.sleep = g.sleep
	events := cm.Subscribe()
	if err := cm.Add(testPrinterCfg("p1")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	waitFor(t, "connected", func() bool { return cm.State("p1") == StateConnected })
	(<-first).stalled.Store(true)

	g.next(t) // session gave up; the loop waits to reconnect
	g.release <- struct{}{}

	var states []ConnState
	for connected := 0; connected < 2; {
		select {
		case ev := <-events:
			states = append(states, ev.State)
			if ev.State == StateConnected {
				connected++
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no reconnect; states=%v", states)
		}
	}
	want := []ConnState{StateConnected, StateDisconnected, StateReconnecting, StateConnecting, StateConnected}
	if !isSubsequence(want, states) {
		t.Fatalf("states = %v, want %v in order", states, want)
	}
}

func TestSession_PrintOnStalledPrinterReturnsError(t *testing.T) {
	setUnresponsiveLimit(t, time.Hour)
	tests := []struct {
		name string
		// waitRecorded: wait until the heartbeat reported the timeout first.
		waitRecorded bool
	}{
		{"after the timeout was recorded", true},
		{"while the heartbeat is still stalled", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := newStallTransport(30 * time.Millisecond)
			enc := &ioEncoder{}
			s := startStallSession(t, tr, enc)
			waitFor(t, "connected", func() bool { return s.Status().Connected })
			tr.stalled.Store(true)
			if tc.waitRecorded {
				waitFor(t, "not responding", func() bool { return s.Status().LastError == ErrPrinterNotResponding.Error() })
			} else {
				<-tr.entered
			}

			start := time.Now()
			err := s.Print(image.NewGray(image.Rect(0, 0, 8, 8)), "mock-model", encoder.PrintOpts{})
			if !errors.Is(err, ErrPrinterNotResponding) {
				t.Fatalf("Print = %v, want ErrPrinterNotResponding", err)
			}
			if d := time.Since(start); d > 2*time.Second {
				t.Fatalf("Print blocked for %v on a stalled printer", d)
			}
			if got := enc.encodes.Load(); got != 0 {
				t.Fatalf("Encode ran %d times on a stalled printer", got)
			}
		})
	}
}
