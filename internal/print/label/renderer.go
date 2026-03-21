package label

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	fontHeight  = 13
	fontWidth   = 7
	lineSpacing = 4
	padding     = 8
)

// Render produces a label image from the given data using the named template.
// widthPx controls the image width; height is calculated automatically.
// dpi is accepted for API compatibility but basicfont is resolution-independent.
func Render(data LabelData, template string, widthPx, dpi int) (image.Image, error) {
	switch template {
	case "simple":
		return renderSimple(data, widthPx)
	case "standard":
		return renderStandard(data, widthPx)
	case "compact":
		return renderCompact(data, widthPx)
	case "detailed":
		return renderDetailed(data, widthPx)
	default:
		return nil, fmt.Errorf("unknown template %q: valid templates are %v", template, templateNames)
	}
}

// newCanvas creates a white RGBA image of the given dimensions.
func newCanvas(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	return img
}

// drawText draws a string at (x, y) where y is the baseline.
func drawText(img *image.RGBA, x, y int, text string, col color.Color) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

// lineH returns the total height per line of text (font + spacing).
func lineH() int {
	return fontHeight + lineSpacing
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
func drawBarcode(dst *image.RGBA, content string, x, y, w, h int) error {
	if content == "" {
		return nil
	}
	bc, err := code128.Encode(content)
	if err != nil {
		return fmt.Errorf("barcode encode: %w", err)
	}
	scaled, err := barcode.Scale(bc, w, h)
	if err != nil {
		return fmt.Errorf("barcode scale: %w", err)
	}
	draw.Draw(dst, image.Rect(x, y, x+w, y+h), scaled, image.Point{}, draw.Src)
	return nil
}

// renderSimple: white background, large centered name text.
func renderSimple(data LabelData, widthPx int) (image.Image, error) {
	height := padding*2 + lineH()
	img := newCanvas(widthPx, height)

	nameX := (widthPx - len(data.Name)*fontWidth) / 2
	if nameX < padding {
		nameX = padding
	}
	drawText(img, nameX, padding+fontHeight, data.Name, color.Black)
	return img, nil
}

// renderStandard: name at top, location below, QR code on right side.
func renderStandard(data LabelData, widthPx int) (image.Image, error) {
	qrSize := 80
	textAreaW := widthPx - qrSize - padding*3
	lines := 1
	if data.Location != "" {
		lines++
	}
	height := padding*2 + lines*lineH()
	if qrSize+padding*2 > height {
		height = qrSize + padding*2
	}

	img := newCanvas(widthPx, height)

	// Text on the left
	y := padding + fontHeight
	drawText(img, padding, y, data.Name, color.Black)
	_ = textAreaW

	if data.Location != "" {
		y += lineH()
		drawText(img, padding, y, data.Location, color.RGBA{80, 80, 80, 255})
	}

	// QR on the right
	qrX := widthPx - qrSize - padding
	if err := drawQR(img, data.QRContent, qrX, padding, qrSize); err != nil {
		return nil, err
	}

	return img, nil
}

// renderCompact: name + description in smaller font, tight layout.
func renderCompact(data LabelData, widthPx int) (image.Image, error) {
	lines := 1
	if data.Description != "" {
		lines++
	}
	height := padding*2 + lines*lineH()
	img := newCanvas(widthPx, height)

	y := padding + fontHeight
	drawText(img, padding, y, data.Name, color.Black)

	if data.Description != "" {
		y += lineH()
		drawText(img, padding, y, data.Description, color.RGBA{60, 60, 60, 255})
	}

	return img, nil
}

// renderDetailed: name + description + location + QR code + barcode at bottom.
func renderDetailed(data LabelData, widthPx int) (image.Image, error) {
	qrSize := 96
	barcodeH := 32
	barcodeW := widthPx - padding*2

	// Count text lines
	textLines := 1 // name always
	if data.Description != "" {
		textLines++
	}
	if data.Location != "" {
		textLines++
	}
	textH := textLines*lineH() + padding*2

	// QR area
	qrAreaH := qrSize + padding*2

	mainH := textH
	if qrAreaH > mainH {
		mainH = qrAreaH
	}

	// Barcode at bottom
	totalH := mainH + barcodeH + padding
	if data.BarcodeID == "" {
		totalH = mainH
	}

	img := newCanvas(widthPx, totalH)

	// Text column (left)
	y := padding + fontHeight
	drawText(img, padding, y, data.Name, color.Black)

	if data.Description != "" {
		y += lineH()
		drawText(img, padding, y, data.Description, color.RGBA{60, 60, 60, 255})
	}
	if data.Location != "" {
		y += lineH()
		drawText(img, padding, y, data.Location, color.RGBA{80, 80, 80, 255})
	}

	// QR on right
	qrX := widthPx - qrSize - padding
	if err := drawQR(img, data.QRContent, qrX, padding, qrSize); err != nil {
		return nil, err
	}

	// Barcode at bottom
	if data.BarcodeID != "" {
		barcodeY := mainH
		if err := drawBarcode(img, data.BarcodeID, padding, barcodeY, barcodeW, barcodeH); err != nil {
			return nil, err
		}
	}

	return img, nil
}
