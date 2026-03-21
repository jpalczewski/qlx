# Icon & Color System Design

**Issues:** #9 (Icons), #10 (Colors) + Tag enrichment
**Branch:** `notes-icon-colors`
**Scope:** Shared icon/color system for Items, Containers, and Tags. Notes (#11) deferred to separate branch.

---

## 1. Shared Palette Package

New package `internal/shared/palette/` — single source of truth for both backend and frontend.

### Colors (`colors.go`)

Curated palette of 10 flat colors, chosen for contrast on dark theme (`--color-bg: #1a1a2e`) and colorblind accessibility.

```go
type Color struct {
    Name string // e.g. "teal"
    Hex  string // e.g. "#2ec4b6"
}

var Colors = [...]Color{ ... } // 10 colors

func ValidColor(name string) bool
func RandomColor() Color
```

**Palette:**

| Name | Hex |
|------|-----|
| `red` | `#e94560` |
| `orange` | `#f4845f` |
| `amber` | `#f5a623` |
| `yellow` | `#ffc93c` |
| `green` | `#4ecca3` |
| `teal` | `#2ec4b6` |
| `blue` | `#4d9de0` |
| `indigo` | `#7b6cf6` |
| `purple` | `#b07cd8` |
| `pink` | `#e84393` |

### Icons (`icons.go`)

Phosphor Icons (MIT license), curated subset of ~50-80 SVGs, `regular` weight. Expandable later.

```go
type Icon struct {
    Name     string // e.g. "wrench"
    Category string // e.g. "tools"
}

var Icons = [...]Icon{ ... } // curated 50-80

func ValidIcon(name string) bool
func RandomIcon() Icon
```

**Categories:**
- Tools & Hardware (wrench, hammer, screwdriver, nut...)
- Electronics (cpu, circuit-board, lightning, battery...)
- Clothing & Textiles (shirt, pants, scissors...)
- Food & Kitchen (bowl, knife, flame, thermometer...)
- Chemicals & Lab (flask, test-tube, drop, warning...)
- Office & Documents (file, folder, clipboard, pen...)
- Home & Storage (house, box, archive, shelf...)
- Transport (truck, package, barcode...)

### Testability

`RandomColor()` and `RandomIcon()` use `math/rand` internally. For deterministic tests, the service layer receives `color` and `icon` as parameters (see Section 2), so tests pass explicit values and never depend on random selection. The random assignment is done by UI/API handlers before calling the service.

### Icon Embedding

SVG files stored in `internal/shared/palette/phosphor/` (co-located with the palette package so `go:embed` works with relative paths).

```go
// in internal/shared/palette/icons_embed.go
//go:embed phosphor/*.svg
var IconFS embed.FS

func SVG(name string) ([]byte, error)
```

Served two ways:
1. **HTTP handler** — `GET /static/icons/{name}.svg` — registered in `app/server.go`, reads from `palette.IconFS`. Used for direct linking and label rendering.
2. **Template function** — `{{icon "wrench"}}` — inline `<svg>` in HTML (preferred for lists, inherits `currentColor`)

---

## 2. Model Changes

Add `Color` and `Icon` string fields to three models in `store/models.go`:

```go
type Container struct {
    // ...existing fields...
    Color string `json:"color"` // palette color name
    Icon  string `json:"icon"`  // icon name
}

type Item struct {
    // ...existing fields...
    Color string `json:"color"`
    Icon  string `json:"icon"`
}

type Tag struct {
    // ...existing fields...
    Color string `json:"color"`
    Icon  string `json:"icon"`
}
```

### Data Migration: V1 → V2

Add `migrateV1ToV2` to `store/migrate.go`. The migration adds empty `color` and `icon` fields to all existing items, containers, and tags. This is consistent with the V0→V1 pattern (which added `quantity`, `tag_ids`). A backup is created before migration as per existing behavior.

```go
func migrateV1ToV2(data map[string]any) error {
    // Add color="" and icon="" to all items, containers, tags where missing
}
```

### Lazy Fill (rendering)

Empty `color`/`icon` after migration render with defaults in UI:
- Gray dot (`--color-text-muted`) + type-specific fallback icon (`box` for containers, `clipboard` for items, `tag` for tags)
- Clean distinction between "not set" (empty string) and "user chose X"

### Store Interface Changes

Extend signatures to accept `color` and `icon` parameters:

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

### Service Layer

All three entity types follow the same pattern:

- **Create**: service validates `palette.ValidColor()` + `palette.ValidIcon()`, passes to store
- **Update**: same validation, empty string allowed (means "clear to default")
- **Tag service**: extend existing `TagService` in `internal/service/tags.go` with color/icon validation (same pattern as `InventoryService`)

### Callers to Update

All callers of the changed interfaces must be updated to pass `color` and `icon`:
- `internal/ui/handlers.go` — UI create/edit handlers (extract `r.FormValue("color")`, `r.FormValue("icon")`)
- `internal/api/handlers_*.go` — API create/edit handlers (extract from JSON body)
- `internal/ui/handlers_bulk.go` — bulk operations (pass through existing values)
- `internal/service/inventory.go` — service methods
- `internal/service/tags.go` — tag service methods
- `internal/store/store.go` — store implementation
- `internal/store/store_test.go` — all test callsites
- `internal/service/inventory_test.go` — inventory service tests
- `internal/service/tags_test.go` — tag service tests

UI/API handlers call `palette.RandomColor().Name` and `palette.RandomIcon().Name` for create forms (passed as defaults in the form / generated on submit).

---

## 3. CSS Integration

### Tokens (`tokens.css`)

Add `--palette-*` CSS custom properties:
```css
--palette-red: #e94560;
--palette-orange: #f4845f;
/* ...etc for all 10 colors */
```

### New CSS Files

- `static/css/shared/pickers.css` — color/icon picker grid styling

---

## 4. Picker UI

Reusable template partials + vanilla JS. Embedded in create/edit forms for items, containers, and tags.

### Color Picker

- Grid of 10 color circles
- Click to select (outline ring on selected)
- Value stored in `<input type="hidden" name="color">`
- Pre-selected: random color (create) or current value (edit)

### Icon Picker

- Grid of icons grouped by category tabs
- Icons rendered as inline SVG
- Click to select, value to `<input type="hidden" name="icon">`
- Default category expanded, rest collapsed
- Pre-selected: random icon (create) or current value (edit)

### Files

- `templates/components/color_picker.html` — defines `{{define "fields/color-picker"}}` (matches existing `fields/*` naming from `form_fields.html`)
- `templates/components/icon_picker.html` — defines `{{define "fields/icon-picker"}}`
- `static/js/pickers.js` — vanilla JS (click → select → hidden input)

### Behavior

- Purely client-side interaction (no HTMX requests for picker)
- Form submits normally with hidden input values

### Integration

```html
{{template "fields/color-picker" .Color}}
{{template "fields/icon-picker" .Icon}}
```

---

## 5. Visualization

### List Views (container_list_item.html, item_list_item.html)

- Replace hardcoded emoji (box, clipboard) with inline SVG of chosen icon
- Color dot (8px circle) before icon, in entity's color
- Entities without color/icon: gray dot (`--color-text-muted`) + default icon per type (`box` for containers, `clipboard` for items)

### Tag Chips (tag-chips.css)

- Small icon (12-14px) before tag name
- Tag color as chip background (opacity ~0.15) + color as border
- Tags without color: current style (`--color-bg-alt`)

### Detail Views (item.html, containers.html)

- Icon next to name (~24px)
- Color `border-left: 3px solid` on detail card
- Breadcrumbs: small icon (16px) next to container name

### Forms

- Picker section above name/description fields

---

## 6. Out of Scope

- **Notes (#11)** — separate branch
- **Label rendering** — icon rasterization for thermal printer is a followup
- **Full icon set** — start curated (~50-80), expand later
- **Emoji fallback** — potential future enhancement
