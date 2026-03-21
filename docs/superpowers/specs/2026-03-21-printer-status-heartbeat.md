# Printer Status + Heartbeat — Design Spec

## Problem

QLX nie wie nic o stanie drukarki — bateria, typ taśmy, zużycie, pokrywa. Połączenie BLE otwieramy tylko na czas druku i zamykamy. Użytkownik drukuje w ciemno.

## Wymagania

1. **Persistent connection** — po dodaniu drukarki utrzymujemy stałe połączenie BLE/serial
2. **Heartbeat co 2s** — odpytujemy drukarkę o status (bateria, pokrywa, taśma)
3. **RFID info** — przy połączeniu czytamy info o taśmie (typ, total/used etykiet)
4. **Live UI** — status widoczny na stronie drukarek + skrót w navbarze
5. **Auto-reconnect** — po rozłączeniu próba ponownego połączenia

## Architektura

```
┌─────────────────────────────────────────────────┐
│ PrinterManager (nowy, zastępuje direct print)    │
│ - utrzymuje persistent connections               │
│ - heartbeat goroutine per drukarka               │
│ - cache statusu per drukarka                     │
│ - drukowanie przez istniejące połączenie          │
├─────────────────────────────────────────────────┤
│ PrinterSession (per drukarka)                    │
│ - transport (otwarty stale)                      │
│ - heartbeat ticker co 2s                         │
│ - status cache (bateria, taśma, pokrywa)         │
│ - reconnect logic                                │
│ - mutex na transport (heartbeat vs print)         │
├─────────────────────────────────────────────────┤
│ UI: SSE endpoint /api/printers/events            │
│ - push statusu do przeglądarki                   │
│ - navbar aktualizuje się live                    │
└─────────────────────────────────────────────────┘
```

### PrinterSession

```go
type PrinterStatus struct {
    Connected    bool   `json:"connected"`
    Battery      int    `json:"battery"`       // 0-100 lub -1 jeśli nieznany
    LidClosed    bool   `json:"lid_closed"`
    PaperLoaded  bool   `json:"paper_loaded"`
    LabelType    string `json:"label_type"`    // "with-gaps", "transparent", etc.
    TotalLabels  int    `json:"total_labels"`  // z RFID, -1 jeśli brak
    UsedLabels   int    `json:"used_labels"`
    LastError    string `json:"last_error"`
    LastUpdated  time.Time `json:"last_updated"`
}

type PrinterSession struct {
    config    store.PrinterConfig
    transport transport.Transport
    encoder   encoder.Encoder
    status    PrinterStatus
    mu        sync.Mutex
    stop      chan struct{}
}
```

### Niimbot heartbeat flow

Komendy do odpytania (z niimbluelib):

```
1. CONNECT (0xC1) → response zawiera ConnectResult
2. GET_INFO (0x40) z podtypem:
   - PrinterModelId (0x08) → model ID
   - BatteryChargeLevel (0x0A) → bateria
   - SerialNumber (0x0B) → serial
   - SoftWareVersion (0x09) → firmware
3. RFID_INFO (0x1A) → typ taśmy, total/used etykiet, UUID, barcode
4. HEARTBEAT (0xDC, data: 0x01) → response (10-20 bytes):
   - lidClosed, chargeLevel, paperInserted, rfidReadState
```

Heartbeat response format (zależy od modelu):
- 13 bytes (B1): skip 9, lidClosed, chargeLevel, paperInserted, rfidSuccess
- 10 bytes (D110): skip 8, lidClosed, chargeLevel
- 19 bytes: skip 15, lidClosed, chargeLevel, paperInserted, rfidSuccess
- 20 bytes: skip 18, paperInserted, rfidSuccess

### PrinterManager

```go
type PrinterManager struct {
    store      *store.Store
    encoders   map[string]encoder.Encoder
    sessions   map[string]*PrinterSession  // printerID → session
    mu         sync.RWMutex
    sseClients map[chan PrinterStatusEvent]struct{}
}

func (m *PrinterManager) Connect(printerID string) error
func (m *PrinterManager) Disconnect(printerID string) error
func (m *PrinterManager) GetStatus(printerID string) PrinterStatus
func (m *PrinterManager) AllStatuses() map[string]PrinterStatus
func (m *PrinterManager) Print(printerID string, data label.LabelData, template string) error
func (m *PrinterManager) SubscribeSSE() <-chan PrinterStatusEvent
func (m *PrinterManager) UnsubscribeSSE(ch <-chan PrinterStatusEvent)
```

### SSE (Server-Sent Events)

Endpoint: `GET /api/printers/events`

```
data: {"printer_id":"abc","status":{"connected":true,"battery":85,"lid_closed":true,...}}

data: {"printer_id":"abc","status":{"battery":84,...}}
```

UI słucha via `EventSource("/api/printers/events")` i aktualizuje:
- Navbar: ikona baterii + procent
- Strona drukarek: karta ze statusem per drukarka

### UI — navbar

```html
<nav>
    <a href="/ui" ...>QLX</a>
    <a href="/ui/printers" ...>Drukarki</a>
    <span id="printer-status"></span>  <!-- aktualizowane przez SSE -->
</nav>
```

JS nasłuchuje SSE i aktualizuje `#printer-status`:
```
🔋 85% 📋 OK    ← connected, battery, paper loaded
🔴 Offline      ← disconnected
⚠️ Pokrywa!     ← lid open
```

### UI — strona drukarek

Każda drukarka ma kartę ze statusem:

```
┌─────────────────────────────────┐
│ Brother kuchnia          🟢 ON  │
│ brother-ql / QL-700 / USB      │
│ Status: gotowa                  │
├─────────────────────────────────┤
│ Niimbot biuro            🟢 ON  │
│ niimbot / B1 / BLE             │
│ 🔋 85%  📋 Etykiety: 142/300   │
│ Pokrywa: zamknięta              │
│ Taśma: with-gaps                │
└─────────────────────────────────┘
```

### Lifecycle

1. **Start serwera** → PrinterManager czyta drukarki z Store → próbuje połączyć się z każdą
2. **Połączenie** → CONNECT → GET_INFO → RFID_INFO → start heartbeat goroutine
3. **Heartbeat co 2s** → aktualizuje status cache → push SSE do klientów
4. **Drukowanie** → lock mutex na sesji → print → unlock (heartbeat czeka)
5. **Rozłączenie** → stop heartbeat → reconnect po 5s
6. **Dodanie drukarki** → nowa sesja → connect
7. **Usunięcie drukarki** → disconnect → usuń sesję

### Brother QL status

Brother nie ma heartbeat, ale po `ESC i S` (status request) zwraca 32 bajty ze statusem (model, media type, media width, errors). Można pollować co 5s (nie 2s — Brother jest wolniejszy).

## Pliki do stworzenia

```
internal/print/
├── manager.go           — PrinterManager (zastępuje PrintService)
├── manager_test.go
├── session.go           — PrinterSession (connection + heartbeat)
├── status.go            — PrinterStatus model
```

## Pliki do modyfikacji

```
internal/print/service.go          — usunąć lub refaktorować do managera
internal/print/encoder/niimbot/niimbot.go — dodać Heartbeat(), GetInfo(), RfidInfo()
internal/print/encoder/encoder.go  — rozszerzyć interfejs o StatusQuery
internal/api/server.go             — SSE endpoint, status endpoints
internal/ui/server.go              — route, template
internal/ui/handlers.go            — status handlers
internal/embedded/templates/printers.html — live status karty
internal/embedded/templates/layout.html   — navbar status
internal/embedded/static/ui-lite.js       — SSE listener
internal/app/server.go             — inject PrinterManager zamiast PrintService
cmd/qlx/main.go                    — create PrinterManager
```

## Weryfikacja

```bash
make test && make build-mac
./qlx-darwin --port 8080 --data ./data --trace

# 1. Dodaj Niimbot B1 przez BLE scan
# 2. Strona /ui/printers — karta ze statusem (bateria, taśma)
# 3. Navbar — 🔋 85%
# 4. Wyłącz drukarkę → status zmienia się na 🔴 Offline
# 5. Włącz z powrotem → auto-reconnect → status wraca
# 6. Drukuj → heartbeat pauzuje na czas druku, potem wraca
```
