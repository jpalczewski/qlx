# Batch Refactoring: Deduplication & Clean Interfaces

**Date:** 2026-03-21
**Scope:** Issues #35, #36, #37, #40, #41 + JS tree picker deduplication
**Branch:** `feat/frontend-css-fixes` (to be renamed or kept)

## 1. Extract `findModel` helper (#37)

New private function in `internal/print/manager.go`:

```go
func findModel(enc encoder.Encoder, modelID string) *encoder.ModelInfo {
    for _, mi := range enc.Models() {
        if mi.ID == modelID {
            info := mi
            return &info
        }
    }
    return nil
}
```

**Important:** `ConnectPrinter` passes `nil` modelInfo to `NewSession` intentionally (tolerates unknown models). `Print` and `PrintImage` check for nil and return an error. The helper returns `*ModelInfo` (nil-safe), callers decide whether nil is an error.

Replaces 3 blocks:
- `ConnectPrinter` (lines 109-116) — uses result directly, nil is OK
- `Print` (lines 187-194) — checks nil, returns error
- `PrintImage` (lines 244-251) — checks nil, returns error

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

New function in `internal/shared/webutil/`:

```go
func FormatContainerPath(path []store.Container, sep string) string
```

Placed in `webutil` (not `store`) because this is a presentation/formatting concern, not persistence logic.

Replaces 4 duplicated path-building loops:
- `api/server.go` lines ~321-326 (CSV export, sep `" -> "`)
- `api/server.go` lines ~420-427 (HandlePrint)
- `ui/handlers.go` lines ~279-288 (HandleItemPrint)
- `ui/handlers.go` lines ~579-588 (HandleContainerItemsJSON)

**Files:** `internal/shared/webutil/` (new helper), `internal/api/server.go`, `internal/ui/handlers.go`

## 4. Deduplicate `resolveTagIDs` (#35)

The `loadLayout` FuncMap closure in `ui/server.go` and `s.resolveTagIDs()` in `ui/handlers_tags.go` duplicate the same loop over tag IDs calling `GetTag`.

**Wiring constraint:** `loadLayout` is called during `Server` construction, before `*Server` is fully assembled — it cannot reference `s.resolveTagIDs()` directly.

**Solution:** Pass a `func([]string) []store.Tag` callback into `loadLayout` (or set the FuncMap entry after construction). The callback calls `store.GetTag` in a loop. `resolveTagIDs` method on Server delegates to the same callback or is removed if only used in FuncMap.

**Files:** `internal/ui/handlers_tags.go`, `internal/ui/server.go`

## 5. Standardize `parent_id` query param (#41)

Change from `parent` to `parent_id` across all surfaces. Complete change list:

**Go handlers** (`internal/ui/handlers_tags.go`):
- `HandleTags` line 12: `r.URL.Query().Get("parent")` → `"parent_id"`
- `HandleTagCreate` line ~56: redirect `"/ui/tags?parent="` → `"?parent_id="`
- `HandleTagDelete` line ~75: redirect `"/ui/tags?parent="` → `"?parent_id="`
- `HandleTagUpdate` line ~97: redirect `"/ui/tags?parent="` → `"?parent_id="`

**Templates:**
- `pages/tags/tags.html` lines ~9, ~17: `?parent={{ .ID }}` → `?parent_id={{ .ID }}`
- `partials/tags/tag_list_item.html` line ~3: `?parent={{ .ID }}` → `?parent_id={{ .ID }}`
- `pages/search/search.html` line ~35: `?parent={{ .ID }}` → `?parent_id={{ .ID }}`

**JS** (if any build URLs with `?parent=` for tags — verify during implementation).

**Files:** `internal/ui/handlers_tags.go`, 3-4 template files

## 6. JS tree picker factory (deduplication)

New `internal/embedded/static/js/shared/tree-picker.js` exposes:

```js
qlx.createTreePicker = function(config) {
  // config: {
  //   id: string,              // dialog element id
  //   title: string,           // dialog heading
  //   endpoint: string,        // tree data endpoint (e.g. "/ui/partials/tree")
  //   searchEndpoint: string,  // search endpoint (e.g. "/ui/partials/tree/search")
  //   searchPlaceholder: string,
  //   confirmLabel: string,    // confirm button text
  //   onConfirm: function(selectedId: string) // called with picked node's data-id
  // }
  // returns: { open() }
}
```

**`onConfirm(selectedId)`** receives the selected tree node's `data-id`. Each thin wrapper implements its own fetch/bulk logic in the callback body. The factory does NOT contain any bulk operation knowledge.

Contains extracted shared helpers:
- `handleTreeExpand(expandEl, treeContainer, endpoint)` — parameterized endpoint
- `handleTreeLabelSelect(labelEl, treeContainer, state)` — manages selection state internally

`move-picker.js` reduces to config + `executeBulkMove(targetId)`.
`tag-picker.js` reduces to config + `executeBulkTag(tagId)`.

**Files:**
- `internal/embedded/static/js/shared/tree-picker.js` (new)
- `internal/embedded/static/js/inventory/move-picker.js` (simplify)
- `internal/embedded/static/js/tags/tag-picker.js` (simplify)
- `internal/embedded/templates/layouts/base.html` (add script tag before move-picker)

## Implementation Order

1. #37 (`findModel`) — no dependencies
2. #40 (interface) — after #37 to avoid merge conflicts in `manager.go` (no semantic dependency)
3. #36 (`FormatContainerPath`) — independent
4. #35 (`resolveTagIDs`) — independent
5. #41 (`parent_id`) — independent
6. JS tree picker — independent of Go changes

Steps 3-6 can be parallelized.
