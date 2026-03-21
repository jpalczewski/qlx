# Printer Status + Heartbeat Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persistent printer connections with heartbeat polling, RFID tape info, live SSE status updates in navbar and printer cards.

**Architecture:** PrinterManager replaces PrintService, maintains PrinterSession per printer with heartbeat goroutine. SSE pushes status to browser. Mutex serializes heartbeat vs print.

**Tech Stack:** Go goroutines, SSE (Server-Sent Events), existing BLE/serial transports

**Spec:** `docs/superpowers/specs/2026-03-21-printer-status-heartbeat.md`

---

## File Structure

```
Create:
  internal/print/status.go            — PrinterStatus model
  internal/print/session.go           — PrinterSession (transport + heartbeat goroutine)
  internal/print/manager.go           — PrinterManager (replaces PrintService)
  internal/print/manager_test.go      — integration tests
  internal/print/encoder/niimbot/info.go — Heartbeat, GetInfo, RfidInfo commands

Modify:
  internal/print/encoder/encoder.go    — add StatusQuerier interface
  internal/print/service.go            — remove (replaced by manager)
  internal/print/service_test.go       — remove (replaced by manager_test)
  internal/api/server.go               — SSE endpoint, use PrinterManager
  internal/ui/server.go                — use PrinterManager
  internal/ui/handlers.go              — status handlers
  internal/app/server.go               — inject PrinterManager
  cmd/qlx/main.go                      — create PrinterManager
  internal/embedded/templates/printers.html — live status cards
  internal/embedded/templates/layout.html   — navbar printer status
  internal/embedded/static/ui-lite.js       — SSE listener + DOM updates
```

---

### Task 1: PrinterStatus model + StatusQuerier interface

**Files:**
- Create: `internal/print/status.go`
- Modify: `internal/print/encoder/encoder.go`

- [ ] **Step 1: Create status.go**

```go
package print

import "time"

type PrinterStatus struct {
    Connected   bool      `json:"connected"`
    Battery     int       `json:"battery"`      // 0-100, -1 if unknown
    LidClosed   bool      `json:"lid_closed"`
    PaperLoaded bool      `json:"paper_loaded"`
    LabelType   string    `json:"label_type"`   // "with-gaps", etc.
    TotalLabels int       `json:"total_labels"` // from RFID, -1 if unknown
    UsedLabels  int       `json:"used_labels"`
    LastError   string    `json:"last_error"`
    LastUpdated time.Time `json:"last_updated"`
}
```

- [ ] **Step 2: Add StatusQuerier interface to encoder.go**

```go
// StatusQuerier is optionally implemented by encoders that support status queries.
type StatusQuerier interface {
    // Connect sends initial handshake and reads printer info.
    Connect(tr transport.Transport) error
    // Heartbeat reads current printer status.
    Heartbeat(tr transport.Transport) (HeartbeatResult, error)
    // RfidInfo reads tape/label RFID data.
    RfidInfo(tr transport.Transport) (RfidResult, error)
}

type HeartbeatResult struct {
    Battery     int
    LidClosed   bool
    PaperLoaded bool
}

type RfidResult struct {
    LabelType   string
    TotalLabels int
    UsedLabels  int
}
```

- [ ] **Step 3: Verify build**
- [ ] **Step 4: Commit**

---

### Task 2: Niimbot info commands (Heartbeat, RfidInfo)

**Files:**
- Create: `internal/print/encoder/niimbot/info.go`
- Create: `internal/print/encoder/niimbot/info_test.go`

- [ ] **Step 1: Write tests for Heartbeat and RfidInfo**

Mock transport returns proper heartbeat response (13 bytes for B1) and RFID response. Test that parsed HeartbeatResult has correct battery/lid/paper values.

- [ ] **Step 2: Implement info.go**

NiimbotEncoder implements StatusQuerier:
- `Connect(tr)` — send 0xC1, read response
- `Heartbeat(tr)` — send 0xDC with data [0x01], parse 13-byte response (B1 format): skip 9, lidClosed=byte[9]==0, chargeLevel=byte[10], paperInserted=byte[11]==0, rfidSuccess=byte[12]
- `RfidInfo(tr)` — send 0x1A with data [0x01], parse response: if data[0]==0 no tag, else parse uuid(8), barcode(vstring), serial(vstring), totalPaper(u16), usedPaper(u16), type(u8)

Use existing `transceiveWithResponse` and `readOnePacket` from niimbot.go.

- [ ] **Step 3: Run tests**
- [ ] **Step 4: Commit**

---

### Task 3: PrinterSession (persistent connection + heartbeat)

**Files:**
- Create: `internal/print/session.go`

- [ ] **Step 1: Implement PrinterSession**

```go
type PrinterSession struct {
    config    store.PrinterConfig
    transport transport.Transport
    enc       encoder.Encoder
    querier   encoder.StatusQuerier // nil if encoder doesn't support it
    status    PrinterStatus
    mu        sync.Mutex
    stop      chan struct{}
    onUpdate  func(printerID string, status PrinterStatus)
}

func NewSession(cfg store.PrinterConfig, tr transport.Transport, enc encoder.Encoder, onUpdate func(string, PrinterStatus)) *PrinterSession

func (s *PrinterSession) Start() error    // open transport, connect, read RFID, start heartbeat goroutine
func (s *PrinterSession) Stop()           // stop heartbeat, close transport
func (s *PrinterSession) Status() PrinterStatus
func (s *PrinterSession) Print(img image.Image, model string, opts encoder.PrintOpts) error  // lock mutex, encode, unlock
```

Heartbeat goroutine:
```go
func (s *PrinterSession) heartbeatLoop() {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-s.stop:
            return
        case <-ticker.C:
            s.mu.Lock()
            result, err := s.querier.Heartbeat(s.transport)
            s.mu.Unlock()
            if err != nil {
                // mark disconnected, try reconnect
            } else {
                // update status, call onUpdate
            }
        }
    }
}
```

- [ ] **Step 2: Verify build**
- [ ] **Step 3: Commit**

---

### Task 4: PrinterManager (replaces PrintService)

**Files:**
- Create: `internal/print/manager.go`
- Create: `internal/print/manager_test.go`
- Remove: `internal/print/service.go`, `internal/print/service_test.go`

- [ ] **Step 1: Implement PrinterManager**

```go
type PrinterStatusEvent struct {
    PrinterID string        `json:"printer_id"`
    Status    PrinterStatus `json:"status"`
}

type PrinterManager struct {
    store       *store.Store
    encoders    map[string]encoder.Encoder
    sessions    map[string]*PrinterSession
    mu          sync.RWMutex
    sseMu       sync.Mutex
    sseClients  map[chan PrinterStatusEvent]struct{}
    transportFn TransportFactory
}

func NewPrinterManager(s *store.Store) *PrinterManager
func (m *PrinterManager) RegisterEncoder(enc encoder.Encoder)
func (m *PrinterManager) AvailableEncoders() map[string]encoder.Encoder
func (m *PrinterManager) Start()                    // connect all printers from store
func (m *PrinterManager) Stop()                     // disconnect all
func (m *PrinterManager) ConnectPrinter(id string) error
func (m *PrinterManager) DisconnectPrinter(id string)
func (m *PrinterManager) GetStatus(id string) PrinterStatus
func (m *PrinterManager) AllStatuses() map[string]PrinterStatus
func (m *PrinterManager) Print(printerID string, data label.LabelData, template string) error
func (m *PrinterManager) SubscribeSSE() chan PrinterStatusEvent
func (m *PrinterManager) UnsubscribeSSE(ch chan PrinterStatusEvent)
```

Print() renders label, then calls session.Print() (uses existing open transport).

SSE: when session calls onUpdate, manager broadcasts to all sseClients channels.

- [ ] **Step 2: Write tests** — mock encoder with StatusQuerier, verify connect/heartbeat/print flow
- [ ] **Step 3: Remove old service.go and service_test.go**
- [ ] **Step 4: Run all tests**
- [ ] **Step 5: Commit**

---

### Task 5: Wire PrinterManager into app

**Files:**
- Modify: `internal/api/server.go` — replace `*print.PrintService` with `*print.PrinterManager`, add SSE + status endpoints
- Modify: `internal/ui/server.go` — replace PrintService with PrinterManager
- Modify: `internal/ui/handlers.go` — use manager for print, add connect/disconnect handlers
- Modify: `internal/app/server.go` — use PrinterManager
- Modify: `cmd/qlx/main.go` — create PrinterManager, call Start(), defer Stop()

- [ ] **Step 1: Update api/server.go**

Replace `printService *print.PrintService` with `printerManager *print.PrinterManager`.

Add endpoints:
```
GET  /api/printers/status         — all printer statuses
GET  /api/printers/{id}/status    — single printer status
POST /api/printers/{id}/connect   — connect to printer
POST /api/printers/{id}/disconnect — disconnect
GET  /api/printers/events         — SSE stream
```

SSE handler:
```go
func (s *Server) HandlePrinterEvents(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    flusher := w.(http.Flusher)

    ch := s.printerManager.SubscribeSSE()
    defer s.printerManager.UnsubscribeSSE(ch)

    for {
        select {
        case <-r.Context().Done():
            return
        case evt := <-ch:
            data, _ := json.Marshal(evt)
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()
        }
    }
}
```

- [ ] **Step 2: Update ui/server.go and handlers.go** — use PrinterManager
- [ ] **Step 3: Update app/server.go** — pass PrinterManager
- [ ] **Step 4: Update main.go** — create manager, Start(), defer Stop()
- [ ] **Step 5: Fix all tests**
- [ ] **Step 6: Run all tests**
- [ ] **Step 7: Commit**

---

### Task 6: Live UI — SSE listener + printer status cards

**Files:**
- Modify: `internal/embedded/static/ui-lite.js` — SSE EventSource listener
- Modify: `internal/embedded/templates/printers.html` — status cards with IDs
- Modify: `internal/embedded/templates/layout.html` — navbar printer status

- [ ] **Step 1: Add SSE listener to ui-lite.js**

```javascript
// SSE for printer status updates
var evtSource = null;
function initSSE() {
    if (evtSource) return;
    evtSource = new EventSource('/api/printers/events');
    evtSource.onmessage = function(e) {
        var evt = JSON.parse(e.data);
        updatePrinterStatus(evt.printer_id, evt.status);
        updateNavbarStatus(evt.printer_id, evt.status);
    };
}
```

Use safe DOM methods (createElement/textContent) to update:
- `#printer-status-{id}` on printers page — battery, tape, lid
- `#printer-status` in navbar — summary icon

- [ ] **Step 2: Update printers.html** — add `id="printer-status-{printerID}"` divs in each printer card
- [ ] **Step 3: Update layout.html** — `#printer-status` span with initial "..." text
- [ ] **Step 4: Verify build**
- [ ] **Step 5: Commit**

---

### Task 7: End-to-end verification

- [ ] **Step 1: Run all tests**
- [ ] **Step 2: Build and run with trace**

```bash
make build-mac && ./qlx-darwin --port 8080 --data ./data --trace
```

- [ ] **Step 3: Test flow**

1. Open /ui/printers
2. Add Niimbot B1 via BLE scan
3. Watch status card appear with battery %, tape info
4. Check navbar shows 🔋 XX%
5. Open browser console — verify SSE events arriving
6. Print a label — verify heartbeat pauses then resumes
7. Turn off printer — verify status goes to 🔴 Offline
8. Turn back on — verify auto-reconnect

- [ ] **Step 4: Final commit if fixes needed**
