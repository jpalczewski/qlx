# Batch Operations, Tags & Search — Design Spec

## Overview

Extend QLX inventory UI with batch operations (quick entry, multi-select, bulk move/delete), hierarchical tags, and global search. Approach: hybrid HTMX + vanilla JS, consistent with existing patterns in the codebase.

## 1. Store Migration System

### Problem
Adding fields (quantity, tags) to existing models requires a migration strategy. Ad-hoc fixups don't scale.

### Design
- Top-level `"version": int` field in the JSON store file (absent = version 0)
- On startup, the store checks version and runs sequential migration functions (`v0→v1`, `v1→v2`, ...)
- Each migration is a Go function: `func(data map[string]any) error` operating on raw parsed JSON
- Before each migration, backup the store file atomically to `data/backup-v{N}.json`
- First migration (v0→v1): adds `quantity` to items (default 1), adds `tags` collection, adds `tag_ids` to items and containers

### Atomicity & Startup Order
- Migration must: (1) write backup atomically (temp + rename), (2) run migration function on in-memory data, (3) write migrated data atomically (temp + rename, same pattern as existing `Save()`), (4) only then update in-memory version
- If the process is killed mid-migration, the backup file exists and the store file is either the old version (migration not persisted) or the new version (migration completed). No ambiguous state.
- The server must NOT accept HTTP requests until all migrations complete. `NewStore()` runs migrations before returning.
- On first startup after adding migration support, `NewStore()` must detect missing `version` field (treat as v0) and persist migrated data before returning

### Store File Format (after v1)
```json
{
  "version": 1,
  "containers": { ... },
  "items": { ... },
  "tags": { ... }
}
```

## 2. Data Model Changes

### Item — New Fields
- `Quantity int` (`json:"quantity"`) — default 1, minimum 1
- `TagIDs []string` (`json:"tag_ids"`) — list of tag UUIDs

### Container — New Fields
- `TagIDs []string` (`json:"tag_ids"`) — list of tag UUIDs

### Tag — New Model
```go
type Tag struct {
    ID        string    `json:"id"`
    ParentID  string    `json:"parent_id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}
```
- `ParentID` empty = root-level tag
- Hierarchical: arbitrary nesting depth
- Delete constraint: tag with children cannot be deleted (same pattern as containers)
- On delete of a leaf tag: remove that tag ID from all items' and containers' `TagIDs` slices (cascade cleanup, single `Save()` call)

### Store — New Collections & Methods
- `tags map[string]*Tag` in `storeData`
- CRUD: `CreateTag`, `GetTag`, `UpdateTag`, `DeleteTag`
- Query: `TagChildren(id)`, `TagPath(id)`, `TagDescendants(id)` (single-pass: collect all tags, walk from root down building a flat set — O(N) regardless of depth, safe for MIPS. Expected scale: hundreds of tags, not thousands)
- Assignment: `AddTag(objectType, objectID, tagID)`, `RemoveTag(objectType, objectID, tagID)`
- Filter: `ItemsByTag(tagID)` — returns items whose `TagIDs` intersect with `{tagID} ∪ TagDescendants(tagID)` (filtering by a parent tag matches items carrying any descendant tag)
- Bulk: `MoveItems(ids []string, targetContainerID string)`, `MoveContainers(ids []string, targetParentID string)` — cycle detection must validate ALL moves in the batch before committing any (pre-validate, then apply). This prevents partial moves that create cycles when two containers in the batch are in the same ancestor chain. Single write lock held for the entire operation.
- Bulk: `DeleteItems(ids []string)`, `DeleteContainers(ids []string)` (containers only if empty)
- Search: `SearchContainers(q string)`, `SearchItems(q string)`, `SearchTags(q string)` — case-insensitive substring match on name

## 3. Quick Entry — Batch Adding

### Pattern
Inline form at the bottom of each list. On submit: HTMX `POST` appends a new `<li>` fragment to the list, form resets, focus stays. No page reload, no navigation.

### Container Quick Entry
```html
<!-- Note: <ul id="container-list"> must ALWAYS be rendered (even when empty) -->
<!-- The empty-state message goes inside the <ul> and is removed on first insert -->
<form hx-post="/ui/actions/containers"
      hx-target="#container-list"
      hx-swap="beforeend"
      hx-on::after-request="if(event.detail.successful) this.reset()">
    <input type="hidden" name="parent_id" value="{{.Container.ID}}">
    <input type="text" name="name" placeholder="Nazwa kontenera..." autofocus>
</form>
```

### Item Quick Entry
```html
<!-- Note: <ul id="item-list"> must ALWAYS be rendered (even when empty) -->
<form hx-post="/ui/actions/items"
      hx-target="#item-list"
      hx-swap="beforeend"
      hx-on::after-request="if(event.detail.successful) this.reset()">
    <input type="hidden" name="container_id" value="{{.Container.ID}}">
    <input type="text" name="name" placeholder="Nazwa...">
    <input type="number" name="quantity" value="1" min="1">
</form>
```

### Template Prerequisites
- The existing `<ul class="container-list">` and `<ul class="item-list">` in `containers.html` must get `id` attributes: `id="container-list"` and `id="item-list"`
- Both `<ul>` elements must ALWAYS be rendered (even when the list is empty) so HTMX `beforeend` has a target. The current conditional `<p class="empty">` should move inside the `<ul>` as a `<li class="empty-state">` that the server's quick-entry response removes via OOB swap when the first item is added

### Tag Quick Entry
Same pattern as containers, on the `/ui/tags` page.

### Server-Side Changes
- `HandleContainerCreate` and `HandleItemCreate` in `handlers.go`: when request has `HX-Request` header and targets a list element, return a single `<li>` HTML fragment instead of a redirect/full page
- New partial templates: `container_list_item.html`, `item_list_item.html`, `tag_list_item.html`
- Newly added element gets a CSS class for flash animation (`@keyframes flash`)
- Errors shown via toast (same mechanism as existing drag-and-drop errors)

## 4. Multi-Select & Bulk Operations

### Checkboxes
- Each `<li>` for containers and items gets a checkbox: `<input type="checkbox" class="bulk-select" data-id="..." data-type="container|item">`
- Hidden by default. Shown when user clicks a "Select" toggle button, or on mobile via long-press on an element
- Selection state managed in JS (`Set` of `{id, type}` objects in `ui-lite.js`)

### Action Bar
- Sticky bar at bottom of screen, appears when `selection.size > 0`
- Content: `"Selected: N items"` + buttons: "Move to...", "Tag...", "Delete selected", "Deselect all"
- Disappears when selection is cleared

### Bulk Move — Move Picker Dialog
- Click "Move to..." opens a native `<dialog>` (modal)
- Inside: search input at top + container tree below
- Tree loaded lazily via HTMX: `GET /ui/partials/tree?parent_id=` returns children of a container as `<ul>` — click arrow to expand a branch
- Search: `GET /ui/partials/tree/search?q=` returns flat list of matching containers with full breadcrumb path
- Click a container → "Move here" button activates → submit
- Endpoint: `POST /ui/actions/bulk/move` — JS sends JSON body (intentional exception to form-encoded convention, same as existing `HandleItemPrint` and `HandleTemplateSave` which already use JSON)
- Body: `{"ids": [{"id": "...", "type": "container|item"}, ...], "target_container_id": "..."}`
- After success: HTMX reloads current container list, selection cleared

### Bulk Delete
- Click "Delete selected" → confirmation `<dialog>`: "Delete N elements? This cannot be undone."
- `POST /ui/actions/bulk/delete` — JSON body: `{"ids": [{"id": "...", "type": "container|item"}, ...]}`
- Server: deletes items directly; containers only if empty (same constraint as single delete)
- Partial failure: response is HTTP 200 with JSON body `{"deleted": ["id1", ...], "failed": [{"id": "...", "reason": "..."}]}`. JS reads the response, removes successfully deleted elements from DOM, shows toast for failures. This avoids the HTMX OOB-on-non-2xx problem (HTMX ignores OOB swaps on non-2xx responses).
- Full success: all elements removed from DOM, action bar hides

### Bulk Tagging
- Click "Tag..." opens tag picker dialog (same component pattern as move picker, but for tags)
- `POST /ui/actions/bulk/tags` — JSON body: `{"ids": [{"id": "...", "type": "container|item"}, ...], "tag_id": "..."}`

### Note on JSON in UI Handlers
Bulk UI endpoints use `json.NewDecoder(r.Body)` instead of `r.FormValue` because array-of-objects payloads have no clean form-encoded representation. This is an intentional exception, consistent with existing handlers that already use JSON (`HandleItemPrint`, `HandleTemplateSave`).

### Multi Drag-and-Drop
- Extension of existing code in `ui-lite.js`
- On `dragstart`: if dragged element is in `selection`, drag the entire selection. Visual feedback: badge with count on cursor (via `e.dataTransfer.setDragImage` with a dynamically created element)
- On `drop`: instead of single move fetch, send `POST /ui/actions/bulk/move`
- If dragged element is NOT in selection — single drag behavior as before

## 5. Tags — UI

### Tag Management Page
- New page: `GET /ui/tags` — tree view of tags (analogous to containers)
- Quick entry for tags: same pattern as containers (inline input, `beforeend`, reset)
- CRUD: create, rename, delete (leaves only), move in hierarchy (drag-and-drop)

### Tag Assignment on Items/Containers
- On container view and item detail: "Tags" section with assigned tags as badge/chip elements
- Click "+" opens a `<dialog>` with mini tag-tree + search (reusable component — same pattern as move picker)
- Click a tag assigns it → `POST /ui/actions/items/{id}/tags` with `tag_id` → server returns OOB swap of updated tag list
- Click "x" on badge removes tag → `DELETE /ui/actions/items/{id}/tags/{tag_id}`
- Same for containers: `/ui/actions/containers/{id}/tags`

### Tag Filtering
- On container view: filter bar above list with tag dropdown/autocomplete
- Filter `?tag=ID` on `GET /ui/containers/{id}` — server returns only items/containers having that tag or its descendant (upward inheritance)
- HTMX: filter changes `hx-get` parameters and reloads list
- Active filters visible as badges with "x" to remove

### Tag Inheritance
- When a tag "Sensors" (child of "Electronics") is assigned to an item, filtering by "Electronics" also returns that item
- Implementation: `TagDescendants(id)` returns all descendant tag IDs; filtering checks if item's `TagIDs` intersects with `{tagID} ∪ TagDescendants(tagID)`

## 6. Global Search

### UI
- Search input in the page header (always visible)
- Typing triggers `GET /ui/search?q=` (debounced, HTMX) → results replace `#content`
- Results grouped in sections: Containers / Items / Tags
- Each result shows name + breadcrumb path (parent chain)
- Click on result → navigates to that object

### Server
- `GET /ui/search?q=` — HTMX fragment with grouped results
- `GET /api/search?q=` — JSON with all matches
- Store methods: `SearchContainers(q)`, `SearchItems(q)`, `SearchTags(q)` — case-insensitive substring match on name
- No indexing needed at this data scale

## 7. New API Endpoints

### Tags
- `GET /api/tags` — all tags, or filtered by `?parent_id=`
- `POST /api/tags` — create tag
- `GET /api/tags/{id}` — single tag
- `PUT /api/tags/{id}` — update tag
- `DELETE /api/tags/{id}` — delete tag (must be leaf)
- `PATCH /api/tags/{id}/move` — move in hierarchy (`{"parent_id": "..."}`)
- `GET /api/tags/{id}/descendants` — recursive descendants

### Tag Assignment
- `POST /api/items/{id}/tags` — assign tag to item (`{"tag_id": "..."}`)
- `DELETE /api/items/{id}/tags/{tag_id}` — remove tag from item
- `POST /api/containers/{id}/tags` — assign tag to container
- `DELETE /api/containers/{id}/tags/{tag_id}` — remove tag from container

### Bulk Operations
- `POST /api/bulk/move` — `{"ids": [{"id": "...", "type": "container|item"}, ...], "target_container_id": "..."}`
- `POST /api/bulk/delete` — `{"ids": [{"id": "...", "type": "container|item"}, ...]}`
- `POST /api/bulk/tags` — `{"ids": [{"id": "...", "type": "container|item"}, ...], "tag_id": "..."}`

### Search
- `GET /api/search?q=` — returns `{"containers": [...], "items": [...], "tags": [...]}`

## 8. New UI Endpoints

### Tags
- `GET /ui/tags` — tag tree page
- `POST /ui/actions/tags` — create tag
- `PUT /ui/actions/tags/{id}` — update tag
- `DELETE /ui/actions/tags/{id}` — delete tag
- `POST /ui/actions/tags/{id}/move` — move tag

### Tag Assignment
- `POST /ui/actions/items/{id}/tags` — assign tag
- `DELETE /ui/actions/items/{id}/tags/{tag_id}` — remove tag
- `POST /ui/actions/containers/{id}/tags` — assign tag
- `DELETE /ui/actions/containers/{id}/tags/{tag_id}` — remove tag

### Partials (HTMX fragments)
- `GET /ui/partials/tree?parent_id=` — container tree children (for move picker)
- `GET /ui/partials/tree/search?q=` — container search results
- `GET /ui/partials/tag-tree?parent_id=` — tag tree children (for tag picker)
- `GET /ui/partials/tag-tree/search?q=` — tag search results

### Bulk Operations
- `POST /ui/actions/bulk/move`
- `POST /ui/actions/bulk/delete`
- `POST /ui/actions/bulk/tags`

### Search
- `GET /ui/search?q=`

## 9. New Templates

### Full-page templates (register in `templateFiles` map in `ui.NewServer`)
- `tags.html` — tag tree page (analogous to `containers.html`), wrapper: `tags`
- `search.html` — global search results page, wrapper: `search`

### Partial templates (register in `sharedFiles` in `ui.NewServer` for cross-template use)
- `partials/container_list_item.html` — single container `<li>` (for quick entry response)
- `partials/item_list_item.html` — single item `<li>` (for quick entry response)
- `partials/tag_list_item.html` — single tag `<li>` (for quick entry response)
- `partials/tree_picker.html` — reusable tree picker dialog (used for move and tag assignment)

### Template registration
Each new full-page template must be added to the `templateFiles` map in `ui.NewServer` with its wrapper block name. Partials must be added to `sharedFiles` so they are parsed into every template's clone (same pattern as `partials/breadcrumb.html`). Missing registration causes a `template.Must` panic at startup.

## 10. JS Changes (`ui-lite.js`)

- **Selection module**: `Set`-based selection state, checkbox event handlers, selection mode toggle
- **Action bar**: render/hide based on selection state, button event handlers
- **Move picker**: dialog open/close, tree navigation clicks, search input debounce
- **Tag picker**: same component pattern as move picker
- **Multi drag-and-drop**: extend existing drag handlers to check selection, create composite drag image, send bulk move on drop
- **Search**: debounced input in header, HTMX trigger
- **Flash animation**: CSS class added to newly inserted elements

## 11. Naming Disambiguation

The existing `Template` model has a `Tags []string` field — these are **free-text labels** for the label template designer (e.g., "60x40", "barcode"). The new `Tag` model is a **separate entity** with UUIDs and hierarchy, used for inventory categorization. These two tag systems are unrelated:
- `Template.Tags` — plain strings, template filtering in the designer
- `store.Tag` / `Item.TagIDs` / `Container.TagIDs` — UUID-based inventory tags

Store methods for inventory tags (`CreateTag`, `DeleteTag`, etc.) do not conflict with template-related code since templates have no tag CRUD — they just store a flat string slice.

## 12. Out of Scope

- SQL migration — staying with JSON store + migration system
- Tag colors/icons — possible future enhancement
- Bulk print from selection — existing "Print all items" covers the container-level case
- Drag-and-drop on mobile (touch events) — existing limitation, unchanged
- Undo/redo for bulk operations
