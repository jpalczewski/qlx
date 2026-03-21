# Printer Backends — Design Spec

## Problem

QLX ma UI do drukowania etykiet ale brakuje całego backendu druku. System musi obsługiwać drukarki różnych firm (Brother, Niimbot) z różnymi protokołami, przez różne transporty (USB, Bluetooth, remote HTTP). Drukarki konfigurowane z WebUI, nie z CLI.

## Scope pierwszej iteracji

- **Brother QL-700** — USB, raster protocol ESC/P
- **Niimbot B1** — Bluetooth Serial (RFCOMM), pakietowy protokół binarny
- **Remote QLX** — HTTP do innej instancji QLX

## Architektura

```
┌──────────────────────────────────────────────────────────────┐
│  WebUI / API                                                  │
│  - konfiguracja drukarek (CRUD)                               │
│  - wybór drukarki + szablonu przy druku                       │
│  - podgląd etykiety przed drukiem                             │
├──────────────────────────────────────────────────────────────┤
│  PrintService          internal/print/service.go              │
│  - orchestruje: render → encode → transport                   │
│  - zarządza zarejestrowanymi drukarkami                       │
├──────────────────────────────────────────────────────────────┤
│  LabelRenderer         internal/print/label/                  │
│  - tekst → image.Image (Go stdlib image + freetype)           │
│  - QR code, barcode, grafiki                                  │
│  - szablony etykiet (simple, standard, compact, detailed)     │
├──────────────────────────────────────────────────────────────┤
│  Encoder (plugin)      internal/print/encoder/                │
│                        ├── brother/   Brother QL raster        │
│                        └── niimbot/   Niimbot packet protocol  │
│  image.Image → []byte device-specific                         │
├──────────────────────────────────────────────────────────────┤
│  Transport (plugin)    internal/print/transport/               │
│                        ├── usb.go     write to /dev/usb/lp0   │
│                        ├── serial.go  serial port (BT RFCOMM) │
│                        └── remote.go  HTTP POST to QLX        │
│  []byte → device                                              │
└──────────────────────────────────────────────────────────────┘
```

## Interfejsy

### Encoder

```go
// internal/print/encoder/encoder.go

type Encoder interface {
    // Name returns encoder identifier, e.g. "brother-ql", "niimbot"
    Name() string

    // Models returns list of supported printer models
    Models() []ModelInfo

    // Encode converts a rendered label image to device-specific bytes.
    // For bidirectional protocols (Niimbot), it writes/reads via transport directly.
    Encode(img image.Image, model string, opts PrintOpts, transport Transport) error
}

type ModelInfo struct {
    ID             string   // "QL-700", "B1"
    Name           string   // human-readable
    DPI            int
    PrintWidthPx   int      // printhead pixels
    MediaTypes     []string // "die-cut", "endless", "with-gaps", etc.
    DensityRange   [2]int   // min, max
    DensityDefault int
}

type PrintOpts struct {
    Density  int
    AutoCut  bool
    Quantity int
}
```

**Uwaga:** Niimbot wymaga bidirectional communication (wysyła pakiet → czeka na response). Dlatego `Encode` dostaje `Transport` — Brother pisze unidirectionally, Niimbot rozmawia.

### Transport

```go
// internal/print/transport/transport.go

type Transport interface {
    Name() string                    // "usb", "serial", "remote"
    Open(address string) error
    Write(data []byte) error
    Read(buf []byte) (int, error)    // for bidirectional (Niimbot)
    Close() error
}
```

### PrinterConfig (persisted in Store)

```go
// internal/print/printer.go

type PrinterConfig struct {
    ID        string `json:"id"`
    Name      string `json:"name"`        // "Brother kuchnia", "Niimbot biuro"
    Encoder   string `json:"encoder"`     // "brother-ql", "niimbot"
    Model     string `json:"model"`       // "QL-700", "B1"
    Transport string `json:"transport"`   // "usb", "serial", "remote"
    Address   string `json:"address"`     // "/dev/usb/lp0", "AA:BB:CC:DD", "http://192.168.1.5:8080"
}
```

### PrintService

```go
// internal/print/service.go

type PrintService struct {
    store     *store.Store
    encoders  map[string]Encoder     // "brother-ql" → BrotherEncoder
    transports map[string]Transport  // "usb" → USBTransport (factory)
}

func (s *PrintService) Print(printerID string, item *store.Item, templateName string) error
func (s *PrintService) ListPrinters() []PrinterConfig
func (s *PrintService) AddPrinter(cfg PrinterConfig) error
func (s *PrintService) RemovePrinter(id string) error
func (s *PrintService) TestPrinter(id string) error
```

## Brother QL-700 — Encoder

Reference: `pklaus/brother_ql` (Python), `DiUS/qlprint` (C), `vxel/brotherql` (Java)

### Protocol flow

```
1. Write 200x 0x00                    (clear buffer)
2. Write 0x1B 0x40                    (ESC @ = initialize)
3. Write 0x1B 0x69 0x7A + 10 bytes    (media/quality info)
     - flags, media_type, width_mm, length_mm
     - raster_lines (u32 LE), page_number, reserved
4. Write 0x1B 0x69 0x4D 0x40          (autocut on)
5. Write 0x1B 0x69 0x41 0x01          (cut every 1)
6. Write 0x1B 0x69 0x4B <flags>       (expanded mode: cut_at_end)
7. Write 0x1B 0x69 0x64 <u16 LE>     (margin in dots)
8. For each raster line:
     Write 0x67 0x00 0x5A + 90 bytes  (raster data, 720px, right-to-left)
9. Write 0x1A                          (print with feed, last page)
```

### QL-700 specifics
- `number_bytes_per_row`: 90 (720 pixels)
- No compression support
- No mode switching needed
- DPI: 300
- Status response: 32 bytes (readable via USB `read()`)
- Raster bits: right-to-left, 1 = print dot (black)
- Labels: 62mm endless, 29x90 die-cut, etc. (see `brother_ql/labels.py`)

### Image preparation
1. Scale image to label width (e.g. 696px printable for 62mm endless)
2. Pad to 720px (90 bytes) with offset_r
3. Convert to 1-bit monochrome
4. Flip left-right (raster format is right-to-left)
5. Pack 8 pixels per byte

## Niimbot B1 — Encoder

Reference: `AndBondStyle/niimprint` (Python), `MultiMote/niimbluelib` (TypeScript)

### B1 model info
- DPI: 203
- Printhead pixels: 384
- Print direction: "top" (no rotation needed)
- Paper types: WithGaps, Black, Transparent
- Density: 1-5, default 3
- Protocol variant: B1 print task

### Packet format

```
0x55 0x55 <type:u8> <len:u8> <data:bytes> <checksum:u8> 0xAA 0xAA

checksum = type XOR len XOR data[0] XOR data[1] XOR ...
```

Bidirectional — every command gets a response.

### Protocol flow (B1 print task, from community wiki)

```
1. SET_DENSITY (0x21)       → density:u8 (1-5, default 3)
2. SET_LABEL_TYPE (0x23)    → type:u8 (1=gaps, 2=black, 3=transparent)
3. PRINT_START (0x01)       → totalPages:u16 BE, 0x00 0x00 0x00, pageColor:u8 (7 bytes)
4. PAGE_START (0x03)        → 0x01
5. SET_PAGE_SIZE (0x13)     → rows:u16 BE, cols:u16 BE, copies:u16 BE (6 bytes)
6. For each row:
     PrintEmptyRow (0x84)        → rowNum:u16 BE, repeatCount:u8 (void rows)
     PrintBitmapRow (0x85)       → rowNum:u16 BE, blackPxCount:3 bytes, repeat:u8, bitmap
     PrintBitmapRowIndexed (0x83)→ rowNum:u16 BE, count:u8, repeat:u8, positions:u16 LE[]
                                   (used when <7 black pixels per row)
7. PAGE_END (0xE3)          → 0x01
8. Poll PRINT_STATUS (0xA3) → page:u16, progress1:u8, progress2:u8
9. PRINT_END (0xF3)         → 0x01, poll until response.data[0] == true
```

B1 nie wymaga PrinterCheckLine co 200 linii (w odróżnieniu od B21_V1).

### Image preparation
1. Image size must be width=384px (printhead pixels)
2. Convert to grayscale → invert → 1-bit
3. Pack 8 pixels per byte, MSB = leftmost pixel
4. Optimize: skip void rows (all white), compress repeated rows

## Niimbot — inne warianty (referencje na przyszłość)

| Print Task | Modele | Źródło |
|------------|--------|--------|
| D11_V1 | D11, D11S | `niimbluelib/src/print_tasks/OldD11PrintTask.ts` |
| D110 | B21S, D110, D11 v1/v2 | `niimbluelib/src/print_tasks/D110PrintTask.ts` |
| B1 | D110_M, B1, B21_C2B, M2_H, N1, D101 | `niimbluelib/src/print_tasks/B1PrintTask.ts` |
| B21_V1 | B21, B21_L2B | `niimbluelib/src/print_tasks/B21V1PrintTask.ts` |
| D110M_V4 | D110_M v4, D11_H, B21_PRO, B1_PRO | `niimbluelib/src/print_tasks/D110MV4PrintTask.ts` |

Kluczowe różnice między wariantami:
- Różne komendy i sekwencje inicjalizacji
- Różne formaty danych linii rastrowych
- Niektóre obsługują indexed pixels (0x83) zamiast full bitmap (0x85)
- Print direction: "left" (obraz rotowany 90° CW) vs "top"

Pełna baza modeli: `niimbluelib/src/printer_models.ts` (80+ modeli z DPI, printhead pixels, paper types, density ranges).

## Brother QL — inne modele (referencje na przyszłość)

| Model | bytes/row | Kompresja | Two-color | Źródło |
|-------|-----------|-----------|-----------|--------|
| QL-500/550/560/570 | 90 | nie | nie | `brother_ql/models.py` |
| QL-580N, QL-650TD | 90 | tak | nie | j.w. |
| QL-700 | 90 | nie | nie | **implementujemy** |
| QL-710W, QL-720NW | 90 | tak | nie | j.w. |
| QL-800/810W/820NWB | 90 | nie/tak/tak | tak (red) | j.w. |
| QL-1050/1060N/1100+ | 162 | tak | nie | j.w. |

Etykiety: `brother_ql/labels.py` — 30+ rozmiarów z `dots_total`, `dots_printable`, `offset_r`, `feed_margin`.

## Label Renderer

Wspólny dla wszystkich drukarek. Konwertuje dane itema + szablon → `image.Image`.

```go
// internal/print/label/renderer.go

type LabelData struct {
    Name        string
    Description string
    Location    string   // container path "Pokój → Półka → Pudełko"
    QRContent   string   // URL or text for QR code
    BarcodeID   string   // item ID for barcode
}

func Render(data LabelData, template string, widthPx, dpi int) (image.Image, error)
```

Szablony (hardcoded, Go stdlib `image/draw`):
- **simple** — tylko nazwa, duży font
- **standard** — nazwa + lokalizacja + QR
- **compact** — nazwa + opis, mały font
- **detailed** — nazwa + opis + lokalizacja + QR + barcode

Zależności Go:
- `image`, `image/draw`, `image/color` — stdlib
- QR code: `github.com/skip2/go-qrcode` (pure Go, no CGO)
- Barcode: `github.com/boombuler/barcode` (pure Go)
- Font rendering: `golang.org/x/image/font`, `golang.org/x/image/font/basicfont`

## Persistence — drukarki w Store

Rozszerzyć `store.Store` o `printers map[string]*PrinterConfig`:

```go
type storeData struct {
    Containers map[string]*Container     `json:"containers"`
    Items      map[string]*Item          `json:"items"`
    Printers   map[string]*PrinterConfig `json:"printers"`
}
```

CRUD: `AddPrinter`, `GetPrinter`, `UpdatePrinter`, `DeletePrinter`, `AllPrinters`.

## API & UI Endpoints

### API
```
GET    /api/printers                    — lista drukarek
POST   /api/printers                    — dodaj drukarkę
PUT    /api/printers/{id}               — edytuj
DELETE /api/printers/{id}               — usuń
POST   /api/printers/{id}/test          — test print
GET    /api/encoders                    — dostępne encodery + modele
POST   /api/items/{id}/print            — drukuj etykietę
```

### UI
```
GET    /ui/printers                     — strona konfiguracji drukarek
POST   /ui/actions/printers             — dodaj drukarkę (form)
DELETE /ui/actions/printers/{id}        — usuń drukarkę
POST   /ui/actions/items/{id}/print     — drukuj (wybór drukarki + szablonu)
```

## Remote QLX Transport

Instancja QLX z podpiętymi drukarkami eksponuje `POST /api/items/{id}/print`. Lokalna instancja wysyła tam request z `printer_id`, `template`, `item_data`.

Transport `remote` to HTTP client który:
1. `Write()` → `POST <address>/api/print` z body = encoded bytes
2. Sprawdza response status

## Weryfikacja

```bash
make test
make run

# 1. Dodać drukarkę w UI (/ui/printers)
# 2. Test print z /ui/printers
# 3. Drukuj etykietę z widoku itema (wybór drukarki + szablonu)
# 4. Sprawdź log w CLI — kolorowe logi z PrintService
```

## Pliki do stworzenia

```
internal/print/
├── service.go           — PrintService, orchestration
├── printer.go           — PrinterConfig model
├── encoder/
│   ├── encoder.go       — Encoder interface
│   ├── brother/
│   │   ├── brother.go   — BrotherEncoder
│   │   ├── models.go    — QL model definitions
│   │   └── labels.go    — label size definitions
│   └── niimbot/
│       ├── niimbot.go   — NiimbotEncoder
│       ├── packet.go    — packet format
│       └── models.go    — Niimbot model definitions
├── transport/
│   ├── transport.go     — Transport interface
│   ├── usb.go           — USB file writer
│   ├── serial.go        — Serial/BT RFCOMM
│   └── remote.go        — HTTP client to another QLX
└── label/
    ├── renderer.go      — label rendering
    └── templates.go     — template definitions
```

## Pliki do modyfikacji

```
internal/store/store.go      — dodać printers map + CRUD
internal/store/models.go     — dodać PrinterConfig
internal/api/server.go       — endpointy printer + print
internal/ui/server.go        — routes, templates
internal/ui/handlers.go      — printer UI handlers
internal/app/server.go       — inject PrintService
cmd/qlx/main.go              — create PrintService
```

## Multiplatform — USB i Serial na macOS / Windows / Linux

### USB Transport (Brother QL)

| OS | Ścieżka | Uwagi |
|---|---|---|
| Linux | `/dev/usb/lp0` | Kernel printer class driver, `open()` + `write()` |
| macOS | `/dev/usb/lp0` nie istnieje | Trzeba użyć libusb via CGO lub **przerobić na raw file write po odkryciu portu** |
| Windows | `\\.\USB001` lub libusb | WinUSB driver albo raw port |

**Rekomendacja:** Na MIPS (target) wystarczy `open("/dev/usb/lp0")`. Na macOS/Windows dodać transport `usb-libusb` w przyszłości. Dla v1 — USB transport = plik device (`os.OpenFile`), działa na Linuxie i macOS (jeśli driver załadowany).

### Serial/BT Transport (Niimbot)

| OS | Ścieżka | Uwagi |
|---|---|---|
| Linux | `/dev/rfcomm0` lub BT socket (`AF_BLUETOOTH` + `BTPROTO_RFCOMM`) | Socket wymaga CGO lub external bind |
| macOS | `/dev/tty.NiimbotB1-SerialPort` | Serial port po sparowaniu w System Preferences |
| Windows | `COM3` itp. | Serial port po sparowaniu w Bluetooth settings |

**Rekomendacja:** Użyć serial port (`/dev/tty.*`, `/dev/rfcomm*`, `COM*`) — działa cross-platform przez `os.OpenFile` z termios. Pakiet `go.bug.st/serial` (pure Go) obsługuje wszystkie 3 OS bez CGO.

### Zależność: `go.bug.st/serial`

Pure Go serial port library. Obsługuje Linux, macOS, Windows. Zero CGO. Idealne na MIPS i cross-compilation.

```go
import "go.bug.st/serial"

port, err := serial.Open("/dev/tty.NiimbotB1", &serial.Mode{BaudRate: 115200})
port.Write(packet)
port.Read(buf)
```

## Źródła i referencje protokołów

### Niimbot
- [NIIMBOT Community Wiki — Protocol](https://printers.niim.blue/interfacing/proto/) — pełna specyfikacja pakietów, komend, wariantów
- [NIIMBOT Community Wiki — Print Tasks](https://printers.niim.blue/interfacing/print-tasks/) — flow druku per model
- [MultiMote/niimbluelib](https://github.com/MultiMote/niimbluelib) — najkompletniejsza implementacja (TypeScript), 80+ modeli, 5 wariantów protokołu
- [AndBondStyle/niimprint](https://github.com/AndBondStyle/niimprint) — prosta Python implementacja (B1, B21, D11)

### Brother QL
- [Brother QL Series Command Reference (PDF)](https://download.brother.com/welcome/docp000678/cv_qlseries_eng_raster_600.pdf) — oficjalna specyfikacja raster protocol
- [pklaus/brother_ql](https://github.com/pklaus/brother_ql) — referencyjna Python implementacja, 17 modeli, 30+ etykiet
- [vxel/brotherql](https://github.com/vxel/brotherql) — Java implementacja z USB/TCP/file backends, two-color support
- [DiUS/qlprint](https://github.com/DiUS/qlprint) — C implementacja, minimalistyczna, dobra referencja binary protocol
