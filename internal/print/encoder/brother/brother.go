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

	copies := max(opts.Copies, 1)

	rasterHeight := height
	if opts.HighRes {
		rasterHeight = height * 2
	}

	// 1. Clear buffer
	if _, err := tr.Write(make([]byte, 200)); err != nil {
		return err
	}

	// 2. ESC @ — initialize
	if _, err := tr.Write([]byte{0x1B, 0x40}); err != nil {
		return err
	}

	// 3. Read status for media type
	st, stErr := requestStatus(tr)
	mediaType := mediaContinuous
	mediaWidth := byte(62)
	mediaLength := byte(0)
	if stErr == nil {
		mediaType = st.MediaType
		mediaWidth = byte(st.MediaWidth)   //nolint:gosec
		mediaLength = byte(st.MediaLength) //nolint:gosec
	}

	// 4. Autocut mode
	if opts.CutEvery > 0 {
		if _, err := tr.Write([]byte{0x1B, 0x69, 0x4D, 0x40}); err != nil {
			return err
		}
	} else {
		if _, err := tr.Write([]byte{0x1B, 0x69, 0x4D, 0x00}); err != nil {
			return err
		}
	}

	// 5. Cut every N labels
	cutEvery := byte(1)
	if opts.CutEvery > 0 {
		cutEvery = byte(opts.CutEvery) //nolint:gosec
	}
	if _, err := tr.Write([]byte{0x1B, 0x69, 0x41, cutEvery}); err != nil {
		return err
	}

	// 6. Expanded mode: cut-at-end (bit 3), high-res 600 DPI (bit 6)
	expandedMode := byte(0x00)
	if opts.CutEvery > 0 {
		expandedMode |= 0x08
	}
	if opts.HighRes {
		expandedMode |= 0x40
	}
	if _, err := tr.Write([]byte{0x1B, 0x69, 0x4B, expandedMode}); err != nil {
		return err
	}

	// 7. Dynamic margin: 0 for die-cut, 35 for continuous
	margin := byte(35)
	if mediaType == mediaDieCut {
		margin = 0
	}
	if _, err := tr.Write([]byte{0x1B, 0x69, 0x64, margin, 0x00}); err != nil {
		return err
	}

	// Pre-encode all raster rows (pixel data shared across copies)
	rowBuf := make([]byte, 3+ql700.BytesPerRow)
	rowBuf[0] = 0x67
	rowBuf[1] = 0x00
	rowBuf[2] = byte(ql700.BytesPerRow) //nolint:gosec

	rows := make([][]byte, height)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		pixels := make([]byte, ql700.BytesPerRow)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			gray := (19595*r + 38470*g + 7471*b + 1<<15) >> 24
			var bit byte
			if gray < 128 {
				bit = 1
			}
			flippedX := (bounds.Max.X - 1) - x
			byteIdx := flippedX / 8
			bitIdx := uint(7 - (flippedX % 8))
			pixels[byteIdx] |= bit << bitIdx
		}
		rows[y-bounds.Min.Y] = pixels
	}

	// Copy loop
	for c := 0; c < copies; c++ {
		// ESC i z — media/quality info with page number
		//nolint:gosec
		mediaInfo := make([]byte, 13)
		mediaInfo[0] = 0x1B
		mediaInfo[1] = 0x69
		mediaInfo[2] = 0x7A
		mediaInfo[3] = 0xCE
		mediaInfo[4] = mediaType
		mediaInfo[5] = mediaWidth
		mediaInfo[6] = mediaLength
		binary.LittleEndian.PutUint32(mediaInfo[7:11], uint32(rasterHeight)) //nolint:gosec
		mediaInfo[11] = byte(c)                                              //nolint:gosec
		mediaInfo[12] = 0x00
		if _, err := tr.Write(mediaInfo); err != nil {
			return err
		}

		// Raster rows
		for _, pixels := range rows {
			copy(rowBuf[3:], pixels)
			if _, err := tr.Write(rowBuf); err != nil {
				return err
			}
			if opts.HighRes {
				if _, err := tr.Write(rowBuf); err != nil {
					return err
				}
			}
		}

		// Print command
		if c < copies-1 {
			if _, err := tr.Write([]byte{0x0C}); err != nil {
				return err
			}
		} else {
			if _, err := tr.Write([]byte{0x1A}); err != nil {
				return err
			}
		}
	}

	return nil
}
