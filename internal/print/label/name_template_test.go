package label

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type nameCase struct {
	name string
	tpl  string
	data LabelData
	date bool
}

func nameCases() []nameCase {
	return []nameCase{
		{"title-only", "name", LabelData{Name: "Kabel USB-C"}, false},
		{"title-location", "name", LabelData{Name: "Zasilacz 12V", Location: "Pracownia → Regał 2 → Szuflada"}, false},
		{"with-qr", "name-qr", LabelData{Name: "Zasilacz 12V", Location: "Pracownia → Regał 2", QRContent: "https://qlx.local/item/42"}, false},
		{"with-date", "name", LabelData{Name: "Kabel USB-C", Location: "Pracownia"}, true},
		{"very-long", "name", LabelData{
			Name:     "Zestaw śrubek do elektroniki precyzyjnej z końcówkami wymiennymi i etui",
			Location: "Pracownia → Regał 2 → Szuflada górna → Pudełko z drobnymi częściami",
		}, false},
	}
}

func TestNameTemplateFitsPrintableArea(t *testing.T) {
	for mn, media := range fitMedias {
		for _, c := range nameCases() {
			t.Run(mn+"/"+c.name, func(t *testing.T) {
				img, err := Render(c.data, c.tpl, media, RenderOpts{PrintDate: c.date})
				if err != nil {
					t.Fatal(err)
				}
				if img.Bounds().Dx() != media.WidthPx || img.Bounds().Dy() != media.HeightPx {
					t.Fatalf("size = %v", img.Bounds().Size())
				}
				ink := inkBounds(img)
				if ink.Empty() {
					t.Fatal("blank label")
				}
				if ink.Min.X < 0 || ink.Min.Y < 0 || ink.Max.X > media.WidthPx || ink.Max.Y > media.HeightPx {
					t.Errorf("ink %v outside the label", ink)
				}
			})
		}
	}
}

func TestNameTemplateTitleLargerThanSimple(t *testing.T) {
	for mn, media := range fitMedias {
		t.Run(mn, func(t *testing.T) {
			data := LabelData{Name: "Kabel"}
			name, err := Render(data, "name", media, RenderOpts{})
			if err != nil {
				t.Fatal(err)
			}
			simple, err := Render(data, "simple", media, RenderOpts{})
			if err != nil {
				t.Fatal(err)
			}
			if inkBounds(name).Dy() < 2*inkBounds(simple).Dy() {
				t.Errorf("name ink %v should be at least twice as tall as simple ink %v", inkBounds(name), inkBounds(simple))
			}
		})
	}
}

func TestNameTemplateQROptIn(t *testing.T) {
	media := fitMedias["384x160@203"]
	data := LabelData{Name: "Ab", QRContent: "https://qlx.local/item/1"}
	plain, err := Render(data, "name", media, RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	withQR, err := Render(data, "name-qr", media, RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	// The QR sits at the top-right corner: only name-qr has ink there.
	corner := image.Rect(media.WidthPx-8-100, 8, media.WidthPx-8, 8+100)
	if !inkIn(plain, corner).Empty() {
		t.Error("name template must not draw a QR code")
	}
	if inkIn(withQR, corner).Empty() {
		t.Error("name-qr template must draw a QR code")
	}
}

// inkIn returns the bounding box of dark pixels inside r.
func inkIn(img image.Image, r image.Rectangle) image.Rectangle {
	return inkBounds(img.(interface {
		SubImage(image.Rectangle) image.Image
	}).SubImage(r))
}

// anyInk reports whether any non-white pixel (including light gray) lies in r.
func anyInk(img image.Image, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if cr, cg, cb, _ := img.At(x, y).RGBA(); cr != 0xffff || cg != 0xffff || cb != 0xffff {
				return true
			}
		}
	}
	return false
}

func TestNameTemplateDateFooterKept(t *testing.T) {
	old := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC) }
	defer func() { nowFunc = old }()

	media := fitMedias["696x271@300"]
	data := LabelData{Name: "Kabel"}
	with, err := Render(data, "name", media, RenderOpts{PrintDate: true})
	if err != nil {
		t.Fatal(err)
	}
	without, err := Render(data, "name", media, RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	footer := image.Rect(0, media.HeightPx-30, media.WidthPx, media.HeightPx)
	if !anyInk(with, footer) {
		t.Error("date footer missing")
	}
	if anyInk(without, footer) {
		t.Error("unexpected ink in footer area without PrintDate")
	}
}

func TestNameTemplateLocationArrowRenders(t *testing.T) {
	// "→" has no glyph in Noto Sans; the fitted location must not contain it.
	if got := strings.ReplaceAll("A → B", "→", "›"); strings.Contains(got, "→") {
		t.Fatal("sanity")
	}
	rt, err := resolveSlotText(Element{Slot: "location", Fit: "fill", FontFamily: "noto-sans"}, "A → B", Schema{}, 384, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rt.text, "→") {
		t.Errorf("arrow not substituted: %q", rt.text)
	}
}

// TestWriteNameSamples renders sample PNGs for manual review. It only runs
// when QLX_SAMPLES_DIR is set; PNGs are never committed.
func TestWriteNameSamples(t *testing.T) {
	dir := os.Getenv("QLX_SAMPLES_DIR")
	if dir == "" {
		t.Skip("set QLX_SAMPLES_DIR to write sample PNGs")
	}
	old := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC) }
	defer func() { nowFunc = old }()

	for mn, media := range fitMedias {
		for _, c := range nameCases() {
			img, err := Render(c.data, c.tpl, media, RenderOpts{PrintDate: c.date})
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Join( //nolint:gosec // test-only path from an env var
				dir, c.name+"-"+strings.ReplaceAll(mn, "@", "_")+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, img); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
