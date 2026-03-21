# Batch Refactoring: Deduplication & Clean Interfaces — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate code duplication and tighten interfaces across Go backend and JS frontend (issues #35, #36, #37, #40, #41 + JS tree picker dedup).

**Architecture:** Six independent refactoring changes touching `internal/print/`, `internal/ui/`, `internal/api/`, `internal/shared/webutil/`, and frontend JS. Each change is self-contained and can be verified independently.

**Tech Stack:** Go 1.22+, vanilla JS (IIFE modules), HTML templates, HTMX

**Spec:** `docs/superpowers/specs/2026-03-21-batch-refactoring-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|---------------|
| Modify | `internal/print/manager.go` | Tasks 1, 2: extract `findModel`, add `PrinterConfigStore` interface |
| Verify | `cmd/qlx/main.go:66` | Task 2: calls `NewPrinterManager(s)` — implicit interface satisfaction |
| Verify | `internal/app/server.go:19` | Task 2: receives `*PrinterManager` — no change needed |
| Create | `internal/shared/webutil/format.go` | Task 3: `FormatContainerPath` helper |
| Create | `internal/shared/webutil/format_test.go` | Task 3: tests |
| Modify | `internal/api/server.go` | Task 3: use `FormatContainerPath` |
| Modify | `internal/ui/handlers.go` | Task 3: use `FormatContainerPath` |
| Modify | `internal/ui/server.go` | Task 4: wire `resolveTagIDs` callback |
| Modify | `internal/ui/handlers_tags.go` | Task 4: remove duplicate; Task 5: `parent_id` |
| Modify | `internal/embedded/templates/pages/tags/tags.html` | Task 5: `parent` → `parent_id` |
| Modify | `internal/embedded/templates/partials/tags/tag_list_item.html` | Task 5: `parent` → `parent_id` |
| Modify | `internal/embedded/templates/pages/search/search.html` | Task 5: `parent` → `parent_id` |
| Create | `internal/embedded/static/js/shared/tree-picker.js` | Task 6: shared factory |
| Modify | `internal/embedded/static/js/inventory/move-picker.js` | Task 6: simplify |
| Modify | `internal/embedded/static/js/tags/tag-picker.js` | Task 6: simplify |
| Modify | `internal/embedded/templates/layouts/base.html` | Task 6: add script tag |

---

## Task 1: Extract `findModel` helper (#37)

**Files:**
- Modify: `internal/print/manager.go`

- [ ] **Step 1: Add `findModel` helper function**

Add after line 48 (after `NewPrinterManager`):

```go
// findModel returns the ModelInfo matching modelID, or nil if not found.
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

- [ ] **Step 2: Replace block in `ConnectPrinter` (lines 109-116)**

Replace:
```go
	// Find model info for DPI/width
	var modelInfo *encoder.ModelInfo
	for _, mi := range enc.Models() {
		if mi.ID == cfg.Model {
			info := mi
			modelInfo = &info
			break
		}
	}
```

With:
```go
	modelInfo := findModel(enc, cfg.Model)
```

Note: `ConnectPrinter` tolerates nil modelInfo — no error check needed.

- [ ] **Step 3: Replace block in `Print` (lines 187-197)**

Replace:
```go
	var modelInfo *encoder.ModelInfo
	for _, mi := range enc.Models() {
		if mi.ID == cfg.Model {
			info := mi
			modelInfo = &info
			break
		}
	}
	if modelInfo == nil {
		return fmt.Errorf("model not found: %s", cfg.Model)
	}
```

With:
```go
	modelInfo := findModel(enc, cfg.Model)
	if modelInfo == nil {
		return fmt.Errorf("model not found: %s", cfg.Model)
	}
```

- [ ] **Step 4: Replace block in `PrintImage` (lines 244-254)**

Same replacement as Step 3 — replace the identical block with `findModel` + nil check.

- [ ] **Step 5: Run tests**

Run: `make test`
Expected: all tests pass, no behavior change.

- [ ] **Step 6: Run lint**

Run: `make lint`
Expected: no new warnings.

- [ ] **Step 7: Commit**

```bash
git add internal/print/manager.go
git commit -m "refactor(print): extract findModel helper (#37)"
```

---

## Task 2: Narrow `PrinterManager` dependency to interface (#40)

**Files:**
- Modify: `internal/print/manager.go`
- Verify: `cmd/qlx/main.go` (no change needed — implicit interface satisfaction)

- [ ] **Step 1: Add `PrinterConfigStore` interface**

Add before `PrinterManager` struct definition (before line 29):

```go
// PrinterConfigStore provides read-only access to printer configuration.
type PrinterConfigStore interface {
	GetPrinter(id string) *store.PrinterConfig
	AllPrinters() []store.PrinterConfig
}
```

- [ ] **Step 2: Change `PrinterManager.store` field type**

In the `PrinterManager` struct (line 30), change:
```go
store       *store.Store
```
To:
```go
store       PrinterConfigStore
```

- [ ] **Step 3: Change `NewPrinterManager` parameter type**

Change line 39:
```go
func NewPrinterManager(s *store.Store) *PrinterManager {
```
To:
```go
func NewPrinterManager(s PrinterConfigStore) *PrinterManager {
```

- [ ] **Step 4: Remove unused `store` import if possible**

Check if `store` is still imported for other types (e.g., `store.PrinterConfig` in the interface). It will still be needed — the interface references `*store.PrinterConfig`. No action needed.

- [ ] **Step 5: Run tests**

Run: `make test`
Expected: all tests pass. `*store.Store` satisfies `PrinterConfigStore` implicitly.

- [ ] **Step 6: Run lint**

Run: `make lint`
Expected: no new warnings.

- [ ] **Step 7: Commit**

```bash
git add internal/print/manager.go
git commit -m "refactor(print): narrow PrinterManager to PrinterConfigStore interface (#40)"
```

---

## Task 3: Extract `FormatContainerPath` helper (#36)

**Files:**
- Create: `internal/shared/webutil/format.go`
- Create: `internal/shared/webutil/format_test.go`
- Modify: `internal/api/server.go`
- Modify: `internal/ui/handlers.go`

- [ ] **Step 1: Write test file**

Create `internal/shared/webutil/format_test.go`:

```go
package webutil

import (
	"testing"

	"github.com/erxyi/qlx/internal/store"
)

func TestFormatContainerPath(t *testing.T) {
	tests := []struct {
		name string
		path []store.Container
		sep  string
		want string
	}{
		{
			name: "empty path",
			path: nil,
			sep:  " → ",
			want: "",
		},
		{
			name: "single container",
			path: []store.Container{{Name: "Box A"}},
			sep:  " → ",
			want: "Box A",
		},
		{
			name: "multiple containers unicode arrow",
			path: []store.Container{{Name: "Room"}, {Name: "Shelf"}, {Name: "Box"}},
			sep:  " → ",
			want: "Room → Shelf → Box",
		},
		{
			name: "multiple containers ASCII arrow for CSV",
			path: []store.Container{{Name: "Room"}, {Name: "Shelf"}},
			sep:  " -> ",
			want: "Room -> Shelf",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatContainerPath(tt.path, tt.sep)
			if got != tt.want {
				t.Errorf("FormatContainerPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/shared/webutil/ -run TestFormatContainerPath -v`
Expected: FAIL — `FormatContainerPath` not defined.

- [ ] **Step 3: Write implementation**

Create `internal/shared/webutil/format.go`:

```go
package webutil

import (
	"strings"

	"github.com/erxyi/qlx/internal/store"
)

// FormatContainerPath joins container names with the given separator.
func FormatContainerPath(path []store.Container, sep string) string {
	names := make([]string, len(path))
	for i, c := range path {
		names[i] = c.Name
	}
	return strings.Join(names, sep)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/shared/webutil/ -run TestFormatContainerPath -v`
Expected: PASS

- [ ] **Step 5: Replace in `api/server.go` — CSV export (lines 331-336)**

Replace:
```go
		path := s.inventory.ContainerPath(item.ContainerID)
		var pathStrs []string
		for _, c := range path {
			pathStrs = append(pathStrs, c.Name)
		}
		pathStr := strings.Join(pathStrs, " -> ")
```

With:
```go
		path := s.inventory.ContainerPath(item.ContainerID)
		pathStr := webutil.FormatContainerPath(path, " -> ")
```

- [ ] **Step 6: Replace in `api/server.go` — HandlePrint (lines 426-431)**

Replace:
```go
	path := s.inventory.ContainerPath(item.ContainerID)
	var pathStrs []string
	for _, c := range path {
		pathStrs = append(pathStrs, c.Name)
	}
```

And the `Location` line using `strings.Join(pathStrs, " → ")` with:

```go
	path := s.inventory.ContainerPath(item.ContainerID)
```

And change `Location` to:
```go
		Location:    webutil.FormatContainerPath(path, " → "),
```

- [ ] **Step 7: Replace in `ui/handlers.go` — HandleItemPrint (lines 250-255)**

Replace:
```go
	path := s.inventory.ContainerPath(item.ContainerID)
	pathParts := make([]string, 0, len(path))
	for _, c := range path {
		pathParts = append(pathParts, c.Name)
	}
```

And change `Location` to:
```go
		Location:    webutil.FormatContainerPath(path, " → "),
```

Remove the `pathParts` variable entirely.

- [ ] **Step 8: Replace in `ui/handlers.go` — HandleContainerItemsJSON (lines 550-557)**

Replace:
```go
		path := s.inventory.ContainerPath(item.ContainerID)
		var parts []string
		for _, c := range path {
			parts = append(parts, c.Name)
		}
```

And change `"location"` to:
```go
			"location": webutil.FormatContainerPath(path, " → "),
```

- [ ] **Step 9: Add `webutil` import where needed**

Ensure `internal/shared/webutil` is imported in `api/server.go` and `ui/handlers.go`. Remove unused `strings` import if it was only used for these joins (unlikely — verify).

- [ ] **Step 10: Run tests**

Run: `make test`
Expected: all tests pass.

- [ ] **Step 11: Run lint**

Run: `make lint`
Expected: no new warnings.

- [ ] **Step 12: Commit**

```bash
git add internal/shared/webutil/format.go internal/shared/webutil/format_test.go internal/api/server.go internal/ui/handlers.go
git commit -m "refactor: extract FormatContainerPath helper (#36)"
```

---

## Task 4: Deduplicate `resolveTagIDs` (#35)

**Files:**
- Modify: `internal/ui/server.go` (lines 136-160)
- Modify: `internal/ui/handlers_tags.go` (lines 115-124)

- [ ] **Step 1: Change `loadTemplates` and `loadLayout` to accept a callback**

In `server.go`, change `loadTemplates` (line 136):

```go
func loadTemplates(resolveTagsFn func([]string) []store.Tag) map[string]*template.Template {
	layoutTmpl := loadLayout(resolveTagsFn)
	mergeHTMLDir(layoutTmpl, "templates/partials")
	mergeHTMLDir(layoutTmpl, "templates/components")
	return discoverPages(layoutTmpl)
}
```

Change `loadLayout` (line 143):

```go
func loadLayout(resolveTagsFn func([]string) []store.Tag) *template.Template {
	content, err := embedded.Templates.ReadFile("templates/layouts/base.html")
	if err != nil {
		panic(err)
	}
	return template.Must(template.New("layout").Funcs(template.FuncMap{
		"dict":        dict,
		"resolveTags": resolveTagsFn,
	}).Parse(string(content)))
}
```

- [ ] **Step 2: Update `NewServer` to pass the callback**

In `NewServer` (line 114), change:
```go
	templates := loadTemplates(s)
```

To:
```go
	resolveTagsFn := func(ids []string) []store.Tag {
		tags := make([]store.Tag, 0, len(ids))
		for _, id := range ids {
			if t := tagsSvc.GetTag(id); t != nil {
				tags = append(tags, *t)
			}
		}
		return tags
	}
	templates := loadTemplates(resolveTagsFn)
```

Note: uses `tagsSvc.GetTag(id)` (the `*service.TagService` parameter), NOT `s.GetTag()` — `*ui.Server` has no `GetTag` method.

- [ ] **Step 3: Verify `resolveTagIDs` method is kept and consistent**

`resolveTagIDs` in `handlers_tags.go` (line 116) stays as-is — it's used by `HandleItemTagAdd`, `HandleItemTagRemove`, `HandleContainerTagAdd`, `HandleContainerTagRemove`. It calls `s.tags.GetTag(id)` which is the same underlying path as the new FuncMap callback (`tagsSvc.GetTag`). The duplication is eliminated because `loadLayout` no longer has its own independent copy of the loop.

No code change needed — just verify `resolveTagIDs` still exists at line 116 and is unchanged.

- [ ] **Step 4: Run tests**

Run: `make test`
Expected: all tests pass.

- [ ] **Step 5: Run lint**

Run: `make lint`
Expected: no new warnings.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/server.go
git commit -m "refactor(ui): deduplicate resolveTagIDs via callback (#35)"
```

---

## Task 5: Standardize `parent_id` query param (#41)

**Files:**
- Modify: `internal/ui/handlers_tags.go` (lines 12, 56, 75, 97)
- Modify: `internal/embedded/templates/pages/tags/tags.html` (lines 9, 17)
- Modify: `internal/embedded/templates/partials/tags/tag_list_item.html` (line 3)
- Modify: `internal/embedded/templates/pages/search/search.html` (line 35)

- [ ] **Step 1: Update Go handler — `HandleTags` (line 12)**

Change:
```go
	parentID := r.URL.Query().Get("parent")
```
To:
```go
	parentID := r.URL.Query().Get("parent_id")
```

- [ ] **Step 2: Update Go handler — `HandleTagCreate` redirect (line 56)**

Change:
```go
	http.Redirect(w, r, "/ui/tags?parent="+parentID, http.StatusSeeOther)
```
To:
```go
	http.Redirect(w, r, "/ui/tags?parent_id="+parentID, http.StatusSeeOther)
```

- [ ] **Step 3: Update Go handler — `HandleTagUpdate` redirect (line 75)**

Same change as Step 2.

- [ ] **Step 4: Update Go handler — `HandleTagDelete` redirect (line 97)**

Same change as Step 2.

- [ ] **Step 5: Update template `tags.html` (lines 9, 17)**

Replace all occurrences of `?parent={{ .ID }}` with `?parent_id={{ .ID }}` (both `href` and `hx-get` attributes on both lines).

- [ ] **Step 6: Update template `tag_list_item.html` (line 3)**

Replace `?parent={{ .Data.ID }}` with `?parent_id={{ .Data.ID }}` (both `href` and `hx-get`).

- [ ] **Step 7: Update template `search.html` (line 35)**

Replace `?parent={{ .ID }}` with `?parent_id={{ .ID }}` (both `href` and `hx-get`).

- [ ] **Step 8: Verify no other `?parent=` references for tags**

Run: `grep -r "?parent=" internal/embedded/templates/ internal/ui/ internal/api/`
Expected: no remaining hits (except `parent_id=` which is correct).

- [ ] **Step 9: Run tests**

Run: `make test`
Expected: all tests pass.

- [ ] **Step 10: Run E2E tests (if tag tests exist)**

Run: `cd e2e && npx playwright test tests/tags.spec.ts`
Expected: pass (tests use API, not URL params — but verify).

- [ ] **Step 11: Run lint**

Run: `make lint`
Expected: no new warnings.

- [ ] **Step 12: Commit**

```bash
git add internal/ui/handlers_tags.go internal/embedded/templates/pages/tags/tags.html internal/embedded/templates/partials/tags/tag_list_item.html internal/embedded/templates/pages/search/search.html
git commit -m "fix(ui): standardize tag query param to parent_id (#41)"
```

---

## Task 6: JS tree picker factory (deduplication)

**Files:**
- Create: `internal/embedded/static/js/shared/tree-picker.js`
- Modify: `internal/embedded/static/js/inventory/move-picker.js`
- Modify: `internal/embedded/static/js/tags/tag-picker.js`
- Modify: `internal/embedded/templates/layouts/base.html`

- [ ] **Step 1: Create `tree-picker.js` with shared factory**

Create `internal/embedded/static/js/shared/tree-picker.js`:

```js
(function () {
  var qlx = window.qlx = window.qlx || {};

  /**
   * Expand or collapse a tree node, fetching children from the given endpoint.
   * @param {Element} expandEl - the clicked expand toggle
   * @param {HTMLElement} treeContainer - the tree root container
   * @param {string} endpoint - base URL for fetching children (e.g. "/ui/partials/tree")
   */
  function handleTreeExpand(expandEl, treeContainer, endpoint) {
    var li = expandEl.closest(".tree-node");
    if (!li) return;
    var id = li.getAttribute("data-id");
    var ul = li.querySelector("ul.tree-branch");

    if (ul && ul.children.length > 0) {
      ul.style.display = ul.style.display === "none" ? "" : "none";
      expandEl.textContent = ul.style.display === "none" ? "\u25B6" : "\u25BC";
      return;
    }

    var url = endpoint + "?parent_id=" + encodeURIComponent(id);

    fetch(url)
      .then(function (r) { return r.text(); })
      .then(function (html) {
        if (!ul) {
          ul = document.createElement("ul");
          ul.className = "tree-branch";
          li.appendChild(ul);
        }
        ul.textContent = "";
        var parser = new DOMParser();
        var doc = parser.parseFromString(html, "text/html");
        while (doc.body.firstChild) {
          ul.appendChild(doc.body.firstChild);
        }
        if (window.htmx) htmx.process(ul);
        expandEl.textContent = "\u25BC";
      })
      .catch(function (err) {
        console.error("tree expand failed:", err);
      });
  }

  /**
   * Create a tree picker dialog.
   * @param {Object} config
   * @param {string} config.id - dialog element id
   * @param {string|function(): string} config.title - dialog heading text (string or getter fn for deferred i18n)
   * @param {string} config.endpoint - tree data endpoint
   * @param {string} config.searchEndpoint - search endpoint for hx-get
   * @param {string|function(): string} config.searchPlaceholder - placeholder text (string or getter fn)
   * @param {string|function(): string} config.confirmLabel - confirm button text (string or getter fn)
   * @param {function(string): void} config.onConfirm - called with selected node data-id
   * @returns {{ open: function(): void }}
   */
  qlx.createTreePicker = function createTreePicker(config) {
    /** @type {string|null} */
    var selectedId = null;
    var treeContainerId = config.id + "-tree-container";
    var confirmBtnId = config.id + "-confirm";

    /** Resolve a config value that may be a string or a getter function. */
    function resolve(val) {
      return typeof val === "function" ? val() : val;
    }

    function getOrCreateDialog() {
      var dlg = document.getElementById(config.id);
      if (dlg) return /** @type {HTMLDialogElement} */ (dlg);

      dlg = document.createElement("dialog");
      dlg.id = config.id;

      var picker = document.createElement("div");
      picker.className = "tree-picker";

      var title = document.createElement("h3");
      title.textContent = resolve(config.title);
      picker.appendChild(title);

      var searchInput = document.createElement("input");
      searchInput.type = "text";
      searchInput.className = "tree-search";
      searchInput.placeholder = resolve(config.searchPlaceholder);
      searchInput.setAttribute("hx-get", config.searchEndpoint);
      searchInput.setAttribute("hx-trigger", "input changed delay:300ms");
      searchInput.setAttribute("hx-target", "#" + treeContainerId);
      picker.appendChild(searchInput);

      var treeContainer = document.createElement("div");
      treeContainer.id = treeContainerId;
      treeContainer.style.flex = "1";
      treeContainer.style.overflowY = "auto";
      picker.appendChild(treeContainer);

      var footer = document.createElement("div");
      footer.className = "tree-picker-footer";

      var cancelBtn = document.createElement("button");
      cancelBtn.className = "btn btn-secondary btn-small";
      cancelBtn.textContent = qlx.t("action.cancel");
      cancelBtn.type = "button";
      cancelBtn.addEventListener("click", function () {
        /** @type {HTMLDialogElement} */ (dlg).close();
      });
      footer.appendChild(cancelBtn);

      var confirmBtn = document.createElement("button");
      confirmBtn.className = "btn btn-primary btn-small";
      confirmBtn.textContent = resolve(config.confirmLabel);
      confirmBtn.type = "button";
      confirmBtn.disabled = true;
      confirmBtn.id = confirmBtnId;
      confirmBtn.addEventListener("click", function () {
        if (selectedId) {
          config.onConfirm(selectedId);
          /** @type {HTMLDialogElement} */ (dlg).close();
        }
      });
      footer.appendChild(confirmBtn);

      picker.appendChild(footer);
      dlg.appendChild(picker);
      document.body.appendChild(dlg);

      // Delegate click events for tree nodes
      treeContainer.addEventListener("click", function (e) {
        var expandEl = /** @type {HTMLElement} */ (e.target).closest(".tree-expand");
        if (expandEl) {
          handleTreeExpand(expandEl, treeContainer, config.endpoint);
          return;
        }
        var labelEl = /** @type {HTMLElement} */ (e.target).closest(".tree-label");
        if (labelEl) {
          treeContainer.querySelectorAll(".tree-label.selected").forEach(function (el) {
            el.classList.remove("selected");
          });
          labelEl.classList.add("selected");

          var li = labelEl.closest(".tree-node");
          selectedId = li ? li.getAttribute("data-id") : null;

          var btn = document.getElementById(confirmBtnId);
          if (btn) /** @type {HTMLButtonElement} */ (btn).disabled = !selectedId;
        }
      });

      return /** @type {HTMLDialogElement} */ (dlg);
    }

    return {
      open: function () {
        selectedId = null;
        var dlg = getOrCreateDialog();
        var confirmBtn = document.getElementById(confirmBtnId);
        if (confirmBtn) /** @type {HTMLButtonElement} */ (confirmBtn).disabled = true;

        var treeContainer = document.getElementById(treeContainerId);
        if (treeContainer) {
          treeContainer.querySelectorAll(".tree-label.selected").forEach(function (el) {
            el.classList.remove("selected");
          });
          treeContainer.textContent = "";
        }

        // Load root tree
        fetch(config.endpoint + "?parent_id=")
          .then(function (r) { return r.text(); })
          .then(function (html) {
            if (treeContainer) {
              treeContainer.textContent = "";
              var parser = new DOMParser();
              var doc = parser.parseFromString(html, "text/html");
              while (doc.body.firstChild) {
                treeContainer.appendChild(doc.body.firstChild);
              }
              if (window.htmx) htmx.process(treeContainer);
            }
          })
          .catch(function (err) {
            console.error("tree load failed:", err);
          });

        dlg.showModal();
      }
    };
  };
})();
```

- [ ] **Step 2: Add script tag in `base.html`**

In `internal/embedded/templates/layouts/base.html`, add after line 35 (after `htmx-hooks.js`):
```html
    <script src="/static/js/shared/tree-picker.js" defer></script>
```

- [ ] **Step 3: Simplify `move-picker.js`**

Replace entire file `internal/embedded/static/js/inventory/move-picker.js` with:

**Important:** `qlx.t()` calls are wrapped in functions (not evaluated eagerly) because translations may not be loaded yet when `defer` scripts execute. The existing pickers deferred these lookups to lazy dialog creation; the factory must do the same via getter functions.

```js
(function () {
  var qlx = window.qlx = window.qlx || {};

  var picker = qlx.createTreePicker({
    id: "move-picker",
    title: function () { return qlx.t("inventory.move_to_container"); },
    endpoint: "/ui/partials/tree",
    searchEndpoint: "/ui/partials/tree/search",
    searchPlaceholder: function () { return qlx.t("nav.search_placeholder"); },
    confirmLabel: function () { return qlx.t("action.move"); },
    onConfirm: function (targetId) {
      var ids = qlx.selectionEntries();
      fetch("/ui/actions/bulk/move", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ids: ids, target_container_id: targetId })
      })
        .then(function (resp) {
          if (!resp.ok) {
            return resp.json().then(function (data) {
              qlx.showToast(data.error || qlx.t("error.status") + " " + resp.status, true);
            });
          }
          qlx.showToast(qlx.t("bulk.moved") + " " + ids.length, false);
          qlx.clearSelection();
          htmx.ajax("GET", window.location.pathname, { target: "#content" });
        })
        .catch(function (err) {
          console.error("bulk move failed:", err);
          qlx.showToast(qlx.t("error.connection"), true);
        });
    }
  });

  qlx.openMovePicker = function () { picker.open(); };
})();
```

- [ ] **Step 4: Simplify `tag-picker.js`**

Replace entire file `internal/embedded/static/js/tags/tag-picker.js` with:

```js
(function () {
  var qlx = window.qlx = window.qlx || {};

  var picker = qlx.createTreePicker({
    id: "tag-picker",
    title: function () { return qlx.t("tags.add_tag"); },
    endpoint: "/ui/partials/tag-tree",
    searchEndpoint: "/ui/partials/tag-tree/search",
    searchPlaceholder: function () { return qlx.t("tags.search_tags"); },
    confirmLabel: function () { return qlx.t("tags.tag_action"); },
    onConfirm: function (tagId) {
      var ids = qlx.selectionEntries();
      fetch("/ui/actions/bulk/tags", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ids: ids, tag_id: tagId })
      })
        .then(function (resp) {
          if (!resp.ok) {
            return resp.json().then(function (data) {
              qlx.showToast(data.error || qlx.t("error.status") + " " + resp.status, true);
            });
          }
          qlx.showToast(qlx.t("bulk.tagged") + " " + ids.length, false);
          qlx.clearSelection();
          htmx.ajax("GET", window.location.pathname, { target: "#content" });
        })
        .catch(function (err) {
          console.error("bulk tag failed:", err);
          qlx.showToast(qlx.t("error.connection"), true);
        });
    }
  });

  qlx.openTagPicker = function () { picker.open(); };
})();
```

- [ ] **Step 5: Build and verify**

Run: `make build-mac`
Expected: binary builds successfully with embedded assets.

- [ ] **Step 6: Run E2E tests**

Run: `cd e2e && npx playwright test tests/batch-operations.spec.ts tests/tags.spec.ts`
Expected: pass — pickers work identically.

- [ ] **Step 7: Commit**

```bash
git add internal/embedded/static/js/shared/tree-picker.js internal/embedded/static/js/inventory/move-picker.js internal/embedded/static/js/tags/tag-picker.js internal/embedded/templates/layouts/base.html
git commit -m "refactor(js): extract shared tree picker factory, deduplicate move/tag pickers"
```

---

## Final Verification

- [ ] **Run full test suite**

Run: `make test && make lint && make build-mac`
Expected: all pass.

- [ ] **Run E2E**

Run: `cd e2e && npx playwright test`
Expected: all pass.
