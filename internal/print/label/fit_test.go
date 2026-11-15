package label

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

// inkBounds returns the bounding box of all dark (printed) pixels.
func inkBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	r := image.Rectangle{Min: b.Max, Max: b.Min}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			if c.Y >= 128 {
				continue
			}
			r.Min.X, r.Min.Y = min(r.Min.X, x), min(r.Min.Y, y)
			r.Max.X, r.Max.Y = max(r.Max.X, x+1), max(r.Max.Y, y+1)
		}
	}
	if r.Empty() {
		return image.Rectangle{}
	}
	return r
}

func fitSchema(elems ...Element) Schema {
	return Schema{Name: "fit-test", Padding: 8, Elements: elems}
}

var fitMedias = map[string]MediaInfo{
	"696x271@300": {WidthPx: 696, HeightPx: 271, DPI: 300},
	"384x160@203": {WidthPx: 384, HeightPx: 160, DPI: 203},
}

func TestFitTextStaysInPrintableArea(t *testing.T) {
	names := map[string]string{
		"short":    "Kabel",
		"medium":   "Zasilacz 12V 2A",
		"polish":   "Łączniki żółte, ćwiczeniowe",
		"long":     "Zestaw śrubek do elektroniki precyzyjnej",
		"verylong": strings.Repeat("Bardzo długa nazwa przedmiotu ", 6),
		"oneword":  "Supercalifragilisticexpialidocious",
	}
	for mn, media := range fitMedias {
		for nn, name := range names {
			t.Run(mn+"/"+nn, func(t *testing.T) {
				s := fitSchema(Element{Slot: "title", Fit: "fill", Align: "left", Color: "#000000"})
				img, err := renderSchema(s, LabelData{Name: name}, media, RenderOpts{})
				if err != nil {
					t.Fatal(err)
				}
				if img.Bounds().Dx() != media.WidthPx || img.Bounds().Dy() != media.HeightPx {
					t.Fatalf("size = %v, want %dx%d", img.Bounds().Size(), media.WidthPx, media.HeightPx)
				}
				ink := inkBounds(img)
				area := image.Rect(s.Padding, s.Padding, media.WidthPx-s.Padding, media.HeightPx-s.Padding)
				if ink.Empty() || !ink.In(area) {
					t.Errorf("ink %v not inside printable area %v", ink, area)
				}
			})
		}
	}
}

func TestFitTextFillsHeightForShortNames(t *testing.T) {
	for mn, media := range fitMedias {
		for _, name := range []string{"Kabel", "Zasilacz", "Baterie AA"} {
			t.Run(mn+"/"+name, func(t *testing.T) {
				s := fitSchema(Element{Slot: "title", Fit: "fill", Color: "#000000"})
				img, err := renderSchema(s, LabelData{Name: name}, media, RenderOpts{})
				if err != nil {
					t.Fatal(err)
				}
				ink := inkBounds(img)
				// Either the height or the width is the limiting factor.
				hShare := float64(ink.Dy()) / float64(media.HeightPx)
				wShare := float64(ink.Dx()) / float64(media.WidthPx-2*s.Padding)
				if hShare < 0.45 && wShare < 0.85 {
					t.Errorf("text does not fill the label: height share %.2f, width share %.2f", hShare, wShare)
				}
			})
		}
	}
}

func TestFitTextLongNames(t *testing.T) {
	media := fitMedias["384x160@203"]
	run := func(t *testing.T, name string, el Element) resolvedText {
		t.Helper()
		el.Slot, el.Fit = "title", "fill"
		s := fitSchema(el)
		s.Elements[0].FontSize = 13
		textElems, _, _, err := resolveElements(s, LabelData{Name: name}, media.WidthPx, s.Padding, 0)
		if err != nil {
			t.Fatal(err)
		}
		out := fitElements(textElems, fitArea{widthPx: media.WidthPx, totalH: media.HeightPx, pad: s.Padding})
		return out[0]
	}
	sizeOf := func(rt resolvedText) int { return rt.face.Metrics().Height.Ceil() }

	short := run(t, "Kabel", Element{})
	if len(short.lines) != 1 {
		t.Fatalf("short name lines = %v", short.lines)
	}
	medium := run(t, "Zasilacz 12V", Element{})
	if len(medium.lines) != 1 || sizeOf(medium) >= sizeOf(short) {
		t.Errorf("medium name should be one smaller line, got %v (size %d vs %d)", medium.lines, sizeOf(medium), sizeOf(short))
	}

	wrapped := run(t, "Zestaw śrubek do elektroniki", Element{MinSize: 30})
	if len(wrapped.lines) != 2 {
		t.Errorf("expected wrap to 2 lines, got %v", wrapped.lines)
	}

	truncated := run(t, strings.Repeat("słowo ", 30), Element{MinSize: 30})
	if len(truncated.lines) > 2 || !strings.HasSuffix(truncated.lines[len(truncated.lines)-1], "...") {
		t.Errorf("expected truncation with ellipsis, got %q", truncated.lines)
	}
}

func TestFitIgnoredOnContinuousMedia(t *testing.T) {
	s := fitSchema(Element{Slot: "title", Fit: "fill", FontSize: 20, Color: "#000000"})
	img, err := renderSchema(s, LabelData{Name: "Kabel"}, MediaInfo{WidthPx: 384, DPI: 203}, RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dy() > 60 {
		t.Errorf("continuous label height = %d, fit must not inflate it", img.Bounds().Dy())
	}
}

func TestFitSharesSpaceWithOtherElements(t *testing.T) {
	media := fitMedias["696x271@300"]
	s := fitSchema(
		Element{Slot: "title", Fit: "fill", Color: "#000000"},
		Element{Slot: "location", FontSize: 26, FontFamily: "noto-sans", Color: "#000000"},
	)
	data := LabelData{Name: "Kabel", Location: "Pracownia"}
	img, err := renderSchema(s, data, media, RenderOpts{PrintDate: true})
	if err != nil {
		t.Fatal(err)
	}
	ink := inkBounds(img)
	if ink.Max.Y > media.HeightPx-s.Padding {
		t.Errorf("ink %v exceeds bottom padding", ink)
	}
}

func TestFitVAlignMiddle(t *testing.T) {
	media := fitMedias["696x271@300"]
	el := Element{Slot: "title", Fit: "fill", MaxSize: 60, Color: "#000000"}
	top := fitSchema(el)
	mid := fitSchema(el)
	mid.VAlign = "middle"
	a, err := renderSchema(top, LabelData{Name: "Kabel"}, media, RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := renderSchema(mid, LabelData{Name: "Kabel"}, media, RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if inkBounds(b).Min.Y <= inkBounds(a).Min.Y {
		t.Errorf("middle-aligned ink %v should start below top-aligned %v", inkBounds(b), inkBounds(a))
	}
}

func TestEllipsize(t *testing.T) {
	face, err := LoadFace("noto-sans-bold", 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := ellipsize("ab", face, 500); got != "ab" {
		t.Errorf("fitting text changed: %q", got)
	}
	got := ellipsize(strings.Repeat("W", 40), face, 120)
	if !strings.HasSuffix(got, "...") || len(got) >= 40 {
		t.Errorf("ellipsize = %q", got)
	}
}

func TestFitWithIconAndPolishCapitalsStaysInArea(t *testing.T) {
	media := fitMedias["384x160@203"]
	s := fitSchema(Element{Slot: "title", Fit: "fill", Color: "#000000"})
	img, err := renderSchema(s, LabelData{Name: "Żółć Łódź Świętość", Icon: "wrench"}, media, RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	ink := inkBounds(img)
	area := image.Rect(s.Padding, s.Padding, media.WidthPx-s.Padding, media.HeightPx-s.Padding)
	if !ink.In(area) {
		t.Errorf("ink %v not inside printable area %v", ink, area)
	}
}

func TestFitSharesHeightAfterDroppingLowPriority(t *testing.T) {
	media := fitMedias["384x160@203"]
	title := Element{Slot: "title", Fit: "fill", Color: "#000000"}
	tags := Element{Slot: "tags", FontSize: 11, Wrap: true, Color: "#000000"}
	children := Element{Slot: "children", FontSize: 11, Color: "#000000"}
	data := LabelData{
		Name:     "Kabel",
		Tags:     []LabelTag{{Name: "arduino"}, {Name: "narzędzia"}, {Name: "kable"}, {Name: "zasilanie"}},
		Children: []LabelChild{{Name: "Rezystory"}, {Name: "Kondensatory"}, {Name: "LED"}},
	}
	only, err := renderSchema(fitSchema(title), LabelData{Name: "Kabel"}, media, RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	img, err := renderSchema(fitSchema(title, tags, children), data, media, RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	// Tags and children that do not fit next to a reasonably sized title are
	// dropped, so the title keeps (about) the size it has without them.
	if got, want := inkBounds(img).Dy(), inkBounds(only).Dy(); got < want*8/10 {
		t.Errorf("title starved by low-priority elements: ink height %d vs %d", got, want)
	}
	if inkBounds(img).Max.Y > media.HeightPx-8 {
		t.Errorf("ink %v exceeds bottom padding", inkBounds(img))
	}
}

func TestFitHugeMinSizeIsClamped(t *testing.T) {
	media := fitMedias["384x160@203"]
	s := fitSchema(Element{Slot: "title", Fit: "fill", MinSize: 1 << 30, Color: "#000000"})
	if _, err := renderSchema(s, LabelData{Name: "Kabel"}, media, RenderOpts{}); err != nil {
		t.Fatal(err)
	}
}

func TestFitDefaultFontOnlyWhenFitActive(t *testing.T) {
	s := fitSchema(Element{Slot: "title", Fit: "fill", FontSize: 20, Color: "#000000"})
	if got := effectiveFontFamily(s.Elements[0], s); got != "spleen" {
		t.Errorf("effectiveFontFamily = %q, want spleen on continuous media", got)
	}
	rt, err := resolveSlotText(s.Elements[0], "Kabel", s, 384, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rt.family != fitDefaultFont {
		t.Errorf("fit family = %q, want %q", rt.family, fitDefaultFont)
	}
}

func TestParseSchemaRejectsInvalidFitAndValign(t *testing.T) {
	tests := []struct {
		name, json, want string
	}{
		{"bad fit", `{"name":"x","elements":[{"slot":"title","fit":"Fill"}]}`, `invalid fit "Fill"`},
		{"bad valign", `{"name":"x","valign":"center","elements":[]}`, `invalid valign "center"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSchema([]byte(tt.json))
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), `schema "x"`) {
				t.Errorf("err = %v, want it to name the schema and contain %q", err, tt.want)
			}
		})
	}
	for _, ok := range []string{`{"name":"x","valign":"middle","elements":[{"slot":"title","fit":"fill"}]}`,
		`{"name":"x","elements":[{"slot":"title","fit":"none"}]}`} {
		if _, err := parseSchema([]byte(ok)); err != nil {
			t.Errorf("valid schema rejected: %v", err)
		}
	}
}

func TestLargestSize(t *testing.T) {
	ok := func(n int) bool { return n <= 37 }
	if got := largestSize(10, 100, ok); got != 37 {
		t.Errorf("got %d, want 37", got)
	}
	if got := largestSize(40, 100, ok); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
	if got := largestSize(10, 20, ok); got != 20 {
		t.Errorf("got %d, want 20", got)
	}
}
