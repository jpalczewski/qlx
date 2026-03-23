# Phase 1a — Template Batch Design Spec

## Overview

Phase 1a bundles five tightly related changes to the label printing system: a multi-font registry, container label printing (#56), ad-hoc label printing (#62), inline icon rendering, and extended built-in schemas with tags/children support. Template auto-discovery (#44) is already done.

## 1. Font Registry

### Storage

Fonts are embedded in the binary via `go:embed`. Directory: `internal/print/label/fonts/`.

Five entries in the registry:

| Name | Type | Unicode/PL | File | Notes |
|------|------|-----------|------|-------|
| `spleen` | monospace bitmap | Yes | `spleen-8x16.otf`, `spleen-12x24.otf` | Existing. Two sizes, threshold at 20px |
| `noto-sans` | proportional | Yes | `noto-sans-regular.ttf` | ~400KB |
| `terminus` | monospace bitmap | Yes | `terminus-regular.otf` | ~200KB |
| `go-mono` | monospace | Yes | `go-mono-regular.ttf` | ~400KB, BSD license |
| `basic` | built-in bitmap | ASCII-only | none (basicfont.Face7x13) | Transliterates Polish via `transliteratePL()` |

### Implementation

New file `font_registry.go` replaces current `font.go`:

```go
var fontCatalog = map[string]fontEntry{
    "spleen":    {path: "fonts/spleen-12x24.otf", smallPath: "fonts/spleen-8x16.otf", threshold: 20},
    "noto-sans": {path: "fonts/noto-sans-regular.ttf"},
    "terminus":  {path: "fonts/terminus-regular.otf"},
    "go-mono":   {path: "fonts/go-mono-regular.ttf"},
    "basic":     {builtin: true},
}
```

`LoadFace(name string, sizePx float64) (font.Face, error)` — lazy-loaded with cache keyed by `(name, sizePx)`. Thread-safe via `sync.Map`.

### Schema integration

`font_family` field at two levels:

1. **Schema level** — default for all elements. If absent, defaults to `"spleen"` (backward compatible).
2. **Element level** — overrides schema default.

```json
{
  "name": "detailed",
  "font_family": "spleen",
  "elements": [
    {"slot": "title", "font_size": 24, "font_family": "noto-sans"},
    {"slot": "description", "font_size": 13}
  ]
}
```

Element `description` inherits `"spleen"` from schema. Element `title` uses `"noto-sans"`.

## 2. Icon Rasterization

### Mechanism

Phosphor SVG icons from `palette.IconFS` rasterized to bitmap at runtime.

```go
func rasterizeIcon(name string, sizePx int) (image.Image, error)
```

Uses `srwiley/oksvg` + `srwiley/rasterx` (pure Go, no CGO). Cache via `sync.Map` keyed by `(name, sizePx)`.

### Usage in renderer

- **Title slot** — object's icon rendered inline, left of text. Size matches `font_size`. Text shifts right by `iconSize + gap`.
- **Children slot** — each child's icon rendered at children font_size.
- **Tags slot** — each tag's icon rendered at tags font_size.

Icons are optional — if the object/tag has no icon, text renders without gap.

## 3. LabelData Extension

```go
type LabelData struct {
    Name        string   // item/container name → "title" slot
    Description string   // description → "description" slot
    Location    string   // parent path "Room → Shelf" → "location" slot
    QRContent   string   // URL for QR code
    BarcodeID   string   // ID for barcode
    Icon        string   // Phosphor icon name for title
    Tags        []Tag    // assigned tags with names, icons, paths
    Children    []Child  // sub-containers + items (container only)
}

type Tag struct {
    Name string   // tag display name
    Icon string   // Phosphor icon name
    Path []string // ancestor names, root-first (e.g. ["elektronika", "arduino"])
}

type Child struct {
    Name string // child name
    Icon string // Phosphor icon name
}
```

## 4. Schema Element Extension

```go
type Element struct {
    Slot       string  `json:"slot"`        // "title","description","location","tags","children","qr","barcode"
    FontSize   float64 `json:"font_size"`   // default 13
    FontFamily string  `json:"font_family"` // override per element
    Align      string  `json:"align"`       // "left","center","right"
    Wrap       bool    `json:"wrap"`
    Color      string  `json:"color"`       // hex, default "#000000"
    Size       int     `json:"size"`        // QR size px
    Height     int     `json:"height"`      // barcode height px
    ShowPath   string  `json:"show_path"`   // tags only: "auto"|"true"|"false", default "auto"
    ShowIcons  *bool   `json:"show_icons"`  // render inline icons, default true for title/children/tags
}
```

### Tags `show_path` behavior

- `"true"` — always render full path: `"elektronika > arduino"`
- `"false"` — leaf name only: `"arduino"`
- `"auto"` (default) — full path, falls back to leaf-only if text overflows available width

## 5. Built-in Schemas

| Schema | Slots | Notes |
|--------|-------|-------|
| `micro` | title, description, location, tags | `basic` font (ASCII, transliteration) |
| `simple` | title, description, location, tags | |
| `compact` | title, description, tags | |
| `standard` | title, location, tags, QR | |
| `detailed` | title, description, location, tags, children, QR, barcode | |
| `contents` | title, children | **New.** Maximizes space for children list |

All existing schemas gain `tags` slot. `detailed` gains `children`. `contents` is new.

Schemas that don't include a slot simply ignore the corresponding data — backward compatible.

## 6. Container Label Printing (#56)

### Endpoint

```
POST /containers/{id}/print
Body: {
    "printer_id": "uuid",
    "templates": ["detailed", "contents"],
    "print_date": true,
    "show_children": true
}
```

`templates` is an array — container can print multiple labels sequentially in one request. Items keep single `template` string.

### LabelData mapping

| Field | Source |
|-------|--------|
| `Name` | container.Name |
| `Description` | container.Description |
| `Location` | parent container path joined with " → " |
| `QRContent` | `/ui/containers/{id}` |
| `BarcodeID` | container.ID |
| `Icon` | container.Icon |
| `Tags` | container's tags with paths |
| `Children` | sub-containers + items (if `show_children` true) |

### UI

Print section in container detail view:
- Select: printer
- Multi-select: templates (checkboxes, can pick 1+)
- Checkbox: "Dodaj datę wydruku"
- Checkbox: "Pokaż zawartość" (populates children)
- Button: Print

## 7. Ad-hoc Label Printing (#62)

### Endpoint

```
POST /adhoc/print
Body: {
    "text": "Remember to check shelf 3",
    "printer_id": "uuid",
    "template": "micro",
    "print_date": true
}
```

### LabelData mapping

| Field | Source |
|-------|--------|
| `Name` | user text |
| `Description` | empty |
| `Location` | empty |
| `QRContent` | empty |
| `BarcodeID` | empty |
| `Icon` | empty |
| `Tags` | empty |
| `Children` | empty |

### UI

New "Quick Print" page accessible from main navigation:
- Textarea: text to print
- Select: printer
- Select: template
- Checkbox: "Dodaj datę wydruku"
- Button: Print

## 8. Print Date Metadata

### Mechanism

New field `PrintDate bool` in `PrintOpts`. Not part of schema — handled by renderer unconditionally.

When `PrintDate` is true:
- Renderer appends a metadata line at the very bottom of the label (below barcode if present)
- Font: `basic` (7×13, ASCII-only)
- Text: `"Wydrukowano: 2026-03-22 14:30"` (transliterated via `transliteratePL()`)
- Small padding above the line

This is independent of schema — any schema can have the date appended.

## 9. Print Workflow Changes

### Item print (existing, extended)

```
POST /items/{id}/print
Body: {"printer_id": "...", "template": "simple", "print_date": false}
```

Same as before, but LabelData now includes `Icon` and `Tags`. `print_date` is new optional field.

### Container print (new)

```
POST /containers/{id}/print
Body: {"printer_id": "...", "templates": ["detailed"], "print_date": true, "show_children": true}
```

Handler iterates `templates` array, calls `PrinterManager.Print()` for each sequentially.

### Ad-hoc print (new)

```
POST /adhoc/print
Body: {"text": "...", "printer_id": "...", "template": "micro", "print_date": false}
```

### Designer templates

Designer templates (client-side Fabric.js) continue to work as before — `{render: "client"}` response. The new LabelData fields (`Icon`, `Tags`, `Children`) are passed as `item_data` for client-side rendering if the designer supports them. Out of scope for Phase 1a — designer template support for icons/tags/children is a future enhancement.

## 10. Dependencies

- `srwiley/oksvg` — SVG path parsing (pure Go)
- `srwiley/rasterx` — SVG rasterization (pure Go)
- Font files: Noto Sans Regular, Terminus Regular, Go Mono Regular (embedded, open-source licenses)

## 11. Files Affected

### New files
- `internal/print/label/font_registry.go` — font catalog + loading
- `internal/print/label/icon.go` — SVG rasterization + cache
- `internal/print/label/schemas/contents.json` — new schema
- `internal/print/label/fonts/noto-sans-regular.ttf`
- `internal/print/label/fonts/go-mono-regular.ttf`
- `internal/print/label/fonts/terminus-regular.otf`
- `internal/handler/adhoc.go` — ad-hoc print handler
- `internal/embedded/templates/pages/labels/quick_print.html` — ad-hoc UI

### Modified files
- `internal/print/label/font.go` — replaced by font_registry.go
- `internal/print/label/templates.go` — LabelData extended with Icon, Tags, Children
- `internal/print/label/schema_renderer.go` — render tags, children, icons inline, print date
- `internal/print/label/renderer.go` — pass new fields through
- `internal/print/label/schemas/*.json` — all schemas updated with new slots
- `internal/handler/print.go` — container print endpoint, print_date option
- `internal/handler/containers.go` — print section data in container detail
- `internal/embedded/templates/pages/inventory/containers.html` — print section UI
- `internal/embedded/templates/pages/inventory/item.html` — print_date checkbox
- `internal/app/server.go` — register new routes
- `go.mod` — add oksvg, rasterx

## 12. Out of Scope

- Font management UI (future — JSON config only for now)
- Designer template support for icons/tags/children
- Print queue (#57)
- Label preview before printing (#26)
- Runtime font loading from disk
