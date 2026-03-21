# Printer Backends Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add printing support to QLX — Brother QL-700 (USB), Niimbot B1 (Bluetooth serial), and remote QLX (HTTP), with printer management from WebUI.

**Architecture:** Plugin system with 4 layers: Transport (USB/serial/HTTP) → Encoder (Brother raster / Niimbot packets) → Label Renderer (image generation) → PrintService (orchestration). Printers persisted in Store, configured from WebUI.

**Tech Stack:** Go stdlib `image`, `go.bug.st/serial` (cross-platform serial), `github.com/skip2/go-qrcode`, `github.com/boombuler/barcode`

**Spec:** `docs/superpowers/specs/2026-03-21-printer-backends-design.md`

---

## File Structure

```
internal/print/
├── printer.go               — PrinterConfig model
├── service.go               — PrintService orchestration
├── service_test.go           — integration tests
├── encoder/
│   ├── encoder.go           — Encoder interface + registry
│   ├── brother/
│   │   ├── brother.go       — BrotherEncoder (raster protocol)
│   │   ├── brother_test.go  — raster encoding tests
│   │   ├── models.go        — QL model definitions
│   │   └── labels.go        — label size definitions
│   └── niimbot/
│       ├── niimbot.go       — NiimbotEncoder (B1 print task)
│       ├── niimbot_test.go  — packet + encoding tests
│       ├── packet.go        — packet format + checksum
│       ├── packet_test.go   — packet unit tests
│       └── models.go        — Niimbot model definitions
├── transport/
│   ├── transport.go         — Transport interface + registry
│   ├── file.go              — File transport (USB device files)
│   ├── serial.go            — Serial port transport (BT)
│   ├── remote.go            — HTTP transport to another QLX
│   └── mock.go              — Mock transport for tests
└── label/
    ├── renderer.go          — image rendering
    ├── renderer_test.go     — rendering tests
    └── templates.go         — label template definitions

Modify:
  internal/store/models.go     — add PrinterConfig
  internal/store/store.go      — add printers CRUD
  internal/store/store_test.go — printer persistence tests
  internal/api/server.go       — printer + print API endpoints
  internal/ui/server.go        — printer UI routes + templates
  internal/ui/handlers.go      — printer UI handlers
  internal/app/server.go       — inject PrintService
  cmd/qlx/main.go              — create PrintService
  go.mod                       — new dependencies
```

---

### Task 1: Transport interfaces + mock

**Files:**
- Create: `internal/print/transport/transport.go`
- Create: `internal/print/transport/mock.go`

- [ ] **Step 1: Write Transport interface**

```go
// internal/print/transport/transport.go
package transport

type Transport interface {
	Name() string
	Open(address string) error
	Write(data []byte) (int, error)
	Read(buf []byte) (int, error)
	Close() error
}
```

- [ ] **Step 2: Write MockTransport for testing**

```go
// internal/print/transport/mock.go
package transport

type MockTransport struct {
	Written  []byte
	ReadData []byte
	readPos  int
}

func (m *MockTransport) Name() string              { return "mock" }
func (m *MockTransport) Open(address string) error  { return nil }
func (m *MockTransport) Write(data []byte) (int, error) {
	m.Written = append(m.Written, data...)
	return len(data), nil
}
func (m *MockTransport) Read(buf []byte) (int, error) {
	n := copy(buf, m.ReadData[m.readPos:])
	m.readPos += n
	return n, nil
}
func (m *MockTransport) Close() error { return nil }

func (m *MockTransport) SetReadData(data []byte) {
	m.ReadData = data
	m.readPos = 0
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/print/transport/`

- [ ] **Step 4: Commit**

```
git add internal/print/transport/
git commit -m "feat(print): add Transport interface and mock"
```

---

### Task 2: File transport (USB) + serial transport

**Files:**
- Create: `internal/print/transport/file.go`
- Create: `internal/print/transport/serial.go`
- Create: `internal/print/transport/remote.go`
- Modify: `go.mod`

- [ ] **Step 1: Write FileTransport (for USB /dev/usb/lp0)**

```go
// internal/print/transport/file.go
package transport

import "os"

type FileTransport struct {
	f *os.File
}

func (t *FileTransport) Name() string { return "usb" }
func (t *FileTransport) Open(address string) error {
	f, err := os.OpenFile(address, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	t.f = f
	return nil
}
func (t *FileTransport) Write(data []byte) (int, error) { return t.f.Write(data) }
func (t *FileTransport) Read(buf []byte) (int, error)    { return t.f.Read(buf) }
func (t *FileTransport) Close() error                    { return t.f.Close() }
```

- [ ] **Step 2: Add go.bug.st/serial dependency**

Run: `go get go.bug.st/serial`

- [ ] **Step 3: Write SerialTransport (for Bluetooth RFCOMM)**

```go
// internal/print/transport/serial.go
package transport

import "go.bug.st/serial"

type SerialTransport struct {
	port serial.Port
}

func (t *SerialTransport) Name() string { return "serial" }
func (t *SerialTransport) Open(address string) error {
	port, err := serial.Open(address, &serial.Mode{BaudRate: 115200})
	if err != nil {
		return err
	}
	t.port = port
	return nil
}
func (t *SerialTransport) Write(data []byte) (int, error) { return t.port.Write(data) }
func (t *SerialTransport) Read(buf []byte) (int, error)    { return t.port.Read(buf) }
func (t *SerialTransport) Close() error                    { return t.port.Close() }
```

- [ ] **Step 4: Write RemoteTransport (HTTP to another QLX)**

```go
// internal/print/transport/remote.go
package transport

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

type RemoteTransport struct {
	address  string
	lastResp []byte
}

func (t *RemoteTransport) Name() string { return "remote" }
func (t *RemoteTransport) Open(address string) error {
	t.address = address
	return nil
}
func (t *RemoteTransport) Write(data []byte) (int, error) {
	resp, err := http.Post(t.address+"/api/print", "application/octet-stream", bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	t.lastResp, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("remote print failed: %s", resp.Status)
	}
	return len(data), nil
}
func (t *RemoteTransport) Read(buf []byte) (int, error) {
	return copy(buf, t.lastResp), nil
}
func (t *RemoteTransport) Close() error { return nil }
```

- [ ] **Step 5: Verify build**

Run: `go build ./internal/print/transport/`

- [ ] **Step 6: Commit**

```
git add internal/print/transport/ go.mod go.sum
git commit -m "feat(print): add USB, serial, and remote transports"
```

---

### Task 3: Niimbot packet format

**Files:**
- Create: `internal/print/encoder/niimbot/packet.go`
- Create: `internal/print/encoder/niimbot/packet_test.go`

- [ ] **Step 1: Write failing packet tests**

```go
// internal/print/encoder/niimbot/packet_test.go
package niimbot

import (
	"bytes"
	"testing"
)

func TestPacketToBytes(t *testing.T) {
	pkt := Packet{Type: 0x21, Data: []byte{0x03}}
	got := pkt.ToBytes()
	want := []byte{0x55, 0x55, 0x21, 0x01, 0x03, 0x23, 0xAA, 0xAA}
	// checksum: 0x21 ^ 0x01 ^ 0x03 = 0x23
	if !bytes.Equal(got, want) {
		t.Errorf("ToBytes() = %x, want %x", got, want)
	}
}

func TestPacketFromBytes(t *testing.T) {
	raw := []byte{0x55, 0x55, 0x21, 0x01, 0x03, 0x23, 0xAA, 0xAA}
	pkt, err := ParsePacket(raw)
	if err != nil {
		t.Fatalf("ParsePacket() error = %v", err)
	}
	if pkt.Type != 0x21 {
		t.Errorf("Type = %x, want 0x21", pkt.Type)
	}
	if !bytes.Equal(pkt.Data, []byte{0x03}) {
		t.Errorf("Data = %x, want [03]", pkt.Data)
	}
}

func TestPacketBadChecksum(t *testing.T) {
	raw := []byte{0x55, 0x55, 0x21, 0x01, 0x03, 0xFF, 0xAA, 0xAA}
	_, err := ParsePacket(raw)
	if err == nil {
		t.Error("expected checksum error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/print/encoder/niimbot/ -v`
Expected: FAIL (package doesn't exist yet)

- [ ] **Step 3: Implement Packet**

```go
// internal/print/encoder/niimbot/packet.go
package niimbot

import (
	"errors"
	"fmt"
)

var (
	packetHead = []byte{0x55, 0x55}
	packetTail = []byte{0xAA, 0xAA}
)

type Packet struct {
	Type byte
	Data []byte
}

func (p Packet) checksum() byte {
	cs := p.Type ^ byte(len(p.Data))
	for _, b := range p.Data {
		cs ^= b
	}
	return cs
}

func (p Packet) ToBytes() []byte {
	buf := make([]byte, 0, len(p.Data)+7)
	buf = append(buf, packetHead...)
	buf = append(buf, p.Type, byte(len(p.Data)))
	buf = append(buf, p.Data...)
	buf = append(buf, p.checksum())
	buf = append(buf, packetTail...)
	return buf
}

func ParsePacket(data []byte) (Packet, error) {
	if len(data) < 7 {
		return Packet{}, errors.New("packet too short")
	}
	if data[0] != 0x55 || data[1] != 0x55 {
		return Packet{}, errors.New("bad header")
	}
	if data[len(data)-2] != 0xAA || data[len(data)-1] != 0xAA {
		return Packet{}, errors.New("bad tail")
	}

	typ := data[2]
	dlen := int(data[3])
	if len(data) < dlen+7 {
		return Packet{}, fmt.Errorf("truncated: need %d, got %d", dlen+7, len(data))
	}

	pktData := make([]byte, dlen)
	copy(pktData, data[4:4+dlen])

	pkt := Packet{Type: typ, Data: pktData}
	if data[4+dlen] != pkt.checksum() {
		return Packet{}, fmt.Errorf("checksum mismatch: got %x, want %x", data[4+dlen], pkt.checksum())
	}
	return pkt, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/print/encoder/niimbot/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
git add internal/print/encoder/niimbot/
git commit -m "feat(niimbot): implement packet format with checksum"
```

---

### Task 4: Encoder interface + model definitions

**Files:**
- Create: `internal/print/encoder/encoder.go`
- Create: `internal/print/encoder/brother/models.go`
- Create: `internal/print/encoder/brother/labels.go`
- Create: `internal/print/encoder/niimbot/models.go`

- [ ] **Step 1: Write Encoder interface**

```go
// internal/print/encoder/encoder.go
package encoder

import (
	"image"

	"github.com/erxyi/qlx/internal/print/transport"
)

type Encoder interface {
	Name() string
	Models() []ModelInfo
	Encode(img image.Image, model string, opts PrintOpts, tr transport.Transport) error
}

type ModelInfo struct {
	ID             string
	Name           string
	DPI            int
	PrintWidthPx   int
	MediaTypes     []string
	DensityRange   [2]int
	DensityDefault int
}

type PrintOpts struct {
	Density  int
	AutoCut  bool
	Quantity int
}
```

- [ ] **Step 2: Write Brother model definitions (QL-700 only for v1)**

```go
// internal/print/encoder/brother/models.go
package brother

type qlModel struct {
	ID              string
	Name            string
	BytesPerRow     int
	MinLengthDots   int
	MaxLengthDots   int
	Compression     bool
	ModeSwitching   bool
	Cutting         bool
}

var ql700 = qlModel{
	ID:            "QL-700",
	Name:          "Brother QL-700",
	BytesPerRow:   90,
	MinLengthDots: 150,
	MaxLengthDots: 11811,
	Compression:   false,
	ModeSwitching: false,
	Cutting:       true,
}

var allModels = []qlModel{ql700}
```

- [ ] **Step 3: Write Brother label definitions**

```go
// internal/print/encoder/brother/labels.go
package brother

type labelDef struct {
	ID            string
	TapeWidthMm   int
	TapeLengthMm  int
	DotsPrintW    int
	DotsPrintL    int
	OffsetR       int
	FeedMargin    int
	MediaType     byte // 0x0A = continuous, 0x0B = die-cut
}

const (
	mediaContinuous byte = 0x0A
	mediaDieCut     byte = 0x0B
)

var allLabels = []labelDef{
	{"62", 62, 0, 696, 0, 12, 35, mediaContinuous},
	{"29", 29, 0, 306, 0, 6, 35, mediaContinuous},
	{"29x90", 29, 90, 306, 991, 6, 0, mediaDieCut},
	{"62x29", 62, 29, 696, 271, 12, 0, mediaDieCut},
	{"62x100", 62, 100, 696, 1109, 12, 0, mediaDieCut},
}
```

- [ ] **Step 4: Write Niimbot model definitions (B1 only for v1)**

```go
// internal/print/encoder/niimbot/models.go
package niimbot

type niimbotModel struct {
	ID             string
	Name           string
	DPI            int
	PrintheadPx    int
	DensityMin     int
	DensityMax     int
	DensityDefault int
}

var b1 = niimbotModel{
	ID:             "B1",
	Name:           "Niimbot B1",
	DPI:            203,
	PrintheadPx:    384,
	DensityMin:     1,
	DensityMax:     5,
	DensityDefault: 3,
}

var allModels = []niimbotModel{b1}
```

- [ ] **Step 5: Verify build**

Run: `go build ./internal/print/encoder/...`

- [ ] **Step 6: Commit**

```
git add internal/print/encoder/
git commit -m "feat(print): add Encoder interface, Brother QL-700 and Niimbot B1 model definitions"
```

---

### Task 5: Brother QL raster encoder

**Files:**
- Create: `internal/print/encoder/brother/brother.go`
- Create: `internal/print/encoder/brother/brother_test.go`

- [ ] **Step 1: Write failing test — raster output for tiny image**

```go
// internal/print/encoder/brother/brother_test.go
package brother

import (
	"image"
	"image/color"
	"testing"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

func TestBrotherEncode_StartsWithClear(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 720, 1))
	mock := &transport.MockTransport{}

	enc := &BrotherEncoder{}
	err := enc.Encode(img, "QL-700", encoder.PrintOpts{}, mock)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// First 200 bytes should be 0x00 (clear)
	for i := 0; i < 200; i++ {
		if mock.Written[i] != 0x00 {
			t.Fatalf("byte %d = %x, want 0x00", i, mock.Written[i])
		}
	}
	// Next 2 bytes: ESC @
	if mock.Written[200] != 0x1B || mock.Written[201] != 0x40 {
		t.Fatalf("init = %x %x, want 1B 40", mock.Written[200], mock.Written[201])
	}
}

func TestBrotherEncode_EndsWithPrint(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 720, 1))
	mock := &transport.MockTransport{}

	enc := &BrotherEncoder{}
	_ = enc.Encode(img, "QL-700", encoder.PrintOpts{AutoCut: true}, mock)

	// Last byte should be 0x1A (print with feed)
	last := mock.Written[len(mock.Written)-1]
	if last != 0x1A {
		t.Errorf("last byte = %x, want 0x1A", last)
	}
}

func TestBrotherEncode_RasterLine(t *testing.T) {
	// 720px wide, 1 row, all black
	img := image.NewGray(image.Rect(0, 0, 720, 1))
	for x := 0; x < 720; x++ {
		img.SetGray(x, 0, color.Gray{Y: 0}) // black
	}
	mock := &transport.MockTransport{}

	enc := &BrotherEncoder{}
	_ = enc.Encode(img, "QL-700", encoder.PrintOpts{}, mock)

	// Find raster command: 0x67 0x00 0x5A (90 bytes)
	found := false
	for i := 0; i < len(mock.Written)-2; i++ {
		if mock.Written[i] == 0x67 && mock.Written[i+1] == 0x00 && mock.Written[i+2] == 90 {
			found = true
			// All 90 data bytes should be 0xFF (all dots on, right-to-left)
			for j := 0; j < 90; j++ {
				if mock.Written[i+3+j] != 0xFF {
					t.Errorf("raster byte %d = %x, want 0xFF", j, mock.Written[i+3+j])
				}
			}
			break
		}
	}
	if !found {
		t.Error("raster command 0x67 not found")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/print/encoder/brother/ -v`

- [ ] **Step 3: Implement BrotherEncoder**

Implement `internal/print/encoder/brother/brother.go` with:
- `BrotherEncoder` struct implementing `encoder.Encoder`
- `Name()` returns `"brother-ql"`
- `Models()` returns `ModelInfo` for QL-700
- `Encode()` implements the full Brother QL raster protocol:
  1. Write 200x 0x00 (clear)
  2. Write ESC @ (init)
  3. Write media/quality info (ESC i z + 10 bytes)
  4. Write autocut settings
  5. Write expanded mode
  6. Write margins
  7. For each row: convert to 1-bit, flip left-right, pack into 90 bytes, write with 0x67 0x00 prefix
  8. Write 0x1A (print)

Reference: `ql.c` from DiUS/qlprint, `raster.py` from pklaus/brother_ql

- [ ] **Step 4: Run tests**

Run: `go test ./internal/print/encoder/brother/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
git add internal/print/encoder/brother/
git commit -m "feat(brother): implement QL-700 raster encoder"
```

---

### Task 6: Niimbot B1 encoder

**Files:**
- Create: `internal/print/encoder/niimbot/niimbot.go`
- Create: `internal/print/encoder/niimbot/niimbot_test.go`

- [ ] **Step 1: Write failing tests — B1 print flow**

Test that encoding a small image produces the correct packet sequence:
1. SET_DENSITY (0x21)
2. SET_LABEL_TYPE (0x23)
3. PRINT_START (0x01) with 7-byte payload
4. PAGE_START (0x03)
5. SET_PAGE_SIZE (0x13) with 6-byte payload
6. PrintBitmapRow (0x85) or PrintEmptyRow (0x84) per line
7. PAGE_END (0xE3)
8. PRINT_END (0xF3)

Mock transport should echo valid response packets for each command.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/print/encoder/niimbot/ -v`

- [ ] **Step 3: Implement NiimbotEncoder**

Implement `internal/print/encoder/niimbot/niimbot.go` with:
- `NiimbotEncoder` struct implementing `encoder.Encoder`
- `transceive()` helper: send packet, read response, validate
- `encodeImage()`: convert image to 1-bit rows, produce PrintBitmapRow/PrintEmptyRow packets
- `Encode()`: full B1 print flow per spec

Reference: `niimprint/printer.py` (Python), `niimbluelib/src/print_tasks/B1PrintTask.ts`

- [ ] **Step 4: Run tests**

Run: `go test ./internal/print/encoder/niimbot/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
git add internal/print/encoder/niimbot/
git commit -m "feat(niimbot): implement B1 encoder with packet protocol"
```

---

### Task 7: Label renderer

**Files:**
- Create: `internal/print/label/templates.go`
- Create: `internal/print/label/renderer.go`
- Create: `internal/print/label/renderer_test.go`
- Modify: `go.mod`

- [ ] **Step 1: Add dependencies**

Run: `go get github.com/skip2/go-qrcode github.com/boombuler/barcode golang.org/x/image`

- [ ] **Step 2: Write failing test — render simple template**

```go
// internal/print/label/renderer_test.go
package label

import "testing"

func TestRenderSimple(t *testing.T) {
	data := LabelData{Name: "Test Item"}
	img, err := Render(data, "simple", 384, 203)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
	if img.Bounds().Dy() == 0 {
		t.Error("height should be > 0")
	}
}

func TestRenderStandard_HasQR(t *testing.T) {
	data := LabelData{
		Name:      "HDMI Cable",
		Location:  "Room → Shelf → Box",
		QRContent: "https://qlx.local/item/123",
	}
	img, err := Render(data, "standard", 696, 300)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dx() != 696 {
		t.Errorf("width = %d, want 696", img.Bounds().Dx())
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/print/label/ -v`

- [ ] **Step 4: Implement LabelData, template definitions, and Render()**

`templates.go`: define LabelData struct and template names.
`renderer.go`: implement Render() using Go stdlib `image/draw`, `golang.org/x/image/font/basicfont` for text, `go-qrcode` for QR, `barcode` for barcodes.

Templates:
- `simple`: large text with name only
- `standard`: name + location + QR code
- `compact`: name + description in smaller font
- `detailed`: name + description + location + QR + barcode

- [ ] **Step 5: Run tests**

Run: `go test ./internal/print/label/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```
git add internal/print/label/ go.mod go.sum
git commit -m "feat(label): add label renderer with 4 templates (simple, standard, compact, detailed)"
```

---

### Task 8: PrinterConfig in Store

**Files:**
- Modify: `internal/store/models.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`

- [ ] **Step 1: Write failing test — printer CRUD**

Add to `store_test.go`:

```go
func TestPrinterCRUD(t *testing.T) {
	s := NewMemoryStore()

	p := s.AddPrinter("Brother kuchnia", "brother-ql", "QL-700", "usb", "/dev/usb/lp0")
	if p.ID == "" {
		t.Error("AddPrinter should set ID")
	}

	got := s.GetPrinter(p.ID)
	if got == nil || got.Name != "Brother kuchnia" {
		t.Error("GetPrinter failed")
	}

	all := s.AllPrinters()
	if len(all) != 1 {
		t.Errorf("AllPrinters count = %d, want 1", len(all))
	}

	err := s.DeletePrinter(p.ID)
	if err != nil {
		t.Fatalf("DeletePrinter error: %v", err)
	}
	if s.GetPrinter(p.ID) != nil {
		t.Error("printer should be deleted")
	}
}
```

- [ ] **Step 2: Run tests to verify it fails**

Run: `go test ./internal/store/ -run TestPrinterCRUD -v`

- [ ] **Step 3: Add PrinterConfig to models.go**

```go
type PrinterConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Encoder   string `json:"encoder"`
	Model     string `json:"model"`
	Transport string `json:"transport"`
	Address   string `json:"address"`
}
```

- [ ] **Step 4: Add printers map + CRUD to store.go**

Add `printers map[string]*PrinterConfig` to `Store` and `storeData`. Add methods: `AddPrinter`, `GetPrinter`, `DeletePrinter`, `AllPrinters`. Follow existing patterns from containers/items.

- [ ] **Step 5: Run all tests**

Run: `go test ./internal/store/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```
git add internal/store/
git commit -m "feat(store): add PrinterConfig persistence with CRUD"
```

---

### Task 9: PrintService

**Files:**
- Create: `internal/print/printer.go`
- Create: `internal/print/service.go`
- Create: `internal/print/service_test.go`

- [ ] **Step 1: Write PrintService**

Orchestrates: get printer config from store → create transport → open → get encoder → render label → encode → close.

```go
type PrintService struct {
	store    *store.Store
	encoders map[string]encoder.Encoder
}

func NewPrintService(s *store.Store) *PrintService
func (ps *PrintService) RegisterEncoder(enc encoder.Encoder)
func (ps *PrintService) Print(printerID string, data label.LabelData, templateName string) error
func (ps *PrintService) AvailableEncoders() []encoder.Encoder
```

- [ ] **Step 2: Write integration test with mock transport**

Test full flow: register encoder, add printer to store, call Print() with mock transport, verify data was written.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/print/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```
git add internal/print/
git commit -m "feat(print): add PrintService orchestrating render → encode → transport"
```

---

### Task 10: API endpoints for printers + print

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/app/server.go`
- Modify: `cmd/qlx/main.go`

- [ ] **Step 1: Add printer API routes**

```
GET    /api/printers           → list printers from store
POST   /api/printers           → add printer
DELETE /api/printers/{id}      → delete printer
GET    /api/encoders           → list available encoders + models
POST   /api/items/{id}/print   → print label (body: printer_id, template)
```

- [ ] **Step 2: Wire PrintService into app.Server**

Modify `app.NewServer` to accept `*print.PrintService`, pass it to `api.NewServer`.

- [ ] **Step 3: Create PrintService in main.go**

Register Brother and Niimbot encoders.

- [ ] **Step 4: Run all tests**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```
git add internal/api/ internal/app/ cmd/qlx/
git commit -m "feat(api): add printer management and print endpoints"
```

---

### Task 11: UI for printer management + print

**Files:**
- Create: `internal/embedded/templates/printers.html`
- Modify: `internal/ui/server.go`
- Modify: `internal/ui/handlers.go`
- Modify: `internal/embedded/templates/item.html`
- Modify: `internal/embedded/templates/layout.html`

- [ ] **Step 1: Create printers.html template**

Form to add printer (name, encoder select, model select, transport select, address input). List of configured printers with delete buttons.

- [ ] **Step 2: Add printer routes to UI server**

```
GET    /ui/printers                    → printer config page
POST   /ui/actions/printers            → add printer
DELETE /ui/actions/printers/{id}       → delete printer
POST   /ui/actions/items/{id}/print    → print label
```

- [ ] **Step 3: Update item.html — add printer selection to print form**

Replace hardcoded print form with dynamic printer selector + template selector.

- [ ] **Step 4: Add nav link to layout.html**

Add "Drukarki" link in nav bar.

- [ ] **Step 5: Run full app test**

Run: `make test && make run`
Verify: navigate to /ui/printers, add a printer, go to item detail, print form shows printer selection.

- [ ] **Step 6: Commit**

```
git add internal/embedded/ internal/ui/
git commit -m "feat(ui): add printer management page and print from item view"
```

---

### Task 12: End-to-end verification

- [ ] **Step 1: Run all tests**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: Run the app**

Run: `make run`

- [ ] **Step 3: Manual test flow**

1. Open http://localhost:8080/ui/printers
2. Add a printer (e.g. Brother QL-700, USB, /dev/usb/lp0)
3. Create a container + item
4. Open item detail
5. Select printer + template, click print
6. Verify CLI logs show print flow
7. If no physical printer: verify no crash, proper error message

- [ ] **Step 4: Final commit if any fixes needed**
