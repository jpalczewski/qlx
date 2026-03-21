# Batch Refactoring: Deduplication & Clean Interfaces

**Date:** 2026-03-21
**Scope:** Issues #35, #36, #37, #40, #41 + JS tree picker deduplication
**Branch:** `feat/frontend-css-fixes` (to be renamed or kept)

## 1. Extract `findModel` helper (#37)

New private function in `internal/print/manager.go`:

```go
func findModel(enc encoder.Encoder, modelID string) (*encoder.ModelInfo, error) {
    for _, mi := range enc.Models() {
        if mi.ID == modelID {
            return &mi, nil
        }
    }
    return nil, fmt.Errorf("model not found: %s", modelID)
}
```

Replaces 3 identical blocks in `ConnectPrinter` (lines 110-116), `Print` (lines 188-194), `PrintImage` (lines 244-251).

**Files:** `internal/print/manager.go`

## 2. Narrow `PrinterManager` dependency (#40)

New interface in `internal/print/manager.go`:

```go
type PrinterConfigStore interface {
    GetPrinter(id string) *store.PrinterConfig
    AllPrinters() []store.PrinterConfig
}
```

- `PrinterManager.store` changes type from `*store.Store` to `PrinterConfigStore`
- `NewPrinterManager` accepts `PrinterConfigStore`
- `*store.Store` satisfies the interface implicitly — no store changes needed

**Files:** `internal/print/manager.go`, `internal/app/server.go`

## 3. Extract `FormatContainerPath` helper (#36)

New function in `internal/store/`:

```go
func FormatContainerPath(path []Container, sep string) string
```

Replaces 4 duplicated path-building loops:
- `api/server.go` lines ~321-326 (CSV export, sep `" -> "`)
- `api/server.go` lines ~420-427 (HandlePrint)
- `ui/handlers.go` lines ~279-288 (HandleItemPrint)
- `ui/handlers.go` lines ~579-588 (HandleContainerItemsJSON)

**Files:** `internal/store/` (new helper), `internal/api/server.go`, `internal/ui/handlers.go`

## 4. Deduplicate `resolveTagIDs` (#35)

`ui/server.go` FuncMap closure calls `s.resolveTagIDs()` instead of its own loop. Remove the duplicate logic from the closure.

**Files:** `internal/ui/handlers_tags.go`, `internal/ui/server.go`

## 5. Standardize `parent_id` query param (#41)

Change `ui/handlers_tags.go` from `r.URL.Query().Get("parent")` to `r.URL.Query().Get("parent_id")`. Update any JS or templates that construct URLs with `?parent=`.

**Files:** `internal/ui/handlers_tags.go`, templates/JS as needed

## 6. JS tree picker factory (deduplication)

New `internal/embedded/static/js/shared/tree-picker.js` exposes:

```js
qlx.createTreePicker = function(config) {
  // config: { id, title, endpoint, searchEndpoint, confirmLabel, onConfirm }
  // returns: { open(), dialog }
}
```

Contains extracted `handleTreeExpand(expandEl, treeContainer, endpoint)` and `handleTreeLabelSelect(labelEl, treeContainer, confirmBtnId, onSelect)`.

`move-picker.js` and `tag-picker.js` reduce to ~15-20 lines of config each.

**Files:**
- `internal/embedded/static/js/shared/tree-picker.js` (new)
- `internal/embedded/static/js/inventory/move-picker.js` (simplify)
- `internal/embedded/static/js/tags/tag-picker.js` (simplify)
- `internal/embedded/templates/layouts/base.html` (add script tag)

## Implementation Order

1. #37 (`findModel`) — no dependencies
2. #40 (interface) — after #37, both touch `manager.go`
3. #36 (`FormatContainerPath`) — independent
4. #35 (`resolveTagIDs`) — independent
5. #41 (`parent_id`) — independent
6. JS tree picker — independent of Go changes

Steps 3-6 can be parallelized.
