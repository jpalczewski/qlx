package brother

import (
	"image"
	"image/color"
	"testing"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

func makeImage(width, height int, fill color.Gray) image.Image {
	img := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetGray(x, y, fill)
		}
	}
	return img
}

func encode(t *testing.T, img image.Image) []byte {
	t.Helper()
	enc := &BrotherEncoder{}
	mock := &transport.MockTransport{}
	opts := encoder.PrintOpts{CutEvery: 1}
	if err := enc.Encode(img, "QL-700", opts, mock); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	return mock.Written
}

func TestBrotherEncode_StartsWithClear(t *testing.T) {
	img := makeImage(720, 1, color.Gray{Y: 255})
	data := encode(t, img)

	// First 200 bytes must be 0x00
	if len(data) < 202 {
		t.Fatalf("output too short: %d bytes", len(data))
	}
	for i := 0; i < 200; i++ {
		if data[i] != 0x00 {
			t.Fatalf("byte %d: expected 0x00, got 0x%02X", i, data[i])
		}
	}
	// Followed by ESC @
	if data[200] != 0x1B || data[201] != 0x40 {
		t.Fatalf("bytes 200-201: expected ESC @, got 0x%02X 0x%02X", data[200], data[201])
	}
}

func TestBrotherEncode_EndsWithPrint(t *testing.T) {
	img := makeImage(720, 1, color.Gray{Y: 255})
	data := encode(t, img)

	last := data[len(data)-1]
	if last != 0x1A {
		t.Fatalf("last byte: expected 0x1A, got 0x%02X", last)
	}
}

func TestBrotherEncode_RasterLine_AllBlack(t *testing.T) {
	// 720px wide, 1px tall, all black (Y=0)
	img := makeImage(720, 1, color.Gray{Y: 0})
	data := encode(t, img)

	// Find the raster line: look for 0x67 0x00 0x5A sequence
	rasterStart := -1
	for i := 0; i+2 < len(data); i++ {
		if data[i] == 0x67 && data[i+1] == 0x00 && data[i+2] == 0x5A {
			rasterStart = i
			break
		}
	}
	if rasterStart == -1 {
		t.Fatal("raster line header 0x67 0x00 0x5A not found")
	}

	if rasterStart+3+90 > len(data) {
		t.Fatalf("not enough bytes after raster header")
	}

	for i := 0; i < 90; i++ {
		b := data[rasterStart+3+i]
		if b != 0xFF {
			t.Fatalf("raster byte %d: expected 0xFF (all black), got 0x%02X", i, b)
		}
	}
}

func TestBrotherEncode_RasterLine_AllWhite(t *testing.T) {
	// 720px wide, 1px tall, all white (Y=255)
	img := makeImage(720, 1, color.Gray{Y: 255})
	data := encode(t, img)

	rasterStart := -1
	for i := 0; i+2 < len(data); i++ {
		if data[i] == 0x67 && data[i+1] == 0x00 && data[i+2] == 0x5A {
			rasterStart = i
			break
		}
	}
	if rasterStart == -1 {
		t.Fatal("raster line header 0x67 0x00 0x5A not found")
	}

	if rasterStart+3+90 > len(data) {
		t.Fatalf("not enough bytes after raster header")
	}

	for i := 0; i < 90; i++ {
		b := data[rasterStart+3+i]
		if b != 0x00 {
			t.Fatalf("raster byte %d: expected 0x00 (all white), got 0x%02X", i, b)
		}
	}
}

func TestEncode_MultiCopy(t *testing.T) {
	img := makeImage(720, 5, color.Gray{Y: 255})
	tr := &transport.MockTransport{}
	enc := &BrotherEncoder{}

	err := enc.Encode(img, "QL-700", encoder.PrintOpts{Copies: 3, CutEvery: 1}, tr)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Count 0x0C (print without feed): should be 2 (copies-1)
	ffCount := 0
	for _, b := range tr.Written {
		if b == 0x0C {
			ffCount++
		}
	}
	if ffCount != 2 {
		t.Errorf("0x0C count = %d, want 2", ffCount)
	}

	// Last byte should be 0x1A (print with feed)
	if tr.Written[len(tr.Written)-1] != 0x1A {
		t.Errorf("last byte = 0x%02X, want 0x1A", tr.Written[len(tr.Written)-1])
	}
}

func TestEncode_NoCut(t *testing.T) {
	img := makeImage(720, 5, color.Gray{Y: 255})
	tr := &transport.MockTransport{}
	enc := &BrotherEncoder{}

	err := enc.Encode(img, "QL-700", encoder.PrintOpts{Copies: 1, CutEvery: 0}, tr)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Find ESC i M in flat Written bytes — should be 0x00 (autocut off)
	found := false
	for i := 0; i < len(tr.Written)-3; i++ {
		if tr.Written[i] == 0x1B && tr.Written[i+1] == 0x69 && tr.Written[i+2] == 0x4D {
			if tr.Written[i+3] != 0x00 {
				t.Errorf("ESC i M byte = 0x%02X, want 0x00 (autocut off)", tr.Written[i+3])
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("ESC i M not found in output")
	}
}

func TestEncode_HighRes_DoublesRows(t *testing.T) {
	img := makeImage(720, 3, color.Gray{Y: 255})
	trNormal := &transport.MockTransport{}
	trHiRes := &transport.MockTransport{}
	enc := &BrotherEncoder{}

	if err := enc.Encode(img, "QL-700", encoder.PrintOpts{Copies: 1, CutEvery: 1, HighRes: false}, trNormal); err != nil {
		t.Fatalf("normal encode error: %v", err)
	}
	if err := enc.Encode(img, "QL-700", encoder.PrintOpts{Copies: 1, CutEvery: 1, HighRes: true}, trHiRes); err != nil {
		t.Fatalf("hi-res encode error: %v", err)
	}

	// Count raster lines: hi-res should have 2x as many 0x67 headers
	countRasterHeaders := func(data []byte) int {
		count := 0
		for i := 0; i < len(data)-2; i++ {
			if data[i] == 0x67 && data[i+1] == 0x00 && data[i+2] == 0x5A {
				count++
				i += 2
			}
		}
		return count
	}
	normal := countRasterHeaders(trNormal.Written)
	hiRes := countRasterHeaders(trHiRes.Written)
	if hiRes != normal*2 {
		t.Errorf("hi-res raster rows = %d, want %d (2x normal %d)", hiRes, normal*2, normal)
	}
}
