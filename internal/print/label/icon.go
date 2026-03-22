package label

import (
	"fmt"
	"image"
	"strings"
	"sync"

	"github.com/erxyi/qlx/internal/shared/palette"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

var iconCache sync.Map

// RasterizeIcon renders a Phosphor SVG icon to a bitmap of sizePx × sizePx.
// Returns nil image (no error) if name is empty. Cached by (name, sizePx).
func RasterizeIcon(name string, sizePx int) (image.Image, error) {
	if name == "" {
		return nil, nil
	}

	key := fmt.Sprintf("%s:%d", name, sizePx)
	if cached, ok := iconCache.Load(key); ok {
		return cached.(image.Image), nil
	}

	svgData, err := palette.SVG(name)
	if err != nil {
		return nil, fmt.Errorf("rasterize icon %q: %w", name, err)
	}

	// Replace currentColor with black — oksvg doesn't support CSS currentColor.
	svgStr := strings.ReplaceAll(string(svgData), "currentColor", "black")

	icon, err := oksvg.ReadIconStream(strings.NewReader(svgStr))
	if err != nil {
		return nil, fmt.Errorf("parse icon %q SVG: %w", name, err)
	}

	icon.SetTarget(0, 0, float64(sizePx), float64(sizePx))

	img := image.NewRGBA(image.Rect(0, 0, sizePx, sizePx))
	// Fill with transparent background
	for i := range img.Pix {
		img.Pix[i] = 0
	}

	scanner := rasterx.NewScannerGV(sizePx, sizePx, img, img.Bounds())
	dasher := rasterx.NewDasher(sizePx, sizePx, scanner)
	icon.Draw(dasher, 1.0)

	iconCache.Store(key, img)
	return img, nil
}
