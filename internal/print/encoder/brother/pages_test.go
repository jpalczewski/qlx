package brother

import (
	"image"
	"image/color"
	"testing"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

// recTransport records every Write as a separate chunk.
type recTransport struct {
	transport.MockTransport
	chunks [][]byte
}

func (r *recTransport) Write(data []byte) (int, error) {
	r.chunks = append(r.chunks, append([]byte(nil), data...))
	return len(data), nil
}

// pageJob is the parsed structure of one encoded job.
type pageJob struct {
	heights   []uint32 // raster height per ESC i z
	pageNums  []byte   // page number byte per ESC i z
	prints    []byte   // 0x0C / 0x1A commands in order
	cutEvery  byte     // ESC i A argument
	autocut   byte     // ESC i M argument
	cutAtEnd  bool     // ESC i K bit 3
	rasterRow int      // number of raster rows
}

// parseJob classifies the writes of one encoded job by their command prefix.
//
//nolint:gocyclo // flat switch over protocol commands
func parseJob(chunks [][]byte) pageJob {
	var j pageJob
	for _, c := range chunks {
		switch {
		case len(c) == 13 && c[0] == 0x1B && c[1] == 0x69 && c[2] == 0x7A:
			j.heights = append(j.heights, uint32(c[7])|uint32(c[8])<<8|uint32(c[9])<<16|uint32(c[10])<<24)
			j.pageNums = append(j.pageNums, c[11])
		case len(c) == 4 && c[0] == 0x1B && c[1] == 0x69 && c[2] == 0x41:
			j.cutEvery = c[3]
		case len(c) == 4 && c[0] == 0x1B && c[1] == 0x69 && c[2] == 0x4D:
			j.autocut = c[3]
		case len(c) == 4 && c[0] == 0x1B && c[1] == 0x69 && c[2] == 0x4B:
			j.cutAtEnd = c[3]&0x08 != 0
		case len(c) == 1 && (c[0] == 0x0C || c[0] == 0x1A):
			j.prints = append(j.prints, c[0])
		case len(c) > 3 && c[0] == 0x67:
			j.rasterRow++
		}
	}
	return j
}

func TestEncodePages_PairIsOneCutUnit(t *testing.T) {
	pages := []image.Image{makeImage(720, 5, color.Gray{Y: 255}), makeImage(720, 7, color.Gray{Y: 0})}
	tr := &recTransport{}
	var enc encoder.PagedEncoder = &BrotherEncoder{}

	if err := enc.EncodePages(pages, "QL-700", encoder.PrintOpts{Copies: 2, CutEvery: 1}, tr); err != nil {
		t.Fatalf("EncodePages: %v", err)
	}
	j := parseJob(tr.chunks)

	wantHeights := []uint32{5, 7, 5, 7}
	if len(j.heights) != len(wantHeights) {
		t.Fatalf("pages = %v, want %v (2 copies of a 2-page set)", j.heights, wantHeights)
	}
	for i, h := range wantHeights {
		if j.heights[i] != h {
			t.Errorf("page %d height = %d, want %d", i, j.heights[i], h)
		}
		if j.pageNums[i] != byte(i) {
			t.Errorf("page %d number = %d, want %d", i, j.pageNums[i], i)
		}
	}
	if j.rasterRow != 2*(5+7) {
		t.Errorf("raster rows = %d, want %d", j.rasterRow, 2*(5+7))
	}
	// One cut unit = one pair: the printer cuts every 2 pages, never between them.
	if j.cutEvery != 2 {
		t.Errorf("ESC i A = %d, want 2 (CutEvery counts pairs)", j.cutEvery)
	}
	if j.autocut != 0x40 || !j.cutAtEnd {
		t.Errorf("autocut = 0x%02X cutAtEnd = %v, want 0x40 and true", j.autocut, j.cutAtEnd)
	}
	if string(j.prints) != string([]byte{0x0C, 0x0C, 0x0C, 0x1A}) {
		t.Errorf("print commands = %v, want 3x 0x0C then 0x1A", j.prints)
	}
}

func TestEncodePages_NoCutAndSinglePageUnchanged(t *testing.T) {
	pages := []image.Image{makeImage(720, 5, color.Gray{Y: 255}), makeImage(720, 5, color.Gray{Y: 255})}
	tr := &recTransport{}
	if err := (&BrotherEncoder{}).EncodePages(pages, "QL-700", encoder.PrintOpts{Copies: 1}, tr); err != nil {
		t.Fatalf("EncodePages: %v", err)
	}
	if j := parseJob(tr.chunks); j.autocut != 0 || j.cutAtEnd || len(j.heights) != 2 {
		t.Errorf("no-cut pair: autocut=0x%02X cutAtEnd=%v pages=%d", j.autocut, j.cutAtEnd, len(j.heights))
	}

	// Encode of one image is a one-page set: cut every 1 page.
	tr = &recTransport{}
	if err := (&BrotherEncoder{}).Encode(makeImage(720, 5, color.Gray{Y: 255}), "QL-700", encoder.PrintOpts{Copies: 3, CutEvery: 1}, tr); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if j := parseJob(tr.chunks); j.cutEvery != 1 || len(j.heights) != 3 {
		t.Errorf("single: cutEvery=%d pages=%d, want 1 and 3", j.cutEvery, len(j.heights))
	}
}

func TestEncodePages_RejectsEmptyAndWideImages(t *testing.T) {
	enc := &BrotherEncoder{}
	if err := enc.EncodePages(nil, "QL-700", encoder.PrintOpts{}, &recTransport{}); err == nil {
		t.Error("expected error for no pages")
	}
	wide := []image.Image{makeImage(720, 5, color.Gray{Y: 255}), makeImage(800, 5, color.Gray{Y: 255})}
	if err := enc.EncodePages(wide, "QL-700", encoder.PrintOpts{}, &recTransport{}); err == nil {
		t.Error("expected error for a page wider than the printhead")
	}
}
