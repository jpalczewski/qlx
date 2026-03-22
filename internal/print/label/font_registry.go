package label

import (
	"embed"
	"fmt"
	"sort"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
)

//go:embed fonts
var fontsFS embed.FS

// fontEntry describes a single font in the catalog.
type fontEntry struct {
	// file is the path within fontsFS (e.g. "fonts/noto-sans-regular.ttf").
	// Empty for the built-in basic font.
	file string
	// smallFile is an optional smaller variant (used by spleen below threshold).
	smallFile string
	// threshold is the size boundary for switching between small and large variants.
	threshold float64
	// bitmap indicates the font has fixed sizes and should use the variant's native size.
	bitmap bool
	// basic indicates this is the built-in basicfont.Face7x13 (no file needed).
	basic bool
}

// fontCatalog maps font names to their catalog entries.
var fontCatalog = map[string]fontEntry{
	"spleen": {
		file:      "fonts/spleen-12x24.otf",
		smallFile: "fonts/spleen-8x16.otf",
		threshold: 20,
		bitmap:    true,
	},
	"noto-sans": {
		file: "fonts/noto-sans-regular.ttf",
	},
	"go-mono": {
		file: "fonts/go-mono-regular.ttf",
	},
	"terminus": {
		file: "fonts/terminus-regular.ttf",
	},
	"basic": {
		basic: true,
	},
}

// parsedFonts caches parsed opentype.Font objects by file path.
var parsedFonts sync.Map // map[string]*opentype.Font

// faceCache caches font.Face objects by "name:size" key.
var faceCache sync.Map // map[string]font.Face

// LoadFace returns a font.Face for the given font name and pixel size.
// Results are cached; subsequent calls with the same name and size return the same Face.
func LoadFace(name string, sizePx float64) (font.Face, error) {
	key := fmt.Sprintf("%s:%.1f", name, sizePx)

	if cached, ok := faceCache.Load(key); ok {
		return cached.(font.Face), nil
	}

	entry, ok := fontCatalog[name]
	if !ok {
		return nil, fmt.Errorf("unknown font: %q", name)
	}

	if entry.basic {
		face := basicfont.Face7x13
		faceCache.Store(key, face)
		return face, nil
	}

	// Determine which file and size to use.
	file := entry.file
	size := sizePx
	if entry.bitmap {
		if entry.smallFile != "" && sizePx < entry.threshold {
			file = entry.smallFile
			size = 16.0
		} else {
			size = 24.0
		}
	}

	otFont, err := parsedFont(file)
	if err != nil {
		return nil, err
	}

	face, err := opentype.NewFace(otFont, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("create face %s@%.0fpx: %w", name, sizePx, err)
	}

	faceCache.Store(key, face)
	return face, nil
}

// parsedFont returns a cached parsed opentype.Font for the given embedded file path.
func parsedFont(file string) (*opentype.Font, error) {
	if cached, ok := parsedFonts.Load(file); ok {
		return cached.(*opentype.Font), nil
	}

	data, err := fontsFS.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read font %s: %w", file, err)
	}

	otFont, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font %s: %w", file, err)
	}

	parsedFonts.Store(file, otFont)
	return otFont, nil
}

// FontNames returns a sorted list of all available font names.
func FontNames() []string {
	names := make([]string, 0, len(fontCatalog))
	for name := range fontCatalog {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// IsBasicFont returns true if the named font is the built-in ASCII-only basic font.
func IsBasicFont(name string) bool {
	entry, ok := fontCatalog[name]
	return ok && entry.basic
}

// TransliteratePL replaces Polish diacritic characters with their ASCII equivalents.
func TransliteratePL(s string) string {
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
