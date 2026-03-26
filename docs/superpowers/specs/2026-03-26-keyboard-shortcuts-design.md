# Keyboard Shortcuts for Power Users

**Issue**: #31
**Date**: 2026-03-26
**Approach**: Central dispatcher (Approach A)

## Shortcut Map

| Key | Action | Context |
|-----|--------|---------|
| `/` | Focus search | Global |
| `Ctrl+K` | Focus search | Global |
| `i` | Focus quick-entry items | Container view |
| `c` | Focus quick-entry containers | Container view |
| `m` | Open container navigator | Global |
| `s` | Toggle selection mode | Container view |
| `a` | Toggle select all | Selection mode |
| `?` | Show help overlay | Global |
| `Escape` | Close/cancel (hierarchical) | Global, including in inputs |
| Arrow Up/Down | Navigate list | Container view, no input focus |
| `Enter` | Open highlighted item | When `.kb-active` present |

## Architecture: Central Dispatcher

### New file: `js/shared/keyboard.js`

Single global `keydown` listener on `document`. Shortcuts defined as array of objects:

```js
var shortcuts = [
  { key: "/",   handler: focusSearch,          label: "keyboard.focus_search",    global: true },
  { key: "k",   ctrl: true, handler: focusSearch, label: "keyboard.focus_search", global: true },
  { key: "i",   handler: focusItemEntry,       label: "keyboard.new_item",        context: "container" },
  { key: "c",   handler: focusContainerEntry,  label: "keyboard.new_container",   context: "container" },
  { key: "m",   handler: openContainerNav,     label: "keyboard.go_to_container", global: true },
  { key: "s",   handler: enterSelectionMode,   label: "keyboard.selection_mode",  context: "container" },
  { key: "a",   handler: toggleSelectAll,      label: "keyboard.select_all",      context: "selection" },
  { key: "?",   handler: showHelpDialog,       label: "keyboard.help",            global: true },
  { key: "Escape", handler: handleEscape,       label: "keyboard.close",          global: true, allowInInput: true },
];
```

### Dispatcher logic

1. If focus on `input`/`textarea`/`select`/`[contenteditable]` — only pass `Escape` and `Ctrl+K`
2. If a `<dialog>` is open — only pass `Escape` (native behavior handles it)
3. Match key against shortcut map — invoke handler

### Context detection from DOM

- `"container"` — `#content` contains `.container-view`
- `"selection"` — `#content` has class `.selection-mode`

## Escape Hierarchy

Priority from top:

1. Open dialog — close (native)
2. Focus on input/textarea — `blur()` (return focus to `<body>`, re-enable shortcuts)
3. Selection mode active — exit selection mode + clear selection
4. `.kb-active` on list — remove highlight

## List Navigation (Arrow Keys + Enter)

Handled in `keyboard.js`. Operates on `.container-list` and `.item-list`:

- **State**: `qlx._activeListIndex` — index of highlighted `<li>` in active list
- **Arrow Up/Down**: Moves `.kb-active` class between `<li>` elements (skips `.empty-state`). Scrolls into view via `scrollIntoView({ block: "nearest" })`
- **Enter**: On highlighted `<li>` — simulates click on inner `<a>` (hx-get navigation)
- **List transition**: Last item in container-list + Down — jumps to first item in item-list (and vice versa Up from first item-list element)
- **Reset**: After every HTMX swap of `#content` — clear `_activeListIndex`, remove `.kb-active`

Works globally when no input is focused.

## Container Navigator (`m`)

New instance of `createTreePicker`:

```js
var navPicker = qlx.createTreePicker({
  id: "container-nav-picker",
  title: function () { return qlx.t("keyboard.go_to_container"); },
  endpoint: "/partials/tree",
  searchEndpoint: "/partials/tree/search",
  searchPlaceholder: function () { return qlx.t("nav.search_placeholder"); },
  confirmLabel: function () { return qlx.t("keyboard.open"); },
  onConfirm: function (targetId) {
    htmx.ajax("GET", "/containers/" + targetId, { target: "#content" });
  }
});
```

Reuses existing tree-picker. Only difference: `onConfirm` navigates instead of bulk move.

## Help Overlay (`?`)

Dialog `<dialog id="keyboard-help">` built dynamically from `shortcuts` array. Generated once, cached.

**Layout**: Two-column list — key in `<kbd>` on left, description on right. Grouped:
- **Navigation**: `/`/`Ctrl+K`, `m`, arrows, Enter
- **Actions**: `i`, `c`, `s`, `a`
- **General**: `?`, Escape

Labels from i18n (`qlx.t(shortcut.label)`). Closing: Escape (native dialog) + backdrop click.

**CSS**: New file `css/dialogs/keyboard-help.css`.

## Refactoring: selection.js

1. **Remove backward compat** (lines 86-87): Dead code — `window.clearSelection` and `window.initBulkSelect`
2. **Extract `qlx.toggleSelectionMode()`** from private `onSelectToggle()` — public API needed for `s` shortcut
3. **Add `qlx.selectAll()`** — iterates `.bulk-select` checkboxes in `#content`, toggles all, updates `selection` Map and action bar
4. **Add `qlx.isSelectionMode()`** — getter checking `#content.classList.contains("selection-mode")`, needed by dispatcher for `"selection"` context

## Integration

### base.html

Add `keyboard.js` as last script (after all modules):
```html
<script src="/static/js/shared/keyboard.js" defer></script>
```

### New CSS files (added to base.html)

- `css/dialogs/keyboard-help.css` — help overlay styles
- `css/inventory/kb-active.css` — `.kb-active` list highlight

### New i18n keys

- `keyboard.help` — "Keyboard shortcuts"
- `keyboard.new_item` — "New item"
- `keyboard.new_container` — "New container"
- `keyboard.go_to_container` — "Go to container"
- `keyboard.open` — "Open"
- `keyboard.selection_mode` — "Selection mode"
- `keyboard.select_all` — "Select all"
- `keyboard.close` — "Close / cancel"
- `keyboard.navigate` — "Navigate list"
- `keyboard.open_selected` — "Open selected"
- `keyboard.focus_search` — "Focus search"
- `keyboard.nav_group` — "Navigation"
- `keyboard.action_group` — "Actions"
- `keyboard.general_group` — "General"

### Existing code left unchanged

- `quick-entry.js` Escape handling — contextual, works on focused textarea
- `tabs.js` arrow key handling — contextual, works on focused tab button
- `tag-autocomplete.js` keydown — contextual, works on focused input

## E2E Tests

New file: `e2e/tests/keyboard.spec.ts`

1. `/` focuses search
2. `Ctrl+K` focuses search
3. `?` opens help overlay
4. Escape closes help overlay
5. `s` enables selection mode (in container view)
6. `a` selects all (in selection mode)
7. Escape exits selection mode
8. `i` focuses item quick-entry
9. `c` focuses container quick-entry
10. Arrow keys navigate list (`.kb-active` class)
11. Enter opens highlighted element
12. `m` opens container navigator dialog
13. Shortcuts ignored in inputs; Escape blurs input, then shortcuts work
14. Shortcuts ignored when dialog is open
