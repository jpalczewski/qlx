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

// reinitEncoder reports a sticky status condition until Reinitialize is called
// (when clearsOnReinit is set), like a Brother media mismatch cleared by ESC @.
type reinitEncoder struct {
	scriptedEncoder
	active         atomic.Bool
	clearsOnReinit bool
	reinits        atomic.Int32
	encodes        atomic.Int32
}

var errMediaMismatch = &encoder.StatusError{Source: "mock", Conditions: []string{"replace media"}}

func (e *reinitEncoder) Heartbeat(transport.Transport) (encoder.HeartbeatResult, error) {
	e.count.Add(1)
	res := encoder.HeartbeatResult{LidClosed: true, PaperLoaded: true}
	if e.active.Load() {
		return res, errMediaMismatch
	}
	return res, nil
}

func (e *reinitEncoder) Reinitialize(transport.Transport) error {
	e.reinits.Add(1)
	if e.clearsOnReinit {
		e.active.Store(false)
	}
	return nil
}

func (e *reinitEncoder) Encode(image.Image, string, encoder.PrintOpts, transport.Transport) error {
	e.encodes.Add(1)
	return nil
}

func TestSession_PrintReinitializesStickyCondition(t *testing.T) {
	tests := []struct {
		name        string
		clears      bool
		wantErr     bool
		wantEncodes int32
	}{
		{name: "condition clears after re-init", clears: true, wantEncodes: 1},
		{name: "condition persists after re-init", clears: false, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			enc := &reinitEncoder{clearsOnReinit: tc.clears}
			enc.active.Store(true)
			s := NewSession(testPrinterCfg("p1"), &countingTransport{}, enc, nil, nil)
			if err := s.Start(context.Background()); err != nil {
				t.Fatalf("Start: %v", err)
			}
			t.Cleanup(s.Stop)
			waitFor(t, "condition reported", func() bool { return s.Status().LastError != "" })

			err := s.Print(image.NewGray(image.Rect(0, 0, 8, 8)), "mock-model", encoder.PrintOpts{})
			var se *encoder.StatusError
			if tc.wantErr != errors.As(err, &se) {
				t.Fatalf("Print err = %v, wantErr %v", err, tc.wantErr)
			}
			if enc.reinits.Load() < 1 {
				t.Error("printer was not re-initialised before refusing/printing")
			}
			if got := enc.encodes.Load(); got != tc.wantEncodes {
				t.Errorf("encodes = %d, want %d", got, tc.wantEncodes)
			}
			st := s.Status()
			if tc.wantErr && st.LastError != errMediaMismatch.Error() {
				t.Errorf("LastError = %q, want %q", st.LastError, errMediaMismatch.Error())
			}
			if !tc.wantErr && (st.LastError != "" || !st.Connected) {
				t.Errorf("status after cleared condition = %+v", st)
			}
		})
	}
}

func TestSession_UnchangedConditionIsNotRepublished(t *testing.T) {
	enc := &reinitEncoder{}
	enc.active.Store(true)
	var updates atomic.Int32
	s := NewSession(testPrinterCfg("p1"), &countingTransport{}, enc, nil, func(string, PrinterStatus) { updates.Add(1) })
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(s.Stop)

	waitFor(t, "condition reported", func() bool { return s.Status().LastError != "" })
	before, beats := updates.Load(), enc.count.Load()
	waitFor(t, "more heartbeats", func() bool { return enc.count.Load() >= beats+5 })
	if got := updates.Load(); got != before {
		t.Fatalf("%d status updates published for an unchanged condition", got-before)
	}
}

func TestSession_FatalErrorResetsCondition(t *testing.T) {
	var fatal atomic.Bool
	enc := &scriptedEncoder{hb: func(transport.Transport) (encoder.HeartbeatResult, error) {
		if fatal.Load() {
			return encoder.HeartbeatResult{}, errFatalHeartbeat
		}
		return encoder.HeartbeatResult{LidClosed: true}, errMediaMismatch
	}}
	s := NewSession(testPrinterCfg("p1"), &countingTransport{}, enc, nil, nil)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, "condition reported", func() bool { return s.Status().LastError == errMediaMismatch.Error() })

	fatal.Store(true)
	<-s.stopped
	if st := s.Status(); st.Connected || st.LastError != errFatalHeartbeat.Error() {
		t.Fatalf("status after fatal error = %+v", st)
	}
	err := s.Print(image.NewGray(image.Rect(0, 0, 8, 8)), "mock-model", encoder.PrintOpts{})
	if !errors.Is(err, ErrPrinterDisconnected) {
		t.Fatalf("Print after fatal error = %v, want ErrPrinterDisconnected (not a stale condition)", err)
	}
}
