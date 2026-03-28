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
