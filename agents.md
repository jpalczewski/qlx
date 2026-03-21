# QLX — Project Specification

> Label printer proxy + inventory system for Brother QL-700 on GL-iNet GL-AR 150 router.
> Single Go binary. HTMX frontend. Runs on MIPS.

## Overview

QLX is a self-contained web application that runs on a GL-iNet GL-AR 150 travel router (OpenWrt, MIPS 24Kc, 64MB RAM, 16MB flash). It serves as a network-accessible label printing proxy for a Brother QL-700 USB label printer, combined with a lightweight inventory system for managing items and nested containers.

The project produces a single statically-linked Go binary (~2-3 MB after compression) with all assets (HTML templates, fonts, static files) embedded via `go:embed`. No external dependencies at runtime.

**Repository:** `github.com/<user>/qlx`
**License:** MIT

---

## Hardware Constraints

| Resource | Value | Implication |
|----------|-------|-------------|
| CPU | MIPS 24Kc @ 400MHz, soft-float | No FPU. Use `GOMIPS=softfloat`. Server-side rendering must be fast. |
| RAM | 64 MB (shared with OS) | ~35 MB available. Go runtime + GC eats ~5-10 MB. Stream image processing line-by-line. |
| Flash | 16 MB (10-12 MB free on overlay) | JSON data store lives here. Binary lives in tmpfs. |
| tmpfs | ~50 MB available | Binary deployed here, lost on reboot. Autostart re-fetches it. |
| USB | 1 port, occupied by printer | No USB storage. Binary must be fetched over network on boot. |
| Network | WiFi + Ethernet, address TBD | App configurable via `--host` flag. |

---

## Target Hardware

### GL-iNet GL-AR 150
- SoC: Atheros AR9331 (ar71xx family)
- OS: OpenWrt
- Kernel module needed: `kmod-usb-printer` → exposes `/dev/usb/lp0`

### Brother QL-700
- Connection: USB (printer class, `/dev/usb/lp0`)
- Resolution: 300 DPI
- Max print width: 720 pixels (62mm tape)
- Protocol: Brother QL Raster Command protocol
- Reference: https://download.brother.com/welcome/docp000678/cv_qlseries_eng_raster_600.pdf
- No printer driver needed — raw USB communication

---

## Build & Cross-Compilation

```bash
# Development (host machine)
go test ./...
go build -o qlx ./cmd/qlx/

# Production (MIPS target)
CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat \
  go build -trimpath -gcflags=all="-B" -ldflags="-s -w" \
  -o qlx-mips ./cmd/qlx/

# Optional: UPX compression (test on device — may not work on all MIPS)
upx -9 qlx-mips
```

**Binary size budget:**

| Component | Estimated Size |
|-----------|---------------|
| Go binary (stripped, no fonts) | ~5-6 MB |
| 2 subset fonts (embedded) | ~80 KB |
| HTMX + CSS (embedded) | ~30 KB |
| HTML templates (embedded) | ~10 KB |
| **Total without UPX** | **~5-6 MB** |
| **Total with UPX** | **~2-3 MB** |

---

## Deployment

Binary is deployed to tmpfs and re-fetched on every boot:

```sh
# /etc/rc.local on OpenWrt
wget -O /tmp/qlx http://<server>/qlx-mips
chmod +x /tmp/qlx
/tmp/qlx --device /dev/usb/lp0 --port 9100 --data /overlay/qlx/ &
```

Persistent data (JSON store) lives on flash overlay at `/overlay/qlx/data.json`.

---

## Architecture

```
┌──────────────────────────────────────────────┐
│  qlx on GL-AR 150                            │
│                                              │
│  ┌─────────────────────────────────────────┐ │
│  │  HTTP Server (net/http, Go 1.22+)       │ │
│  │                                         │ │
│  │  Content Negotiation:                   │ │
│  │  Accept: text/html  → html/template     │ │
│  │  Accept: application/json → JSON        │ │
│  │                                         │ │
│  │  Endpoints:                             │ │
│  │  GET  /                  containers     │ │
│  │  GET  /container/{id}    items list     │ │
│  │  POST /container         create         │ │
│  │  PUT  /container/{id}    update         │ │
│  │  DELETE /container/{id}  delete         │ │
│  │  GET  /item/{id}         detail         │ │
│  │  POST /item              create         │ │
│  │  PUT  /item/{id}         update         │ │
│  │  DELETE /item/{id}       delete         │ │
│  │  POST /item/{id}/print   print label    │ │
│  │  POST /print/bulk        bulk print     │ │
│  │  POST /api/print/raw     raw PNG print  │ │
│  │  GET  /api/status        printer status │ │
│  │  GET  /export/json       full export    │ │
│  │  GET  /export/csv        CSV export     │ │
│  └────────────┬────────────────────────────┘ │
│               │                              │
│  ┌────────────▼────────────────────────────┐ │
│  │  Store (JSON file + sync.RWMutex)       │ │
│  │  /overlay/qlx/data.json                 │ │
│  └────────────┬────────────────────────────┘ │
│               │                              │
│  ┌────────────▼────────────────────────────┐ │
│  │  Label Engine                           │ │
│  │  text rendering (golang.org/x/image)    │ │
│  │  hardcoded templates (3-4 types)        │ │
│  │  image → 1bpp raster + dithering        │ │
│  └────────────┬────────────────────────────┘ │
│               │                              │
│  ┌────────────▼────────────────────────────┐ │
│  │  Brother QL Protocol                    │ │
│  │  invalidate → init → status → media     │ │
│  │  → raster lines → print command         │ │
│  └────────────┬────────────────────────────┘ │
│               │                              │
│          /dev/usb/lp0                        │
└──────────────┼───────────────────────────────┘
               │
           🖨️ QL-700
```

---

## Data Model

### Container

```go
type Container struct {
    ID          string    `json:"id"`          // UUID v4
    ParentID    string    `json:"parent_id"`   // empty string = root
    Name        string    `json:"name"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
}
```

Containers are nested via `ParentID` (adjacency list). Depth is unlimited but realistically 3-4 levels (e.g. room → shelf → box). With hundreds of records, recursive tree traversal is fine — no need for materialized paths.

### Item

```go
type Item struct {
    ID          string    `json:"id"`           // UUID v4
    ContainerID string    `json:"container_id"` // FK to Container
    Name        string    `json:"name"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
}
```

### Store

```go
type Store struct {
    mu         sync.RWMutex
    path       string
    Containers map[string]*Container `json:"containers"`
    Items      map[string]*Item      `json:"items"`
}
```

- Entire store loaded into memory on startup (hundreds of records = ~50-100 KB)
- Written to disk on every mutation (atomic write via temp file + rename)
- `sync.RWMutex` for concurrent read access from HTTP handlers
- No external database dependencies

### Helper Methods Required

```go
func (s *Store) ContainerPath(id string) []Container    // returns [root, ..., leaf]
func (s *Store) ContainerChildren(id string) []Container // direct children
func (s *Store) ContainerItems(id string) []Item         // items in container
func (s *Store) MoveItem(itemID, newContainerID string) error
func (s *Store) MoveContainer(containerID, newParentID string) error // prevent cycles
```

---

## Brother QL-700 Raster Protocol

Implement from scratch. Do NOT use external crates/packages.

### Command Sequence

```
1. Invalidate    → 200 × 0x00 (flush printer buffer)
2. Initialize    → [0x1B, 0x40]
3. Status req    → [0x1B, 0x69, 0x53] → read 32 bytes response
4. Media info    → [0x1B, 0x69, 0x7A, ...] (tape type, width, length)
5. Auto-cut      → [0x1B, 0x69, 0x4D, flags]
6. Margins       → [0x1B, 0x69, 0x64, lo, hi]
7. Raster data   → [0x67, 0x00, 0x5A, <90 bytes>] × N lines
8. Print         → [0x1A] (print with feeding, last page)
                    [0x0C] (print without feeding, intermediate pages)
```

### Raster Format

- Each line = 90 bytes = 720 pixels (1 bit per pixel, monochrome)
- MSB first within each byte
- Bit 1 = black, Bit 0 = white
- Lines sent top-to-bottom

### Status Response (32 bytes)

| Byte | Meaning |
|------|---------|
| 0 | Print head mark (0x80) |
| 1 | Size (0x20) |
| 2-3 | Brother code ("B", 0x30) |
| 8 | Error info 1 (no media, cutter jam, etc.) |
| 9 | Error info 2 |
| 10 | Media width (mm) |
| 11 | Media type (0x0A = continuous, 0x0B = die-cut) |
| 17 | Media length (mm, 0 for continuous) |
| 18 | Status type (0x00 = reply, 0x01 = printing done, 0x02 = error, 0x06 = phase change) |
| 19 | Phase type |
| 21 | Notification number |

### Device Interface

```go
type Device interface {
    Write(data []byte) (int, error)
    Read(buf []byte) (int, error)
    Close() error
}

// Real implementation
type USBDevice struct {
    file *os.File // /dev/usb/lp0
}

// Test mock
type MockDevice struct {
    Written []byte
    ReadBuf []byte
}
```

### Auto-detect Media

On startup and before each print job, send status request. Parse response bytes 10-11 to determine loaded tape width and type. Validate that print data dimensions match loaded media.

---

## Label Templates

3-4 hardcoded templates. Each renders to an `image.Image` that gets converted to 1bpp raster.

### Available Fields

- `{{.Name}}` — item name
- `{{.Description}}` — item description
- `{{.ContainerPath}}` — full path string, e.g. "Szafa → Półka 3 → Pudełko A"

### Template Definitions

```go
type LabelTemplate struct {
    ID          string
    Name        string
    Description string
    Width       int // pixels, depends on tape
    Render      func(data LabelData, width int) *image.Gray
}

type LabelData struct {
    Name          string
    Description   string
    ContainerPath string
}
```

### Suggested Templates

1. **simple** — Name only, large font, centered. For quick labeling.
2. **standard** — Name (large) + container path (small) + description (small). Two or three lines.
3. **compact** — Name + container path on one line, smaller font. For narrow tapes.
4. **detailed** — All fields with separator lines. For wider tapes (62mm).

Template selection by user in print UI. Each template should gracefully handle missing fields (e.g. no description → skip that line, don't leave blank space).

---

## Frontend

### Stack

- **HTMX** (htmx.min.js ~14KB) — embedded via `go:embed`
- **html/template** — Go standard library, server-side rendering
- **Vanilla CSS** — no framework, minimal custom styles
- **Zero custom JavaScript** — all interactivity via HTMX attributes

### Content Negotiation

Every handler checks `Accept` header:

```go
func isJSON(r *http.Request) bool {
    return r.Header.Get("Accept") == "application/json"
}

func (s *Server) HandleItem(w http.ResponseWriter, r *http.Request) {
    item := s.store.GetItem(r.PathValue("id"))
    if item == nil {
        http.NotFound(w, r)
        return
    }
    if isJSON(r) {
        json.NewEncoder(w).Encode(item)
        return
    }
    s.render(w, "item.html", item)
}
```

Browser requests (via HTMX) get HTML fragments. `curl`/external apps with `Accept: application/json` get JSON. Same endpoints, no duplication.

### Navigation Pattern

HTMX swaps content into a `#content` div. No full page reloads.

```html
<!-- layout.html -->
<body>
  <nav>
    <a hx-get="/" hx-target="#content">QLX</a>
    <span id="printer-status"
          hx-get="/api/status" hx-trigger="every 3s"
          hx-swap="innerHTML"></span>
  </nav>
  <main id="content">
    {{ block "content" . }}{{ end }}
  </main>
  <script src="/static/htmx.min.js"></script>
</body>
```

### Key HTMX Interactions

```html
<!-- Container list → click to open -->
<div hx-get="/container/{{.ID}}" hx-target="#content">{{.Name}}</div>

<!-- Breadcrumb navigation -->
{{ range .Path }}
  <a hx-get="/container/{{.ID}}" hx-target="#content">{{.Name}}</a> →
{{ end }}

<!-- Create item form -->
<form hx-post="/item" hx-target="#content">
  <input name="name" required>
  <textarea name="description"></textarea>
  <input type="hidden" name="container_id" value="{{.ContainerID}}">
  <button type="submit">Dodaj</button>
</form>

<!-- Print preview (live, debounced) -->
<form hx-post="/item/{{.ID}}/preview" hx-trigger="change delay:300ms"
      hx-target="#preview">
  <select name="template">
    <option value="simple">Prosty</option>
    <option value="standard">Standardowy</option>
    <option value="detailed">Szczegółowy</option>
  </select>
</form>
<div id="preview"></div>  <!-- receives <img src="data:image/png;base64,..."> -->
<button hx-post="/item/{{.ID}}/print">🖨️ Drukuj</button>

<!-- Bulk print: checkboxes + single button -->
<form hx-post="/print/bulk" hx-target="#print-result">
  {{ range .Items }}
    <label>
      <input type="checkbox" name="item_ids" value="{{.ID}}">
      {{.Name}}
    </label>
  {{ end }}
  <select name="template">...</select>
  <button type="submit">Drukuj zaznaczone</button>
</form>
<div id="print-result"></div>
```

---

## Fonts

Two fonts, subset to Latin + Latin Extended (Polish characters), embedded via `go:embed`.

### Subsetting (build-time, requires `pyftsubset`)

```bash
# Inter — sans-serif, for labels and UI
pyftsubset Inter-Regular.ttf \
  --unicodes="U+0000-00FF,U+0100-017F,U+2000-206F,U+20AC" \
  --layout-features="kern,liga" \
  --no-hinting --desubroutinize \
  --output-file="fonts/inter-latin.ttf"

# JetBrains Mono — monospace, for serial numbers / codes
pyftsubset JetBrainsMono-Regular.ttf \
  --unicodes="U+0000-00FF,U+0100-017F" \
  --layout-features="kern,liga" \
  --no-hinting --desubroutinize \
  --output-file="fonts/jetbrains-mono-latin.ttf"
```

Unicode ranges:
- `U+0000-00FF` — Basic Latin + Latin-1 Supplement
- `U+0100-017F` — Latin Extended-A (Polish: ąćęłńóśźż ĄĆĘŁŃÓŚŹŻ)
- `U+2000-206F` — General Punctuation (em dash, quotes, etc.)
- `U+20AC` — Euro sign

### Embedding

```go
package embedded

import "embed"

//go:embed fonts/inter-latin.ttf
var InterFont []byte

//go:embed fonts/jetbrains-mono-latin.ttf
var JetBrainsMonoFont []byte

//go:embed static/*
var StaticFiles embed.FS

//go:embed templates/*
var Templates embed.FS
```

---

## Image Processing & Rasterization

### Pipeline

```
LabelData → Template.Render() → image.Gray (8bpp) → Floyd-Steinberg dither → 1bpp packed bytes → raster lines
```

### Key Implementation Details

1. **Text rendering**: Use `golang.org/x/image/font/opentype` to parse embedded TTF, `golang.org/x/image/font` for drawing. Render to `image.Gray`.

2. **Dithering**: Floyd-Steinberg dithering converts grayscale to black/white with perceived tonal range. Important for any anti-aliased text at small sizes.

3. **1bpp conversion**: Pack 8 pixels into 1 byte, MSB first. Each raster line = 90 bytes (720 pixels). If image is narrower than 720px, center it and pad with white (0x00 bytes).

4. **Memory efficiency**: Process image line-by-line where possible. On 64MB RAM device, a full 62mm × 100mm label at 300DPI = 720 × 1200 pixels × 1 byte = 864 KB. This is fine. But avoid holding multiple full images simultaneously.

5. **Rotation**: QL-700 prints portrait (720px wide). If label is landscape-oriented (e.g. 29mm tape), rotate 90°.

### Raw Print Mode

`POST /api/print/raw` accepts a PNG image directly. The server:
1. Decodes PNG
2. Converts to grayscale
3. Resizes to fit tape width (720px for 62mm, 554px for 29mm, etc.)
4. Dithers to 1bpp
5. Sends raster commands to printer

This endpoint always returns JSON (machine-to-machine).

---

## Export

### JSON Export

`GET /export/json` — returns the full store as-is:

```json
{
  "containers": { "uuid": { ... }, ... },
  "items": { "uuid": { ... }, ... }
}
```

### CSV Export

`GET /export/csv` — flattened items with container path:

```csv
item_id,item_name,item_description,container_path,created_at
uuid,Cable HDMI,2m black,Szafa → Półka 3 → Pudełko A,2025-01-15T10:30:00Z
```

---

## Runtime Configuration

```go
type Config struct {
    DevicePath string // --device, default "/dev/usb/lp0"
    Port       int    // --port, default 9100
    Host       string // --host, default "0.0.0.0"
    DataDir    string // --data, default "/overlay/qlx/"
    BaseURL    string // --base-url, default "http://<host>:<port>"
}
```

Parse with `flag` package (stdlib). No external config libraries.

---

## Memory & Performance

```go
func init() {
    debug.SetMemoryLimit(16 * 1024 * 1024) // 16 MB soft limit
    debug.SetGCPercent(20)                  // aggressive GC
}
```

- Print jobs are sequential (USB is synchronous). Use `sync.Mutex` on the printer device.
- Bulk print: iterate items, print one by one with auto-cut between labels.
- Status polling: HTMX `hx-trigger="every 3s"` — lightweight, returns ~100 bytes HTML fragment.

---

## Project Structure

```
qlx/
├── cmd/qlx/
│   └── main.go                    # entry point, flag parsing, wiring
├── internal/
│   ├── brother/
│   │   ├── protocol.go            # command builders (invalidate, init, raster line, etc.)
│   │   ├── protocol_test.go       # table-driven tests on byte sequences
│   │   ├── status.go              # parse 32-byte status response
│   │   ├── status_test.go         # known status bytes → expected structs
│   │   ├── media.go               # media types, widths, validation
│   │   ├── media_test.go
│   │   ├── device.go              # Device interface + USBDevice + MockDevice
│   │   └── printer.go             # high-level Printer (init, print job, status query)
│   ├── raster/
│   │   ├── convert.go             # image.Image → [][]byte (1bpp raster lines)
│   │   ├── convert_test.go        # pixel-perfect conversion tests
│   │   ├── dither.go              # Floyd-Steinberg dithering
│   │   └── dither_test.go
│   ├── label/
│   │   ├── templates.go           # template definitions (simple, standard, compact, detailed)
│   │   ├── templates_test.go      # render each template, verify output dimensions
│   │   ├── renderer.go            # LabelData → image.Gray using templates + fonts
│   │   └── renderer_test.go
│   ├── store/
│   │   ├── models.go              # Container, Item structs
│   │   ├── store.go               # JSON-backed CRUD, file I/O, mutex
│   │   └── store_test.go          # CRUD operations, tree traversal, cycle prevention
│   ├── web/
│   │   ├── server.go              # http.ServeMux setup, middleware, template loading
│   │   ├── handlers_containers.go # list, create, update, delete containers
│   │   ├── handlers_items.go      # list, create, update, delete items
│   │   ├── handlers_print.go      # print single, bulk, raw, preview
│   │   ├── handlers_export.go     # JSON + CSV export
│   │   ├── handlers_test.go       # httptest-based, test both HTML and JSON responses
│   │   └── respond.go             # content negotiation helper (HTML vs JSON)
│   └── embedded/
│       ├── fonts.go               # //go:embed fonts/*.ttf
│       └── static.go              # //go:embed static/* templates/*
├── templates/
│   ├── layout.html                # base layout with nav, #content, htmx.min.js
│   ├── containers.html            # container list (root or children of parent)
│   ├── container_form.html        # create/edit container
│   ├── items.html                 # item list within container + breadcrumbs
│   ├── item.html                  # item detail + print controls
│   ├── item_form.html             # create/edit item
│   ├── print_preview.html         # <img> with rendered label preview
│   ├── print_result.html          # success/error after printing
│   └── bulk_print.html            # checkbox list + template selector
├── static/
│   ├── htmx.min.js                # v2.x, ~14KB
│   └── style.css                  # minimal custom CSS
├── fonts/
│   ├── inter-latin.ttf            # subset, ~40-50 KB
│   └── jetbrains-mono-latin.ttf   # subset, ~30-40 KB
├── .github/
│   └── workflows/
│       └── ci.yml
├── Makefile
├── go.mod
├── LICENSE
├── README.md
└── agents.md                      # this file
```

---

## CI Pipeline

```yaml
name: CI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test ./... -v -race
      - run: go vet ./...

  build-mips:
    runs-on: ubuntu-latest
    needs: test
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Build for MIPS
        run: |
          CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat \
          go build -trimpath -gcflags=all="-B" -ldflags="-s -w" \
          -o qlx-mips ./cmd/qlx/
      - name: Report size
        run: ls -lh qlx-mips
      - uses: actions/upload-artifact@v4
        with:
          name: qlx-mips
          path: qlx-mips

  build-host:
    runs-on: ubuntu-latest
    needs: test
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Build for host
        run: go build -o qlx ./cmd/qlx/
      - uses: actions/upload-artifact@v4
        with:
          name: qlx-linux-amd64
          path: qlx
```

---

## Testing Strategy

### Unit Tests (no hardware needed)

All protocol, raster, and store logic is pure — no side effects, no I/O in tests.

**brother/protocol_test.go** — table-driven, verify exact byte sequences:
```go
func TestBuildInvalidate(t *testing.T) {
    data := BuildInvalidate()
    if len(data) != 200 {
        t.Errorf("invalidate length = %d, want 200", len(data))
    }
    for i, b := range data {
        if b != 0x00 {
            t.Errorf("byte %d = %02x, want 0x00", i, b)
        }
    }
}
```

**brother/status_test.go** — parse known status responses:
```go
func TestParseStatus(t *testing.T) {
    tests := []struct {
        name string
        raw  [32]byte
        want PrinterStatus
    }{
        {
            name: "ready with 62mm continuous",
            raw:  [32]byte{0x80, 0x20, 0x42, 0x30, ...},
            want: PrinterStatus{Ready: true, MediaWidth: 62, MediaType: Continuous},
        },
        {
            name: "no media error",
            raw:  [32]byte{0x80, 0x20, 0x42, 0x30, ..., 0x01, ...},
            want: PrinterStatus{Ready: false, Error: ErrNoMedia},
        },
    }
    // ...
}
```

**raster/convert_test.go** — pixel-perfect 1bpp conversion:
```go
func TestImageToRaster_BlackPixel(t *testing.T) {
    img := image.NewGray(image.Rect(0, 0, 720, 1))
    // Set first pixel to black
    img.SetGray(0, 0, color.Gray{0})
    
    lines := ToRasterLines(img)
    if lines[0][0]&0x80 == 0 {
        t.Error("first pixel should be black (MSB set)")
    }
}
```

**store/store_test.go** — CRUD + tree operations:
```go
func TestContainerPath(t *testing.T) {
    s := NewMemoryStore()
    root := s.CreateContainer("", "Room", "")
    shelf := s.CreateContainer(root.ID, "Shelf", "")
    box := s.CreateContainer(shelf.ID, "Box", "")
    
    path := s.ContainerPath(box.ID)
    if len(path) != 3 {
        t.Fatalf("path length = %d, want 3", len(path))
    }
    if path[0].Name != "Room" || path[2].Name != "Box" {
        t.Error("unexpected path order")
    }
}

func TestMoveContainer_PreventsCycle(t *testing.T) {
    s := NewMemoryStore()
    a := s.CreateContainer("", "A", "")
    b := s.CreateContainer(a.ID, "B", "")
    
    err := s.MoveContainer(a.ID, b.ID) // A under B, but B is under A
    if err == nil {
        t.Error("expected cycle detection error")
    }
}
```

**web/handlers_test.go** — httptest, test both HTML and JSON:
```go
func TestHandleItem_HTML(t *testing.T) {
    srv := newTestServer()
    item := srv.store.CreateItem(containerID, "Test", "Desc")
    
    req := httptest.NewRequest("GET", "/item/"+item.ID, nil)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    
    if w.Code != 200 {
        t.Errorf("status = %d", w.Code)
    }
    if !strings.Contains(w.Body.String(), "Test") {
        t.Error("response should contain item name")
    }
}

func TestHandleItem_JSON(t *testing.T) {
    srv := newTestServer()
    item := srv.store.CreateItem(containerID, "Test", "Desc")
    
    req := httptest.NewRequest("GET", "/item/"+item.ID, nil)
    req.Header.Set("Accept", "application/json")
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    
    var got Item
    json.Unmarshal(w.Body.Bytes(), &got)
    if got.Name != "Test" {
        t.Errorf("name = %q, want %q", got.Name, "Test")
    }
}
```

---

## Dependencies (go.mod)

Minimal. Prefer stdlib.

```
module github.com/<user>/qlx

go 1.22

require (
    golang.org/x/image v0.x.x    // font rendering (opentype, font.Drawer)
    github.com/google/uuid v1.x.x // UUID generation for IDs
)
```

That's it. No HTTP framework, no ORM, no config library.

- **HTTP**: `net/http` with Go 1.22 pattern matching (`GET /item/{id}`)
- **JSON**: `encoding/json`
- **Templates**: `html/template`
- **Images**: `image`, `image/png`, `image/color`, `image/draw`
- **CSV**: `encoding/csv`
- **Flags**: `flag`

---

## Implementation Order

Suggested order for incremental development with working software at each stage:

### Phase 1: Protocol Foundation
1. `internal/brother/protocol.go` + tests — command builders
2. `internal/brother/status.go` + tests — status parsing
3. `internal/brother/device.go` — Device interface + mock
4. `internal/brother/media.go` + tests — media types

### Phase 2: Raster Engine
5. `internal/raster/convert.go` + tests — image → 1bpp
6. `internal/raster/dither.go` + tests — Floyd-Steinberg

### Phase 3: Label Rendering
7. `internal/label/renderer.go` + tests — text → image.Gray
8. `internal/label/templates.go` + tests — hardcoded templates

### Phase 4: Store
9. `internal/store/models.go` — structs
10. `internal/store/store.go` + tests — JSON CRUD

### Phase 5: Web Layer
11. `internal/web/server.go` — mux setup
12. `internal/web/respond.go` — content negotiation
13. `internal/web/handlers_containers.go` + tests
14. `internal/web/handlers_items.go` + tests
15. `internal/web/handlers_print.go` + tests
16. `internal/web/handlers_export.go` + tests

### Phase 6: Templates & Static
17. `templates/*.html` — HTMX templates
18. `static/` — htmx.min.js + style.css
19. `internal/embedded/` — go:embed

### Phase 7: Integration
20. `cmd/qlx/main.go` — wire everything
21. `Makefile` — build targets
22. `.github/workflows/ci.yml`

### Phase 8: Polish
23. README.md with screenshots
24. Font subsetting in Makefile
25. Test on actual GL-AR 150 hardware
