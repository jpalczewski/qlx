package label

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// TestPairWarningsMatchLayout: a warning is reported exactly when the layout
// carries the corresponding shortening (no stale warnings after a retry).
func TestPairWarningsMatchLayout(t *testing.T) {
	for _, mc := range pairMedias {
		for seed := 0; seed < 150; seed++ {
			r := rand.New(rand.NewSource(int64(seed)*31 + 7)) //nolint:gosec // deterministic test data
			d, opts := synData(r)
			pages, _, warns, err := planPair(d, mc.media, opts)
			if err != nil {
				t.Fatal(err)
			}
			levels := locationLevels(d)
			loc, hasLoc := findBlock(pages[1], "location")
			cut := hasLoc && strings.HasPrefix(linesText(loc), ellipsis)
			if cut != hasWarn(warns, PairWarnLocationShortened) {
				t.Fatalf("%s seed %d: location cut=%v but warnings %v (levels %q, drawn %q)", mc.name, seed, cut, warns, levels, linesText(loc))
			}
			desc := pairNormalize(d.Description)
			if opts.PrintDescription && desc != "" {
				d1, on1 := findBlock(pages[0], "desc")
				d2, on2 := findBlock(pages[1], "desc")
				full := (on1 && linesText(d1) == desc) || (on2 && linesText(d2) == desc)
				if full == hasWarn(warns, PairWarnDescriptionShortened) {
					t.Fatalf("%s seed %d: description full=%v but warnings %v", mc.name, seed, full, warns)
				}
			}
		}
	}
}

func TestPairLocationLevels(t *testing.T) {
	tests := []struct {
		name string
		d    LabelData
		want []string
	}{
		{"separator", LabelData{Location: "Pokój" + LocationSeparator + "Półka"}, []string{"Pokój", "Półka"}},
		{"bare arrow in a name", LabelData{Location: "Strefa A→B" + LocationSeparator + "Półka"}, []string{"Strefa A→B", "Półka"}},
		{"path wins", LabelData{Location: "x → y", LocationPath: []string{"A → B", "C"}}, []string{"A → B", "C"}},
		{"empty", LabelData{}, nil},
	}
	for _, tt := range tests {
		if got := locationLevels(tt.d); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestPairEmptyChildrenSkipped(t *testing.T) {
	d := LabelData{Name: "Pudełko", Location: "Pracownia", Children: []LabelChild{
		{Name: "  "}, {Name: "Dioda"}, {Name: ""}, {Name: "Rezystory", IsContainer: true},
	}}
	if got := contextCounts(d.Children); got != "Zawiera: 1 pojemnik · 1 przedmiot" {
		t.Errorf("counts = %q", got)
	}
	pages, _, _, _ := planPair(d, pairMedias[0].media, RenderOpts{})
	lb, ok := findBlock(pages[1], "children")
	if !ok {
		t.Fatal("no child list")
	}
	if got := linesText(lb); strings.Contains(got, "· ·") || strings.HasSuffix(got, "·") {
		t.Errorf("stray separator: %q", got)
	}
}

func TestPairRejectsUnusableDPI(t *testing.T) {
	if _, err := RenderPair(LabelData{Name: "x"}, MediaInfo{WidthPx: 696, HeightPx: 271, DPI: 1}, RenderOpts{}); err == nil {
		t.Error("DPI 1 must be rejected")
	}
}
