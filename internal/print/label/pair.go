package label

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"slices"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

// PairTemplate is the code-defined template that renders one item as a pair of
// labels: 1/2 identifies the item (name, optionally the description), 2/2
// gives its context (location, description if it did not fit on 1/2, tags and,
// for containers, what it holds). Both labels share one header bar ("twin
// bar"). It is not a JSON schema: GetSchema and Render do not know it, use
// RenderPages or RenderPair.
const PairTemplate = "pair"

// PairResult is the outcome of RenderPair.
type PairResult struct {
	// Pages holds exactly two images: identity (1/2) and context (2/2).
	Pages []image.Image
	// Warnings lists the last resorts that were applied because the content
	// did not fit at the legibility floors. It is empty for typical content.
	// Content at the upper end of the documented ranges (name ~71 chars,
	// description ~70, location ~125, 3 tags) can still trigger the description
	// and location shortenings when several fields are near their maximum at
	// once; on the small 384x160 B1 canvas this happens more often.
	Warnings []PairWarning
}

// RenderPages renders data with the named template and returns one image per
// label to print. Every schema template yields exactly one page, identical to
// Render; PairTemplate yields exactly two (page 1 = identity, page 2 =
// context), or only page 1 when opts.FirstPageOnly is set. Use RenderPair to also learn about last-resort warnings.
func RenderPages(data LabelData, template string, media MediaInfo, opts RenderOpts) ([]image.Image, error) {
	if template == PairTemplate {
		res, err := RenderPair(data, media, opts)
		if err != nil {
			return nil, err
		}
		if len(res.Warnings) > 0 {
			webutil.LogInfo("label: pair template used last resorts for %q: %v", data.Name, res.Warnings)
		}
		if opts.FirstPageOnly {
			return res.Pages[:1], nil
		}
		return res.Pages, nil
	}
	if _, ok := GetSchema(template); !ok {
		return nil, fmt.Errorf("unknown template %q: valid templates are %v", template, TemplateNames())
	}
	img, err := Render(data, template, media, opts)
	if err != nil {
		return nil, err
	}
	return []image.Image{img}, nil
}

// TemplateNames returns the names of all templates RenderPages accepts: the
// built-in schemas (see SchemaNames) plus PairTemplate, sorted.
func TemplateNames() []string {
	names := append(SchemaNames(), PairTemplate)
	slices.Sort(names)
	return names
}

// RenderPair renders the paired template for one item.
//
// The canvas is media.WidthPx x media.HeightPx; die-cut media is expected
// (HeightPx == 0 falls back to the 62x29 mm aspect ratio). media.DPI sets the
// physical size of the fonts; 0 means 203 for canvases up to 384 px wide and
// 300 otherwise. opts.PrintDescription enables the description (off by
// default), opts.PrintDate puts the date into the bars next to the counter.
func RenderPair(data LabelData, media MediaInfo, opts RenderOpts) (PairResult, error) {
	pages, g, warns, err := planPair(data, media, opts)
	if err != nil {
		return PairResult{}, err
	}
	res := PairResult{Warnings: warns}
	for _, pg := range pages {
		res.Pages = append(res.Pages, drawPairPage(g, pg))
	}
	return res, nil
}

// planPair lays out both pages without drawing anything.
func planPair(data LabelData, media MediaInfo, opts RenderOpts) (pages [2]pairPage, g pairGeom, warns []PairWarning, err error) {
	g, err = newPairGeom(media)
	if err != nil {
		return pages, g, nil, err
	}
	for _, name := range []string{pairFontRegular, pairFontBold} {
		if _, err = LoadFace(name, float64(g.absFloor)); err != nil {
			return pages, g, nil, fmt.Errorf("pair template: %w", err)
		}
	}
	g.m = newPairMeasurer()
	p := &pairPlanner{g: g, m: g.m}

	name := pairNormalize(data.Name)
	desc := ""
	if opts.PrintDescription {
		desc = pairNormalize(data.Description)
	}
	blocks1, shown := p.planIdentity(name, desc, desc != "")
	ctxDesc := ""
	if !shown {
		ctxDesc = desc
	}
	pages[0] = pairPage{blocks: blocks1}
	pages[1] = p.planContext(data, ctxDesc)

	levels := locationLevels(data)
	hint1 := ""
	if len(levels) > 0 {
		hint1 = levels[len(levels)-1]
	}
	date := ""
	if opts.PrintDate {
		date = nowFunc().Format("02.01.2006")
	}
	pages[0].bar = pairBar{rect: g.bar, hint: hint1, counter: "1/2", date: date, icon: data.Icon}
	pages[1].bar = pairBar{rect: g.bar, hint: name, counter: "2/2", date: date, icon: data.Icon}
	return pages, g, p.warns, nil
}

// fillRoundRect fills r with c; the corner test is exact integer arithmetic on
// doubled pixel-centre coordinates, so the shape is identical on every CPU.
func fillRoundRect(img *image.RGBA, r image.Rectangle, radius int, c color.Color) {
	radius = max(0, min(radius, min(r.Dx(), r.Dy())/2))
	r2 := 4 * radius * radius
	col := color.RGBAModel.Convert(c).(color.RGBA)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dx := max(0, 2*(r.Min.X+radius)-(2*x+1), (2*x+1)-2*(r.Max.X-radius))
			dy := max(0, 2*(r.Min.Y+radius)-(2*y+1), (2*y+1)-2*(r.Max.Y-radius))
			if dx*dx+dy*dy <= r2 {
				img.SetRGBA(x, y, col)
			}
		}
	}
}

func drawPairText(img *image.RGBA, bold bool, px, x, baseline int, s string, c color.Color) {
	drawTextFace(img, x, baseline, s, c, pairFace(bold, px))
}

func drawPairPage(g pairGeom, pg pairPage) *image.RGBA {
	img := newCanvas(g.w, g.h)
	m := g.m
	if m == nil {
		m = newPairMeasurer()
	}
	drawPairBar(img, g, m, pg.bar)
	for _, b := range pg.blocks {
		sp := m.space(b.px)
		for i, line := range b.lines {
			x := b.rect.Min.X
			base := b.rect.Min.Y + b.base + i*b.step
			for _, r := range line {
				drawPairText(img, r.bold, b.px, x, base, r.text, color.Black)
				x += m.width(r.bold, b.px, r.text) + sp
			}
		}
	}
	for _, pl := range pg.pills {
		r := pl.rect
		fillRoundRect(img, r, r.Dy()/2, color.Black)
		in := r.Inset(g.tagOutline)
		fillRoundRect(img, in, max(0, r.Dy()/2-g.tagOutline), color.White)
		drawPairText(img, pl.bold, pl.px, pl.textX, pl.baseline, pl.text, color.Black)
	}
	return img
}

// drawPairBar draws the black header bar with the optional icon, the hint on
// the left and date + counter on the right. All bar text is bold: thin white
// strokes on black do not survive the 1-bit threshold.
func drawPairBar(img *image.RGBA, g pairGeom, m *pairMeasurer, b pairBar) {
	fillRoundRect(img, b.rect, g.barR, color.Black)
	px := g.fpx(pairBarFont)
	baseline := b.rect.Min.Y + (b.rect.Dy()+pairCapHeight(px, true))/2

	right := b.rect.Max.X - g.s(16)
	cw := m.width(true, px, b.counter)
	drawPairText(img, true, px, right-cw, baseline, b.counter, color.White)
	limit := right - cw
	if b.date != "" {
		dw := m.width(true, px, b.date)
		dx := right - cw - g.s(18) - dw
		drawPairText(img, true, px, dx, baseline, b.date, color.White)
		limit = dx
	}

	tx := g.tx
	if b.icon != "" {
		size := g.s(28)
		if ic, err := RasterizeIcon(b.icon, size); err == nil && ic != nil {
			ix, iy := g.tx-g.s(6), b.rect.Min.Y+(b.rect.Dy()-size)/2
			ir := image.Rect(ix, iy, ix+size, iy+size)
			draw.DrawMask(img, ir, image.White, image.Point{}, ic, image.Point{}, draw.Over)
			tx = g.tx + g.s(30)
		}
	}
	if hint := m.ellipsize(true, px, b.hint, limit-g.s(24)-tx); hint != "" {
		drawPairText(img, true, px, tx, baseline, hint, color.White)
	}
}
