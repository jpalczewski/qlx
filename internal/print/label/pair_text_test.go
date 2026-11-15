package label

import (
	"reflect"
	"strings"
	"testing"
)

func TestPairPlural(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "pojemników"}, {1, "pojemnik"}, {2, "pojemniki"}, {3, "pojemniki"}, {4, "pojemniki"},
		{5, "pojemników"}, {11, "pojemników"}, {12, "pojemników"}, {13, "pojemników"}, {14, "pojemników"},
		{15, "pojemników"}, {21, "pojemników"}, {22, "pojemniki"}, {23, "pojemniki"}, {24, "pojemniki"},
		{25, "pojemników"}, {32, "pojemniki"}, {100, "pojemników"}, {101, "pojemników"}, {102, "pojemniki"},
		{111, "pojemników"}, {112, "pojemników"}, {114, "pojemników"}, {122, "pojemniki"},
	}
	for _, tt := range tests {
		if got := pairPlural(tt.n, "pojemnik", "pojemniki", "pojemników"); got != tt.want {
			t.Errorf("pairPlural(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestContextCounts(t *testing.T) {
	mk := func(containers, items int) []LabelChild {
		var out []LabelChild
		for i := 0; i < containers; i++ {
			out = append(out, LabelChild{Name: "c", IsContainer: true})
		}
		for i := 0; i < items; i++ {
			out = append(out, LabelChild{Name: "i"})
		}
		return out
	}
	tests := []struct {
		containers, items int
		want              string
	}{
		{0, 0, ""},
		{1, 1, "Zawiera: 1 pojemnik · 1 przedmiot"},
		{2, 3, "Zawiera: 2 pojemniki · 3 przedmioty"},
		{5, 22, "Zawiera: 5 pojemników · 22 przedmioty"},
		{0, 12, "Zawiera: 12 przedmiotów"},
		{24, 0, "Zawiera: 24 pojemniki"},
	}
	for _, tt := range tests {
		if got := contextCounts(mk(tt.containers, tt.items)); got != tt.want {
			t.Errorf("contextCounts(%d,%d) = %q, want %q", tt.containers, tt.items, got, tt.want)
		}
	}
}

func TestPairTokens(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"  ", nil},
		{"kabel  do  w domu", []string{"kabel", "do", "w domu"}},
		{"A teraz Z domu i o tym", []string{"A teraz", "Z domu", "i o tym"}},
		{"a w z", []string{"a w z"}},
		{"ala ma kota o", []string{"ala", "ma", "kota", "o"}},
		{"wa za", []string{"wa", "za"}},
	}
	for _, tt := range tests {
		if got := pairTokens(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("pairTokens(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

//nolint:gocyclo // table of subtests
func TestPairWrap(t *testing.T) {
	m := newPairMeasurer()
	const px = 30
	text := "żółta kaczka w  wannie z gumową piłką"
	full := m.width(true, px, pairNormalize(text))

	t.Run("fits one line", func(t *testing.T) {
		lines, ok := m.wrap(true, px, text, full, 3, false)
		if !ok || len(lines) != 1 || lines[0] != pairNormalize(text) {
			t.Errorf("got %q ok=%v", lines, ok)
		}
	})
	t.Run("minimal line count then balanced", func(t *testing.T) {
		lines, ok := m.wrap(true, px, text, full*6/10, 3, false)
		if !ok || len(lines) != 2 {
			t.Fatalf("got %q ok=%v", lines, ok)
		}
		a, b := m.width(true, px, lines[0]), m.width(true, px, lines[1])
		// greedy would fill line 1 up to 60% of the width; balancing must be closer.
		if abs(a-b) > full/5 {
			t.Errorf("lines not balanced: %d vs %d (%q)", a, b, lines)
		}
	})
	t.Run("single-letter words stay glued", func(t *testing.T) {
		for w := full / 3; w <= full; w += 7 {
			lines, ok := m.wrap(true, px, text, w, 0, false)
			if !ok {
				continue
			}
			for _, l := range lines {
				fs := strings.Fields(l)
				if pairIsSingle(fs[len(fs)-1]) && len(fs) > 0 && l != lines[len(lines)-1] {
					t.Errorf("line %q ends with a single-letter word (width %d)", l, w)
				}
			}
		}
	})
	t.Run("too many lines", func(t *testing.T) {
		if _, ok := m.wrap(true, px, text, full/4, 2, false); ok {
			t.Error("expected failure for maxLines")
		}
	})
	t.Run("long word only split as last resort", func(t *testing.T) {
		long := "Nieprzeciwdziałającemu"
		w := m.width(true, px, long) - 10
		if _, ok := m.wrap(true, px, long, w, 0, false); ok {
			t.Error("a too wide word must fail without split")
		}
		lines, ok := m.wrap(true, px, "a "+long, w, 0, true)
		if !ok || strings.ReplaceAll(strings.Join(lines, ""), " ", "") != "a"+long {
			t.Errorf("split lines %q ok=%v", lines, ok)
		}
		for _, l := range lines {
			if m.width(true, px, l) > w {
				t.Errorf("line %q wider than %d", l, w)
			}
		}
	})
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestPairEllipsize(t *testing.T) {
	m := newPairMeasurer()
	s := "Dokumentacja techniczna stanowiska"
	full := m.width(true, 24, s)
	if got := m.ellipsize(true, 24, s, full); got != s {
		t.Errorf("fitting text changed: %q", got)
	}
	got := m.ellipsize(true, 24, s, full/2)
	if !strings.HasSuffix(got, pairEllipsis) || m.width(true, 24, got) > full/2 {
		t.Errorf("ellipsized %q (w=%d, max %d)", got, m.width(true, 24, got), full/2)
	}
	if got := m.ellipsize(true, 24, s, 3); got != "" {
		t.Errorf("nothing fits, got %q", got)
	}
}

func TestPairFlowKeepsLevelsTogether(t *testing.T) {
	m := newPairMeasurer()
	const px = 28
	levels := []string{"Pracownia", "Regał na bardzo długie rzeczy", "Szuflada"}
	runs, ok := m.flowRuns(px, locationParts(levels, false), 10000, false)
	if !ok || len(runs) != 3 || runs[0].text != "Pracownia ›" || runs[2].text != "Szuflada" || !runs[2].bold || runs[0].bold {
		t.Fatalf("runs = %+v", runs)
	}
	// A level wider than the line breaks at words, the separator stays on its last word.
	runs, ok = m.flowRuns(px, locationParts(levels, false), m.width(false, px, "Pracownia ›")+20, false)
	if !ok {
		t.Fatal("not ok")
	}
	var texts []string
	for _, r := range runs {
		texts = append(texts, r.text)
	}
	if want := []string{"Pracownia ›", "Regał", "na", "bardzo", "długie", "rzeczy ›", "Szuflada"}; !reflect.DeepEqual(texts, want) {
		t.Errorf("texts = %q, want %q", texts, want)
	}
}
