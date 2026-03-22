# Tag UI Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the tag UI — autocomplete component for adding tags, tag chip navigation, container tag chips, and tag detail page.

**Architecture:** Shared vanilla JS `TagAutocomplete` component with three mounting modes (hash/field/inline). New `ContainersByTag` store method + `HandleTagView` UI handler for tag detail page. HTMX-driven partial updates for all tag operations.

**Tech Stack:** Go 1.22+ (store, handlers), vanilla JS (autocomplete), HTMX (partial updates), HTML templates, CSS

**Spec:** `docs/superpowers/specs/2026-03-22-tag-ui-improvements-design.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|----------------|
| `internal/embedded/static/js/tags/tag-autocomplete.js` | Shared autocomplete component (fetch, filter, dropdown, create, ARIA) |
| `internal/embedded/static/css/tags/tag-autocomplete.css` | Autocomplete dropdown + input styles |
| `internal/embedded/templates/pages/tags/tag_detail.html` | Tag detail page template (stats, items, containers, children) |
| `e2e/tests/tag-ui.spec.ts` | E2E tests for tag chips, autocomplete, tag detail page |

### Modified Files

| File | Change |
|------|--------|
| `internal/store/tags.go` | Add `ContainersByTag(tagID string) []Container` |
| `internal/store/tags_test.go` | Add `TestContainersByTag` |
| `internal/ui/handlers_tags.go` | Add `HandleTagView`, `TagDetailData`, `TagStats` |
| `internal/ui/server.go` | Register `GET /ui/tags/{id}` route |
| `internal/embedded/templates/partials/tags/tag_chips.html` | Tag name as `<a>` link, keep `x` button separate |
| `internal/embedded/templates/pages/inventory/containers.html` | Add tag chips to container list items |
| `internal/embedded/templates/pages/inventory/item.html` | Add tag chips section to item detail |
| `internal/embedded/templates/pages/inventory/item_form.html` | Add tag field with autocomplete |
| `internal/embedded/templates/pages/inventory/container_form.html` | Add tag field with autocomplete |
| `internal/embedded/templates/layouts/base.html` | Include `tag-autocomplete.js` and `tag-autocomplete.css` |
| `internal/embedded/static/i18n/en/tags.json` | New translation keys for autocomplete + detail page |
| `internal/embedded/static/i18n/pl/tags.json` | Same keys in Polish |

---

### Task 1: `ContainersByTag` Store Method + Tests

**Files:**
- Modify: `internal/store/tags.go` (after `ItemsByTag` method, ~line 300)
- Modify: `internal/store/tags_test.go` (after `TestItemsByTag`, ~line 238)

- [ ] **Step 1: Write the failing test**

Add to `internal/store/tags_test.go`:

```go
func TestContainersByTag(t *testing.T) {
	s := NewMemoryStore()

	// Create tag hierarchy: warehouse > shelf
	warehouse, _ := s.CreateTag("", "Warehouse", "blue", "")
	shelf, _ := s.CreateTag(warehouse.ID, "Shelf", "green", "")

	// Create containers
	root, _ := s.CreateContainer("", "Root", "", "", "")
	boxA, _ := s.CreateContainer(root.ID, "Box A", "", "", "")
	boxB, _ := s.CreateContainer(root.ID, "Box B", "", "", "")

	// Tag: Box A -> warehouse, Box B -> shelf (child of warehouse)
	s.AddContainerTag(boxA.ID, warehouse.ID)
	s.AddContainerTag(boxB.ID, shelf.ID)

	// ContainersByTag(warehouse) should return both (includes descendant shelf)
	containers := s.ContainersByTag(warehouse.ID)
	if len(containers) != 2 {
		t.Fatalf("ContainersByTag(warehouse) count = %d, want 2", len(containers))
	}
	names := map[string]bool{containers[0].Name: true, containers[1].Name: true}
	if !names["Box A"] || !names["Box B"] {
		t.Errorf("ContainersByTag names = %v, want Box A and Box B", names)
	}

	// ContainersByTag(shelf) should return only Box B
	shelfContainers := s.ContainersByTag(shelf.ID)
	if len(shelfContainers) != 1 {
		t.Fatalf("ContainersByTag(shelf) count = %d, want 1", len(shelfContainers))
	}
	if shelfContainers[0].Name != "Box B" {
		t.Errorf("ContainersByTag(shelf) container = %q, want Box B", shelfContainers[0].Name)
	}

	// Nonexistent tag returns empty
	none := s.ContainersByTag("nonexistent")
	if len(none) != 0 {
		t.Errorf("ContainersByTag(nonexistent) count = %d, want 0", len(none))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestContainersByTag -v`
Expected: FAIL — `s.ContainersByTag` undefined

- [ ] **Step 3: Implement `ContainersByTag`**

Add to `internal/store/tags.go` after the `ItemsByTag` method:

```go
// ContainersByTag returns all containers whose TagIDs include tagID or any of its descendants.
func (s *Store) ContainersByTag(tagID string) []Container {
	s.mu.RLock()
	defer s.mu.RUnlock()

	relevant := make(map[string]struct{})
	relevant[tagID] = struct{}{}
	for _, id := range s.tagDescendantsLocked(tagID) {
		relevant[id] = struct{}{}
	}

	var result []Container
	for _, c := range s.containers {
		for _, tid := range c.TagIDs {
			if _, ok := relevant[tid]; ok {
				result = append(result, *c)
				break
			}
		}
	}
	return result
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestContainersByTag -v`
Expected: PASS

- [ ] **Step 5: Run all store tests**

Run: `go test ./internal/store/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store/tags.go internal/store/tags_test.go
git commit -m "feat(store): add ContainersByTag method with descendant BFS"
```

---

### Task 2: Tag Detail Page Handler + View Model

**Files:**
- Modify: `internal/ui/handlers_tags.go` (add handler + view models)
- Modify: `internal/ui/server.go` (register route)

- [ ] **Step 1: Add view models to `handlers_tags.go`**

Add at the top of `internal/ui/handlers_tags.go` (after imports):

```go
// TagStats holds statistics for a tag detail page.
type TagStats struct {
	ItemCount      int
	ContainerCount int
	TotalQuantity  int
}

// TagDetailData is the view model for the tag detail page.
type TagDetailData struct {
	Tag        store.Tag
	Path       []store.Tag
	Items      []store.Item
	Containers []store.Container
	Stats      TagStats
	Children   []store.Tag
}
```

- [ ] **Step 2: Add `HandleTagView` handler**

Add to `internal/ui/handlers_tags.go`:

```go
// HandleTagView renders the tag detail page showing all items and containers with this tag.
func (s *Server) HandleTagView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tag := s.tags.GetTag(id)
	if tag == nil {
		http.NotFound(w, r)
		return
	}

	// ItemsByTag and ContainersByTag live on *store.Store (not TagService interface).
	// Access via s.store which is the *store.Store field on ui.Server.
	items := s.store.ItemsByTag(id)
	containers := s.store.ContainersByTag(id)

	totalQty := 0
	for _, item := range items {
		totalQty += item.Quantity
	}

	path := s.tags.TagPath(id)
	children := s.tags.TagChildren(id)

	data := TagDetailData{
		Tag:        *tag,
		Path:       path,
		Items:      items,
		Containers: containers,
		Stats: TagStats{
			ItemCount:      len(items),
			ContainerCount: len(containers),
			TotalQuantity:  totalQty,
		},
		Children: children,
	}

	// render takes 4 args: (w, r, templateName, data)
	// Template name "tag-detail" maps to pages/tags/tag_detail.html
	s.render(w, r, "tag-detail", data)
}
```

- [ ] **Step 4: Register route in `server.go`**

Add to the tag routes section in `internal/ui/server.go`:

```go
mux.HandleFunc("GET /ui/tags/{id}", s.HandleTagView)
```

**Important:** This must come AFTER `GET /ui/tags` (exact match). Go 1.22+ ServeMux handles this correctly — the wildcard pattern is more specific for paths like `/ui/tags/some-uuid`.

- [ ] **Step 5: Build and verify compilation**

Run: `make build-mac`
Expected: Compiles without errors

- [ ] **Step 6: Commit**

```bash
git add internal/ui/handlers_tags.go internal/ui/server.go
git commit -m "feat(ui): add HandleTagView handler and TagDetailData view model"
```

---

### Task 3: Tag Detail Page Template

**Files:**
- Create: `internal/embedded/templates/pages/tags/tag_detail.html`
- Modify: `internal/embedded/static/i18n/en/tags.json` (new keys)
- Modify: `internal/embedded/static/i18n/pl/tags.json` (new keys)

- [ ] **Step 1: Add translation keys**

**Merge** the following keys into the existing JSON object in `internal/embedded/static/i18n/en/tags.json` (add before the closing `}`; don't forget a comma after the last existing key):

```json
"tags.detail_title": "Tag: {0}",
"tags.stats_items": "Items",
"tags.stats_containers": "Containers",
"tags.stats_total_quantity": "Total quantity",
"tags.containers_section": "Containers",
"tags.items_section": "Items",
"tags.children_section": "Child tags",
"tags.no_tagged_objects": "No items or containers tagged with this tag.",
"tags.quantity_label": "qty",
"tags.create_tag_prompt": "Create \"{0}\"?"
```

**Merge** equivalent Polish translations into `internal/embedded/static/i18n/pl/tags.json`:

```json
"tags.detail_title": "Tag: {0}",
"tags.stats_items": "Przedmioty",
"tags.stats_containers": "Kontenery",
"tags.stats_total_quantity": "Łączna ilość",
"tags.containers_section": "Kontenery",
"tags.items_section": "Przedmioty",
"tags.children_section": "Podtagi",
"tags.no_tagged_objects": "Brak przedmiotów i kontenerów z tym tagiem.",
"tags.quantity_label": "szt",
"tags.create_tag_prompt": "Utwórz \"{0}\"?"
```

**Note:** `tags.search_tags` and other keys already exist — do not duplicate them.

- [ ] **Step 2: Create tag detail template**

Create `internal/embedded/templates/pages/tags/tag_detail.html`:

```html
{{ define "tag-detail" }}
<div class="tag-detail-view">
    <nav class="breadcrumb">
        <a href="/ui/tags" hx-get="/ui/tags" hx-target="#content">{{.T "tags.title"}}</a>
        {{ range .Data.Path }}
        <span class="sep">/</span>
        <a href="/ui/tags/{{ .ID }}" hx-get="/ui/tags/{{ .ID }}" hx-target="#content">{{ .Name }}</a>
        {{ end }}
    </nav>

    <h1>{{ .Data.Tag.Name }}</h1>

    <div class="tag-stats">
        <div class="stat">
            <span class="stat-value">{{ .Data.Stats.ItemCount }}</span>
            <span class="stat-label">{{.T "tags.stats_items"}}</span>
        </div>
        <div class="stat">
            <span class="stat-value">{{ .Data.Stats.ContainerCount }}</span>
            <span class="stat-label">{{.T "tags.stats_containers"}}</span>
        </div>
        <div class="stat">
            <span class="stat-value">{{ .Data.Stats.TotalQuantity }}</span>
            <span class="stat-label">{{.T "tags.stats_total_quantity"}}</span>
        </div>
    </div>

    {{ if and (eq .Data.Stats.ItemCount 0) (eq .Data.Stats.ContainerCount 0) }}
    <p class="empty-state">{{.T "tags.no_tagged_objects"}}</p>
    {{ end }}

    {{ if .Data.Containers }}
    <section>
        <h3>{{.T "tags.containers_section"}}</h3>
        <ul class="container-list">
            {{ range .Data.Containers }}
            <li>
                <a href="/ui/containers/{{ .ID }}" hx-get="/ui/containers/{{ .ID }}" hx-target="#content" class="container-item">
                    <span class="name"><span class="color-dot"{{ if .Color }} style="background-color: {{ paletteHex .Color }}"{{ end }}></span><span class="entity-icon">{{ if .Icon }}{{ icon .Icon }}{{ else }}{{ icon "package" }}{{ end }}</span>{{ .Name }}</span>
                    {{ if .Description }}<span class="desc">{{ .Description }}</span>{{ end }}
                </a>
            </li>
            {{ end }}
        </ul>
    </section>
    {{ end }}

    {{ if .Data.Items }}
    <section>
        <h3>{{.T "tags.items_section"}}</h3>
        <ul class="item-list">
            {{ range .Data.Items }}
            <li>
                <a href="/ui/items/{{ .ID }}" hx-get="/ui/items/{{ .ID }}" hx-target="#content" class="item-item">
                    <span class="name"><span class="color-dot"{{ if .Color }} style="background-color: {{ paletteHex .Color }}"{{ end }}></span><span class="entity-icon">{{ if .Icon }}{{ icon .Icon }}{{ else }}{{ icon "clipboard-text" }}{{ end }}</span>{{ .Name }}</span>
                    {{ if .Description }}<span class="desc">{{ .Description }}</span>{{ end }}
                </a>
                {{ if .TagIDs }}
                {{ template "tag-chips" dict "Data" (dict "ObjectID" .ID "ObjectType" "item" "Tags" (resolveTags .TagIDs)) }}
                {{ end }}
            </li>
            {{ end }}
        </ul>
    </section>
    {{ end }}

    {{ if .Data.Children }}
    <section>
        <h3>{{.T "tags.children_section"}}</h3>
        <ul class="tag-list">
            {{ range .Data.Children }}
            <li>
                <a href="/ui/tags/{{ .ID }}" hx-get="/ui/tags/{{ .ID }}" hx-target="#content">{{ .Name }}</a>
            </li>
            {{ end }}
        </ul>
    </section>
    {{ end }}
</div>
{{ end }}
```

- [ ] **Step 3: Build and manually verify**

Run: `make build-mac && make run`
Navigate to a tag's URL like `/ui/tags/{some-tag-id}` in the browser.
Expected: Tag detail page renders with breadcrumb, stats, lists.

- [ ] **Step 4: Commit**

```bash
git add internal/embedded/templates/pages/tags/tag_detail.html internal/embedded/static/i18n/en/tags.json internal/embedded/static/i18n/pl/tags.json
git commit -m "feat(ui): add tag detail page template with stats and translations"
```

---

### Task 4: Tag Chip Navigation + Container Tag Chips

**Files:**
- Modify: `internal/embedded/templates/partials/tags/tag_chips.html`
- Modify: `internal/embedded/templates/pages/inventory/containers.html`
- Modify: `internal/embedded/templates/pages/inventory/item.html`

- [ ] **Step 1: Update tag chips — make name a link**

Replace the content of `internal/embedded/templates/partials/tags/tag_chips.html`:

```html
{{ define "tag-chips" }}
<div class="tag-chips" id="tag-chips-{{ .Data.ObjectID }}">
    {{ range .Data.Tags }}
    <span class="tag-chip"{{ if .Color }} style="background-color: {{ paletteHex .Color }}22; border-color: {{ paletteHex .Color }}"{{ end }}>
        {{ if .Icon }}<span class="entity-icon">{{ icon .Icon }}</span>{{ end }}
        <a href="/ui/tags/{{ .ID }}" hx-get="/ui/tags/{{ .ID }}" hx-target="#content" class="tag-name">{{ .Name }}</a>
        <button class="tag-remove"
                hx-delete="/ui/actions/{{ $.Data.ObjectType }}s/{{ $.Data.ObjectID }}/tags/{{ .ID }}"
                hx-target="#tag-chips-{{ $.Data.ObjectID }}"
                hx-swap="outerHTML">&times;</button>
    </span>
    {{ end }}
    <button class="tag-add" data-object-id="{{ .Data.ObjectID }}" data-object-type="{{ .Data.ObjectType }}">+</button>
</div>
{{ end }}
```

- [ ] **Step 2: Add tag chips to container list items**

In `internal/embedded/templates/pages/inventory/containers.html`, after the container `<a>` tag in the children loop (after line 27 `</a>`), add:

```html
                {{ if .TagIDs }}
                {{ template "tag-chips" dict "Data" (dict "ObjectID" .ID "ObjectType" "container" "Tags" (resolveTags .TagIDs)) }}
                {{ end }}
```

- [ ] **Step 3: Add tag chips to item detail page**

In `internal/embedded/templates/pages/inventory/item.html`, add a tag chips section (after the description, before the print section). Find the appropriate location and add:

```html
    {{ if .Data.Item.TagIDs }}
    <section class="item-tags">
        {{ template "tag-chips" dict "Data" (dict "ObjectID" .Data.Item.ID "ObjectType" "item" "Tags" (resolveTags .Data.Item.TagIDs)) }}
    </section>
    {{ end }}
```

- [ ] **Step 4: Add CSS for tag name link**

Add to `internal/embedded/static/css/tags/tag-chips.css`:

```css
.tag-chip .tag-name { color: inherit; text-decoration: none; }
.tag-chip .tag-name:hover { text-decoration: underline; }
```

- [ ] **Step 5: Build and verify**

Run: `make build-mac && make run`
Check: tag names are clickable links navigating to `/ui/tags/{id}`, container list shows tag chips, item detail shows tag chips.

- [ ] **Step 6: Commit**

```bash
git add internal/embedded/templates/partials/tags/tag_chips.html internal/embedded/templates/pages/inventory/containers.html internal/embedded/templates/pages/inventory/item.html internal/embedded/static/css/tags/tag-chips.css
git commit -m "feat(ui): tag chip navigation links and container/item tag chips"
```

---

### Task 5: TagAutocomplete CSS

**Files:**
- Create: `internal/embedded/static/css/tags/tag-autocomplete.css`
- Modify: `internal/embedded/templates/layouts/base.html`

- [ ] **Step 1: Create autocomplete CSS**

Create `internal/embedded/static/css/tags/tag-autocomplete.css`:

```css
.tag-ac-wrap { position: relative; display: inline-block; }

.tag-ac-dropdown {
    position: absolute;
    z-index: 100;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: 6px;
    box-shadow: 0 4px 12px rgba(0,0,0,.15);
    max-height: 16rem;
    overflow-y: auto;
    width: max-content;
    min-width: 12rem;
    max-width: 20rem;
}
.tag-ac-dropdown.above { bottom: 100%; margin-bottom: 2px; }
.tag-ac-dropdown.below { top: 100%; margin-top: 2px; }

.tag-ac-dropdown[role="listbox"] { list-style: none; margin: 0; padding: 0.25rem 0; }

.tag-ac-option {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.35rem 0.6rem;
    cursor: pointer;
    font-size: 0.85rem;
}
.tag-ac-option:hover,
.tag-ac-option[aria-selected="true"] {
    background: var(--color-bg-alt);
}
.tag-ac-option .color-dot {
    width: 8px; height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
}
.tag-ac-option .entity-icon svg { width: 12px; height: 12px; }
.tag-ac-option.create { font-style: italic; color: var(--color-text-muted); }

.tag-ac-error {
    padding: 0.35rem 0.6rem;
    font-size: 0.85rem;
    color: var(--color-danger, #c33);
}

.tag-ac-input {
    font-size: 0.85rem;
    padding: 0.15rem 0.4rem;
    border: 1px dashed var(--color-border);
    border-radius: 12px;
    background: var(--color-bg);
    outline: none;
    width: 6rem;
}
```

- [ ] **Step 2: Include CSS in base layout**

In `internal/embedded/templates/layouts/base.html`, add after the `tag-chips.css` link:

```html
    <link rel="stylesheet" href="/static/css/tags/tag-autocomplete.css">
```

- [ ] **Step 3: Commit**

```bash
git add internal/embedded/static/css/tags/tag-autocomplete.css internal/embedded/templates/layouts/base.html
git commit -m "feat(ui): add tag autocomplete CSS styles"
```

---

### Task 6: TagAutocomplete JS Component

**Files:**
- Create: `internal/embedded/static/js/tags/tag-autocomplete.js`
- Modify: `internal/embedded/templates/layouts/base.html` (add script tag)

- [ ] **Step 1: Create the autocomplete component**

Create `internal/embedded/static/js/tags/tag-autocomplete.js`. Uses safe DOM methods only (`createElement`, `textContent`, `appendChild`). No `innerHTML`.

```js
(function () {
  var qlx = window.qlx = window.qlx || {};
  var cache = null;
  var debounceTimer = null;

  function invalidateCache() { cache = null; }

  function fetchTags() {
    if (cache) return Promise.resolve(cache);
    return fetch("/api/tags")
      .then(function (r) { return r.json(); })
      .then(function (tags) { cache = tags || []; return cache; });
  }

  function filterTags(tags, query) {
    var q = query.toLowerCase();
    var exact = false;
    var results = tags.filter(function (t) {
      if (t.name.toLowerCase() === q) exact = true;
      return t.name.toLowerCase().indexOf(q) !== -1;
    });
    return { results: results.slice(0, 8), exactMatch: exact };
  }

  function createTag(name) {
    return fetch("/api/tags", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: name, color: "gray", icon: "" })
    }).then(function (r) {
      if (!r.ok) return r.json().then(function (d) { throw new Error(d.error || "Error"); });
      invalidateCache();
      return r.json();
    });
  }

  function positionDropdown(dropdown, anchor) {
    var rect = anchor.getBoundingClientRect();
    var spaceBelow = window.innerHeight - rect.bottom;
    dropdown.classList.remove("above", "below");
    if (spaceBelow < 200 && rect.top > spaceBelow) {
      dropdown.classList.add("above");
    } else {
      dropdown.classList.add("below");
    }
  }

  // Map palette names to hex codes (subset from palette.Colors).
  // Kept in sync manually — only used for color dots in autocomplete dropdown.
  // Palette name → hex (from internal/shared/palette/colors.go)
  var paletteHex = {
    red: "#e94560", orange: "#f4845f", amber: "#f5a623", yellow: "#ffc93c",
    green: "#4ecca3", teal: "#2ec4b6", blue: "#4d9de0", indigo: "#7b6cf6",
    purple: "#b07cd8", pink: "#e84393"
  };

  function buildOption(tag, index, onPick) {
    var opt = document.createElement("div");
    opt.className = "tag-ac-option";
    opt.setAttribute("role", "option");
    opt.setAttribute("data-index", String(index));
    opt.setAttribute("data-id", tag.id);

    var dot = document.createElement("span");
    dot.className = "color-dot";
    if (tag.color) dot.style.backgroundColor = paletteHex[tag.color] || tag.color;
    opt.appendChild(dot);

    var nameSpan = document.createElement("span");
    nameSpan.textContent = tag.name;
    opt.appendChild(nameSpan);

    opt.addEventListener("mousedown", function (e) {
      e.preventDefault();
      onPick(tag);
    });
    return opt;
  }

  function buildCreateOption(query, index, onCreate) {
    var opt = document.createElement("div");
    opt.className = "tag-ac-option create";
    opt.setAttribute("role", "option");
    opt.setAttribute("data-index", String(index));

    var label = document.createElement("span");
    var promptText = qlx.t ? qlx.t("tags.create_tag_prompt") : 'Create "{0}"?';
    label.textContent = promptText.replace("{0}", query);
    opt.appendChild(label);

    opt.addEventListener("mousedown", function (e) {
      e.preventDefault();
      onCreate(query);
    });
    return opt;
  }

  function renderDropdown(tags, query, onPick, onCreate) {
    var div = document.createElement("div");
    div.className = "tag-ac-dropdown below";
    div.setAttribute("role", "listbox");

    tags.forEach(function (tag, i) {
      div.appendChild(buildOption(tag, i, onPick));
    });

    if (onCreate && query.length > 0) {
      div.appendChild(buildCreateOption(query, tags.length, onCreate));
    }

    return div;
  }

  qlx.TagAutocomplete = function (opts) {
    var anchor = opts.anchor;
    var onSelect = opts.onSelect || function () {};
    var onCancel = opts.onCancel || function () {};
    var dropdown = null;
    var activeIndex = -1;
    var input = null;

    function open(inputEl) {
      input = inputEl;
      update(input.value);
      input.addEventListener("input", onInput);
      input.addEventListener("keydown", onKeydown);
      input.addEventListener("blur", onBlur);
    }

    function close() {
      if (dropdown && dropdown.parentNode) dropdown.parentNode.removeChild(dropdown);
      dropdown = null;
      activeIndex = -1;
      if (input) {
        input.removeEventListener("input", onInput);
        input.removeEventListener("keydown", onKeydown);
        input.removeEventListener("blur", onBlur);
        input = null;
      }
    }

    function onBlur() {
      setTimeout(function () { close(); onCancel(); }, 150);
    }

    function onInput() {
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(function () { update(input.value); }, 150);
    }

    function onKeydown(e) {
      if (!dropdown) return;
      var options = dropdown.querySelectorAll("[role=option]");
      if (e.key === "ArrowDown") {
        e.preventDefault();
        activeIndex = Math.min(activeIndex + 1, options.length - 1);
        highlightOption(options);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        activeIndex = Math.max(activeIndex - 1, 0);
        highlightOption(options);
      } else if (e.key === "Enter" && activeIndex >= 0) {
        e.preventDefault();
        options[activeIndex].dispatchEvent(new MouseEvent("mousedown"));
      } else if (e.key === "Escape") {
        e.preventDefault();
        close();
        onCancel();
      }
    }

    function highlightOption(options) {
      options.forEach(function (o, i) {
        o.setAttribute("aria-selected", i === activeIndex ? "true" : "false");
      });
      if (input && activeIndex >= 0) {
        input.setAttribute("aria-activedescendant", options[activeIndex].getAttribute("data-id") || "");
      }
    }

    function update(query) {
      var currentQuery = query.trim();
      fetchTags().then(function (tags) {
        var filtered = filterTags(tags, currentQuery);
        if (dropdown && dropdown.parentNode) dropdown.parentNode.removeChild(dropdown);
        activeIndex = -1;
        var showCreate = !filtered.exactMatch && currentQuery.length > 0;
        dropdown = renderDropdown(
          filtered.results,
          currentQuery,
          function (tag) { close(); onSelect(tag); },
          showCreate ? function (name) {
            createTag(name).then(function (tag) {
              close();
              onSelect(tag);
            }).catch(function (err) {
              showError(err.message);
            });
          } : null
        );
        positionDropdown(dropdown, anchor);
        anchor.parentNode.style.position = "relative";
        anchor.parentNode.appendChild(dropdown);
      });
    }

    function showError(msg) {
      if (!dropdown) return;
      var existing = dropdown.querySelector(".tag-ac-error");
      if (existing) existing.parentNode.removeChild(existing);
      var err = document.createElement("div");
      err.className = "tag-ac-error";
      err.textContent = msg;
      dropdown.appendChild(err);
    }

    return { open: open, close: close };
  };

  qlx.invalidateTagCache = invalidateCache;
})();
```

- [ ] **Step 2: Include JS in base layout**

In `internal/embedded/templates/layouts/base.html`, add after `tag-picker.js` script:

```html
    <script src="/static/js/tags/tag-autocomplete.js" defer></script>
```

- [ ] **Step 3: Build and verify**

Run: `make build-mac`
Expected: Compiles. `qlx.TagAutocomplete` is now available globally.

- [ ] **Step 4: Commit**

```bash
git add internal/embedded/static/js/tags/tag-autocomplete.js internal/embedded/templates/layouts/base.html
git commit -m "feat(ui): add TagAutocomplete JS component with cache, ARIA, and create"
```

---

### Task 7: Inline Mode — `+` Button on Tag Chips

**Files:**
- Create: `internal/embedded/static/js/tags/tag-inline.js` (new file for inline mode wiring)
- Modify: `internal/embedded/templates/layouts/base.html` (include script)

- [ ] **Step 1: Create inline mode wiring**

Create `internal/embedded/static/js/tags/tag-inline.js`. Uses safe DOM methods only — no `innerHTML`.

```js
(function () {
  var qlx = window.qlx = window.qlx || {};

  function initInlineTagAdd() {
    document.addEventListener("click", function (e) {
      var btn = e.target.closest(".tag-add");
      if (!btn) return;

      var objectId = btn.getAttribute("data-object-id");
      var objectType = btn.getAttribute("data-object-type");
      var chipsDiv = btn.closest(".tag-chips");

      // Replace button with input
      var input = document.createElement("input");
      input.type = "text";
      input.className = "tag-ac-input";
      input.placeholder = qlx.t ? qlx.t("tags.search_tags") : "Tag...";
      btn.style.display = "none";
      chipsDiv.appendChild(input);
      input.focus();

      var ac = qlx.TagAutocomplete({
        anchor: input,
        onSelect: function (tag) {
          // POST assign tag — response is the tag-chips partial HTML
          fetch("/ui/actions/" + objectType + "s/" + objectId + "/tags", {
            method: "POST",
            headers: { "Content-Type": "application/x-www-form-urlencoded" },
            body: "tag_id=" + encodeURIComponent(tag.id)
          }).then(function (resp) {
            if (resp.ok) {
              // Refresh chips via HTMX
              htmx.ajax("GET", window.location.pathname, { target: "#content" });
            }
          });
          cleanup();
        },
        onCancel: function () {
          cleanup();
        }
      });

      function cleanup() {
        if (input.parentNode) input.parentNode.removeChild(input);
        btn.style.display = "";
      }

      ac.open(input);
    });
  }

  // Init on load — uses event delegation so works for dynamically added chips
  document.addEventListener("DOMContentLoaded", initInlineTagAdd);
})();
```

- [ ] **Step 2: Include in base layout**

Add to `internal/embedded/templates/layouts/base.html` after `tag-autocomplete.js`:

```html
    <script src="/static/js/tags/tag-inline.js" defer></script>
```

- [ ] **Step 3: Build and test manually**

Run: `make build-mac && make run`
Test: Navigate to an item with tags, click `+`, type in the input, select a tag. Verify chip appears.

- [ ] **Step 4: Commit**

```bash
git add internal/embedded/static/js/tags/tag-inline.js internal/embedded/templates/layouts/base.html
git commit -m "feat(ui): wire inline tag add via + button on tag chips"
```

---

### Task 8: Hash Mode — Quick-Entry `#` Trigger

**Files:**
- Create: `internal/embedded/static/js/tags/tag-hash.js`
- Modify: `internal/embedded/templates/layouts/base.html`

- [ ] **Step 1: Create hash mode wiring**

Create `internal/embedded/static/js/tags/tag-hash.js`:

```js
(function () {
  var qlx = window.qlx = window.qlx || {};
  var pendingTagId = null;
  var activeHashAC = null; // guard against re-entrancy

  function initHashTagging() {
    document.addEventListener("input", function (e) {
      var input = e.target;
      if (!input.matches || !input.matches(".quick-entry input[name=name]")) return;

      var val = input.value;
      var hashIdx = val.indexOf("#");
      if (hashIdx === -1) return;

      var query = val.substring(hashIdx + 1);
      if (query.indexOf(" ") !== -1) return; // stop at space after #
      if (query.length === 0) return;

      // Prevent multiple simultaneous instances
      if (activeHashAC) return;

      var ac = qlx.TagAutocomplete({
        anchor: input,
        onSelect: function (tag) {
          // Strip #query from the name
          var before = input.value.substring(0, hashIdx);
          var afterHash = input.value.substring(hashIdx);
          var spaceIdx = afterHash.indexOf(" ", 1);
          var after = spaceIdx !== -1 ? afterHash.substring(spaceIdx) : "";
          input.value = (before + after).trim();

          activeHashAC = null; // clear guard
          pendingTagId = tag.id;

          // Determine object type from the form
          var form = input.closest(".quick-entry");
          var isItem = !!form.querySelector("input[name=container_id]");
          var objectType = isItem ? "item" : "container";
          var targetSelector = form.getAttribute("hx-target");

          // Register one-shot afterSwap listener
          var target = document.querySelector(targetSelector);
          if (target) {
            var handler = function () {
              target.removeEventListener("htmx:afterSwap", handler);
              if (!pendingTagId) return;

              var newEl = target.querySelector("li[data-id]:last-of-type");
              if (!newEl) { pendingTagId = null; return; }

              var newId = newEl.getAttribute("data-id");
              fetch("/ui/actions/" + objectType + "s/" + newId + "/tags", {
                method: "POST",
                headers: { "Content-Type": "application/x-www-form-urlencoded" },
                body: "tag_id=" + encodeURIComponent(pendingTagId)
              }).then(function (resp) {
                if (resp.ok) {
                  // Refresh the page to show the tag chip
                  htmx.ajax("GET", window.location.pathname, { target: "#content" });
                }
              });
              pendingTagId = null;
            };
            target.addEventListener("htmx:afterSwap", handler);
          }
        },
        onCancel: function () {
          activeHashAC = null; // clear guard
          // Leave the # text as-is if user cancels
        }
      });

      activeHashAC = ac;
      ac.open(input);
    });
  }

  document.addEventListener("DOMContentLoaded", initHashTagging);
})();
```

- [ ] **Step 2: Include in base layout**

Add to `internal/embedded/templates/layouts/base.html` after `tag-inline.js`:

```html
    <script src="/static/js/tags/tag-hash.js" defer></script>
```

- [ ] **Step 3: Build and test manually**

Run: `make build-mac && make run`
Test: Navigate to a container, type `Test Item #` in the item quick-entry. Verify autocomplete opens, select a tag, submit. New item should appear with the tag.

- [ ] **Step 4: Commit**

```bash
git add internal/embedded/static/js/tags/tag-hash.js internal/embedded/templates/layouts/base.html
git commit -m "feat(ui): add hash trigger for tag autocomplete in quick-entry"
```

---

### Task 9: Field Mode — Edit/Create Forms

**Files:**
- Create: `internal/embedded/static/js/tags/tag-field.js`
- Modify: `internal/embedded/templates/pages/inventory/item_form.html`
- Modify: `internal/embedded/templates/pages/inventory/container_form.html`
- Modify: `internal/embedded/templates/layouts/base.html`

- [ ] **Step 1: Add tag field section to item form**

In `internal/embedded/templates/pages/inventory/item_form.html`, before the submit button, add:

```html
    {{ if .Data.Item }}
    <div class="form-group">
        <label>{{.T "tags.title"}}</label>
        <div id="form-tag-chips-{{ .Data.Item.ID }}">
            {{ if .Data.Item.TagIDs }}
            {{ template "tag-chips" dict "Data" (dict "ObjectID" .Data.Item.ID "ObjectType" "item" "Tags" (resolveTags .Data.Item.TagIDs)) }}
            {{ end }}
        </div>
        <input type="text" class="tag-ac-input tag-field-input" placeholder="{{ .T "tags.search_tags" }}" data-object-id="{{ .Data.Item.ID }}" data-object-type="item">
    </div>
    {{ end }}
```

- [ ] **Step 2: Add tag field section to container form**

Same pattern in `internal/embedded/templates/pages/inventory/container_form.html`:

```html
    {{ if .Data.Container }}
    <div class="form-group">
        <label>{{.T "tags.title"}}</label>
        <div id="form-tag-chips-{{ .Data.Container.ID }}">
            {{ if .Data.Container.TagIDs }}
            {{ template "tag-chips" dict "Data" (dict "ObjectID" .Data.Container.ID "ObjectType" "container" "Tags" (resolveTags .Data.Container.TagIDs)) }}
            {{ end }}
        </div>
        <input type="text" class="tag-ac-input tag-field-input" placeholder="{{ .T "tags.search_tags" }}" data-object-id="{{ .Data.Container.ID }}" data-object-type="container">
    </div>
    {{ end }}
```

- [ ] **Step 3: Create field mode wiring**

Create `internal/embedded/static/js/tags/tag-field.js`. Uses safe DOM methods only — no `innerHTML`.

```js
(function () {
  var qlx = window.qlx = window.qlx || {};

  function initFieldTagging() {
    document.addEventListener("focus", function (e) {
      var input = e.target;
      if (!input.matches || !input.matches(".tag-field-input")) return;

      var objectId = input.getAttribute("data-object-id");
      var objectType = input.getAttribute("data-object-type");

      var ac = qlx.TagAutocomplete({
        anchor: input,
        onSelect: function (tag) {
          input.value = "";
          // POST assign tag
          fetch("/ui/actions/" + objectType + "s/" + objectId + "/tags", {
            method: "POST",
            headers: { "Content-Type": "application/x-www-form-urlencoded" },
            body: "tag_id=" + encodeURIComponent(tag.id)
          }).then(function (resp) {
            if (resp.ok) {
              // Refresh the whole page to update chips
              htmx.ajax("GET", window.location.pathname, { target: "#content" });
            }
          });
        },
        onCancel: function () {
          input.value = "";
        }
      });

      ac.open(input);
    }, true); // capture phase for focus
  }

  document.addEventListener("DOMContentLoaded", initFieldTagging);
})();
```

- [ ] **Step 4: Include in base layout**

Add to `internal/embedded/templates/layouts/base.html` after `tag-hash.js`:

```html
    <script src="/static/js/tags/tag-field.js" defer></script>
```

- [ ] **Step 5: Build and test manually**

Run: `make build-mac && make run`
Test: Edit an item, focus the tag field input, type a tag name, select it. Tag chip should appear.

- [ ] **Step 6: Commit**

```bash
git add internal/embedded/static/js/tags/tag-field.js internal/embedded/templates/pages/inventory/item_form.html internal/embedded/templates/pages/inventory/container_form.html internal/embedded/templates/layouts/base.html
git commit -m "feat(ui): add tag field with autocomplete in edit forms"
```

---

### Task 10: Lint + Full Test Pass

- [ ] **Step 1: Run linter**

Run: `make lint`
Expected: No errors. Fix any issues.

- [ ] **Step 2: Run all Go tests**

Run: `make test`
Expected: All PASS

- [ ] **Step 3: Fix any issues and commit**

```bash
git add -u
git commit -m "fix: address lint and test issues"
```

---

### Task 11: E2E Tests

**Files:**
- Create: `e2e/tests/tag-ui.spec.ts`

- [ ] **Step 1: Write E2E tests**

Create `e2e/tests/tag-ui.spec.ts`:

```typescript
import { test, expect } from '../fixtures/app';

test.describe('Tag UI improvements', () => {

  test('tag chip links navigate to tag detail page', async ({ page, app }) => {
    // Setup: create a container, item, tag, and assign tag
    const baseURL = app.baseURL;
    await page.request.post(`${baseURL}/api/containers`, {
      data: { name: 'Test Container' }
    });
    const containerResp = await page.request.get(`${baseURL}/api/containers`);
    const containers = await containerResp.json();
    const containerId = containers[0].id;

    await page.request.post(`${baseURL}/api/items`, {
      data: { name: 'Test Item', container_id: containerId }
    });
    const itemsResp = await page.request.get(`${baseURL}/api/items`);
    const items = await itemsResp.json();
    const itemId = items[0].id;

    await page.request.post(`${baseURL}/api/tags`, {
      data: { name: 'TestTag', color: 'blue', icon: '' }
    });
    const tagsResp = await page.request.get(`${baseURL}/api/tags`);
    const tags = await tagsResp.json();
    const tagId = tags[0].id;

    await page.request.post(`${baseURL}/api/items/${itemId}/tags`, {
      data: { tag_id: tagId }
    });

    // Navigate to container view
    await page.goto(`${baseURL}/ui/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    // Click the tag chip name
    const tagLink = page.locator('.tag-chip .tag-name', { hasText: 'TestTag' });
    await expect(tagLink).toBeVisible();

    const responsePromise = page.waitForResponse(r =>
      r.url().includes(`/ui/tags/${tagId}`) && r.status() === 200
    );
    await tagLink.click();
    await responsePromise;

    // Verify tag detail page
    await expect(page.locator('h1')).toContainText('TestTag');
    await expect(page.locator('.tag-stats')).toBeVisible();
    await expect(page.locator('.stat-value').first()).toContainText('1'); // 1 item
  });

  test('tag detail page shows statistics and tagged objects', async ({ page, app }) => {
    const baseURL = app.baseURL;

    // Setup: create tag, container, items with tag
    const tagResp = await page.request.post(`${baseURL}/api/tags`, {
      data: { name: 'StatsTag', color: 'green', icon: '' }
    });
    const tag = await tagResp.json();

    const contResp = await page.request.post(`${baseURL}/api/containers`, {
      data: { name: 'Tagged Container' }
    });
    const container = await contResp.json();
    await page.request.post(`${baseURL}/api/containers/${container.id}/tags`, {
      data: { tag_id: tag.id }
    });

    const itemResp = await page.request.post(`${baseURL}/api/items`, {
      data: { name: 'Tagged Item', container_id: container.id, quantity: 5 }
    });
    const item = await itemResp.json();
    await page.request.post(`${baseURL}/api/items/${item.id}/tags`, {
      data: { tag_id: tag.id }
    });

    // Navigate to tag detail
    await page.goto(`${baseURL}/ui/tags/${tag.id}`, { waitUntil: 'domcontentloaded' });

    // Verify stats
    await expect(page.locator('.tag-stats')).toContainText('1'); // 1 item
    await expect(page.locator('.tag-stats')).toContainText('1'); // 1 container
    await expect(page.locator('.tag-stats')).toContainText('5'); // total qty

    // Verify listed objects
    await expect(page.locator('.container-list')).toContainText('Tagged Container');
    await expect(page.locator('.item-list')).toContainText('Tagged Item');
  });

  test('inline + button opens autocomplete and assigns tag', async ({ page, app }) => {
    const baseURL = app.baseURL;

    // Setup
    const tagResp = await page.request.post(`${baseURL}/api/tags`, {
      data: { name: 'InlineTag', color: 'red', icon: '' }
    });
    const tag = await tagResp.json();

    const contResp = await page.request.post(`${baseURL}/api/containers`, {
      data: { name: 'Inline Container' }
    });
    const container = await contResp.json();
    const itemResp = await page.request.post(`${baseURL}/api/items`, {
      data: { name: 'Inline Item', container_id: container.id }
    });
    const item = await itemResp.json();

    // Assign a tag first so + button is visible (tag-chips renders only if TagIDs not empty)
    const otherTag = await (await page.request.post(`${baseURL}/api/tags`, {
      data: { name: 'OtherTag', color: 'blue', icon: '' }
    })).json();
    await page.request.post(`${baseURL}/api/items/${item.id}/tags`, {
      data: { tag_id: otherTag.id }
    });

    // Navigate to container
    await page.goto(`${baseURL}/ui/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    // Click + button
    const addBtn = page.locator('.tag-add').first();
    await expect(addBtn).toBeVisible();
    await addBtn.click();

    // Input should appear
    const input = page.locator('.tag-ac-input');
    await expect(input).toBeVisible();
    await input.fill('Inline');

    // Dropdown should show InlineTag
    const option = page.locator('.tag-ac-option', { hasText: 'InlineTag' });
    await expect(option).toBeVisible();
    await option.click();

    // Tag chip should appear
    await expect(page.locator('.tag-chip', { hasText: 'InlineTag' })).toBeVisible();
  });

  test('container list shows tag chips', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const tagResp = await page.request.post(`${baseURL}/api/tags`, {
      data: { name: 'ContTag', color: 'purple', icon: '' }
    });
    const tag = await tagResp.json();

    const contResp = await page.request.post(`${baseURL}/api/containers`, {
      data: { name: 'Root' }
    });
    const root = await contResp.json();

    const childResp = await page.request.post(`${baseURL}/api/containers`, {
      data: { name: 'Tagged Child', parent_id: root.id }
    });
    const child = await childResp.json();
    await page.request.post(`${baseURL}/api/containers/${child.id}/tags`, {
      data: { tag_id: tag.id }
    });

    await page.goto(`${baseURL}/ui/containers/${root.id}`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('.tag-chip', { hasText: 'ContTag' })).toBeVisible();
  });
});
```

- [ ] **Step 2: Run E2E tests**

Run: `make test-e2e`
Expected: All tests pass. Fix failures.

- [ ] **Step 3: Commit**

```bash
git add e2e/tests/tag-ui.spec.ts
git commit -m "test(e2e): add tag UI improvement tests"
```

---

### Task 12: Final Verification

- [ ] **Step 1: Run all Go tests**

Run: `make test`

- [ ] **Step 2: Run lint**

Run: `make lint`

- [ ] **Step 3: Run E2E tests**

Run: `make test-e2e`

- [ ] **Step 4: Manual smoke test**

Run: `make build-mac && make run`
Verify:
1. Tag chips on items show clickable tag names → navigates to tag detail page
2. Tag chips on containers appear in container list
3. Tag detail page shows stats, items, containers, child tags
4. `+` button on tag chips opens inline autocomplete
5. Quick-entry `#` trigger opens autocomplete, assigns tag after submit
6. Edit form has tag field with autocomplete
7. Empty state on tag detail page when no tagged objects

- [ ] **Step 5: Commit any fixes**

```bash
git add -u
git commit -m "fix: final adjustments from smoke testing"
```
