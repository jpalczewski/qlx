package brother

import (
	"encoding/binary"
	"fmt"
	"image"
	"slices"

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

// Encode prints one image, opts.Copies times.
func (e *BrotherEncoder) Encode(img image.Image, model string, opts encoder.PrintOpts, tr transport.Transport) error {
	return e.EncodePages([]image.Image{img}, model, opts, tr)
}

// EncodePages prints a sequence of pages as one job and repeats the whole
// sequence opts.Copies times, so pages of one set stay adjacent. opts.CutEvery
// counts sets, not pages: with two pages, CutEvery 1 cuts after every second
// page and never between the two. It implements encoder.PagedEncoder.
//
//nolint:gocyclo // encoder protocol requires sequential steps
func (e *BrotherEncoder) EncodePages(pages []image.Image, model string, opts encoder.PrintOpts, tr transport.Transport) error {
	if model != ql700.ID {
		return fmt.Errorf("unsupported model: %s", model)
	}
	if len(pages) == 0 {
		return fmt.Errorf("no pages to print")
	}

	headPx := ql700.BytesPerRow * 8
	for _, pg := range pages {
		if w := pg.Bounds().Dx(); w <= 0 || w > headPx {
			return fmt.Errorf("image width must be at most %d pixels, got %d", headPx, w)
		}
	}

	copies := max(opts.Copies, 1)

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

	// An image narrower than the printhead must be exactly the printable width of
	// the loaded media. Place it on a full head row with the unprintable margin
	// pins left white (Command Reference, section 3.2.3).
	pages = slices.Clone(pages)
	for i, pg := range pages {
		if width := pg.Bounds().Dx(); width != headPx {
			def, ok := lookupLabel(mediaType, int(mediaWidth), int(mediaLength))
			if !ok || def.DotsPrintW != width {
				return fmt.Errorf("image width must be %d pixels (printhead) or the printable width of the loaded media, got %d", headPx, width)
			}
			pages[i] = padToHead(pg, headPx, def)
		}
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
		cutEvery = byte(min(opts.CutEvery*len(pages), 255)) //nolint:gosec
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
	pageRows := make([][][]byte, len(pages))
	for i, pg := range pages {
		pageRows[i] = encodeRasterRows(pg)
	}

	total := copies * len(pages)
	rowBuf := make([]byte, 3+ql700.BytesPerRow)
	rowBuf[0] = 0x67
	rowBuf[1] = 0x00
	rowBuf[2] = byte(ql700.BytesPerRow) //nolint:gosec

	// Page loop: the whole set of pages, copies times
	for n := 0; n < total; n++ {
		rows := pageRows[n%len(pages)]
		rasterHeight := len(rows)
		if opts.HighRes {
			rasterHeight *= 2
		}

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
		mediaInfo[11] = byte(n)                                              //nolint:gosec
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
		if n < total-1 {
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

// encodeRasterRows converts an image to printhead rows: 1 bit per pixel,
// mirrored horizontally, black where the luminance is below 50%.
func encodeRasterRows(img image.Image) [][]byte {
	bounds := img.Bounds()
	rows := make([][]byte, bounds.Dy())
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
	return rows
}
