package print

import (
	"context"
	"errors"
	"image"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

// isSubsequence reports whether want appears in got in order (gaps allowed).
func isSubsequence(want, got []ConnState) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}
	return i == len(want)
}

// unwrapTracker returns the transport a session was created with; sessions wrap
// it in a failTrackingTransport.
func unwrapTracker(tr transport.Transport) transport.Transport {
	if w, ok := tr.(*failTrackingTransport); ok {
		return w.Transport
	}
	return tr
}

// gatedSleep replaces ConnectionManager.sleep: every call reports its delay on
// calls and then blocks until the test releases it, so tests assert on the
// requested delays and event order instead of elapsed time.
type gatedSleep struct {
	calls   chan time.Duration
	release chan struct{}
}

func newGatedSleep() *gatedSleep {
	return &gatedSleep{calls: make(chan time.Duration), release: make(chan struct{})}
}

func (g *gatedSleep) sleep(ctx context.Context, d time.Duration) bool {
	select {
	case g.calls <- d:
	case <-ctx.Done():
		return false
	}
	select {
	case <-g.release:
		return true
	case <-ctx.Done():
		return false
	}
}

// next waits for the next sleep request.
func (g *gatedSleep) next(t *testing.T) time.Duration {
	t.Helper()
	select {
	case d := <-g.calls:
		return d
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a reconnect sleep")
		return 0
	}
}

func newManager(t *testing.T, trFn TransportFactoryFn, enc encoder.Encoder) *ConnectionManager {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cm := NewConnectionManager(trFn, func(string) encoder.Encoder { return enc })
	cm.Start(ctx)
	t.Cleanup(func() { cancel(); cm.Stop() })
	return cm
}

func TestConnectionManager_DelayAfterDrop(t *testing.T) {
	const quick = 7 * time.Millisecond
	tests := []struct {
		name      string
		threshold time.Duration
		want      []time.Duration
	}{
		// Sessions never stay up long enough: delays grow and stay at the cap.
		{"flapping", time.Hour, []time.Duration{time.Hour, 2 * time.Hour, 2 * time.Hour}},
		// Every session counts as stable: quick retry, backoff reset each time.
		{"stable", time.Nanosecond, []time.Duration{quick, quick, quick}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			origSteps, origQuick, origStable := backoffSteps, quickRetryDelay, stableConnectionThreshold
			backoffSteps = []time.Duration{time.Hour, 2 * time.Hour}
			quickRetryDelay = quick
			stableConnectionThreshold = tc.threshold
			t.Cleanup(func() { backoffSteps, quickRetryDelay, stableConnectionThreshold = origSteps, origQuick, origStable })

			enc := &scriptedEncoder{hb: func(transport.Transport) (encoder.HeartbeatResult, error) {
				return encoder.HeartbeatResult{}, errFatalHeartbeat
			}}
			g := newGatedSleep()
			cm := newManager(t, func(string) transport.Transport { return &transport.MockTransport{} }, enc)
			cm.sleep = g.sleep
			if err := cm.Add(testPrinterCfg("p1")); err != nil {
				t.Fatalf("Add: %v", err)
			}
			for i, want := range tc.want {
				if got := g.next(t); got != want {
					t.Fatalf("delay #%d = %v, want %v", i+1, got, want)
				}
				g.release <- struct{}{}
			}
		})
	}
}

// noHeartbeatEncoder is an Encoder without StatusQuerier; Encode writes one byte.
type noHeartbeatEncoder struct{}

func (noHeartbeatEncoder) Name() string { return "mock" }
func (noHeartbeatEncoder) Models() []encoder.ModelInfo {
	return []encoder.ModelInfo{{ID: "mock-model", DPI: 203, PrintWidthPx: 696}}
}
func (noHeartbeatEncoder) Encode(_ image.Image, _ string, _ encoder.PrintOpts, tr transport.Transport) error {
	_, err := tr.Write([]byte{0x01})
	return err
}

func TestConnectionManager_NoHeartbeatSessionStaysUp(t *testing.T) {
	var made atomic.Int32
	var first atomic.Pointer[countingTransport]
	g := newGatedSleep()
	ctx, cancel := context.WithCancel(context.Background())
	cm := NewConnectionManager(
		func(string) transport.Transport {
			made.Add(1)
			tr := &countingTransport{}
			first.CompareAndSwap(nil, tr)
			return tr
		},
		func(string) encoder.Encoder { return noHeartbeatEncoder{} },
	)
	cm.sleep = g.sleep
	cm.Start(ctx)
	t.Cleanup(func() { cancel(); cm.Stop() })

	if err := cm.Add(testPrinterCfg("p1")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	waitFor(t, "connected", func() bool { return cm.State("p1") == StateConnected })

	// Shut down; a torn-down session would already have asked to sleep.
	cancel()
	cm.Stop()
	select {
	case d := <-g.calls:
		t.Fatalf("session was torn down and the loop asked to sleep %v", d)
	default:
	}
	if got := made.Load(); got != 1 {
		t.Fatalf("transports created = %d, want 1", got)
	}
	if got := first.Load().closes.Load(); got != 1 {
		t.Fatalf("closes after shutdown = %d, want 1", got)
	}
}

// failingWriteTransport fails every Write once broken is set.
type failingWriteTransport struct {
	countingTransport
	broken atomic.Bool
}

func (f *failingWriteTransport) Write(p []byte) (int, error) {
	if f.broken.Load() {
		return 0, errors.New("usb: write: no such device")
	}
	return f.countingTransport.Write(p)
}

func TestConnectionManager_NoHeartbeatPrintFailureReconnects(t *testing.T) {
	var made atomic.Int32
	var first atomic.Pointer[failingWriteTransport]
	g := newGatedSleep()
	cm := newManager(t, func(string) transport.Transport {
		tr := &failingWriteTransport{}
		if made.Add(1) == 1 {
			first.Store(tr)
		}
		return tr
	}, noHeartbeatEncoder{})
	cm.sleep = g.sleep
	events := cm.Subscribe()
	if err := cm.Add(testPrinterCfg("p1")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	waitFor(t, "connected", func() bool { return cm.State("p1") == StateConnected })

	// The cable is pulled: the next print fails on the transport.
	first.Load().broken.Store(true)
	img := image.NewGray(image.Rect(0, 0, 8, 8))
	if err := cm.Session("p1").Print(img, "mock-model", encoder.PrintOpts{}); err == nil {
		t.Fatal("Print on a dead transport succeeded")
	}
	g.next(t) // the loop noticed and waits to reconnect
	if got := first.Load().closes.Load(); got != 1 {
		t.Fatalf("dead transport closed %d times, want 1", got)
	}
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
	if got := made.Load(); got != 2 {
		t.Fatalf("transports created = %d, want 2", got)
	}
}

// slowOpenTransport blocks in Open until its context ends, then lingers a bit,
// tracking how many Opens run at once.
type slowOpenTransport struct {
	transport.MockTransport
	opens, active, maxActive *atomic.Int32
}

func (s *slowOpenTransport) Open(ctx context.Context, _ string) error {
	s.opens.Add(1)
	n := s.active.Add(1)
	for {
		m := s.maxActive.Load()
		if n <= m || s.maxActive.CompareAndSwap(m, n) {
			break
		}
	}
	<-ctx.Done()
	time.Sleep(60 * time.Millisecond) // slow to release, like a USB claim
	s.active.Add(-1)
	return ctx.Err()
}

func slowOpenManager(t *testing.T) (*ConnectionManager, *atomic.Int32, *atomic.Int32) {
	var opens, active, maxActive atomic.Int32
	enc := &scriptedEncoder{hb: func(transport.Transport) (encoder.HeartbeatResult, error) {
		return encoder.HeartbeatResult{}, nil
	}}
	cm := newManager(t, func(string) transport.Transport {
		return &slowOpenTransport{opens: &opens, active: &active, maxActive: &maxActive}
	}, enc)
	return cm, &opens, &maxActive
}

func TestConnectionManager_NewLoopWaitsForOldLoop(t *testing.T) {
	tests := []struct {
		name   string
		replay func(cm *ConnectionManager) error
	}{
		{"Reconnect", func(cm *ConnectionManager) error { return cm.Reconnect("p1") }},
		{"Remove then Add", func(cm *ConnectionManager) error {
			if err := cm.Remove("p1"); err != nil {
				return err
			}
			return cm.Add(testPrinterCfg("p1"))
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cm, opens, maxActive := slowOpenManager(t)
			if err := cm.Add(testPrinterCfg("p1")); err != nil {
				t.Fatalf("Add: %v", err)
			}
			waitFor(t, "first Open", func() bool { return opens.Load() == 1 })

			start := time.Now()
			if err := tc.replay(cm); err != nil {
				t.Fatalf("replay: %v", err)
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("replay blocked the caller")
			}
			waitFor(t, "second Open", func() bool { return opens.Load() == 2 })
			if got := maxActive.Load(); got != 1 {
				t.Fatalf("max concurrent Opens = %d, want 1 (old loop still held the device)", got)
			}
		})
	}
}

func TestConnectionManager_ReconnectHidesOldSession(t *testing.T) {
	enc := &scriptedEncoder{hb: func(transport.Transport) (encoder.HeartbeatResult, error) {
		return encoder.HeartbeatResult{LidClosed: true, PaperLoaded: true}, nil
	}}
	cm := newManager(t, func(string) transport.Transport { return &countingTransport{} }, enc)
	if err := cm.Add(testPrinterCfg("p1")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	waitFor(t, "connected", func() bool { return cm.State("p1") == StateConnected })
	old := cm.Session("p1")

	if err := cm.Reconnect("p1"); err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	if got := cm.Session("p1"); got == old {
		t.Fatal("cm.Session still returns the old generation's session right after Reconnect")
	}
	waitFor(t, "new session", func() bool {
		s := cm.Session("p1")
		if s == old {
			t.Fatal("old session reappeared")
		}
		return s != nil && cm.State("p1") == StateConnected
	})
	if old.Status().Connected {
		t.Error("old session still reports Connected")
	}
}

func TestConnectionManager_StoppedManagerRejectsLoops(t *testing.T) {
	enc := &scriptedEncoder{hb: func(transport.Transport) (encoder.HeartbeatResult, error) {
		return encoder.HeartbeatResult{}, nil
	}}
	cm := newManager(t, func(string) transport.Transport { return &transport.MockTransport{} }, enc)
	if err := cm.Add(testPrinterCfg("p1")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	cm.Stop()
	if err := cm.Add(testPrinterCfg("p2")); !errors.Is(err, errManagerStopped) {
		t.Fatalf("Add after Stop = %v, want errManagerStopped", err)
	}
	if err := cm.Reconnect("p1"); !errors.Is(err, errManagerStopped) {
		t.Fatalf("Reconnect after Stop = %v, want errManagerStopped", err)
	}
}

// openFailTransport fails Open after "acquiring" something that needs Close.
type openFailTransport struct {
	countingTransport
}

func (o *openFailTransport) Open(context.Context, string) error {
	return errors.New("libusb: claim interface failed")
}

func TestSession_StartFailureClosesTransport(t *testing.T) {
	tr := &openFailTransport{}
	s := NewSession(testPrinterCfg("p1"), tr, noHeartbeatEncoder{}, nil, nil)
	if err := s.Start(context.Background()); err == nil {
		t.Fatal("Start succeeded")
	}
	s.Stop()
	if got := tr.closes.Load(); got != 1 {
		t.Fatalf("closes = %d, want exactly 1", got)
	}
}
