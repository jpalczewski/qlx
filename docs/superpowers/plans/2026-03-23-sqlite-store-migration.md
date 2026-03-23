# SQLite Store Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the JSON file store with SQLite backend, remove asset system, clean up handler/API inconsistencies, and add FTS5 full-text search.

**Architecture:** New `internal/store/sqlite/` package implementing existing service interfaces via `ncruces/go-sqlite3` (pure Go). Goose v3 manages SQL migrations. Models and errors stay in `internal/store/`. Auto-migration from JSON at startup. Service layer drops `Save()` / `Saveable`. Handler cleanup bundles unified error responses, `RenderPartial` on `Responder`, and status code fixes.

**Tech Stack:** Go 1.25, `ncruces/go-sqlite3`, `pressly/goose/v3`, SQLite FTS5, standard library testing

**Spec:** `docs/superpowers/specs/2026-03-23-sqlite-store-migration-design.md`

---

## File Map

### New files

| File | Responsibility |
|------|---------------|
| `internal/store/errors.go` | Error sentinels extracted from `store.go` |
| `internal/store/interfaces.go` | Sub-interfaces moved from `service/interfaces.go` (avoids import cycle) |
| `internal/store/store.go` | `Store` aggregate interface (replaces struct) |
| `internal/store/sqlite/sqlite.go` | `SQLiteStore` struct, `New()`, `Close()`, pragma setup |
| `internal/store/sqlite/migrations.go` | `go:embed` for SQL files |
| `internal/store/sqlite/migrations/001_initial_schema.sql` | Tables, indexes, FK constraints |
| `internal/store/sqlite/migrations/002_fts5_indexes.sql` | FTS5 virtual tables + sync triggers |
| `internal/store/sqlite/containers.go` | `ContainerStore` impl |
| `internal/store/sqlite/containers_test.go` | Container CRUD + hierarchy tests |
| `internal/store/sqlite/items.go` | `ItemStore` impl |
| `internal/store/sqlite/items_test.go` | Item CRUD tests |
| `internal/store/sqlite/tags.go` | `TagStore` impl (including `ResolveTagIDs`, `TagItemStats`) |
| `internal/store/sqlite/tags_test.go` | Tag CRUD + junction + cascade tests |
| `internal/store/sqlite/printers.go` | `PrinterStore` impl (including `GetPrinter`) |
| `internal/store/sqlite/printers_test.go` | Printer CRUD tests |
| `internal/store/sqlite/templates.go` | `TemplateStore` impl (`CreateTemplate`/`UpdateTemplate` split) |
| `internal/store/sqlite/templates_test.go` | Template CRUD tests |
| `internal/store/sqlite/bulk.go` | `BulkStore` impl (transactional) |
| `internal/store/sqlite/bulk_test.go` | Bulk move/delete/tag tests |
| `internal/store/sqlite/search.go` | `SearchStore` impl (FTS5 for items/containers, LIKE for tags) |
| `internal/store/sqlite/search_test.go` | FTS5 search tests |
| `internal/store/sqlite/export.go` | `ExportStore` impl |
| `internal/store/sqlite/export_test.go` | Export tests |
| `internal/store/sqlite/migrate_json.go` | Auto JSON→SQLite import |
| `internal/store/sqlite/migrate_json_test.go` | Migration test with fixtures |
| `internal/store/sqlite/testdata/` | Test fixtures (legacy JSON, partitioned JSON) |

### Modified files

| File | Changes |
|------|---------|
| `internal/store/models.go` | Remove `Asset` struct. Keep `TagIDs` on `Container`/`Item` (computed). |
| `internal/service/interfaces.go` | **Deleted** — interfaces moved to `store/interfaces.go` |
| `internal/service/inventory.go` | Remove `Saveable` from store interface. Remove `Save()` calls. `DeleteContainer` returns `(string, error)`. `DeleteItem` returns `(string, error)`. |
| `internal/service/tags.go` | Remove `Saveable`. Remove `Save()` calls. `DeleteTag` returns `(string, error)`. Add `ResolveTagIDs`, `TagItemStats` passthrough. |
| `internal/service/templates.go` | Remove `Saveable`. Remove `Save()` calls. Replace `SaveTemplate` with `UpdateTemplate`. `DeleteTemplate` returns `error`. |
| `internal/service/printers.go` | Remove `Saveable`. Remove `Save()` calls. Add `GetPrinter` passthrough. |
| `internal/service/bulk.go` | Remove `Saveable`. Remove `Save()` calls. |
| `internal/service/search.go` | No changes (already no `Saveable`). |
| `internal/service/export.go` | No changes. |
| `internal/handler/responder.go` | Add `RenderPartial` to `Responder` interface. |
| `internal/handler/responder_html.go` | `RenderPartial` already exists — just ensure interface compliance. |
| `internal/handler/containers.go` | Use `RenderPartial` via interface. Delete returns 204. Remove `ItemsJSON`. Delete uses returned `parentID`. |
| `internal/handler/items.go` | Use `RenderPartial` via interface. Delete returns 204. Delete uses returned `containerID`. |
| `internal/handler/tags.go` | Use `RenderPartial` via interface. Delete returns 204. Delete uses returned `parentID`. |
| `internal/handler/templates.go` | Use `UpdateTemplate` service method. POST returns 201 + object. |
| `internal/handler/print.go` | Use `TagService.ResolveTagIDs`. Remove local `resolveTags`. |
| `internal/handler/debug.go` | Use `GetPrinter` instead of linear scan. |
| `internal/shared/webutil/errors.go` | Add `WriteError` unified helper. |
| `internal/app/server.go` | Wire `sqlite.New()`. Remove `resolveTags` closure. Remove asset service/handler. `Shutdown` calls `db.Close()`. |
| `cmd/qlx/main.go` | Pass `dataDir` directly to `NewServer`. Remove `store.NewStore` call. |
| `internal/app/server_test.go` | Replace `NewMemoryStore()` with `sqlite.New()` |
| `internal/print/manager_test.go` | Replace `NewMemoryStore()` x4 with `sqlite.New()` |
| `go.mod` / `go.sum` | Add `ncruces/go-sqlite3`, `pressly/goose/v3`. |

### Deleted files

| File | Reason |
|------|--------|
| `internal/store/store.go` (old impl) | Replaced by `sqlite/` package. Old `Store` struct code is deleted. File is repurposed for `Store` interface. |
| `internal/store/tags.go` | Replaced by `sqlite/tags.go` |
| `internal/store/bulk.go` | Replaced by `sqlite/bulk.go` |
| `internal/store/search.go` | Replaced by `sqlite/search.go` |
| `internal/store/migrate.go` | Replaced by goose migrations + `sqlite/migrate_json.go` |
| `internal/store/store_test.go` | Replaced by `sqlite/*_test.go` |
| `internal/store/tags_test.go` | Replaced by `sqlite/tags_test.go` |
| `internal/store/bulk_test.go` | Replaced by `sqlite/bulk_test.go` |
| `internal/store/search_test.go` | Replaced by `sqlite/search_test.go` |
| `internal/store/migrate_test.go` | Replaced by `sqlite/migrate_json_test.go` |
| `internal/service/interfaces.go` | Interfaces moved to `store/interfaces.go` |
| `internal/service/assets.go` | Asset system removed |
| `internal/handler/assets.go` | Asset system removed |

---

## Task 1: Add Dependencies and SQL Migrations

**Files:**
- Modify: `go.mod`
- Create: `internal/store/sqlite/migrations/001_initial_schema.sql`
- Create: `internal/store/sqlite/migrations/002_fts5_indexes.sql`
- Create: `internal/store/sqlite/migrations.go`

- [ ] **Step 1: Add ncruces/go-sqlite3 and goose dependencies**

```bash
cd /Users/erxyi/Projekty/qlx
go get github.com/ncruces/go-sqlite3
go get github.com/ncruces/go-sqlite3/driver
go get github.com/pressly/goose/v3
go mod tidy
```

- [ ] **Step 2: Create 001_initial_schema.sql**

Create `internal/store/sqlite/migrations/001_initial_schema.sql` with the full schema from the spec: `tags`, `containers`, `items`, `item_tags`, `container_tags`, `printer_configs`, `templates` tables. Include all indexes. Include `+goose Up` / `+goose Down` annotations.

Reference: spec lines 67-143.

- [ ] **Step 3: Create 002_fts5_indexes.sql**

Create `internal/store/sqlite/migrations/002_fts5_indexes.sql` with FTS5 virtual tables and sync triggers for items and containers.

Reference: spec lines 147-191. Include `+goose Up` and `+goose Down` annotations. The Down migration must drop FTS virtual tables and triggers in reverse order.

- [ ] **Step 4: Create migrations.go with go:embed**

Create `internal/store/sqlite/migrations.go`:

```go
package sqlite

import "embed"

//go:embed migrations/*.sql
var migrationFS embed.FS
```

- [ ] **Step 5: Verify build**

```bash
go build ./internal/store/sqlite/
```

Expected: compiles without errors.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/store/sqlite/
git commit -m "feat(store): add SQLite dependencies and SQL migration files"
```

---

## Task 2: SQLiteStore Core — New(), Close(), Pragmas

**Files:**
- Create: `internal/store/sqlite/sqlite.go`
- Create: `internal/store/sqlite/sqlite_test.go`

- [ ] **Step 1: Write test for New() and Close()**

Create `internal/store/sqlite/sqlite_test.go`:

```go
package sqlite

import (
    "os"
    "path/filepath"
    "testing"
)

func testStore(t *testing.T) *SQLiteStore {
    t.Helper()
    db, err := New(t.TempDir())
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { db.Close() })
    return db
}

func TestNew_CreatesDBFile(t *testing.T) {
    dir := t.TempDir()
    db, err := New(dir)
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    dbPath := filepath.Join(dir, "qlx.db")
    if _, err := os.Stat(dbPath); os.IsNotExist(err) {
        t.Fatalf("expected %s to exist", dbPath)
    }
}

func TestNew_RunsMigrations(t *testing.T) {
    db := testStore(t)

    // Verify tables exist by querying them
    var count int
    err := db.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='containers'").Scan(&count)
    if err != nil {
        t.Fatal(err)
    }
    if count != 1 {
        t.Fatal("containers table not created by migrations")
    }
}

func TestNew_SetsPragmas(t *testing.T) {
    db := testStore(t)

    var journalMode string
    db.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
    if journalMode != "wal" {
        t.Fatalf("expected WAL journal mode, got %s", journalMode)
    }

    var fk int
    db.db.QueryRow("PRAGMA foreign_keys").Scan(&fk)
    if fk != 1 {
        t.Fatal("expected foreign_keys ON")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/store/sqlite/ -run TestNew -v
```

Expected: FAIL — `SQLiteStore` type and `New` function not defined.

- [ ] **Step 3: Implement SQLiteStore**

Create `internal/store/sqlite/sqlite.go`:

```go
package sqlite

import (
    "database/sql"
    "fmt"
    "path/filepath"

    _ "github.com/ncruces/go-sqlite3/driver"
    _ "github.com/ncruces/go-sqlite3/embed"
    "github.com/pressly/goose/v3"
)

// SQLiteStore implements the store.Store interface using SQLite.
type SQLiteStore struct {
    db      *sql.DB
    dataDir string
}

// New opens (or creates) a SQLite database in dataDir, runs migrations, and returns the store.
// Pass ":memory:" as dataDir for in-memory testing.
func New(dataDir string) (*SQLiteStore, error) {
    var dsn string
    if dataDir == ":memory:" {
        dsn = ":memory:"
    } else {
        dsn = filepath.Join(dataDir, "qlx.db")
    }

    db, err := sql.Open("sqlite3", dsn)
    if err != nil {
        return nil, fmt.Errorf("open sqlite: %w", err)
    }

    // Single connection for WAL mode consistency
    db.SetMaxOpenConns(1)

    if err := setPragmas(db); err != nil {
        db.Close()
        return nil, fmt.Errorf("set pragmas: %w", err)
    }

    if err := runMigrations(db); err != nil {
        db.Close()
        return nil, fmt.Errorf("run migrations: %w", err)
    }

    return &SQLiteStore{db: db, dataDir: dataDir}, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
    return s.db.Close()
}

func setPragmas(db *sql.DB) error {
    pragmas := []string{
        "PRAGMA journal_mode = WAL",
        "PRAGMA synchronous = NORMAL",
        "PRAGMA busy_timeout = 5000",
        "PRAGMA cache_size = -4096",
        "PRAGMA foreign_keys = ON",
        "PRAGMA temp_store = MEMORY",
        "PRAGMA auto_vacuum = INCREMENTAL", // only effective on first DB creation; no-op on existing DBs
    }
    for _, p := range pragmas {
        if _, err := db.Exec(p); err != nil {
            return fmt.Errorf("%s: %w", p, err)
        }
    }
    return nil
}

func runMigrations(db *sql.DB) error {
    goose.SetBaseFS(migrationFS)
    if err := goose.SetDialect("sqlite3"); err != nil {
        return err
    }
    return goose.Up(db, "migrations")
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/store/sqlite/ -run TestNew -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go
git commit -m "feat(store): SQLiteStore core with New(), Close(), pragma setup, goose migrations"
```

---

## Task 3: Extract Errors, Move Interfaces to Store Package, Remove Old Implementation

This task is atomic — the codebase won't compile until all steps are complete.

**Files:**
- Create: `internal/store/errors.go`
- Create: `internal/store/interfaces.go` — sub-interfaces moved FROM `service/interfaces.go`
- Modify: `internal/store/models.go` — remove `Asset` struct
- Rewrite: `internal/store/store.go` — aggregate `Store` interface
- Modify: `internal/service/interfaces.go` — delete file (interfaces moved to `store/`)
- Delete: old JSON implementation files

- [ ] **Step 1: Create errors.go**

Extract all error sentinels from `internal/store/store.go` (lines ~284-310) and `internal/store/tags.go` (lines 11-14) into `internal/store/errors.go`:

```go
package store

import "errors"

var (
    ErrContainerNotFound    = errors.New("container not found")
    ErrItemNotFound         = errors.New("item not found")
    ErrTagNotFound          = errors.New("tag not found")
    ErrPrinterNotFound      = errors.New("printer not found")
    ErrTemplateNotFound     = errors.New("template not found")
    ErrContainerHasChildren = errors.New("container has children")
    ErrContainerHasItems    = errors.New("container has items")
    ErrTagHasChildren       = errors.New("tag has children")
    ErrCycleDetected        = errors.New("cycle detected")
    ErrInvalidParent        = errors.New("invalid parent")
    ErrInvalidContainer     = errors.New("invalid container")
)
```

Check exact error names in current `store.go` and `tags.go` before extracting.

- [ ] **Step 2: Remove Asset struct from models.go**

Remove `Asset` struct (lines 65-70 in `models.go`).

- [ ] **Step 3: Move sub-interfaces from service/ to store/**

Create `internal/store/interfaces.go` with ALL sub-interfaces moved from `internal/service/interfaces.go`. This solves the import cycle: `store/` defines interfaces, `service/` imports them, `sqlite/` implements them.

Apply spec changes while moving:

```go
package store

// ContainerStore defines container-related store operations.
type ContainerStore interface {
    GetContainer(id string) *Container
    CreateContainer(parentID, name, desc, color, icon string) *Container
    UpdateContainer(id, name, desc, color, icon string) (*Container, error)
    DeleteContainer(id string) (string, error) // returns parentID
    MoveContainer(id, newParentID string) error
    ContainerChildren(parentID string) []Container
    ContainerItems(containerID string) []Item
    ContainerPath(id string) []Container
    AllContainers() []Container
}

// ItemStore defines item-related store operations.
type ItemStore interface {
    GetItem(id string) *Item
    CreateItem(containerID, name, desc string, qty int, color, icon string) *Item
    UpdateItem(id, name, desc string, qty int, color, icon string) (*Item, error)
    DeleteItem(id string) (string, error) // returns containerID
    MoveItem(id, containerID string) error
}

// TagStore defines tag-related store operations.
type TagStore interface {
    GetTag(id string) *Tag
    CreateTag(parentID, name, color, icon string) *Tag
    UpdateTag(id, name, color, icon string) (*Tag, error)
    DeleteTag(id string) (string, error) // returns parentID
    MoveTag(id, newParentID string) error
    AllTags() []Tag
    TagChildren(parentID string) []Tag
    TagPath(id string) []Tag
    TagDescendants(id string) []string
    AddItemTag(itemID, tagID string) error
    RemoveItemTag(itemID, tagID string) error
    AddContainerTag(containerID, tagID string) error
    RemoveContainerTag(containerID, tagID string) error
    ItemsByTag(tagID string) []Item
    ContainersByTag(tagID string) []Container
    ResolveTagIDs(ids []string) []Tag
    TagItemStats(id string) (int, int, error)
}

// BulkStore defines bulk operation store methods.
type BulkStore interface {
    BulkMove(itemIDs, containerIDs []string, targetID string) []BulkError
    BulkDelete(itemIDs, containerIDs []string) ([]string, []BulkError)
    BulkAddTag(itemIDs, containerIDs []string, tagID string) error
}

// SearchStore defines search-related store operations.
type SearchStore interface {
    SearchContainers(query string) []Container
    SearchItems(query string) []Item
    SearchTags(query string) []Tag
}

// PrinterStore defines printer-related store operations.
type PrinterStore interface {
    AllPrinters() []PrinterConfig
    GetPrinter(id string) *PrinterConfig
    AddPrinter(name, encoder, model, transport, address string) *PrinterConfig
    DeletePrinter(id string) error
    UpdatePrinterOffset(id string, offsetX, offsetY int) error
}

// TemplateStore defines template-related store operations.
type TemplateStore interface {
    AllTemplates() []Template
    GetTemplate(id string) *Template
    CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*Template, error)
    UpdateTemplate(id, name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*Template, error)
    DeleteTemplate(id string) error
}

// ExportStore defines export-related store operations.
type ExportStore interface {
    ExportData() (map[string]*Container, map[string]*Item)
    AllItems() []Item
    AllContainers() []Container
}
```

- [ ] **Step 4: Rewrite store.go as aggregate interface**

Replace `internal/store/store.go` entirely:

```go
package store

// Store is the aggregate interface for all store operations.
type Store interface {
    ContainerStore
    ItemStore
    TagStore
    BulkStore
    SearchStore
    PrinterStore
    TemplateStore
    ExportStore
    Close() error
}
```

- [ ] **Step 5: Delete old service/interfaces.go**

Delete `internal/service/interfaces.go`. All service files must now import interfaces from `store` package (e.g., `store.ContainerStore`). Update service struct definitions accordingly:

```go
// Before (in service/inventory.go):
store interface { ContainerStore; ItemStore; Saveable }

// After:
store interface { store.ContainerStore; store.ItemStore }
```

- [ ] **Step 6: Delete old store implementation files**

Delete these files (replaced by `sqlite/` package):
- `internal/store/tags.go`
- `internal/store/bulk.go`
- `internal/store/search.go`
- `internal/store/migrate.go`
- `internal/store/store_test.go`
- `internal/store/tags_test.go`
- `internal/store/bulk_test.go`
- `internal/store/search_test.go`
- `internal/store/migrate_test.go`

- [ ] **Step 7: Verify store package compiles**

```bash
go build ./internal/store/...
```

Expected: compiles (interfaces + models + errors only, no implementation).

Note: The rest of the codebase (services, handlers) will NOT compile until Tasks 4-5 update them. This is expected.

- [ ] **Step 8: Commit**

```bash
git add -A internal/store/ internal/service/interfaces.go
git commit -m "refactor(store): move interfaces to store package, remove old JSON implementation"
```

---

## Task 4: Update Service Layer — Remove Save(), Use store.* Interfaces

**Files:**
- Modify: `internal/service/inventory.go`
- Modify: `internal/service/tags.go`
- Modify: `internal/service/templates.go`
- Modify: `internal/service/printers.go`
- Modify: `internal/service/bulk.go`
- Modify: `internal/service/search.go`
- Modify: `internal/service/export.go`
- Delete: `internal/service/assets.go`

All services must update their store interface embeds to reference `store.XxxStore` instead of the locally-defined interfaces (which were deleted in Task 3).

- [ ] **Step 1: Update InventoryService**

In `internal/service/inventory.go`:
- Change store interface embed to use `store.ContainerStore` and `store.ItemStore` (remove `Saveable`)
- Remove all `s.store.Save()` calls (lines 68, 93, 104, 112, 143, 168, 180, 188)
- Update `DeleteContainer` to capture and return parentID:
  ```go
  func (s *InventoryService) DeleteContainer(id string) (string, error) {
      return s.store.DeleteContainer(id)
  }
  ```
- Same pattern for `DeleteItem` (returns containerID)

- [ ] **Step 2: Update TagService**

In `internal/service/tags.go`:
- Change store interface embed to use `store.TagStore`, `store.ItemStore`, `store.ContainerStore` (remove `Saveable`)
- Remove all `s.store.Save()` calls
- Update `DeleteTag` to return `(string, error)`
- Add passthrough methods:
  ```go
  func (s *TagService) ResolveTagIDs(ids []string) []store.Tag {
      return s.store.ResolveTagIDs(ids)
  }

  func (s *TagService) TagItemStats(id string) (int, int, error) {
      return s.store.TagItemStats(id)
  }
  ```

- [ ] **Step 3: Update TemplateService**

In `internal/service/templates.go`:
- Change store interface embed to use `store.TemplateStore` (remove `Saveable`)
- Remove all `s.store.Save()` calls
- Replace `SaveTemplate(t store.Template)` with `UpdateTemplate(...)`:
  ```go
  func (s *TemplateService) UpdateTemplate(id, name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*store.Template, error) {
      return s.store.UpdateTemplate(id, name, tags, target, widthMM, heightMM, widthPx, heightPx, elements)
  }
  ```
- `CreateTemplate` now returns `(*store.Template, error)`
- `DeleteTemplate` now returns `error` (already does, just remove Save)

- [ ] **Step 4: Update PrinterService**

In `internal/service/printers.go`:
- Change store interface embed to use `store.PrinterStore` (remove `Saveable`)
- Remove all `s.store.Save()` calls
- Add:
  ```go
  func (s *PrinterService) GetPrinter(id string) *store.PrinterConfig {
      return s.store.GetPrinter(id)
  }
  ```

- [ ] **Step 5: Update BulkService**

In `internal/service/bulk.go`:
- Change store interface embed to use `store.BulkStore` (remove `Saveable`)
- Remove all `s.store.Save()` calls
- Simplify return types (no more separate save error):
  ```go
  func (s *BulkService) Move(...) []store.BulkError { return s.store.BulkMove(...) }
  func (s *BulkService) Delete(...) ([]string, []store.BulkError) { return s.store.BulkDelete(...) }
  func (s *BulkService) AddTag(...) error { return s.store.BulkAddTag(...) }
  ```

- [ ] **Step 6: Update SearchService and ExportService**

These don't use `Saveable`, but verify they reference `store.SearchStore` / `store.ExportStore` correctly.

- [ ] **Step 7: Delete assets.go**

```bash
rm internal/service/assets.go
```

- [ ] **Step 8: Commit**

```bash
git add -A internal/service/
git commit -m "refactor(service): use store.* interfaces, remove Save() calls, delete AssetService"
```

---

## Task 5: SQLite Container Store

**Files:**
- Create: `internal/store/sqlite/containers.go`
- Create: `internal/store/sqlite/containers_test.go`

- [ ] **Step 1: Write container CRUD tests**

Test cases (table-driven):
- `TestCreateContainer` — create root container (empty parentID), create child
- `TestGetContainer` — found, not found returns nil
- `TestUpdateContainer` — updates fields, returns error for missing ID
- `TestDeleteContainer` — returns parentID, error if has children, error if has items
- `TestContainerChildren` — returns only direct children
- `TestContainerItems` — returns items in container
- `TestContainerPath` — returns root-to-leaf path
- `TestAllContainers` — returns all
- `TestMoveContainer` — moves, cycle detection error

Each test uses `testStore(t)` from `sqlite_test.go`.

Container `TagIDs` must be populated via LEFT JOIN on `container_tags`.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/store/sqlite/ -run TestContainer -v -count=1
```

Expected: FAIL — methods not defined.

- [ ] **Step 3: Implement containers.go**

Implement all `ContainerStore` interface methods. Key patterns:
- `CreateContainer`: `INSERT INTO containers (...) VALUES (...)`, generate UUID, return `*store.Container`
- `GetContainer`: `SELECT ... FROM containers WHERE id = ?` + populate `TagIDs` via subquery `SELECT tag_id FROM container_tags WHERE container_id = ?`
- `DeleteContainer`: `DELETE FROM containers WHERE id = ? RETURNING parent_id` — check for children and items first
- `ContainerPath`: recursive CTE `WITH RECURSIVE path AS (...)`
- `MoveContainer`: cycle detection via CTE before update

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/store/sqlite/ -run TestContainer -v -count=1
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite/containers.go internal/store/sqlite/containers_test.go
git commit -m "feat(store): SQLite ContainerStore implementation with tests"
```

---

## Task 6: SQLite Item Store

**Files:**
- Create: `internal/store/sqlite/items.go`
- Create: `internal/store/sqlite/items_test.go`

- [ ] **Step 1: Write item CRUD tests**

Test cases:
- `TestCreateItem` — creates item with container, quantity default 1
- `TestGetItem` — found, not found
- `TestUpdateItem` — updates fields, not found error
- `TestDeleteItem` — returns containerID, not found error
- `TestMoveItem` — moves to different container, invalid container error

Item `TagIDs` populated via LEFT JOIN on `item_tags`.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/store/sqlite/ -run TestItem -v -count=1
```

- [ ] **Step 3: Implement items.go**

All `ItemStore` interface methods. Same patterns as containers.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/store/sqlite/ -run TestItem -v -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite/items.go internal/store/sqlite/items_test.go
git commit -m "feat(store): SQLite ItemStore implementation with tests"
```

---

## Task 7: SQLite Tag Store

**Files:**
- Create: `internal/store/sqlite/tags.go`
- Create: `internal/store/sqlite/tags_test.go`

- [ ] **Step 1: Write tag tests**

Test cases:
- `TestCreateTag` — root tag, child tag
- `TestGetTag` — found, not found
- `TestUpdateTag` — updates name/color/icon
- `TestDeleteTag` — returns parentID, error if has children, CASCADE removes from junction tables
- `TestMoveTag` — moves, cycle detection
- `TestTagChildren`, `TestTagPath`, `TestTagDescendants`
- `TestAllTags`
- `TestAddItemTag`, `TestRemoveItemTag` — junction table ops
- `TestAddContainerTag`, `TestRemoveContainerTag`
- `TestItemsByTag`, `TestContainersByTag` — query via junction
- `TestResolveTagIDs` — multiple IDs, some missing
- `TestTagItemStats` — count items and sum quantity across tag descendants
- `TestDeleteTag_CascadesJunctions` — delete tag → junction rows gone

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/store/sqlite/ -run TestTag -v -count=1
```

- [ ] **Step 3: Implement tags.go**

Implement all `TagStore` interface methods including new ones:
- `ResolveTagIDs`: `SELECT * FROM tags WHERE id IN (?, ?, ...)`
- `TagItemStats`: `SELECT COUNT(*), COALESCE(SUM(i.quantity), 0) FROM item_tags it JOIN items i ON i.id = it.item_id WHERE it.tag_id IN (SELECT id FROM tag_descendants_cte)`
- `TagDescendants`: recursive CTE
- `DeleteTag`: verify no children, then `DELETE ... RETURNING parent_id` (CASCADE handles junctions)

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/store/sqlite/ -run TestTag -v -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite/tags.go internal/store/sqlite/tags_test.go
git commit -m "feat(store): SQLite TagStore implementation with ResolveTagIDs, TagItemStats"
```

---

## Task 8: SQLite Printer and Template Stores

**Files:**
- Create: `internal/store/sqlite/printers.go`
- Create: `internal/store/sqlite/printers_test.go`
- Create: `internal/store/sqlite/templates.go`
- Create: `internal/store/sqlite/templates_test.go`

- [ ] **Step 1: Write printer tests**

Test cases:
- `TestAddPrinter` — creates with UUID
- `TestGetPrinter` — found, not found returns nil
- `TestAllPrinters`
- `TestDeletePrinter` — deletes, not found error
- `TestUpdatePrinterOffset`

- [ ] **Step 2: Write template tests**

Test cases:
- `TestCreateTemplate` — returns `(*Template, error)` with generated ID
- `TestGetTemplate` — found, not found
- `TestUpdateTemplate` — updates all fields, sets `updated_at`, not found error
- `TestDeleteTemplate` — returns error, not found error
- `TestAllTemplates`

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/store/sqlite/ -run "TestPrinter|TestTemplate" -v -count=1
```

- [ ] **Step 4: Implement printers.go and templates.go**

Straightforward CRUD. Templates store `tags` and `elements` as JSON TEXT columns.

`CreateTemplate` generates UUID, sets `created_at` and `updated_at`, returns `(*store.Template, error)`.
`UpdateTemplate` sets `updated_at = datetime('now')`, returns `(*store.Template, error)`.

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/store/sqlite/ -run "TestPrinter|TestTemplate" -v -count=1
```

- [ ] **Step 6: Commit**

```bash
git add internal/store/sqlite/printers.go internal/store/sqlite/printers_test.go internal/store/sqlite/templates.go internal/store/sqlite/templates_test.go
git commit -m "feat(store): SQLite PrinterStore and TemplateStore with tests"
```

---

## Task 9: SQLite Bulk, Search, and Export Stores

**Files:**
- Create: `internal/store/sqlite/bulk.go`
- Create: `internal/store/sqlite/bulk_test.go`
- Create: `internal/store/sqlite/search.go`
- Create: `internal/store/sqlite/search_test.go`
- Create: `internal/store/sqlite/export.go`
- Create: `internal/store/sqlite/export_test.go`

- [ ] **Step 1: Write bulk tests**

Test cases:
- `TestBulkMove` — moves items and containers, returns per-entity errors
- `TestBulkDelete` — deletes items and containers, returns deleted IDs, per-entity errors for containers with children
- `TestBulkAddTag` — adds tag to items and containers

Bulk ops should use a transaction internally.

- [ ] **Step 2: Write search tests**

Test cases:
- `TestSearchItems_ByName` — FTS5 match on item name
- `TestSearchItems_ByDescription` — FTS5 match on description
- `TestSearchItems_PrefixMatch` — `foo*` matches `foobar`
- `TestSearchContainers_ByName`
- `TestSearchContainers_ByDescription`
- `TestSearchTags_ByName` — LIKE match (not FTS5)
- `TestSearch_EmptyQuery` — returns empty slices
- `TestSearch_NoResults`

- [ ] **Step 3: Write export tests**

Test cases:
- `TestExportData` — returns maps of all containers and items
- `TestAllItems`, `TestAllContainers` — flat lists

- [ ] **Step 4: Run all tests to verify they fail**

```bash
go test ./internal/store/sqlite/ -run "TestBulk|TestSearch|TestExport" -v -count=1
```

- [ ] **Step 5: Implement bulk.go**

Use `db.BeginTx()` for atomicity. Call individual move/delete methods within the transaction.

- [ ] **Step 6: Implement search.go**

FTS5 query for items and containers:
```sql
SELECT i.* FROM items i
JOIN items_fts ON items_fts.rowid = i.rowid
WHERE items_fts MATCH ?
ORDER BY rank
```

Tags: `SELECT * FROM tags WHERE name LIKE '%' || ? || '%'` (case-insensitive via COLLATE NOCASE or SQLite default).

Populate `TagIDs` on returned items/containers.

- [ ] **Step 7: Implement export.go**

`ExportData`: `SELECT * FROM containers` + `SELECT * FROM items`, build maps.
`AllItems` / `AllContainers`: flat list queries. Populate `TagIDs`.

- [ ] **Step 8: Run tests to verify they pass**

```bash
go test ./internal/store/sqlite/ -run "TestBulk|TestSearch|TestExport" -v -count=1
```

- [ ] **Step 9: Commit**

```bash
git add internal/store/sqlite/bulk.go internal/store/sqlite/bulk_test.go internal/store/sqlite/search.go internal/store/sqlite/search_test.go internal/store/sqlite/export.go internal/store/sqlite/export_test.go
git commit -m "feat(store): SQLite BulkStore, SearchStore (FTS5), ExportStore with tests"
```

---

## Task 10: JSON → SQLite Auto-Migration

**Files:**
- Create: `internal/store/sqlite/migrate_json.go`
- Create: `internal/store/sqlite/migrate_json_test.go`
- Create: `internal/store/sqlite/testdata/legacy_v0.json`
- Create: `internal/store/sqlite/testdata/partitioned/meta.json`
- Create: `internal/store/sqlite/testdata/partitioned/containers.json`
- Create: `internal/store/sqlite/testdata/partitioned/items.json`
- Create: `internal/store/sqlite/testdata/partitioned/tags.json`
- Create: `internal/store/sqlite/testdata/partitioned/printers.json`
- Create: `internal/store/sqlite/testdata/partitioned/templates.json`

- [ ] **Step 1: Create test fixtures**

Create realistic test fixtures:
- `legacy_v0.json`: old format with `"version": 0`, no tags, no color/icon
- `partitioned/`: current format with `meta.json` marker + individual collection JSONs

Include hierarchical data (nested containers, tags with parents, items with tag_ids).

- [ ] **Step 2: Write migration tests**

Test cases:
- `TestMigrateJSON_Partitioned` — loads partitioned JSON, verifies data in SQLite, checks `.migrated` files exist
- `TestMigrateJSON_LegacyV0` — loads v0 JSON, verifies V0→V1→V2 migration ran + data in SQLite
- `TestMigrateJSON_AlreadyMigrated` — DB not empty → skip import
- `TestMigrateJSON_NoJSONFiles` — fresh install → skip import
- `TestMigrateJSON_BackupNotOverwritten` — `.migrated` already exists → skip rename

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/store/sqlite/ -run TestMigrateJSON -v -count=1
```

- [ ] **Step 4: Implement migrate_json.go**

Key function: `migrateFromJSON(db *sql.DB, dataDir string) error`

Logic:
1. Detect JSON files (check `meta.json` or `data.json`)
2. Check if DB is empty (`SELECT COUNT(*) FROM containers`)
3. Load JSON (partitioned or legacy with V0→V1→V2 in-memory migration)
4. Topological sort for hierarchical inserts (tags parents first, containers parents first)
5. Single transaction: INSERT all data
6. Rename JSON files to `.migrated`

Copy relevant migration functions from old `migrate.go` for legacy support (the V0→V1 and V1→V2 transforms). This is the only place legacy migration logic is needed.

- [ ] **Step 5: Wire into New()**

Call `migrateFromJSON` in `New()` after goose migrations, before returning the store.

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/store/sqlite/ -run TestMigrateJSON -v -count=1
```

- [ ] **Step 7: Run full SQLite test suite**

```bash
go test ./internal/store/sqlite/ -v -count=1
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/store/sqlite/migrate_json.go internal/store/sqlite/migrate_json_test.go internal/store/sqlite/testdata/
git commit -m "feat(store): auto-migrate JSON to SQLite at startup with test fixtures"
```

---

## Task 11: Handler & API Cleanup

**Files:**
- Modify: `internal/handler/responder.go` — add `RenderPartial` to interface
- Modify: `internal/handler/containers.go` — use interface `RenderPartial`, 204 on DELETE, remove `ItemsJSON`, use returned parentID
- Modify: `internal/handler/items.go` — use interface `RenderPartial`, 204 on DELETE, use returned containerID
- Modify: `internal/handler/tags.go` — use interface `RenderPartial`, 204 on DELETE, use returned parentID
- Modify: `internal/handler/templates.go` — use `UpdateTemplate`, POST returns 201
- Modify: `internal/handler/print.go` — use `TagService.ResolveTagIDs`, remove `resolveTags`
- Modify: `internal/handler/debug.go` — use `GetPrinter`
- Modify: `internal/shared/webutil/errors.go` — add `WriteError`
- Delete: `internal/handler/assets.go`

- [ ] **Step 1: Add `RenderPartial` to `Responder` interface**

In `internal/handler/responder.go`, add to interface:

```go
type Responder interface {
    Respond(w http.ResponseWriter, r *http.Request, status int, data any, tmpl string, vmFn func() any)
    RespondError(w http.ResponseWriter, r *http.Request, err error)
    Redirect(w http.ResponseWriter, r *http.Request, url string, jsonData any)
    RenderPartial(w http.ResponseWriter, r *http.Request, tmpl, define string, data any) bool
}
```

Add no-op to `JSONResponder`:
```go
func (j *JSONResponder) RenderPartial(w http.ResponseWriter, r *http.Request, tmpl, define string, data any) bool {
    return false
}
```

`HTMLResponder.RenderPartial` already exists (line 90-105).

- [ ] **Step 2: Add `WriteError` to webutil**

In `internal/shared/webutil/errors.go`:

```go
func WriteError(w http.ResponseWriter, status int, err error) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
```

- [ ] **Step 3: Update containers.go**

- Replace type-assert pattern (lines 89-94) with:
  ```go
  if h.resp.RenderPartial(w, r, "containers", "container-list-item", container) {
      return
  }
  ```
- `Delete` (lines 122-141): remove Get-before-Delete, use returned parentID:
  ```go
  parentID, err := h.inventory.DeleteContainer(id)
  if err != nil { h.resp.RespondError(w, r, err); return }
  ```
- Change Delete response to 204:
  ```go
  h.resp.Respond(w, r, http.StatusNoContent, nil, "containers", func() any { ... })
  ```
- Remove `ItemsJSON` method and route registration (line 33, lines 162-175)

- [ ] **Step 4: Update items.go**

- Replace type-assert (lines 76-81) with `h.resp.RenderPartial(...)`
- `Delete` (lines 114-136): remove Get-before-Delete, use returned containerID, 204 status
- Remove redundant container_id check (lines 60-63) — service layer validates

- [ ] **Step 5: Update tags.go**

- Replace type-assert with `h.resp.RenderPartial(...)`
- `Delete`: use returned parentID, 204 status

- [ ] **Step 6: Update templates.go**

Replace `Save` handler's update path (lines 169-195):
```go
// Instead of mutating tmpl fields and calling SaveTemplate:
tmpl, err := h.templates.UpdateTemplate(id, req.Name, req.Tags, req.Target, req.WidthMM, req.HeightMM, req.WidthPx, req.HeightPx, req.Elements)
```

Create path returns 201 with created object instead of `{"ok": true}`.

- [ ] **Step 7: Update print.go — replace resolveTags**

Remove `resolveTags` method (lines 36-53). Use `h.tags.ResolveTagIDs(tagIDs)` and map to `label.LabelTag`:

```go
func (h *PrintHandler) resolveTags(tagIDs []string) []label.LabelTag {
    tags := h.tags.ResolveTagIDs(tagIDs)
    var result []label.LabelTag
    for _, tag := range tags {
        tagPath := h.tags.TagPath(tag.ID)
        pathNames := make([]string, len(tagPath))
        for i, t := range tagPath {
            pathNames[i] = t.Name
        }
        result = append(result, label.LabelTag{Name: tag.Name, Icon: tag.Icon, Path: pathNames})
    }
    return result
}
```

Note: This still calls `TagPath` per tag. This can be optimized later but preserves behavior.

- [ ] **Step 8: Update debug.go — use GetPrinter**

Replace linear scan (lines 62-69):
```go
cfg := h.printers.GetPrinter(id)
if cfg == nil {
    webutil.WriteError(w, http.StatusNotFound, store.ErrPrinterNotFound)
    return
}
```

- [ ] **Step 9: Delete handler/assets.go**

```bash
rm internal/handler/assets.go
```

- [ ] **Step 10: Commit**

```bash
git add -A internal/handler/ internal/shared/webutil/
git commit -m "refactor(handler): RenderPartial interface, 204 on DELETE, remove assets, unified WriteError"
```

---

## Task 12: Wire Everything — Composition Root and Main

**Files:**
- Modify: `internal/app/server.go`
- Modify: `cmd/qlx/main.go`

- [ ] **Step 1: Update server.go**

- Import `sqlite` package
- Replace `store.NewStore(path)` with `sqlite.New(dataDir)`
- Remove `resolveTags` closure (lines 41-49 of `server.go`). Replace with `tagService.ResolveTagIDs`:
  ```go
  resolveTagsFn := tagService.ResolveTagIDs
  tmplMap := handler.LoadTemplates(resolveTagsFn)
  ```
  Note: `LoadTemplates` signature is `func(func([]string) []store.Tag) map[string]*template.Template` — `tagService.ResolveTagIDs` already matches this signature (`func([]string) []store.Tag`).
- Remove `AssetService` creation and `AssetHandler` registration
- Remove asset route registrations
- `Shutdown()` calls `db.Close()` instead of `store.Save()`
- Remove all `Saveable` references from service constructor calls

- [ ] **Step 2: Update main.go**

- Pass `dataDir` to `app.NewServer()` config (remove `filepath.Join(dataDir, "data.json")`)
- Remove `store.NewStore()` call
- Remove any `Save()` call in shutdown sequence

- [ ] **Step 3: Verify Mac build**

```bash
make build-mac
```

Expected: compiles without errors.

- [ ] **Step 4: Verify MIPS build**

```bash
make build-mips
```

Expected: compiles. MIPS build uses `-tags minimal` which excludes BLE. Verify that SQLite isn't pulled in transitively for the minimal build. If it is (because `app/server.go` imports `sqlite`), add build-tag gating:
- The MIPS agent build doesn't use a local store (it's a dumb remote agent)
- If needed, gate the `sqlite` import with `//go:build !minimal` or make the store initialization conditional

- [ ] **Step 5: Commit**

```bash
git add internal/app/server.go cmd/qlx/main.go
git commit -m "feat: wire SQLite store into composition root, update main.go"
```

---

## Task 13: Update Service, App, and Print Tests

**Files:**
- Modify: `internal/service/inventory_test.go`
- Modify: `internal/service/tags_test.go`
- Modify: `internal/service/templates_test.go`
- Modify: `internal/service/bulk_test.go`
- Modify: `internal/app/server_test.go` (uses `store.NewMemoryStore()`)
- Modify: `internal/print/manager_test.go` (uses `store.NewMemoryStore()` x4)

- [ ] **Step 1: Update test helper**

Replace `store.NewMemoryStore()` with `sqlite.New(":memory:")` in ALL test files that reference it:
- `internal/service/*_test.go` (4 files)
- `internal/app/server_test.go` (2 usages)
- `internal/print/manager_test.go` (4 usages)

Create a shared helper in each test package:

```go
func testSQLiteStore(t *testing.T) *sqlite.SQLiteStore {
    t.Helper()
    db, err := sqlite.New(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { db.Close() })
    return db
}
```

- [ ] **Step 2: Remove Save() assertions**

Remove any test assertions that check `Save()` was called or that verify on-disk persistence.

- [ ] **Step 3: Update Delete test assertions**

Tests that call `DeleteContainer`, `DeleteItem`, `DeleteTag` now receive `(string, error)` instead of `error`. Update to capture the returned parent/container ID.

- [ ] **Step 4: Update template test assertions**

Tests that call `SaveTemplate` should use `UpdateTemplate` instead. Tests for `CreateTemplate` now check `(*Template, error)` return.

- [ ] **Step 5: Update server_test.go**

`internal/app/server_test.go` passes `store.NewMemoryStore()` to `NewServer`. The `NewServer` signature changed to accept a config with `DataDir`. Update to use `sqlite.New(t.TempDir())` or adjust `NewServer` call to match new signature.

- [ ] **Step 6: Update manager_test.go**

`internal/print/manager_test.go` creates stores for printer manager tests. Replace all 4 `store.NewMemoryStore()` calls with `sqlite.New(":memory:")`.

- [ ] **Step 7: Run all affected tests**

```bash
go test ./internal/service/ ./internal/app/ ./internal/print/ -v -count=1
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/service/*_test.go internal/app/server_test.go internal/print/manager_test.go
git commit -m "test(service): update tests for SQLite store — remove Save(), update signatures"
```

---

## Task 14: Update Handler Tests

**Files:**
- Modify: `internal/handler/containers_test.go`
- Modify: `internal/handler/items_test.go`
- Modify: `internal/handler/tags_test.go`
- Modify: `internal/handler/print_test.go`
- Modify: `internal/handler/adhoc_test.go`
- Modify: `internal/handler/debug_test.go`
- Modify: `internal/handler/responder_test.go`
- Modify: `internal/handler/templates_test.go` (if exists — check `templates.go` test coverage)

- [ ] **Step 1: Update handler test setup**

Replace `store.NewMemoryStore()` with `sqlite.New(":memory:")` in test helpers.

- [ ] **Step 2: Update DELETE assertions**

Tests expecting `200 {"ok":true}` on DELETE should now expect `204 No Content`.

- [ ] **Step 3: Update template test assertions**

Tests for template save/create should match new signatures and response codes.

- [ ] **Step 4: Remove ItemsJSON test**

Remove any test for `GET /containers/{id}/items-json`.

- [ ] **Step 5: Run handler tests**

```bash
go test ./internal/handler/ -v -count=1
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/*_test.go
git commit -m "test(handler): update tests for SQLite store, 204 DELETEs, removed endpoints"
```

---

## Task 15: Full Build and Test Suite

**Files:** None (verification only)

- [ ] **Step 1: Run full test suite**

```bash
make test
```

Expected: all PASS.

- [ ] **Step 2: Run lint**

```bash
make lint
```

Expected: no errors.

- [ ] **Step 3: Build all targets**

```bash
make build-mac
```

Expected: compiles without errors.

- [ ] **Step 4: Manual smoke test**

```bash
make run
```

Verify:
1. Server starts, creates `data/qlx.db`
2. Can create containers, items, tags via UI
3. Search works (type in search box)
4. Print flow works (if printer available)

- [ ] **Step 5: Test JSON migration**

If you have existing `data/` with JSON files:
1. Back up `data/`
2. Run `make run`
3. Verify data appears in UI
4. Verify `*.migrated` files exist
5. Verify `qlx.db` exists

- [ ] **Step 6: Run E2E tests**

```bash
make test-e2e
```

Expected: all PASS.

- [ ] **Step 7: Commit any remaining fixes**

If any tests needed fixing, commit.

---

## Task 16: Cleanup and Final Commit

**Files:**
- Update: `.gitignore` — add `*.db`, `*.db-wal`, `*.db-shm`

- [ ] **Step 1: Update .gitignore**

Add SQLite-specific entries:

```
*.db
*.db-wal
*.db-shm
```

- [ ] **Step 2: Verify no leftover references**

```bash
grep -r "NewMemoryStore\|store\.Save\|Saveable\|AssetStore\|AssetService\|AssetHandler\|assets\.go\|items-json" internal/ --include="*.go" | grep -v _test.go | grep -v sqlite/migrate_json.go
```

Expected: no results (all old references removed).

- [ ] **Step 3: Final commit**

```bash
git add -A
git commit -m "chore: cleanup .gitignore, verify no leftover JSON store references"
```
