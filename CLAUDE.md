# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

QLX is a multi-printer label printing proxy with a lightweight inventory system. Single statically-linked Go binary with embedded assets. Primary printer: Niimbot B1 via BLE. Future deployment target: MIPS (embedded Linux, ~64MB RAM).

## Build & Run Commands

```bash
# Primary dev build (Mac M4, BLE support)
make build-mac

# Dev server
make run                    # starts on :8080, data in ./data

# Tests
make test                   # all tests
make test-ble               # with BLE build tag
go test ./internal/store/ -run TestName -v  # single test

# Lint
make lint                   # golangci-lint (govet, staticcheck, errcheck, gosec, etc.)
make lint-fix               # auto-fix

# Cross-compile for MIPS
make build-mips             # CGO_ENABLED=0, -tags minimal

# Git hooks
make install-hooks          # lefthook: gofmt, go vet, golangci-lint, conventional commits
```

## Commit Convention

Conventional commits enforced by lefthook: `feat|fix|refactor|docs|test|chore|build|ci|perf(scope): message`

## Architecture

```
cmd/qlx/main.go              → Entry point (flags: --device, --port, --host, --data, --trace)
internal/
  app/server.go              → Composition root (ui.Server + api.Server)
  ui/                        → HTMX UI handlers, templates, view models
  api/server.go              → JSON API (containers, items, printers, print, export)
  store/                     → JSON file persistence with sync.RWMutex
  print/
    manager.go               → PrinterManager (session lifecycle, SSE events)
    session.go               → Per-printer persistent connection + heartbeat
    encoder/                 → Protocol plugins (Encoder interface)
      brother/               → QL-700 raster protocol
      niimbot/               → B1 packet protocol (bidirectional)
    transport/               → Connection plugins (USB, Serial, BLE, Remote HTTP, Mock)
    label/                   → Label rendering (templates, QR/barcode, Fabric.js export)
  shared/webutil/            → Logging, response helpers, middleware
  embedded/                  → go:embed declarations
```

### Key Design Decisions

- **Plugin architecture**: Encoder and Transport are clean interfaces — extend by adding implementations
- **No external HTTP framework**: Uses Go 1.22+ `http.ServeMux` with pattern matching (`GET /api/items/{id}`)
- **HTMX + vanilla JS**: Deliberate choice — no frontend frameworks, no bundler. All JS is vanilla
- **JSON file store**: No database; `sync.RWMutex`-protected maps, atomic disk writes (temp + rename)
- **Embedded assets**: All static files, fonts, templates compiled into binary via `go:embed`
- **Build tags**: `ble` (macOS BLE via CoreBluetooth, requires CGO), `minimal` (MIPS: USB + Remote only)

### Content Negotiation

- `/api/*` → JSON responses
- `/ui/*` → Full HTML on direct GET, HTMX fragments when `HX-Request` header present

### Print Workflow

1. POST `/api/items/{id}/print` with template
2. PrinterManager resolves session → Label.Render() produces image
3. Encoder converts image → protocol commands → Transport writes to device
4. Session heartbeat monitors status asynchronously via SSE

## Code Patterns

### Store Mutations — Always SaveOrFail

Every store mutation must call `webutil.SaveOrFail(w, store.Save)`. Never ignore Save errors.

### Frontend Safety

Use safe DOM methods only: `createElement`, `textContent`, `appendChild`. **Never use `innerHTML`**.

### Logging

Use `webutil.LogInfo`, `webutil.LogError`, `webutil.LogTrace` — not `fmt.Println` or `log.*`.

### IDs

UUID v4 via `github.com/google/uuid`. JSON field names use `snake_case`.

### Testing

- Table-driven tests, `httptest` for handlers
- `NewMemoryStore()` for unit tests (no disk I/O)
- `MockTransport` for encoder tests
- Standard library only — no external test frameworks

## MIPS Constraints

Target has ~35MB usable RAM. Respect memory tuning in main.go: `SetMemoryLimit(16MB)`, `SetGCPercent(20)`. Process label images line-by-line where possible.

## Design Principles

- **Modular & reusable**: Keep interfaces clean, packages decoupled, code easy to extend
- **Document public APIs**: Exported functions and types should have clear godoc comments
- **Standard library first**: Minimize external dependencies

## Tool Usage

- Use **Serena** proactively for code exploration (symbol overview, find references) instead of reading entire files
- Use **Context7** for up-to-date library documentation lookup
- Use **Playwright** for frontend testing when applicable
