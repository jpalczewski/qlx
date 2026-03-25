# Container Export Modal — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a two-step export modal supporting CSV/JSON/Markdown with per-container, recursive, and full-inventory export.

**Architecture:** New `ExportItem` model + efficient SQLite queries (recursive CTE, GROUP_CONCAT). Service layer formats output via `io.Writer`. Unified `/export` endpoint replaces old `/export/json` + `/export/csv`. Frontend: lazy-init `<dialog>` factory with fetch-based preview.

**Tech Stack:** Go 1.26, SQLite (ncruces/go-sqlite3), vanilla JS, native `<dialog>`, Playwright E2E.

**Spec:** `docs/superpowers/specs/2026-03-24-container-export-modal-design.md`

---

## File Structure

### New files
| File | Purpose |
|------|---------|
| `internal/store/export_item.go` | `ExportItem` struct definition |
| `internal/service/export_format.go` | CSV/JSON/Markdown formatters (write to `io.Writer`) |
| `internal/service/export_format_test.go` | Formatter unit tests |
| `internal/embedded/static/js/shared/export-dialog.js` | Modal dialog factory |
| `internal/embedded/static/css/dialogs/export.css` | Modal + dropdown styles |
| `internal/embedded/static/i18n/en/export.json` | English translations |
| `internal/embedded/static/i18n/pl/export.json` | Polish translations |

### Modified files
| File | Change |
|------|--------|
| `internal/store/interfaces.go:78-83` | Add `ExportItems`, `ExportContainerTree` to `ExportStore` |
| `internal/store/sqlite/export.go` | Implement new methods with recursive CTE + GROUP_CONCAT |
| `internal/store/sqlite/export_test.go` | Tests for new query methods |
| `internal/service/export.go` | Add `ExportCSV`, `ExportJSON`, `ExportMarkdown` + path helper |
| `internal/handler/export.go` | Rewrite: unified `/export` endpoint with query params |
| `internal/app/server.go:54` | No change needed (handler constructor stays compatible) |
| `internal/embedded/templates/layouts/base.html` | Add CSS + JS includes |
| `internal/embedded/templates/pages/settings/settings.html:20-26` | Replace export links with modal button |
| `internal/embedded/templates/pages/inventory/containers.html:11-17` | Add "more actions" dropdown with Export |
| `e2e/tests/export.spec.ts` | Rewrite for new endpoint + modal E2E |

---

## Task 1: ExportItem model

**Files:**
- Create: `internal/store/export_item.go`

- [ ] **Step 1: Create ExportItem struct**

```go
// internal/store/export_item.go
package store

import "time"

// ExportItem is a denormalized item for export — includes resolved tag names.
type ExportItem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Quantity    int       `json:"quantity"`
	ContainerID string    `json:"container_id"`
	TagNames    []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/erxyi/Projekty/qlx && go build ./internal/store/...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/store/export_item.go
git commit -m "feat(store): add ExportItem model for denormalized export data"
```

---

## Task 2: ExportStore interface — add new methods

**Files:**
- Modify: `internal/store/interfaces.go:78-83`

- [ ] **Step 1: Add new methods to ExportStore**

In `internal/store/interfaces.go`, replace the `ExportStore` interface (lines 78-83) with:

```go
// ExportStore defines export-related store operations.
type ExportStore interface {
	ExportData() (map[string]*Container, map[string]*Item)
	AllItems() []Item
	AllContainers() []Container
	ExportItems(containerID string, recursive bool) ([]ExportItem, error)
	ExportContainerTree(containerID string) ([]Container, error)
}
```

Keep `ExportData`, `AllItems`, `AllContainers` for now — they're used by existing code. Remove in a follow-up after the handler is rewritten.

- [ ] **Step 2: Verify it compiles (expect failure — methods not implemented yet)**

Run: `cd /Users/erxyi/Projekty/qlx && go build ./internal/store/... 2>&1 | head -5`
Expected: compiles (interface is just a contract). The SQLite store won't satisfy it until Task 3.

- [ ] **Step 3: Commit**

```bash
git add internal/store/interfaces.go
git commit -m "feat(store): extend ExportStore interface with ExportItems and ExportContainerTree"
```

---

## Task 3: SQLite implementation — ExportItems + ExportContainerTree

**Files:**
- Modify: `internal/store/sqlite/export.go`
- Modify: `internal/store/sqlite/export_test.go`

- [ ] **Step 1: Write failing test for ExportContainerTree**

Add to `internal/store/sqlite/export_test.go`:

```go
func TestExportStore_ExportContainerTree(t *testing.T) {
	db := testStore(t)

	root := db.CreateContainer("", "Root", "", "", "")
	child := db.CreateContainer(root.ID, "Child", "", "", "")
	grandchild := db.CreateContainer(child.ID, "Grandchild", "", "", "")
	_ = db.CreateContainer("", "Unrelated", "", "", "")

	tree, err := db.ExportContainerTree(root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 3 {
		t.Fatalf("got %d containers, want 3 (root+child+grandchild)", len(tree))
	}

	ids := make(map[string]bool)
	for _, c := range tree {
		ids[c.ID] = true
	}
	if !ids[root.ID] || !ids[child.ID] || !ids[grandchild.ID] {
		t.Error("tree missing expected containers")
	}
}
```

- [ ] **Step 2: Run test — verify it fails**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/sqlite/ -run TestExportStore_ExportContainerTree -v`
Expected: FAIL (method not implemented).

- [ ] **Step 3: Implement ExportContainerTree**

Add to `internal/store/sqlite/export.go`:

```go
// ExportContainerTree returns a container and all its descendants using a recursive CTE.
func (s *SQLiteStore) ExportContainerTree(containerID string) ([]store.Container, error) {
	rows, err := s.db.Query(`
		WITH RECURSIVE subtree(id) AS (
			SELECT id FROM containers WHERE id = ?
			UNION ALL
			SELECT c.id FROM containers c
			JOIN subtree s ON c.parent_id = s.id
		)
		SELECT id, COALESCE(parent_id, ''), name, description, color, icon, created_at
		FROM containers
		WHERE id IN (SELECT id FROM subtree)
		ORDER BY name`, containerID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var containers []store.Container
	for rows.Next() {
		var c store.Container
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.Description, &c.Color, &c.Icon, &c.CreatedAt); err != nil {
			continue
		}
		containers = append(containers, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range containers {
		containers[i].TagIDs = s.containerTagIDs(containers[i].ID)
	}
	return containers, nil
}
```

- [ ] **Step 4: Run test — verify it passes**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/sqlite/ -run TestExportStore_ExportContainerTree -v`
Expected: PASS.

- [ ] **Step 5: Write failing test for ExportItems (single container, no recursion)**

Add to `internal/store/sqlite/export_test.go`:

```go
func TestExportStore_ExportItems_SingleContainer(t *testing.T) {
	db := testStore(t)

	tag := db.CreateTag("", "Electronics", "", "")
	c := db.CreateContainer("", "Box", "", "", "")
	item := db.CreateItem(c.ID, "Widget", "A small widget", 3, "", "")
	db.AddItemTag(item.ID, tag.ID)

	items, err := db.ExportItems(c.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Name != "Widget" {
		t.Errorf("name = %q, want Widget", items[0].Name)
	}
	if items[0].Quantity != 3 {
		t.Errorf("quantity = %d, want 3", items[0].Quantity)
	}
	if len(items[0].TagNames) != 1 || items[0].TagNames[0] != "Electronics" {
		t.Errorf("tags = %v, want [Electronics]", items[0].TagNames)
	}
}
```

- [ ] **Step 6: Run test — verify it fails**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/sqlite/ -run TestExportStore_ExportItems_SingleContainer -v`
Expected: FAIL.

- [ ] **Step 7: Implement ExportItems**

Add to `internal/store/sqlite/export.go`:

```go
import (
	"strings"

	"github.com/erxyi/qlx/internal/store"
)

// ExportItems returns denormalized items with resolved tag names.
// If containerID is empty, returns all items. If recursive is true, includes items from sub-containers.
func (s *SQLiteStore) ExportItems(containerID string, recursive bool) ([]store.ExportItem, error) {
	var query string
	var args []any

	switch {
	case containerID == "":
		query = `
			SELECT i.id, i.name, i.description, i.quantity, i.container_id,
			       i.created_at, COALESCE(GROUP_CONCAT(t.name, ';'), '') as tag_names
			FROM items i
			LEFT JOIN item_tags it ON it.item_id = i.id
			LEFT JOIN tags t ON t.id = it.tag_id
			GROUP BY i.id
			ORDER BY i.name`
	case recursive:
		query = `
			WITH RECURSIVE subtree(id) AS (
				SELECT id FROM containers WHERE id = ?
				UNION ALL
				SELECT c.id FROM containers c
				JOIN subtree s ON c.parent_id = s.id
			)
			SELECT i.id, i.name, i.description, i.quantity, i.container_id,
			       i.created_at, COALESCE(GROUP_CONCAT(t.name, ';'), '') as tag_names
			FROM items i
			LEFT JOIN item_tags it ON it.item_id = i.id
			LEFT JOIN tags t ON t.id = it.tag_id
			WHERE i.container_id IN (SELECT id FROM subtree)
			GROUP BY i.id
			ORDER BY i.name`
		args = []any{containerID}
	default:
		query = `
			SELECT i.id, i.name, i.description, i.quantity, i.container_id,
			       i.created_at, COALESCE(GROUP_CONCAT(t.name, ';'), '') as tag_names
			FROM items i
			LEFT JOIN item_tags it ON it.item_id = i.id
			LEFT JOIN tags t ON t.id = it.tag_id
			WHERE i.container_id = ?
			GROUP BY i.id
			ORDER BY i.name`
		args = []any{containerID}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []store.ExportItem
	for rows.Next() {
		var item store.ExportItem
		var tagStr string
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Quantity,
			&item.ContainerID, &item.CreatedAt, &tagStr); err != nil {
			continue
		}
		if tagStr != "" {
			item.TagNames = strings.Split(tagStr, ";")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
```

- [ ] **Step 8: Run test — verify it passes**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/sqlite/ -run TestExportStore_ExportItems -v`
Expected: PASS.

- [ ] **Step 9: Write test for ExportItems recursive**

Add to `internal/store/sqlite/export_test.go`:

```go
func TestExportStore_ExportItems_Recursive(t *testing.T) {
	db := testStore(t)

	root := db.CreateContainer("", "Root", "", "", "")
	child := db.CreateContainer(root.ID, "Child", "", "", "")
	db.CreateItem(root.ID, "RootItem", "", 1, "", "")
	db.CreateItem(child.ID, "ChildItem", "", 1, "", "")
	_ = db.CreateContainer("", "Other", "", "", "")
	db.CreateItem(db.CreateContainer("", "Unrelated", "", "", "").ID, "UnrelatedItem", "", 1, "", "")

	items, err := db.ExportItems(root.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	names := map[string]bool{}
	for _, i := range items {
		names[i.Name] = true
	}
	if !names["RootItem"] || !names["ChildItem"] {
		t.Errorf("got names %v, want RootItem and ChildItem", names)
	}
}
```

- [ ] **Step 10: Run test — verify it passes**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/sqlite/ -run TestExportStore_ExportItems_Recursive -v`
Expected: PASS.

- [ ] **Step 11: Write test for ExportItems with no tags**

Add to `internal/store/sqlite/export_test.go`:

```go
func TestExportStore_ExportItems_NoTags(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	db.CreateItem(c.ID, "Plain", "", 1, "", "")

	items, err := db.ExportItems(c.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].TagNames != nil {
		t.Errorf("tags = %v, want nil", items[0].TagNames)
	}
}
```

- [ ] **Step 12: Run test — verify it passes**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/sqlite/ -run TestExportStore_ExportItems_NoTags -v`
Expected: PASS.

- [ ] **Step 13: Verify full test suite**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/sqlite/ -v`
Expected: all tests PASS.

- [ ] **Step 14: Commit**

```bash
git add internal/store/sqlite/export.go internal/store/sqlite/export_test.go
git commit -m "feat(store): implement ExportItems and ExportContainerTree with recursive CTE"
```

---

## Task 4: Export formatters — CSV, JSON, Markdown

**Files:**
- Create: `internal/service/export_format.go`
- Create: `internal/service/export_format_test.go`

- [ ] **Step 1: Write container path helper and define types**

Create `internal/service/export_format.go`:

```go
package service

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/erxyi/qlx/internal/store"
)

// buildContainerPaths builds a map of container ID → full path string.
func buildContainerPaths(containers []store.Container) map[string]string {
	byID := make(map[string]store.Container, len(containers))
	for _, c := range containers {
		byID[c.ID] = c
	}

	paths := make(map[string]string, len(containers))
	for _, c := range containers {
		var parts []string
		cur := c
		for {
			parts = append([]string{cur.Name}, parts...)
			if cur.ParentID == "" {
				break
			}
			parent, ok := byID[cur.ParentID]
			if !ok {
				break
			}
			cur = parent
		}
		paths[c.ID] = strings.Join(parts, " > ")
	}
	return paths
}
```

- [ ] **Step 2: Write failing test for CSV formatter**

Create `internal/service/export_format_test.go`:

```go
package service

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/erxyi/qlx/internal/store"
)

var testTime = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

func testContainers() []store.Container {
	return []store.Container{
		{ID: "c1", Name: "Root", ParentID: ""},
		{ID: "c2", Name: "Child", ParentID: "c1"},
	}
}

func testItems() []store.ExportItem {
	return []store.ExportItem{
		{ID: "i1", Name: "Widget", Description: "A widget", Quantity: 3, ContainerID: "c1", TagNames: []string{"Electronics", "Fragile"}, CreatedAt: testTime},
		{ID: "i2", Name: "Gadget", Description: "", Quantity: 1, ContainerID: "c2", TagNames: nil, CreatedAt: testTime},
	}
}

func TestFormatCSV(t *testing.T) {
	var buf bytes.Buffer
	paths := buildContainerPaths(testContainers())

	err := FormatCSV(&buf, testItems(), paths)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (header + 2 rows)", len(lines))
	}
	if !strings.Contains(lines[0], "item_id") {
		t.Errorf("header missing item_id: %s", lines[0])
	}
	if !strings.Contains(lines[1], "Electronics;Fragile") {
		t.Errorf("row 1 missing tags: %s", lines[1])
	}
	if !strings.Contains(lines[1], "Root") {
		t.Errorf("row 1 missing container path: %s", lines[1])
	}
	if !strings.Contains(lines[2], "Root > Child") {
		t.Errorf("row 2 missing nested path: %s", lines[2])
	}
}
```

- [ ] **Step 3: Run test — verify it fails**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/service/ -run TestFormatCSV -v`
Expected: FAIL (FormatCSV not defined).

- [ ] **Step 4: Implement FormatCSV**

Add to `internal/service/export_format.go`:

```go
// FormatCSV writes items as CSV to w. Paths maps container ID → display path.
func FormatCSV(w io.Writer, items []store.ExportItem, paths map[string]string) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := cw.Write([]string{"item_id", "item_name", "quantity", "tags", "description", "container_path", "created_at"}); err != nil {
		return err
	}

	for _, item := range items {
		tags := strings.Join(item.TagNames, ";")
		if err := cw.Write([]string{
			item.ID,
			item.Name,
			fmt.Sprintf("%d", item.Quantity),
			tags,
			item.Description,
			paths[item.ContainerID],
			item.CreatedAt.Format(time.RFC3339),
		}); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 5: Run test — verify it passes**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/service/ -run TestFormatCSV -v`
Expected: PASS.

- [ ] **Step 6: Write failing test for JSON flat format**

Add to `internal/service/export_format_test.go`:

```go
func TestFormatJSONFlat(t *testing.T) {
	var buf bytes.Buffer
	paths := buildContainerPaths(testContainers())

	err := FormatJSONFlat(&buf, testItems(), paths)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), `"container_path"`) {
		t.Error("missing container_path field")
	}
	if !strings.Contains(buf.String(), `"Widget"`) {
		t.Error("missing item name")
	}
}
```

- [ ] **Step 7: Implement FormatJSONFlat**

Add to `internal/service/export_format.go`:

```go
type jsonFlatItem struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Quantity      int      `json:"quantity"`
	Tags          []string `json:"tags"`
	ContainerPath string   `json:"container_path"`
	CreatedAt     string   `json:"created_at"`
}

// FormatJSONFlat writes items as a flat JSON array with container paths.
func FormatJSONFlat(w io.Writer, items []store.ExportItem, paths map[string]string) error {
	out := make([]jsonFlatItem, len(items))
	for i, item := range items {
		out[i] = jsonFlatItem{
			ID:            item.ID,
			Name:          item.Name,
			Description:   item.Description,
			Quantity:      item.Quantity,
			Tags:          item.TagNames,
			ContainerPath: paths[item.ContainerID],
			CreatedAt:     item.CreatedAt.Format(time.RFC3339),
		}
		if out[i].Tags == nil {
			out[i].Tags = []string{}
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
```

- [ ] **Step 8: Run test — verify it passes**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/service/ -run TestFormatJSONFlat -v`
Expected: PASS.

- [ ] **Step 9: Write failing test for JSON grouped (recursive) format**

Add to `internal/service/export_format_test.go`:

```go
func TestFormatJSONGrouped(t *testing.T) {
	var buf bytes.Buffer

	containers := testContainers()
	items := testItems()

	err := FormatJSONGrouped(&buf, containers, items)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), `"children"`) {
		t.Error("missing children field")
	}
	if !strings.Contains(buf.String(), `"Root"`) {
		t.Error("missing root container")
	}
}
```

- [ ] **Step 10: Implement FormatJSONGrouped**

Add to `internal/service/export_format.go`:

```go
type jsonContainerNode struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Items    []jsonFlatItem      `json:"items"`
	Children []jsonContainerNode `json:"children"`
}

type jsonGroupedExport struct {
	Containers []jsonContainerNode `json:"containers"`
}

// FormatJSONGrouped writes items grouped by container as a nested JSON tree.
func FormatJSONGrouped(w io.Writer, containers []store.Container, items []store.ExportItem) error {
	// Index items by container ID.
	itemsByContainer := make(map[string][]store.ExportItem)
	for _, item := range items {
		itemsByContainer[item.ContainerID] = append(itemsByContainer[item.ContainerID], item)
	}

	// Index containers by parent ID.
	childrenOf := make(map[string][]store.Container)
	for _, c := range containers {
		childrenOf[c.ParentID] = append(childrenOf[c.ParentID], c)
	}

	// Find roots (containers whose parent is not in the set).
	containerIDs := make(map[string]bool, len(containers))
	for _, c := range containers {
		containerIDs[c.ID] = true
	}

	var roots []store.Container
	for _, c := range containers {
		if c.ParentID == "" || !containerIDs[c.ParentID] {
			roots = append(roots, c)
		}
	}

	var buildNode func(c store.Container) jsonContainerNode
	buildNode = func(c store.Container) jsonContainerNode {
		node := jsonContainerNode{
			ID:       c.ID,
			Name:     c.Name,
			Items:    make([]jsonFlatItem, 0),
			Children: make([]jsonContainerNode, 0),
		}
		for _, item := range itemsByContainer[c.ID] {
			tags := item.TagNames
			if tags == nil {
				tags = []string{}
			}
			node.Items = append(node.Items, jsonFlatItem{
				ID:          item.ID,
				Name:        item.Name,
				Description: item.Description,
				Quantity:    item.Quantity,
				Tags:        tags,
				CreatedAt:   item.CreatedAt.Format(time.RFC3339),
			})
		}
		for _, child := range childrenOf[c.ID] {
			node.Children = append(node.Children, buildNode(child))
		}
		return node
	}

	export := jsonGroupedExport{Containers: make([]jsonContainerNode, 0, len(roots))}
	for _, root := range roots {
		export.Containers = append(export.Containers, buildNode(root))
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(export)
}
```

- [ ] **Step 11: Run test — verify it passes**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/service/ -run TestFormatJSONGrouped -v`
Expected: PASS.

- [ ] **Step 12: Write failing test for Markdown table format**

Add to `internal/service/export_format_test.go`:

```go
func TestFormatMarkdownTable(t *testing.T) {
	var buf bytes.Buffer
	paths := buildContainerPaths(testContainers())

	err := FormatMarkdownTable(&buf, testItems(), paths)
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "| item_id") {
		t.Error("missing table header")
	}
	if !strings.Contains(out, "| ---") {
		t.Error("missing separator row")
	}
	if !strings.Contains(out, "Widget") {
		t.Error("missing item data")
	}
}
```

- [ ] **Step 13: Implement FormatMarkdownTable**

Add to `internal/service/export_format.go`:

```go
// FormatMarkdownTable writes items as a pipe-delimited Markdown table.
func FormatMarkdownTable(w io.Writer, items []store.ExportItem, paths map[string]string) error {
	fmt.Fprintln(w, "| item_id | item_name | quantity | tags | description | container_path | created_at |")
	fmt.Fprintln(w, "| --- | --- | --- | --- | --- | --- | --- |")

	for _, item := range items {
		tags := strings.Join(item.TagNames, "; ")
		fmt.Fprintf(w, "| %s | %s | %d | %s | %s | %s | %s |\n",
			item.ID, item.Name, item.Quantity, tags,
			item.Description, paths[item.ContainerID],
			item.CreatedAt.Format(time.RFC3339))
	}
	return nil
}
```

- [ ] **Step 14: Run test — verify it passes**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/service/ -run TestFormatMarkdownTable -v`
Expected: PASS.

- [ ] **Step 15: Write failing test for Markdown document format**

Add to `internal/service/export_format_test.go`:

```go
func TestFormatMarkdownDocument(t *testing.T) {
	var buf bytes.Buffer

	containers := testContainers()
	items := testItems()

	err := FormatMarkdownDocument(&buf, containers, items)
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "## Root") {
		t.Error("missing Root header")
	}
	if !strings.Contains(out, "## Child") {
		t.Error("missing Child header")
	}
	if !strings.Contains(out, "- **Widget**") {
		t.Error("missing item bullet")
	}
}
```

- [ ] **Step 16: Implement FormatMarkdownDocument**

Add to `internal/service/export_format.go`:

```go
// FormatMarkdownDocument writes items grouped by container as a Markdown document with headers.
func FormatMarkdownDocument(w io.Writer, containers []store.Container, items []store.ExportItem) error {
	itemsByContainer := make(map[string][]store.ExportItem)
	for _, item := range items {
		itemsByContainer[item.ContainerID] = append(itemsByContainer[item.ContainerID], item)
	}

	childrenOf := make(map[string][]store.Container)
	for _, c := range containers {
		childrenOf[c.ParentID] = append(childrenOf[c.ParentID], c)
	}

	containerIDs := make(map[string]bool, len(containers))
	for _, c := range containers {
		containerIDs[c.ID] = true
	}

	var roots []store.Container
	for _, c := range containers {
		if c.ParentID == "" || !containerIDs[c.ParentID] {
			roots = append(roots, c)
		}
	}

	var writeContainer func(c store.Container, depth int)
	writeContainer = func(c store.Container, depth int) {
		prefix := strings.Repeat("#", depth+1)
		fmt.Fprintf(w, "%s %s\n\n", prefix, c.Name)

		for _, item := range itemsByContainer[c.ID] {
			line := fmt.Sprintf("- **%s**", item.Name)
			if item.Quantity > 1 {
				line += fmt.Sprintf(" (x%d)", item.Quantity)
			}
			if item.Description != "" {
				line += fmt.Sprintf(" — %s", item.Description)
			}
			if len(item.TagNames) > 0 {
				line += fmt.Sprintf(" [%s]", strings.Join(item.TagNames, ", "))
			}
			fmt.Fprintln(w, line)
		}

		if len(itemsByContainer[c.ID]) > 0 {
			fmt.Fprintln(w)
		}

		for _, child := range childrenOf[c.ID] {
			writeContainer(child, depth+1)
		}
	}

	for _, root := range roots {
		writeContainer(root, 1)
	}
	return nil
}
```

- [ ] **Step 17: Run test — verify it passes**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/service/ -run TestFormatMarkdownDocument -v`
Expected: PASS.

- [ ] **Step 18: Write test for empty export**

Add to `internal/service/export_format_test.go`:

```go
func TestFormatCSV_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := FormatCSV(&buf, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Errorf("got %d lines, want 1 (header only)", len(lines))
	}
}
```

- [ ] **Step 19: Run all formatter tests**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/service/ -run TestFormat -v`
Expected: all PASS.

- [ ] **Step 20: Commit**

```bash
git add internal/service/export_format.go internal/service/export_format_test.go
git commit -m "feat(service): add CSV, JSON, and Markdown export formatters"
```

---

## Task 5: ExportService — orchestration methods

**Files:**
- Modify: `internal/service/export.go`

- [ ] **Step 1: Rewrite ExportService with new public methods**

**Important:** Keep the old `ExportJSON()`, `AllItems()`, `AllContainers()` methods until Task 12 removes them — the old handler still references them until Task 6 rewrites it.

Replace `internal/service/export.go` with:

```go
package service

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/erxyi/qlx/internal/store"
)

// ExportService handles data export operations.
type ExportService struct {
	store store.ExportStore
}

// NewExportService creates a new ExportService.
func NewExportService(s store.ExportStore) *ExportService {
	return &ExportService{store: s}
}

// ExportCSV writes CSV export to w.
func (s *ExportService) ExportCSV(w io.Writer, containerID string, recursive bool) error {
	items, containers, err := s.fetchExportData(containerID, recursive)
	if err != nil {
		return err
	}
	paths := buildContainerPaths(containers)
	return FormatCSV(w, items, paths)
}

// ExportJSON writes JSON export to w.
func (s *ExportService) ExportJSON(w io.Writer, containerID string, recursive bool) error {
	items, containers, err := s.fetchExportData(containerID, recursive)
	if err != nil {
		return err
	}

	grouped := (containerID != "" && recursive) || containerID == ""
	if grouped {
		return FormatJSONGrouped(w, containers, items)
	}
	paths := buildContainerPaths(containers)
	return FormatJSONFlat(w, items, paths)
}

// ExportMarkdown writes Markdown export to w.
func (s *ExportService) ExportMarkdown(w io.Writer, containerID string, recursive bool, style string) error {
	items, containers, err := s.fetchExportData(containerID, recursive)
	if err != nil {
		return err
	}
	paths := buildContainerPaths(containers)

	switch style {
	case "document":
		return FormatMarkdownDocument(w, containers, items)
	case "both":
		if err := FormatMarkdownTable(w, items, paths); err != nil {
			return err
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, "---")
		fmt.Fprintln(w)
		return FormatMarkdownDocument(w, containers, items)
	default: // "table"
		return FormatMarkdownTable(w, items, paths)
	}
}

// ExportToString is a convenience wrapper that returns the formatted export as a string.
func (s *ExportService) ExportToString(format, containerID string, recursive bool, mdStyle string) (string, error) {
	var buf bytes.Buffer
	var err error

	switch format {
	case "csv":
		err = s.ExportCSV(&buf, containerID, recursive)
	case "json":
		err = s.ExportJSON(&buf, containerID, recursive)
	case "md":
		err = s.ExportMarkdown(&buf, containerID, recursive, mdStyle)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// fetchExportData loads items and containers for the given scope.
func (s *ExportService) fetchExportData(containerID string, recursive bool) ([]store.ExportItem, []store.Container, error) {
	if containerID == "" {
		items, err := s.store.ExportItems("", false)
		if err != nil {
			return nil, nil, err
		}
		containers := s.store.AllContainers()
		return items, containers, nil
	}

	items, err := s.store.ExportItems(containerID, recursive)
	if err != nil {
		return nil, nil, err
	}

	var containers []store.Container
	if recursive {
		containers, err = s.store.ExportContainerTree(containerID)
		if err != nil {
			return nil, nil, err
		}
	} else {
		containers = s.store.AllContainers()
	}
	return items, containers, nil
}

// Deprecated: kept for backward compatibility until handler rewrite (Task 6).
func (s *ExportService) ExportJSONLegacy() (map[string]*store.Container, map[string]*store.Item) {
	return s.store.ExportData()
}

func (s *ExportService) AllItems() []store.Item {
	return s.store.AllItems()
}

func (s *ExportService) AllContainers() []store.Container {
	return s.store.AllContainers()
}

// SanitizeFilename returns a filesystem-safe version of a container name.
func SanitizeFilename(name string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-")
	return strings.ToLower(r.Replace(name))
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/erxyi/Projekty/qlx && go build ./internal/service/...`
Expected: no errors.

- [ ] **Step 3: Run all service tests**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/service/ -v`
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/service/export.go
git commit -m "feat(service): rewrite ExportService with unified format methods"
```

---

## Task 6: Rewrite ExportHandler — unified /export endpoint

**Files:**
- Modify: `internal/handler/export.go`

- [ ] **Step 1: Rewrite the handler**

Replace `internal/handler/export.go` with:

```go
package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

// ExportHandler handles HTTP requests for data export.
type ExportHandler struct {
	export    *service.ExportService
	inventory *service.InventoryService
}

// NewExportHandler creates a new ExportHandler.
func NewExportHandler(export *service.ExportService, inv *service.InventoryService) *ExportHandler {
	return &ExportHandler{export: export, inventory: inv}
}

// RegisterRoutes registers export routes on the given mux.
func (h *ExportHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /export", h.Export)
}

// Export handles GET /export with query params: format, container, recursive, md_style, download.
func (h *ExportHandler) Export(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	containerID := r.URL.Query().Get("container")
	recursive := r.URL.Query().Get("recursive") == "true"
	mdStyle := r.URL.Query().Get("md_style")
	download := r.URL.Query().Get("download") == "true"

	if format == "" || (format != "csv" && format != "json" && format != "md") {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or missing format parameter (csv, json, md)"})
		return
	}

	if mdStyle == "" {
		mdStyle = "table"
	}
	if mdStyle != "table" && mdStyle != "document" && mdStyle != "both" {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid md_style parameter (table, document, both)"})
		return
	}

	// Validate container exists if specified.
	var containerName string
	if containerID != "" {
		c := h.inventory.GetContainer(containerID)
		if c == nil {
			webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "container not found"})
			return
		}
		containerName = c.Name
	}

	// Build filename.
	var filename string
	ext := format
	if ext == "md" {
		ext = "md"
	}
	if containerID != "" {
		filename = "qlx-" + service.SanitizeFilename(containerName) + "-export." + ext
	} else {
		filename = "qlx-export." + ext
	}

	// Set content type.
	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	case "json":
		w.Header().Set("Content-Type", "application/json")
	case "md":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	}

	if download {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	}

	var err error
	switch format {
	case "csv":
		err = h.export.ExportCSV(w, containerID, recursive)
	case "json":
		err = h.export.ExportJSON(w, containerID, recursive)
	case "md":
		err = h.export.ExportMarkdown(w, containerID, recursive, mdStyle)
	}

	if err != nil {
		webutil.LogError("export %s: %v", format, err)
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/erxyi/Projekty/qlx && go build ./...`
Expected: no errors. Uses `h.inventory.GetContainer(id)` which returns `*store.Container` (nil on not found).

- [ ] **Step 3: Run lint**

Run: `cd /Users/erxyi/Projekty/qlx && make lint`
Expected: no issues.

- [ ] **Step 4: Commit**

```bash
git add internal/handler/export.go
git commit -m "feat(handler): rewrite export handler with unified /export endpoint"
```

---

## Task 7: i18n — translation keys

**Files:**
- Create: `internal/embedded/static/i18n/en/export.json`
- Create: `internal/embedded/static/i18n/pl/export.json`
- Modify: `internal/embedded/static/i18n/en/settings.json`
- Modify: `internal/embedded/static/i18n/pl/settings.json`
- Modify: `internal/embedded/static/i18n/en/actions.json`
- Modify: `internal/embedded/static/i18n/pl/actions.json`

- [ ] **Step 1: Create English export translations**

```json
{
  "export.title": "Export",
  "export.format": "Format",
  "export.format_csv": "CSV",
  "export.format_json": "JSON",
  "export.format_md": "Markdown",
  "export.md_style": "Markdown style",
  "export.md_table": "Table",
  "export.md_document": "Document",
  "export.md_both": "Both",
  "export.recursive": "Include sub-containers",
  "export.preview": "Preview",
  "export.download": "Download",
  "export.copy": "Copy to clipboard",
  "export.copied": "Copied!",
  "export.back": "Back",
  "export.filename": "File"
}
```

- [ ] **Step 2: Create Polish export translations**

```json
{
  "export.title": "Eksport",
  "export.format": "Format",
  "export.format_csv": "CSV",
  "export.format_json": "JSON",
  "export.format_md": "Markdown",
  "export.md_style": "Styl Markdown",
  "export.md_table": "Tabela",
  "export.md_document": "Dokument",
  "export.md_both": "Oba",
  "export.recursive": "Uwzględnij podkontenery",
  "export.preview": "Podgląd",
  "export.download": "Pobierz",
  "export.copy": "Kopiuj do schowka",
  "export.copied": "Skopiowano!",
  "export.back": "Wróć",
  "export.filename": "Plik"
}
```

- [ ] **Step 3: Update settings translations — replace export keys**

In `en/settings.json` replace `"settings.export_json"` and `"settings.export_csv"` with `"settings.export"`:

```json
{
  "nav.settings": "Settings",
  "settings.title": "Settings",
  "settings.language": "Language",
  "settings.data": "Data",
  "settings.export": "Export data"
}
```

Do the same for `pl/settings.json`.

- [ ] **Step 4: Add export action to actions translations**

Add to `en/actions.json`: `"action.export": "Export"`
Add to `pl/actions.json`: `"action.export": "Eksport"`

- [ ] **Step 5: Verify i18n loads without error**

Run: `cd /Users/erxyi/Projekty/qlx && make build-mac && ./qlx --port 0 &; sleep 2; kill %1`
Expected: no i18n errors in logs.

- [ ] **Step 6: Commit**

```bash
git add internal/embedded/static/i18n/
git commit -m "feat(i18n): add export modal translation keys (en/pl)"
```

---

## Task 8: Export dialog JS

**Files:**
- Create: `internal/embedded/static/js/shared/export-dialog.js`

- [ ] **Step 1: Implement the export dialog factory**

Create `internal/embedded/static/js/shared/export-dialog.js`. Follow the tree-picker lazy-init pattern. Key behaviors:

- `qlx.openExportDialog(opts)` — opts: `{ containerId, containerName }` or `{}` for full inventory
- Step 1: format radios (CSV/JSON/MD), MD sub-options, recursive checkbox, Preview button
- Step 2: `<pre>` preview, filename label, Download + Copy + Back buttons
- Preview via `fetch("/export?format=...&container=...&recursive=...")`, cache response text
- Download via Blob URL + invisible `<a>` click
- Copy via `navigator.clipboard.writeText()`
- All DOM via `createElement` + `textContent` (no innerHTML)
- i18n via `qlx.t()`
- Show/hide MD sub-options when MD radio selected

The full JS implementation should be ~150-200 lines. Use the existing `tree-picker.js` and `delete-confirm.js` patterns as reference for dialog lifecycle, event handling, and safe DOM construction.

- [ ] **Step 2: Verify syntax**

Run: `cd /Users/erxyi/Projekty/qlx && node --check internal/embedded/static/js/shared/export-dialog.js`
Expected: no syntax errors.

- [ ] **Step 3: Commit**

```bash
git add internal/embedded/static/js/shared/export-dialog.js
git commit -m "feat(ui): add export dialog JS component"
```

---

## Task 9: Export dialog CSS

**Files:**
- Create: `internal/embedded/static/css/dialogs/export.css`

- [ ] **Step 1: Create export dialog styles**

Create `internal/embedded/static/css/dialogs/export.css`. Build on existing `dialog.css` base. Key rules:

- `.export-dialog` — max-width: 600px (wider than default 500px for preview)
- `.export-options` — flex column, gap for radio groups
- `.export-format-group`, `.export-md-group` — radio group styling
- `.export-preview` — `<pre>` block: `max-height: 50vh`, `overflow: auto`, `font-size: 0.85rem`, `background: var(--color-bg)`, `border: 1px solid var(--color-border)`, `border-radius: 4px`, `padding: 1rem`
- `.export-filename` — small muted text above preview
- `.export-actions` — footer button row, flex with gap
- `@media (max-width: 600px)` — full viewport dialog
- `.export-md-group[hidden]` — `display: none`

**Dropdown styles** (no existing dropdown pattern in codebase):
- `.dropdown` — `position: relative`, `display: inline-block`
- `.dropdown-toggle` — styled like existing `.button.secondary.small`, content: `⋯` (vertical ellipsis)
- `.dropdown-menu` — `display: none`, `position: absolute`, `right: 0`, `background: var(--color-bg-card)`, `border: 1px solid var(--color-border)`, `border-radius: 4px`, `box-shadow: 0 2px 8px rgba(0,0,0,0.15)`, `z-index: 10`, `min-width: 150px`
- `.dropdown.open .dropdown-menu` — `display: block`
- `.dropdown-item` — `display: block`, `width: 100%`, `padding: 0.5rem 1rem`, `text-align: left`, `border: none`, `background: none`, `cursor: pointer`
- `.dropdown-item:hover` — `background: var(--color-bg-hover)`

**Dropdown JS toggle** (add to export-dialog.js or inline): click handler on `.dropdown-toggle` toggles `.open` class on parent `.dropdown`. Click outside closes it via `document.addEventListener("click", ...)`.

**Alternative:** If this is too much for export.css, create `internal/embedded/static/css/shared/dropdown.css` and `internal/embedded/static/js/shared/dropdown.js` as reusable components. Either approach is fine — the implementer should decide based on whether other features will need dropdowns soon.

- [ ] **Step 2: Commit**

```bash
git add internal/embedded/static/css/dialogs/export.css
git commit -m "feat(ui): add export dialog CSS styles"
```

---

## Task 10: Wire into base layout + templates

**Files:**
- Modify: `internal/embedded/templates/layouts/base.html` — add CSS + JS
- Modify: `internal/embedded/templates/pages/settings/settings.html` — replace export links
- Modify: `internal/embedded/templates/pages/inventory/containers.html` — add dropdown

- [ ] **Step 1: Add CSS and JS includes to base layout**

In `base.html`:
- Add `<link rel="stylesheet" href="/static/css/dialogs/export.css">` in the `<head>` section alongside other dialog CSS.
- Add `<script src="/static/js/shared/export-dialog.js"></script>` in the scripts section alongside other shared JS.

- [ ] **Step 2: Update settings page — replace export links with modal button**

In `settings.html`, replace the two `<a>` download links (lines 23-24 approximately) with a single button:

```html
<button class="btn btn-secondary" onclick="qlx.openExportDialog({})">
  {{.T "settings.export"}}
</button>
```

- [ ] **Step 3: Add "more actions" dropdown to container detail header**

In `containers.html`, after the Edit button (line 17), add a dropdown:

```html
<div class="dropdown">
    <button class="button secondary small dropdown-toggle">&#8943;</button>
    <div class="dropdown-menu">
        <button class="dropdown-item" onclick="qlx.openExportDialog({containerId: '{{ .Data.Container.ID }}', containerName: '{{ .Data.Container.Name }}'})">
            {{.T "action.export"}}
        </button>
    </div>
</div>
```

Check if there's an existing dropdown pattern/CSS in the codebase first. If not, add minimal dropdown CSS to the export.css or a new `dropdown.css`.

- [ ] **Step 4: Build and verify manually**

Run: `cd /Users/erxyi/Projekty/qlx && make build-mac && ./qlx --port 8080`

Open browser: verify Export button appears in settings page and dropdown appears on container detail.

- [ ] **Step 5: Commit**

```bash
git add internal/embedded/templates/
git commit -m "feat(ui): wire export dialog into settings and container views"
```

---

## Task 11: Handler unit tests

**Files:**
- Create: `internal/handler/export_test.go`

- [ ] **Step 1: Write handler tests**

Create `internal/handler/export_test.go` with `httptest` tests:

```go
package handler_test
```

Test cases (table-driven):
- `GET /export?format=csv` → 200, Content-Type `text/csv; charset=utf-8`
- `GET /export?format=json` → 200, Content-Type `application/json`
- `GET /export?format=md` → 200, Content-Type `text/markdown; charset=utf-8`
- `GET /export` (no format) → 400
- `GET /export?format=invalid` → 400
- `GET /export?format=csv&container=nonexistent-id` → 404
- `GET /export?format=csv&download=true` → `Content-Disposition: attachment` header
- `GET /export?format=csv&container={valid-id}` → 200, contains only that container's items
- `GET /export?format=md&md_style=invalid` → 400

Use `:memory:` SQLite store with seed data. Construct real `ExportService` + `InventoryService` over the test store — no mocks.

- [ ] **Step 2: Run tests**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/handler/ -run TestExport -v`
Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/handler/export_test.go
git commit -m "test(handler): add unit tests for unified /export endpoint"
```

---

## Task 12: E2E tests

**Files:**
- Modify: `e2e/tests/export.spec.ts`

- [ ] **Step 1: Rewrite E2E tests for new export system**

Replace `e2e/tests/export.spec.ts` with tests covering:

1. **API tests** (no modal):
   - `GET /export?format=csv` — verify headers, CSV content
   - `GET /export?format=json` — verify JSON structure
   - `GET /export?format=md` — verify Markdown output
   - `GET /export?format=csv&container={id}` — per-container export
   - `GET /export?format=csv&container={id}&recursive=true` — recursive
   - `GET /export?format=invalid` — 400 response
   - `GET /export?format=csv&container=nonexistent` — 404 response
   - `GET /export?format=csv&download=true` — Content-Disposition header

2. **Modal tests**:
   - Settings page: click Export → modal opens → select CSV → Preview → verify `<pre>` content → click Download
   - Container page: dropdown → Export → modal opens → select JSON → Preview → verify content
   - Markdown sub-options: select MD → verify style options appear → select Document → Preview
   - Copy to clipboard (if Playwright supports clipboard assertions)
   - Back button: returns to step 1

Use the existing fixture pattern from `e2e/fixtures/app.ts`. Create test data via API calls in a setup step. Use `page.waitForResponse()` for HTMX assertions.

- [ ] **Step 2: Run E2E tests**

Run: `cd /Users/erxyi/Projekty/qlx && make test-e2e`
Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add e2e/tests/export.spec.ts
git commit -m "test(e2e): rewrite export tests for unified endpoint and modal"
```

---

## Task 13: Cleanup — remove old ExportData method and legacy service methods

**Files:**
- Modify: `internal/store/interfaces.go`
- Modify: `internal/store/sqlite/export.go`
- Modify: `internal/store/sqlite/export_test.go`

- [ ] **Step 1: Remove ExportData from interface**

In `internal/store/interfaces.go`, remove the `ExportData() (map[string]*Container, map[string]*Item)` line from `ExportStore`.

- [ ] **Step 2: Remove ExportData implementation from SQLite store**

In `internal/store/sqlite/export.go`, remove the `ExportData` method.

- [ ] **Step 2b: Remove legacy methods from ExportService**

In `internal/service/export.go`, remove `ExportJSONLegacy()`, the old `AllItems()`, and `AllContainers()` methods that were kept for backward compatibility.

- [ ] **Step 3: Remove ExportData tests**

In `internal/store/sqlite/export_test.go`, remove `TestExportStore_ExportData` and `TestExportStore_ExportData_Empty`.

- [ ] **Step 4: Verify everything compiles and tests pass**

Run: `cd /Users/erxyi/Projekty/qlx && go build ./... && go test ./...`
Expected: no errors, all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/interfaces.go internal/store/sqlite/export.go internal/store/sqlite/export_test.go
git commit -m "refactor(store): remove deprecated ExportData method"
```

---

## Task 14: Final verification

- [ ] **Step 1: Run full test suite**

Run: `cd /Users/erxyi/Projekty/qlx && make test`
Expected: all PASS.

- [ ] **Step 2: Run lint**

Run: `cd /Users/erxyi/Projekty/qlx && make lint`
Expected: no issues.

- [ ] **Step 3: Run E2E tests**

Run: `cd /Users/erxyi/Projekty/qlx && make test-e2e`
Expected: all PASS.

- [ ] **Step 4: Manual smoke test**

Run: `cd /Users/erxyi/Projekty/qlx && make build-mac && ./qlx --port 8080`

Verify:
- Settings page: Export button → modal → CSV/JSON/MD all work → Download + Copy
- Container detail: dropdown → Export → per-container export works
- Recursive checkbox with nested containers
- Markdown style sub-options
