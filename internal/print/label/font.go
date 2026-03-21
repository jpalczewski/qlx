package label

import (
	_ "embed"
	"fmt"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

//go:embed fonts/TerminusTTF-4.49.3.ttf
var terminusData []byte

var (
	terminusFont *opentype.Font
	fontOnce     sync.Once
	fontInitErr  error
)

func initFont() {
	terminusFont, fontInitErr = opentype.Parse(terminusData)
}

// loadFontFace returns a Terminus font face at the given pixel size.
func loadFontFace(sizePx float64) (font.Face, error) {
	fontOnce.Do(initFont)
	if fontInitErr != nil {
		return nil, fmt.Errorf("font init: %w", fontInitErr)
	}
	return opentype.NewFace(terminusFont, &opentype.FaceOptions{
		Size:    sizePx,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}
