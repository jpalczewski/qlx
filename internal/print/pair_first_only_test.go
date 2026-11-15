package print

import (
	"image"
	"testing"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/label"
)

// fixedMediaRenderer renders with the real label engine on a fixed canvas, so
// the render options reach label.RenderPages.
type fixedMediaRenderer struct{}

func (fixedMediaRenderer) Render(data label.LabelData, tmpl string, _ label.MediaInfo, opts label.RenderOpts) ([]image.Image, error) {
	return label.RenderPages(data, tmpl, label.MediaInfo{WidthPx: 384, HeightPx: 160, DPI: 203}, opts)
}

func TestPrint_FirstPageOnlyIsOneLabelPerCopy(t *testing.T) {
	data := label.LabelData{Name: "Synthetic", Location: "Room", QRContent: "/items/x"}

	// Paged encoder: one job holding a single page; cut_every now counts labels.
	paged := &pagedRecordingEncoder{}
	mgr, id := connectedManager(t, paged, fixedMediaRenderer{})
	opts := encoder.PrintOpts{Copies: 3, CutEvery: 1}
	if err := mgr.Print(id, data, label.PairTemplate, label.RenderOpts{FirstPageOnly: true}, opts); err != nil {
		t.Fatalf("Print: %v", err)
	}
	calls := paged.snapshot()
	if len(calls) != 1 || len(calls[0].heights) != 1 {
		t.Fatalf("calls = %+v, want one job with exactly one page", calls)
	}
	if calls[0].opts.Copies != 3 || calls[0].opts.CutEvery != 1 {
		t.Errorf("opts = %+v, want copies 3 and a cut after every label", calls[0].opts)
	}

	// Without the option the same print sends both pages.
	both := &pagedRecordingEncoder{}
	mgr, id = connectedManager(t, both, fixedMediaRenderer{})
	if err := mgr.Print(id, data, label.PairTemplate, label.RenderOpts{}, opts); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if c := both.snapshot(); len(c) != 1 || len(c[0].heights) != 2 {
		t.Errorf("calls = %+v, want one job with two pages", c)
	}

	// Unpaged encoder: one page is a plain Encode, copies and cut_every unchanged.
	plain := &recordingEncoder{}
	mgr, id = connectedManager(t, plain, fixedMediaRenderer{})
	if err := mgr.Print(id, data, label.PairTemplate, label.RenderOpts{FirstPageOnly: true}, opts); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if c := plain.snapshot(); len(c) != 1 || len(c[0].heights) != 1 || c[0].opts.Copies != 3 || c[0].opts.CutEvery != 1 {
		t.Errorf("calls = %+v, want one single-page Encode with copies 3 and cut every 1", c)
	}
}
