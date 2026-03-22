package label

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
)

//go:embed fonts/spleen-12x24.otf
var spleen12x24Data []byte

//go:embed fonts/spleen-8x16.otf
var spleen8x16Data []byte

var (
	spleen12x24 *opentype.Font
	spleen8x16  *opentype.Font
	fontOnce    sync.Once
	fontInitErr error
)

func initFonts() {
	spleen12x24, fontInitErr = opentype.Parse(spleen12x24Data)
	if fontInitErr != nil {
		return
	}
	spleen8x16, fontInitErr = opentype.Parse(spleen8x16Data)
}

// loadFontFace returns a Spleen font face for the given pixel size.
// Sizes ≥ 20px use Spleen 12×24; smaller sizes use Spleen 8×16.
func loadFontFace(sizePx float64) (font.Face, error) {
	fontOnce.Do(initFonts)
	if fontInitErr != nil {
		return nil, fmt.Errorf("font init: %w", fontInitErr)
	}
	f := spleen8x16
	size := 16.0
	if sizePx >= 20 {
		f = spleen12x24
		size = 24.0
	}
	return opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

// loadBasicFontFace returns the Go built-in monospace bitmap font face (ASCII-only, fixed 7×13px).
func loadBasicFontFace() font.Face {
	return basicfont.Face7x13
}

// transliteratePL replaces Polish diacritic characters with their ASCII equivalents.
func transliteratePL(s string) string {
	r := strings.NewReplacer(
		"ą", "a", "Ą", "A",
		"ć", "c", "Ć", "C",
		"ę", "e", "Ę", "E",
		"ł", "l", "Ł", "L",
		"ń", "n", "Ń", "N",
		"ó", "o", "Ó", "O",
		"ś", "s", "Ś", "S",
		"ź", "z", "Ź", "Z",
		"ż", "z", "Ż", "Z",
	)
	return r.Replace(s)
}
