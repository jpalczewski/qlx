package label

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// resolvedText holds a fully prepared text element ready to be drawn.
type resolvedText struct {
	lines []string
	face  font.Face
	col   color.RGBA
	align string
	lineH int
}

// renderSchema renders a label image from a Schema definition and LabelData.
// Elements are laid out vertically. QR is positioned at top-right, barcode at full-width bottom.
// Note: QR is always placed at top-right when present — vertical QR placement is not supported.
func renderSchema(schema Schema, data LabelData, widthPx int) (image.Image, error) {
	pad := schema.Padding
	qrReserved := qrSizeForSchema(schema)

	textElems, qrSize, barcodeH, err := resolveElements(schema, data, widthPx, pad, qrReserved)
	if err != nil {
		return nil, err
	}

	totalH := computeHeight(textElems, qrSize, barcodeH, data.BarcodeID, pad)
	img := newCanvas(widthPx, totalH)

	drawTextElements(img, textElems, widthPx, pad, qrSize)

	if err := maybeDrawQR(img, data.QRContent, widthPx, pad, qrSize); err != nil {
		return nil, err
	}
	if err := maybeDrawBarcode(img, data.BarcodeID, widthPx, pad, barcodeH, totalH); err != nil {
		return nil, err
	}

	return img, nil
}

// resolveElements processes schema elements into resolved text rows and extracted sizes.
func resolveElements(schema Schema, data LabelData, widthPx, pad, qrReserved int) ([]resolvedText, int, int, error) {
	slotText := map[string]string{
		"title":       data.Name,
		"description": data.Description,
		"location":    data.Location,
	}

	var textElems []resolvedText
	var qrSize, barcodeH int

	for _, el := range schema.Elements {
		switch el.Slot {
		case "title", "description", "location":
			text := slotText[el.Slot]
			effectiveFont := el.FontFamily
			if effectiveFont == "" {
				effectiveFont = schema.FontFamily
			}
			if effectiveFont == "" {
				effectiveFont = "spleen"
			}
			if IsBasicFont(effectiveFont) {
				text = TransliteratePL(text)
			}
			rt, err := resolveTextElement(el, text, effectiveFont, widthPx, pad, qrReserved)
			if err != nil {
				return nil, 0, 0, err
			}
			textElems = append(textElems, rt)
		case "qr":
			qrSize = el.Size
		case "barcode":
			barcodeH = el.Height
		}
	}

	return textElems, qrSize, barcodeH, nil
}

// resolveTextElement builds a resolvedText from a single Element definition.
func resolveTextElement(el Element, text, fontFamily string, widthPx, pad, qrReserved int) (resolvedText, error) {
	face, err := LoadFace(fontFamily, el.FontSize)
	if err != nil {
		return resolvedText{}, err
	}
	metrics := face.Metrics()
	var lh int
	if IsBasicFont(fontFamily) {
		lh = (metrics.Ascent + metrics.Descent).Ceil()
	} else {
		lh = (metrics.Ascent + metrics.Descent + fixed.I(int(el.FontSize/4))).Ceil()
	}

	textW := widthPx - pad*2
	if qrReserved > 0 {
		textW -= qrReserved + pad
	}

	var lines []string
	if el.Wrap {
		lines = wrapText(text, face, textW)
	} else if text != "" {
		lines = []string{text}
	}

	return resolvedText{
		lines: lines,
		face:  face,
		col:   parseHexColor(el.Color),
		align: el.Align,
		lineH: lh,
	}, nil
}

// computeHeight calculates the total canvas height for the label.
func computeHeight(textElems []resolvedText, qrSize, barcodeH int, barcodeID string, pad int) int {
	textH := pad
	for _, te := range textElems {
		textH += len(te.lines) * te.lineH
	}
	textH += pad

	totalH := textH
	if qrSize > 0 {
		totalH = max(totalH, qrSize+pad*2)
	}
	if barcodeH > 0 && barcodeID != "" {
		totalH += barcodeH + pad
	}
	return totalH
}

// drawTextElements renders all resolved text rows onto the image.
func drawTextElements(img *image.RGBA, textElems []resolvedText, widthPx, pad, qrSize int) {
	y := pad
	for _, te := range textElems {
		for _, line := range te.lines {
			baseline := y + te.lineH
			x := alignedX(te.face, line, te.align, widthPx, pad, qrSize)
			drawTextFace(img, x, baseline, line, te.col, te.face)
			y += te.lineH
		}
	}
}

// alignedX returns the x coordinate for a text line according to the alignment setting.
func alignedX(face font.Face, line, align string, widthPx, pad, qrSize int) int {
	textAreaW := widthPx - pad*2
	if qrSize > 0 {
		textAreaW -= qrSize + pad
	}

	switch align {
	case "center":
		w := font.MeasureString(face, line).Ceil()
		x := pad + (textAreaW-w)/2
		if x < pad {
			return pad
		}
		return x
	case "right":
		w := font.MeasureString(face, line).Ceil()
		x := pad + textAreaW - w
		if x < pad {
			return pad
		}
		return x
	default: // "left"
		return pad
	}
}

// maybeDrawQR draws the QR code at top-right if both size and content are set.
func maybeDrawQR(img *image.RGBA, qrContent string, widthPx, pad, qrSize int) error {
	if qrSize <= 0 || qrContent == "" {
		return nil
	}
	return drawQR(img, qrContent, widthPx-qrSize-pad, pad, qrSize)
}

// maybeDrawBarcode draws the barcode at the bottom if height and ID are set.
func maybeDrawBarcode(img *image.RGBA, barcodeID string, widthPx, pad, barcodeH, totalH int) error {
	if barcodeH <= 0 || barcodeID == "" {
		return nil
	}
	barcodeY := totalH - barcodeH - pad
	return drawBarcode(img, barcodeID, pad, barcodeY, widthPx-pad*2, barcodeH)
}

// drawTextFace draws text at (x, y) where y is the baseline, using the given font face.
func drawTextFace(img *image.RGBA, x, y int, text string, col color.Color, face font.Face) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

// qrSizeForSchema returns the QR size defined in the schema, or 0 if none.
func qrSizeForSchema(schema Schema) int {
	for _, el := range schema.Elements {
		if el.Slot == "qr" {
			return el.Size
		}
	}
	return 0
}
