# Print Options & Renderer Pipeline — Design Spec

**Date:** 2026-03-28
**Issues:** #75 (partial), #57 (prerequisite)
**Scope:** Expose print options (density, copies, cut control, high-res) in API and UI; make renderer media-aware for die-cut labels.

## Context

The print pipeline hardcodes `Density: modelDefault`, `AutoCut: true`, `Quantity: 1`. Users cannot control print darkness, copy count, or cutting behavior. The renderer produces dynamically-sized images without knowledge of the physical label dimensions, causing undersized output on die-cut labels.

An agent-driven flow is planned — the API must be first-class, with UI as one of its consumers. Print options must be serializable for the upcoming print queue (Spec 2).

## 1. Extended PrintOpts

### Migration from current fields

Current `PrintOpts` has `AutoCut bool`, `Quantity int`, `Density int`. These are replaced:

- `AutoCut: true` + `Quantity: 1` → `CutEvery: 1`, `Copies: 1`
- `AutoCut: false` → `CutEvery: 0`
- `Quantity` → `Copies` (rename)

All existing call sites in `manager.go` and tests are updated in this change.

### New struct

```go
type PrintOpts struct {
    Density  int  `json:"density"`   // 0 = model default; Niimbot 1-5, Brother ignored
    Copies   int  `json:"copies"`    // 0/1 = single; >1 = multi-copy
    CutEvery int  `json:"cut_every"` // 0 = no cut; 1 = every copy; N = every N copies
    HighRes  bool `json:"high_res"`  // Brother: 600 DPI vertical (row duplication); others: ignored
}
```

### Validation (in PrinterManager, before encoder)

- `Density`: clamp to `ModelInfo.DensityRange[min, max]`; 0 maps to `DensityDefault`
- `Copies`: min 1, max 100
- `CutEvery`: 0 to Copies; values > Copies clamped to Copies

### Serialization

`PrintOpts` is a plain struct with JSON tags — directly embeddable in print queue jobs (Spec 2).

## 2. API

### Print endpoints (extended request body)

```
POST /api/items/{id}/print
POST /api/containers/{id}/print
POST /api/notes/{id}/print
POST /api/print-image
```

```json
{
  "printer_id": "uuid",
  "template": "standard",
  "print_date": true,
  "density": 3,
  "copies": 5,
  "cut_every": 1,
  "high_res": true
}
```

All new fields optional — omitted = model defaults. Applies to both `Print()` and `PrintImage()` paths.

### Capabilities endpoint (new)

```
GET /api/printers/{id}/capabilities
```

Example response for Niimbot B1:
```json
{
  "density": { "min": 1, "max": 5, "default": 3 },
  "copies": { "max": 100 },
  "cut_every": { "supported": false },
  "high_res": { "supported": false },
  "media": {
    "width_mm": 48,
    "height_mm": 25,
    "type": "die-cut"
  }
}
```

Example response for Brother QL-700 (continuous):
```json
{
  "density": { "min": 1, "max": 1, "default": 1 },
  "copies": { "max": 100 },
  "cut_every": { "supported": true },
  "high_res": { "supported": true },
  "media": {
    "width_mm": 62,
    "height_mm": 0,
    "type": "continuous"
  }
}
```

Source: `ModelInfo` for static capabilities, session status for media info. Agent uses this endpoint to discover negotiable options.

## 3. MediaInfo and Renderer

### MediaInfo struct

Lives in `label` package (consumed by `Render()`, constructed by manager which already imports `label`).

```go
// label package
type MediaInfo struct {
    WidthPx  int // printhead width in pixels (384, 720)
    HeightPx int // 0 = continuous (dynamic height); >0 = die-cut label height in px
    DPI      int // 203, 300
}
```

Sources:
- **Niimbot B1**: RFID info → `LabelHeightMm` → `mm * DPI / 25.4`
- **Brother QL-700**: status response → `MediaWidth` + `MediaLength` (mm) → px. Die-cut: length > 0; continuous: length = 0.

### Media info refresh policy

Media info is read once at session startup (via `RfidInfo()` / status request) and cached in `PrinterStatus`. It is **not re-read before each print**. If the user swaps label rolls, they must reconnect the printer (disconnect + reconnect in UI) to refresh media info. The capabilities endpoint always returns the cached value.

### Render() signature change

```go
// Before:
func Render(data LabelData, template string, widthPx, dpi int, opts RenderOpts) (image.Image, error)

// After:
func Render(data LabelData, template string, media MediaInfo, opts RenderOpts) (image.Image, error)
```

### Rendering behavior

- **Continuous tape** (`HeightPx == 0`): dynamic height from content, no change from current behavior.
- **Die-cut label** (`HeightPx > 0`): canvas = `WidthPx x HeightPx`. Content rendered with fit-to-area using the following overflow strategy:

**Die-cut overflow priority (lowest priority truncated first):**
1. Tags — removed entirely if space insufficient
2. Children list — removed entirely
3. Description — truncated with ellipsis
4. Location — truncated with ellipsis
5. Title + QR/Barcode — always rendered; minimum font size 8px

If all optional elements are removed and title still overflows at 8px, title is truncated with ellipsis. The renderer never produces an image taller than `HeightPx`.

### Preview endpoints

Preview endpoints (`GET /items/{id}/preview`, etc.) construct `MediaInfo` as follows:
- If `?printer_id=` query param present: use that printer's cached media info
- Otherwise: `MediaInfo{WidthPx: widthFromQuery (default 384), HeightPx: 0, DPI: 203}` — continuous behavior, backward compatible with current previews

### Manager builds MediaInfo from session

```go
status := session.Status()
media := label.MediaInfo{
    WidthPx:  modelInfo.PrintWidthPx,
    HeightPx: pxFromMm(status.LabelHeightMm, modelInfo.DPI),
    DPI:      modelInfo.DPI,
}
```

Fallback: if session has no media info → `HeightPx = 0` (continuous behavior).

## 4. Updated Manager Signatures

```go
// Before:
func (m *PrinterManager) Print(printerID string, data label.LabelData, templateName string, opts label.RenderOpts) error
func (m *PrinterManager) PrintImage(printerID string, img image.Image) error

// After:
func (m *PrinterManager) Print(printerID string, data label.LabelData, templateName string, renderOpts label.RenderOpts, printOpts encoder.PrintOpts) error
func (m *PrinterManager) PrintImage(printerID string, img image.Image, printOpts encoder.PrintOpts) error
```

Both paths accept `PrintOpts`. Handler parses options from request body and passes through. `PrintImage` applies the same options (density, copies, cut_every, high_res) as the schema-based path.

### resolveForPrint() — deduplicate Print/PrintImage setup

`Print()` and `PrintImage()` share a duplicated block: lookup printer config → find encoder → find model → get session → validate print opts → build MediaInfo. Extract to:

```go
type printContext struct {
    cfg       *store.PrinterConfig
    enc       encoder.Encoder
    model     encoder.ModelInfo
    session   *PrinterSession
    media     label.MediaInfo
    printOpts encoder.PrintOpts // validated & defaults applied
}

func (m *PrinterManager) resolveForPrint(printerID string, opts encoder.PrintOpts) (*printContext, error)
```

Both `Print()` and `PrintImage()` become ~10 lines each:

```go
func (m *PrinterManager) Print(printerID string, data label.LabelData, tpl string, renderOpts label.RenderOpts, printOpts encoder.PrintOpts) error {
    ctx, err := m.resolveForPrint(printerID, printOpts)
    if err != nil { return err }
    img, err := label.Render(data, tpl, ctx.media, renderOpts)
    if err != nil { return err }
    img = applyCalibrationOffset(img, ctx.cfg, ctx.media.WidthPx)
    return ctx.session.Print(img, ctx.cfg.Model, ctx.printOpts)
}
```

This is a targeted dedup from issue #75 — the full manager decomposition remains out of scope.

## 5. Encoder Changes

### Brother QL-700

**Multi-copy:**
- Initialize once (clear buffer, ESC @, status read)
- Loop `Copies` times:
  - Send `ESC i z` with page number (0-indexed, incrementing per copy)
  - Set auto-cut / expanded mode / margin (once before loop, not repeated)
  - Send raster rows
  - Pages 1..N-1: `0x0C` (print without feed)
  - Last page: `0x1A` (print with feed)

This avoids re-sending the 200-byte clear + init sequence per copy. Only the media info command, raster data, and print command are repeated.

**CutEvery:**
- `ESC i A {N}` — unfreeze from hardcoded 1 to `opts.CutEvery`
- `CutEvery == 0` → `ESC i M 0x00` (auto-cut OFF), expanded mode bit 3 = 0 (no cut-at-end)
- `CutEvery > 0` → `ESC i M 0x40` (auto-cut ON), `ESC i A {CutEvery}`

**HighRes:**
- Expanded mode bit 6 = 1 → 600 DPI in feed direction
- Implementation: encoder duplicates each raster row (no renderer changes needed)
- Upgrade path: renderer produces taller image at 600 DPI vertical if quality insufficient

**Dynamic margin:**
- Read media type from status response byte 11
- Die-cut: margin = 0
- Continuous: margin = 35

### Niimbot B1

**Copies:**
- Update `totalPages` in `PRINT_START (0x01)` to match `opts.Copies`
- First attempt: set `copies` field in `SET_PAGE_SIZE (0x13)` to `opts.Copies`
- Fallback (if native copies doesn't work): repeat `PAGE_START → rows → PAGE_END` cycle per copy, with `totalPages` set correctly in `PRINT_START`
- `PRINT_END (0xF3)` poll after last copy

**Density:**
- Unfreeze from `modelInfo.DensityDefault` to `opts.Density` (already validated by manager)

**CutEvery / HighRes:**
- Ignored — Niimbot has no cutter or HD mode

## 6. UI

### Print modal controls

Added below existing printer/template selectors:

- **Copies**: number input, default 1, min 1, max 100. Always visible.
- **Density**: range slider with numeric display, min/max/default from capabilities. Hidden if `DensityRange[0] == DensityRange[1]` (Brother).
- **Cut every**: number input, default 1, min 0, max = copies. Label: "Cut every N copies (0 = no cut)". Hidden if encoder doesn't support cutting.
- **High res**: checkbox "High resolution". Hidden if encoder doesn't support it.

### Dynamic show/hide

Changing printer in dropdown → HTMX `hx-get` fetches capabilities → server returns HTML fragment with appropriate controls shown/hidden. Server-driven UI, consistent with project patterns.

## 7. Files Affected

### Interfaces and structs
- `internal/print/encoder/encoder.go` — `PrintOpts` replaces `AutoCut`/`Quantity` with `CutEvery`/`Copies`/`HighRes`
- `internal/print/label/renderer.go` — `Render()` signature change to `MediaInfo`; new `MediaInfo` struct
- `internal/print/label/schema_renderer.go` — fit-to-label logic for die-cut, overflow priority

### Encoders
- `internal/print/encoder/brother/brother.go` — multi-copy loop, cut_every, high_res row duplication, dynamic margin
- `internal/print/encoder/niimbot/niimbot.go` — copies (native + fallback), totalPages update, density unfreeze

### Manager
- `internal/print/manager.go` — `Print()` and `PrintImage()` gain `printOpts` param; build MediaInfo from session status; validate opts

### Handler
- `internal/handler/print.go` — parse new fields from request body, pass to manager; capabilities endpoint; preview accepts optional `printer_id`

### UI
- `internal/embedded/templates/` — print modal controls
- `internal/embedded/static/js/` — dynamic show/hide on printer change

### New endpoint
- `GET /api/printers/{id}/capabilities`

## 8. Out of Scope

- Print queue (Spec 2 — builds on this)
- PrinterManager decomposition (issue #75 — separate effort)
- Designer templates fit-to-label (Fabric.js client-side rendering)
- New schemas/templates
- 600 DPI renderer-side rendering (future upgrade from row duplication)
