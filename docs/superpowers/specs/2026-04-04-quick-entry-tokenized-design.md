# Quick Entry with Tokenized Input

## Summary

Replace the two separate quick-entry forms (items and containers) on the container page with a single tokenized input using `contenteditable`. The input supports `@container` and `#tag` token triggers with autocomplete dropdowns, inline quantity via `x5` syntax, and a Tab toggle between item and container creation modes. The container page list is unified: containers on top, items below, separated visually.

## Component: Tokenized Quick Entry

### Input

A `div[contenteditable]` containing:

- **Token spans** (`<span contenteditable="false">`) for `@container` and `#tag` selections
- **Plain text** for the item/container name and optional `x5` quantity
- A **type toggle button** (left of the input) switchable via Tab

### Token types

| Token | Trigger | Max count | Appearance | Removal |
|-------|---------|-----------|------------|---------|
| `@container` | Type `@` | 1 | Blue chip with container icon + name + `×` | Click `×` or Backspace → reverts to `@inbox` (default from settings) |
| `#tag` | Type `#` | Unlimited | Green chip with tag name (full subtag path) + `×` | Click `×` or Backspace → removes token |

### Pre-filling

- **Inside a container page**: `@container` token is pre-filled with the current container (from server-rendered `data-` attributes).
- **No container context** (e.g., root inventory page): `@inbox` token from settings default container.
- Pre-filled `@container` is deletable — user can replace it with another.

### Keyboard behavior

| Key | Action |
|-----|--------|
| Tab | Toggle type: item (🏷) ↔ container (📦). Changes placeholder text and disables qty parsing for containers. |
| Enter | Submit. Parse input, create item/container, reset form, restore pre-filled `@container`, focus input. |
| Escape | Close active dropdown without selecting. |
| Backspace (at token boundary) | Remove entire token. |
| Arrow Up/Down | Navigate dropdown options. |

### Quantity parsing

Plain text is scanned for `x<number>` pattern (e.g., `x5`, `x12`). Extracted as quantity, removed from name. Default: 1. Only applies when creating items (ignored for containers).

## Autocomplete dropdowns

### Container autocomplete (`@`)

- Triggered when user types `@`.
- Text after `@` filters the list (substring match, case-insensitive).
- **Flat list** of all containers with parent path as subtitle (e.g., `📦 Szuflada A` with `Warsztat / Półka 1` in muted text).
- Arrow keys to navigate, Enter to select, Escape to dismiss.
- Selecting replaces any existing `@container` token (max 1).

**Data source**: New or extended endpoint `GET /api/containers/flat` returning all containers with their ancestor path.

### Tag autocomplete (`#`)

- Triggered when user types `#`.
- Text after `#` filters the list.
- **Flat list** with full subtag path (e.g., `#metal / ferrous`). Same pattern as existing `TagAutocomplete`.
- Reuses existing `tag-autocomplete.js` logic and styling.
- Multiple `#tag` tokens allowed.

## Unified container page list

Replace the two separate lists (sub-containers, items) with one unified list:

- **Containers section** at top (with count badge, each showing its icon).
- **Visual separator** (thin line).
- **Items section** below (with quantity, tags displayed as small chips).
- Each entry shows its own icon (from `icon` field — Phosphor icon).

HTMX behavior: after quick-entry submit, the new item/container is appended to the correct section via `hx-swap="beforeend"` targeting the appropriate section ID.

## API changes

### `POST /items` — extend

Accept optional `tag_ids[]` parameter. When provided, assign tags to the newly created item in the same request (instead of requiring separate `POST /items/{id}/tags` calls).

### `POST /containers` — extend

Accept optional `tag_ids[]` and ensure `parent_id` is handled. Same tag assignment behavior as items.

### `GET /api/containers/flat` — new

Return a flat list of all containers with computed `path` field (ancestor names joined by ` / `). Used by the container autocomplete dropdown.

Response shape:
```json
[
  {
    "id": "uuid",
    "name": "Szuflada A",
    "icon": "archive-box",
    "path": "Warsztat / Półka 1"
  }
]
```

## File changes

| File | Change |
|------|--------|
| `static/js/inventory/quick-entry-tokenized.js` | New: contenteditable manager, token lifecycle, parsing, submit |
| `static/js/inventory/container-autocomplete.js` | New: container AC (flat list + path), analogous to `tag-autocomplete.js` |
| `static/js/tags/tag-autocomplete.js` | No change — reused by tokenized input |
| `static/css/inventory/quick-entry-tokenized.css` | New: token chips, contenteditable focus, dropdown positioning |
| `templates/pages/inventory/containers.html` | Rewrite quick-entry section: one tokenized input, unified list (containers + items) |
| `templates/partials/inventory/container_list_item.html` | Update: show icon, adapt to unified list layout |
| `templates/partials/inventory/item_list_item.html` | Update: show icon + tag chips inline, adapt to unified list layout |
| `handler/items.go` | Extend `Create()`: accept and process `tag_ids[]` |
| `handler/containers.go` | Extend `Create()`: accept and process `tag_ids[]` |
| `handler/containers.go` | New handler: `FlatList()` for `/api/containers/flat` |
| `service/inventory.go` | Extend `CreateItem`/`CreateContainer`: accept tag IDs, assign after creation |
| `static/js/inventory/quick-entry.js` | Remove (replaced by tokenized version) |
| `static/css/inventory/quick-entry.css` | Remove or refactor (replaced by tokenized CSS) |

## Error handling

- Empty name on submit → no-op (don't create).
- No `@container` token → use default container from settings.
- Invalid `x` pattern (e.g., `x0`, `xabc`) → ignore, treat as part of name.
- Container/tag not found by ID on submit → show inline error message, don't reset form.

## Testing

- **Go unit tests**: Extended `POST /items` and `POST /containers` with `tag_ids[]`, new `/api/containers/flat` endpoint.
- **E2E (Playwright)**: Token insertion via `@`/`#`, dropdown navigation, Tab toggle, Enter submit, Backspace token removal, pre-filled container behavior, unified list rendering.
