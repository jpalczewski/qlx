# Frontend & Backend Cleanup — Design Spec

**Date:** 2026-03-21
**Issues:** #4, #7, #8, #12, #34, #38
**Scope:** Full-stack refactor — templating, i18n foundations, JS/CSS modularization, unified errors, service layer

## Motivation

The codebase has grown to ~900-line monolithic JS and CSS files, hardcoded Polish strings in both templates and JS, duplicated business logic across API and UI handlers, inconsistent error mapping, and no handler tests. This cleanup establishes a maintainable foundation for future features (i18n, mobile-first, icons/colors, scan-to-move) without changing external behavior.

## Implementation Sequence

| Step | What | Issues |
|------|------|--------|
| 1 | Templating refactor — PageData wrapper, domain folders | — |
| 2 | i18n scaffold — middleware, translation files, T func | #12 (foundation) |
| 3 | JS split + @ts-check + JSDoc | #4, #7 |
| 4 | CSS split + design tokens | #8 |
| 5 | Unified error-to-HTTP mapping | #34 |
| 6 | Service layer + store interfaces | #38 |

Steps 3 and 4 can run in parallel (independent files, zero dependencies).

---

## Step 1: Templating Refactor

### PageData Wrapper

Every render call wraps page-specific data in a unified context:

```go
type PageData struct {
    Lang       string // "pl", "en"
    translator *Translations // unexported
    Data       any    // per-page struct (ContainerListData, etc.)
}

func (p PageData) T(key string) string {
    return p.translator.Get(p.Lang, key)
}
```

Templates use `.T "key"` (method call, no `call` needed) and `.Data.Field`:

```html
<!-- before -->
<a href="/ui/printers">Drukarki</a>
{{range .Items}}

<!-- after -->
<a href="/ui/printers">{{.T "nav.printers"}}</a>
{{range .Data.Items}}
```

### render() Change

```go
func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
    lang := r.Context().Value(langKey).(string)
    page := PageData{
        Lang:       lang,
        translator: s.translations,
        Data:       data,
    }
    // ... rest unchanged
}
```

### Template Directory Structure

```
templates/
  layouts/
    base.html
  pages/
    inventory/
      containers.html
      container_form.html
      item.html
      item_form.html
    printers/
      printers.html
    labels/
      templates.html
      template_designer.html
    tags/
      tags.html
    search/
      search.html
  partials/
    inventory/
      breadcrumb.html
      container_list_item.html
      item_list_item.html
      tree_children.html
    tags/
      tag_chips.html
      tag_list_item.html
      tag_tree_children.html
  components/
    form_fields.html
```

**Naming convention:**
- `pages/` — full views, rendered with layout or as HTMX fragment (via `render()`)
- `partials/` — DOM fragments, returned by dedicated HTMX endpoints (via `renderPartial()`)
- `components/` — reusable `{{define}}` blocks included in pages and partials

Template loading in `NewServer` updated to walk domain subdirectories. The `go:embed` directive in `embedded.go` changes to embed the `templates` directory as a whole (`//go:embed templates`), then subdirectories are walked at init time via `fs.WalkDir`.

### renderPartial() Change

`renderPartial()` also wraps data in `PageData` so partials have access to `.T` for translated strings:

```go
func (s *Server) renderPartial(w http.ResponseWriter, r *http.Request, tmplName, defineName string, data any) {
    lang := r.Context().Value(langKey).(string)
    page := PageData{
        Lang:       lang,
        translator: s.translations,
        Data:       data,
    }
    // ... execute template with page
}
```

Note: `renderPartial` gains an `*http.Request` parameter (needed for language context). All call sites updated.

### layout.html Rename

`templates/layout.html` becomes `templates/layouts/base.html`.

---

## Step 2: i18n Scaffold

### Language Detection Middleware

```go
func LangMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        lang := "pl" // default
        if c, err := r.Cookie("lang"); err == nil {
            lang = c.Value
        } else {
            lang = parseAcceptLanguage(r) // fallback
        }
        ctx := context.WithValue(r.Context(), langKey, lang)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

Fallback chain: cookie → Accept-Language header → `"pl"` default.

Language switch endpoint: `POST /ui/actions/set-lang` sets cookie + redirects.

### Translation Files

```
static/
  i18n/
    en/
      nav.json
      actions.json
      inventory.json
      printers.json
      labels.json
      tags.json
      search.json
      errors.json
    pl/
      nav.json
      actions.json
      inventory.json
      printers.json
      labels.json
      tags.json
      search.json
      errors.json
```

Embedded via `go:embed`. Loaded at startup. English is the base language — all keys must exist in `en/`. Other languages fall back to `en` for missing keys.

### Translations Loader

```go
type Translations struct {
    langs map[string]map[string]string // lang -> key -> value
}

func (t *Translations) Get(lang, key string) string {
    if val, ok := t.langs[lang][key]; ok {
        return val
    }
    if val, ok := t.langs["en"][key]; ok {
        return val // fallback to English
    }
    return key // last resort: return key itself
}
```

### Key Namespace

Flat dot-separated keys, organized by domain file:

```json
// en/nav.json
{
  "nav.home": "Inventory",
  "nav.printers": "Printers",
  "nav.templates": "Templates",
  "nav.tags": "Tags",
  "nav.search_placeholder": "Search..."
}

// en/actions.json
{
  "action.delete": "Delete",
  "action.cancel": "Cancel",
  "action.move": "Move",
  "action.save": "Save"
}
```

### JS i18n

Merged translations served via API endpoint registered in `api/server.go`:

```go
// GET /api/i18n/{lang} — returns merged JSON for given language
mux.HandleFunc("GET /api/i18n/{lang}", s.HandleI18n)
```

Client-side loader in `shared/i18n.js`:

```js
(function() {
    window.qlx = window.qlx || {};
    var strings = {};

    fetch("/api/i18n/" + document.documentElement.lang)
        .then(function(r) { return r.json(); })
        .then(function(data) { strings = data; });

    qlx.t = function(key) {
        return strings[key] || key;
    };
})();
```

`<html lang="{{.Lang}}">` in `base.html` provides the language to JS.

### Scope Limitation

This step builds infrastructure only. Not all strings need to be extracted immediately — start with `nav`, `actions`, and `errors`. Page-specific strings can be migrated incrementally.

---

## Step 3: JS Modularization

### File Structure

```
static/js/
  shared/
    namespace.js           — window.qlx = {}
    i18n.js                — qlx.t(), fetch /api/i18n/{lang}
    toast.js               — qlx.showToast()
    sse.js                 — EventSource, updatePrinterCard, updateNavbarPrinter
    htmx-hooks.js          — htmx:afterSettle, autofocus, fetchInitialStatuses
  inventory/
    selection.js           — Map, initBulkSelect, action bar, clearSelection
    dragdrop.js            — initDragDrop, single + multi drag
    move-picker.js         — dialog, tree navigation, executeBulkMove
    delete-confirm.js      — dialog, executeBulkDelete
  tags/
    tag-picker.js          — dialog, tree, executeBulkTag
  labels/
    template-filter.js     — filterTemplates by printer model

Note: `label-designer.js`, `label-dither.js`, `label-params.js`, `label-print.js`, and `qlx-format.js` are **out of scope** for this refactor. They are self-contained modules for the label designer feature and do not share code with `ui-lite.js`. They remain as top-level files in `static/` unchanged.
```

### Module Pattern

Each file is an IIFE, registers on `window.qlx`, self-initializes on HTMX events:

```js
// @ts-check
(function() {
    var qlx = window.qlx = window.qlx || {};

    /** @param {string} message @param {boolean} [isError] */
    qlx.showToast = function(message, isError) { /* ... */ };
})();
```

### Shared API (cross-module)

```
qlx.showToast(msg, isError)   — toast.js
qlx.t(key)                    — i18n.js
qlx.clearSelection()          — selection.js (used by move-picker, delete-confirm)
```

### HTMX Re-initialization

Each domain module listens for `htmx:afterSettle` and re-initializes itself:

```js
document.body.addEventListener("htmx:afterSettle", function(e) {
    if (e.detail.target.id === "content") initBulkSelect();
});
```

No central orchestrator — each module is self-contained.

### Loading in base.html

```html
<!-- shared first, order matters due to defer -->
<script src="/static/js/shared/namespace.js" defer></script>
<script src="/static/js/shared/i18n.js" defer></script>
<script src="/static/js/shared/toast.js" defer></script>
<script src="/static/js/shared/sse.js" defer></script>
<script src="/static/js/shared/htmx-hooks.js" defer></script>
<!-- domain -->
<script src="/static/js/inventory/selection.js" defer></script>
<script src="/static/js/inventory/dragdrop.js" defer></script>
<script src="/static/js/inventory/move-picker.js" defer></script>
<script src="/static/js/inventory/delete-confirm.js" defer></script>
<script src="/static/js/tags/tag-picker.js" defer></script>
<script src="/static/js/labels/template-filter.js" defer></script>
```

`defer` guarantees execution order = order in HTML.

### TypeScript Checking

`// @ts-check` at top of every file. JSDoc for all public functions. `tsconfig.json` in project root:

```json
{
    "compilerOptions": {
        "checkJs": true,
        "noEmit": true,
        "target": "ES5",
        "strict": true
    },
    "include": ["internal/embedded/static/js/**/*.js"]
}
```

`tsc --noEmit` added to `make lint`.

---

## Step 4: CSS Organization + Design Tokens

### File Structure

```
static/css/
  shared/
    tokens.css             — :root custom properties
    reset.css              — box-sizing, normalize
    base.css               — body, headings, links, typography
    forms.css              — .form-group, inputs, textarea
    buttons.css            — .btn, .btn-primary/secondary/danger/small
    toast.css              — #toast-container, .toast-*
  layout/
    nav.css                — nav, .brand, #printer-status
    content.css            — #content, .section, .section-header, .empty-state
    responsive.css         — @media queries
  inventory/
    lists.css              — .container-list, .item-list
    cards.css              — .item-detail, .printer-card
    dragdrop.css           — [draggable], .dragging, .drag-over
    selection.css          — .bulk-select, .action-bar, .selection-mode
    quick-entry.css        — .quick-entry-*
  tags/
    tags.css               — .tag, .tag.active, .tag-filter-bar, .tag-list
    tag-chips.css          — .tag-chip, .tag-remove, .tag-add
  labels/
    designer.css           — .designer-* (label designer)
    template-cards.css     — .template-card, .template-actions, .badge
  dialogs/
    dialog.css             — dialog, ::backdrop
    tree-picker.css        — .tree-picker, .tree-node, .tree-label, .tree-branch
  search/
    search.css             — #global-search
```

### Design Tokens

Extending existing CSS custom properties into a coherent system (Pico CSS-inspired organization):

```css
:root {
    /* Colors */
    --color-bg: #1a1a2e;
    --color-bg-alt: #16213e;
    --color-bg-card: #0f3460;
    --color-text: #eee;
    --color-text-muted: #888;
    --color-accent: #e94560;
    --color-accent-hover: #ff6b6b;
    --color-border: #333;
    --color-success: #4ecca3;
    --color-warning: #ffc93c;

    /* Spacing */
    --space-xs: 0.25rem;
    --space-sm: 0.5rem;
    --space-md: 1rem;
    --space-lg: 1.5rem;
    --space-xl: 2rem;

    /* Typography */
    --font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    --font-size-sm: 0.85rem;
    --font-size-base: 1rem;
    --font-size-lg: 1.1rem;
    --font-size-xl: 1.3rem;
    --font-size-h1: 1.5rem;
    --font-size-h2: 1.2rem;
    --line-height: 1.6;

    /* Borders */
    --radius-sm: 4px;
    --radius-md: 8px;
    --radius-pill: 12px;

    /* Shadows */
    --shadow-card: none;

    /* Z-index */
    --z-toast: 1000;
    --z-action-bar: 100;
}
```

Old variable names (`--bg`, `--accent`) replaced with `--color-*` prefix. All component CSS references tokens instead of hardcoded values.

### Loading in base.html

All CSS files loaded as separate `<link>` tags. `responsive.css` last to ensure overrides work. Cache-friendly — changing one component file doesn't invalidate others.

---

## Step 5: Unified Error-to-HTTP Mapping

### Single Error Mapper

```go
// internal/shared/webutil/errors.go

var statusMap = map[error]int{
    store.ErrContainerNotFound:    404,
    store.ErrItemNotFound:         404,
    store.ErrTagNotFound:          404,
    store.ErrPrinterNotFound:      404,
    store.ErrContainerHasChildren: 409,
    store.ErrContainerHasItems:    409,
    store.ErrTagHasChildren:       409,
    store.ErrCycleDetected:        400,
    store.ErrInvalidParent:        400,
    store.ErrInvalidContainer:     400,
}

func StoreHTTPStatus(err error) int {
    for sentinel, code := range statusMap {
        if errors.Is(err, sentinel) {
            return code
        }
    }
    return 400
}

func WriteStoreErrorJSON(w http.ResponseWriter, err error) {
    status := StoreHTTPStatus(err)
    JSON(w, status, map[string]string{"error": err.Error()})
}

func WriteStoreErrorText(w http.ResponseWriter, err error) {
    http.Error(w, err.Error(), StoreHTTPStatus(err))
}
```

Removes `writeStoreError()` from `api/server.go` and `writeTagError()` from `api/handlers_tags.go`. UI handlers replace raw `http.Error()` calls with `WriteStoreErrorText()`.

New store error = one entry in `statusMap`.

---

## Step 6: Service Layer + Store Interfaces

### Package Structure

```
internal/
  service/
    interfaces.go         — store interfaces per domain
    inventory.go          — container/item CRUD, move
    bulk.go               — bulk move, delete, tag
    tags.go               — tag CRUD, assign/remove
    printers.go           — printer CRUD (thin passthrough)
    search.go             — search across entities
```

### Store Interfaces

```go
// internal/service/interfaces.go

type ItemStore interface {
    GetItem(id string) *store.Item
    CreateItem(containerID, name, desc string, qty int) *store.Item
    UpdateItem(id, name, desc string) (*store.Item, error)
    DeleteItem(id string) error
    MoveItem(id, containerID string) error
}

type ContainerStore interface {
    GetContainer(id string) *store.Container
    CreateContainer(parentID, name, desc string) *store.Container
    UpdateContainer(id, name, desc string) (*store.Container, error)
    DeleteContainer(id string) error
    MoveContainer(id, newParentID string) error
    ContainerChildren(parentID string) []store.Container
    ContainerItems(containerID string) []store.Item
    ContainerPath(id string) []store.Container
}

type Saveable interface {
    Save() error
}

type TagStore interface {
    GetTag(id string) *store.Tag
    CreateTag(parentID, name string) *store.Tag
    UpdateTag(id, name string) (*store.Tag, error)
    DeleteTag(id string) error
    MoveTag(id, newParentID string) error
    AllTags() []store.Tag
    TagChildren(parentID string) []store.Tag
    TagDescendants(id string) []string
    AddItemTag(itemID, tagID string) error
    RemoveItemTag(itemID, tagID string) error
    AddContainerTag(containerID, tagID string) error
    RemoveContainerTag(containerID, tagID string) error
}

type SearchStore interface {
    SearchContainers(query string) []store.Container
    SearchItems(query string) []store.Item
    SearchTags(query string) []store.Tag
}

type PrinterStore interface {
    AllPrinters() []store.PrinterConfig
    AddPrinter(name, encoder, model, transport, address string) *store.PrinterConfig
    DeletePrinter(id string) error
}
```

`*store.Store` implements all interfaces naturally (methods already exist).

### Service Pattern

```go
type InventoryService struct {
    store interface {
        ContainerStore
        ItemStore
        Saveable
    }
}

func (s *InventoryService) DeleteItem(id string) error {
    if err := s.store.DeleteItem(id); err != nil {
        return err
    }
    return s.store.Save()
}
```

`Save()` moves into service — handlers no longer call `SaveOrFail`. Handlers become thin adapters.

**Note:** CLAUDE.md's "Store Mutations — Always SaveOrFail" section must be updated to reflect the new pattern: service layer calls `Save()`, handlers check returned error. The `SaveOrFail` helper remains available but is no longer the primary pattern.

```go
// API handler
func (s *Server) HandleItemDelete(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if err := s.inventory.DeleteItem(id); err != nil {
        webutil.WriteStoreErrorJSON(w, err)
        return
    }
    webutil.JSON(w, 200, map[string]string{"status": "ok"})
}
```

### Testability

Service testable with mock store:

```go
func TestDeleteItem_NotFound(t *testing.T) {
    mock := &MockStore{
        deleteItem: func(id string) error { return store.ErrItemNotFound },
    }
    svc := service.NewInventoryService(mock)
    err := svc.DeleteItem("xyz")
    if !errors.Is(err, store.ErrItemNotFound) {
        t.Fatal("expected ErrItemNotFound")
    }
}
```

Handlers testable with mock service + `httptest`.

### Incremental Migration

Not everything at once. Order by duplication severity:

1. `bulk.go` — most duplicated (move, delete, tag identical in API and UI)
2. `tags.go` — tag CRUD + assign repeated
3. `inventory.go` — container/item CRUD
4. `search.go` and `printers.go` — least duplication, last

---

## Constraints

- No build step, no bundler — vanilla JS, vanilla CSS, `go:embed`
- No CSS preprocessor — CSS custom properties only
- No external i18n library — standard library + embedded JSON files
- No external test framework — standard library `testing` + `httptest`
- Must work on MIPS (~35MB RAM) — no runtime CSS/JS processing
- Single binary — all assets compiled in
- `<html lang>` dynamic per request
- Translations compiled into binary via `go:embed`

## Files Affected

### New
- `internal/service/` — interfaces.go, inventory.go, bulk.go, tags.go, printers.go, search.go
- `internal/shared/webutil/errors.go`
- `static/i18n/en/*.json`, `static/i18n/pl/*.json`
- `static/js/shared/*.js`, `static/js/inventory/*.js`, `static/js/tags/*.js`, `static/js/labels/*.js`
- `static/css/shared/*.css`, `static/css/layout/*.css`, `static/css/inventory/*.css`, etc.
- `tsconfig.json`

### Modified
- `internal/ui/server.go` — PageData, render(), template loading with domain folders
- `internal/ui/handlers*.go` — use service layer, WriteStoreErrorText
- `internal/api/server.go` — use service layer, remove writeStoreError
- `internal/api/handlers*.go` — use service layer, WriteStoreErrorJSON
- `internal/app/server.go` — wire LangMiddleware, service layer
- `internal/embedded/embedded.go` — change to `//go:embed templates` (whole dir), `//go:embed static` (whole dir)
- `cmd/qlx/main.go` — create service, pass to servers
- `CLAUDE.md` — update "Store Mutations" section for service layer pattern
- `Makefile` — add `tsc --noEmit` to `lint` target

### Deleted
- `internal/embedded/static/ui-lite.js` — replaced by `js/**/*.js`
- `internal/embedded/static/style.css` — replaced by `css/**/*.css`
- `internal/embedded/templates/*.html` (flat) — moved to `templates/pages/`, `templates/layouts/`
