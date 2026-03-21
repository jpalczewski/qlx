# Build Targets + BLE Discovery — Design Spec

## Problem

QLX musi budować się na różne platformy z różnym zestawem funkcji:
- **Mac M4 (full)**: pełne BLE discovery + scan Niimbotów + USB Serial + Brother USB
- **MIPS Linux (minimal)**: tylko Brother USB, bez BT, bez serial, minimalne zależności

Niimbot drukarki używają **BLE** (Bluetooth Low Energy), nie klasycznego RFCOMM. Na macOS discovery wymaga CoreBluetooth (CGO).

## Build targets

| Target | GOOS/GOARCH | CGO | Transports | Funkcje |
|--------|-------------|-----|------------|---------|
| `qlx-darwin` | darwin/arm64 | **yes** | USB file, Serial, BLE, Remote | Pełne: print + BLE discovery + scan |
| `qlx-mips` | linux/mips | **no** | USB file, Remote | Minimalne: Brother USB + remote only |
| `qlx` (dev) | native | auto | wszystkie dostępne | Development build |

## Build tags

Użyć Go build tags do warunkowej kompilacji:

```
//go:build ble        → pliki z CoreBluetooth/BLE
//go:build !ble       → stub/fallback bez BLE
//go:build !minimal   → serial transport (go.bug.st/serial)
//go:build minimal    → bez serial, bez BLE
```

| Tag | darwin full | mips minimal |
|-----|-------------|--------------|
| `ble` | tak | nie |
| `minimal` | nie | tak |

### Jak to wpływa na transport

```
internal/print/transport/
├── transport.go         — interfejs (zawsze)
├── file.go              — USB device file (zawsze)
├── remote.go            — HTTP remote (zawsze)
├── serial.go            — Serial port, build tag: !minimal
├── serial_stub.go       — stub, build tag: minimal
├── ble.go               — BLE via CoreBluetooth, build tag: ble
├── ble_stub.go           — stub, build tag: !ble
└── mock.go              — test mock (zawsze)
```

### BLE transport (macOS only)

Biblioteka: `tinygo.org/x/bluetooth` — cross-platform Go BLE, macOS via CoreBluetooth (CGO).

Niimbot BLE service UUID: `e7810a71-73ae-499d-8c15-faa9aef0c3f2`

```go
//go:build ble

type BLETransport struct {
    adapter    *bluetooth.Adapter
    device     bluetooth.Device
    char       bluetooth.DeviceCharacteristic
    recvBuf    []byte
}

func (t *BLETransport) Name() string { return "ble" }
func (t *BLETransport) Open(address string) error {
    // 1. Enable adapter
    // 2. Connect to device by address (MAC or UUID)
    // 3. Discover service e7810a71-...
    // 4. Find characteristic with notify + writeWithoutResponse
    // 5. Enable notifications, buffer incoming data
}
```

### BLE Discovery endpoint

```
GET /api/bluetooth/scan    — skanuj BLE urządzenia, zwróć listę Niimbotów
```

Skan trwa ~5s. Filtruje po name prefix (B, D, A, H, etc. — pierwsze litery modeli Niimbot). Zwraca:

```json
[
  {"address": "uuid-or-mac", "name": "B1_1234", "rssi": -45},
  {"address": "uuid-or-mac", "name": "D110_5678", "rssi": -62}
]
```

Na macOS adresy to UUID (CoreBluetooth ukrywa MAC). Użytkownik w UI widzi listę znalezionych drukarek i może jednym kliknięciem dodać ją jako printer config z transport="ble".

### Stub pliki

```go
//go:build !ble

type BLETransport struct{}

func (t *BLETransport) Name() string              { return "ble" }
func (t *BLETransport) Open(address string) error  { return errors.New("BLE not supported in this build") }
func (t *BLETransport) Write(data []byte) (int, error) { return 0, errors.New("BLE not supported") }
func (t *BLETransport) Read(buf []byte) (int, error)   { return 0, errors.New("BLE not supported") }
func (t *BLETransport) Close() error              { return nil }
```

Analogicznie `serial_stub.go` z `//go:build minimal`.

## Makefile

```makefile
build:
	go build -o qlx ./cmd/qlx/

build-mac:
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
	  go build -tags ble -o qlx-darwin ./cmd/qlx/

build-mips:
	CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat \
	  go build -tags minimal -trimpath -gcflags=all="-B" -ldflags="-s -w" \
	  -o qlx-mips ./cmd/qlx/

test:
	go test ./... -v

test-ble:
	go test -tags ble ./... -v
```

## PrintService — transport factory z build tags

`createTransport` w service.go musi uwzględniać dostępność:

```go
func defaultTransportFactory(name string) transport.Transport {
    switch name {
    case "usb":
        return &transport.FileTransport{}
    case "serial":
        return transport.NewSerial()    // zwraca stub lub real
    case "ble":
        return transport.NewBLE()       // zwraca stub lub real
    case "remote":
        return &transport.RemoteTransport{}
    }
    return nil
}
```

`transport.NewSerial()` i `transport.NewBLE()` to factory functions zdefiniowane w plikach z build tags — real impl albo stub.

## UI — BLE scan

Na stronie printers.html, jeśli build ma BLE:

```html
<button onclick="scanBLE()">🔍 Skanuj Bluetooth</button>
<div id="ble-results"></div>
```

JS robi `fetch("/api/bluetooth/scan")`, wyświetla wyniki, klik na wynik = auto-fill formularza dodawania drukarki.

API endpoint `/api/bluetooth/scan` istnieje tylko w buildach z tagiem `ble`. W minimalnym buildzie zwraca 404.

## Pliki do stworzenia / modyfikacji

```
Nowe:
  internal/print/transport/ble.go          — BLE transport (build tag: ble)
  internal/print/transport/ble_stub.go     — BLE stub (build tag: !ble)
  internal/print/transport/serial_stub.go  — Serial stub (build tag: minimal)
  internal/print/transport/factory.go      — NewSerial(), NewBLE() factories
  internal/print/transport/factory_minimal.go — minimal stubs

Modyfikowane:
  internal/print/transport/serial.go       — dodać build tag: !minimal
  internal/print/service.go                — użyć factory functions
  internal/api/server.go                   — dodać /api/bluetooth/scan (conditional)
  internal/ui/handlers.go                  — BLE scan handler (conditional)
  internal/embedded/templates/printers.html — BLE scan button
  Makefile                                 — nowe targety
  go.mod                                   — tinygo.org/x/bluetooth (optional dep)
```

## Weryfikacja

```bash
# Mac full build
make build-mac
./qlx-darwin --port 8080 --data ./data
# → BLE scan działa, serial działa, USB działa

# MIPS minimal build
make build-mips
file qlx-mips  # → ELF 32-bit MSB, MIPS
# → tylko USB + remote, brak BLE/serial

# Dev build (no tags)
make build
# → USB + serial + remote, bez BLE

# Tests
make test       # → bez BLE testów
make test-ble   # → z BLE testami (tylko na macOS)
```

## Źródła

- [tinygo-org/bluetooth](https://github.com/tinygo-org/bluetooth) — Go BLE library, macOS via CoreBluetooth
- [niimbluelib bluetooth_impl.ts](https://github.com/MultiMote/niimbluelib) — Niimbot BLE service UUID + GATT patterns
- [Niimbot BLE protocol](https://printers.niim.blue/interfacing/proto/) — community wiki
