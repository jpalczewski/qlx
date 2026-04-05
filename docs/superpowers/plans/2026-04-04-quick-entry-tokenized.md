# Tokenized Quick Entry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace two quick-entry forms on the container page with a single tokenized contenteditable input supporting `@container` and `#tag` triggers, Tab toggle for item/container mode, and `x5` quantity syntax; unify the container/item lists into one.

**Architecture:** Backend adds a flat container list endpoint and extends item/container creation to accept inline `tag_ids[]`. Frontend introduces a `contenteditable`-based tokenized input with container and tag autocomplete dropdowns. The container page template merges its two lists into a single unified list.

**Tech Stack:** Go 1.22+ (http.ServeMux), vanilla JS (no frameworks), HTMX, Go html/template

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/handler/containers.go` | Modify | Add `FlatList()` handler, extend `Create()` for `tag_ids[]` |
| `internal/handler/items.go` | Modify | Extend `Create()` for `tag_ids[]` |
| `internal/handler/request.go` | Modify | Add `TagIDs` to create request structs |
| `internal/service/inventory.go` | Modify | Add `CreateItemWithTags`, `CreateContainerWithTags` |
| `internal/handler/viewmodels.go` | Modify | Add `FlatContainer` struct |
| `internal/embedded/static/js/inventory/container-autocomplete.js` | Create | Container autocomplete dropdown |
| `internal/embedded/static/js/inventory/quick-entry-tokenized.js` | Create | Contenteditable token input, submit, parsing |
| `internal/embedded/static/css/inventory/quick-entry-tokenized.css` | Create | Token chips, toggle, contenteditable styles |
| `internal/embedded/templates/pages/inventory/containers.html` | Modify | Unified list + tokenized quick entry |
| `internal/embedded/templates/partials/inventory/item_list_item.html` | Modify | Adapt to unified list |
| `internal/embedded/templates/partials/inventory/container_list_item.html` | Modify | Adapt to unified list |
| `internal/embedded/static/js/inventory/quick-entry.js` | Delete | Replaced by tokenized version |
| `e2e/tests/quick-entry-tokenized.spec.ts` | Create | E2E tests |

---

### Task 1: Flat Container List Endpoint

**Files:**
- Modify: `internal/handler/viewmodels.go`
- Modify: `internal/handler/containers.go`

- [ ] **Step 1: Write the test for GET /api/containers/flat**

Create `internal/handler/containers_flat_test.go`:

```go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"qlx/internal/handler"
	"qlx/internal/service"
	"qlx/internal/store/memory"
)

func TestFlatList(t *testing.T) {
	s := memory.NewMemoryStore()
	inv := service.NewInventoryService(s, s)
	tagSvc := service.NewTagService(s)

	root, _ := inv.CreateContainer("", "Warsztat", "", "", "")
	shelf, _ := inv.CreateContainer(root.ID, "Półka 1", "", "", "")
	drawer, _ := inv.CreateContainer(shelf.ID, "Szuflada A", "", "", "archive-box")

	h := handler.NewContainerHandler(inv, nil, nil, nil, nil, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/containers/flat", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []handler.FlatContainer
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 containers, got %d", len(result))
	}

	// Find drawer and check path
	var found bool
	for _, fc := range result {
		if fc.ID == drawer.ID {
			found = true
			if fc.Path != "Warsztat / Półka 1" {
				t.Errorf("expected path 'Warsztat / Półka 1', got %q", fc.Path)
			}
			if fc.Icon != "archive-box" {
				t.Errorf("expected icon 'archive-box', got %q", fc.Icon)
			}
		}
	}
	if !found {
		t.Error("drawer not found in flat list")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -run TestFlatList -v`
Expected: FAIL — `FlatContainer` type and `FlatList` handler not defined.

- [ ] **Step 3: Add FlatContainer view model**

In `internal/handler/viewmodels.go`, add:

```go
// FlatContainer is a flattened container with its ancestor path for autocomplete.
type FlatContainer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
	Path string `json:"path"`
}
```

- [ ] **Step 4: Implement FlatList handler**

In `internal/handler/containers.go`, add the handler method:

```go
// FlatList returns all containers as a flat list with ancestor paths.
func (h *ContainerHandler) FlatList(w http.ResponseWriter, r *http.Request) {
	all := h.inventory.AllContainers()
	byID := make(map[string]*store.Container, len(all))
	for i := range all {
		byID[all[i].ID] = &all[i]
	}

	result := make([]FlatContainer, 0, len(all))
	for _, c := range all {
		path := buildContainerPath(byID, c.ID)
		result = append(result, FlatContainer{
			ID:   c.ID,
			Name: c.Name,
			Icon: c.Icon,
			Path: path,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func buildContainerPath(byID map[string]*store.Container, id string) string {
	c := byID[id]
	if c == nil || c.ParentID == "" {
		return ""
	}
	var parts []string
	cur := c.ParentID
	for cur != "" {
		p := byID[cur]
		if p == nil {
			break
		}
		parts = append([]string{p.Name}, parts...)
		cur = p.ParentID
	}
	return strings.Join(parts, " / ")
}
```

Add the `"encoding/json"` and `"strings"` imports if not present.

- [ ] **Step 5: Add AllContainers to InventoryService**

In `internal/service/inventory.go`, expose AllContainers:

```go
// AllContainers returns all containers.
func (s *InventoryService) AllContainers() []store.Container {
	return s.containers.AllContainers()
}
```

- [ ] **Step 6: Register the route**

In `internal/handler/containers.go` `RegisterRoutes`, add:

```go
mux.HandleFunc("GET /api/containers/flat", h.FlatList)
```

- [ ] **Step 7: Run test to verify it passes**

Run: `go test ./internal/handler/ -run TestFlatList -v`
Expected: PASS

- [ ] **Step 8: Run linter**

Run: `make lint`
Expected: No errors.

- [ ] **Step 9: Commit**

```bash
git add internal/handler/containers.go internal/handler/viewmodels.go internal/service/inventory.go internal/handler/containers_flat_test.go
git commit -m "feat(inventory): add GET /api/containers/flat endpoint"
```

---

### Task 2: Extend Item Creation with Inline Tags

**Files:**
- Modify: `internal/handler/request.go`
- Modify: `internal/handler/items.go`
- Modify: `internal/service/inventory.go`

- [ ] **Step 1: Write the test**

Create `internal/handler/items_tags_test.go`:

```go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"qlx/internal/handler"
	"qlx/internal/service"
	"qlx/internal/store/memory"
)

func TestCreateItemWithTags(t *testing.T) {
	s := memory.NewMemoryStore()
	inv := service.NewInventoryService(s, s)
	tagSvc := service.NewTagService(s)

	container, _ := inv.CreateContainer("", "Box", "", "", "")
	tag1 := tagSvc.CreateTag("", "metal", "", "")
	tag2 := tagSvc.CreateTag("", "round", "", "")

	h := handler.NewItemHandler(inv, tagSvc, nil, nil, nil, nil, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	form := url.Values{}
	form.Set("container_id", container.ID)
	form.Set("name", "Bolt")
	form.Set("quantity", "5")
	form.Add("tag_ids", tag1.ID)
	form.Add("tag_ids", tag2.ID)

	req := httptest.NewRequest("POST", "/items", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify tags were assigned
	items := inv.ContainerItems(container.ID)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(items[0].TagIDs) != 2 {
		t.Errorf("expected 2 tags, got %d", len(items[0].TagIDs))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -run TestCreateItemWithTags -v`
Expected: FAIL — `tag_ids` not handled.

- [ ] **Step 3: Add TagIDs to CreateItemRequest**

In `internal/handler/request.go`, extend:

```go
type CreateItemRequest struct {
	ContainerID string   `json:"container_id" form:"container_id"`
	Name        string   `json:"name" form:"name"`
	Description string   `json:"description" form:"description"`
	Quantity    int      `json:"quantity" form:"quantity"`
	Color       string   `json:"color" form:"color"`
	Icon        string   `json:"icon" form:"icon"`
	TagIDs      []string `json:"tag_ids" form:"tag_ids"`
}
```

Note: `BindRequest` uses reflection on `form` tags. Verify it handles `[]string` slices — if not, parse `tag_ids` manually from `r.Form["tag_ids"]` in the handler.

- [ ] **Step 4: Extend items.go Create handler**

In `internal/handler/items.go` `Create()`, after successful item creation, add tag assignment:

```go
// After: item, err := h.inventory.CreateItem(...)
// Assign tags if provided
for _, tagID := range req.TagIDs {
	if tagID != "" {
		if err := h.tags.AddItemTag(item.ID, tagID); err != nil {
			webutil.LogError("failed to add tag %s to item %s: %v", tagID, item.ID, err)
		}
	}
}

// Re-fetch item to include tag IDs in response
item = h.inventory.GetItem(item.ID)
```

Ensure `h.tags` (TagService) is available on ItemHandler. If not, add it as a dependency.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/handler/ -run TestCreateItemWithTags -v`
Expected: PASS

- [ ] **Step 6: Run linter**

Run: `make lint`
Expected: No errors.

- [ ] **Step 7: Commit**

```bash
git add internal/handler/request.go internal/handler/items.go internal/handler/items_tags_test.go
git commit -m "feat(items): accept tag_ids[] on item creation"
```

---

### Task 3: Extend Container Creation with Inline Tags

**Files:**
- Modify: `internal/handler/request.go`
- Modify: `internal/handler/containers.go`

- [ ] **Step 1: Write the test**

Create `internal/handler/containers_tags_test.go`:

```go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"qlx/internal/handler"
	"qlx/internal/service"
	"qlx/internal/store/memory"
)

func TestCreateContainerWithTags(t *testing.T) {
	s := memory.NewMemoryStore()
	inv := service.NewInventoryService(s, s)
	tagSvc := service.NewTagService(s)

	parent, _ := inv.CreateContainer("", "Root", "", "", "")
	tag := tagSvc.CreateTag("", "fragile", "", "")

	h := handler.NewContainerHandler(inv, tagSvc, nil, nil, nil, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	form := url.Values{}
	form.Set("parent_id", parent.ID)
	form.Set("name", "Shelf A")
	form.Add("tag_ids", tag.ID)

	req := httptest.NewRequest("POST", "/containers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	children := inv.ContainerChildren(parent.ID)
	if len(children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(children))
	}
	if len(children[0].TagIDs) != 1 {
		t.Errorf("expected 1 tag, got %d", len(children[0].TagIDs))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -run TestCreateContainerWithTags -v`
Expected: FAIL

- [ ] **Step 3: Add TagIDs to CreateContainerRequest**

In `internal/handler/request.go`:

```go
type CreateContainerRequest struct {
	ParentID    string   `json:"parent_id" form:"parent_id"`
	Name        string   `json:"name" form:"name"`
	Description string   `json:"description" form:"description"`
	Color       string   `json:"color" form:"color"`
	Icon        string   `json:"icon" form:"icon"`
	TagIDs      []string `json:"tag_ids" form:"tag_ids"`
}
```

- [ ] **Step 4: Extend containers.go Create handler**

In `internal/handler/containers.go` `Create()`, after successful container creation:

```go
// After: container, err := h.inventory.CreateContainer(...)
for _, tagID := range req.TagIDs {
	if tagID != "" {
		if err := h.tags.AddContainerTag(container.ID, tagID); err != nil {
			webutil.LogError("failed to add tag %s to container %s: %v", tagID, container.ID, err)
		}
	}
}
container = h.inventory.GetContainer(container.ID)
```

Ensure `h.tags` is available on ContainerHandler.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/handler/ -run TestCreateContainerWithTags -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/handler/request.go internal/handler/containers.go internal/handler/containers_tags_test.go
git commit -m "feat(containers): accept tag_ids[] on container creation"
```

---

### Task 4: Container Autocomplete JS

**Files:**
- Create: `internal/embedded/static/js/inventory/container-autocomplete.js`

- [ ] **Step 1: Create container-autocomplete.js**

Model after `tag-autocomplete.js`. Key differences: fetches from `/api/containers/flat`, shows path as subtitle, no "Create" option, max 1 selection.

```javascript
(function () {
  "use strict";

  var cache = null;

  function fetchContainers(cb) {
    if (cache) { cb(cache); return; }
    fetch("/api/containers/flat")
      .then(function (r) { return r.json(); })
      .then(function (list) { cache = list; cb(list); })
      .catch(function () { cb([]); });
  }

  window.qlx = window.qlx || {};
  window.qlx.invalidateContainerCache = function () { cache = null; };

  window.qlx.ContainerAutocomplete = function (opts) {
    var anchor = opts.anchor;
    var onSelect = opts.onSelect;
    var onCancel = opts.onCancel || function () {};
    var dropdown = null;
    var inputEl = null;
    var activeIdx = -1;
    var filtered = [];

    function filterContainers(all, query) {
      if (!query) return all.slice(0, 8);
      var q = query.toLowerCase();
      return all.filter(function (c) {
        return c.name.toLowerCase().indexOf(q) !== -1 ||
               c.path.toLowerCase().indexOf(q) !== -1;
      }).slice(0, 8);
    }

    function buildOption(c, idx) {
      var opt = document.createElement("div");
      opt.className = "container-ac-option";
      opt.setAttribute("role", "option");
      opt.setAttribute("data-idx", idx);

      var icon = document.createElement("span");
      icon.className = "container-ac-icon";
      icon.textContent = c.icon ? "" : "📦";
      if (c.icon) {
        var i = document.createElement("i");
        i.className = "ph ph-" + c.icon;
        icon.textContent = "";
        icon.appendChild(i);
      }
      opt.appendChild(icon);

      var name = document.createElement("span");
      name.className = "container-ac-name";
      name.textContent = c.name;
      opt.appendChild(name);

      if (c.path) {
        var path = document.createElement("span");
        path.className = "container-ac-path";
        path.textContent = c.path;
        opt.appendChild(path);
      }

      opt.addEventListener("mousedown", function (e) {
        e.preventDefault();
        selectItem(idx);
      });

      return opt;
    }

    function renderDropdown(items) {
      close();
      if (items.length === 0) return;

      filtered = items;
      activeIdx = 0;

      dropdown = document.createElement("div");
      dropdown.className = "container-ac-dropdown";
      dropdown.setAttribute("role", "listbox");

      items.forEach(function (c, i) {
        var opt = buildOption(c, i);
        if (i === 0) opt.classList.add("active");
        dropdown.appendChild(opt);
      });

      anchor.appendChild(dropdown);
      positionDropdown();
    }

    function positionDropdown() {
      if (!dropdown) return;
      var rect = anchor.getBoundingClientRect();
      var spaceBelow = window.innerHeight - rect.bottom;
      if (spaceBelow < 200) {
        dropdown.classList.add("container-ac-dropdown--above");
      }
    }

    function setActive(idx) {
      if (!dropdown) return;
      var opts = dropdown.querySelectorAll(".container-ac-option");
      opts.forEach(function (o) { o.classList.remove("active"); });
      activeIdx = Math.max(0, Math.min(idx, opts.length - 1));
      if (opts[activeIdx]) {
        opts[activeIdx].classList.add("active");
        opts[activeIdx].scrollIntoView({ block: "nearest" });
      }
    }

    function selectItem(idx) {
      var c = filtered[idx];
      if (c) {
        close();
        onSelect(c);
      }
    }

    function close() {
      if (dropdown) {
        dropdown.remove();
        dropdown = null;
      }
      filtered = [];
      activeIdx = -1;
    }

    function update(query) {
      fetchContainers(function (all) {
        var items = filterContainers(all, query);
        renderDropdown(items);
      });
    }

    function onKeydown(e) {
      if (!dropdown) return false;
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setActive(activeIdx + 1);
        return true;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setActive(activeIdx - 1);
        return true;
      }
      if (e.key === "Enter") {
        e.preventDefault();
        selectItem(activeIdx);
        return true;
      }
      if (e.key === "Escape") {
        e.preventDefault();
        close();
        onCancel();
        return true;
      }
      return false;
    }

    return {
      update: update,
      onKeydown: onKeydown,
      close: close,
      isOpen: function () { return dropdown !== null; }
    };
  };
})();
```

- [ ] **Step 2: Verify file loads without errors**

Open browser dev console, check no JS errors. Verify `qlx.ContainerAutocomplete` is defined.

- [ ] **Step 3: Commit**

```bash
git add internal/embedded/static/js/inventory/container-autocomplete.js
git commit -m "feat(inventory): add container autocomplete JS component"
```

---

### Task 5: Tokenized Quick Entry CSS

**Files:**
- Create: `internal/embedded/static/css/inventory/quick-entry-tokenized.css`

- [ ] **Step 1: Create the CSS file**

```css
/* --- Quick Entry Tokenized --- */

.qe-tokenized {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.qe-type-toggle {
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 0.25rem 0.4rem;
  cursor: pointer;
  font-size: 1.1rem;
  line-height: 1;
  user-select: none;
  flex-shrink: 0;
}

.qe-type-toggle:hover {
  border-color: var(--accent);
}

.qe-input {
  flex: 1;
  min-height: 2rem;
  padding: 0.35rem 0.5rem;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text);
  font-family: inherit;
  font-size: 0.875rem;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.35rem;
  cursor: text;
  position: relative;
}

.qe-input:focus-within {
  border-color: var(--accent);
  outline: none;
}

.qe-input:empty::before {
  content: attr(data-placeholder);
  color: var(--text-muted);
  font-style: italic;
  pointer-events: none;
}

.qe-submit {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 1.1rem;
  padding: 0.25rem;
  flex-shrink: 0;
}

.qe-submit:hover {
  color: var(--accent);
}

/* Tokens */

.qe-token {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.1rem 0.4rem;
  border-radius: 3px;
  font-size: 0.75rem;
  line-height: 1.4;
  white-space: nowrap;
  user-select: none;
}

.qe-token[contenteditable="false"] {
  cursor: default;
}

.qe-token--container {
  background: var(--accent-surface, #1e3a5f);
  color: var(--accent, #4a9eff);
}

.qe-token--tag {
  background: var(--success-surface, #1a2e1a);
  color: var(--success, #5ab85a);
}

.qe-token--default {
  opacity: 0.6;
  border: 1px dashed var(--border);
}

.qe-token-remove {
  cursor: pointer;
  opacity: 0.5;
  margin-left: 0.15rem;
}

.qe-token-remove:hover {
  opacity: 1;
}

.qe-token-icon {
  font-size: 0.7rem;
}

/* Dropdown shared positioning */

.qe-input .container-ac-dropdown,
.qe-input .tag-ac-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  z-index: 100;
  margin-top: 2px;
}

/* Container AC styling */

.container-ac-dropdown {
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  max-height: 16rem;
  overflow-y: auto;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}

.container-ac-dropdown--above {
  top: auto;
  bottom: 100%;
  margin-top: 0;
  margin-bottom: 2px;
}

.container-ac-option {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.35rem 0.6rem;
  cursor: pointer;
  font-size: 0.8rem;
}

.container-ac-option:hover,
.container-ac-option.active {
  background: var(--surface-3, #2a3a4a);
}

.container-ac-icon {
  flex-shrink: 0;
  width: 1.2rem;
  text-align: center;
}

.container-ac-name {
  flex: 1;
}

.container-ac-path {
  color: var(--text-muted);
  font-size: 0.7rem;
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/embedded/static/css/inventory/quick-entry-tokenized.css
git commit -m "feat(inventory): add tokenized quick entry CSS"
```

---

### Task 6: Tokenized Quick Entry JS

**Files:**
- Create: `internal/embedded/static/js/inventory/quick-entry-tokenized.js`

This is the core component. It manages the `contenteditable` div, token lifecycle, triggers, and submission.

- [ ] **Step 1: Create quick-entry-tokenized.js**

```javascript
(function () {
  "use strict";
  window.qlx = window.qlx || {};

  /**
   * TokenizedQuickEntry manages a contenteditable input with @container and #tag tokens.
   *
   * @param {Object} opts
   * @param {HTMLElement} opts.el - the .qe-tokenized root element
   * @param {string} opts.prefillContainerID - pre-filled container ID (from page context)
   * @param {string} opts.prefillContainerName - pre-filled container name
   * @param {string} opts.prefillContainerIcon - pre-filled container icon
   * @param {string} opts.defaultContainerID - default container ID (inbox from settings)
   * @param {string} opts.defaultContainerName - default container name
   * @param {string} opts.onCreated - callback after successful creation
   */
  window.qlx.TokenizedQuickEntry = function (opts) {
    var el = opts.el;
    var input = el.querySelector(".qe-input");
    var toggle = el.querySelector(".qe-type-toggle");
    var mode = "item"; // "item" or "container"
    var containerAC = null;
    var tagAC = null;
    var triggerState = null; // {type: "@"|"#", startOffset, textNode}

    var prefillContainer = {
      id: opts.prefillContainerID || opts.defaultContainerID || "",
      name: opts.prefillContainerName || opts.defaultContainerName || "inbox",
      icon: opts.prefillContainerIcon || ""
    };

    // --- Token creation ---

    function createTokenSpan(type, id, name, icon, isDefault) {
      var span = document.createElement("span");
      span.contentEditable = "false";
      span.className = "qe-token qe-token--" + type;
      if (isDefault) span.classList.add("qe-token--default");
      span.setAttribute("data-token-type", type);
      span.setAttribute("data-token-id", id);

      if (icon) {
        var iconEl = document.createElement("i");
        iconEl.className = "ph ph-" + icon + " qe-token-icon";
        span.appendChild(iconEl);
      }

      var label = document.createElement("span");
      label.textContent = (type === "container" ? "@" : "#") + name;
      span.appendChild(label);

      var remove = document.createElement("span");
      remove.className = "qe-token-remove";
      remove.textContent = "\u00d7";
      remove.addEventListener("mousedown", function (e) {
        e.preventDefault();
        e.stopPropagation();
        removeToken(span);
      });
      span.appendChild(remove);

      return span;
    }

    function insertToken(type, id, name, icon) {
      // Remove trigger text (@query or #query) from input
      removeTriggerText();

      if (type === "container") {
        // Remove existing container token
        var existing = input.querySelector('[data-token-type="container"]');
        if (existing) existing.remove();
      }

      var token = createTokenSpan(type, id, name, icon, false);

      // Insert at cursor or at start for container
      if (type === "container") {
        input.insertBefore(token, input.firstChild);
      } else {
        var sel = window.getSelection();
        if (sel.rangeCount > 0 && input.contains(sel.anchorNode)) {
          var range = sel.getRangeAt(0);
          range.insertNode(token);
          range.setStartAfter(token);
          range.collapse(true);
          sel.removeAllRanges();
          sel.addRange(range);
        } else {
          input.appendChild(token);
        }
      }

      // Add space after token
      var space = document.createTextNode("\u00A0");
      token.after(space);

      input.focus();
      placeCaretAfter(space);
    }

    function removeToken(span) {
      var type = span.getAttribute("data-token-type");
      span.remove();

      // If container was removed, restore default
      if (type === "container" && prefillContainer.id) {
        var def = createTokenSpan("container", prefillContainer.id, prefillContainer.name, prefillContainer.icon, true);
        input.insertBefore(def, input.firstChild);
        var space = document.createTextNode("\u00A0");
        def.after(space);
      }
      input.focus();
    }

    // --- Trigger detection ---

    function detectTrigger() {
      var sel = window.getSelection();
      if (!sel.rangeCount || !input.contains(sel.anchorNode)) return null;

      var node = sel.anchorNode;
      if (node.nodeType !== Node.TEXT_NODE) return null;

      var text = node.textContent;
      var offset = sel.anchorOffset;
      var before = text.substring(0, offset);

      // Find last @ or # that isn't inside a token
      var atIdx = before.lastIndexOf("@");
      var hashIdx = before.lastIndexOf("#");

      var triggerIdx = -1;
      var triggerChar = null;

      if (atIdx > hashIdx) {
        triggerIdx = atIdx;
        triggerChar = "@";
      } else if (hashIdx > atIdx) {
        triggerIdx = hashIdx;
        triggerChar = "#";
      }

      if (triggerIdx === -1) return null;

      // Must be at start of word (index 0 or preceded by space)
      if (triggerIdx > 0 && before[triggerIdx - 1] !== " " && before[triggerIdx - 1] !== "\u00A0") {
        return null;
      }

      var query = before.substring(triggerIdx + 1);
      // No spaces in query (trigger ends at space)
      if (query.indexOf(" ") !== -1 || query.indexOf("\u00A0") !== -1) return null;

      return {
        type: triggerChar === "@" ? "container" : "tag",
        char: triggerChar,
        query: query,
        textNode: node,
        startOffset: triggerIdx
      };
    }

    function removeTriggerText() {
      if (!triggerState || !triggerState.textNode.parentNode) return;
      var text = triggerState.textNode.textContent;
      var sel = window.getSelection();
      var endOffset = sel.rangeCount > 0 ? sel.anchorOffset : text.length;
      triggerState.textNode.textContent =
        text.substring(0, triggerState.startOffset) + text.substring(endOffset);
      triggerState = null;
    }

    // --- Parsing ---

    function parseInput() {
      var containerID = "";
      var tagIDs = [];
      var textParts = [];

      var nodes = Array.from(input.childNodes);
      nodes.forEach(function (node) {
        if (node.nodeType === Node.ELEMENT_NODE && node.classList.contains("qe-token")) {
          var type = node.getAttribute("data-token-type");
          var id = node.getAttribute("data-token-id");
          if (type === "container") containerID = id;
          if (type === "tag") tagIDs.push(id);
        } else if (node.nodeType === Node.TEXT_NODE) {
          var t = node.textContent.replace(/\u00A0/g, " ").trim();
          if (t) textParts.push(t);
        }
      });

      var fullText = textParts.join(" ").trim();

      // Parse x<number> for quantity
      var quantity = 1;
      if (mode === "item") {
        var match = fullText.match(/\bx(\d+)\b/);
        if (match && parseInt(match[1], 10) > 0) {
          quantity = parseInt(match[1], 10);
          fullText = fullText.replace(match[0], "").replace(/\s+/g, " ").trim();
        }
      }

      return {
        name: fullText,
        containerID: containerID || prefillContainer.id,
        tagIDs: tagIDs,
        quantity: quantity,
        mode: mode
      };
    }

    // --- Submission ---

    function submit() {
      // Close any open dropdown
      if (containerAC && containerAC.isOpen()) containerAC.close();
      if (tagAC) tagAC.close();

      var data = parseInput();
      if (!data.name) return;

      var form = new FormData();
      form.append("name", data.name);
      data.tagIDs.forEach(function (id) { form.append("tag_ids", id); });

      var url, hxTarget;
      if (data.mode === "item") {
        url = "/items";
        form.append("container_id", data.containerID);
        form.append("quantity", String(data.quantity));
        hxTarget = "item-list";
      } else {
        url = "/containers";
        form.append("parent_id", data.containerID);
        hxTarget = "container-list";
      }

      fetch(url, {
        method: "POST",
        headers: { "HX-Request": "true", "HX-Target": hxTarget },
        body: form
      })
        .then(function (resp) {
          if (!resp.ok) throw new Error("HTTP " + resp.status);
          return resp.text();
        })
        .then(function (html) {
          var list = document.getElementById(hxTarget);
          if (list) {
            list.insertAdjacentHTML("beforeend", html);
            // Trigger flash animation on new item
            var last = list.lastElementChild;
            if (last) last.classList.add("flash");
          }
          resetInput();
          qlx.invalidateContainerCache();
        })
        .catch(function (err) {
          console.error("Quick entry submit failed:", err);
        });
    }

    function resetInput() {
      // Clear everything
      input.textContent = "";
      // Restore prefilled container
      if (prefillContainer.id) {
        var token = createTokenSpan(
          "container", prefillContainer.id, prefillContainer.name,
          prefillContainer.icon, mode === "container"
        );
        input.appendChild(token);
        input.appendChild(document.createTextNode("\u00A0"));
      }
      input.focus();
      placeCaretAtEnd();
    }

    // --- Mode toggle ---

    function toggleMode() {
      mode = mode === "item" ? "container" : "item";
      toggle.textContent = mode === "item" ? "🏷" : "📦";
      input.setAttribute("data-placeholder",
        mode === "item" ? "Nowy item... (x5 #tag ↵)" : "Nowy kontener... (#tag ↵)");
    }

    // --- Caret helpers ---

    function placeCaretAtEnd() {
      var range = document.createRange();
      range.selectNodeContents(input);
      range.collapse(false);
      var sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
    }

    function placeCaretAfter(node) {
      var range = document.createRange();
      range.setStartAfter(node);
      range.collapse(true);
      var sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
    }

    // --- Event handlers ---

    input.addEventListener("input", function () {
      triggerState = detectTrigger();

      if (triggerState && triggerState.type === "container") {
        if (!containerAC) {
          containerAC = new qlx.ContainerAutocomplete({
            anchor: input,
            onSelect: function (c) {
              insertToken("container", c.id, c.name, c.icon);
              containerAC = null;
            },
            onCancel: function () {
              containerAC = null;
            }
          });
        }
        containerAC.update(triggerState.query);
      } else if (containerAC) {
        containerAC.close();
        containerAC = null;
      }

      if (triggerState && triggerState.type === "tag") {
        if (!tagAC) {
          tagAC = new qlx.TagAutocomplete({
            anchor: input,
            onSelect: function (tag) {
              var parentName = tag.parent_name ? tag.parent_name + " / " : "";
              insertToken("tag", tag.id, parentName + tag.name, "");
              tagAC = null;
            },
            onCancel: function () {
              tagAC = null;
            }
          });
        }
        tagAC.update(triggerState.query);
      } else if (tagAC) {
        tagAC.close();
        tagAC = null;
      }
    });

    input.addEventListener("keydown", function (e) {
      // Let active AC handle first
      if (containerAC && containerAC.onKeydown(e)) return;
      if (tagAC && tagAC.onKeydown && tagAC.onKeydown(e)) return;

      if (e.key === "Tab") {
        e.preventDefault();
        toggleMode();
        return;
      }

      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        submit();
        return;
      }

      if (e.key === "Backspace") {
        var sel = window.getSelection();
        if (sel.rangeCount > 0 && sel.isCollapsed) {
          var range = sel.getRangeAt(0);
          var node = range.startContainer;
          var offset = range.startOffset;

          // Check if previous sibling is a token
          var prev = null;
          if (node === input && offset > 0) {
            prev = input.childNodes[offset - 1];
          } else if (node.nodeType === Node.TEXT_NODE && offset === 0) {
            prev = node.previousSibling;
          }

          // Skip whitespace text nodes
          if (prev && prev.nodeType === Node.TEXT_NODE &&
              prev.textContent.replace(/\u00A0/g, "").trim() === "") {
            prev = prev.previousSibling;
          }

          if (prev && prev.classList && prev.classList.contains("qe-token")) {
            e.preventDefault();
            removeToken(prev);
            return;
          }
        }
      }
    });

    // Clicking the input area focuses contenteditable
    el.addEventListener("click", function (e) {
      if (e.target === el || e.target.classList.contains("qe-submit")) return;
      input.focus();
    });

    // Submit button
    var submitBtn = el.querySelector(".qe-submit");
    if (submitBtn) {
      submitBtn.addEventListener("click", function (e) {
        e.preventDefault();
        submit();
      });
    }

    // Toggle button
    toggle.addEventListener("click", function (e) {
      e.preventDefault();
      toggleMode();
    });

    // --- Init ---
    resetInput();
    toggle.textContent = "🏷";
    input.setAttribute("data-placeholder", "Nowy item... (x5 #tag ↵)");
  };
})();
```

- [ ] **Step 2: Verify in browser**

Load a container page, check console for errors. Verify the contenteditable renders with prefilled container token.

- [ ] **Step 3: Commit**

```bash
git add internal/embedded/static/js/inventory/quick-entry-tokenized.js
git commit -m "feat(inventory): add tokenized quick entry JS component"
```

---

### Task 7: Template Changes — Unified Container Page

**Files:**
- Modify: `internal/embedded/templates/pages/inventory/containers.html`
- Modify: `internal/embedded/templates/partials/inventory/container_list_item.html`
- Modify: `internal/embedded/templates/partials/inventory/item_list_item.html`

- [ ] **Step 1: Update containers.html — replace quick-entry forms with tokenized input**

Replace the two quick-entry sections (container form ~lines 64-81 and item form ~lines 101-120) with a single tokenized quick entry:

```html
{{/* Tokenized Quick Entry */}}
<div class="qe-tokenized"
     data-prefill-container-id="{{ .Data.Container.ID }}"
     data-prefill-container-name="{{ .Data.Container.Name }}"
     data-prefill-container-icon="{{ .Data.Container.Icon }}"
     data-default-container-id="{{ .Data.DefaultContainerID }}"
     data-default-container-name="{{ .Data.DefaultContainerName }}">
  <button type="button" class="qe-type-toggle" title="Tab: item/kontener">🏷</button>
  <div class="qe-input" contenteditable="true" role="textbox"
       data-placeholder="Nowy item... (x5 #tag ↵)"></div>
  <button type="button" class="qe-submit" title="Submit (Enter)">↵</button>
</div>
```

- [ ] **Step 2: Update containers.html — unified list**

Replace the two separate `<ul>` lists (container-list and item-list) with a single unified list:

```html
{{/* Unified list: containers on top, items below */}}
<ul id="container-list" class="inventory-list">
  {{ range .Data.Children }}
    {{ template "container-list-item" pageData $.Lang . }}
  {{ end }}
</ul>

{{ if and .Data.Children .Data.Items }}
<hr class="inventory-separator">
{{ end }}

<ul id="item-list" class="inventory-list">
  {{ range .Data.Items }}
    {{ template "item-list-item" pageData $.Lang . }}
  {{ end }}
</ul>
```

- [ ] **Step 3: Add CSS/JS includes in the template**

Ensure the template includes the new CSS and JS files. Add to the page head/footer (or to the embedded assets list):

```html
<link rel="stylesheet" href="/static/css/inventory/quick-entry-tokenized.css">
<script src="/static/js/inventory/container-autocomplete.js"></script>
<script src="/static/js/inventory/quick-entry-tokenized.js"></script>
```

- [ ] **Step 4: Add initialization script**

At the bottom of the template (or in a page-specific script block):

```html
<script>
document.addEventListener("DOMContentLoaded", function() {
  var el = document.querySelector(".qe-tokenized");
  if (!el) return;
  new qlx.TokenizedQuickEntry({
    el: el,
    prefillContainerID: el.dataset.prefillContainerId,
    prefillContainerName: el.dataset.prefillContainerName,
    prefillContainerIcon: el.dataset.prefillContainerIcon,
    defaultContainerID: el.dataset.defaultContainerId,
    defaultContainerName: el.dataset.defaultContainerName
  });
});
</script>
```

- [ ] **Step 5: Add DefaultContainerID/Name to ContainerListData**

In `internal/handler/viewmodels.go`, extend:

```go
type ContainerListData struct {
	// ... existing fields ...
	DefaultContainerID   string
	DefaultContainerName string
}
```

In `internal/handler/containers.go` `containerListVM`, populate from settings:

```go
vm.DefaultContainerID = h.settings.DefaultContainerID()
vm.DefaultContainerName = h.settings.DefaultContainerName()
```

(Adjust based on how settings are accessed in the codebase.)

- [ ] **Step 6: Remove old quick-entry.js include**

Remove the `<script src="/static/js/inventory/quick-entry.js">` include from the template. Delete `internal/embedded/static/js/inventory/quick-entry.js`.

- [ ] **Step 7: Verify in browser**

Build and run: `make build-mac && make run`. Navigate to a container page. Verify:
- Tokenized input renders with pre-filled @container
- Tab toggles 🏷/📦
- @ triggers container dropdown
- \# triggers tag dropdown
- Enter submits and appends to correct list
- Unified list shows containers on top, items below

- [ ] **Step 8: Commit**

```bash
git add internal/embedded/templates/pages/inventory/containers.html \
        internal/embedded/templates/partials/inventory/container_list_item.html \
        internal/embedded/templates/partials/inventory/item_list_item.html \
        internal/handler/viewmodels.go internal/handler/containers.go
git rm internal/embedded/static/js/inventory/quick-entry.js
git commit -m "feat(inventory): unified list and tokenized quick entry template"
```

---

### Task 8: Adapt TagAutocomplete for External Trigger

**Files:**
- Modify: `internal/embedded/static/js/tags/tag-autocomplete.js`

The existing TagAutocomplete binds to an `<input>` element and manages its own event listeners. The tokenized quick entry needs to drive it externally (calling `update(query)` and `onKeydown(e)` from the contenteditable). Check if the existing API supports this — if the `open()` call requires an input element, add an alternative mode.

- [ ] **Step 1: Add external mode to TagAutocomplete**

Add an `update(query)` and `onKeydown(e)` return value to the TagAutocomplete constructor, similar to the ContainerAutocomplete API:

```javascript
// At the end of the constructor, add return object:
return {
  update: function(query) { /* call internal filter + render */ },
  onKeydown: function(e) { /* handle arrow/enter/escape, return true if consumed */ },
  close: close,
  isOpen: function() { return !!dropdown; }
};
```

Ensure backward compatibility — existing callers using `open(inputEl)` should still work.

- [ ] **Step 2: Verify existing tag-inline.js still works**

Navigate to an item/container detail page, click "+", verify tag autocomplete works as before.

- [ ] **Step 3: Commit**

```bash
git add internal/embedded/static/js/tags/tag-autocomplete.js
git commit -m "refactor(tags): expose update/onKeydown API on TagAutocomplete"
```

---

### Task 9: E2E Tests

**Files:**
- Create: `e2e/tests/quick-entry-tokenized.spec.ts`

- [ ] **Step 1: Write E2E tests**

```typescript
import { test, expect } from "../fixtures/app";

test.describe("Tokenized Quick Entry", () => {
  test("creates item with prefilled container", async ({ page, app }) => {
    // Create a container first
    await page.goto(`${app.baseURL}/containers`);
    // ... create a container via existing UI ...

    // Navigate to the container
    // ... click into it ...

    const input = page.locator(".qe-input");
    await expect(input).toBeVisible();

    // Verify prefilled container token
    const containerToken = input.locator('[data-token-type="container"]');
    await expect(containerToken).toBeVisible();

    // Type item name
    await input.click();
    await page.keyboard.type("Bolt M3 x5");
    await page.keyboard.press("Enter");

    // Verify item appears in list
    await expect(page.locator("#item-list")).toContainText("Bolt M3");
  });

  test("toggle between item and container mode", async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers`);
    // Navigate to a container ...

    const toggle = page.locator(".qe-type-toggle");
    await expect(toggle).toHaveText("🏷");

    await page.keyboard.press("Tab");
    await expect(toggle).toHaveText("📦");

    await page.keyboard.press("Tab");
    await expect(toggle).toHaveText("🏷");
  });

  test("@ trigger shows container dropdown", async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers`);
    // Navigate to a container ...

    const input = page.locator(".qe-input");
    await input.click();

    // Remove prefilled container token
    await page.keyboard.press("Backspace");

    await page.keyboard.type("@");
    const dropdown = page.locator(".container-ac-dropdown");
    await expect(dropdown).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(dropdown).not.toBeVisible();
  });

  test("# trigger shows tag dropdown", async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers`);
    // Create a tag first via API or UI ...

    const input = page.locator(".qe-input");
    await input.click();
    await page.keyboard.type("Test item #");

    const dropdown = page.locator(".tag-ac-dropdown");
    await expect(dropdown).toBeVisible();
  });

  test("backspace removes token", async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers`);
    // Navigate to a container, add a tag token ...

    const input = page.locator(".qe-input");
    await input.click();
    // Type # and select a tag
    // Then position cursor after tag and backspace

    // Verify token is removed
  });
});
```

Note: These are skeleton tests. The implementing agent must flesh them out with proper container/tag creation setup and exact selectors based on the final template.

- [ ] **Step 2: Run E2E tests**

Run: `make test-e2e`
Expected: All tests pass.

- [ ] **Step 3: Commit**

```bash
git add e2e/tests/quick-entry-tokenized.spec.ts
git commit -m "test(e2e): add tokenized quick entry tests"
```

---

### Task 10: Cleanup

- [ ] **Step 1: Remove old quick-entry CSS if unused**

Check if `quick-entry.css` is referenced anywhere else. If not:

```bash
git rm internal/embedded/static/css/inventory/quick-entry.css
```

If the description toggle CSS is still needed (for other forms), keep it.

- [ ] **Step 2: Run full test suite**

```bash
make test
make lint
make test-e2e
```

Expected: All pass.

- [ ] **Step 3: Final commit**

```bash
git add -A
git commit -m "chore(inventory): remove old quick-entry files"
```
