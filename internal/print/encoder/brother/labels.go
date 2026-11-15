package brother

import (
	"image"
	"image/color"
	"image/draw"
)

// labelDef describes one media type as defined in the Brother QL Series
// Command Reference (section 3.2). All dot values are for 300 dpi.
type labelDef struct {
	ID           string
	TapeWidthMm  int
	TapeLengthMm int // 0 for continuous tape
	DotsPrintW   int // printable width in dots
	DotsPrintL   int // printable length in dots (0 for continuous tape)
	OffsetR      int // head pins of margin on the right, see padToHead
	FeedMargin   int
	MediaType    byte
}

const (
	mediaContinuous byte = 0x0A
	mediaDieCut     byte = 0x0B
)

var allLabels = []labelDef{
	{"62", 62, 0, 696, 0, 12, 35, mediaContinuous},
	{"29", 29, 0, 306, 0, 6, 35, mediaContinuous},
	{"29x90", 29, 90, 306, 991, 6, 0, mediaDieCut},
	{"62x29", 62, 29, 696, 271, 12, 0, mediaDieCut},
	{"62x100", 62, 100, 696, 1109, 12, 0, mediaDieCut},
}

// lookupLabel finds the media definition for the media reported by the printer
// status. Continuous tape is matched by width only; die-cut labels by width and length.
func lookupLabel(mediaType byte, widthMm, lengthMm int) (labelDef, bool) {
	for _, l := range allLabels {
		if l.MediaType != mediaType || l.TapeWidthMm != widthMm {
			continue
		}
		if mediaType == mediaDieCut && l.TapeLengthMm != lengthMm {
			continue
		}
		return l, true
	}
	return labelDef{}, false
}

// PrintableArea implements encoder.MediaGeometry. It reports the size of the
// printable area (not the label) so the renderer lays the label out exactly
// where the printer can actually print.
func (e *BrotherEncoder) PrintableArea(mediaType string, widthMm, lengthMm int) (widthPx, heightPx int, ok bool) {
	var mt byte
	switch mediaType {
	case "continuous":
		mt = mediaContinuous
	case "die-cut":
		mt = mediaDieCut
	default:
		return 0, 0, false
	}
	def, found := lookupLabel(mt, widthMm, lengthMm)
	if !found {
		return 0, 0, false
	}
	return def.DotsPrintW, def.DotsPrintL, true
}

// padToHead places an image of the printable width on a full printhead row,
// leaving the unprintable margin pins white.
//
// The encoder fills pins right to left (image column x goes to pin
// headPx-1-x), so the margin on the left of the image equals the margin of the
// right-hand pins, which is OffsetR.
func padToHead(img image.Image, headPx int, def labelDef) image.Image {
	b := img.Bounds()
	dst := image.NewGray(image.Rect(0, 0, headPx, b.Dy()))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(def.OffsetR, 0, def.OffsetR+b.Dx(), b.Dy()), img, b.Min, draw.Src)
	return dst
}
