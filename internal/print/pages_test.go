package print

import (
	"context"
	"errors"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/print/transport"
)

// encodeCall is one recorded encoder invocation.
type encodeCall struct {
	heights []int
	opts    encoder.PrintOpts
}

// recordingEncoder is a status-capable encoder that records Encode calls.
type recordingEncoder struct {
	mockEncoder
	mu    sync.Mutex
	calls []encodeCall
}

func (r *recordingEncoder) record(pages []image.Image, opts encoder.PrintOpts) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := encodeCall{opts: opts}
	for _, p := range pages {
		c.heights = append(c.heights, p.Bounds().Dy())
	}
	r.calls = append(r.calls, c)
}

func (r *recordingEncoder) Encode(img image.Image, _ string, opts encoder.PrintOpts, _ transport.Transport) error {
	r.record([]image.Image{img}, opts)
	return nil
}

func (r *recordingEncoder) snapshot() []encodeCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]encodeCall(nil), r.calls...)
}

// pagedRecordingEncoder additionally implements encoder.PagedEncoder.
type pagedRecordingEncoder struct{ recordingEncoder }

func (p *pagedRecordingEncoder) EncodePages(pages []image.Image, _ string, opts encoder.PrintOpts, _ transport.Transport) error {
	p.record(pages, opts)
	return nil
}

// connectedManager returns a manager with one connected printer driven by enc.
func connectedManager(t *testing.T, enc encoder.Encoder, renderer DocumentRenderer) (*PrinterManager, string) {
	t.Helper()
	s := newTestPrintStore(t)
	cm := NewConnectionManager(
		func(string) transport.Transport { return &transport.MockTransport{} },
		func(string) encoder.Encoder { return enc },
	)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(cm.Stop)
	cm.Start(ctx)

	mgr := NewPrinterManager(s, cm)
	mgr.RegisterEncoder(enc)
	mgr.renderer = renderer

	printer := s.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")
	if err := cm.Add(*printer); err != nil {
		t.Fatalf("cm.Add: %v", err)
	}
	if !waitForState(cm, printer.ID, StateConnected, 2*time.Second) {
		t.Fatalf("printer did not reach connected state")
	}
	return mgr, printer.ID
}

func TestPrint_PagedEncoderGetsWholePairInOneJob(t *testing.T) {
	enc := &pagedRecordingEncoder{}
	mgr, id := connectedManager(t, enc, &spyRenderer{pages: 2})

	opts := encoder.PrintOpts{Copies: 3, CutEvery: 1}
	if err := mgr.Print(id, label.LabelData{Name: "Synthetic"}, label.PairTemplate, label.RenderOpts{}, opts); err != nil {
		t.Fatalf("Print: %v", err)
	}

	calls := enc.snapshot()
	if len(calls) != 1 {
		t.Fatalf("encoder jobs = %d, want 1 (the pair is one job)", len(calls))
	}
	if got := calls[0].heights; len(got) != 2 || got[0] != 10 || got[1] != 11 {
		t.Errorf("pages = %v, want the two rendered pages in order", got)
	}
	if calls[0].opts.Copies != 3 || calls[0].opts.CutEvery != 1 {
		t.Errorf("opts = %+v, want copies 3 and cut every 1 pair", calls[0].opts)
	}
}

func TestPrint_UnpagedEncoderPrintsPairsInOrder(t *testing.T) {
	enc := &recordingEncoder{}
	mgr, id := connectedManager(t, enc, &spyRenderer{pages: 2})

	if err := mgr.Print(id, label.LabelData{Name: "Synthetic"}, label.PairTemplate, label.RenderOpts{}, encoder.PrintOpts{Copies: 2, CutEvery: 1}); err != nil {
		t.Fatalf("Print: %v", err)
	}

	calls := enc.snapshot()
	want := []int{10, 11, 10, 11} // pair, pair: pages of a pair stay adjacent
	if len(calls) != len(want) {
		t.Fatalf("encoder jobs = %d, want %d", len(calls), len(want))
	}
	for i, c := range calls {
		if len(c.heights) != 1 || c.heights[0] != want[i] {
			t.Errorf("job %d pages = %v, want [%d]", i, c.heights, want[i])
		}
		if c.opts.Copies != 1 || c.opts.CutEvery != 0 {
			t.Errorf("job %d opts = %+v, want copies 1 and no cut", i, c.opts)
		}
	}
}

func TestPrint_SinglePageKeepsEncodeSemantics(t *testing.T) {
	enc := &recordingEncoder{}
	mgr, id := connectedManager(t, enc, &spyRenderer{})

	if err := mgr.Print(id, label.LabelData{Name: "Synthetic"}, "simple", label.RenderOpts{}, encoder.PrintOpts{Copies: 4, CutEvery: 2}); err != nil {
		t.Fatalf("Print: %v", err)
	}
	calls := enc.snapshot()
	if len(calls) != 1 || calls[0].opts.Copies != 4 || calls[0].opts.CutEvery != 2 {
		t.Errorf("calls = %+v, want one Encode with copies 4 and cut every 2", calls)
	}
}

func TestPrint_RenderFailureIsRenderError(t *testing.T) {
	enc := &recordingEncoder{}
	mgr, id := connectedManager(t, enc, &spyRenderer{err: errors.New("no pair for this media")})

	err := mgr.Print(id, label.LabelData{Name: "Synthetic"}, label.PairTemplate, label.RenderOpts{}, encoder.PrintOpts{})
	var re *RenderError
	if !errors.As(err, &re) {
		t.Fatalf("error = %v, want *RenderError", err)
	}
	if len(enc.snapshot()) != 0 {
		t.Error("nothing may be sent to the printer when rendering fails")
	}
}
