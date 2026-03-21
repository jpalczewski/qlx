# Label Designer — Design Specification

## Summary

A visual label template designer built with Fabric.js that lets users create reusable, parameterized label layouts with drag-and-drop placement of text, QR codes, barcodes, lines, and images. Templates are stored in the QLX store and can be used to print labels for items and containers.

## Problem

Current label printing uses 4 hardcoded Go templates (simple/standard/compact/detailed) with a single monospace font and fixed layouts. Users cannot customize labels, choose fonts, or create reusable parameterized templates.

## Architecture

**Approach C: Fabric.js + custom format.** Fabric.js provides the interactive designer UI in the browser. On save, the Fabric canvas state is converted to a simpler custom QLX JSON format for storage and server-side rendering.

```
Browser (Fabric.js)              Server (Go)
┌─────────────────┐              ┌──────────────────┐
│ Designer Canvas  │── JSON ────▶│ Store (template)  │
│ Preview Canvas   │── PNG ─────▶│ Encoder → Print   │
│                  │◀── JSON ────│ GET templates     │
└─────────────────┘              └──────────────────┘
```

### Three rendering paths

1. **Browser print (default):** Designer → fill params → Canvas PNG → POST to server → Encoder → Printer
2. **Server print (phase 2):** API call → load template JSON → Go renderer → Encoder → Printer
3. **Batch print (phase 2):** API call → foreach item in container → Go renderer → Encoder → Printer

### Why Fabric.js

- Built-in interactive text editing, object selection, transformers (resize/rotate), and serialization
- `toDataURL()` for PNG export with configurable resolution
- Vanilla JS, no framework dependency — fits existing HTMX + vanilla JS stack
- ~300KB, loaded only on designer pages

### Why custom QLX format (not raw Fabric JSON)

- Fabric JSON is verbose and tightly coupled to Fabric internals
- Custom format is simple enough to interpret in Go for server-side rendering
- Only 5 element types to support — conversion is straightforward

### Fabric.js version

Use **Fabric.js v6** (ES6 classes, tree-shakeable). Pin to `^6.x` in the embedded JS.

## QLX Template Format

```json
{
  "id": "tpl_abc123",
  "name": "Standard Inventory",
  "tags": ["inventory", "items"],
  "target": "universal",
  "width_mm": 50,
  "height_mm": 30,
  "elements": [
    {
      "type": "text",
      "x": 2, "y": 2, "width": 30,
      "text": "{{name}}",
      "font": "sans", "size": 16,
      "bold": true, "italic": false,
      "align": "left"
    },
    {
      "type": "qr",
      "x": 35, "y": 2, "size": 15,
      "content": "{{qr_url}}"
    },
    {
      "type": "barcode",
      "x": 2, "y": 20, "width": 46, "height": 8,
      "content": "{{id}}",
      "format": "code128"
    },
    {
      "type": "line",
      "x1": 2, "y1": 18, "x2": 48, "y2": 18,
      "thickness": 1
    },
    {
      "type": "img",
      "x": 40, "y": 20, "width": 8, "height": 8,
      "src": "asset:logo_abc",
      "fit": "contain"
    }
  ]
}
```

### Template target

- `"universal"` — dimensions in mm, scaled to printer DPI at print time (`px = mm × DPI / 25.4`). Element coordinates are stored in mm. In the Fabric.js editor, a preview DPI is used (default: 203) so the user sees approximate pixel rendering. At print time, coordinates are converted using the actual printer DPI.
- `"printer:B1"` / `"printer:QL-700"` — dimensions in printer pixels, pixel-perfect control. Element coordinates are stored in px. `WidthPx` and `HeightPx` fields used instead of mm.

### Coordinate system

- **Universal templates:** all element positions and sizes are in mm (float64)
- **Printer-specific templates:** all element positions and sizes are in px (int, relative to PrintWidthPx)
- The Fabric ↔ QLX converter handles the mapping between Fabric's pixel coordinates and the template's coordinate system

## Template Variables

| Variable | Description | Context |
|----------|-------------|---------|
| `{{name}}` | Item or container name | item, container |
| `{{description}}` | Item or container description | item, container |
| `{{location}}` | Container path ("Room → Shelf → Box") | item, container |
| `{{id}}` | Item or container ID | item, container |
| `{{qr_url}}` | Full URL to item/container view | item, container |
| `{{date}}` | Print date (YYYY-MM-DD) | context |
| `{{time}}` | Print time (HH:MM) | context |
| `{{printer}}` | Printer name | context |

## Element Types

### text
- `x`, `y` — position
- `width` — text wrap boundary
- `height` — optional max height; overflow is clipped
- `text` — content with `{{param}}` support or literal
- `font` — sans / serif / mono
- `size` — font size in pt
- `bold`, `italic` — boolean
- `align` — left / center / right
- Unknown `{{variables}}` are rendered as-is (literal text)

### qr
- `x`, `y` — position
- `size` — width = height (square)
- `content` — `{{qr_url}}` or literal text

### barcode
- `x`, `y` — position
- `width`, `height` — dimensions
- `content` — `{{id}}` or literal
- `format` — code128 (only supported format for now; extensible later via `boombuler/barcode`)

### line
- `x1`, `y1` — start point
- `x2`, `y2` — end point
- `thickness` — line width in px

Note: Fabric.js Line objects use `left`, `top` plus relative `x1, y1, x2, y2`. The QLX converter translates these to absolute coordinates (`x1 = left + fabric_x1`, etc.) on export, and reverses the transformation on import.

### img
- `x`, `y` — position
- `width`, `height` — dimensions
- `src` — `"asset:<id>"` referencing uploaded asset
- `fit` — contain / cover / stretch

## Store Entities

Templates and assets are added to the existing `storeData` struct as new maps, persisted in the same JSON file alongside containers, items, and printers.

```go
type Template struct {
    ID        string
    Name      string
    Tags      []string
    Target    string    // "universal" | "printer:B1"
    WidthMM   float64   // universal: canvas width in mm
    HeightMM  float64   // universal: canvas height in mm
    WidthPx   int       // printer-specific: canvas width in px
    HeightPx  int       // printer-specific: canvas height in px
    Elements  string    // JSON array of elements
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

Assets (uploaded images) are stored as separate files on disk in a `data/assets/` directory to avoid bloating the JSON store. The store only holds metadata:

```go
type Asset struct {
    ID        string
    Name      string
    MimeType  string
    CreatedAt time.Time
}
// Asset file stored at: data/assets/{id}.bin
```

### Store methods (new)

```go
// Templates
func (s *Store) AllTemplates() []Template
func (s *Store) GetTemplate(id string) *Template
func (s *Store) SaveTemplate(t Template)
func (s *Store) DeleteTemplate(id string)

// Assets
func (s *Store) AllAssets() []Asset
func (s *Store) GetAsset(id string) *Asset
func (s *Store) SaveAsset(a Asset, data []byte) error
func (s *Store) DeleteAsset(id string)
func (s *Store) AssetData(id string) ([]byte, error)
```
```

## UI Flow

### New pages

1. **`/ui/templates`** — list all templates, filter by tags, create/edit/delete
2. **`/ui/templates/new`** — designer canvas for new template
3. **`/ui/templates/{id}/edit`** — designer canvas for editing existing template

### Designer layout

- **Left:** toolbar (text, QR, barcode, line, img tools)
- **Center:** Fabric.js canvas with live preview (params filled with real item/container data)
- **Right:** properties panel (font, size, align, position, content for selected element)
- **Bottom:** save/cancel buttons

### Print from item view (`/ui/items/{id}`)

- Select printer (dropdown)
- Select template (dropdown, replaces hardcoded 4 templates)
- Live preview with item data
- Print button → Canvas renders PNG → POST to server

### Print from container view (`/ui/containers/{id}`)

- Single container label: same as item print flow
- Batch print: select template, checkbox "Include sub-containers", shows item count, "Print All (N)" button

### Navigation

- Add "Templates" link to navbar (between Containers and Printers)

## API Endpoints

### UI Endpoints (HTMX)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/ui/templates` | List templates page |
| GET | `/ui/templates/new` | Designer — new template |
| GET | `/ui/templates/{id}/edit` | Designer — edit template |
| POST | `/ui/actions/templates` | Create template |
| PUT | `/ui/actions/templates/{id}` | Update template |
| DELETE | `/ui/actions/templates/{id}` | Delete template |
| POST | `/ui/actions/templates/{id}/duplicate` | Duplicate template |
| POST | `/ui/actions/assets` | Upload image asset (multipart) |
| GET | `/ui/actions/assets/{id}` | Serve image asset |

### Print Endpoints (updated)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/ui/actions/items/{id}/print` | Print item label (browser PNG) |
| POST | `/ui/actions/containers/{id}/print` | Print container label |
| POST | `/ui/actions/containers/{id}/print-all` | Batch print all items |

### REST API (phase 2)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/templates` | List templates (JSON) |
| GET | `/api/templates/{id}` | Get template (JSON) |
| POST | `/api/print` | Server-side render + print |
| POST | `/api/print/batch` | Batch: container → all items |
| POST | `/api/print/render` | Render → return PNG (no print) |

## Rendering

### Browser-side (phase 1)

- Fabric.js canvas renders the template with filled parameters
- `canvas.toDataURL({format: 'png', multiplier: N})` produces high-res bitmap
- Floyd-Steinberg dithering applied in JS before export (1-bit monochrome)
- PNG sent to server via POST as base64 JSON: `{printer_id: "...", png: "data:image/png;base64,..."}`
- Server decodes PNG and passes `image.Image` to encoder

### PrinterManager changes

New method to accept pre-rendered images:

```go
func (m *PrinterManager) PrintImage(printerID string, img image.Image) error
```

This skips `label.Render()` and passes the image directly to the encoder session. The existing `Print()` method (with `LabelData` + template name) remains for backward compatibility and is used by the server-side renderer in phase 2.

### Server-side (phase 2)

- Go parses QLX JSON, draws elements using `image.Draw`
- Embedded fonts: sans, serif, mono (+ bold variants)
- QR via `skip2/go-qrcode`, barcode via `boombuler/barcode` (already in project)
- Floyd-Steinberg dithering in Go
- Used for API print and batch print (no browser needed)

### Unit conversion (universal templates)

```
canvas_px = mm × (DPI / 25.4)
```

Example: 50mm @ 203 DPI = 400px canvas width

## Dependencies

### JavaScript (browser, loaded on designer pages only)

- **Fabric.js** (~300KB) — canvas editor
- **qrcode-generator** (~4KB) — QR rendering on canvas
- **JsBarcode** (~15KB) — barcode rendering on canvas

### Go (existing)

- `skip2/go-qrcode` — already used
- `boombuler/barcode` — already used
- `golang.org/x/image/font` — already used (extend with embedded TTF fonts in phase 2)

## Phased Implementation

### Phase 1 — MVP

- Template CRUD in store + UI endpoints
- Fabric.js designer (text, qr, barcode, line, img)
- QLX JSON format with Fabric ↔ QLX converter
- Live preview with item/container data
- Browser-side PNG rendering + dithering + print
- `/ui/templates` page with tag filtering
- Asset upload for images
- Print from item view with template selection
- Print container labels
- Navigation update (Templates link)

### Phase 2 — Server & API

- Go server-side renderer (QLX JSON → image.Draw)
- Embedded fonts (sans, serif, mono + bold)
- Floyd-Steinberg dithering in Go
- REST API: `/api/print`, `/api/print/batch`, `/api/print/render`
- Batch print from container (flat + recursive)
- Universal templates (mm → px conversion)

### Phase 3 — Polish

- Template duplication
- Snap-to-grid in designer
- Undo/redo
- Keyboard shortcuts
- Checkbox item selection for batch print

## Validation

- Elements placed outside canvas bounds are allowed (clipped at render time)
- Unknown `{{variables}}` in text are rendered as literal text
- Templates with zero elements are valid (blank label)
- `img` elements referencing non-existent `asset:<id>` render as empty placeholder rectangle
- Template element count soft limit: 50 elements (UI warning, not enforced server-side)
- Asset upload size limit: 1MB per image

## Backward Compatibility

- Existing hardcoded templates (simple/standard/compact/detailed) remain available as fallback
- New template system is additive — no breaking changes to current print flow
- Existing `label.Render()` function stays unchanged; new system uses a separate rendering path
- Migration path: create QLX JSON equivalents of the 4 hardcoded templates as seed data
