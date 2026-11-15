package label

import (
	"fmt"
	"image"
	"strings"

	"golang.org/x/image/font"
)

// Layout of the paired labels. The design is specified in px at 300 dpi on a
// 696x271 canvas (62x29 mm). Chrome (insets, bar, gaps) scales with
// u = min(W/696, H/271); fonts are specified in the same 300 dpi px but follow
// the DPI, so they keep their physical size (mm) on every printer.

const (
	pairDesignW = 696
	pairDesignH = 271

	// Fonts, in px at 300 dpi.
	pairBarFont      = 24
	pairNameMax      = 100 // name alone
	pairNameMaxDesc  = 84  // name sharing the label with the description
	pairNameMin      = 34  // floor of the name when alone
	pairNameMinDesc  = 44  // floor of the name when the description shares the label
	pairDescMin1     = 24  // description on label 1/2
	pairDescMax1     = 30
	pairLocMax       = 40 // breadcrumb on label 2/2
	pairLocMin       = 20 // tight floor
	pairTagFont      = 25
	pairAbsFloor     = 19 // 1.6 mm, absolute legibility floor for any text
	pairBoldBelow    = 21 // regular weight below this switches to bold
	pairNameLines    = 3  // with description
	pairNameLinesAlo = 4  // alone
	pairDescLines1   = 4
	pairDescLines2   = 5
	pairTagRowsMax   = 3
)

// PairWarning flags a last-resort shortening applied to content that does not
// fit even at the legibility floors. The escalation order is: the name is never
// cut; tag pills shrink (25 to 22 to 20 px at 300 dpi); the description is cut
// to at least one line; leading breadcrumb levels become "…"; the description is
// dropped; tag rows are dropped; finally PairWarnOverflow is set.
type PairWarning string

// Warnings reported by RenderPair.
const (
	// PairWarnDescriptionShortened: the description was cut with an ellipsis
	// (at least one line is kept) or dropped.
	PairWarnDescriptionShortened PairWarning = "description_shortened"
	// PairWarnLocationShortened: leading breadcrumb levels were replaced by "…".
	PairWarnLocationShortened PairWarning = "location_shortened"
	// PairWarnTagsDropped: tags beyond the available rows were left out, or a
	// tag wider than the label was cut.
	PairWarnTagsDropped PairWarning = "tags_dropped"
	// PairWarnNameSplit: the name contains a word wider than the label, which
	// had to be split across lines.
	PairWarnNameSplit PairWarning = "name_word_split"
	// PairWarnOverflow: the content still does not fit (input far outside the
	// documented ranges); it may be clipped by the label edge.
	PairWarnOverflow PairWarning = "content_overflow"
)

// pairGeom holds the canvas geometry in output pixels.
type pairGeom struct {
	w, h, dpi int
	num, den  int // u = num/den

	bar                 image.Rectangle
	barR                int
	tx                  int // text column
	bodyTop, bodyBottom int
	bodyX0, bodyX1      int
	bodyW, bodyH        int
	absFloor, boldBelow int
	gapText, gapTags    int
	tagGap, tagPad      int
	tagOutline          int
}

func newPairGeom(media MediaInfo) (pairGeom, error) {
	w, h, dpi := media.WidthPx, media.HeightPx, media.DPI
	if w <= 0 {
		return pairGeom{}, fmt.Errorf("pair template needs a canvas width, got %d", w)
	}
	if h <= 0 { // continuous media: use the 62x29 aspect ratio
		h = (w*pairDesignH + pairDesignW/2) / pairDesignW
	}
	if dpi <= 0 {
		dpi = 300
		if w <= 384 {
			dpi = 203
		}
	}
	g := pairGeom{w: w, h: h, dpi: dpi}
	if w*pairDesignH <= h*pairDesignW { // width limits u
		g.num, g.den = w, pairDesignW
	} else {
		g.num, g.den = h, pairDesignH
	}
	g.bar = image.Rect(g.s(24), g.s(18), w-g.s(24), g.s(18)+g.s(40))
	g.barR = g.s(10)
	g.tx = g.s(40)
	g.bodyTop = g.bar.Max.Y + g.s(12)
	g.bodyBottom = h - g.s(20)
	g.bodyX0, g.bodyX1 = g.tx, w-g.tx
	g.bodyW, g.bodyH = g.bodyX1-g.bodyX0, g.bodyBottom-g.bodyTop
	g.absFloor, g.boldBelow = g.fpx(pairAbsFloor), g.fpx(pairBoldBelow)
	g.gapText, g.gapTags = g.s(10), g.s(8)
	g.tagGap = g.fpx(7)
	g.tagPad = g.fpx(12)
	g.tagOutline = max(2, g.fpx(3))
	if g.bodyW < 100 || g.bodyH < 40 {
		return pairGeom{}, fmt.Errorf("canvas %dx%d too small for the pair template", w, h)
	}
	return g, nil
}

// s scales a chrome length given in design px.
func (g pairGeom) s(v int) int { return (v*g.num + g.den/2) / g.den }

// pillH is the height of a tag pill with font size px.
func (g pairGeom) pillH(px int) int { return px * 31 / 20 }

// fpx converts a font size given in px at 300 dpi to output px.
func (g pairGeom) fpx(px300 int) int { return (px300*g.dpi + 150) / 300 }

// step returns the output size stepping used by the size searches: 2 px at
// 300 dpi, scaled by DPI (at least 1).
func (g pairGeom) step() int { return max(1, g.fpx(2)) }

// pairBlock is a block of text lines. Line i has its baseline at
// rect.Min.Y + base + i*step.
type pairBlock struct {
	kind       string
	px         int
	step, base int
	lines      [][]pairRun
	rect       image.Rectangle
}

// pairStep is the baseline distance for a block kind.
func pairStep(kind string, px int) int {
	if kind == "name" {
		return px * 23 / 20
	}
	return px * 6 / 5
}

// pairBlockHeight is the height of n lines of the given kind.
func pairBlockHeight(kind string, px, n int) int {
	if n <= 0 {
		return 0
	}
	return (n-1)*pairStep(kind, px) + px*11/10
}

func (g pairGeom) newBlock(kind string, px, y int, lines [][]pairRun) pairBlock {
	return pairBlock{
		kind: kind, px: px, step: pairStep(kind, px), base: px * 4 / 5, lines: lines,
		rect: image.Rect(g.bodyX0, y, g.bodyX1, y+pairBlockHeight(kind, px, len(lines))),
	}
}

func singleRuns(lines []string, bold bool) [][]pairRun {
	out := make([][]pairRun, len(lines))
	for i, l := range lines {
		out[i] = []pairRun{{l, bold}}
	}
	return out
}

// pairPill is one outlined tag.
type pairPill struct {
	text     string
	px       int
	bold     bool
	rect     image.Rectangle
	textX    int
	baseline int
}

// pairBar describes the black header bar.
type pairBar struct {
	rect          image.Rectangle
	hint, counter string
	date          string
	icon          string
}

// pairPage is the complete layout of one label.
type pairPage struct {
	bar    pairBar
	blocks []pairBlock
	pills  []pairPill
}

// pairPlanner lays out both pages of one item.
type pairPlanner struct {
	g     pairGeom
	m     *pairMeasurer
	warns []PairWarning
}

// dropWarn removes a warning again (used when a layout attempt is abandoned).
func (p *pairPlanner) dropWarn(w PairWarning) {
	out := p.warns[:0]
	for _, x := range p.warns {
		if x != w {
			out = append(out, x)
		}
	}
	p.warns = out
}

func (p *pairPlanner) warn(w PairWarning) {
	for _, x := range p.warns {
		if x == w {
			return
		}
	}
	p.warns = append(p.warns, w)
}

// pairBold reports whether regular text at px must be drawn bold to stay legible.
func (p *pairPlanner) pairBold(px int) bool { return px < p.g.boldBelow }

// locationLevels splits the container path ("Room → Shelf") into levels.
func locationLevels(loc string) []string {
	var out []string
	for _, l := range strings.Split(loc, "→") {
		if l = pairNormalize(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// planIdentity lays out label 1/2: the name, and the description when
// withDesc is set and it fits next to a name of at least the floor size.
// shown reports whether the description ended up on this label.
func (p *pairPlanner) planIdentity(name, desc string, withDesc bool) (blocks []pairBlock, shown bool) {
	g, m := p.g, p.m
	st := g.step()
	if withDesc {
		for px := g.fpx(pairNameMaxDesc); px >= g.fpx(pairNameMinDesc); px -= st {
			nl, ok := m.wrap(true, px, name, g.bodyW, pairNameLines, false)
			if !ok {
				continue
			}
			dp := min(max(px/2, g.fpx(pairDescMin1)), g.fpx(pairDescMax1))
			dl, ok := m.wrap(false, dp, desc, g.bodyW, pairDescLines1, false)
			if !ok {
				continue
			}
			if pairBlockHeight("name", px, len(nl))+g.gapText+pairBlockHeight("desc", dp, len(dl)) > g.bodyH {
				continue
			}
			nb := g.newBlock("name", px, g.bodyTop, singleRuns(nl, true))
			db := g.newBlock("desc", dp, nb.rect.Max.Y+g.gapText, singleRuns(dl, false))
			return []pairBlock{nb, db}, true
		}
	}
	return []pairBlock{p.planNameAlone(name)}, false
}

// planNameAlone sizes the name on its own: from the large end down to the
// floor in steps, then (input outside the documented range) below the floor
// down to the absolute floor, and only then by splitting a too wide word.
func (p *pairPlanner) planNameAlone(name string) pairBlock {
	g, m := p.g, p.m
	st := g.step()
	for px := g.fpx(pairNameMax); px >= g.fpx(pairNameMin); px -= st {
		if nl, ok := m.wrap(true, px, name, g.bodyW, pairNameLinesAlo, false); ok &&
			pairBlockHeight("name", px, len(nl)) <= g.bodyH {
			return g.newBlock("name", px, g.bodyTop, singleRuns(nl, true))
		}
	}
	for px := g.fpx(pairNameMin) - 1; px >= g.absFloor; px-- {
		if nl, ok := m.wrap(true, px, name, g.bodyW, 0, false); ok &&
			pairBlockHeight("name", px, len(nl)) <= g.bodyH {
			return g.newBlock("name", px, g.bodyTop, singleRuns(nl, true))
		}
	}
	px := g.absFloor
	nl, _ := m.wrap(true, px, name, g.bodyW, 0, true)
	if toks, ok := m.fitTokens(true, px, name, g.bodyW, false); !ok || toks == nil {
		p.warn(PairWarnNameSplit)
	}
	b := g.newBlock("name", px, g.bodyTop, singleRuns(nl, true))
	if b.rect.Max.Y > g.bodyBottom {
		p.warn(PairWarnOverflow)
	}
	return b
}

// layoutPills places the tags as outlined pills in rows, bottom aligned in the
// body. At most maxRows rows are used (extra tags are dropped); a tag wider
// than the body is cut.
func (p *pairPlanner) layoutPills(tags []LabelTag, maxRows, px int) []pairPill {
	g, m := p.g, p.m
	tagH := g.pillH(px)
	bold := p.pairBold(px)
	capH := pairCapHeight(px, bold)
	type placed struct {
		text string
		w    int
		row  int
		x    int
	}
	var pl []placed
	row, x := 0, 0
	for _, t := range tags {
		text := pairNormalize(t.Name)
		if text == "" {
			continue
		}
		if m.width(bold, px, text)+2*g.tagPad > g.bodyW {
			text = m.ellipsize(bold, px, text, g.bodyW-2*g.tagPad)
			p.warn(PairWarnTagsDropped)
		}
		w := m.width(bold, px, text) + 2*g.tagPad
		if x > 0 && x+w > g.bodyW {
			row++
			x = 0
		}
		if row >= maxRows {
			p.warn(PairWarnTagsDropped)
			break
		}
		pl = append(pl, placed{text, w, row, x})
		x += w + g.fpx(10)
	}
	if len(pl) == 0 {
		return nil
	}
	rows := pl[len(pl)-1].row + 1
	top := g.bodyBottom - (rows*tagH + (rows-1)*g.tagGap)
	out := make([]pairPill, len(pl))
	for i, t := range pl {
		y0 := top + t.row*(tagH+g.tagGap)
		out[i] = pairPill{
			text: t.text, px: px, bold: bold,
			rect:     image.Rect(g.bodyX0+t.x, y0, g.bodyX0+t.x+t.w, y0+tagH),
			textX:    g.bodyX0 + t.x + g.tagPad,
			baseline: y0 + (tagH+capH)/2,
		}
	}
	return out
}

// pairCapHeight returns the cap height of the face in px.
func pairCapHeight(px int, bold bool) int {
	b, _ := font.BoundString(pairFace(bold, px), "H")
	return max(1, -b.Min.Y.Floor())
}

// contextCounts builds "Zawiera: N pojemników · M przedmiotów".
func contextCounts(children []LabelChild) string {
	var nc, ni int
	for _, c := range children {
		if c.IsContainer {
			nc++
		} else {
			ni++
		}
	}
	var parts []string
	if nc > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", nc, pairPlural(nc, "pojemnik", "pojemniki", "pojemników")))
	}
	if ni > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", ni, pairPlural(ni, "przedmiot", "przedmioty", "przedmiotów")))
	}
	if len(parts) == 0 {
		return ""
	}
	return "Zawiera: " + strings.Join(parts, " · ")
}

// locationParts returns the breadcrumb parts: every level followed by " ›",
// the last one bold without a separator (all bold when allBold: regular weight
// is too thin at that size).
func locationParts(levels []string, allBold bool) []flowPart {
	parts := make([]flowPart, len(levels))
	for i, l := range levels {
		last := i == len(levels)-1
		parts[i] = flowPart{text: l, bold: last || allBold}
		if !last {
			parts[i].suffix = " ›"
		}
	}
	return parts
}

// ctx2 collects the variable pieces of label 2/2 during planning.
type ctx2 struct {
	levels   []string
	desc     string
	counts   string
	children []LabelChild
	avail    int // body height above the tags
}

// planContext lays out label 2/2 (top aligned, tags pinned to the bottom).
// When the content does not fit, the escalation is: smaller tag pills (down to
// the floor), then the last resorts of lastResort (description, location),
// and only then tag rows are dropped, one at a time.
func (p *pairPlanner) planContext(d LabelData, desc string) pairPage {
	g := p.g
	newCtx := func(pills []pairPill) ctx2 {
		c := ctx2{levels: locationLevels(d.Location), desc: desc, counts: contextCounts(d.Children), children: d.Children, avail: g.bodyH}
		if len(pills) > 0 {
			c.avail = pills[0].rect.Min.Y - g.gapTags - g.bodyTop
		}
		return c
	}
	sizes := []int{g.fpx(pairTagFont), g.fpx(22), g.fpx(pairLocMin)}
	wantTags := 0
	for _, t := range d.Tags {
		if pairNormalize(t.Name) != "" {
			wantTags++
		}
	}
	for i, px := range sizes {
		pills := p.layoutPills(d.Tags, pairTagRowsMax, px)
		if len(pills) < wantTags && i < len(sizes)-1 {
			p.dropWarn(PairWarnTagsDropped) // more rows than allowed: try smaller pills
			continue
		}
		if blocks, ok := p.fitNormal(newCtx(pills)); ok {
			return pairPage{blocks: blocks, pills: pills}
		}
		p.dropWarn(PairWarnTagsDropped) // layoutPills may have warned for a size we abandon
	}
	px := sizes[len(sizes)-1]
	var pg pairPage
	for rows := pairTagRowsMax; rows >= 1; rows-- {
		mark := len(p.warns)
		pills := p.layoutPills(d.Tags, rows, px)
		blocks, ok := p.lastResort(newCtx(pills))
		pg = pairPage{blocks: blocks, pills: pills}
		if ok || rows == 1 || len(pills) == 0 {
			if !ok {
				p.warn(PairWarnOverflow)
			}
			return pg
		}
		p.warns = p.warns[:mark] // retry with fewer tag rows
	}
	return pg
}

// stack stacks blocks (kind, px, lines) from the body top with gaps.
func (p *pairPlanner) stack(specs []pairBlock) []pairBlock {
	y := p.g.bodyTop
	out := make([]pairBlock, 0, len(specs))
	for _, s := range specs {
		if len(s.lines) == 0 {
			continue
		}
		b := p.g.newBlock(s.kind, s.px, y, s.lines)
		out = append(out, b)
		y = b.rect.Max.Y + p.g.gapText
	}
	return out
}

func stackHeight(g pairGeom, specs []pairBlock) int {
	h, n := 0, 0
	for _, s := range specs {
		if len(s.lines) == 0 {
			continue
		}
		h += pairBlockHeight(s.kind, s.px, len(s.lines))
		n++
	}
	if n > 1 {
		h += (n - 1) * g.gapText
	}
	return h
}

// contextSpecs computes the blocks of label 2/2 for a breadcrumb size px.
// ok is false when a word does not fit the width at that size.
func (p *pairPlanner) contextSpecs(c ctx2, locPx int, levels []string, descLines int, split bool) (specs []pairBlock, ok bool) {
	g, m := p.g, p.m
	if len(levels) > 0 {
		runs, ok := m.flowRuns(locPx, locationParts(levels, p.pairBold(locPx)), g.bodyW, split)
		if !ok {
			return nil, false
		}
		specs = append(specs, pairBlock{kind: "location", px: locPx, lines: m.flow(locPx, runs, g.bodyW)})
	}
	small := max(g.fpx(pairLocMin), locPx*7/10)
	if c.desc != "" {
		dl, ok := m.wrap(p.pairBold(small), small, c.desc, g.bodyW, 0, split)
		if !ok {
			return nil, false
		}
		if descLines > 0 && len(dl) > descLines {
			dl = p.cutLines(dl, descLines, small)
		}
		specs = append(specs, pairBlock{kind: "desc", px: small, lines: singleRuns(dl, p.pairBold(small))})
	}
	if c.counts != "" {
		cl, ok := m.wrap(p.pairBold(small), small, c.counts, g.bodyW, 0, split)
		if !ok {
			return nil, false
		}
		specs = append(specs, pairBlock{kind: "counts", px: small, lines: singleRuns(cl, p.pairBold(small))})
	}
	return specs, true
}

// cutLines keeps the first n lines and ends the last one with an ellipsis.
func (p *pairPlanner) cutLines(lines []string, n, px int) []string {
	out := append([]string(nil), lines[:n]...)
	out[n-1] = p.m.ellipsize(p.pairBold(px), px, out[n-1]+" …", p.g.bodyW)
	if out[n-1] == "" {
		out[n-1] = pairEllipsis
	}
	return out
}

// fitNormal picks the largest breadcrumb size at which everything fits within
// the floors.
func (p *pairPlanner) fitNormal(c ctx2) (blocks []pairBlock, ok bool) {
	g := p.g
	for px := g.fpx(pairLocMax); px >= g.fpx(pairLocMin); px-- {
		specs, ok := p.contextSpecs(c, px, c.levels, 0, false)
		if ok && stackHeight(g, specs) <= c.avail {
			return p.withChildren(c, px, specs), true
		}
	}
	return nil, false
}

// withChildren appends the flowing list of child names when it fits entirely
// at or above the floor size; otherwise the counts alone stay.
func (p *pairPlanner) withChildren(c ctx2, locPx int, specs []pairBlock) []pairBlock {
	g, m := p.g, p.m
	if len(c.children) == 0 || c.counts == "" {
		return p.stack(specs)
	}
	startPx := specs[len(specs)-1].px // counts block is last
	for q := startPx; q >= g.fpx(pairLocMin); q-- {
		parts := make([]flowPart, len(c.children))
		for i, ch := range c.children {
			parts[i] = flowPart{text: pairNormalize(ch.Name), bold: ch.IsContainer || p.pairBold(q)}
			if i < len(c.children)-1 {
				parts[i].suffix = " ·"
			}
		}
		runs, ok := m.flowRuns(q, parts, g.bodyW, false)
		if !ok {
			continue
		}
		withList := append([]pairBlock(nil), specs...)
		cnt := &withList[len(withList)-1]
		cl, ok := m.wrap(p.pairBold(q), q, c.counts, g.bodyW, 0, false)
		if !ok {
			continue
		}
		cnt.px, cnt.lines = q, singleRuns(cl, p.pairBold(q))
		withList = append(withList, pairBlock{kind: "children", px: q, lines: m.flow(q, runs, g.bodyW)})
		if stackHeight(g, withList) <= c.avail {
			return p.stack(withList)
		}
	}
	return p.stack(specs)
}
