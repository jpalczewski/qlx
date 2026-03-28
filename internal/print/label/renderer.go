package label

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	qrcode "github.com/skip2/go-qrcode"
)

// RenderOpts controls optional rendering behaviour.
type RenderOpts struct {
	PrintDate bool // append "Wydrukowano: DATE" at bottom
}

// nowFunc is overridable for tests.
var nowFunc = time.Now

// MediaInfo describes the physical print media.
type MediaInfo struct {
	WidthPx  int // printhead width in pixels (384, 720)
	HeightPx int // 0 = continuous (dynamic height); >0 = die-cut label height in px
	DPI      int // 203, 300; used by encoder and manager, not consumed by renderer itself
}

// Render produces a label image from the given data using the named schema.
// media.WidthPx controls the image width; for continuous media (HeightPx==0) height is
// calculated from content; for die-cut media (HeightPx>0) image is exactly HeightPx tall.
func Render(data LabelData, template string, media MediaInfo, opts RenderOpts) (image.Image, error) {
	schema, ok := GetSchema(template)
	if !ok {
		return nil, fmt.Errorf("unknown template %q: valid templates are %v", template, SchemaNames())
	}
	return renderSchema(schema, data, media, opts)
}

// newCanvas creates a white RGBA image of the given dimensions.
func newCanvas(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	return img
}

// drawQR generates a QR code image from content and draws it into dst at (x, y) with the given size.
func drawQR(dst *image.RGBA, content string, x, y, size int) error {
	if content == "" {
		return nil
	}
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return fmt.Errorf("qr encode: %w", err)
	}
	qrImg := qr.Image(size)
	draw.Draw(dst, image.Rect(x, y, x+size, y+size), qrImg, image.Point{}, draw.Src)
	return nil
}

// drawBarcode generates a Code128 barcode and draws it scaled into dst at (x, y, w, h).
// If the content encodes to a barcode wider than w, it truncates content to fit.
func drawBarcode(dst *image.RGBA, content string, x, y, w, h int) error {
	if content == "" {
		return nil
	}

	// Try encoding, shorten content if barcode is wider than available space
	for len(content) > 0 {
		bc, err := code128.Encode(content)
		if err != nil {
			return fmt.Errorf("barcode encode: %w", err)
		}
		if bc.Bounds().Dx() <= w {
			scaled, err := barcode.Scale(bc, w, h)
			if err != nil {
				return fmt.Errorf("barcode scale: %w", err)
			}
			draw.Draw(dst, image.Rect(x, y, x+w, y+h), scaled, image.Point{}, draw.Src)
			return nil
		}
		// Barcode too wide, shorten content
		content = content[:len(content)-1]
	}
	// Content too short to encode, skip silently
	return nil
}
