package label

import "testing"

func TestLoadFontFace(t *testing.T) {
	face, err := loadFontFace(13)
	if err != nil {
		t.Fatalf("loadFontFace: %v", err)
	}
	if face == nil {
		t.Fatal("face is nil")
	}
}

func TestLoadFontFacePolishChars(t *testing.T) {
	face, err := loadFontFace(13)
	if err != nil {
		t.Fatalf("loadFontFace: %v", err)
	}
	for _, r := range "ąćęłńóśźżĄĆĘŁŃÓŚŹŻ" {
		adv, ok := face.GlyphAdvance(r)
		if !ok || adv == 0 {
			t.Errorf("glyph missing for %c", r)
		}
	}
}
