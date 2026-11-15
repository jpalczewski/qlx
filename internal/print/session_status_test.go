package print

import (
	"context"
	"errors"
	"image"
	"sync/atomic"
	"testing"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

func TestSession_StatusErrorIsNonFatal(t *testing.T) {
	var active atomic.Bool
	statusErr := &encoder.StatusError{Source: "mock", Conditions: []string{"cover open"}}

	tr := &countingTransport{}
	enc := &scriptedEncoder{hb: func(transport.Transport) (encoder.HeartbeatResult, error) {
		if active.Load() {
			return encoder.HeartbeatResult{LidClosed: false, PaperLoaded: true}, statusErr
		}
		return encoder.HeartbeatResult{LidClosed: true, PaperLoaded: true}, nil
	}}
	s := NewSession(testPrinterCfg("p1"), tr, enc, nil, nil)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(s.Stop)

	img := image.NewGray(image.Rect(0, 0, 8, 8))

	// Clean: printing works.
	waitFor(t, "first clean heartbeat", func() bool { return enc.count.Load() > 0 && s.Status().LidClosed })
	if err := s.Print(img, "mock-model", encoder.PrintOpts{}); err != nil {
		t.Fatalf("Print while clean: %v", err)
	}

	// Condition active: session stays up, error surfaced, Print refuses.
	active.Store(true)
	waitFor(t, "condition to be reported", func() bool { return s.Status().LastError != "" })
	st := s.Status()
	if !st.Connected || st.LidClosed || st.LastError != statusErr.Error() {
		t.Fatalf("status during condition = %+v", st)
	}
	var se *encoder.StatusError
	if err := s.Print(img, "mock-model", encoder.PrintOpts{}); !errors.As(err, &se) {
		t.Fatalf("Print during condition = %v, want *encoder.StatusError", err)
	}
	select {
	case <-s.stopped:
		t.Fatal("heartbeat loop ended on a status condition")
	default:
	}

	// Recovery: LastError clears, still connected, Print works, never closed.
	active.Store(false)
	waitFor(t, "recovery", func() bool { return s.Status().LastError == "" })
	if st := s.Status(); !st.Connected || !st.LidClosed {
		t.Fatalf("status after recovery = %+v", st)
	}
	if err := s.Print(img, "mock-model", encoder.PrintOpts{}); err != nil {
		t.Fatalf("Print after recovery: %v", err)
	}
	if got := tr.closes.Load(); got != 0 {
		t.Fatalf("transport closed %d times during status condition, want 0", got)
	}

	s.Stop()
	if got := tr.closes.Load(); got != 1 {
		t.Fatalf("closes after Stop = %d, want 1", got)
	}
}
