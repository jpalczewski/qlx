# Phase 1a — Template Batch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add multi-font registry, container/ad-hoc label printing, inline icon rasterization, and extended built-in schemas with tags/children support.

**Architecture:** Extend the existing label rendering pipeline (`internal/print/label/`) with a font registry, SVG icon rasterizer, and new schema slots. Add container print and ad-hoc print endpoints following the existing handler pattern. All fonts embedded in binary.

**Tech Stack:** Go 1.22+, `golang.org/x/image/font/opentype`, `srwiley/oksvg` + `srwiley/rasterx` (SVG rasterization), Phosphor Icons SVG (already embedded)

**Spec:** `docs/superpowers/specs/2026-03-22-template-batch-phase1a-design.md`

---

### Task 1: Font Registry

Replace the hardcoded Spleen font loading with a registry that supports multiple fonts by name.

**Files:**
- Create: `internal/print/label/font_registry.go`
- Create: `internal/print/label/font_registry_test.go`
- Modify: `internal/print/label/schema_renderer.go:81-96` (use registry instead of `loadFontFace`/`loadBasicFontFace`)
- Modify: `internal/print/label/schema.go:23-31` (add `FontFamily` to Element)
- Delete: `internal/print/label/font.go` (replaced by font_registry.go)
- Download: `internal/print/label/fonts/noto-sans-regular.ttf` (from Google Fonts)
- Download: `internal/print/label/fonts/go-mono-regular.ttf` (from Go project)
- Download: `internal/print/label/fonts/terminus-regular.otf` (from Terminus Font project)

**Existing fonts stay in place:** `internal/print/label/fonts/spleen-8x16.otf`, `internal/print/label/fonts/spleen-12x24.otf`

- [ ] **Step 1: Write failing tests for font registry**

```go
// font_registry_test.go
package label

import "testing"

func TestLoadFace_Spleen(t *testing.T) {
    face, err := LoadFace("spleen", 24)
    if err != nil {
        t.Fatalf("LoadFace spleen: %v", err)
    }
    if face == nil {
        t.Fatal("expected non-nil face")
    }
}

func TestLoadFace_SpleenSmall(t *testing.T) {
    face, err := LoadFace("spleen", 14)
    if err != nil {
        t.Fatalf("LoadFace spleen small: %v", err)
    }
    if face == nil {
        t.Fatal("expected non-nil face")
    }
}

func TestLoadFace_Basic(t *testing.T) {
    face, err := LoadFace("basic", 13)
    if err != nil {
        t.Fatalf("LoadFace basic: %v", err)
    }
    if face == nil {
        t.Fatal("expected non-nil face")
    }
}

func TestLoadFace_Unknown(t *testing.T) {
    _, err := LoadFace("nonexistent", 13)
    if err == nil {
        t.Fatal("expected error for unknown font")
    }
}

func TestLoadFace_NotoSans(t *testing.T) {
    face, err := LoadFace("noto-sans", 16)
    if err != nil {
        t.Fatalf("LoadFace noto-sans: %v", err)
    }
    if face == nil {
        t.Fatal("expected non-nil face")
    }
}

func TestLoadFace_GoMono(t *testing.T) {
    face, err := LoadFace("go-mono", 16)
    if err != nil {
        t.Fatalf("LoadFace go-mono: %v", err)
    }
    if face == nil {
        t.Fatal("expected non-nil face")
    }
}

func TestLoadFace_Terminus(t *testing.T) {
    face, err := LoadFace("terminus", 16)
    if err != nil {
        t.Fatalf("LoadFace terminus: %v", err)
    }
    if face == nil {
        t.Fatal("expected non-nil face")
    }
}

func TestFontNames(t *testing.T) {
    names := FontNames()
    expected := map[string]bool{"spleen": true, "basic": true, "noto-sans": true, "go-mono": true, "terminus": true}
    for _, n := range names {
        if !expected[n] {
            t.Errorf("unexpected font name: %s", n)
        }
        delete(expected, n)
    }
    for n := range expected {
        t.Errorf("missing font name: %s", n)
    }
}

func TestTransliteratePL(t *testing.T) {
    got := TransliteratePL("ąćęłńóśźż")
    want := "acelnoszz"
    if got != want {
        t.Errorf("TransliteratePL = %q, want %q", got, want)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/print/label/ -run TestLoadFace -v`
Expected: FAIL — `LoadFace` undefined

- [ ] **Step 3: Download font files**

Download these open-source fonts into `internal/print/label/fonts/`:
- Noto Sans Regular TTF (~400KB) — from Google Fonts, OFL license
- Go Mono Regular TTF (~400KB) — from `golang/image` repo, BSD license
- Terminus Regular OTF (~200KB) — from Terminus Font, OFL license

Verify each file exists and is valid:
```bash
ls -la internal/print/label/fonts/
# Should show: spleen-8x16.otf, spleen-12x24.otf, noto-sans-regular.ttf, go-mono-regular.ttf, terminus-regular.otf
```

- [ ] **Step 4: Implement font_registry.go**

```go
// font_registry.go
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

// fontEntry describes an embedded font.
type fontEntry struct {
    path      string // primary font file path within fontsFS
    smallPath string // optional smaller variant
    threshold float64 // size threshold for switching to small variant
    builtin   bool   // true = use basicfont.Face7x13
}

var fontCatalog = map[string]fontEntry{
    "spleen":    {path: "fonts/spleen-12x24.otf", smallPath: "fonts/spleen-8x16.otf", threshold: 20},
    "noto-sans": {path: "fonts/noto-sans-regular.ttf"},
    "go-mono":   {path: "fonts/go-mono-regular.ttf"},
    "terminus":  {path: "fonts/terminus-regular.otf"},
    "basic":     {builtin: true},
}

// faceCache stores loaded font faces keyed by "name:size".
var faceCache sync.Map

// parsedFonts stores parsed opentype.Font objects keyed by file path.
var parsedFonts sync.Map

func parseFont(path string) (*opentype.Font, error) {
    if f, ok := parsedFonts.Load(path); ok {
        return f.(*opentype.Font), nil
    }
    data, err := fontsFS.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read font %s: %w", path, err)
    }
    f, err := opentype.Parse(data)
    if err != nil {
        return nil, fmt.Errorf("parse font %s: %w", path, err)
    }
    parsedFonts.Store(path, f)
    return f, nil
}

// LoadFace returns a font.Face for the named font at the given pixel size.
// Thread-safe with caching.
func LoadFace(name string, sizePx float64) (font.Face, error) {
    key := fmt.Sprintf("%s:%.1f", name, sizePx)
    if f, ok := faceCache.Load(key); ok {
        return f.(font.Face), nil
    }

    entry, ok := fontCatalog[name]
    if !ok {
        return nil, fmt.Errorf("unknown font: %q (available: %s)", name, strings.Join(FontNames(), ", "))
    }

    if entry.builtin {
        face := basicfont.Face7x13
        faceCache.Store(key, face)
        return face, nil
    }

    // Pick the right variant for fonts with size thresholds
    path := entry.path
    size := sizePx
    if entry.smallPath != "" && sizePx < entry.threshold {
        path = entry.smallPath
        size = 16.0 // fixed size for small Spleen variant
    } else if entry.smallPath != "" && sizePx >= entry.threshold {
        size = 24.0 // fixed size for large Spleen variant
    }

    f, err := parseFont(path)
    if err != nil {
        return nil, err
    }

    face, err := opentype.NewFace(f, &opentype.FaceOptions{
        Size:    size,
        DPI:     72,
        Hinting: font.HintingFull,
    })
    if err != nil {
        return nil, fmt.Errorf("create face %s@%.0f: %w", name, sizePx, err)
    }

    faceCache.Store(key, face)
    return face, nil
}

// FontNames returns sorted names of all available fonts.
func FontNames() []string {
    names := make([]string, 0, len(fontCatalog))
    for n := range fontCatalog {
        names = append(names, n)
    }
    sort.Strings(names)
    return names
}

// IsBasicFont returns true if the named font is ASCII-only and requires transliteration.
func IsBasicFont(name string) bool {
    entry, ok := fontCatalog[name]
    return ok && entry.builtin
}

// TransliteratePL replaces Polish diacritic characters with ASCII equivalents.
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
```

- [ ] **Step 5: Delete font.go**

Remove `internal/print/label/font.go` — all its functionality is now in `font_registry.go`.

- [ ] **Step 6: Add FontFamily field to Element struct**

In `internal/print/label/schema.go`, add to Element:

```go
type Element struct {
    Slot       string  `json:"slot"`
    FontSize   float64 `json:"font_size"`
    FontFamily string  `json:"font_family"` // override schema default; empty = inherit
    Align      string  `json:"align"`
    Wrap       bool    `json:"wrap"`
    Color      string  `json:"color"`
    Size       int     `json:"size"`
    Height     int     `json:"height"`
}
```

- [ ] **Step 7: Update resolveTextElement to use font registry**

In `internal/print/label/schema_renderer.go`, update `resolveTextElement` (lines 81-117) and `resolveElements` (lines 48-78) to:
1. Determine effective font: `el.FontFamily` if set, else `schema.FontFamily`, else `"spleen"`
2. Use `LoadFace(effectiveFont, el.FontSize)` instead of `loadFontFace`/`loadBasicFontFace`
3. Use `IsBasicFont(effectiveFont)` to decide transliteration

```go
func resolveTextElement(el Element, text, schemaFont string, widthPx, pad, qrReserved int) (resolvedText, error) {
    effectiveFont := el.FontFamily
    if effectiveFont == "" {
        effectiveFont = schemaFont
    }
    if effectiveFont == "" {
        effectiveFont = "spleen"
    }

    if IsBasicFont(effectiveFont) {
        text = TransliteratePL(text)
    }

    face, err := LoadFace(effectiveFont, el.FontSize)
    if err != nil {
        return resolvedText{}, err
    }

    metrics := face.Metrics()
    lh := (metrics.Ascent + metrics.Descent).Ceil()
    if !IsBasicFont(effectiveFont) {
        lh = (metrics.Ascent + metrics.Descent + fixed.I(int(el.FontSize/4))).Ceil()
    }

    textW := widthPx - pad*2
    if qrReserved > 0 {
        textW -= qrReserved + pad
    }

    var lines []string
    if el.Wrap {
        lines = wrapText(text, face, textW)
    } else if text != "" {
        lines = []string{text}
    }

    return resolvedText{
        lines: lines,
        face:  face,
        col:   parseHexColor(el.Color),
        align: el.Align,
        lineH: lh,
    }, nil
}
```

- [ ] **Step 8: Run all tests**

Run: `go test ./internal/print/label/ -v`
Expected: All pass including new font registry tests

- [ ] **Step 9: Commit**

```bash
git add internal/print/label/font_registry.go internal/print/label/font_registry_test.go internal/print/label/fonts/ internal/print/label/schema.go internal/print/label/schema_renderer.go
git rm internal/print/label/font.go
git commit -m "feat(label): add multi-font registry with Noto Sans, Go Mono, Terminus"
```

---

### Task 2: LabelData Extension + Tag/Children Types

Extend `LabelData` with Icon, Tags, and Children fields. Add supporting types.

**Files:**
- Modify: `internal/print/label/templates.go`

- [ ] **Step 1: Extend LabelData**

```go
// templates.go
package label

// LabelTag represents a tag with its display info for label rendering.
type LabelTag struct {
    Name string   // tag display name
    Icon string   // Phosphor icon name (may be empty)
    Path []string // ancestor names root-first, e.g. ["elektronika", "arduino"]
}

// LabelChild represents a child container or item for label rendering.
type LabelChild struct {
    Name string // child display name
    Icon string // Phosphor icon name (may be empty)
}

// LabelData holds the data slots for label rendering.
type LabelData struct {
    Name        string       // → "title" slot
    Description string       // → "description" slot
    Location    string       // container path "Room → Shelf" → "location" slot
    QRContent   string       // URL for QR code
    BarcodeID   string       // ID for barcode
    Icon        string       // Phosphor icon name for title
    Tags        []LabelTag   // assigned tags
    Children    []LabelChild // sub-containers + items (container only)
}
```

- [ ] **Step 2: Verify existing tests still pass**

Run: `go test ./internal/print/label/ -v`
Expected: All pass (LabelData extension is backward compatible — new fields zero-valued)

- [ ] **Step 3: Commit**

```bash
git add internal/print/label/templates.go
git commit -m "feat(label): extend LabelData with Icon, Tags, Children fields"
```

---

### Task 3: SVG Icon Rasterizer

Add Phosphor SVG icon rasterization using oksvg/rasterx.

**Files:**
- Modify: `go.mod` (add dependencies)
- Create: `internal/print/label/icon.go`
- Create: `internal/print/label/icon_test.go`

**Docs to check:** https://pkg.go.dev/github.com/srwiley/oksvg and https://pkg.go.dev/github.com/srwiley/rasterx

- [ ] **Step 1: Add dependencies**

```bash
cd /Users/erxyi/Projekty/qlx
go get github.com/srwiley/oksvg github.com/srwiley/rasterx
```

- [ ] **Step 2: Write failing test**

```go
// icon_test.go
package label

import "testing"

func TestRasterizeIcon_Package(t *testing.T) {
    img, err := RasterizeIcon("package", 24)
    if err != nil {
        t.Fatalf("RasterizeIcon: %v", err)
    }
    b := img.Bounds()
    if b.Dx() != 24 || b.Dy() != 24 {
        t.Errorf("size = %dx%d, want 24x24", b.Dx(), b.Dy())
    }
}

func TestRasterizeIcon_Empty(t *testing.T) {
    img, err := RasterizeIcon("", 24)
    if err != nil {
        t.Fatalf("RasterizeIcon empty: %v", err)
    }
    if img != nil {
        t.Error("expected nil image for empty icon name")
    }
}

func TestRasterizeIcon_Unknown(t *testing.T) {
    _, err := RasterizeIcon("nonexistent-icon-xyz", 24)
    if err == nil {
        t.Fatal("expected error for unknown icon")
    }
}

func TestRasterizeIcon_Cached(t *testing.T) {
    img1, _ := RasterizeIcon("package", 16)
    img2, _ := RasterizeIcon("package", 16)
    if img1 != img2 {
        t.Error("expected cached result to return same pointer")
    }
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/print/label/ -run TestRasterizeIcon -v`
Expected: FAIL — `RasterizeIcon` undefined

- [ ] **Step 4: Implement icon.go**

```go
// icon.go
package label

import (
    "fmt"
    "image"
    "image/color"
    "image/draw"
    "strings"
    "sync"

    "github.com/erxyi/qlx/internal/shared/palette"
    "github.com/srwiley/oksvg"
    "github.com/srwiley/rasterx"
)

var iconCache sync.Map // key: "name:size" → *image.RGBA

// RasterizeIcon renders a Phosphor SVG icon to a bitmap of sizePx × sizePx.
// Returns nil image (no error) if name is empty. Cached by (name, sizePx).
func RasterizeIcon(name string, sizePx int) (image.Image, error) {
    if name == "" {
        return nil, nil
    }

    key := fmt.Sprintf("%s:%d", name, sizePx)
    if cached, ok := iconCache.Load(key); ok {
        return cached.(image.Image), nil
    }

    svgData, err := palette.IconSVG(name)
    if err != nil {
        return nil, fmt.Errorf("icon %q: %w", name, err)
    }

    icon, err := oksvg.ReadIconStream(strings.NewReader(string(svgData)))
    if err != nil {
        return nil, fmt.Errorf("parse SVG %q: %w", name, err)
    }

    icon.SetTarget(0, 0, float64(sizePx), float64(sizePx))

    img := image.NewRGBA(image.Rect(0, 0, sizePx, sizePx))
    // Fill transparent
    draw.Draw(img, img.Bounds(), image.NewUniform(color.Transparent), image.Point{}, draw.Src)

    scanner := rasterx.NewScannerGV(sizePx, sizePx, img, img.Bounds())
    dasher := rasterx.NewDasher(sizePx, sizePx, scanner)
    icon.Draw(dasher, 1.0)

    iconCache.Store(key, img)
    return img, nil
}
```

- [ ] **Step 5: Verify palette.IconSVG exists or add it**

Check if `palette.IconSVG(name)` already exists. The current code has `palette.IconFS` and a function that reads `"phosphor/" + name + ".svg"`. If `IconSVG` doesn't exist as a public function, add it to `internal/shared/palette/icons_embed.go`:

```go
// IconSVG returns the raw SVG bytes for the named icon.
func IconSVG(name string) ([]byte, error) {
    return IconFS.ReadFile("phosphor/" + name + ".svg")
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/print/label/ -run TestRasterizeIcon -v`
Expected: All pass

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/print/label/icon.go internal/print/label/icon_test.go internal/shared/palette/icons_embed.go
git commit -m "feat(label): add SVG icon rasterizer with oksvg/rasterx"
```

---

### Task 4: Schema Extension — New Slots and Fields

Add `tags`, `children` slots and `show_path`/`show_icons` fields to Element. Update the renderer to handle them.

**Files:**
- Modify: `internal/print/label/schema.go:23-31` (Element struct)
- Modify: `internal/print/label/schema_renderer.go` (new slot rendering)
- Create: `internal/print/label/schema_renderer_test.go` (render tests with new slots)

- [ ] **Step 1: Extend Element struct**

In `internal/print/label/schema.go`, update Element:

```go
type Element struct {
    Slot       string  `json:"slot"`        // title, description, location, tags, children, qr, barcode
    FontSize   float64 `json:"font_size"`   // pixel size for text slots (default 13)
    FontFamily string  `json:"font_family"` // override schema default; empty = inherit
    Align      string  `json:"align"`       // left, center, right (default left)
    Wrap       bool    `json:"wrap"`         // enable text wrapping
    Color      string  `json:"color"`        // hex color (default "#000000")
    Size       int     `json:"size"`         // px for qr
    Height     int     `json:"height"`       // px for barcode
    ShowPath   string  `json:"show_path"`    // tags only: "auto"|"true"|"false" (default "auto")
    ShowIcons  *bool   `json:"show_icons"`   // render inline icons (default true for title/children/tags)
}
```

Update `parseSchema` defaults — add `ShowPath` default:

```go
for i := range s.Elements {
    // ... existing defaults ...
    if s.Elements[i].ShowPath == "" {
        s.Elements[i].ShowPath = "auto"
    }
}
```

- [ ] **Step 2: Write failing tests for tag/children rendering**

```go
// schema_renderer_test.go
package label

import (
    "image"
    "testing"
)

func TestRenderSchema_WithTags(t *testing.T) {
    data := LabelData{
        Name: "Test Item",
        Tags: []LabelTag{
            {Name: "arduino", Path: []string{"elektronika", "arduino"}},
        },
    }
    img, err := Render(data, "simple", 384, 203)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }
    if img.Bounds().Dx() != 384 {
        t.Errorf("width = %d, want 384", img.Bounds().Dx())
    }
}

func TestRenderSchema_WithChildren(t *testing.T) {
    data := LabelData{
        Name: "Storage Box",
        Children: []LabelChild{
            {Name: "Screwdriver", Icon: "screwdriver"},
            {Name: "Drawer A", Icon: "package"},
        },
    }
    img, err := Render(data, "detailed", 384, 203)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }
    if img.Bounds().Dx() != 384 {
        t.Errorf("width = %d, want 384", img.Bounds().Dx())
    }
}

func TestRenderSchema_WithIcon(t *testing.T) {
    data := LabelData{
        Name: "My Item",
        Icon: "package",
    }
    img, err := Render(data, "simple", 384, 203)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }
    // Image should be non-nil and have correct width
    if img.(*image.RGBA) == nil {
        t.Fatal("expected RGBA image")
    }
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/print/label/ -run TestRenderSchema_ -v`
Expected: FAIL or unexpected behavior (tags/children not rendered yet, but tests should at minimum compile once Element is updated)

- [ ] **Step 4: Update resolveElements to handle new slots**

In `schema_renderer.go`, update `resolveElements` to handle `"tags"` and `"children"` slots:

```go
func resolveElements(schema Schema, data LabelData, widthPx, pad, qrReserved int) ([]resolvedText, int, int, error) {
    slotText := map[string]string{
        "title":       data.Name,
        "description": data.Description,
        "location":    data.Location,
    }

    var textElems []resolvedText
    var qrSize, barcodeH int

    for _, el := range schema.Elements {
        switch el.Slot {
        case "title":
            text := slotText[el.Slot]
            effectiveFont := effectiveFontFamily(el, schema)
            if IsBasicFont(effectiveFont) {
                text = TransliteratePL(text)
            }
            rt, err := resolveTextElement(el, text, schema.FontFamily, widthPx, pad, qrReserved)
            if err != nil {
                return nil, 0, 0, err
            }
            // Prepend icon space if present (icon drawn later in drawTextElements)
            if data.Icon != "" && showIcons(el) {
                rt.iconName = data.Icon
            }
            textElems = append(textElems, rt)

        case "description", "location":
            text := slotText[el.Slot]
            effectiveFont := effectiveFontFamily(el, schema)
            if IsBasicFont(effectiveFont) {
                text = TransliteratePL(text)
            }
            rt, err := resolveTextElement(el, text, schema.FontFamily, widthPx, pad, qrReserved)
            if err != nil {
                return nil, 0, 0, err
            }
            textElems = append(textElems, rt)

        case "tags":
            if len(data.Tags) > 0 {
                text := formatTags(data.Tags, el, widthPx-pad*2-qrReserved)
                effectiveFont := effectiveFontFamily(el, schema)
                if IsBasicFont(effectiveFont) {
                    text = TransliteratePL(text)
                }
                rt, err := resolveTextElement(el, text, schema.FontFamily, widthPx, pad, qrReserved)
                if err != nil {
                    return nil, 0, 0, err
                }
                // Store tag icons for inline rendering
                if showIcons(el) {
                    rt.tagIcons = extractTagIcons(data.Tags)
                }
                textElems = append(textElems, rt)
            }

        case "children":
            if len(data.Children) > 0 {
                text := formatChildren(data.Children)
                effectiveFont := effectiveFontFamily(el, schema)
                if IsBasicFont(effectiveFont) {
                    text = TransliteratePL(text)
                }
                rt, err := resolveTextElement(el, text, schema.FontFamily, widthPx, pad, qrReserved)
                if err != nil {
                    return nil, 0, 0, err
                }
                if showIcons(el) {
                    rt.childIcons = extractChildIcons(data.Children)
                }
                textElems = append(textElems, rt)
            }

        case "qr":
            qrSize = el.Size
        case "barcode":
            barcodeH = el.Height
        }
    }

    return textElems, qrSize, barcodeH, nil
}

// effectiveFontFamily returns the font to use for an element.
func effectiveFontFamily(el Element, schema Schema) string {
    if el.FontFamily != "" {
        return el.FontFamily
    }
    if schema.FontFamily != "" {
        return schema.FontFamily
    }
    return "spleen"
}

// showIcons returns whether icons should be rendered for this element.
func showIcons(el Element) bool {
    if el.ShowIcons != nil {
        return *el.ShowIcons
    }
    // Default: true for title, tags, children
    return el.Slot == "title" || el.Slot == "tags" || el.Slot == "children"
}
```

- [ ] **Step 5: Add tag/children formatting helpers**

Add to `schema_renderer.go`:

```go
// formatTags formats tag list as text. ShowPath controls ancestor display.
func formatTags(tags []LabelTag, el Element, maxWidth int) string {
    var parts []string
    for _, tag := range tags {
        switch el.ShowPath {
        case "true":
            parts = append(parts, "#"+strings.Join(tag.Path, ">"))
        case "false":
            parts = append(parts, "#"+tag.Name)
        default: // "auto"
            full := "#" + strings.Join(tag.Path, ">")
            parts = append(parts, full)
        }
    }
    text := strings.Join(parts, " ")

    // Auto fallback: if text is too long, use leaf names only
    if el.ShowPath == "auto" && len(text) > maxWidth/6 { // rough char estimate
        var short []string
        for _, tag := range tags {
            short = append(short, "#"+tag.Name)
        }
        text = strings.Join(short, " ")
    }

    return text
}

// formatChildren formats children list as comma-separated text.
func formatChildren(children []LabelChild) string {
    var names []string
    for _, c := range children {
        names = append(names, c.Name)
    }
    return strings.Join(names, ", ")
}

// extractTagIcons returns icon names from tags (preserving order, empty strings for no-icon tags).
func extractTagIcons(tags []LabelTag) []string {
    icons := make([]string, len(tags))
    for i, t := range tags {
        icons[i] = t.Icon
    }
    return icons
}

// extractChildIcons returns icon names from children.
func extractChildIcons(children []LabelChild) []string {
    icons := make([]string, len(children))
    for i, c := range children {
        icons[i] = c.Icon
    }
    return icons
}
```

- [ ] **Step 6: Extend resolvedText with icon fields**

```go
type resolvedText struct {
    lines      []string
    face       font.Face
    col        color.RGBA
    align      string
    lineH      int
    iconName   string   // single icon for title
    tagIcons   []string // icons per tag (for tags slot)
    childIcons []string // icons per child (for children slot)
}
```

- [ ] **Step 7: Update drawTextElements to render inline icons**

Update `drawTextElements` in `schema_renderer.go` to draw icons before text when present:

```go
func drawTextElements(img *image.RGBA, textElems []resolvedText, widthPx, pad, qrSize int) {
    y := pad
    for _, te := range textElems {
        for _, line := range te.lines {
            baseline := y + te.lineH
            xOffset := 0

            // Draw inline icon for title
            if te.iconName != "" {
                iconSize := te.lineH - 2
                iconImg, err := RasterizeIcon(te.iconName, iconSize)
                if err == nil && iconImg != nil {
                    iconY := y + 1
                    draw.Draw(img, image.Rect(pad, iconY, pad+iconSize, iconY+iconSize),
                        iconImg, image.Point{}, draw.Over)
                    xOffset = iconSize + 4 // icon + gap
                }
                te.iconName = "" // only draw icon on first line
            }

            x := alignedX(te.face, line, te.align, widthPx, pad+xOffset, qrSize)
            if xOffset > 0 && te.align == "left" {
                x = pad + xOffset
            }
            drawTextFace(img, x, baseline, line, te.col, te.face)
            y += te.lineH
        }
    }
}
```

Note: Tag and child icons are more complex — they appear inline within comma-separated text. A simpler approach for v1: render tag/child icons as a prefix row or skip icon-per-tag rendering and just render the icon for the first line. The exact icon-per-item rendering in tags/children can be refined in a follow-up once the basic pipeline works.

- [ ] **Step 8: Run all tests**

Run: `go test ./internal/print/label/ -v`
Expected: All pass

- [ ] **Step 9: Commit**

```bash
git add internal/print/label/schema.go internal/print/label/schema_renderer.go internal/print/label/schema_renderer_test.go
git commit -m "feat(label): add tags, children, icon rendering to schema pipeline"
```

---

### Task 5: Print Date Metadata

Add `PrintDate` option to renderer — appends timestamp at bottom of label.

**Files:**
- Modify: `internal/print/label/renderer.go` (new `RenderOpts`, pass through)
- Modify: `internal/print/label/schema_renderer.go` (append date line)
- Create: `internal/print/label/renderer_test.go`

- [ ] **Step 1: Add RenderOpts**

In `renderer.go`, change `Render` signature:

```go
// RenderOpts controls optional label rendering behavior.
type RenderOpts struct {
    PrintDate bool // append "Wydrukowano: DATE" at bottom
}

// Render produces a label image from the given data using the named schema.
func Render(data LabelData, template string, widthPx, dpi int, opts RenderOpts) (image.Image, error) {
    schema, ok := GetSchema(template)
    if !ok {
        return nil, fmt.Errorf("unknown template %q: valid templates are %v", template, SchemaNames())
    }
    return renderSchema(schema, data, widthPx, opts)
}
```

- [ ] **Step 2: Pass opts through renderSchema**

Update `renderSchema` signature and add date rendering after barcode:

```go
func renderSchema(schema Schema, data LabelData, widthPx int, opts RenderOpts) (image.Image, error) {
    // ... existing code ...

    // Add print date line height to total if enabled
    var dateLine resolvedText
    if opts.PrintDate {
        dateText := TransliteratePL("Wydrukowano: " + time.Now().Format("2006-01-02 15:04"))
        face := loadBasicFace() // basicfont 7x13
        metrics := face.Metrics()
        lh := (metrics.Ascent + metrics.Descent).Ceil()
        dateLine = resolvedText{
            lines: []string{dateText},
            face:  face,
            col:   parseHexColor("#808080"),
            align: "left",
            lineH: lh,
        }
    }

    totalH := computeHeight(textElems, qrSize, barcodeH, data.BarcodeID, pad)
    if opts.PrintDate {
        totalH += dateLine.lineH + 2 // small gap
    }

    img := newCanvas(widthPx, totalH)
    drawTextElements(img, textElems, widthPx, pad, qrSize)

    // ... QR and barcode ...

    // Draw print date at very bottom
    if opts.PrintDate {
        dateY := totalH - dateLine.lineH - 2
        drawTextFace(img, pad, dateY+dateLine.lineH, dateLine.lines[0], dateLine.col, dateLine.face)
    }

    return img, nil
}
```

Note: `loadBasicFace()` is just `LoadFace("basic", 13)` — use the registry.

- [ ] **Step 3: Update all callers of Render**

In `internal/print/manager.go`, update the `Print` call:

```go
img, err := label.Render(data, templateName, modelInfo.PrintWidthPx, modelInfo.DPI, label.RenderOpts{})
```

This maintains backward compatibility — `PrintDate: false` by default.

- [ ] **Step 4: Write test**

```go
// renderer_test.go
package label

import "testing"

func TestRender_PrintDate(t *testing.T) {
    data := LabelData{Name: "Test"}
    imgWithout, err := Render(data, "simple", 384, 203, RenderOpts{})
    if err != nil {
        t.Fatalf("Render without date: %v", err)
    }
    imgWith, err := Render(data, "simple", 384, 203, RenderOpts{PrintDate: true})
    if err != nil {
        t.Fatalf("Render with date: %v", err)
    }
    // Image with date should be taller
    if imgWith.Bounds().Dy() <= imgWithout.Bounds().Dy() {
        t.Errorf("expected image with date to be taller: %d vs %d",
            imgWith.Bounds().Dy(), imgWithout.Bounds().Dy())
    }
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/print/label/ -v`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add internal/print/label/renderer.go internal/print/label/renderer_test.go internal/print/label/schema_renderer.go internal/print/manager.go
git commit -m "feat(label): add print date metadata option"
```

---

### Task 6: Update Built-in Schemas

Update all JSON schema files and create the new `contents.json`.

**Files:**
- Modify: `internal/print/label/schemas/micro.json`
- Modify: `internal/print/label/schemas/simple.json`
- Modify: `internal/print/label/schemas/compact.json`
- Modify: `internal/print/label/schemas/standard.json`
- Modify: `internal/print/label/schemas/detailed.json`
- Create: `internal/print/label/schemas/contents.json`

- [ ] **Step 1: Update micro.json**

```json
{
    "name": "micro",
    "padding": 4,
    "font_family": "basic",
    "elements": [
        {"slot": "title", "wrap": true},
        {"slot": "description", "font_size": 11},
        {"slot": "location", "font_size": 11},
        {"slot": "tags", "font_size": 11, "wrap": true}
    ]
}
```

- [ ] **Step 2: Update simple.json**

```json
{
    "name": "simple",
    "elements": [
        {"slot": "title", "font_size": 24, "align": "center"},
        {"slot": "description", "font_size": 16, "align": "center"},
        {"slot": "location", "font_size": 13},
        {"slot": "tags", "font_size": 11, "wrap": true}
    ]
}
```

- [ ] **Step 3: Update compact.json**

```json
{
    "name": "compact",
    "elements": [
        {"slot": "title"},
        {"slot": "description"},
        {"slot": "tags", "font_size": 11, "wrap": true}
    ]
}
```

- [ ] **Step 4: Update standard.json**

```json
{
    "name": "standard",
    "elements": [
        {"slot": "title"},
        {"slot": "location"},
        {"slot": "tags", "font_size": 11, "wrap": true},
        {"slot": "qr", "size": 80}
    ]
}
```

- [ ] **Step 5: Update detailed.json**

```json
{
    "name": "detailed",
    "elements": [
        {"slot": "title"},
        {"slot": "description"},
        {"slot": "location"},
        {"slot": "tags", "font_size": 11, "wrap": true},
        {"slot": "children", "font_size": 11, "wrap": true},
        {"slot": "qr", "size": 96},
        {"slot": "barcode", "height": 32}
    ]
}
```

- [ ] **Step 6: Create contents.json**

```json
{
    "name": "contents",
    "elements": [
        {"slot": "title", "font_size": 16},
        {"slot": "children", "font_size": 12, "wrap": true}
    ]
}
```

- [ ] **Step 7: Update PrintItem schema check**

In `internal/handler/print.go`, the `PrintItem` handler hardcodes schema names:

```go
case "simple", "standard", "compact", "detailed":
```

Change to use `label.GetSchema`:

```go
if _, ok := label.GetSchema(req.Template); ok {
    // built-in schema path
} else {
    // designer template path
}
```

This automatically picks up `micro` and `contents` schemas.

- [ ] **Step 8: Run tests**

Run: `go test ./internal/print/label/ -v && go test ./internal/handler/ -v`
Expected: All pass

- [ ] **Step 9: Commit**

```bash
git add internal/print/label/schemas/ internal/handler/print.go
git commit -m "feat(label): update built-in schemas with tags/children, add contents schema"
```

---

### Task 7: Container Label Printing

Add print endpoint for containers with multi-template support.

**Files:**
- Modify: `internal/handler/print.go` (add `PrintContainer` handler)
- Modify: `internal/handler/request.go` (add `ContainerPrintRequest`)
- Modify: `internal/handler/print.go:34-46` (register new route)
- Modify: `internal/print/manager.go` (add `RenderOpts` pass-through)
- Modify: `internal/embedded/templates/pages/inventory/containers.html` (print section UI)

- [ ] **Step 1: Add ContainerPrintRequest**

In `internal/handler/request.go`:

```go
// ContainerPrintRequest is the input for container print operations.
type ContainerPrintRequest struct {
    PrinterID    string   `json:"printer_id"`
    Templates    []string `json:"templates"`
    PrintDate    bool     `json:"print_date"`
    ShowChildren bool     `json:"show_children"`
}
```

- [ ] **Step 2: Add PrintDate to PrintRequest**

In `internal/handler/request.go`, extend existing:

```go
type PrintRequest struct {
    PrinterID string `json:"printer_id"`
    Template  string `json:"template"`
    PrintDate bool   `json:"print_date"`
}
```

- [ ] **Step 3: Write failing test for container print endpoint**

```go
// In a new file or extend existing handler tests
// internal/handler/print_test.go
package handler

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestPrintContainer_NotFound(t *testing.T) {
    // Setup with NewMemoryStore, create PrintHandler
    // POST /containers/nonexistent/print should return 404
}
```

(Full test setup depends on existing test patterns in handler package — follow `containers_test.go` pattern.)

- [ ] **Step 4: Implement PrintContainer handler**

In `internal/handler/print.go`:

```go
// PrintContainer handles POST /containers/{id}/print.
func (h *PrintHandler) PrintContainer(w http.ResponseWriter, r *http.Request) {
    var req ContainerPrintRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
        return
    }

    container := h.inventory.GetContainer(r.PathValue("id"))
    if container == nil {
        http.NotFound(w, r)
        return
    }

    path := h.inventory.ContainerPath(container.ParentID)
    data := label.LabelData{
        Name:        container.Name,
        Description: container.Description,
        Location:    webutil.FormatContainerPath(path, " → "),
        QRContent:   "/containers/" + container.ID,
        BarcodeID:   container.ID,
        Icon:        container.Icon,
    }

    // Resolve tags
    if h.tags != nil {
        for _, tagID := range container.TagIDs {
            tagPath := h.tags.TagPath(tagID)
            if len(tagPath) > 0 {
                tag := tagPath[len(tagPath)-1]
                pathNames := make([]string, len(tagPath))
                for i, t := range tagPath {
                    pathNames[i] = t.Name
                }
                data.Tags = append(data.Tags, label.LabelTag{
                    Name: tag.Name,
                    Icon: tag.Icon,
                    Path: pathNames,
                })
            }
        }
    }

    // Resolve children
    if req.ShowChildren {
        children := h.inventory.ListContainers(container.ID)
        for _, c := range children {
            data.Children = append(data.Children, label.LabelChild{Name: c.Name, Icon: c.Icon})
        }
        items := h.inventory.ListItems(container.ID)
        for _, item := range items {
            data.Children = append(data.Children, label.LabelChild{Name: item.Name, Icon: item.Icon})
        }
    }

    opts := label.RenderOpts{PrintDate: req.PrintDate}

    // Print each template sequentially
    for _, tmplName := range req.Templates {
        if _, ok := label.GetSchema(tmplName); ok {
            if err := h.pm.Print(req.PrinterID, data, tmplName, opts); err != nil {
                webutil.LogError("container print failed: %v", err)
                webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
                return
            }
        } else {
            // Designer template — return client render instruction (only first template)
            tmpl := h.templates.GetTemplate(tmplName)
            if tmpl == nil {
                webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "template not found: " + tmplName})
                return
            }
            webutil.JSON(w, http.StatusOK, map[string]any{
                "ok":        true,
                "render":    "client",
                "template":  tmpl,
                "item_data": data,
            })
            return
        }
    }

    webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
```

Note: This requires adding `tags *service.TagService` to `PrintHandler` and updating `NewPrintHandler`.

- [ ] **Step 5: Update PrintHandler to include TagService**

```go
type PrintHandler struct {
    pm        *print.PrinterManager
    inventory *service.InventoryService
    printers  *service.PrinterService
    templates *service.TemplateService
    tags      *service.TagService // ← new
    resp      Responder
}
```

Update constructor and wiring in `internal/app/server.go`.

- [ ] **Step 6: Update PrintItem to also populate tags**

In `PrintItem`, after building `data`, add tag resolution (same pattern as container).

- [ ] **Step 7: Update PrinterManager.Print signature**

In `internal/print/manager.go`, update `Print` to accept `RenderOpts`:

```go
func (m *PrinterManager) Print(printerID string, data label.LabelData, templateName string, opts label.RenderOpts) error {
    // ...
    img, err := label.Render(data, templateName, modelInfo.PrintWidthPx, modelInfo.DPI, opts)
    // ...
}
```

- [ ] **Step 8: Register route**

In `PrintHandler.RegisterRoutes`:

```go
mux.HandleFunc("POST /containers/{id}/print", h.PrintContainer)
```

- [ ] **Step 9: Update container detail template with print section**

In `internal/embedded/templates/pages/inventory/containers.html`, replace the existing batch-print-items JavaScript with a proper container print section:

- Multi-select checkboxes for schemas (simple, standard, detailed, contents, etc.)
- Printer selector dropdown
- Checkbox: "Dodaj datę wydruku" (print_date)
- Checkbox: "Pokaż zawartość" (show_children)
- Print button that POSTs to `/containers/{id}/print`

(Follow the existing item.html print section pattern, adapting for multi-template.)

- [ ] **Step 10: Run tests**

Run: `go test ./internal/handler/ -v && go test ./internal/print/... -v`
Expected: All pass

- [ ] **Step 11: Commit**

```bash
git add internal/handler/print.go internal/handler/request.go internal/print/manager.go internal/app/server.go internal/embedded/templates/pages/inventory/containers.html
git commit -m "feat: add container label printing with multi-template support"
```

---

### Task 8: Ad-hoc Label Printing

Add "Quick Print" page for printing arbitrary text labels.

**Files:**
- Create: `internal/handler/adhoc.go`
- Create: `internal/embedded/templates/pages/labels/quick_print.html`
- Modify: `internal/handler/request.go` (add `AdhocPrintRequest`)
- Modify: `internal/app/server.go` (register routes)
- Modify: `internal/embedded/templates/layouts/base.html` (add nav link)

- [ ] **Step 1: Add AdhocPrintRequest**

In `internal/handler/request.go`:

```go
// AdhocPrintRequest is the input for ad-hoc label printing.
type AdhocPrintRequest struct {
    Text      string `json:"text" form:"text"`
    PrinterID string `json:"printer_id" form:"printer_id"`
    Template  string `json:"template" form:"template"`
    PrintDate bool   `json:"print_date"`
}
```

- [ ] **Step 2: Implement adhoc handler**

```go
// adhoc.go
package handler

import (
    "encoding/json"
    "net/http"

    "github.com/erxyi/qlx/internal/print"
    "github.com/erxyi/qlx/internal/print/label"
    "github.com/erxyi/qlx/internal/service"
    "github.com/erxyi/qlx/internal/shared/webutil"
)

// AdhocHandler handles ad-hoc label printing.
type AdhocHandler struct {
    pm        *print.PrinterManager
    printers  *service.PrinterService
    templates *service.TemplateService
    resp      Responder
}

// NewAdhocHandler creates a new AdhocHandler.
func NewAdhocHandler(pm *print.PrinterManager, prn *service.PrinterService,
    tmpl *service.TemplateService, resp Responder) *AdhocHandler {
    return &AdhocHandler{pm: pm, printers: prn, templates: tmpl, resp: resp}
}

// RegisterRoutes registers ad-hoc print routes.
func (h *AdhocHandler) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("GET /quick-print", h.Page)
    mux.HandleFunc("POST /adhoc/print", h.Print)
}

// Page handles GET /quick-print.
func (h *AdhocHandler) Page(w http.ResponseWriter, r *http.Request) {
    vm := struct {
        Printers []store.PrinterConfig
        Schemas  []string
    }{
        Printers: h.printers.AllPrinters(),
        Schemas:  label.SchemaNames(),
    }
    h.resp.Respond(w, r, http.StatusOK, vm, "quick_print", func() any { return vm })
}

// Print handles POST /adhoc/print.
func (h *AdhocHandler) Print(w http.ResponseWriter, r *http.Request) {
    var req AdhocPrintRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
        return
    }

    if req.Text == "" {
        webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "text is required"})
        return
    }

    data := label.LabelData{
        Name: req.Text,
    }
    opts := label.RenderOpts{PrintDate: req.PrintDate}

    if _, ok := label.GetSchema(req.Template); ok {
        if err := h.pm.Print(req.PrinterID, data, req.Template, opts); err != nil {
            webutil.LogError("adhoc print failed: %v", err)
            webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
            return
        }
        webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
    } else {
        tmpl := h.templates.GetTemplate(req.Template)
        if tmpl == nil {
            webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
            return
        }
        webutil.JSON(w, http.StatusOK, map[string]any{
            "ok":        true,
            "render":    "client",
            "template":  tmpl,
            "item_data": data,
        })
    }
}
```

- [ ] **Step 3: Create quick_print.html template**

Create `internal/embedded/templates/pages/labels/quick_print.html` following existing page patterns. Include:
- Textarea for text input
- Printer selector (same pattern as item.html)
- Schema selector
- Checkbox: "Dodaj datę wydruku"
- Print button with JS that POSTs to `/adhoc/print`

- [ ] **Step 4: Add nav link**

In `internal/embedded/templates/layouts/base.html`, add after the templates link:

```html
<a href="/quick-print" hx-get="/quick-print" hx-target="#content">{{.T "nav.quick_print"}}</a>
```

Add i18n keys for `nav.quick_print` in `internal/embedded/static/i18n/en/nav.json` and `pl/nav.json`.

- [ ] **Step 5: Wire handler in server.go**

In `internal/app/server.go`, create and register `AdhocHandler`.

- [ ] **Step 6: Run tests**

Run: `go test ./internal/handler/ -v`
Expected: All pass

- [ ] **Step 7: Commit**

```bash
git add internal/handler/adhoc.go internal/handler/request.go internal/embedded/templates/pages/labels/quick_print.html internal/embedded/templates/layouts/base.html internal/embedded/static/i18n/ internal/app/server.go
git commit -m "feat: add ad-hoc Quick Print page for arbitrary text labels"
```

---

### Task 9: E2E Tests

Add Playwright E2E tests for container printing and ad-hoc printing.

**Files:**
- Create: `e2e/tests/container-print.spec.ts`
- Create: `e2e/tests/quick-print.spec.ts`
- Modify: `e2e/tests/print.spec.ts` (update existing item print tests if needed)

**Reference:** Follow existing patterns in `e2e/tests/print.spec.ts` and `e2e/fixtures/app.ts`.

- [ ] **Step 1: Write container print E2E test**

```typescript
// e2e/tests/container-print.spec.ts
import { test, expect } from '../fixtures/app';

test.describe('Container label printing', () => {
    test('print section visible on container detail', async ({ page, app }) => {
        // Create a container via API
        const res = await page.request.post(`${app.baseURL}/containers`, {
            data: { name: 'Print Test Container', description: 'For print testing' }
        });
        const container = await res.json();

        await page.goto(`${app.baseURL}/containers/${container.id}`);
        await expect(page.locator('#container-print-section')).toBeVisible();
    });

    test('container print sends correct request', async ({ page, app }) => {
        // Create container, navigate to detail
        // Click print with selected schemas
        // Verify POST /containers/{id}/print was called with correct body
    });
});
```

- [ ] **Step 2: Write quick print E2E test**

```typescript
// e2e/tests/quick-print.spec.ts
import { test, expect } from '../fixtures/app';

test.describe('Quick Print (ad-hoc)', () => {
    test('quick print page accessible from nav', async ({ page, app }) => {
        await page.goto(`${app.baseURL}/`);
        await page.click('a[href="/quick-print"]');
        await expect(page.locator('textarea')).toBeVisible();
    });

    test('quick print sends correct request', async ({ page, app }) => {
        await page.goto(`${app.baseURL}/quick-print`);
        await page.fill('textarea', 'Test label text');
        // Select printer and template
        // Click print
        // Verify POST /adhoc/print was called
    });
});
```

- [ ] **Step 3: Run E2E tests**

Run: `make test-e2e`
Expected: All pass (existing + new)

- [ ] **Step 4: Commit**

```bash
git add e2e/tests/container-print.spec.ts e2e/tests/quick-print.spec.ts
git commit -m "test(e2e): add container print and quick print E2E tests"
```

---

### Task 10: Integration Testing & Cleanup

Final integration pass — verify everything works together.

**Files:**
- Run all tests
- Verify build compiles for Mac and MIPS targets

- [ ] **Step 1: Run full test suite**

```bash
make test
make lint
```

- [ ] **Step 2: Run E2E tests**

```bash
make test-e2e
```

- [ ] **Step 3: Verify MIPS build**

```bash
make build-mips
```

Expected: Build succeeds. `oksvg`/`rasterx` are pure Go — no CGO needed. Font files are embedded.

- [ ] **Step 4: Verify Mac build**

```bash
make build-mac
```

- [ ] **Step 5: Manual smoke test**

```bash
make run
# Navigate to a container detail → verify print section
# Navigate to Quick Print → verify form
# Create an item with tags → print with "detailed" template → verify tags appear
```

- [ ] **Step 6: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "chore: Phase 1a integration cleanup"
```
