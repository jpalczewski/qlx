# Icon & Color System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a shared icon (Phosphor SVG) and color palette system to Items, Containers, and Tags with picker UI components.

**Architecture:** New `internal/shared/palette/` package as single source of truth for colors and icons. Store/service/handler layers extended with `color` and `icon` string fields. Reusable picker UI components (template partials + vanilla JS) embedded in all create/edit forms.

**Tech Stack:** Go (go:embed for SVGs), Phosphor Icons (regular weight SVG), CSS custom properties, vanilla JS, HTMX forms.

**Spec:** `docs/superpowers/specs/2026-03-21-icon-colors-design.md`

---

## File Map

### New Files
| File | Responsibility |
|------|---------------|
| `internal/shared/palette/colors.go` | Color palette definition, validation, random selection |
| `internal/shared/palette/colors_test.go` | Color palette tests |
| `internal/shared/palette/icons.go` | Icon catalog definition, categories, validation, random selection |
| `internal/shared/palette/icons_test.go` | Icon catalog tests |
| `internal/shared/palette/icons_embed.go` | go:embed for SVG files, SVG() accessor |
| `internal/shared/palette/icons_embed_test.go` | Embed accessor tests |
| `internal/shared/palette/phosphor/*.svg` | ~50-80 curated Phosphor Icons SVGs |
| `internal/embedded/templates/components/color_picker.html` | Color picker partial (`fields/color-picker`) |
| `internal/embedded/templates/components/icon_picker.html` | Icon picker partial (`fields/icon-picker`) |
| `internal/embedded/static/css/shared/pickers.css` | Picker grid styling |
| `internal/embedded/static/js/pickers.js` | Picker interaction (click → select → hidden input) |

### Modified Files
| File | Changes |
|------|---------|
| `internal/store/models.go` | Add `Color`, `Icon` fields to Container, Item, Tag |
| `internal/store/migrate.go` | Add `migrateV1ToV2` |
| `internal/store/store.go` | Update CreateContainer, UpdateContainer, CreateItem, UpdateItem signatures |
| `internal/store/tags.go` | Update CreateTag, UpdateTag signatures |
| `internal/store/store_test.go` | Update all callsites with color/icon params |
| `internal/service/interfaces.go` | Update ItemStore, ContainerStore, TagStore interfaces |
| `internal/service/inventory.go` | Add color/icon params, validation |
| `internal/service/inventory_test.go` | Update test callsites |
| `internal/service/tags.go` | Add color/icon params, validation |
| `internal/service/tags_test.go` | Update test callsites |
| `internal/ui/handlers.go` | Extract color/icon form values, pass to service (containers, items) |
| `internal/ui/handlers_tags.go` | Extract color/icon form values, pass to service (tags) |
| `internal/ui/server.go` | Add `icon` template function, register icon HTTP handler, add palette route |
| `internal/api/server.go` | Update API create/update handlers for containers and items |
| `internal/api/handlers_tags.go` | Update API tag create/update handlers |
| `internal/app/server.go` | Register icon static handler |
| `internal/embedded/static/css/shared/tokens.css` | Add `--palette-*` CSS custom properties |
| `internal/embedded/static/css/tags/tag-chips.css` | Color-aware tag chip styling |
| `internal/embedded/static/css/inventory/lists.css` | Color dot + icon in list items |
| `internal/embedded/templates/partials/inventory/container_list_item.html` | Replace emoji with inline SVG + color dot |
| `internal/embedded/templates/partials/inventory/item_list_item.html` | Replace emoji with inline SVG + color dot |
| `internal/embedded/templates/pages/inventory/containers.html` | Icon + color in detail view, breadcrumbs |
| `internal/embedded/templates/pages/inventory/item.html` | Icon + color in detail view |
| `internal/embedded/templates/pages/inventory/container_form.html` | Add picker partials |
| `internal/embedded/templates/pages/inventory/item_form.html` | Add picker partials |
| `internal/embedded/templates/pages/tags/tags.html` | Add color/icon pickers to tag quick-add form |
| `internal/embedded/templates/partials/tags/tag_chips.html` | Icon + color in tag chips |

---

## Task 1: Palette Package — Colors

**Files:**
- Create: `internal/shared/palette/colors.go`
- Create: `internal/shared/palette/colors_test.go`

- [ ] **Step 1: Write failing tests for color palette**

```go
// internal/shared/palette/colors_test.go
package palette

import "testing"

func TestValidColor(t *testing.T) {
	tests := []struct {
		name  string
		color string
		want  bool
	}{
		{"valid red", "red", true},
		{"valid teal", "teal", true},
		{"invalid", "neon", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidColor(tt.color); got != tt.want {
				t.Errorf("ValidColor(%q) = %v, want %v", tt.color, got, tt.want)
			}
		})
	}
}

func TestRandomColor(t *testing.T) {
	c := RandomColor()
	if c.Name == "" || c.Hex == "" {
		t.Errorf("RandomColor() returned empty: %+v", c)
	}
	if !ValidColor(c.Name) {
		t.Errorf("RandomColor() returned invalid color: %s", c.Name)
	}
}

func TestColorByName(t *testing.T) {
	c, ok := ColorByName("blue")
	if !ok {
		t.Fatal("ColorByName(blue) not found")
	}
	if c.Hex != "#4d9de0" {
		t.Errorf("ColorByName(blue).Hex = %s, want #4d9de0", c.Hex)
	}
	_, ok = ColorByName("nonexistent")
	if ok {
		t.Error("ColorByName(nonexistent) should return false")
	}
}

func TestAllColors(t *testing.T) {
	colors := AllColors()
	if len(colors) != 10 {
		t.Errorf("AllColors() len = %d, want 10", len(colors))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/shared/palette/ -run TestValidColor -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement colors.go**

```go
// internal/shared/palette/colors.go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/shared/palette/ -run "TestValidColor|TestRandomColor|TestColorByName|TestAllColors" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/shared/palette/colors.go internal/shared/palette/colors_test.go
git commit -m "feat(palette): add color palette with validation and random selection"
```

---

## Task 2: Palette Package — Icons Catalog

**Files:**
- Create: `internal/shared/palette/icons.go`
- Create: `internal/shared/palette/icons_test.go`

- [ ] **Step 1: Write failing tests for icon catalog**

```go
// internal/shared/palette/icons_test.go
package palette

import "testing"

func TestValidIcon(t *testing.T) {
	tests := []struct {
		name string
		icon string
		want bool
	}{
		{"valid wrench", "wrench", true},
		{"valid cpu", "cpu", true},
		{"invalid", "unicorn", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidIcon(tt.icon); got != tt.want {
				t.Errorf("ValidIcon(%q) = %v, want %v", tt.icon, got, tt.want)
			}
		})
	}
}

func TestRandomIcon(t *testing.T) {
	ic := RandomIcon()
	if ic.Name == "" || ic.Category == "" {
		t.Errorf("RandomIcon() returned empty: %+v", ic)
	}
	if !ValidIcon(ic.Name) {
		t.Errorf("RandomIcon() returned invalid icon: %s", ic.Name)
	}
}

func TestIconByName(t *testing.T) {
	ic, ok := IconByName("wrench")
	if !ok {
		t.Fatal("IconByName(wrench) not found")
	}
	if ic.Category == "" {
		t.Error("IconByName(wrench).Category is empty")
	}
	_, ok = IconByName("nonexistent")
	if ok {
		t.Error("IconByName(nonexistent) should return false")
	}
}

func TestIconCategories(t *testing.T) {
	cats := IconCategories()
	if len(cats) == 0 {
		t.Error("IconCategories() is empty")
	}
	for _, cat := range cats {
		if cat.Name == "" || len(cat.Icons) == 0 {
			t.Errorf("empty category: %+v", cat)
		}
	}
}

func TestAllIcons(t *testing.T) {
	icons := AllIcons()
	if len(icons) < 50 {
		t.Errorf("AllIcons() len = %d, want >= 50", len(icons))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/shared/palette/ -run TestValidIcon -v`
Expected: FAIL

- [ ] **Step 3: Implement icons.go**

```go
// internal/shared/palette/icons.go
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
```

Note: The exact icon names must match Phosphor Icons file names. Verify against the Phosphor repo when downloading SVGs. Adjust names as needed.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/shared/palette/ -run "TestValidIcon|TestRandomIcon|TestIconByName|TestIconCategories|TestAllIcons" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/shared/palette/icons.go internal/shared/palette/icons_test.go
git commit -m "feat(palette): add icon catalog with categories, validation, random selection"
```

---

## Task 3: Download & Embed Phosphor SVGs

**Files:**
- Create: `internal/shared/palette/phosphor/*.svg` (~60 files)
- Create: `internal/shared/palette/icons_embed.go`
- Create: `internal/shared/palette/icons_embed_test.go`

- [ ] **Step 1: Download Phosphor Icons SVGs**

Download the `regular` weight SVGs from `phosphor-icons/core` repo for each icon name in the catalog. Place in `internal/shared/palette/phosphor/`.

```bash
# Clone phosphor-icons/core (sparse checkout for just regular SVGs)
cd /tmp
git clone --depth 1 --filter=blob:none --sparse https://github.com/phosphor-icons/core.git
cd core
git sparse-checkout set assets/regular

# Copy needed icons to project
DEST=/Users/erxyi/Projekty/qlx/internal/shared/palette/phosphor
mkdir -p "$DEST"
for icon in wrench hammer screwdriver nut gear paint-brush ruler scissors \
  cpu circuit-board lightning battery-full monitor camera speaker-high wifi-high \
  t-shirt pants sneaker coat-hanger backpack \
  cooking-pot knife wine coffee leaf grain \
  flask test-tube drop warning thermometer fire \
  file-text folder clipboard-text pen notebook envelope calendar printer \
  house package archive-box lockers lamp bed armchair door \
  truck car airplane barcode map-pin globe \
  star heart tag magnifying-glass chat-circle flag shield key lock user users first-aid-kit; do
  src="assets/regular/${icon}.svg"
  if [ -f "$src" ]; then
    cp "$src" "$DEST/${icon}.svg"
  else
    echo "MISSING: $icon"
  fi
done

# Clean up
rm -rf /tmp/core
```

Verify all files present. Adjust icon names for any that don't match Phosphor's naming (e.g., Phosphor may use `t-shirt` vs `tshirt`). Update `icons.go` catalog to match actual filenames.

- [ ] **Step 2: Write failing embed test**

```go
// internal/shared/palette/icons_embed_test.go
package palette

import "testing"

func TestSVG(t *testing.T) {
	data, err := SVG("wrench")
	if err != nil {
		t.Fatalf("SVG(wrench) error: %v", err)
	}
	if len(data) == 0 {
		t.Error("SVG(wrench) returned empty data")
	}
	if string(data[:4]) != "<svg" && string(data[:5]) != "<?xml" {
		t.Errorf("SVG(wrench) does not start with <svg: %s", string(data[:20]))
	}
}

func TestSVGNotFound(t *testing.T) {
	_, err := SVG("nonexistent")
	if err == nil {
		t.Error("SVG(nonexistent) should return error")
	}
}

func TestAllIconsHaveSVG(t *testing.T) {
	for _, ic := range icons {
		data, err := SVG(ic.Name)
		if err != nil {
			t.Errorf("SVG(%s) error: %v", ic.Name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("SVG(%s) returned empty data", ic.Name)
		}
	}
}
```

- [ ] **Step 3: Implement icons_embed.go**

```go
// internal/shared/palette/icons_embed.go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/shared/palette/ -run "TestSVG|TestAllIconsHaveSVG" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/shared/palette/phosphor/ internal/shared/palette/icons_embed.go internal/shared/palette/icons_embed_test.go
git commit -m "feat(palette): embed curated Phosphor Icons SVGs"
```

---

## Task 4: Store — Model & Migration

**Files:**
- Modify: `internal/store/models.go`
- Modify: `internal/store/migrate.go`

- [ ] **Step 1: Add Color and Icon fields to models**

In `internal/store/models.go`, add `Color` and `Icon` fields to `Container`, `Item`, and `Tag`:

```go
// Container — add after TagIDs field:
Color string `json:"color"`
Icon  string `json:"icon"`

// Item — add after TagIDs field:
Color string `json:"color"`
Icon  string `json:"icon"`

// Tag — add after CreatedAt field:
Color string `json:"color"`
Icon  string `json:"icon"`
```

- [ ] **Step 2: Add migrateV1ToV2 to migrate.go**

Add to `migrations` slice and implement:

```go
var migrations = []Migration{
	migrateV0ToV1,
	migrateV1ToV2,
}

// migrateV1ToV2 adds color and icon fields to items, containers, and tags.
func migrateV1ToV2(data map[string]any) error {
	for _, collection := range []string{"items", "containers", "tags"} {
		entries, ok := data[collection].(map[string]any)
		if !ok {
			continue
		}
		for _, v := range entries {
			entry, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if _, exists := entry["color"]; !exists {
				entry["color"] = ""
			}
			if _, exists := entry["icon"]; !exists {
				entry["icon"] = ""
			}
		}
	}
	return nil
}
```

- [ ] **Step 3: Run existing tests to verify nothing breaks**

Run: `go test ./internal/store/ -v`
Expected: PASS (existing tests still work — new fields default to empty string)

**Note on partitioned stores:** `migrateV1ToV2` only runs for legacy monolithic `data.json` files. Partitioned stores (the current format) bypass the migration system — but this is safe because Go's `json.Unmarshal` leaves missing fields at their zero value (empty string), which matches the lazy-fill semantics. The migration exists for completeness and consistency with the V0→V1 pattern.

- [ ] **Step 4: Commit**

```bash
git add internal/store/models.go internal/store/migrate.go
git commit -m "feat(store): add color/icon fields to models, add V1→V2 migration"
```

---

## Task 5: Store — Update Create/Update Signatures

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/tags.go`
- Modify: `internal/store/store_test.go`

- [ ] **Step 1: Update store.go — CreateContainer signature**

Change `CreateContainer(parentID, name, description string)` to `CreateContainer(parentID, name, description, color, icon string)`:

```go
func (s *Store) CreateContainer(parentID, name, description, color, icon string) *Container {
	// ...existing code...
	c := &Container{
		ID:          uuid.New().String(),
		ParentID:    parentID,
		Name:        name,
		Description: description,
		Color:       color,
		Icon:        icon,
		CreatedAt:   time.Now(),
		TagIDs:      []string{},
	}
	// ...rest unchanged...
}
```

- [ ] **Step 2: Update store.go — UpdateContainer signature**

Change `UpdateContainer(id, name, description string)` to `UpdateContainer(id, name, description, color, icon string)`:

```go
func (s *Store) UpdateContainer(id, name, description, color, icon string) (*Container, error) {
	// ...existing lock/lookup...
	c.Name = name
	c.Description = description
	c.Color = color
	c.Icon = icon
	// ...rest unchanged...
}
```

- [ ] **Step 3: Update store.go — CreateItem and UpdateItem signatures**

Same pattern — add `color, icon string` parameters:

```go
func (s *Store) CreateItem(containerID, name, description string, quantity int, color, icon string) *Item
func (s *Store) UpdateItem(id, name, description string, quantity int, color, icon string) (*Item, error)
```

Set `Color: color` and `Icon: icon` in the struct literal / update.

- [ ] **Step 4: Update tags.go — CreateTag and UpdateTag signatures**

```go
func (s *Store) CreateTag(parentID, name, color, icon string) *Tag
func (s *Store) UpdateTag(id, name, color, icon string) (*Tag, error)
```

Set `Color: color` and `Icon: icon` in the struct literal / update.

- [ ] **Step 5: Update store_test.go — fix all callsites**

Add `"", ""` (empty color/icon) to all existing Create*/Update* calls in tests. Example:

```go
// Before:
s.CreateContainer("", "Box", "A box")
// After:
s.CreateContainer("", "Box", "A box", "", "")
```

Do this for every callsite in store_test.go.

- [ ] **Step 6: Run store tests**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/store/store.go internal/store/tags.go internal/store/store_test.go
git commit -m "feat(store): extend Create/Update signatures with color and icon params"
```

---

## Task 6: Service Layer — Interfaces & Implementation

**Files:**
- Modify: `internal/service/interfaces.go`
- Modify: `internal/service/inventory.go`
- Modify: `internal/service/tags.go`
- Modify: `internal/service/inventory_test.go`
- Modify: `internal/service/tags_test.go`

- [ ] **Step 1: Update interfaces.go**

Add `color, icon string` to all Create/Update methods in the three store interfaces:

```go
// ItemStore
CreateItem(containerID, name, desc string, qty int, color, icon string) *store.Item
UpdateItem(id, name, desc string, qty int, color, icon string) (*store.Item, error)

// ContainerStore
CreateContainer(parentID, name, desc string, color, icon string) *store.Container
UpdateContainer(id, name, desc string, color, icon string) (*store.Container, error)

// TagStore
CreateTag(parentID, name, color, icon string) *store.Tag
UpdateTag(id, name, color, icon string) (*store.Tag, error)
```

- [ ] **Step 2: Update inventory.go — add validation and pass-through**

In `CreateContainer`, `UpdateContainer`, `CreateItem`, `UpdateItem`:
- Add `color, icon string` params to the service method signatures
- Add validation:

```go
if color != "" && !palette.ValidColor(color) {
	return nil, fmt.Errorf("invalid color: %s", color)
}
if icon != "" && !palette.ValidIcon(icon) {
	return nil, fmt.Errorf("invalid icon: %s", icon)
}
```

- Pass `color, icon` to the store method calls.

Import `"github.com/erxyi/qlx/internal/shared/palette"`.

- [ ] **Step 3: Update tags.go — add validation and pass-through**

Same pattern for `CreateTag` and `UpdateTag`:
- Add `color, icon string` params
- Validate with palette
- Pass to store

- [ ] **Step 4: Update inventory_test.go — fix callsites and add validation tests**

Fix all existing calls to add `"", ""` for color/icon. Add new test cases:

```go
{
	name:    "create container with valid color and icon",
	// ... setup ...
	// call CreateContainer with "blue", "wrench"
	// assert container.Color == "blue" and container.Icon == "wrench"
},
{
	name:    "create container with invalid color",
	// ... setup ...
	// call CreateContainer with "neon", ""
	// assert error contains "invalid color"
},
{
	name:    "create item with invalid icon",
	// ... setup ...
	// call CreateItem with "", "unicorn"
	// assert error contains "invalid icon"
},
```

- [ ] **Step 5: Update tags_test.go — fix callsites and add validation tests**

Same pattern — fix existing calls, add color/icon validation test cases.

- [ ] **Step 6: Run all service tests**

Run: `go test ./internal/service/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/service/interfaces.go internal/service/inventory.go internal/service/tags.go internal/service/inventory_test.go internal/service/tags_test.go
git commit -m "feat(service): add color/icon validation to inventory and tag services"
```

---

## Task 7: UI Handlers — Extract & Pass Color/Icon

**Files:**
- Modify: `internal/ui/handlers.go`
- Modify: `internal/ui/handlers_tags.go`

- [ ] **Step 1: Update HandleContainerCreate**

Add form value extraction:
```go
color := r.FormValue("color")
icon := r.FormValue("icon")
```

Pass to `s.inventory.CreateContainer(parentID, name, description, color, icon)`.

- [ ] **Step 2: Update HandleContainerUpdate**

Same pattern — extract `color`, `icon` from form, pass to `s.inventory.UpdateContainer`.

- [ ] **Step 3: Update HandleItemCreate**

Extract `color`, `icon`, pass to `s.inventory.CreateItem`.

- [ ] **Step 4: Update HandleItemUpdate**

Extract `color`, `icon`, pass to `s.inventory.UpdateItem`.

- [ ] **Step 5: Update HandleTagCreate in handlers_tags.go**

Extract `color`, `icon` from form, pass to `s.tags.CreateTag(parentID, name, color, icon)`:
```go
color := r.FormValue("color")
icon := r.FormValue("icon")
tag, err := s.tags.CreateTag(parentID, name, color, icon)
```

- [ ] **Step 6: Update HandleTagUpdate in handlers_tags.go**

Extract `color`, `icon`, pass to `s.tags.UpdateTag(id, name, color, icon)`.

- [ ] **Step 7: Update HandleContainerEdit and HandleItemEdit view models**

Ensure the form data structs pass `Color` and `Icon` values to templates so edit forms can pre-select.

- [ ] **Step 8: Verify compile**

Run: `go build ./cmd/qlx/`
Expected: SUCCESS

- [ ] **Step 9: Commit**

```bash
git add internal/ui/handlers.go internal/ui/handlers_tags.go
git commit -m "feat(ui): extract color/icon from forms, pass to service layer"
```

---

## Task 8: API Handlers — Color/Icon in JSON

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/api/handlers_tags.go`

- [ ] **Step 1: Update API container create/update handlers**

In `HandleContainerCreate` and `HandleContainerUpdate`, extract `color` and `icon` from JSON body / form values. Pass to `s.inventory.CreateContainer` / `UpdateContainer`.

- [ ] **Step 2: Update API item create/update handlers**

Same pattern for `HandleItemCreate` and `HandleItemUpdate`.

- [ ] **Step 3: Update API tag create/update handlers**

In `handlers_tags.go`, update `HandleTagCreate` and `HandleTagUpdate` to extract and pass `color`, `icon`.

- [ ] **Step 4: Verify compile and run existing tests**

Run: `go build ./cmd/qlx/ && go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/server.go internal/api/handlers_tags.go
git commit -m "feat(api): support color/icon in create/update JSON endpoints"
```

---

## Task 9: CSS — Palette Tokens & Picker Styles

**Files:**
- Modify: `internal/embedded/static/css/shared/tokens.css`
- Create: `internal/embedded/static/css/shared/pickers.css`

- [ ] **Step 1: Add palette CSS custom properties to tokens.css**

Append to `:root` block:
```css
/* Palette colors */
--palette-red: #e94560;
--palette-orange: #f4845f;
--palette-amber: #f5a623;
--palette-yellow: #ffc93c;
--palette-green: #4ecca3;
--palette-teal: #2ec4b6;
--palette-blue: #4d9de0;
--palette-indigo: #7b6cf6;
--palette-purple: #b07cd8;
--palette-pink: #e84393;
```

- [ ] **Step 2: Create pickers.css**

```css
/* Color picker */
.color-picker-grid {
	display: flex;
	flex-wrap: wrap;
	gap: var(--space-xs);
	margin-bottom: var(--space-sm);
}

.color-swatch {
	width: 28px;
	height: 28px;
	border-radius: 50%;
	border: 2px solid transparent;
	cursor: pointer;
	transition: border-color 0.15s;
}

.color-swatch:hover {
	border-color: var(--color-text);
}

.color-swatch.selected {
	border-color: var(--color-text);
	box-shadow: 0 0 0 2px var(--color-bg), 0 0 0 4px var(--color-text);
}

/* Icon picker */
.icon-picker-categories {
	margin-bottom: var(--space-sm);
}

.icon-picker-category {
	margin-bottom: var(--space-xs);
}

.icon-picker-category-header {
	font-size: var(--font-size-sm);
	color: var(--color-text-muted);
	cursor: pointer;
	padding: var(--space-xs) 0;
	user-select: none;
}

.icon-picker-category-header::before {
	content: "▸ ";
}

.icon-picker-category.open .icon-picker-category-header::before {
	content: "▾ ";
}

.icon-picker-grid {
	display: none;
	flex-wrap: wrap;
	gap: var(--space-xs);
	padding: var(--space-xs) 0;
}

.icon-picker-category.open .icon-picker-grid {
	display: flex;
}

.icon-swatch {
	width: 36px;
	height: 36px;
	display: flex;
	align-items: center;
	justify-content: center;
	border-radius: var(--radius-sm);
	border: 2px solid transparent;
	cursor: pointer;
	color: var(--color-text);
	transition: border-color 0.15s;
}

.icon-swatch:hover {
	border-color: var(--color-text-muted);
}

.icon-swatch.selected {
	border-color: var(--color-accent);
	background: var(--color-bg-alt);
}

.icon-swatch svg {
	width: 20px;
	height: 20px;
}

/* Color dot in lists */
.color-dot {
	display: inline-block;
	width: 8px;
	height: 8px;
	border-radius: 50%;
	margin-right: var(--space-xs);
	flex-shrink: 0;
}

/* Entity icon in lists */
.entity-icon {
	display: inline-flex;
	align-items: center;
	margin-right: var(--space-xs);
	color: currentColor;
}

.entity-icon svg {
	width: 16px;
	height: 16px;
}

.entity-icon.large svg {
	width: 24px;
	height: 24px;
}
```

- [ ] **Step 3: Include pickers.css in the HTML layout**

Add `<link>` tag in the base layout template (or wherever CSS is included) for `pickers.css`.

- [ ] **Step 4: Commit**

```bash
git add internal/embedded/static/css/shared/tokens.css internal/embedded/static/css/shared/pickers.css
git commit -m "feat(css): add palette tokens and picker/color-dot styles"
```

---

## Task 10: Icon HTTP Handler & Template Function

**Files:**
- Modify: `internal/ui/server.go`
- Modify: `internal/app/server.go`

- [ ] **Step 1: Add `icon` template function to ui/server.go**

In the `loadTemplates()` function's `funcMap`, add an `icon` function that returns inline SVG as `template.HTML`:

```go
import "github.com/erxyi/qlx/internal/shared/palette"

// In funcMap:
"icon": func(name string) template.HTML {
	data, err := palette.SVG(name)
	if err != nil {
		return ""
	}
	return template.HTML(data)
},
"paletteHex": func(name string) string {
	c, ok := palette.ColorByName(name)
	if !ok {
		return ""
	}
	return c.Hex
},
```

- [ ] **Step 2: Add icon HTTP handler in app/server.go**

Register a route to serve SVGs:

```go
mux.HandleFunc("GET /static/icons/{name}", func(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	// Strip .svg extension if present
	name = strings.TrimSuffix(name, ".svg")
	data, err := palette.SVG(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(data)
})
```

- [ ] **Step 3: Verify compile and test manually**

Run: `go build ./cmd/qlx/`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/ui/server.go internal/app/server.go
git commit -m "feat: add icon template function and SVG HTTP handler"
```

---

## Task 11: Picker UI — Templates & JS

**Files:**
- Create: `internal/embedded/templates/components/color_picker.html`
- Create: `internal/embedded/templates/components/icon_picker.html`
- Create: `internal/embedded/static/js/pickers.js`

- [ ] **Step 1: Create color_picker.html**

```html
{{ define "fields/color-picker" }}
<div class="form-group">
    <label>Color</label>
    <div class="color-picker-grid" data-picker="color">
        {{ range .allColors }}
        <button type="button"
            class="color-swatch{{ if eq .Name $.selected }} selected{{ end }}"
            data-value="{{ .Name }}"
            style="background-color: {{ .Hex }}"
            title="{{ .Name }}">
        </button>
        {{ end }}
    </div>
    <input type="hidden" name="color" value="{{ .selected }}">
</div>
{{ end }}
```

- [ ] **Step 2: Create icon_picker.html**

```html
{{ define "fields/icon-picker" }}
<div class="form-group">
    <label>Icon</label>
    <div class="icon-picker-categories" data-picker="icon">
        {{ range .allCategories }}
        <div class="icon-picker-category{{ if eq .Name $.firstCategory }} open{{ end }}">
            <div class="icon-picker-category-header">{{ .Name }}</div>
            <div class="icon-picker-grid">
                {{ range .Icons }}
                <button type="button"
                    class="icon-swatch{{ if eq .Name $.selected }} selected{{ end }}"
                    data-value="{{ .Name }}"
                    title="{{ .Name }}">
                    {{ icon .Name }}
                </button>
                {{ end }}
            </div>
        </div>
        {{ end }}
    </div>
    <input type="hidden" name="icon" value="{{ .selected }}">
</div>
{{ end }}
```

- [ ] **Step 3: Create pickers.js**

```javascript
document.addEventListener("DOMContentLoaded", function () {
    // Color picker
    document.querySelectorAll("[data-picker='color']").forEach(function (grid) {
        var hidden = grid.parentElement.querySelector("input[name='color']");
        grid.addEventListener("click", function (e) {
            var swatch = e.target.closest(".color-swatch");
            if (!swatch) return;
            grid.querySelectorAll(".color-swatch").forEach(function (s) {
                s.classList.remove("selected");
            });
            swatch.classList.add("selected");
            hidden.value = swatch.getAttribute("data-value");
        });
    });

    // Icon picker
    document.querySelectorAll("[data-picker='icon']").forEach(function (container) {
        var hidden = container.parentElement.querySelector("input[name='icon']");

        // Category toggle
        container.querySelectorAll(".icon-picker-category-header").forEach(function (header) {
            header.addEventListener("click", function () {
                header.parentElement.classList.toggle("open");
            });
        });

        // Icon selection
        container.addEventListener("click", function (e) {
            var swatch = e.target.closest(".icon-swatch");
            if (!swatch) return;
            container.querySelectorAll(".icon-swatch").forEach(function (s) {
                s.classList.remove("selected");
            });
            swatch.classList.add("selected");
            hidden.value = swatch.getAttribute("data-value");
        });
    });
});
```

- [ ] **Step 4: Include pickers.js in the base layout**

Add `<script src="/static/js/pickers.js"></script>` to the layout template.

- [ ] **Step 5: Verify compile**

Run: `go build ./cmd/qlx/`
Expected: SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/embedded/templates/components/color_picker.html internal/embedded/templates/components/icon_picker.html internal/embedded/static/js/pickers.js
git commit -m "feat(ui): add color and icon picker components"
```

---

## Task 12: Forms — Integrate Pickers

**Files:**
- Modify: `internal/embedded/templates/pages/inventory/container_form.html`
- Modify: `internal/embedded/templates/pages/inventory/item_form.html`
- Modify: `internal/ui/server.go` (view model helpers)

- [ ] **Step 1: Add palette data to form view models**

In `ui/server.go`, extend `ContainerFormData` and `ItemFormData` (or use template functions) to provide color/icon data. Add helper functions to funcMap:

```go
"allColors": palette.AllColors,
"iconCategories": palette.IconCategories,
```

- [ ] **Step 2: Update container_form.html**

Add picker partials before the name-desc fields:

```html
{{ template "fields/color-picker" dict "allColors" (allColors) "selected" .Data.Container.Color }}
{{ template "fields/icon-picker" dict "allCategories" (iconCategories) "selected" .Data.Container.Icon "firstCategory" "tools" }}
```

For create mode (new container), the handler should pre-assign random color/icon values.

- [ ] **Step 3: Update item_form.html**

Same pattern — add picker partials.

- [ ] **Step 4: Update quick-add forms in containers.html**

The inline quick-add forms for containers and items need hidden color/icon inputs with random defaults (generated server-side).

- [ ] **Step 5: Add pickers to tag quick-add form in tags.html**

In `internal/embedded/templates/pages/tags/tags.html`, the quick-entry form currently has only a `name` input. Add hidden inputs for `color` and `icon` with random defaults (set server-side in the template data or via a compact inline picker). Since the quick-entry form is minimal, add a small color dot selector and hidden icon input with a random default. The full picker can be shown in a dedicated tag edit form if needed.

```html
<input type="hidden" name="color" value="{{ .Data.DefaultColor }}">
<input type="hidden" name="icon" value="{{ .Data.DefaultIcon }}">
```

Update `HandleTags` in `ui/handlers_tags.go` to include `DefaultColor` and `DefaultIcon` in the `TagTreeData`.

- [ ] **Step 6: Test manually — start server, create item/container, verify pickers work**

Run: `make run`
Navigate to create container form, verify color/icon pickers appear and submit correctly.

- [ ] **Step 7: Commit**

```bash
git add internal/embedded/templates/pages/inventory/container_form.html internal/embedded/templates/pages/inventory/item_form.html internal/ui/server.go
git commit -m "feat(ui): integrate color/icon pickers into create/edit forms"
```

---

## Task 13: List Views — Color Dots & Icons

**Files:**
- Modify: `internal/embedded/templates/partials/inventory/container_list_item.html`
- Modify: `internal/embedded/templates/partials/inventory/item_list_item.html`
- Modify: `internal/embedded/static/css/inventory/lists.css`

- [ ] **Step 1: Update container_list_item.html**

Replace the hardcoded `📦` emoji with a color dot + inline SVG icon:

```html
<span class="color-dot" {{ if .Color }}style="background-color: {{ paletteHex .Color }}"{{ end }}></span>
<span class="entity-icon">{{ if .Icon }}{{ icon .Icon }}{{ else }}{{ icon "package" }}{{ end }}</span>
```

- [ ] **Step 2: Update item_list_item.html**

Replace `📋` emoji similarly:

```html
<span class="color-dot" {{ if .Color }}style="background-color: {{ paletteHex .Color }}"{{ end }}></span>
<span class="entity-icon">{{ if .Icon }}{{ icon .Icon }}{{ else }}{{ icon "clipboard-text" }}{{ end }}</span>
```

- [ ] **Step 3: Update lists.css if needed**

Add flex alignment for the dot + icon combo in list items.

- [ ] **Step 4: Test manually — verify list views show dots and icons**

Run: `make run`
Create items/containers with different colors/icons, verify list appearance.

- [ ] **Step 5: Commit**

```bash
git add internal/embedded/templates/partials/inventory/container_list_item.html internal/embedded/templates/partials/inventory/item_list_item.html internal/embedded/static/css/inventory/lists.css
git commit -m "feat(ui): show color dots and icons in list views"
```

---

## Task 14: Detail Views — Color & Icon Display

**Files:**
- Modify: `internal/embedded/templates/pages/inventory/containers.html`
- Modify: `internal/embedded/templates/pages/inventory/item.html`

- [ ] **Step 1: Update containers.html detail view**

- Add icon (24px) next to container name in the header
- Add `border-left: 3px solid` with entity color on the container card
- Update breadcrumbs to show small icons (16px) next to container names

- [ ] **Step 2: Update item.html detail view**

Same pattern — icon next to name, color border on card.

- [ ] **Step 3: Test manually**

Run: `make run`
Navigate to detail views, verify icon and color display.

- [ ] **Step 4: Commit**

```bash
git add internal/embedded/templates/pages/inventory/containers.html internal/embedded/templates/pages/inventory/item.html
git commit -m "feat(ui): show color and icon in detail views and breadcrumbs"
```

---

## Task 15: Tag Chips — Color & Icon

**Files:**
- Modify: `internal/embedded/templates/partials/tags/tag_chips.html`
- Modify: `internal/embedded/static/css/tags/tag-chips.css`

- [ ] **Step 1: Update tag_chips.html**

Add icon and color-aware background to tag chips:

```html
<span class="tag-chip" {{ if .Color }}style="background-color: {{ paletteHex .Color }}22; border-color: {{ paletteHex .Color }}"{{ end }}>
    {{ if .Icon }}<span class="entity-icon">{{ icon .Icon }}</span>{{ end }}
    <span>{{ .Name }}</span>
    <!-- existing remove button -->
</span>
```

(The `22` suffix on hex gives ~13% opacity for the background.)

- [ ] **Step 2: Update tag-chips.css**

Add border support and icon spacing:

```css
.tag-chip {
    /* ...existing styles... */
    border: 1px solid transparent;
}

.tag-chip .entity-icon {
    margin-right: 0.2rem;
}

.tag-chip .entity-icon svg {
    width: 12px;
    height: 12px;
}
```

- [ ] **Step 3: Test manually**

Create tags with colors/icons, assign to items, verify chip appearance.

- [ ] **Step 4: Commit**

```bash
git add internal/embedded/templates/partials/tags/tag_chips.html internal/embedded/static/css/tags/tag-chips.css
git commit -m "feat(ui): color-aware tag chips with icons"
```

---

## Task 16: E2E Tests

**Files:**
- Create: `e2e/tests/icon-colors.spec.ts`

- [ ] **Step 1: Write E2E tests for color/icon picker**

```typescript
import { test, expect } from "../fixtures/app";

test.describe("Color and Icon System", () => {
    test("container create form shows color picker", async ({ page, app }) => {
        await page.goto(app.baseURL + "/ui");
        // Open the quick-add details to get to the full form
        await page.click("a[href*='/edit']"); // or navigate to new container form
        await page.waitForResponse((r) => r.url().includes("/ui/") && r.status() === 200);

        const colorGrid = page.locator("[data-picker='color']");
        await expect(colorGrid).toBeVisible();

        const swatches = colorGrid.locator(".color-swatch");
        await expect(swatches).toHaveCount(10);

        // Random default should be pre-selected
        const selected = colorGrid.locator(".color-swatch.selected");
        await expect(selected).toHaveCount(1);

        const hiddenInput = page.locator("input[name='color']");
        await expect(hiddenInput).toHaveAttribute("value", /.+/);
    });

    test("selecting a color updates hidden input", async ({ page, app }) => {
        // Navigate to container create form
        await page.goto(app.baseURL + "/ui/containers/new"); // adjust URL as needed
        await page.waitForResponse((r) => r.url().includes("/ui/") && r.status() === 200);

        const colorGrid = page.locator("[data-picker='color']");
        const targetSwatch = colorGrid.locator(".color-swatch[data-value='teal']");
        await targetSwatch.click();

        await expect(targetSwatch).toHaveClass(/selected/);
        const hiddenInput = page.locator("input[name='color']");
        await expect(hiddenInput).toHaveAttribute("value", "teal");
    });

    test("icon picker categories expand/collapse", async ({ page, app }) => {
        await page.goto(app.baseURL + "/ui/containers/new");
        await page.waitForResponse((r) => r.url().includes("/ui/") && r.status() === 200);

        const categories = page.locator(".icon-picker-category");
        // First category should be open
        await expect(categories.first()).toHaveClass(/open/);

        // Click second category header to open it
        const secondHeader = categories.nth(1).locator(".icon-picker-category-header");
        await secondHeader.click();
        await expect(categories.nth(1)).toHaveClass(/open/);
    });

    test("created container shows color dot and icon in list", async ({ page, app }) => {
        // Create container via API with known color/icon
        const resp = await page.request.post(app.baseURL + "/api/containers", {
            data: { name: "Test Box", description: "", color: "blue", icon: "wrench" },
        });
        expect(resp.status()).toBe(201);

        await page.goto(app.baseURL + "/ui");
        await page.waitForResponse((r) => r.url().includes("/ui") && r.status() === 200);

        // Verify color dot with blue background
        const listItem = page.locator("li").filter({ hasText: "Test Box" });
        const dot = listItem.locator(".color-dot");
        await expect(dot).toBeVisible();

        // Verify SVG icon is present (not emoji)
        const icon = listItem.locator(".entity-icon svg");
        await expect(icon).toBeVisible();
    });

    test("tag chips show color and icon", async ({ page, app }) => {
        // Create a container and tag via API
        const containerResp = await page.request.post(app.baseURL + "/api/containers", {
            data: { name: "Tag Test Container", description: "" },
        });
        const container = await containerResp.json();

        const itemResp = await page.request.post(app.baseURL + "/api/items", {
            data: { name: "Tag Test Item", container_id: container.id, description: "" },
        });
        const item = await itemResp.json();

        const tagResp = await page.request.post(app.baseURL + "/api/tags", {
            data: { name: "Urgent", color: "red", icon: "warning" },
        });
        const tag = await tagResp.json();

        // Assign tag to item
        await page.request.post(app.baseURL + `/api/items/${item.id}/tags/${tag.id}`);

        // Navigate to item detail
        await page.goto(app.baseURL + `/ui/items/${item.id}`);
        await page.waitForResponse((r) => r.url().includes("/ui/items/") && r.status() === 200);

        // Assert tag chip has colored background and icon
        const chip = page.locator(".tag-chip").filter({ hasText: "Urgent" });
        await expect(chip).toBeVisible();
        await expect(chip.locator(".entity-icon svg")).toBeVisible();
    });
});
```

- [ ] **Step 2: Run E2E tests**

Run: `make test-e2e`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add e2e/tests/icon-colors.spec.ts
git commit -m "test(e2e): add icon and color system E2E tests"
```

---

## Task 17: Lint & Final Verification

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: PASS

- [ ] **Step 2: Run linter**

Run: `make lint`
Expected: PASS (fix any issues)

- [ ] **Step 3: Run E2E tests**

Run: `make test-e2e`
Expected: PASS

- [ ] **Step 4: Manual smoke test**

Start server with `make run`. Verify:
- Create container → random color + icon assigned
- Edit container → picker pre-selects current values
- Create item → random color + icon assigned
- List views show dots + icons
- Detail views show icon + color border
- Tags with color/icon display correctly in chips
- API endpoints accept and return color/icon fields

- [ ] **Step 5: Final commit if any fixes needed**

```bash
git commit -m "fix: lint and test fixes for icon-color system"
```
