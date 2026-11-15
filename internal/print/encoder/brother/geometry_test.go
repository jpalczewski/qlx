package brother

import (
	"bytes"
	"image/color"
	"testing"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

// statusReply builds a 32-byte status response for the loaded media.
func statusReply(mediaType byte, widthMm, lengthMm byte) []byte {
	d := make([]byte, statusSize)
	d[0], d[1], d[2], d[3] = statusHeader0, statusHeader1, 'B', '0'
	d[10] = widthMm
	d[11] = mediaType
	d[17] = lengthMm
	d[18] = statusTypeReply
	return d
}

func TestPrintableArea(t *testing.T) {
	tests := []struct {
		name      string
		mediaType string
		widthMm   int
		lengthMm  int
		wantW     int
		wantH     int
		wantOK    bool
	}{
		{"62x29 die-cut", "die-cut", 62, 29, 696, 271, true},
		{"62x100 die-cut", "die-cut", 62, 100, 696, 1109, true},
		{"62 mm continuous", "continuous", 62, 0, 696, 0, true},
		{"29 mm continuous", "continuous", 29, 0, 306, 0, true},
		{"die-cut with unknown length", "die-cut", 62, 30, 0, 0, false},
		{"unknown media type", "with-gaps", 62, 29, 0, 0, false},
		{"no media", "none", 0, 0, 0, 0, false},
	}
	enc := &BrotherEncoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h, ok := enc.PrintableArea(tt.mediaType, tt.widthMm, tt.lengthMm)
			if ok != tt.wantOK || w != tt.wantW || h != tt.wantH {
				t.Errorf("PrintableArea(%q, %d, %d) = (%d, %d, %v), want (%d, %d, %v)",
					tt.mediaType, tt.widthMm, tt.lengthMm, w, h, ok, tt.wantW, tt.wantH, tt.wantOK)
			}
		})
	}
}

// The encoder must implement the optional geometry interface.
var _ encoder.MediaGeometry = (*BrotherEncoder)(nil)

func TestEncode_PrintableWidthIsPaddedToHead(t *testing.T) {
	// No status reply: the encoder falls back to 62 mm continuous (696 printable dots).
	img := makeImage(696, 2, color.Gray{Y: 0})
	data := encode(t, img)

	i := bytes.Index(data, []byte{0x67, 0x00, 0x5A})
	if i < 0 {
		t.Fatal("raster line header not found")
	}
	row := data[i+3 : i+3+90]

	// Pins 12..707 are black: pins 0-7 white, byte 1 = pins 8-15 (12-15 black),
	// bytes 2..87 fully black, byte 88 = pins 704-711 (704-707 black), byte 89 white.
	want := make([]byte, 90)
	want[1] = 0x0F
	for b := 2; b <= 87; b++ {
		want[b] = 0xFF
	}
	want[88] = 0xF0
	if !bytes.Equal(row, want) {
		t.Errorf("raster row mismatch\n got % X\nwant % X", row, want)
	}
}

func TestEncode_RejectsUnexpectedWidth(t *testing.T) {
	for _, width := range []int{700, 721} {
		enc := &BrotherEncoder{}
		mock := &transport.MockTransport{}
		if err := enc.Encode(makeImage(width, 2, color.Gray{Y: 0}), "QL-700", encoder.PrintOpts{}, mock); err == nil {
			t.Errorf("width %d: expected an error", width)
		}
	}
}

func TestEncode_DieCut62x29(t *testing.T) {
	const rows = 271
	enc := &BrotherEncoder{}
	mock := &transport.MockTransport{}
	mock.SetReadData(statusReply(mediaDieCut, 62, 29))

	img := makeImage(696, rows, color.Gray{Y: 255})
	if err := enc.Encode(img, "QL-700", encoder.PrintOpts{CutEvery: 1}, mock); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	data := mock.Written

	h := bytes.Index(data, []byte{0x1B, 0x69, 0x7A})
	if h < 0 {
		t.Fatal("print information command (ESC i z) not found")
	}
	hdr := data[h : h+13]
	if hdr[4] != mediaDieCut || hdr[5] != 62 || hdr[6] != 29 {
		t.Errorf("media in header = type 0x%02X, %d x %d mm; want die-cut 62 x 29", hdr[4], hdr[5], hdr[6])
	}
	rasterNumber := int(hdr[7]) | int(hdr[8])<<8 | int(hdr[9])<<16 | int(hdr[10])<<24
	if rasterNumber != rows {
		t.Errorf("raster number = %d, want %d", rasterNumber, rows)
	}

	// Each raster row is 3 header bytes + 90 data bytes; the job ends with 0x1A.
	body := data[h+13 : len(data)-1]
	if len(body) != rows*93 {
		t.Errorf("raster data = %d bytes, want %d (%d rows)", len(body), rows*93, rows)
	}
	if data[len(data)-1] != 0x1A {
		t.Errorf("last byte = 0x%02X, want 0x1A", data[len(data)-1])
	}
}

func TestParseStatus_ErrorBitsPerCommandReference(t *testing.T) {
	tests := []struct {
		name         string
		err2         byte
		wantCover    bool
		wantMismatch bool
	}{
		{"cover opened while printing (bit 4)", 0x10, true, false},
		{"media mismatch (bit 0)", 0x01, false, true},
		{"no error", 0x00, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d [statusSize]byte
			copy(d[:], statusReply(mediaDieCut, 62, 29))
			d[9] = tt.err2
			st, err := parseStatus(d)
			if err != nil {
				t.Fatalf("parseStatus: %v", err)
			}
			if st.CoverOpen != tt.wantCover || st.ReplaceMedia != tt.wantMismatch {
				t.Errorf("CoverOpen=%v ReplaceMedia=%v, want %v and %v",
					st.CoverOpen, st.ReplaceMedia, tt.wantCover, tt.wantMismatch)
			}
		})
	}
}
