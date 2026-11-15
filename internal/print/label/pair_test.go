package label

import (
	"bufio"
	"fmt"
	"image"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"golang.org/x/image/font"
)

// All fixtures here are synthetic: words are generated from syllables.

var pairMedias = []struct {
	name  string
	media MediaInfo
}{
	{"62x29@300", MediaInfo{WidthPx: 696, HeightPx: 271, DPI: 300}},
	{"b1@203", MediaInfo{WidthPx: 384, HeightPx: 160, DPI: 203}},
}

var (
	synOnsets = []string{"b", "c", "cz", "d", "dz", "f", "g", "h", "j", "k", "l", "ł", "m", "n", "ń", "p", "r", "s", "sz", "ś", "t", "w", "z", "ż", "ź", "ć", "gr", "st", "prz"}
	synVowels = []string{"a", "e", "i", "o", "u", "y", "ą", "ę", "ó", "ia", "ie"}
	synCodas  = []string{"", "", "", "n", "k", "ł", "j", "m", "ch"}
	synSingle = []string{"w", "z", "i", "o", "u", "a", "W", "Z"}
)

func synWord(r *rand.Rand) string {
	var sb strings.Builder
	for n := 1 + r.Intn(4); n > 0; n-- {
		sb.WriteString(synOnsets[r.Intn(len(synOnsets))])
		sb.WriteString(synVowels[r.Intn(len(synVowels))])
		if n == 1 {
			sb.WriteString(synCodas[r.Intn(len(synCodas))])
		}
	}
	return sb.String()
}

// synText builds a Polish-looking text of minLen..maxLen characters (after
// whitespace normalisation), with lowercase starts and occasional double spaces.
func synText(r *rand.Rand, minLen, maxLen int) string {
	target := minLen + r.Intn(maxLen-minLen+1)
	var words []string
	n := 0
	for tries := 0; tries < 60 && n < target; tries++ {
		w := synWord(r)
		if r.Intn(8) == 0 {
			w = synSingle[r.Intn(len(synSingle))]
		}
		add := utf8.RuneCountInString(w)
		if len(words) > 0 {
			add++
		}
		if n+add > maxLen {
			continue
		}
		words = append(words, w)
		n += add
	}
	if len(words) == 0 {
		words = []string{strings.Repeat("a", minLen)}
	}
	var sb strings.Builder
	for i, w := range words {
		if i > 0 {
			sb.WriteString(" ")
			if r.Intn(10) == 0 {
				sb.WriteString(" ")
			}
		}
		sb.WriteString(w)
	}
	return sb.String()
}

// synData generates one item within the documented ranges.
func synData(r *rand.Rand) (LabelData, RenderOpts) {
	d := LabelData{Name: synText(r, 10, 71)}
	if r.Intn(3) != 0 {
		d.Description = synText(r, 1, 70)
	}
	// location: 1-5 levels, up to ~125 characters including separators
	nl := 1 + r.Intn(5)
	budget := 125 - 3*(nl-1)
	var levels []string
	for i := 0; i < nl; i++ {
		maxL := min(35, budget-(nl-1-i)*3)
		if maxL < 3 {
			break
		}
		l := synText(r, 3, maxL)
		budget -= utf8.RuneCountInString(l)
		levels = append(levels, l)
	}
	d.Location = strings.Join(levels, " → ")
	for i := r.Intn(4); i > 0; i-- {
		d.Tags = append(d.Tags, LabelTag{Name: synText(r, 3, 25)})
	}
	if r.Intn(2) == 0 {
		d.Icon = "lightning"
	}
	if r.Intn(3) == 0 {
		for i := 1 + r.Intn(12); i > 0; i-- {
			d.Children = append(d.Children, LabelChild{Name: synText(r, 3, 18), IsContainer: r.Intn(3) == 0})
		}
	}
	return d, RenderOpts{PrintDate: r.Intn(2) == 0, PrintDescription: r.Intn(4) != 0}
}

func hasWarn(ws []PairWarning, w PairWarning) bool {
	for _, x := range ws {
		if x == w {
			return true
		}
	}
	return false
}

func linesText(b pairBlock) string {
	var parts []string
	for _, l := range b.lines {
		var ts []string
		for _, r := range l {
			ts = append(ts, r.text)
		}
		parts = append(parts, strings.Join(ts, " "))
	}
	return strings.Join(parts, " ")
}

func findBlock(pg pairPage, kind string) (pairBlock, bool) {
	for _, b := range pg.blocks {
		if b.kind == kind {
			return b, true
		}
	}
	return pairBlock{}, false
}

// blockInk returns the bounding box of the glyph ink of a block.
func blockInk(b pairBlock, m *pairMeasurer) image.Rectangle {
	var out image.Rectangle
	sp := m.space(b.px)
	for i, line := range b.lines {
		x := b.rect.Min.X
		base := b.rect.Min.Y + b.base + i*b.step
		for _, r := range line {
			bb, _ := font.BoundString(pairFace(r.bold, b.px), r.text)
			out = out.Union(image.Rect(x+bb.Min.X.Floor(), base+bb.Min.Y.Floor(), x+bb.Max.X.Ceil(), base+bb.Max.Y.Ceil()))
			x += m.width(r.bold, b.px, r.text) + sp
		}
	}
	return out
}

type pairBox struct {
	what string
	r    image.Rectangle
}

func pageBoxes(pg pairPage, m *pairMeasurer) []pairBox {
	out := []pairBox{{"bar", pg.bar.rect}}
	for _, b := range pg.blocks {
		out = append(out, pairBox{b.kind, blockInk(b, m)})
	}
	for _, p := range pg.pills {
		out = append(out, pairBox{"pill " + p.text, p.rect})
	}
	return out
}

func printableArea(g pairGeom) image.Rectangle {
	return image.Rect(g.s(24), g.s(18), g.w-g.s(24), g.h-g.s(20))
}

// checkPairInvariants verifies everything that must hold for any content.
//
//nolint:gocyclo // a flat list of independent assertions
func checkPairInvariants(t *testing.T, d LabelData, opts RenderOpts, media MediaInfo) (pages [2]pairPage, g pairGeom, warns []PairWarning) {
	t.Helper()
	pages, g, warns, err := planPair(d, media, opts)
	if err != nil {
		t.Fatal(err)
	}
	m := newPairMeasurer()
	area := printableArea(g)

	// geometry: inside the printable area, no overlaps, same bar on both labels
	if pages[0].bar.rect != pages[1].bar.rect {
		t.Errorf("bar geometry differs: %v vs %v", pages[0].bar.rect, pages[1].bar.rect)
	}
	for pi, pg := range pages {
		boxes := pageBoxes(pg, m)
		for i, a := range boxes {
			if !a.r.In(area) {
				t.Errorf("page %d: %s ink %v outside printable %v", pi+1, a.what, a.r, area)
			}
			for _, b := range boxes[i+1:] {
				if a.r.Overlaps(b.r) {
					t.Errorf("page %d: %s %v overlaps %s %v", pi+1, a.what, a.r, b.what, b.r)
				}
			}
		}
		// legibility floors
		for _, b := range pg.blocks {
			if b.px < g.absFloor {
				t.Errorf("page %d: %s at %dpx below absolute floor %d", pi+1, b.kind, b.px, g.absFloor)
			}
			for _, l := range b.lines {
				for _, r := range l {
					if !r.bold && b.px < g.boldBelow {
						t.Errorf("page %d: regular text %q at %dpx, want bold below %d", pi+1, r.text, b.px, g.boldBelow)
					}
				}
			}
		}
		for _, p := range pg.pills {
			if p.px < g.absFloor || (!p.bold && p.px < g.boldBelow) {
				t.Errorf("pill %q at %dpx bold=%v", p.text, p.px, p.bold)
			}
		}
	}
	return pages, g, warns
}

//nolint:gocyclo // a flat list of independent assertions
func checkPairContent(t *testing.T, d LabelData, opts RenderOpts, pages [2]pairPage, g pairGeom, warns []PairWarning) {
	t.Helper()
	// name: complete and in order, on label 1/2, unless a word had to be split
	name, ok := findBlock(pages[0], "name")
	if !ok {
		t.Fatal("no name block")
	}
	if !hasWarn(warns, PairWarnNameSplit) {
		if got, want := linesText(name), pairNormalize(d.Name); got != want {
			t.Errorf("name = %q, want %q", got, want)
		}
	}
	minName := g.fpx(pairNameMin)
	if _, withDesc := findBlock(pages[0], "desc"); withDesc {
		minName = g.fpx(pairNameMinDesc)
	}
	if name.px < minName && !hasWarn(warns, PairWarnOverflow) {
		t.Errorf("name at %dpx, floor %d", name.px, minName)
	}

	// description: exactly one label (when enabled), complete unless last resort
	desc := pairNormalize(d.Description)
	d1, on1 := findBlock(pages[0], "desc")
	d2, on2 := findBlock(pages[1], "desc")
	switch {
	case !opts.PrintDescription || desc == "":
		if on1 || on2 {
			t.Error("description printed although disabled or empty")
		}
	case on1 && on2:
		t.Error("description on both labels")
	case on1:
		if linesText(d1) != desc {
			t.Errorf("desc on 1/2 = %q, want %q", linesText(d1), desc)
		}
	case on2:
		if got := linesText(d2); got != desc && !hasWarn(warns, PairWarnDescriptionShortened) {
			t.Errorf("desc on 2/2 = %q, want %q", got, desc)
		}
	default:
		if !hasWarn(warns, PairWarnDescriptionShortened) {
			t.Error("description missing without a warning")
		}
	}

	// location breadcrumb: levels in order, last one bold
	levels := locationLevels(d.Location)
	if loc, ok := findBlock(pages[1], "location"); ok {
		var got []string
		for _, l := range loc.lines {
			for _, r := range l {
				got = append(got, r.text)
			}
		}
		joined := strings.TrimSpace(strings.ReplaceAll(strings.Join(got, " "), " ›", ""))
		if !hasWarn(warns, PairWarnLocationShortened) {
			if want := strings.Join(levels, " "); joined != want {
				t.Errorf("location = %q, want %q", joined, want)
			}
		}
		last := loc.lines[len(loc.lines)-1]
		if !last[len(last)-1].bold {
			t.Error("last breadcrumb level is not bold")
		}
	} else if len(levels) > 0 {
		t.Error("location missing")
	}

	// tags: all pills, in order
	if !hasWarn(warns, PairWarnTagsDropped) {
		var want, got []string
		for _, tg := range d.Tags {
			want = append(want, pairNormalize(tg.Name))
		}
		for _, p := range pages[1].pills {
			got = append(got, p.text)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("tags = %q, want %q", got, want)
		}
	}

	// containers: counts line, and the child list is complete or absent
	if want := contextCounts(d.Children); want != "" {
		cb, ok := findBlock(pages[1], "counts")
		if !ok || linesText(cb) != want {
			t.Errorf("counts = %q (found=%v), want %q", linesText(cb), ok, want)
		}
		if lb, ok := findBlock(pages[1], "children"); ok {
			var names []string
			for _, c := range d.Children {
				names = append(names, pairNormalize(c.Name))
			}
			got := strings.ReplaceAll(linesText(lb), " ·", "")
			if got != strings.Join(names, " ") {
				t.Errorf("children = %q, want %q", got, strings.Join(names, " "))
			}
			for _, l := range lb.lines {
				for _, r := range l {
					if want := childBold(d.Children, r.text) || lb.px < g.boldBelow; want != r.bold {
						t.Errorf("child run %q bold=%v", r.text, r.bold)
					}
				}
			}
		}
	}
}

// childBold finds whether the run belongs to a container (for whole-name runs).
func childBold(children []LabelChild, run string) bool {
	run = strings.TrimSuffix(run, " ·")
	for _, c := range children {
		if pairNormalize(c.Name) == run {
			return c.IsContainer
		}
	}
	// part of a name broken at words: take the first child containing the word
	for _, c := range children {
		for _, w := range strings.Fields(c.Name) {
			if w == run {
				return c.IsContainer
			}
		}
	}
	return false
}

func TestPairProperties(t *testing.T) {
	const cases = 120
	warned := map[string]int{}
	for _, mc := range pairMedias {
		for seed := 0; seed < cases; seed++ {
			r := rand.New(rand.NewSource(int64(seed)*7919 + 13)) //nolint:gosec // deterministic test data
			d, opts := synData(r)
			name := fmt.Sprintf("%s/seed%d", mc.name, seed)
			t.Run(name, func(t *testing.T) {
				pages, g, warns := checkPairInvariants(t, d, opts, mc.media)
				checkPairContent(t, d, opts, pages, g, warns)
				for _, w := range warns {
					warned[mc.name+" "+string(w)]++
				}
				// planning is deterministic
				pages2, _, warns2, _ := planPair(d, mc.media, opts)
				if !reflect.DeepEqual(pages, pages2) || !reflect.DeepEqual(warns, warns2) {
					t.Error("plan differs between runs")
				}
			})
			if t.Failed() {
				t.Logf("failing input: %+v opts=%+v", d, opts)
				return
			}
		}
	}
	t.Logf("last-resort warnings over %d generated items per canvas: %v", cases, warned)
	if n := warned["62x29@300 "+string(PairWarnOverflow)] + warned["b1@203 "+string(PairWarnOverflow)]; n > 0 {
		t.Errorf("%d generated items overflowed", n)
	}
}

// TestPairInkInsideElements renders a subset and checks every drawn pixel
// belongs to a laid-out element, and that two renders are byte-identical.
//
//nolint:gocyclo // a flat list of independent assertions
func TestPairInkInsideElements(t *testing.T) {
	for _, mc := range pairMedias {
		for seed := 1000; seed < 1030; seed++ {
			r := rand.New(rand.NewSource(int64(seed))) //nolint:gosec // deterministic test data
			d, opts := synData(r)
			res, err := RenderPair(d, mc.media, opts)
			if err != nil {
				t.Fatal(err)
			}
			res2, _ := RenderPair(d, mc.media, opts)
			pages, g, _, _ := planPair(d, mc.media, opts)
			m := newPairMeasurer()
			area := printableArea(g)
			if len(res.Pages) != 2 {
				t.Fatalf("pages = %d", len(res.Pages))
			}
			for pi, img := range res.Pages {
				if !reflect.DeepEqual(img.(*image.RGBA).Pix, res2.Pages[pi].(*image.RGBA).Pix) {
					t.Errorf("%s seed %d page %d: renders differ", mc.name, seed, pi+1)
				}
				boxes := pageBoxes(pages[pi], m)
				b := img.Bounds()
				for y := b.Min.Y; y < b.Max.Y; y++ {
					for x := b.Min.X; x < b.Max.X; x++ {
						c := img.At(x, y)
						rr, gg, bb, _ := c.RGBA()
						if rr == 0xffff && gg == 0xffff && bb == 0xffff {
							continue
						}
						pt := image.Pt(x, y)
						if !pt.In(area) {
							t.Fatalf("%s seed %d page %d: ink at %v outside %v", mc.name, seed, pi+1, pt, area)
						}
						inside := false
						for _, bx := range boxes {
							if pt.In(bx.r.Inset(-1)) {
								inside = true
								break
							}
						}
						if !inside {
							t.Fatalf("%s seed %d page %d: stray ink at %v", mc.name, seed, pi+1, pt)
						}
					}
				}
			}
		}
	}
}

func TestRenderPages(t *testing.T) {
	media := MediaInfo{WidthPx: 384, HeightPx: 160, DPI: 203}
	d := LabelData{Name: "Kabel USB", Location: "Pokój → Półka", QRContent: "x", BarcodeID: "y"}

	for _, tpl := range SchemaNames() {
		pages, err := RenderPages(d, tpl, media, RenderOpts{})
		if err != nil {
			t.Fatalf("%s: %v", tpl, err)
		}
		if len(pages) != 1 {
			t.Fatalf("%s: %d pages, want 1", tpl, len(pages))
		}
		want, _ := Render(d, tpl, media, RenderOpts{})
		if imageDigest(pages[0]) != imageDigest(want) {
			t.Errorf("%s: RenderPages differs from Render", tpl)
		}
	}
	pages, err := RenderPages(d, PairTemplate, media, RenderOpts{})
	if err != nil || len(pages) != 2 {
		t.Fatalf("pair: %d pages, err %v", len(pages), err)
	}
	for i, p := range pages {
		if p.Bounds().Dx() != 384 || p.Bounds().Dy() != 160 {
			t.Errorf("page %d size %v", i+1, p.Bounds())
		}
	}
	if _, err := RenderPages(d, "nope", media, RenderOpts{}); err == nil {
		t.Error("unknown template must fail")
	}
	if _, err := RenderPages(d, PairTemplate, MediaInfo{}, RenderOpts{}); err == nil {
		t.Error("zero-width canvas must fail")
	}
	if names := TemplateNames(); !hasString(names, PairTemplate) || !hasString(names, "standard") {
		t.Errorf("TemplateNames = %v", names)
	}
	if hasString(SchemaNames(), PairTemplate) {
		t.Error("SchemaNames must not list the code-defined template")
	}
}

func hasString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func TestPairDescriptionOptionAndDate(t *testing.T) {
	media := pairMedias[0].media
	d := LabelData{Name: "Myszka", Description: "bezprzewodowa", Location: "Pracownia → Szuflada"}
	pages, _, _, _ := planPair(d, media, RenderOpts{})
	for i, pg := range pages {
		if _, ok := findBlock(pg, "desc"); ok {
			t.Errorf("page %d has a description although the option is off", i+1)
		}
		if pg.bar.date != "" {
			t.Errorf("page %d has a date although PrintDate is off", i+1)
		}
	}
	pages, _, _, _ = planPair(d, media, RenderOpts{PrintDescription: true, PrintDate: true})
	if _, ok := findBlock(pages[0], "desc"); !ok {
		t.Error("short description should sit on label 1/2")
	}
	if pages[0].bar.date == "" || pages[1].bar.date != pages[0].bar.date {
		t.Errorf("dates: %q %q", pages[0].bar.date, pages[1].bar.date)
	}
	if pages[0].bar.hint != "Szuflada" || pages[1].bar.hint != "Myszka" || pages[0].bar.counter != "1/2" || pages[1].bar.counter != "2/2" {
		t.Errorf("bars: %+v %+v", pages[0].bar, pages[1].bar)
	}
	// no location: empty hint on 1/2, no breadcrumb on 2/2
	d.Location = ""
	pages, _, _, _ = planPair(d, media, RenderOpts{})
	if pages[0].bar.hint != "" {
		t.Errorf("hint = %q", pages[0].bar.hint)
	}
	if _, ok := findBlock(pages[1], "location"); ok {
		t.Error("breadcrumb without location")
	}
}

// TestPairDescriptionMovesToContext: a long description does not fit next to a
// name of at least 44px, so it moves to label 2/2 and the name gets the room.
func TestPairDescriptionMovesToContext(t *testing.T) {
	media := pairMedias[0].media
	d := LabelData{
		Name:        "Zażółć gęślą jaźń dokumentacja techniczna wzorcowego stanowiska pomiarowego i nie tylko",
		Description: "łączniki, uchwyty montażowe, instrukcje serwisowe oraz zapasowe części, etc",
		Location:    "Pracownia → Regał",
	}
	pages, g, warns, _ := planPair(d, media, RenderOpts{PrintDescription: true})
	if _, ok := findBlock(pages[0], "desc"); ok {
		t.Error("description should have moved to 2/2")
	}
	if b, ok := findBlock(pages[1], "desc"); !ok || linesText(b) != pairNormalize(d.Description) {
		t.Errorf("description on 2/2: %v %q", ok, linesText(b))
	}
	if len(warns) != 0 {
		t.Errorf("warnings: %v", warns)
	}
	if name, _ := findBlock(pages[0], "name"); name.px < g.fpx(pairNameMin) {
		t.Errorf("name %dpx", name.px)
	}
}

func repeatWords(n int, w string) string {
	return strings.TrimSpace(strings.Repeat(w+" ", n))
}

func TestPairWorstCases(t *testing.T) {
	maxName := "Zażółć gęślą jaźń dokumentacja techniczna wzorcowego stanowiska pomiarowego i nie tylko" // 71+ chars
	maxName = string([]rune(maxName)[:71])
	cases := []struct {
		name string
		d    LabelData
		opts RenderOpts
		// warnings allowed (the documented last resorts)
		allow []PairWarning
	}{
		{
			name: "name alone at max length",
			d:    LabelData{Name: maxName, Location: "A → B"},
		},
		{
			name: "max name, description and location, two tags",
			d: LabelData{
				Name:        maxName,
				Description: "łączniki, uchwyty montażowe, instrukcje serwisowe oraz zapasowe części, etc",
				Location:    "Pokój wzorcowy → Szafka segregatorowa → Segregator archiwalny → Specjalizacje o małym priorytecie → Dokumenty testowe",
				Tags:        []LabelTag{{Name: "Urząd Testowy"}, {Name: "Pokazać kiedyś komuś"}},
			},
			opts:  RenderOpts{PrintDescription: true, PrintDate: true},
			allow: []PairWarning{PairWarnDescriptionShortened, PairWarnLocationShortened}, // only on the small B1 canvas
		},
		{
			name: "three long tags",
			d: LabelData{
				Name: "Pudełko", Location: "Pracownia → Regał",
				Tags: []LabelTag{{Name: "Elektronika zasilająca"}, {Name: "Narzędzia precyzyjne"}, {Name: "Zapasy na zimę i lato"}},
			},
		},
		{
			name: "container with many children",
			d: LabelData{
				Name: "Skrzynia", Location: "Garaż → Półka",
				Children: manyChildren(40),
			},
		},
		{
			name: "single very long word",
			d:    LabelData{Name: "Nieprzeciwdziałającemu", Location: "Pracownia"},
		},
		{
			name:  "word wider than the label",
			d:     LabelData{Name: strings.Repeat("Nieprzeciwdziałającemu", 3), Location: "Pracownia"},
			allow: []PairWarning{PairWarnNameSplit},
		},
		{
			name:  "tag wider than the label",
			d:     LabelData{Name: "Pudełko", Location: "Pracownia", Tags: []LabelTag{{Name: repeatWords(12, "Nieprzeciwdziałającemu")}}},
			allow: []PairWarning{PairWarnTagsDropped},
		},
	}
	for _, mc := range pairMedias {
		for _, tc := range cases {
			t.Run(mc.name+"/"+tc.name, func(t *testing.T) {
				opts := tc.opts
				pages, g, warns := checkPairInvariants(t, tc.d, opts, mc.media)
				for _, w := range warns {
					ok := false
					for _, a := range tc.allow {
						ok = ok || a == w
					}
					if !ok || (mc.name == "62x29@300" && tc.name == "max name, description and location, two tags") {
						t.Errorf("unexpected warning %q", w)
					}
				}
				if len(tc.allow) == 0 || tc.name == "max name, description and location, two tags" {
					checkPairContent(t, tc.d, opts, pages, g, warns)
				}
				name, _ := findBlock(pages[0], "name")
				loc, _ := findBlock(pages[1], "location")
				t.Logf("name %dpx x%d lines, location %dpx x%d lines, warnings %v", name.px, len(name.lines), loc.px, len(loc.lines), warns)
			})
		}
	}
}

func manyChildren(n int) []LabelChild {
	r := rand.New(rand.NewSource(5)) //nolint:gosec // deterministic test data
	out := make([]LabelChild, n)
	for i := range out {
		out[i] = LabelChild{Name: synText(r, 4, 14), IsContainer: i%4 == 0}
	}
	return out
}

func TestPairChildrenListOnlyWhenComplete(t *testing.T) {
	media := pairMedias[0].media
	few := LabelData{Name: "Pudełko", Location: "Pracownia", Children: manyChildren(4)}
	pages, _, _, _ := planPair(few, media, RenderOpts{})
	if _, ok := findBlock(pages[1], "children"); !ok {
		t.Error("a short child list should be printed")
	}
	many := LabelData{Name: "Pudełko", Location: "Pracownia", Children: manyChildren(60)}
	pages, _, _, _ = planPair(many, media, RenderOpts{})
	if _, ok := findBlock(pages[1], "children"); ok {
		t.Error("a list that does not fit must not be printed partially")
	}
	if cb, ok := findBlock(pages[1], "counts"); !ok || !strings.HasPrefix(linesText(cb), "Zawiera: ") {
		t.Errorf("counts missing: %q", linesText(cb))
	}
}

//nolint:gocyclo // a flat list of independent assertions
func TestPairGeometry(t *testing.T) {
	g, err := newPairGeom(MediaInfo{WidthPx: 696, HeightPx: 271, DPI: 300})
	if err != nil {
		t.Fatal(err)
	}
	if g.bodyW != 616 || g.bodyH != 181 || g.bar != image.Rect(24, 18, 672, 58) || g.tx != 40 || g.barR != 10 {
		t.Errorf("62x29 geometry: %+v", g)
	}
	b1, err := newPairGeom(MediaInfo{WidthPx: 384, HeightPx: 160, DPI: 203})
	if err != nil {
		t.Fatal(err)
	}
	// fonts follow the dpi: the floors stay 1.6 mm
	if b1.absFloor != 13 || b1.fpx(pairBarFont) != 16 || b1.fpx(pairNameMinDesc) != 30 || b1.fpx(pairNameMin) != 23 || b1.fpx(pairLocMin) != 14 {
		t.Errorf("B1 font sizes: floor %d bar %d", b1.absFloor, b1.fpx(pairBarFont))
	}
	if b1.bodyW < 330 || b1.bodyW > 345 || b1.bodyH < 100 {
		t.Errorf("B1 body %dx%d", b1.bodyW, b1.bodyH)
	}
	if _, err := newPairGeom(MediaInfo{WidthPx: 40, HeightPx: 20}); err == nil {
		t.Error("tiny canvas must be rejected")
	}
	// continuous media falls back to the 62x29 aspect ratio
	c, err := newPairGeom(MediaInfo{WidthPx: 696})
	if err != nil || c.h != 271 || c.dpi != 300 {
		t.Errorf("continuous: %v %+v", err, c)
	}
}

func TestPairRoundRectIsSymmetric(t *testing.T) {
	img := newCanvas(60, 40)
	fillRoundRect(img, image.Rect(5, 5, 55, 35), 10, image.Black)
	flipped := image.NewRGBA(img.Bounds())
	for y := 0; y < 40; y++ {
		for x := 0; x < 60; x++ {
			flipped.Set(59-x, y, img.At(x, y))
		}
	}
	if !reflect.DeepEqual(img.Pix, flipped.Pix) {
		t.Error("rounded rectangle is not mirror symmetric")
	}
	if r, _, _, _ := img.At(5, 5).RGBA(); r != 0xffff {
		t.Error("corner pixel must be cut off")
	}
	if r, _, _, _ := img.At(30, 5).RGBA(); r != 0 {
		t.Error("edge pixel must be filled")
	}
}

// --- golden digests for the new template (separate files, per GOARCH) ---

func pairGoldenFile() string { return "testdata/golden_pair_digests_" + runtime.GOARCH + ".txt" }

func goldenPairCases() map[string]struct {
	d    LabelData
	opts RenderOpts
} {
	type c = struct {
		d    LabelData
		opts RenderOpts
	}
	return map[string]c{
		"worst": {LabelData{
			Name:        "Zażółć gęślą jaźń dokumentacja techniczna wzorcowego stanowiska pomiarowego i nie tylko",
			Description: "łączniki, uchwyty montażowe, instrukcje serwisowe oraz zapasowe części, etc",
			Location:    "Pokój wzorcowy → Szafka segregatorowa → Segregator archiwalny → Specjalizacje o małym priorytecie → Dokumenty testowe",
			Tags:        []LabelTag{{Name: "Urząd Testowy"}, {Name: "Pokazać kiedyś komuś"}},
		}, RenderOpts{PrintDescription: true, PrintDate: true}},
		"typical": {LabelData{
			Name: "Myszka USB", Description: "bezprzewodowa, odbiornik w środku",
			Location: "Pracownia → Szuflada z kablami", Tags: []LabelTag{{Name: "Elektronika"}},
		}, RenderOpts{PrintDescription: true}},
		"icon": {LabelData{
			Name: "Zasilacz impulsowy 12V", Icon: "lightning", Description: "do taśm LED, 5 A, wtyczka DC 5,5 mm",
			Location: "Pracownia → Regał 2 → Szuflada", Tags: []LabelTag{{Name: "Elektronika"}, {Name: "12V"}},
		}, RenderOpts{PrintDescription: true, PrintDate: true}},
		"container": {LabelData{
			Name: "Pudełko z drobiazgami", Icon: "package", Location: "Pracownia → Regał 2",
			Tags: []LabelTag{{Name: "Elektronika"}},
			Children: []LabelChild{
				{Name: "Rezystory", IsContainer: true}, {Name: "Kondensatory", IsContainer: true},
				{Name: "Dioda LED"}, {Name: "Tranzystor"}, {Name: "Przekaźnik 5V"},
			},
		}, RenderOpts{}},
	}
}

func renderGoldenPair() (map[string]image.Image, error) {
	fixed := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	old := nowFunc
	nowFunc = func() time.Time { return fixed }
	defer func() { nowFunc = old }()
	out := map[string]image.Image{}
	for _, mc := range pairMedias {
		for name, c := range goldenPairCases() {
			pages, err := RenderPages(c.d, PairTemplate, mc.media, c.opts)
			if err != nil {
				return nil, err
			}
			for i, p := range pages {
				out[fmt.Sprintf("pair/%s/%s/page%d", name, mc.name, i+1)] = p
			}
		}
	}
	return out, nil
}

// TestGoldenPair guards the paired template pixel for pixel. Re-record with
// `go test ./internal/print/label -run GoldenPair -update` (per GOARCH).
func TestGoldenPair(t *testing.T) {
	imgs, err := renderGoldenPair()
	if err != nil {
		t.Fatal(err)
	}
	keys := sortedKeys(imgs)
	if *updateGolden {
		if err := writeDigests(pairGoldenFile(), keys, imgs); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := readDigests(pairGoldenFile())
	if err != nil {
		t.Fatalf("%v (run with -update to create)", err)
	}
	if len(want) != len(keys) {
		t.Errorf("golden entries = %d, rendered = %d", len(want), len(keys))
	}
	for _, k := range keys {
		if got := imageDigest(imgs[k]); got != want[k] {
			t.Errorf("%s: digest changed\n got  %s\n want %s", k, got, want[k])
		}
	}
}

func sortedKeys(m map[string]image.Image) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func writeDigests(path string, keys []string, imgs map[string]image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "%s %s\n", k, imageDigest(imgs[k]))
	}
	return os.WriteFile(path, []byte(sb.String()), 0o600)
}

func readDigests(path string) (map[string]string, error) {
	f, err := os.Open(path) //nolint:gosec // fixed testdata path
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only
	m := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if parts := strings.Fields(sc.Text()); len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m, sc.Err()
}
