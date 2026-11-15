package label

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

var updateGolden = flag.Bool("update", false, "rewrite golden digest files in testdata/")

// goldenFile returns the digest file for the current architecture: the x/image
// vector rasterizer rounds float32 differently on arm64 (fused multiply-add)
// and amd64, so anti-aliased pixels differ between them.
func goldenFile() string {
	return "testdata/golden_digests_" + runtime.GOARCH + ".txt"
}

// imageDigest returns a stable digest of an image's size and RGBA pixels.
// Digests are stored instead of PNG files so that no binary images are committed.
func imageDigest(img image.Image) string {
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	h := sha256.New()
	fmt.Fprintf(h, "%dx%d\n", b.Dx(), b.Dy())
	h.Write(rgba.Pix)
	return hex.EncodeToString(h.Sum(nil))
}

func goldenDatasets() map[string]LabelData {
	return map[string]LabelData{
		"short": {
			Name:      "Kabel USB",
			Location:  "Pokój → Półka",
			QRContent: "https://qlx.local/item/1",
			BarcodeID: "abc-123",
		},
		"full": {
			Name:        "Zestaw śrubek do elektroniki precyzyjnej",
			Description: "Komplet 32 końcówek, żółte etui, używane rzadko",
			Location:    "Pracownia → Regał 2 → Szuflada górna → Pudełko",
			QRContent:   "https://qlx.local/container/42",
			BarcodeID:   "7d1b9c1e-0000-4000-8000-123456789abc",
			Icon:        "wrench",
			Tags: []LabelTag{
				{Name: "arduino", Path: []string{"elektronika", "arduino"}},
				{Name: "narzędzia", Path: []string{"narzędzia"}},
			},
			Children: []LabelChild{{Name: "Rezystory"}, {Name: "Kondensatory"}, {Name: "LED"}},
		},
	}
}

// goldenSchemas lists the built-in schemas that existed before text fitting
// was introduced; their appearance must never change.
var goldenSchemas = []string{"compact", "contents", "detailed", "micro", "simple", "standard"}

func renderGoldenImages() (map[string]image.Image, error) {
	fixed := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	old := nowFunc
	nowFunc = func() time.Time { return fixed }
	defer func() { nowFunc = old }()

	medias := map[string]MediaInfo{
		"384x0":   {WidthPx: 384, DPI: 203},
		"384x160": {WidthPx: 384, HeightPx: 160, DPI: 203},
	}
	out := map[string]image.Image{}
	for _, tpl := range goldenSchemas {
		for dn, data := range goldenDatasets() {
			for mn, media := range medias {
				for _, date := range []bool{false, true} {
					img, err := Render(data, tpl, media, RenderOpts{PrintDate: date})
					if err != nil {
						return nil, fmt.Errorf("%s/%s/%s: %w", tpl, dn, mn, err)
					}
					out[fmt.Sprintf("%s/%s/%s/date=%t", tpl, dn, mn, date)] = img
				}
			}
		}
	}
	return out, nil
}

func readGolden(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(goldenFile())
	if err != nil {
		t.Fatalf("open %s (run with -update to create): %v", goldenFile(), err)
	}
	defer f.Close() //nolint:errcheck // read-only
	m := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

// TestGoldenBuiltinSchemas guards that existing built-in templates keep rendering
// pixel-identically on Niimbot-like media (384 px wide, continuous and 160 px die-cut).
// Run `go test ./internal/print/label -run Golden -update` to re-record deliberately.
func TestGoldenBuiltinSchemas(t *testing.T) {
	imgs, err := renderGoldenImages()
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(imgs))
	for k := range imgs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenFile()), 0o750); err != nil {
			t.Fatal(err)
		}
		var sb strings.Builder
		for _, k := range keys {
			fmt.Fprintf(&sb, "%s %s\n", k, imageDigest(imgs[k]))
		}
		if err := os.WriteFile(goldenFile(), []byte(sb.String()), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}

	want := readGolden(t)
	if len(want) != len(keys) {
		t.Errorf("golden entries = %d, rendered = %d", len(want), len(keys))
	}
	for _, k := range keys {
		if got := imageDigest(imgs[k]); got != want[k] {
			t.Errorf("%s: digest changed\n got  %s\n want %s", k, got, want[k])
		}
	}
}
