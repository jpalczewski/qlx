# Tag UI Improvements Design

**Date:** 2026-03-22
**Status:** Approved
**Approach:** A — shared `TagAutocomplete` component, three mounting modes

## Problem

Backend tag system is fully implemented (CRUD, assignment, tree traversal, bulk ops) but the UI is incomplete:
- The `+` button on tag chips has no click handler — clicking does nothing
- Tag chips render only on items, not containers
- No way to add tags to individual objects without bulk selection
- No tag detail page showing all tagged objects and statistics
- Tag chip names are not clickable/navigable

## Requirements

1. **Quick-entry `#` trigger** — typing `#` in quick-entry name field opens autocomplete from existing tags + "Create «X»?" option for new tags
2. **Dedicated tag field in edit/create forms** — autocomplete input, not in quick-entry
3. **`+` button on tag chips** — transforms into inline autocomplete input
4. **Clicking tag chip name** — navigates to tag detail view
5. **Tag chips on containers** — currently only on items, add to containers too
6. **Tag detail page** `/ui/tags/{id}` — lists all items/containers with that tag + basic statistics

## Design

### Component: `TagAutocomplete`

**File:** `internal/embedded/static/js/tags/tag-autocomplete.js`

Single vanilla JS component, zero dependencies. Reused in three contexts.

**API:**
```js
qlx.TagAutocomplete({
  anchor: HTMLElement,        // element to position dropdown against
  mode: "inline" | "field" | "hash",
  onSelect: function(tag) {}, // callback after tag selected/created
  onCancel: function() {}     // callback on Escape/blur
})
```

**Behavior:**
- Fetches `GET /api/tags` on first open, caches in memory (invalidated after creating, deleting, or renaming a tag)
- Client-side filtering by `tag.Name` (case-insensitive, substring match), debounced 150ms
- Dropdown below anchor, max 8 results, scrollable. Flips above if insufficient viewport space below.
- Last option: "Create «typed text»?" when no exact match — POST `/api/tags` with default color/icon, then `onSelect` callback. On error: show inline error message in dropdown.
- Keyboard: arrows navigate, Enter selects, Escape closes
- Each result shows color dot + icon + tag name
- ARIA: dropdown has `role="listbox"`, items have `role="option"`, input has `aria-activedescendant`
- Only one tag per quick-entry submission (first `#` match wins)

### Mounting Modes

| Mode | Trigger | Where | After selection |
|------|---------|-------|-----------------|
| `hash` | `#` in name input | Quick-entry | Strips `#text` from value, stores tag ID; after form submit creates object then POST assigns tag |
| `field` | Focus on dedicated input | Edit/create form | POST assign immediately (object exists), adds chip |
| `inline` | Click `+` button | Tag chips | Replaces `+` with input, POST assign, HTMX refresh chips |

### Quick-entry Integration (`hash` mode)

- Listener on `input` event in `.quick-entry input[name=name]`
- Detects `#` — opens autocomplete positioned below input
- After tag selection: strips `#fragment` from value, stores tag ID in a **JS closure variable** (not DOM attribute, survives form reset)
- **Which forms:** Both item and container quick-entry forms. The JS listener reads the form's hidden input to determine object type: if `input[name=container_id]` exists → item form (POST to `/ui/actions/items/{id}/tags`), if `input[name=parent_id]` → container form (POST to `/ui/actions/containers/{id}/tags`)
- **ID extraction mechanism:** Backend quick-entry handlers return an `<li>` fragment with `data-id="{newID}"`. JS registers a **one-shot, target-scoped** `htmx:afterSwap` listener on the form's `hx-target` element (e.g., `#item-list`) before submit. The listener fires once after the new `<li>` is swapped in, reads `data-id` from the last `<li>[data-id]` (selector: `li[data-id]:last-of-type` to skip empty-state placeholder), POSTs tag assignment, then removes itself. The closure variable is cleared after use.
- Timing: the `htmx:afterSwap` listener fires after DOM swap but before `hx-on::after-request` resets the form, so the tag ID in the closure is still available
- If `#foo #bar` is typed, only the first `#foo` is processed. `#bar` remains as literal text in the name.

### Edit/Create Form Integration (`field` mode)

- New block below description field: label "Tags" + autocomplete input + rendered chips below
- Chips have `x` to unassign (HTMX DELETE, same as existing tag chips)
- Object already exists → POST assign immediately after selection

### Inline `+` Button Integration (`inline` mode)

- Click on `.tag-add` replaces button with `<input>` of same width
- Autocomplete dropdown below input
- After selection: POST assign, HTMX refreshes entire `#tag-chips-{id}`
- Escape / blur: restores `+` button

### Tag Chips on Containers

Add `{{ if .TagIDs }}{{ template "tag-chips" ... }}{{ end }}` in `containers.html` for container list items, with `ObjectType` = `"container"`.

### Tag Chip Navigation

Tag name in chip becomes `<a href="/ui/tags/{id}" hx-get="/ui/tags/{id}" hx-target="#content">` instead of `<span>`. The link wraps **only the name text**, not the entire chip — the `x` remove button remains a separate sibling element outside the link to avoid click propagation conflicts. Consistent with existing SPA-like navigation pattern.

### Tag Detail Page: `/ui/tags/{id}`

**New handler:** `HandleTagView` in `ui/handlers_tags.go`

**New store method:** `ContainersByTag(tagID)` — analogous to existing `ItemsByTag`, searches containers with given tag + descendant tags (BFS). UI-only (no new API endpoint needed). Must have unit tests analogous to `TestItemsByTag`.

**Route:** `GET /ui/tags/{id}` — Go 1.22+ `http.ServeMux` distinguishes exact `GET /ui/tags` from wildcard `GET /ui/tags/{id}`. Note: trailing slash `/ui/tags/` is not matched by either pattern.

**View model:**
```go
type TagDetailData struct {
    Tag        store.Tag
    Path       []store.Tag
    Items      []store.Item
    Containers []store.Container
    Stats      TagStats
    Children   []store.Tag
}

type TagStats struct {
    ItemCount      int
    ContainerCount int
    TotalQuantity  int
}
```

**Template:** `pages/tags/tag_detail.html`

Layout:
- Breadcrumb: Tags / Parent / TagName
- Statistics card: item count, container count, total quantity (sum of `Quantity` across all items returned by `ItemsByTag`, which includes descendant tags)
- Containers section: list with links
- Items section: list with quantity, tag chips
- Child tags section: links to `/ui/tags/{child_id}`

**Empty state:** When a tag has zero items and zero containers, show a message: "No items or containers tagged with «TagName»."

## Files to Create

| File | Purpose |
|------|---------|
| `internal/embedded/static/js/tags/tag-autocomplete.js` | Shared autocomplete component |
| `internal/embedded/static/css/tags/tag-autocomplete.css` | Autocomplete dropdown styles |
| `internal/embedded/templates/pages/tags/tag_detail.html` | Tag detail page template |
| `e2e/tests/tag-autocomplete.spec.ts` | E2E tests for autocomplete and tag detail page |

## Files to Modify

| File | Change |
|------|--------|
| `internal/store/tags.go` | Add `ContainersByTag(tagID string) []Container` method (on `*Store`, same pattern as `ItemsByTag` — not in interface) |
| `internal/store/tags_test.go` | Add `TestContainersByTag` (analogous to `TestItemsByTag`) |
| `internal/ui/handlers_tags.go` | Add `HandleTagView` (accesses `store.ContainersByTag` and `store.ItemsByTag` directly), `TagDetailData` and `TagStats` view models |
| `internal/ui/server.go` | Register route `GET /ui/tags/{id}` |
| `internal/embedded/templates/partials/tags/tag_chips.html` | Make tag name a link with `hx-get`/`hx-target="#content"`, keep `x` button |
| `internal/embedded/templates/pages/inventory/containers.html` | Add tag chips to container list items |
| `internal/embedded/templates/layouts/base.html` | Include `tag-autocomplete.js` and `.css` |
| `internal/ui/handlers.go` | Wire tag field in edit/create form handlers |

## Out of Scope

- Tag filtering/search on inventory pages (future feature)
- Drag-and-drop tag assignment
- Tag merge/rename from UI
- Bulk tag removal
