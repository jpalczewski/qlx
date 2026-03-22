package label

import (
	"sort"
	"testing"
)

func TestLoadFaceSpleen(t *testing.T) {
	face, err := LoadFace("spleen", 24)
	if err != nil {
		t.Fatalf("LoadFace spleen large: %v", err)
	}
	if face == nil {
		t.Fatal("face is nil")
	}
}

func TestLoadFaceSpleenSmall(t *testing.T) {
	face, err := LoadFace("spleen", 13)
	if err != nil {
		t.Fatalf("LoadFace spleen small: %v", err)
	}
	if face == nil {
		t.Fatal("face is nil")
	}
}

func TestLoadFaceBasic(t *testing.T) {
	face, err := LoadFace("basic", 13)
	if err != nil {
		t.Fatalf("LoadFace basic: %v", err)
	}
	if face == nil {
		t.Fatal("face is nil")
	}
}

func TestLoadFaceNotoSans(t *testing.T) {
	face, err := LoadFace("noto-sans", 16)
	if err != nil {
		t.Fatalf("LoadFace noto-sans: %v", err)
	}
	if face == nil {
		t.Fatal("face is nil")
	}
}

func TestLoadFaceGoMono(t *testing.T) {
	face, err := LoadFace("go-mono", 14)
	if err != nil {
		t.Fatalf("LoadFace go-mono: %v", err)
	}
	if face == nil {
		t.Fatal("face is nil")
	}
}

func TestLoadFaceTerminus(t *testing.T) {
	face, err := LoadFace("terminus", 16)
	if err != nil {
		t.Fatalf("LoadFace terminus: %v", err)
	}
	if face == nil {
		t.Fatal("face is nil")
	}
}

func TestLoadFaceUnknown(t *testing.T) {
	_, err := LoadFace("nonexistent", 16)
	if err == nil {
		t.Fatal("expected error for unknown font, got nil")
	}
}

func TestLoadFaceCached(t *testing.T) {
	face1, err := LoadFace("go-mono", 12)
	if err != nil {
		t.Fatalf("LoadFace first call: %v", err)
	}
	face2, err := LoadFace("go-mono", 12)
	if err != nil {
		t.Fatalf("LoadFace second call: %v", err)
	}
	// sync.Map returns the same interface value for cached entries.
	if face1 != face2 {
		t.Error("expected cached face to be the same pointer")
	}
}

func TestFontNames(t *testing.T) {
	names := FontNames()
	if len(names) != 5 {
		t.Fatalf("FontNames() = %v, want 5 entries", names)
	}
	want := []string{"basic", "go-mono", "noto-sans", "spleen", "terminus"}
	sorted := make([]string, len(names))
	copy(sorted, names)
	sort.Strings(sorted)
	for i, n := range sorted {
		if n != want[i] {
			t.Errorf("FontNames()[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestIsBasicFont(t *testing.T) {
	if !IsBasicFont("basic") {
		t.Error("IsBasicFont(\"basic\") = false, want true")
	}
	if IsBasicFont("spleen") {
		t.Error("IsBasicFont(\"spleen\") = true, want false")
	}
	if IsBasicFont("unknown") {
		t.Error("IsBasicFont(\"unknown\") = true, want false")
	}
}

func TestTransliteratePL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ąćęłńóśźż", "acelnoszz"},
		{"ĄĆĘŁŃÓŚŹŻ", "ACELNOSZZ"},
		{"Zażółć gęślą jaźń", "Zazolc gesla jazn"},
		{"Hello", "Hello"},
		{"", ""},
	}
	for _, tt := range tests {
		got := TransliteratePL(tt.input)
		if got != tt.want {
			t.Errorf("TransliteratePL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLoadFaceSpleenPolishChars(t *testing.T) {
	face, err := LoadFace("spleen", 13)
	if err != nil {
		t.Fatalf("LoadFace: %v", err)
	}
	for _, r := range "ąćęłńóśźżĄĆĘŁŃÓŚŹŻ" {
		adv, ok := face.GlyphAdvance(r)
		if !ok || adv == 0 {
			t.Errorf("glyph missing for %c", r)
		}
	}
}
