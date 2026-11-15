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
	"github.com/erxyi/qlx/internal/print/transport"
)

// scriptedEncoder is an Encoder + StatusQuerier whose heartbeat results are
// driven by a callback receiving the session's transport, so tests can key
// behaviour per session instead of on a shared counter.
type scriptedEncoder struct {
	hb    func(tr transport.Transport) (encoder.HeartbeatResult, error)
	count atomic.Int32
}

func (e *scriptedEncoder) Name() string { return "mock" }
func (e *scriptedEncoder) Models() []encoder.ModelInfo {
	return []encoder.ModelInfo{{ID: "mock-model", DPI: 203, PrintWidthPx: 696}}
}
func (e *scriptedEncoder) Encode(_ image.Image, _ string, _ encoder.PrintOpts, tr transport.Transport) error {
	_, err := tr.Write([]byte{0x01})
	return err
}
func (e *scriptedEncoder) Connect(context.Context, transport.Transport) error { return nil }
func (e *scriptedEncoder) RfidInfo(context.Context, transport.Transport) (encoder.RfidResult, error) {
	return encoder.RfidResult{}, nil
}
func (e *scriptedEncoder) HeartbeatInterval() time.Duration { return 5 * time.Millisecond }
func (e *scriptedEncoder) Heartbeat(tr transport.Transport) (encoder.HeartbeatResult, error) {
	e.count.Add(1)
	return e.hb(tr)
}

// countingTransport counts Close calls.
type countingTransport struct {
	transport.MockTransport
	closes atomic.Int32
}

func (c *countingTransport) Close() error {
	c.closes.Add(1)
	return nil
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

var errFatalHeartbeat = errors.New("usb: device unplugged")

func TestSession_ClosesTransportExactlyOnce(t *testing.T) {
	tests := []struct {
		name string
		// fatal: the heartbeat fails (ending the session) before Stop() is called.
		fatal bool
	}{
		{name: "fatal heartbeat error then Stop", fatal: true},
		{name: "Stop only", fatal: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := &countingTransport{}
			enc := &scriptedEncoder{hb: func(transport.Transport) (encoder.HeartbeatResult, error) {
				if tc.fatal {
					return encoder.HeartbeatResult{}, errFatalHeartbeat
				}
				return encoder.HeartbeatResult{LidClosed: true, PaperLoaded: true}, nil
			}}
			s := NewSession(testPrinterCfg("p1"), tr, enc, nil, nil)
			if err := s.Start(context.Background()); err != nil {
				t.Fatalf("Start: %v", err)
			}

			if tc.fatal {
				<-s.stopped
				if got := tr.closes.Load(); got != 1 {
					t.Fatalf("closes after fatal error = %d, want 1", got)
				}
				if st := s.Status(); st.Connected || st.LastError != errFatalHeartbeat.Error() {
					t.Fatalf("status = %+v, want disconnected with LastError", st)
				}
				img := image.NewGray(image.Rect(0, 0, 8, 8))
				if err := s.Print(img, "mock-model", encoder.PrintOpts{}); !errors.Is(err, ErrPrinterDisconnected) {
					t.Fatalf("Print after fatal error = %v, want ErrPrinterDisconnected", err)
				}
				if len(tr.Written) != 0 {
					t.Fatalf("Print wrote %d bytes to a closed transport", len(tr.Written))
				}
			}
			s.Stop()
			s.Stop() // idempotent

			if got := tr.closes.Load(); got != 1 {
				t.Fatalf("total closes = %d, want exactly 1", got)
			}
		})
	}
}

// claimTransport mimics a USB interface that can be claimed by only one
// transport at a time: Open fails while another transport holds the claim.
type claimTransport struct {
	transport.MockTransport
	id      int
	claimed *atomic.Bool
	log     *eventLog
	beats   atomic.Int32
}

type eventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLog) add(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, fmt.Sprintf(format, args...))
}

func (l *eventLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.events...)
}

func (l *eventLog) indexOf(ev string) int {
	for i, e := range l.snapshot() {
		if e == ev {
			return i
		}
	}
	return -1
}

func (c *claimTransport) Open(context.Context, string) error {
	if !c.claimed.CompareAndSwap(false, true) {
		c.log.add("open#%d:busy", c.id)
		return errors.New("libusb: bad access [code -3]")
	}
	c.log.add("open#%d:ok", c.id)
	return nil
}

func (c *claimTransport) Close() error {
	c.log.add("close#%d", c.id)
	c.claimed.Store(false)
	return nil
}

//nolint:gocyclo // linear scenario test with several assertions
func TestConnectionManager_ReleasesTransportBeforeReconnect(t *testing.T) {
	orig := backoffSteps
	backoffSteps = []time.Duration{10 * time.Millisecond}
	t.Cleanup(func() { backoffSteps = orig })

	var claimed atomic.Bool
	log := &eventLog{}
	var next atomic.Int32

	enc := &scriptedEncoder{hb: func(tr transport.Transport) (encoder.HeartbeatResult, error) {
		// Only the first session fails, after a few clean beats.
		if ct, ok := unwrapTracker(tr).(*claimTransport); ok && ct.id == 1 && ct.beats.Add(1) == 3 {
			return encoder.HeartbeatResult{}, errFatalHeartbeat
		}
		return encoder.HeartbeatResult{LidClosed: true, PaperLoaded: true}, nil
	}}

	ctx, cancel := context.WithCancel(context.Background())
	cm := NewConnectionManager(
		func(string) transport.Transport {
			return &claimTransport{id: int(next.Add(1)), claimed: &claimed, log: log}
		},
		func(string) encoder.Encoder { return enc },
	)
	cm.Start(ctx)
	t.Cleanup(func() { cancel(); cm.Stop() })

	events := cm.Subscribe()
	if err := cm.Add(testPrinterCfg("p1")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Collect state changes until the second Connected (or timeout).
	var states []ConnState
	var disconnectMsg string
	connected := 0
	timeout := time.After(3 * time.Second)
collect:
	for {
		select {
		case ev := <-events:
			states = append(states, ev.State)
			if ev.State == StateDisconnected {
				disconnectMsg = ev.Message
			}
			if ev.State == StateConnected {
				connected++
				if connected == 2 {
					break collect
				}
			}
		case <-timeout:
			t.Fatalf("never reconnected; states=%v log=%v", states, log.snapshot())
		}
	}

	want := []ConnState{StateConnecting, StateConnected, StateDisconnected, StateReconnecting, StateConnecting, StateConnected}
	if !isSubsequence(want, states) {
		t.Errorf("states = %v, want them to contain %v in order", states, want)
	}
	if disconnectMsg != errFatalHeartbeat.Error() {
		t.Errorf("disconnect message = %q, want %q", disconnectMsg, errFatalHeartbeat.Error())
	}

	c1, o2 := log.indexOf("close#1"), log.indexOf("open#2:ok")
	if c1 < 0 || o2 < 0 || c1 > o2 {
		t.Errorf("transport #1 must close before #2 opens; log=%v", log.snapshot())
	}
}
