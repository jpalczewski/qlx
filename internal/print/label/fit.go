package label

import (
	"sort"
	"strings"

	"golang.org/x/image/font"
)

// Text fitting.
//
// An element with `"fit": "fill"` is sized to the printable area of a die-cut
// label instead of using a fixed font_size. The renderer picks the largest
// integer pixel size (so the font face cache stays bounded) that fits, using
// an outline font:
//
//  1. shrink a single line down to the minimum size,
//  2. then allow a second line (largest size that wraps to at most two lines),
//  3. then keep the minimum size and truncate with an ellipsis.
//
// Continuous media (HeightPx == 0) has no height to fill, so fit is ignored
// there and the element renders at its font_size.

const (
	// fitDefaultFont is used for fitted text when neither element nor schema
	// names a font: Noto Sans Bold has the thick strokes thermal printers need.
	fitDefaultFont = "noto-sans-bold"

	fitFloorSize  = 8   // never go below this many pixels
	fitMinDefault = 16  // lower bound of the default minimum size
	fitMaxCap     = 600 // absolute upper bound of the search range
	fitMaxLines   = 2
	ellipsis      = "..."
)

// fitEnabled reports whether the element asks for fitted text.
func fitEnabled(el Element) bool {
	if el.Fit != "fill" {
		return false
	}
	switch el.Slot {
	case "title", "description", "location":
	default:
		return false
	}
	return true
}

// fitArea describes the die-cut label geometry available to fitted text.
type fitArea struct {
	widthPx, totalH, pad int
	qrReserved, qrSize   int
	barcodeH             int
	barcodeID            string
	dateLineH            int
}

// fitElements replaces every fit-enabled element with its fitted layout.
// The vertical space left by the other elements is shared equally between
// the fit elements.
func fitElements(elems []resolvedText, a fitArea) []resolvedText {
	fitCount := 0
	fixedH := a.pad * 2
	for _, e := range elems {
		if e.fit && canFit(e) {
			fitCount++
		} else {
			fixedH += e.height()
		}
	}
	if fitCount == 0 {
		return elems
	}
	if a.barcodeH > 0 && a.barcodeID != "" {
		fixedH += a.barcodeH + a.pad
	}

	textW := a.widthPx - a.pad*2
	if a.qrReserved > 0 {
		textW -= a.qrReserved + a.pad
	}

	elems, fixedH = dropLowPriority(elems, a, textW, fixedH)
	remaining := a.totalH - a.dateLineH - fixedH

	out := make([]resolvedText, len(elems))
	copy(out, elems)
	left := fitCount
	for i, e := range out {
		if !e.fit || !canFit(e) {
			continue
		}
		share := remaining / left
		fitted := fitText(e, textW, share)
		out[i] = fitted
		remaining -= fitted.height()
		left--
	}
	return out
}

// dropLowPriority removes trailing tags and children (the lowest priority, see
// truncateForDieCut) while the fit elements could not keep their minimum
// height next to them. It returns the remaining elements and fixed height.
func dropLowPriority(elems []resolvedText, a fitArea, textW, fixedH int) ([]resolvedText, int) {
	minNeed := 0
	for _, e := range elems {
		if e.fit && canFit(e) {
			minNeed += reserveHeight(e, textW, a.totalH-a.dateLineH-a.pad*2, false)
		}
	}
	for len(elems) > 1 {
		last := elems[len(elems)-1]
		if (last.slot != "tags" && last.slot != "children") || fixedH+minNeed <= a.totalH-a.dateLineH {
			break
		}
		fixedH -= last.height()
		elems = elems[:len(elems)-1]
	}
	return elems, fixedH
}

// reserveHeight returns the height one line of the element needs at its
// minimum size (or, with useMax, at its explicit max_size).
func reserveHeight(e resolvedText, textW, availH int, useMax bool) int {
	size := e.elem.MinSize
	if useMax && e.elem.MaxSize > 0 {
		size = e.elem.MaxSize
	}
	if size <= 0 {
		size = max(fitMinDefault, availH/4)
	}
	m, ok := measure(e, min(size, fitMaxCap), textW, false)
	if !ok {
		return 0
	}
	return m.blockH
}

// canFit reports whether a resolved element can be fitted: it needs text and
// a scalable font.
func canFit(e resolvedText) bool {
	if e.text == "" || e.family == "" {
		return false
	}
	if IsBasicFont(e.family) {
		return false
	}
	entry, ok := fontCatalog[e.family]
	return ok && !entry.bitmap
}

// fitMetrics describes the measured layout of text at one size.
type fitMetrics struct {
	face     font.Face
	lines    []string
	top      int // cap/accent height above the first baseline
	bottom   int // descender extent below the last baseline
	step     int // distance between baselines
	iconSize int // icon edge in px (cap height); the icon is drawn at exactly this size
	iconW    int // horizontal space reserved for the icon
	blockH   int
	maxLine  int // widest line in px
}

// measure lays the text out at the given pixel size.
func measure(e resolvedText, size, textW int, wrap bool) (fitMetrics, bool) {
	face, err := LoadFace(e.family, float64(size))
	if err != nil {
		return fitMetrics{}, false
	}
	m := fitMetrics{face: face}

	capBounds, _ := font.BoundString(face, "H")
	capH := max(-capBounds.Min.Y.Floor(), 1)
	m.top = capH
	m.step = size * 5 / 4

	availW := textW
	if e.iconName != "" {
		m.iconSize = capH
		m.iconW = capH + 4
		availW -= m.iconW
	}
	if availW < 1 {
		availW = 1
	}

	if wrap {
		m.lines = wrapText(e.text, face, availW)
	} else {
		m.lines = []string{e.text}
	}
	for _, l := range m.lines {
		b, adv := font.BoundString(face, l)
		m.top = max(m.top, -b.Min.Y.Floor())
		m.bottom = max(m.bottom, b.Max.Y.Ceil())
		m.maxLine = max(m.maxLine, max(adv.Ceil(), b.Max.X.Ceil()))
	}
	m.bottom = max(m.bottom, 0)
	if len(m.lines) == 0 {
		return m, false
	}
	m.blockH = m.top + (len(m.lines)-1)*m.step + m.bottom
	return m, true
}

// fits reports whether the layout stays within the width and height limits
// and within the maximum number of lines.
func (m fitMetrics) fits(textW, availH, maxLines int) bool {
	return len(m.lines) <= maxLines && m.maxLine <= textW-m.iconW && m.blockH <= availH
}

// largestSize returns the largest size in [lo, hi] for which ok holds,
// assuming ok is (approximately) monotonic: true for small sizes, false for
// large ones. Returns 0 if ok(lo) is false.
func largestSize(lo, hi int, ok func(int) bool) int {
	if lo > hi || !ok(lo) {
		return 0
	}
	n := sort.Search(hi-lo+1, func(i int) bool { return !ok(lo + i) })
	return lo + n - 1
}

// fitText computes the fitted layout of one element for a box of textW x availH.
func fitText(e resolvedText, textW, availH int) resolvedText {
	lo := e.elem.MinSize
	if lo <= 0 {
		lo = max(fitMinDefault, availH/4)
	}
	hi := e.elem.MaxSize
	if hi <= 0 {
		hi = availH * 6 / 5
	}
	lo = min(lo, fitMaxCap)
	hi = min(max(hi, lo), fitMaxCap)

	// A minimum that cannot even hold one line by height is lowered until it does.
	for lo > fitFloorSize {
		if m, ok := measure(e, lo, textW, false); ok && m.blockH <= availH {
			break
		}
		lo--
	}
	hi = max(hi, lo)

	single := func(size int) bool {
		m, ok := measure(e, size, textW, false)
		return ok && m.fits(textW, availH, 1)
	}
	double := func(size int) bool {
		m, ok := measure(e, size, textW, true)
		return ok && m.fits(textW, availH, fitMaxLines)
	}

	if size := largestSize(lo, hi, single); size > 0 {
		m, _ := measure(e, size, textW, false)
		return applyFitMetrics(e, m)
	}
	if size := largestSize(lo, hi, double); size > 0 {
		m, _ := measure(e, size, textW, true)
		return applyFitMetrics(e, m)
	}

	// Minimum size, wrapped, then truncated to what fits.
	m, ok := measure(e, lo, textW, true)
	if !ok {
		return e
	}
	allowed := 1
	for n := fitMaxLines; n > 1; n-- {
		if m.top+(n-1)*m.step+m.bottom <= availH {
			allowed = n
			break
		}
	}
	lineW := textW - m.iconW
	if len(m.lines) > allowed {
		rest := strings.Join(m.lines[allowed-1:], " ")
		m.lines = append(m.lines[:allowed-1:allowed-1], rest)
	}
	last := len(m.lines) - 1
	m.lines[last] = ellipsize(m.lines[last], m.face, lineW)
	m.blockH = m.top + last*m.step + m.bottom
	return applyFitMetrics(e, m)
}

// applyFitMetrics copies the measured layout into the resolved element.
func applyFitMetrics(e resolvedText, m fitMetrics) resolvedText {
	e.face = m.face
	e.lines = m.lines
	e.lineH = m.step
	e.firstBaseline = m.top
	e.iconSize = m.iconSize
	e.bottom = m.bottom
	return e
}

// ellipsize shortens text (with a trailing "...") until it fits maxWidth.
// Text that already fits is returned unchanged.
func ellipsize(text string, face font.Face, maxWidth int) string {
	if font.MeasureString(face, text).Ceil() <= maxWidth {
		return text
	}
	runes := []rune(strings.TrimSpace(text))
	for len(runes) > 0 {
		cand := strings.TrimRight(string(runes), " ") + ellipsis
		if font.MeasureString(face, cand).Ceil() <= maxWidth {
			return cand
		}
		runes = runes[:len(runes)-1]
	}
	return ellipsis
}
