package palette

import (
	"embed"
	"fmt"
)

//go:embed phosphor/*.svg
var IconFS embed.FS

// SVG returns the raw SVG bytes for the given icon name.
func SVG(name string) ([]byte, error) {
	data, err := IconFS.ReadFile("phosphor/" + name + ".svg")
	if err != nil {
		return nil, fmt.Errorf("icon %q not found: %w", name, err)
	}
	return data, nil
}
