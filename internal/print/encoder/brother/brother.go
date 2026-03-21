package brother

import (
	"encoding/binary"
	"fmt"
	"image"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

// BrotherEncoder implements the Brother QL-700 raster protocol.
type BrotherEncoder struct{}

func (e *BrotherEncoder) Name() string {
	return "brother-ql"
}

func (e *BrotherEncoder) Models() []encoder.ModelInfo {
	return []encoder.ModelInfo{modelInfo(ql700)}
}

//nolint:gocyclo // encoder protocol requires sequential steps
func (e *BrotherEncoder) Encode(img image.Image, model string, opts encoder.PrintOpts, tr transport.Transport) error {
	if model != ql700.ID {
		return fmt.Errorf("unsupported model: %s", model)
	}

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	if width != ql700.BytesPerRow*8 {
		return fmt.Errorf("image width must be %d pixels, got %d", ql700.BytesPerRow*8, width)
	}

	// 1. Clear buffer: 200 x 0x00
	clearBuf := make([]byte, 200)
	if _, err := tr.Write(clearBuf); err != nil {
		return err
	}

	// 2. ESC @ — initialize
	if _, err := tr.Write([]byte{0x1B, 0x40}); err != nil {
		return err
	}

	// 3. Media/quality info: ESC i z + 10 bytes
	//nolint:gosec // G115: value range is validated by protocol constraints
	rasterLines := uint32(height)
	mediaInfo := make([]byte, 13)
	mediaInfo[0] = 0x1B
	mediaInfo[1] = 0x69
	mediaInfo[2] = 0x7A
	mediaInfo[3] = 0x86 // flags: quality + media_type + media_width
	mediaInfo[4] = byte(mediaContinuous)
	mediaInfo[5] = 62 // media_width_mm (default 62mm)
	mediaInfo[6] = 0  // media_length_mm (0 for continuous)
	binary.LittleEndian.PutUint32(mediaInfo[7:11], rasterLines)
	mediaInfo[11] = 0x00 // page_number (starting page)
	mediaInfo[12] = 0x00 // reserved
	if _, err := tr.Write(mediaInfo); err != nil {
		return err
	}

	// 4. Autocut on (if AutoCut)
	if opts.AutoCut {
		if _, err := tr.Write([]byte{0x1B, 0x69, 0x4D, 0x40}); err != nil {
			return err
		}
	}

	// 5. Cut every 1 label
	if _, err := tr.Write([]byte{0x1B, 0x69, 0x41, 0x01}); err != nil {
		return err
	}

	// 6. Expanded mode: cut at end
	if _, err := tr.Write([]byte{0x1B, 0x69, 0x4B, 0x08}); err != nil {
		return err
	}

	// 7. Margin: 35 dots LE
	if _, err := tr.Write([]byte{0x1B, 0x69, 0x64, 0x23, 0x00}); err != nil {
		return err
	}

	// 8. Raster lines
	rowBuf := make([]byte, 3+ql700.BytesPerRow)
	rowBuf[0] = 0x67
	rowBuf[1] = 0x00
	rowBuf[2] = byte(ql700.BytesPerRow) //nolint:gosec // G115: value range is validated by protocol constraints

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		pixels := make([]byte, ql700.BytesPerRow)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Convert to grayscale and threshold
			r, g, b, _ := img.At(x, y).RGBA()
			// RGBA returns 16-bit values; convert to 8-bit
			gray := (19595*r + 38470*g + 7471*b + 1<<15) >> 24
			var bit byte
			if gray < 128 {
				bit = 1 // black = print dot
			}

			// Flip horizontally: pixel at x maps to bit position (width-1-x) from the left
			// Pack: MSB = leftmost after flip = rightmost physical pixel
			flippedX := (bounds.Max.X - 1) - x
			byteIdx := flippedX / 8
			bitIdx := uint(7 - (flippedX % 8))
			pixels[byteIdx] |= bit << bitIdx
		}
		copy(rowBuf[3:], pixels)
		if _, err := tr.Write(rowBuf); err != nil {
			return err
		}
	}

	// 9. Print with feed (last page)
	if _, err := tr.Write([]byte{0x1A}); err != nil {
		return err
	}

	return nil
}
