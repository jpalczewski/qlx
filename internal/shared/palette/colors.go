package palette

import "math/rand"

// Color represents a named color from the curated palette.
type Color struct {
	Name string
	Hex  string
}

var colors = [...]Color{
	{"red", "#e94560"},
	{"orange", "#f4845f"},
	{"amber", "#f5a623"},
	{"yellow", "#ffc93c"},
	{"green", "#4ecca3"},
	{"teal", "#2ec4b6"},
	{"blue", "#4d9de0"},
	{"indigo", "#7b6cf6"},
	{"purple", "#b07cd8"},
	{"pink", "#e84393"},
}

var colorIndex = func() map[string]Color {
	m := make(map[string]Color, len(colors))
	for _, c := range colors {
		m[c.Name] = c
	}
	return m
}()

// ValidColor reports whether name is a valid palette color.
func ValidColor(name string) bool {
	_, ok := colorIndex[name]
	return ok
}

// ColorByName returns the color with the given name.
func ColorByName(name string) (Color, bool) {
	c, ok := colorIndex[name]
	return c, ok
}

// RandomColor returns a random color from the palette.
func RandomColor() Color {
	return colors[rand.Intn(len(colors))]
}

// AllColors returns all palette colors in display order.
func AllColors() []Color {
	result := make([]Color, len(colors))
	copy(result, colors[:])
	return result
}
