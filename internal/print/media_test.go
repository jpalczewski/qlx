package print

import (
	"image"
	"testing"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

// plainEncoder does not implement encoder.MediaGeometry.
type plainEncoder struct{}

func (plainEncoder) Name() string                { return "plain" }
func (plainEncoder) Models() []encoder.ModelInfo { return nil }
func (plainEncoder) Encode(image.Image, string, encoder.PrintOpts, transport.Transport) error {
	return nil
}

// geometryEncoder reports a fixed printable area for die-cut 62x29 labels.
type geometryEncoder struct{ plainEncoder }

func (geometryEncoder) PrintableArea(mediaType string, widthMm, lengthMm int) (int, int, bool) {
	if mediaType == "die-cut" && widthMm == 62 && lengthMm == 29 {
		return 696, 271, true
	}
	return 0, 0, false
}

func TestMediaForStatus(t *testing.T) {
	model := &encoder.ModelInfo{DPI: 300, PrintWidthPx: 720}

	tests := []struct {
		name       string
		enc        encoder.Encoder
		status     PrinterStatus
		wantWidth  int
		wantHeight int
	}{
		{
			name:       "encoder without geometry uses printhead width and media length",
			enc:        plainEncoder{},
			status:     PrinterStatus{LabelType: "die-cut", LabelWidthMm: 62, LabelHeightMm: 29},
			wantWidth:  720,
			wantHeight: 342,
		},
		{
			name:       "known media uses the printable area",
			enc:        geometryEncoder{},
			status:     PrinterStatus{LabelType: "die-cut", LabelWidthMm: 62, LabelHeightMm: 29},
			wantWidth:  696,
			wantHeight: 271,
		},
		{
			name:       "unknown media falls back to printhead width and media length",
			enc:        geometryEncoder{},
			status:     PrinterStatus{LabelType: "die-cut", LabelWidthMm: 62, LabelHeightMm: 30},
			wantWidth:  720,
			wantHeight: 354,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MediaForStatus(tt.enc, model, tt.status)
			if got.WidthPx != tt.wantWidth || got.HeightPx != tt.wantHeight || got.DPI != 300 {
				t.Errorf("MediaForStatus = %+v, want width %d height %d dpi 300", got, tt.wantWidth, tt.wantHeight)
			}
		})
	}
}
