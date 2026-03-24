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

## Export Formats

### CSV

Flat rows always. Columns: `item_name, quantity, tags, description, container_path, created_at`.

When recursive, `container_path` shows full path (e.g. `Storage > Box A > Drawer 1`).

### JSON

- **Flat** (non-recursive): array of item objects with `container_path` field.
- **Recursive**: grouped nested structure:
  ```json
  {
    "container": {
      "id": "...", "name": "...",
      "items": [...],
      "children": [
        { "id": "...", "name": "...", "items": [...], "children": [...] }
      ]
    }
  }
  ```

### Markdown

Three styles controlled by `md_style`:

- **`table`** — pipe-delimited Markdown table, same columns as CSV. Always flat.
- **`document`** — headers per container (`## Container Name`), bullet list of items with details. Grouped when recursive.
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
       i.created_at, GROUP_CONCAT(it.tag_id, ',') as tag_ids
FROM items i
LEFT JOIN item_tags it ON it.item_id = i.id
WHERE i.container_id IN (SELECT id FROM subtree)
GROUP BY i.id
ORDER BY i.name
```

### Container path resolution

```sql
WITH RECURSIVE path(id, name, parent_id, depth) AS (
    SELECT id, name, parent_id, 0 FROM containers WHERE id = ?
    UNION ALL
    SELECT c.id, c.name, c.parent_id, p.depth + 1
    FROM containers c JOIN path p ON c.id = p.parent_id
)
SELECT GROUP_CONCAT(name, ' -> ') FROM (SELECT name FROM path ORDER BY depth DESC)
```

For full-inventory CSV, container paths are precomputed in a single Go map walk.

### Streaming output

CSV and Markdown write directly to `io.Writer` — no intermediate string allocation.

### New ExportStore interface methods

- `ExportItems(containerID string, recursive bool) ([]ExportItem, error)` — items with tag IDs and container path pre-resolved.
- `ExportContainerTree(containerID string) ([]Container, error)` — flat list of container + all descendants.

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

### Implementation details

- Preview fetched via `fetch()`, response text cached in JS variable
- Download: create Blob URL from cached text, trigger via `<a download>` click
- Copy: `navigator.clipboard.writeText()` from cached text
- Safe DOM only — `createElement`, `textContent`, `appendChild` (no `innerHTML`)
- Deferred i18n: title/labels resolved at open time via `qlx.t()`
- Responsive: full viewport on `max-width: 600px`

## UI Integration

### Container detail view

"Export" option added to the existing "more actions" dropdown menu. Calls `qlx.openExportDialog({ containerId, containerName })`.

### Settings page

Current two `<a download>` links replaced with a single "Export" button that opens `qlx.openExportDialog({})`.

## Route & Handler Changes

- **New:** `GET /export` — unified endpoint, single `ExportHandler` method
- **Removed:** `GET /export/json`, `GET /export/csv`
- Handler validates params (400 bad format, 404 missing container), delegates to `ExportService`, sets headers based on `download` param.

## Testing

### Unit tests (Go)

- `ExportService` formatting: table-driven, one per format/style/recursive combination
- Input via `:memory:` SQLite with seed data
- Assert exact string output

### SQLite query tests

- Recursive CTE: 3-level container tree, verify all descendants
- GROUP_CONCAT: items with 0, 1, multiple tags
- Container path: root item, nested 3 levels deep

### Handler tests

- `httptest` for `/export` endpoint
- Query param validation (400, 404)
- Content-Type and Content-Disposition headers
- `download=true` triggers attachment disposition

### E2E tests (Playwright)

- Container detail: more actions > Export > select format > Preview > Download + Copy
- Settings page: Export button > full inventory modal
- All three formats verified
- Recursive toggle tested
