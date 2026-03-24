# Container Export Modal — Design Spec

**Issue:** #20 — Container export (CSV/JSON)
**Date:** 2026-03-24

## Summary

Export container contents (or full inventory) as CSV, JSON, or Markdown via a two-step modal dialog. Supports per-container and full-inventory export, recursive sub-container inclusion, and clipboard copy.

## API Design

### Unified Endpoint

```
GET /export?format=csv|json|md&container={id}&recursive=true&md_style=table|document|both&download=true
```

**Parameters:**

| Param       | Required | Default | Description                                                  |
|-------------|----------|---------|--------------------------------------------------------------|
| `format`    | yes      | —       | `csv`, `json`, `md`                                          |
| `container` | no       | —       | Container UUID. Omitted = full inventory.                    |
| `recursive` | no       | `false` | Include nested sub-containers. Only meaningful with container.|
| `md_style`  | no       | `table` | `table`, `document`, `both`. Only used when format=md.       |
| `download`  | no       | `false` | When true, sets `Content-Disposition: attachment`.           |

**Response Content-Types:**
- CSV: `text/csv; charset=utf-8`
- JSON: `application/json`
- Markdown: `text/markdown; charset=utf-8`

**Filename generation:** `qlx-export.{ext}` for full inventory, `qlx-{sanitized-container-name}-export.{ext}` for per-container.

**Replaces:** `/export/json` and `/export/csv` (current routes removed).

**Breaking change:** The CSV column format changes from the old `item_id, item_name, item_description, container_path, created_at` to the new format described below. This is acceptable — no external consumers depend on the old format.

### Full-inventory export behavior

When `container` is omitted, the export covers all items across all containers:
- **CSV/Markdown table:** flat list of all items with `container_path` column.
- **JSON:** `{ "containers": [ { "id": ..., "name": ..., "items": [...], "children": [...] } ] }` — array of root containers with nested tree structure. All items grouped under their container.
- **Markdown document:** grouped by root containers with nested sub-headers.

The `recursive` param is ignored for full-inventory export (it's inherently "all containers").

### Empty state

When the export scope contains zero items:
- API returns valid but empty output (CSV with headers only, JSON with empty arrays, Markdown with a "No items" note).
- Modal preview shows the empty output — no special "nothing to export" message needed. The user sees what they'd get.

## Export Formats

### CSV

Flat rows always. Columns: `item_id, item_name, quantity, tags, description, container_path, created_at`.

- `item_id` retained for traceability.
- `tags` column: semicolon-delimited tag names (not IDs). Example: `Electronics; Fragile`. Semicolons chosen to avoid CSV comma conflicts.
- `container_path` shows full path with ` > ` separator (e.g. `Storage > Box A > Drawer 1`).

### JSON

- **Flat** (non-recursive, single container): array of item objects with `container_path` field.
- **Recursive / full-inventory**: grouped nested structure:
  ```json
  {
    "containers": [
      {
        "id": "...", "name": "...",
        "items": [{ "id": "...", "name": "...", "quantity": 1, "tags": ["Electronics"], ... }],
        "children": [
          { "id": "...", "name": "...", "items": [...], "children": [...] }
        ]
      }
    ]
  }
  ```
- Recursive JSON requires building the full tree in memory (cannot be streamed). This is fine — inventory data is small.

### Markdown

Three styles controlled by `md_style`:

- **`table`** — pipe-delimited Markdown table, same columns as CSV. Always flat.
- **`document`** — headers per container (`## Container Name`), bullet list of items with details. Grouped when recursive or full-inventory.
- **`both`** — table section first, then document section below.

## SQLite Query Strategy

No caching layer. Efficient queries instead.

### Recursive CTE for sub-container collection

```sql
WITH RECURSIVE subtree(id) AS (
    SELECT id FROM containers WHERE id = ?
    UNION ALL
    SELECT c.id FROM containers c
    JOIN subtree s ON c.parent_id = s.id
)
```

### Items with tags in single query (GROUP_CONCAT)

```sql
SELECT i.id, i.name, i.description, i.quantity, i.container_id,
       i.created_at, GROUP_CONCAT(t.name, ';') as tag_names
FROM items i
LEFT JOIN item_tags it ON it.item_id = i.id
LEFT JOIN tags t ON t.id = it.tag_id
WHERE i.container_id IN (SELECT id FROM subtree)
GROUP BY i.id
ORDER BY i.name
```

Note: joins through to `tags` table to get tag names directly, not just IDs.

### Container path resolution

Done in Go, not SQL. After fetching all containers, build a `map[string]Container` and walk parent pointers to assemble paths. This avoids `GROUP_CONCAT` ordering issues in SQLite and is simpler for both per-container and full-inventory cases.

```go
func buildContainerPaths(containers []Container) map[string]string {
    byID := make(map[string]Container, len(containers))
    for _, c := range containers { byID[c.ID] = c }
    paths := make(map[string]string, len(containers))
    for _, c := range containers {
        var parts []string
        cur := c
        for {
            parts = append([]string{cur.Name}, parts...)
            if cur.ParentID == "" { break }
            cur = byID[cur.ParentID]
        }
        paths[c.ID] = strings.Join(parts, " > ")
    }
    return paths
}
```

### Streaming output

CSV and Markdown write directly to `io.Writer` — no intermediate string allocation. JSON recursive format is built in memory then marshaled (inventory data is small).

### ExportStore interface changes

**New methods:**
- `ExportItems(containerID string, recursive bool) ([]ExportItem, error)` — items with tag names and container ID.
- `ExportContainerTree(containerID string) ([]Container, error)` — flat list of container + all descendants.

**Removed methods:** `ExportData() (map[string]*Container, map[string]*Item)` — replaced by the new methods above.

**Retained methods:** `AllItems() []Item`, `AllContainers() []Container` — used by full-inventory export and potentially elsewhere.

### ExportItem type

```go
type ExportItem struct {
    ID          string
    Name        string
    Description string
    Quantity    int
    ContainerID string
    TagNames    []string  // resolved tag names, not IDs
    CreatedAt   time.Time
}
```

Container path is resolved at the service layer (not in the store) using `buildContainerPaths()`.

## Modal Component

### Two-step dialog

**Step 1 — Options:**
- Format radio buttons: CSV / JSON / Markdown
- When Markdown selected: sub-radio for style (Table / Document / Both)
- When per-container context: "Include sub-containers" checkbox
- "Preview" button (disabled until format selected)

**Step 2 — Preview:**
- Scrollable `<pre>` block (monospace, max-height ~50vh)
- Filename shown above preview
- Two action buttons: "Download" / "Copy to clipboard"
- "Back" button to return to step 1
- Copy button shows brief "Copied!" feedback

### Architecture

- **JS:** `internal/embedded/static/js/shared/export-dialog.js` — lazy-init factory (same pattern as tree-picker)
- **CSS:** `internal/embedded/static/css/dialogs/export.css` — extends base `dialog.css`
- **Public API:**
  - `qlx.openExportDialog({ containerId, containerName })` — per-container
  - `qlx.openExportDialog({})` — full inventory

### Script loading

`export-dialog.js` is included in the base layout (`layout.html`) so it's available on all pages. It creates no DOM until `qlx.openExportDialog()` is called (lazy init), so zero cost when not used.

### Implementation details

- Preview fetched via `fetch()`, response text cached in JS variable
- Download: create Blob URL from cached text, trigger via `<a download>` click
- Copy: `navigator.clipboard.writeText()` from cached text
- Safe DOM only — `createElement`, `textContent`, `appendChild` (no `innerHTML`)
- Deferred i18n: title/labels resolved at open time via `qlx.t()` (already exists on the `qlx` namespace, used by tree-picker and other components)
- Responsive: full viewport on `max-width: 600px`

## UI Integration

### Container detail view

No "more actions" dropdown currently exists on the container detail page. **Create one:** a dropdown button (styled consistently with existing buttons) placed next to the Edit button in the container header. Initially contains only "Export". This dropdown pattern can later host other actions (e.g. move, duplicate).

Calls `qlx.openExportDialog({ containerId, containerName })`.

### Settings page

Current two `<a download>` links replaced with a single "Export" button that opens `qlx.openExportDialog({})`.

## Route & Handler Changes

- **New:** `GET /export` — unified endpoint, single `ExportHandler` method
- **Removed:** `GET /export/json`, `GET /export/csv`
- Handler constructor takes only `ExportService` (and `InventoryService` for container existence checks). Container path resolution moves to `ExportService`, so the handler is thinner.
- Handler validates params (400 bad format, 404 missing container), delegates to `ExportService`, sets headers based on `download` param.

## Testing

### Unit tests (Go)

- `ExportService` formatting: table-driven, one per format/style/recursive combination
- Input via `:memory:` SQLite with seed data
- Assert exact string output
- Empty-state tests: container with zero items

### SQLite query tests

- Recursive CTE: 3-level container tree, verify all descendants
- GROUP_CONCAT tag name aggregation: items with 0, 1, multiple tags
- Container path (Go helper): root item, nested 3 levels deep

### Handler tests

- `httptest` for `/export` endpoint
- Query param validation (400, 404)
- Content-Type and Content-Disposition headers
- `download=true` triggers attachment disposition

### E2E tests (Playwright)

- Container detail: more actions dropdown > Export > select format > Preview > Download + Copy
- Settings page: Export button > full inventory modal
- All three formats verified
- Recursive toggle tested
- Empty container export
