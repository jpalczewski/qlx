package label

import (
	"embed"
	"encoding/json"
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"sync"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

// Schema defines a label template layout declaratively.
type Schema struct {
	Name       string    `json:"name"`
	Padding    int       `json:"padding"`
	FontFamily string    `json:"font_family"` // "basic" = Go basicfont (ASCII-only); default = Terminus
	Elements   []Element `json:"elements"`
}

// Element defines a single layout slot in a schema.
type Element struct {
	Slot       string  `json:"slot"`        // title, description, location, qr, barcode, tags, children
	FontSize   float64 `json:"font_size"`   // pixel size for text slots
	FontFamily string  `json:"font_family"` // override schema default; empty = inherit
	Align      string  `json:"align"`       // left, center, right
	Wrap       bool    `json:"wrap"`        // enable text wrapping
	Color      string  `json:"color"`       // hex color e.g. "#505050"
	Size       int     `json:"size"`        // px for qr
	Height     int     `json:"height"`      // px for barcode
	ShowPath   string  `json:"show_path"`   // tags only: "auto"|"true"|"false" (default "auto")
	ShowIcons  *bool   `json:"show_icons"`  // render inline icons (default true for title/children/tags)
}

// parseSchema parses JSON bytes into a Schema, applying defaults.
func parseSchema(data []byte) (Schema, error) {
	var s Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return Schema{}, fmt.Errorf("parse schema: %w", err)
	}
	if s.Padding == 0 {
		s.Padding = 8
	}
	for i := range s.Elements {
		if s.Elements[i].FontSize == 0 {
			s.Elements[i].FontSize = 13
		}
		if s.Elements[i].Align == "" {
			s.Elements[i].Align = "left"
		}
		if s.Elements[i].Color == "" {
			s.Elements[i].Color = "#000000"
		}
		if s.Elements[i].ShowPath == "" {
			s.Elements[i].ShowPath = "auto"
		}
	}
	return s, nil
}

//go:embed schemas/*.json
var schemasFS embed.FS

var (
	builtinSchemas map[string]Schema
	schemasOnce    sync.Once
	schemasInitErr error
)

func initSchemas() {
	builtinSchemas = make(map[string]Schema)
	entries, err := schemasFS.ReadDir("schemas")
	if err != nil {
		schemasInitErr = fmt.Errorf("read schemas dir: %w", err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := schemasFS.ReadFile("schemas/" + entry.Name())
		if err != nil {
			schemasInitErr = fmt.Errorf("read schema %s: %w", entry.Name(), err)
			return
		}
		schema, err := parseSchema(data)
		if err != nil {
			schemasInitErr = fmt.Errorf("parse schema %s: %w", entry.Name(), err)
			return
		}
		builtinSchemas[schema.Name] = schema
	}
}

// GetSchema returns a built-in schema by name.
func GetSchema(name string) (Schema, bool) {
	schemasOnce.Do(initSchemas)
	if schemasInitErr != nil {
		webutil.LogError("label: schema init failed: %v", schemasInitErr)
		return Schema{}, false
	}
	s, ok := builtinSchemas[name]
	return s, ok
}

// SchemaNames returns sorted names of all built-in schemas.
func SchemaNames() []string {
	schemasOnce.Do(initSchemas)
	if schemasInitErr != nil {
		webutil.LogError("label: schema init failed: %v", schemasInitErr)
		return nil
	}
	names := make([]string, 0, len(builtinSchemas))
	for n := range builtinSchemas {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// parseHexColor converts a hex color string to color.RGBA.
func parseHexColor(hex string) color.RGBA {
	if len(hex) == 7 && hex[0] == '#' {
		r, _ := strconv.ParseUint(hex[1:3], 16, 8)
		g, _ := strconv.ParseUint(hex[3:5], 16, 8)
		b, _ := strconv.ParseUint(hex[5:7], 16, 8)
		return color.RGBA{uint8(r), uint8(g), uint8(b), 255}
	}
	return color.RGBA{0, 0, 0, 255}
}
