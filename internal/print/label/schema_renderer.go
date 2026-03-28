package label

import (
	"image"
	"image/color"
	"image/draw"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// resolvedText holds a fully prepared text element ready to be drawn.
type resolvedText struct {
	lines    []string
	face     font.Face
	col      color.RGBA
	align    string
	lineH    int
	iconName string // single icon for title slot
	slot     string // "title", "description", "location", "tags", "children"
}

// effectiveFontFamily returns the font to use for an element.
func effectiveFontFamily(el Element, schema Schema) string {
	if el.FontFamily != "" {
		return el.FontFamily
	}
	if schema.FontFamily != "" {
		return schema.FontFamily
	}
	return "spleen"
}

// showIcons returns whether icons should be rendered for this element.
func showIcons(el Element) bool {
	if el.ShowIcons != nil {
		return *el.ShowIcons
	}
	return el.Slot == "title" || el.Slot == "tags" || el.Slot == "children"
}

// formatTags formats tag list as text. ShowPath controls ancestor display.
// "true": always show path "#elektronika>arduino"
// "false": leaf only "#arduino"
// "auto": try full path, fallback to leaf if text is too wide.
func formatTags(tags []LabelTag, el Element, face font.Face, maxWidth int) string {
	if len(tags) == 0 {
		return ""
	}

	switch el.ShowPath {
	case "true":
		return formatTagsWithPath(tags)
	case "false":
		return formatTagsLeafOnly(tags)
	default: // "auto"
		full := formatTagsWithPath(tags)
		w := font.MeasureString(face, full).Ceil()
		if w <= maxWidth {
			return full
		}
		return formatTagsLeafOnly(tags)
	}
}

// formatTagsWithPath formats tags with full ancestor path.
func formatTagsWithPath(tags []LabelTag) string {
	parts := make([]string, len(tags))
	for i, tag := range tags {
		if len(tag.Path) > 1 {
			parts[i] = "#" + strings.Join(tag.Path, ">")
		} else {
			parts[i] = "#" + tag.Name
		}
	}
	return strings.Join(parts, " ")
}

// formatTagsLeafOnly formats tags showing only leaf names.
func formatTagsLeafOnly(tags []LabelTag) string {
	parts := make([]string, len(tags))
	for i, tag := range tags {
		parts[i] = "#" + tag.Name
	}
	return strings.Join(parts, " ")
}

// formatChildren formats children list as comma-separated text.
func formatChildren(children []LabelChild) string {
	if len(children) == 0 {
		return ""
	}
	parts := make([]string, len(children))
	for i, c := range children {
		parts[i] = c.Name
	}
	return strings.Join(parts, ", ")
}

// renderSchema renders a label image from a Schema definition and LabelData.
// Elements are laid out vertically. QR is positioned at top-right, barcode at full-width bottom.
// Note: QR is always placed at top-right when present — vertical QR placement is not supported.
func renderSchema(schema Schema, data LabelData, media MediaInfo, opts RenderOpts) (image.Image, error) {
	widthPx := media.WidthPx
	pad := schema.Padding
	qrReserved := qrSizeForSchema(schema)

	textElems, qrSize, barcodeH, err := resolveElements(schema, data, widthPx, pad, qrReserved)
	if err != nil {
		return nil, err
	}

	// Prepare optional print-date line.
	var dateLine resolvedText
	var dateLineH int
	if opts.PrintDate {
		var pErr error
		dateLine, pErr = buildDateLine()
		if pErr != nil {
			return nil, pErr
		}
		dateLineH = dateLine.lineH + 2 // 2px gap above the date
	}

	contentH := computeHeight(textElems, qrSize, barcodeH, data.BarcodeID, pad) + dateLineH

	totalH := contentH
	if media.HeightPx > 0 {
		totalH = media.HeightPx
		// Die-cut: if content overflows, truncate elements by priority
		if contentH > totalH {
			textElems = truncateForDieCut(textElems, qrSize, barcodeH, data.BarcodeID, pad, dateLineH, totalH)
		}
	}

	img := newCanvas(widthPx, totalH)

	drawTextElements(img, textElems, widthPx, pad, qrSize)

	if err := maybeDrawQR(img, data.QRContent, widthPx, pad, qrSize); err != nil {
		return nil, err
	}
	if err := maybeDrawBarcode(img, data.BarcodeID, widthPx, pad, barcodeH, totalH-dateLineH); err != nil {
		return nil, err
	}

	// Draw date line at the very bottom.
	if opts.PrintDate {
		drawDateLine(img, dateLine, widthPx, pad, totalH)
	}

	return img, nil
}

// buildDateLine creates a resolvedText for the print-date footer.
func buildDateLine() (resolvedText, error) {
	face, err := LoadFace("basic", 13)
	if err != nil {
		return resolvedText{}, err
	}
	text := TransliteratePL("Wydrukowano: " + nowFunc().Format("2006-01-02 15:04"))
	metrics := face.Metrics()
	lh := (metrics.Ascent + metrics.Descent).Ceil()
	return resolvedText{
		lines: []string{text},
		face:  face,
		col:   parseHexColor("#808080"),
		align: "left",
		lineH: lh,
	}, nil
}

// drawDateLine renders the print-date footer at the bottom of the label.
func drawDateLine(img *image.RGBA, dl resolvedText, widthPx, pad, totalH int) {
	baseline := totalH - pad
	x := alignedX(dl.face, dl.lines[0], dl.align, widthPx, pad, 0)
	drawTextFace(img, x, baseline, dl.lines[0], dl.col, dl.face)
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
			rt, err := resolveSlotText(el, slotText[el.Slot], schema, widthPx, pad, qrReserved)
			if err != nil {
				return nil, 0, 0, err
			}
			if el.Slot == "title" && data.Icon != "" && showIcons(el) {
				rt.iconName = data.Icon
			}
			rt.slot = el.Slot
			textElems = append(textElems, rt)

		case "tags":
			if rt, ok, err := resolveTagsSlot(el, data.Tags, schema, widthPx, pad, qrReserved); err != nil {
				return nil, 0, 0, err
			} else if ok {
				rt.slot = "tags"
				textElems = append(textElems, rt)
			}

		case "children":
			if rt, ok, err := resolveChildrenSlot(el, data.Children, schema, widthPx, pad, qrReserved); err != nil {
				return nil, 0, 0, err
			} else if ok {
				rt.slot = "children"
				textElems = append(textElems, rt)
			}

		case "qr":
			qrSize = el.Size
		case "barcode":
			barcodeH = el.Height
		}
	}

	return textElems, qrSize, barcodeH, nil
}

// resolveSlotText resolves a simple text slot (title, description, location).
func resolveSlotText(el Element, text string, schema Schema, widthPx, pad, qrReserved int) (resolvedText, error) {
	ff := effectiveFontFamily(el, schema)
	if IsBasicFont(ff) {
		text = TransliteratePL(text)
	}
	return resolveTextElement(el, text, ff, widthPx, pad, qrReserved)
}

// resolveTagsSlot resolves a tags slot element. Returns (rt, true, nil) if tags were rendered,
// (zero, false, nil) if no tags, or (zero, false, err) on error.
func resolveTagsSlot(el Element, tags []LabelTag, schema Schema, widthPx, pad, qrReserved int) (resolvedText, bool, error) {
	if len(tags) == 0 {
		return resolvedText{}, false, nil
	}
	ff := effectiveFontFamily(el, schema)
	face, err := LoadFace(ff, el.FontSize)
	if err != nil {
		return resolvedText{}, false, err
	}
	maxW := widthPx - pad*2
	if qrReserved > 0 {
		maxW -= qrReserved + pad
	}
	text := formatTags(tags, el, face, maxW)
	if IsBasicFont(ff) {
		text = TransliteratePL(text)
	}
	rt, err := resolveTextElement(el, text, ff, widthPx, pad, qrReserved)
	if err != nil {
		return resolvedText{}, false, err
	}
	return rt, true, nil
}

// resolveChildrenSlot resolves a children slot element. Returns (rt, true, nil) if children were rendered,
// (zero, false, nil) if no children, or (zero, false, err) on error.
func resolveChildrenSlot(el Element, children []LabelChild, schema Schema, widthPx, pad, qrReserved int) (resolvedText, bool, error) {
	if len(children) == 0 {
		return resolvedText{}, false, nil
	}
	ff := effectiveFontFamily(el, schema)
	text := formatChildren(children)
	if IsBasicFont(ff) {
		text = TransliteratePL(text)
	}
	rt, err := resolveTextElement(el, text, ff, widthPx, pad, qrReserved)
	if err != nil {
		return resolvedText{}, false, err
	}
	return rt, true, nil
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
		for i, line := range te.lines {
			baseline := y + te.lineH
			xOffset := 0

			// Draw inline icon before first line of title.
			if i == 0 && te.iconName != "" {
				iconSize := te.lineH - 2
				if iconSize > 0 {
					iconImg, err := RasterizeIcon(te.iconName, iconSize)
					if err == nil && iconImg != nil {
						iconY := y + 1
						draw.Draw(img, image.Rect(pad, iconY, pad+iconSize, iconY+iconSize),
							iconImg, image.Point{}, draw.Over)
						xOffset = iconSize + 4
					}
				}
			}

			x := alignedX(te.face, line, te.align, widthPx, pad, qrSize)
			drawTextFace(img, x+xOffset, baseline, line, te.col, te.face)
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

// truncateForDieCut removes/truncates elements by priority until content fits within maxH.
// Priority (lowest priority removed/truncated first):
//  1. tags — removed entirely
//  2. children — removed entirely
//  3. description — lines truncated with ellipsis, then removed
//  4. location — lines truncated with ellipsis, then removed
//  5. title — truncated with ellipsis as last resort
func truncateForDieCut(elems []resolvedText, qrSize, barcodeH int, barcodeID string, pad, dateLineH, maxH int) []resolvedText {
	fits := func() bool {
		return computeHeight(elems, qrSize, barcodeH, barcodeID, pad)+dateLineH <= maxH
	}

	// Phase 1: remove tags and children entirely (lowest priority).
	// This assumes the conventional schema element order: title → description → location → tags → children.
	// If a schema uses a different order, tags/children may not be at the tail and will not be removed here —
	// they will be caught by Phase 2 iteration which checks by slot name.
	for !fits() && len(elems) > 1 {
		last := elems[len(elems)-1]
		if last.slot != "tags" && last.slot != "children" {
			break
		}
		elems = elems[:len(elems)-1]
	}

	// Phase 2: truncate description and location line-by-line with ellipsis
	for !fits() {
		truncated := false
		for i := len(elems) - 1; i >= 0; i-- {
			s := elems[i].slot
			if s != "description" && s != "location" {
				continue
			}
			if len(elems[i].lines) > 1 {
				// Remove last line and add ellipsis to new last line
				elems[i].lines = elems[i].lines[:len(elems[i].lines)-1]
				last := elems[i].lines[len(elems[i].lines)-1]
				runes := []rune(last)
				if len(runes) > 3 {
					elems[i].lines[len(elems[i].lines)-1] = string(runes[:len(runes)-3]) + "..."
				}
				truncated = true
				break
			} else if len(elems[i].lines) == 1 {
				// Single line: remove entire element
				elems = append(elems[:i], elems[i+1:]...)
				truncated = true
				break
			}
		}
		if !fits() && !truncated {
			break // nothing more to remove in this phase
		}
	}

	// Phase 3: truncate title lines as last resort
	for !fits() && len(elems) > 0 && len(elems[0].lines) > 1 {
		lines := elems[0].lines
		elems[0].lines = lines[:len(lines)-1]
		last := elems[0].lines[len(elems[0].lines)-1]
		runes := []rune(last)
		if len(runes) > 3 {
			elems[0].lines[len(elems[0].lines)-1] = string(runes[:len(runes)-3]) + "..."
		}
	}
	// If a single-line title still overflows after truncation, it will be clipped
	// at the canvas edge by the draw package — this is intentional.

	return elems
}
