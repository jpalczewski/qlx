# Keyboard Shortcuts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add global keyboard shortcuts for power users — navigation, selection, quick-entry focus, container navigator, and help overlay.

**Architecture:** Central dispatcher in `keyboard.js` with a single `keydown` listener, shortcut map driving both behavior and help overlay generation. Refactor `selection.js` to expose public API for keyboard-driven selection.

**Tech Stack:** Vanilla JS (IIFE + `window.qlx`), HTMX, native `<dialog>`, existing `createTreePicker`

**Spec:** `docs/superpowers/specs/2026-03-26-keyboard-shortcuts-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|---------------|
| Create | `internal/embedded/static/i18n/en/keyboard.json` | English keyboard i18n keys |
| Create | `internal/embedded/static/i18n/pl/keyboard.json` | Polish keyboard i18n keys |
| Create | `internal/embedded/static/css/dialogs/keyboard-help.css` | Help overlay dialog styles |
| Create | `internal/embedded/static/css/inventory/kb-active.css` | List keyboard navigation highlight |
| Create | `internal/embedded/static/js/shared/keyboard.js` | Central dispatcher, list nav, help overlay, container navigator |
| Create | `e2e/tests/keyboard.spec.ts` | E2E tests for all shortcuts |
| Modify | `internal/embedded/static/js/inventory/selection.js` | Expose public API: `toggleSelectionMode`, `selectAll`, `isSelectionMode`; remove dead backward compat |
| Modify | `internal/embedded/templates/layouts/base.html` | Add CSS + JS references |

---

### Task 1: i18n Keys

**Files:**
- Create: `internal/embedded/static/i18n/en/keyboard.json`
- Create: `internal/embedded/static/i18n/pl/keyboard.json`

- [ ] **Step 1: Create English i18n file**

```json
{
  "keyboard.help": "Keyboard shortcuts",
  "keyboard.new_item": "New item",
  "keyboard.new_container": "New container",
  "keyboard.go_to_container": "Go to container",
  "keyboard.open": "Open",
  "keyboard.selection_mode": "Selection mode",
  "keyboard.select_all": "Select all",
  "keyboard.close": "Close / cancel",
  "keyboard.navigate": "Navigate list",
  "keyboard.open_selected": "Open selected",
  "keyboard.focus_search": "Focus search",
  "keyboard.nav_group": "Navigation",
  "keyboard.action_group": "Actions",
  "keyboard.general_group": "General"
}
```

- [ ] **Step 2: Create Polish i18n file**

```json
{
  "keyboard.help": "Skróty klawiszowe",
  "keyboard.new_item": "Nowy przedmiot",
  "keyboard.new_container": "Nowy kontener",
  "keyboard.go_to_container": "Przejdź do kontenera",
  "keyboard.open": "Otwórz",
  "keyboard.selection_mode": "Tryb zaznaczania",
  "keyboard.select_all": "Zaznacz wszystko",
  "keyboard.close": "Zamknij / anuluj",
  "keyboard.navigate": "Nawiguj po liście",
  "keyboard.open_selected": "Otwórz zaznaczony",
  "keyboard.focus_search": "Szukaj",
  "keyboard.nav_group": "Nawigacja",
  "keyboard.action_group": "Akcje",
  "keyboard.general_group": "Ogólne"
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/embedded/static/i18n/en/keyboard.json internal/embedded/static/i18n/pl/keyboard.json
git commit -m "feat(i18n): add keyboard shortcut translation keys (#31)"
```

---

### Task 2: Refactor selection.js

**Files:**
- Modify: `internal/embedded/static/js/inventory/selection.js`

- [ ] **Step 1: Remove backward compat aliases**

Delete lines 86-87:
```js
// DELETE these two lines:
window.clearSelection = qlx.clearSelection;
window.initBulkSelect = qlx.initBulkSelect;
```

- [ ] **Step 2: Extract `qlx.toggleSelectionMode()`**

Replace the private `onSelectToggle` function (line 66-73) with a public `qlx.toggleSelectionMode` and update the click handler reference:

```js
qlx.toggleSelectionMode = function toggleSelectionMode() {
  var content = document.getElementById("content");
  if (!content) return;
  content.classList.toggle("selection-mode");
  if (!content.classList.contains("selection-mode")) {
    qlx.clearSelection();
  }
};
```

Update `initBulkSelect` to use `qlx.toggleSelectionMode` instead of `onSelectToggle`:
```js
toggleBtn.removeEventListener("click", qlx.toggleSelectionMode);
toggleBtn.addEventListener("click", qlx.toggleSelectionMode);
```

- [ ] **Step 3: Add `qlx.isSelectionMode()`**

```js
qlx.isSelectionMode = function isSelectionMode() {
  var content = document.getElementById("content");
  return content ? content.classList.contains("selection-mode") : false;
};
```

- [ ] **Step 4: Add `qlx.selectAll()`**

```js
qlx.selectAll = function selectAll() {
  var checkboxes = document.querySelectorAll(".bulk-select");
  var allChecked = selection.size > 0 && selection.size === checkboxes.length;
  checkboxes.forEach(function (cb) {
    var el = /** @type {HTMLInputElement} */ (cb);
    var li = el.closest("[data-id]");
    if (!li) return;
    var id = li.getAttribute("data-id");
    var type = li.getAttribute("data-type") || "item";
    if (allChecked) {
      el.checked = false;
      selection.delete(id);
    } else {
      el.checked = true;
      selection.set(id, type);
    }
  });
  updateActionBar();
};
```

- [ ] **Step 5: Verify the app still builds**

Run: `make build-mac`
Expected: Successful build (JS is embedded, no compilation step, but ensures no Go embed breakage)

- [ ] **Step 6: Commit**

```bash
git add internal/embedded/static/js/inventory/selection.js
git commit -m "refactor(selection): expose public API for keyboard shortcuts (#31)"
```

---

### Task 3: CSS Files

**Files:**
- Create: `internal/embedded/static/css/dialogs/keyboard-help.css`
- Create: `internal/embedded/static/css/inventory/kb-active.css`
- Modify: `internal/embedded/templates/layouts/base.html`

- [ ] **Step 1: Create keyboard help overlay CSS**

File: `internal/embedded/static/css/dialogs/keyboard-help.css`

```css
#keyboard-help { max-width: 420px; }
#keyboard-help h3 { margin: 0 0 1rem; font-size: 1.1rem; }
.kb-help-group { margin-bottom: 1rem; }
.kb-help-group-title {
  font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em;
  color: var(--color-text-muted); margin-bottom: 0.25rem;
}
.kb-help-row {
  display: flex; align-items: center; gap: 0.75rem;
  padding: 0.25rem 0;
}
.kb-help-row kbd {
  display: inline-block; min-width: 1.8rem; text-align: center;
  padding: 0.15rem 0.4rem; border-radius: 4px;
  border: 1px solid var(--color-border); background: var(--color-bg);
  font-family: inherit; font-size: 0.8rem;
}
.kb-help-row span { font-size: 0.9rem; }
```

- [ ] **Step 2: Create kb-active highlight CSS**

File: `internal/embedded/static/css/inventory/kb-active.css`

```css
.container-list li.kb-active,
.item-list li.kb-active {
  outline: 2px solid var(--color-accent);
  outline-offset: -2px;
  border-radius: 4px;
}
```

- [ ] **Step 3: Add CSS and JS references to base.html**

In `internal/embedded/templates/layouts/base.html`, add after the `selection.css` link (line 21):

```html
<link rel="stylesheet" href="/static/css/inventory/kb-active.css">
```

Add after the `dialog.css` link (line 30):

```html
<link rel="stylesheet" href="/static/css/dialogs/keyboard-help.css">
```

Add `keyboard.js` as the last `<script>` tag, after `notes.js` (line 57):

```html
<script src="/static/js/shared/keyboard.js" defer></script>
```

- [ ] **Step 4: Commit**

```bash
git add internal/embedded/static/css/dialogs/keyboard-help.css \
        internal/embedded/static/css/inventory/kb-active.css \
        internal/embedded/templates/layouts/base.html
git commit -m "feat(ui): add CSS for keyboard navigation and help overlay (#31)"
```

---

### Task 4: Central Keyboard Dispatcher

**Files:**
- Create: `internal/embedded/static/js/shared/keyboard.js`

This is the main implementation file. It contains: dispatcher, all handlers, list navigation, container navigator, and help overlay.

- [ ] **Step 1: Create keyboard.js with dispatcher core + simple handlers**

File: `internal/embedded/static/js/shared/keyboard.js`

Start with the IIFE skeleton, shortcut map, dispatcher logic, and the simpler handlers (`/`, `Ctrl+K`, `i`, `c`, `s`, `a`, `Escape`):

```js
(function () {
  var qlx = window.qlx = window.qlx || {};

  // ── Shortcut definitions ────────────────────────────────────────────────
  // Each entry: { key, ctrl?, handler, label, group, global?, context?, allowInInput? }
  // `label` is an i18n key. `group` is used for help overlay grouping.
  var shortcuts = [
    { key: "/",      handler: focusSearch,         label: "keyboard.focus_search",    group: "nav",     global: true },
    { key: "k",      ctrl: true, handler: focusSearch, label: "keyboard.focus_search", group: "nav",   global: true },
    { key: "m",      handler: openContainerNav,    label: "keyboard.go_to_container", group: "nav",     global: true },
    { key: "i",      handler: focusItemEntry,      label: "keyboard.new_item",        group: "action",  context: "container" },
    { key: "c",      handler: focusContainerEntry, label: "keyboard.new_container",   group: "action",  context: "container" },
    { key: "s",      handler: toggleSelection,     label: "keyboard.selection_mode",  group: "action",  context: "container" },
    { key: "a",      handler: toggleSelectAll,     label: "keyboard.select_all",      group: "action",  context: "selection" },
    { key: "?",      handler: showHelp,            label: "keyboard.help",            group: "general", global: true },
    { key: "Escape", handler: handleEscape,        label: "keyboard.close",           group: "general", global: true, allowInInput: true }
  ];

  // ── Context detection ───────────────────────────────────────────────────
  function getContext() {
    var content = document.getElementById("content");
    if (!content) return "";
    if (qlx.isSelectionMode && qlx.isSelectionMode()) return "selection";
    if (content.querySelector(".container-view")) return "container";
    return "";
  }

  function isInputFocused() {
    var el = document.activeElement;
    if (!el) return false;
    var tag = el.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
    if (el.getAttribute("contenteditable") === "true") return true;
    return false;
  }

  function isDialogOpen() {
    return !!document.querySelector("dialog[open]");
  }

  // ── Dispatcher ──────────────────────────────────────────────────────────
  document.addEventListener("keydown", function (e) {
    var inInput = isInputFocused();
    var dialogOpen = isDialogOpen();

    for (var i = 0; i < shortcuts.length; i++) {
      var s = shortcuts[i];
      // Match key
      if (e.key !== s.key) continue;
      // Match ctrl modifier
      if (s.ctrl && !(e.ctrlKey || e.metaKey)) continue;
      if (!s.ctrl && (e.ctrlKey || e.metaKey) && s.key !== "Escape") continue;

      // Gate: input focus
      if (inInput && !s.allowInInput && !(s.ctrl)) continue;
      // Gate: dialog open — only Escape passes
      if (dialogOpen && s.key !== "Escape") continue;

      // Gate: context
      if (s.context) {
        var ctx = getContext();
        if (s.context === "container" && ctx !== "container" && ctx !== "selection") continue;
        if (s.context === "selection" && ctx !== "selection") continue;
      }

      e.preventDefault();
      s.handler(e);
      return;
    }

    // List navigation (not in shortcut map — arrow keys + Enter)
    if (!inInput && !dialogOpen) {
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        // Only on container/item views
        if (getContext() === "container" || getContext() === "selection") {
          e.preventDefault();
          navigateList(e.key === "ArrowDown" ? 1 : -1);
        }
      } else if (e.key === "Enter") {
        var active = document.querySelector(".kb-active");
        if (active) {
          e.preventDefault();
          openActiveItem();
        }
      }
    }
  });

  // ── Handlers ────────────────────────────────────────────────────────────
  function focusSearch() {
    var search = document.getElementById("global-search");
    if (search) search.focus();
  }

  function focusItemEntry() {
    var input = document.querySelector(".items .quick-entry input[name='name']");
    if (input) /** @type {HTMLElement} */ (input).focus();
  }

  function focusContainerEntry() {
    var input = document.querySelector(".containers .quick-entry input[name='name']");
    if (input) /** @type {HTMLElement} */ (input).focus();
  }

  function toggleSelection() {
    if (qlx.toggleSelectionMode) qlx.toggleSelectionMode();
  }

  function toggleSelectAll() {
    if (qlx.selectAll) qlx.selectAll();
  }

  function handleEscape() {
    // Priority 1: If dialog open, native behavior handles it (we don't preventDefault on open dialog Escape actually—but we already did above, so close manually)
    var openDialog = document.querySelector("dialog[open]");
    if (openDialog) {
      /** @type {HTMLDialogElement} */ (openDialog).close();
      return;
    }
    // Priority 2: Blur focused input
    if (isInputFocused()) {
      /** @type {HTMLElement} */ (document.activeElement).blur();
      return;
    }
    // Priority 3: Exit selection mode
    if (qlx.isSelectionMode && qlx.isSelectionMode()) {
      if (qlx.toggleSelectionMode) qlx.toggleSelectionMode();
      return;
    }
    // Priority 4: Remove list highlight
    clearListHighlight();
  }

  // ── List Navigation ─────────────────────────────────────────────────────
  var activeListIndex = -1;

  function getNavigableItems() {
    var items = [];
    var containerItems = document.querySelectorAll("#container-list > li:not(.empty-state)");
    var itemItems = document.querySelectorAll("#item-list > li:not(.empty-state)");
    for (var i = 0; i < containerItems.length; i++) items.push(containerItems[i]);
    for (var j = 0; j < itemItems.length; j++) items.push(itemItems[j]);
    return items;
  }

  function navigateList(direction) {
    var items = getNavigableItems();
    if (items.length === 0) return;

    // Clear previous
    for (var i = 0; i < items.length; i++) items[i].classList.remove("kb-active");

    activeListIndex += direction;
    if (activeListIndex < 0) activeListIndex = 0;
    if (activeListIndex >= items.length) activeListIndex = items.length - 1;

    items[activeListIndex].classList.add("kb-active");
    items[activeListIndex].scrollIntoView({ block: "nearest" });
  }

  function openActiveItem() {
    var active = document.querySelector(".kb-active");
    if (!active) return;
    var link = active.querySelector("a");
    if (link) link.click();
  }

  function clearListHighlight() {
    activeListIndex = -1;
    var highlighted = document.querySelectorAll(".kb-active");
    for (var i = 0; i < highlighted.length; i++) highlighted[i].classList.remove("kb-active");
  }

  // Reset on HTMX swap
  document.body.addEventListener("htmx:afterSwap", function (e) {
    if (!e.detail || !e.detail.target) return;
    if (e.detail.target.id !== "content") return;
    clearListHighlight();
  });

  // ── Container Navigator ─────────────────────────────────────────────────
  var navPicker = null;

  function openContainerNav() {
    if (!navPicker && qlx.createTreePicker) {
      navPicker = qlx.createTreePicker({
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
    }
    if (navPicker) navPicker.open();
  }

  // ── Help Overlay ────────────────────────────────────────────────────────
  var helpDialog = null;

  var helpLayout = [
    {
      group: "keyboard.nav_group",
      items: [
        { keys: ["/", "Ctrl+K"], label: "keyboard.focus_search" },
        { keys: ["m"],           label: "keyboard.go_to_container" },
        { keys: ["\u2191 \u2193"], label: "keyboard.navigate" },
        { keys: ["Enter"],       label: "keyboard.open_selected" }
      ]
    },
    {
      group: "keyboard.action_group",
      items: [
        { keys: ["i"], label: "keyboard.new_item" },
        { keys: ["c"], label: "keyboard.new_container" },
        { keys: ["s"], label: "keyboard.selection_mode" },
        { keys: ["a"], label: "keyboard.select_all" }
      ]
    },
    {
      group: "keyboard.general_group",
      items: [
        { keys: ["?"],   label: "keyboard.help" },
        { keys: ["Esc"], label: "keyboard.close" }
      ]
    }
  ];

  function showHelp() {
    if (!helpDialog) {
      helpDialog = document.createElement("dialog");
      helpDialog.id = "keyboard-help";

      var title = document.createElement("h3");
      title.textContent = qlx.t("keyboard.help");
      helpDialog.appendChild(title);

      for (var g = 0; g < helpLayout.length; g++) {
        var group = helpLayout[g];
        var section = document.createElement("div");
        section.className = "kb-help-group";

        var groupTitle = document.createElement("div");
        groupTitle.className = "kb-help-group-title";
        groupTitle.textContent = qlx.t(group.group);
        section.appendChild(groupTitle);

        for (var r = 0; r < group.items.length; r++) {
          var item = group.items[r];
          var row = document.createElement("div");
          row.className = "kb-help-row";

          for (var k = 0; k < item.keys.length; k++) {
            var kbd = document.createElement("kbd");
            kbd.textContent = item.keys[k];
            row.appendChild(kbd);
          }

          var desc = document.createElement("span");
          desc.textContent = qlx.t(item.label);
          row.appendChild(desc);

          section.appendChild(row);
        }

        helpDialog.appendChild(section);
      }

      // Close on backdrop click
      helpDialog.addEventListener("click", function (e) {
        if (e.target === helpDialog) helpDialog.close();
      });

      document.body.appendChild(helpDialog);
    }
    helpDialog.showModal();
  }
})();
```

- [ ] **Step 2: Verify the app builds and loads**

Run: `make build-mac`
Expected: Successful build

- [ ] **Step 3: Commit**

```bash
git add internal/embedded/static/js/shared/keyboard.js
git commit -m "feat(ui): add keyboard shortcuts dispatcher with all handlers (#31)"
```

---

### Task 5: E2E Tests

**Files:**
- Create: `e2e/tests/keyboard.spec.ts`

**Important context for the implementer:**
- Import `test, expect` from `../fixtures/app` (NOT from `@playwright/test`)
- The fixture sets `lang=pl` cookie — i18n values will be Polish
- Use `app.baseURL` for navigation
- HTMX flows: use `page.waitForResponse()` + DOM assertions
- Need a container with items for most tests — create them via API first or quick-entry

- [ ] **Step 1: Create E2E test file**

File: `e2e/tests/keyboard.spec.ts`

```typescript
import { test, expect } from '../fixtures/app';

test.describe('Keyboard Shortcuts', () => {

  test('/ focuses global search', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.keyboard.press('/');
    await expect(page.locator('#global-search')).toBeFocused();
  });

  test('Ctrl+K focuses global search', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.keyboard.press('Control+k');
    await expect(page.locator('#global-search')).toBeFocused();
  });

  test('? opens help overlay', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.keyboard.press('Shift+/');  // ? = Shift+/
    await expect(page.locator('#keyboard-help')).toBeVisible();
  });

  test('Escape closes help overlay', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.keyboard.press('Shift+/');
    await expect(page.locator('#keyboard-help')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.locator('#keyboard-help')).not.toBeVisible();
  });

  test('Escape blurs focused input', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.locator('#global-search').focus();
    await expect(page.locator('#global-search')).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(page.locator('#global-search')).not.toBeFocused();
  });

  test('shortcuts ignored when input is focused', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.locator('#global-search').focus();
    await page.keyboard.press('s');
    // s should type into input, not trigger selection mode
    await expect(page.locator('#content')).not.toHaveClass(/selection-mode/);
  });

  test('shortcuts ignored when dialog is open', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.keyboard.press('Shift+/');  // open help
    await expect(page.locator('#keyboard-help')).toBeVisible();
    await page.keyboard.press('s');  // should not trigger selection mode
    await expect(page.locator('#content')).not.toHaveClass(/selection-mode/);
    await page.keyboard.press('Escape');  // close help
    await expect(page.locator('#keyboard-help')).not.toBeVisible();
  });

  test('m opens container navigator', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.keyboard.press('m');
    await expect(page.locator('#container-nav-picker')).toBeVisible();
  });
});

test.describe('Keyboard Shortcuts — Container View', () => {
  test.describe.configure({ mode: 'serial' });

  let containerName: string;

  test('setup: create container with items', async ({ page, app }) => {
    containerName = `KB Test ${Date.now()}`;
    await page.goto(`${app.baseURL}/`);
    await page.fill('.containers .quick-entry input[name="name"]', containerName);
    const resp = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.press('.containers .quick-entry input[name="name"]', 'Enter');
    await resp;
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);

    // Add two items
    for (const name of ['Item A', 'Item B']) {
      await page.fill('.items .quick-entry input[name="name"]', name);
      const itemResp = page.waitForResponse(r => r.url().includes('/items') && r.request().method() === 'POST');
      await page.press('.items .quick-entry input[name="name"]', 'Enter');
      await itemResp;
    }
    await expect(page.locator('#item-list li:not(.empty-state)')).toHaveCount(2);
  });

  test('i focuses item quick-entry', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.keyboard.press('i');
    await expect(page.locator('.items .quick-entry input[name="name"]')).toBeFocused();
  });

  test('c focuses container quick-entry', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.keyboard.press('c');
    await expect(page.locator('.containers .quick-entry input[name="name"]')).toBeFocused();
  });

  test('s toggles selection mode', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    // Click somewhere neutral to ensure body focus
    await page.locator('h2').click();
    await page.keyboard.press('s');
    await expect(page.locator('#content')).toHaveClass(/selection-mode/);
  });

  test('a selects all in selection mode', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.locator('h2').click();
    await page.keyboard.press('s');
    await expect(page.locator('#content')).toHaveClass(/selection-mode/);
    await page.keyboard.press('a');
    const checkboxes = page.locator('.bulk-select');
    const count = await checkboxes.count();
    for (let i = 0; i < count; i++) {
      await expect(checkboxes.nth(i)).toBeChecked();
    }
  });

  test('Escape exits selection mode', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.locator('h2').click();
    await page.keyboard.press('s');
    await expect(page.locator('#content')).toHaveClass(/selection-mode/);
    await page.keyboard.press('Escape');
    await expect(page.locator('#content')).not.toHaveClass(/selection-mode/);
  });

  test('arrow keys navigate list items', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.locator('h2').click();
    await page.keyboard.press('ArrowDown');
    // First navigable item should be highlighted
    const firstItem = page.locator('#item-list li:not(.empty-state)').first();
    await expect(firstItem).toHaveClass(/kb-active/);
    await page.keyboard.press('ArrowDown');
    const secondItem = page.locator('#item-list li:not(.empty-state)').nth(1);
    await expect(secondItem).toHaveClass(/kb-active/);
    // First should no longer be active
    await expect(firstItem).not.toHaveClass(/kb-active/);
  });

  test('Enter opens highlighted item', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.locator('h2').click();
    await page.keyboard.press('ArrowDown');
    const responsePromise = page.waitForResponse(r => r.url().includes('/items/'));
    await page.keyboard.press('Enter');
    await responsePromise;
    // Should have navigated to item detail
    await expect(page.locator('h2')).toContainText('Item');
  });

  test('Escape clears list highlight', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.locator('h2').click();
    await page.keyboard.press('ArrowDown');
    await expect(page.locator('.kb-active')).toHaveCount(1);
    await page.keyboard.press('Escape');
    await expect(page.locator('.kb-active')).toHaveCount(0);
  });
});
```

- [ ] **Step 2: Run E2E tests**

Run: `make test-e2e` (or `cd e2e && npx playwright test keyboard.spec.ts`)
Expected: All tests pass

- [ ] **Step 3: Fix any failures and re-run**

Debug with: `make test-e2e-ui` for interactive mode

- [ ] **Step 4: Commit**

```bash
git add e2e/tests/keyboard.spec.ts
git commit -m "test(e2e): add keyboard shortcuts E2E tests (#31)"
```
