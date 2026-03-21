# Build Targets + BLE Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add conditional compilation with build tags so QLX builds for Mac M4 (full BLE+Serial+USB), MIPS (USB-only minimal), and dev (Serial+USB). Add BLE discovery for Niimbot printers on macOS.

**Architecture:** Go build tags (`ble`, `minimal`) with stub files for unsupported features. Factory functions route to real or stub transport. BLE via `tinygo.org/x/bluetooth` (CoreBluetooth on macOS).

**Tech Stack:** Go build tags, `tinygo.org/x/bluetooth`, CoreBluetooth (CGO on macOS)

**Spec:** `docs/superpowers/specs/2026-03-21-build-targets-bt-discovery.md`

---

## File Structure

```
Modify:
  internal/print/transport/serial.go        — add build tag !minimal
  internal/print/service.go                 — add BLE to transport factory
  internal/api/server.go                    — call registerBluetoothRoutes
  internal/embedded/templates/printers.html — BLE scan button + results
  Makefile                                  — new build targets
  go.mod                                    — tinygo.org/x/bluetooth

Create:
  internal/print/transport/serial_stub.go   — stub for minimal builds
  internal/print/transport/ble.go           — BLE transport (macOS, tag: ble)
  internal/print/transport/ble_stub.go      — BLE stub (tag: !ble)
  internal/api/bluetooth.go                 — BLE scan API handler (tag: ble)
  internal/api/bluetooth_stub.go            — stub handler (tag: !ble)
```

---

### Task 1: Serial build tags (minimal vs full)

**Files:**
- Modify: `internal/print/transport/serial.go` — add `//go:build !minimal`
- Create: `internal/print/transport/serial_stub.go` — stub with `//go:build minimal`

- [ ] **Step 1: Add build tag to serial.go**

Add `//go:build !minimal` as first line (before package declaration) of `internal/print/transport/serial.go`.

- [ ] **Step 2: Create serial_stub.go**

```go
//go:build minimal

package transport

import "errors"

type SerialTransport struct{}

func (t *SerialTransport) Name() string              { return "serial" }
func (t *SerialTransport) Open(address string) error  { return errors.New("serial not supported in minimal build") }
func (t *SerialTransport) Write(data []byte) (int, error) { return 0, errors.New("serial not supported") }
func (t *SerialTransport) Read(buf []byte) (int, error)   { return 0, errors.New("serial not supported") }
func (t *SerialTransport) Close() error              { return nil }
```

- [ ] **Step 3: Verify both builds**

```bash
go build ./internal/print/transport/
go build -tags minimal ./internal/print/transport/
```

- [ ] **Step 4: Commit**

```bash
git add internal/print/transport/serial*.go
git commit -m "feat(build): add minimal build tag to exclude serial transport"
```

---

### Task 2: BLE transport + scan stubs (non-BLE builds)

**Files:**
- Create: `internal/print/transport/ble_stub.go`

- [ ] **Step 1: Create BLE stub with transport + scan**

```go
//go:build !ble

package transport

import "errors"

type BLETransport struct{}

func (t *BLETransport) Name() string              { return "ble" }
func (t *BLETransport) Open(address string) error  { return errors.New("BLE not supported in this build") }
func (t *BLETransport) Write(data []byte) (int, error) { return 0, errors.New("BLE not supported") }
func (t *BLETransport) Read(buf []byte) (int, error)   { return 0, errors.New("BLE not supported") }
func (t *BLETransport) Close() error              { return nil }

type BLEScanResult struct {
    Address string `json:"address"`
    Name    string `json:"name"`
    RSSI    int    `json:"rssi"`
}

func ScanBLE() ([]BLEScanResult, error) {
    return nil, errors.New("BLE scanning not supported in this build")
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/print/transport/
```

- [ ] **Step 3: Commit**

```bash
git add internal/print/transport/ble_stub.go
git commit -m "feat(build): add BLE transport and scan stubs for non-BLE builds"
```

---

### Task 3: BLE transport real implementation (macOS)

**Files:**
- Create: `internal/print/transport/ble.go`
- Modify: `go.mod`

- [ ] **Step 1: Add tinygo bluetooth dependency**

```bash
go get tinygo.org/x/bluetooth
```

- [ ] **Step 2: Create BLE transport + scan**

Create `internal/print/transport/ble.go` with `//go:build ble` tag containing:
- `BLETransport` struct implementing Transport interface via `tinygo.org/x/bluetooth`
- Niimbot BLE service UUID: `e7810a71-73ae-499d-8c15-faa9aef0c3f2`
- `Open()`: enable adapter, connect to device by address, discover service, find characteristic with notify+writeWithoutResponse, enable notifications
- `Write()`: write to characteristic without response
- `Read()`: read from notification buffer (populated by callback), poll with timeout
- `Close()`: disconnect device
- `BLEScanResult` struct (same as stub)
- `ScanBLE()`: enable adapter, scan for 5s, filter by Niimbot name prefixes (B,D,A,H,N,C,K,S,P,T,M,E), return results

- [ ] **Step 3: Verify BLE build on macOS**

```bash
CGO_ENABLED=1 go build -tags ble ./internal/print/transport/
```

- [ ] **Step 4: Commit**

```bash
git add internal/print/transport/ble.go go.mod go.sum
git commit -m "feat(ble): add BLE transport and discovery via CoreBluetooth"
```

---

### Task 4: Update PrintService + API for BLE

**Files:**
- Modify: `internal/print/service.go` — add BLE to transport factory
- Create: `internal/api/bluetooth.go` (tag: ble)
- Create: `internal/api/bluetooth_stub.go` (tag: !ble)
- Modify: `internal/api/server.go` — call registerBluetoothRoutes

- [ ] **Step 1: Add BLE to transport factory in service.go**

Add `case "ble": return &transport.BLETransport{}` to the transport factory switch.

- [ ] **Step 2: Create bluetooth.go (ble builds)**

```go
//go:build ble

package api

import (
    "net/http"
    "github.com/erxyi/qlx/internal/print/transport"
    "github.com/erxyi/qlx/internal/shared/webutil"
)

func (s *Server) HandleBluetoothScan(w http.ResponseWriter, r *http.Request) {
    results, err := transport.ScanBLE()
    if err != nil {
        webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
        return
    }
    webutil.JSON(w, http.StatusOK, results)
}

func (s *Server) registerBluetoothRoutes(mux *http.ServeMux) {
    mux.HandleFunc("GET /api/bluetooth/scan", s.HandleBluetoothScan)
}
```

- [ ] **Step 3: Create bluetooth_stub.go (non-ble builds)**

```go
//go:build !ble

package api

import "net/http"

func (s *Server) registerBluetoothRoutes(mux *http.ServeMux) {}
```

- [ ] **Step 4: Add registerBluetoothRoutes call to RegisterRoutes in server.go**

Add `s.registerBluetoothRoutes(mux)` at the end of `RegisterRoutes`.

- [ ] **Step 5: Verify all builds**

```bash
go build ./...
go build -tags minimal ./...
go build -tags ble ./...  # macOS only
```

- [ ] **Step 6: Commit**

```bash
git add internal/print/service.go internal/api/bluetooth*.go internal/api/server.go
git commit -m "feat(api): add BLE scan endpoint and transport factory support"
```

---

### Task 5: UI BLE scan + Makefile

**Files:**
- Modify: `internal/embedded/templates/printers.html`
- Modify: `Makefile`

- [ ] **Step 1: Add BLE scan button to printers.html**

Add a scan section before the "Dodaj drukarkę" details. Use a button that calls `fetch('/api/bluetooth/scan')`, displays results as a list using safe DOM manipulation (createElement/textContent, not innerHTML), and on click auto-fills the add-printer form fields (name, encoder=niimbot, transport=ble, address).

- [ ] **Step 2: Update Makefile**

```makefile
.PHONY: build build-mac build-mips test test-ble run clean deps

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

run:
	go run ./cmd/qlx/ --port 8080 --data ./data

clean:
	rm -f qlx qlx-darwin qlx-mips

deps:
	go mod download
	go mod tidy
```

- [ ] **Step 3: Verify all targets**

```bash
make build
make build-mips
make test
```

- [ ] **Step 4: Commit**

```bash
git add internal/embedded/templates/printers.html Makefile
git commit -m "feat(ui/build): add BLE scan UI and Mac/MIPS/dev Makefile targets"
```

---

### Task 6: End-to-end verification

- [ ] **Step 1: Run all tests**

```bash
make test
```

- [ ] **Step 2: Test default build**

```bash
make build && ./qlx --port 8080 --data ./data
# Open /ui/printers — BLE scan button shows, clicking returns "not supported"
```

- [ ] **Step 3: Test MIPS build**

```bash
make build-mips && file qlx-mips
# → ELF 32-bit MSB, MIPS
```

- [ ] **Step 4: Test Mac build (if on macOS with XCode)**

```bash
make build-mac && ./qlx-darwin --port 8080 --data ./data
# Open /ui/printers → click "Skanuj Bluetooth" → should scan for 5s
```

- [ ] **Step 5: Final commit if fixes needed**
