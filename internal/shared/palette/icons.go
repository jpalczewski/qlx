package palette

import "math/rand"

// Icon represents a named icon from the curated set.
type Icon struct {
	Name     string
	Category string
}

// IconCategory groups icons by theme.
type IconCategory struct {
	Name  string
	Icons []Icon
}

// icons is the curated icon catalog. Each name must match a file in phosphor/.
var icons = [...]Icon{
	// Tools & Hardware
	{"wrench", "tools"}, {"hammer", "tools"}, {"screwdriver", "tools"},
	{"nut", "tools"}, {"gear", "tools"}, {"paint-brush", "tools"},
	{"ruler", "tools"}, {"scissors", "tools"},
	// Electronics
	{"cpu", "electronics"}, {"circuit-board", "electronics"}, {"lightning", "electronics"},
	{"battery-full", "electronics"}, {"monitor", "electronics"}, {"camera", "electronics"},
	{"speaker-high", "electronics"}, {"wifi-high", "electronics"},
	// Clothing & Textiles
	{"t-shirt", "clothing"}, {"pants", "clothing"}, {"sneaker", "clothing"},
	{"coat-hanger", "clothing"}, {"backpack", "clothing"},
	// Food & Kitchen
	{"cooking-pot", "food"}, {"knife", "food"}, {"wine", "food"},
	{"coffee", "food"}, {"leaf", "food"}, {"grain", "food"},
	// Chemicals & Lab
	{"flask", "chemicals"}, {"test-tube", "chemicals"}, {"drop", "chemicals"},
	{"warning", "chemicals"}, {"thermometer", "chemicals"}, {"fire", "chemicals"},
	// Office & Documents
	{"file-text", "office"}, {"folder", "office"}, {"clipboard-text", "office"},
	{"pen", "office"}, {"notebook", "office"}, {"envelope", "office"},
	{"calendar", "office"}, {"printer", "office"},
	// Home & Storage
	{"house", "home"}, {"package", "home"}, {"archive-box", "home"},
	{"lockers", "home"}, {"lamp", "home"}, {"bed", "home"},
	{"armchair", "home"}, {"door", "home"},
	// Transport
	{"truck", "transport"}, {"car", "transport"}, {"airplane", "transport"},
	{"barcode", "transport"}, {"map-pin", "transport"}, {"globe", "transport"},
	// Misc
	{"star", "misc"}, {"heart", "misc"}, {"tag", "misc"},
	{"magnifying-glass", "misc"}, {"chat-circle", "misc"}, {"flag", "misc"},
	{"shield", "misc"}, {"key", "misc"}, {"lock", "misc"},
	{"user", "misc"}, {"users", "misc"}, {"first-aid-kit", "misc"},
}

var iconIndex = func() map[string]Icon {
	m := make(map[string]Icon, len(icons))
	for _, ic := range icons {
		m[ic.Name] = ic
	}
	return m
}()

// ValidIcon reports whether name is a valid icon in the catalog.
func ValidIcon(name string) bool {
	_, ok := iconIndex[name]
	return ok
}

// IconByName returns the icon with the given name.
func IconByName(name string) (Icon, bool) {
	ic, ok := iconIndex[name]
	return ic, ok
}

// RandomIcon returns a random icon from the catalog.
func RandomIcon() Icon {
	return icons[rand.Intn(len(icons))]
}

// AllIcons returns all icons in catalog order.
func AllIcons() []Icon {
	result := make([]Icon, len(icons))
	copy(result, icons[:])
	return result
}

// IconCategories returns icons grouped by category in display order.
func IconCategories() []IconCategory {
	order := []string{"tools", "electronics", "clothing", "food", "chemicals", "office", "home", "transport", "misc"}
	catMap := make(map[string][]Icon)
	for _, ic := range icons {
		catMap[ic.Category] = append(catMap[ic.Category], ic)
	}
	var result []IconCategory
	for _, name := range order {
		if ics, ok := catMap[name]; ok {
			result = append(result, IconCategory{Name: name, Icons: ics})
		}
	}
	return result
}
