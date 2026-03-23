# SQLite Store Migration Design

> GitHub Issue: #74 — refactor: rewrite store layer with SQLite backend

## Overview

Replace the JSON file store with SQLite, adding FTS5 full-text search, proper FK constraints, and automatic migration from existing JSON data. Bundle with handler/API cleanup to eliminate deduplication and inconsistencies.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| JSON → SQLite migration | Automatic at startup | Single-user appliance, zero manual intervention |
| `Save()` / `Saveable` | Remove entirely | SQLite mutations are atomic; no dirty-flag pattern needed |
| Package structure | Split: `store/` (models/errors) + `store/sqlite/` (impl) | Future backend swap possible (Postgres, etc.) |
| Full-text search | FTS5 from day one | Issue requires it; triggers are trivial in migration SQL |
| Asset storage | Binary files on disk, metadata in SQLite | Large images don't belong in DB; simpler backup |
| Pagination/sorting | Follow-up PR | Scope is already large; SQLite makes it trivial later |

## Package Structure

```
internal/store/
  models.go            # Container, Item, Tag, PrinterConfig, Template, Asset
  errors.go            # ErrContainerNotFound, ErrItemNotFound, etc.
  store.go             # Store interface (aggregate of all sub-interfaces)
  sqlite/
    sqlite.go          # SQLiteStore struct, New(), Close(), pragma setup
    migrations.go      # go:embed for SQL migration files
    migrate_json.go    # Auto-import JSON → SQLite at startup
    containers.go      # ContainerStore implementation
    items.go           # ItemStore implementation
    tags.go            # TagStore implementation
    printers.go        # PrinterStore implementation
    templates.go       # TemplateStore implementation
    assets.go          # AssetStore implementation
    bulk.go            # BulkStore implementation (transactions)
    search.go          # SearchStore implementation (FTS5)
    export.go          # ExportStore implementation
    migrations/
      001_initial_schema.sql
      002_fts5_indexes.sql
```

## Store Interface

```go
// store/store.go
type Store interface {
    ContainerStore
    ItemStore
    TagStore
    BulkStore
    SearchStore
    PrinterStore
    TemplateStore
    AssetStore
    ExportStore
    Close() error
}
```

`Saveable` is removed. `Close()` replaces it (closes DB connection).

## SQL Schema

### Migration 001: Initial Schema

```sql
CREATE TABLE tags (
    id          TEXT PRIMARY KEY,
    parent_id   TEXT REFERENCES tags(id),
    name        TEXT NOT NULL,
    color       TEXT NOT NULL DEFAULT '',
    icon        TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE containers (
    id          TEXT PRIMARY KEY,
    parent_id   TEXT REFERENCES containers(id),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    color       TEXT NOT NULL DEFAULT '',
    icon        TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE items (
    id           TEXT PRIMARY KEY,
    container_id TEXT NOT NULL REFERENCES containers(id),
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    quantity     INTEGER NOT NULL DEFAULT 1,
    color        TEXT NOT NULL DEFAULT '',
    icon         TEXT NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE item_tags (
    item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    tag_id  TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (item_id, tag_id)
);

CREATE TABLE container_tags (
    container_id TEXT NOT NULL REFERENCES containers(id) ON DELETE CASCADE,
    tag_id       TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (container_id, tag_id)
);

CREATE TABLE printer_configs (
    id        TEXT PRIMARY KEY,
    name      TEXT NOT NULL,
    encoder   TEXT NOT NULL,
    model     TEXT NOT NULL,
    transport TEXT NOT NULL,
    address   TEXT NOT NULL DEFAULT '',
    offset_x  INTEGER NOT NULL DEFAULT 0,
    offset_y  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE templates (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    tags       TEXT NOT NULL DEFAULT '[]',
    target     TEXT NOT NULL DEFAULT 'universal',
    width_mm   REAL NOT NULL DEFAULT 0,
    height_mm  REAL NOT NULL DEFAULT 0,
    width_px   INTEGER NOT NULL DEFAULT 0,
    height_px  INTEGER NOT NULL DEFAULT 0,
    elements   TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE assets (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    mime_type  TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_containers_parent  ON containers(parent_id);
CREATE INDEX idx_items_container    ON items(container_id);
CREATE INDEX idx_tags_parent        ON tags(parent_id);
CREATE INDEX idx_item_tags_tag      ON item_tags(tag_id);
CREATE INDEX idx_container_tags_tag ON container_tags(tag_id);
```

### Migration 002: FTS5 Indexes

```sql
CREATE VIRTUAL TABLE items_fts USING fts5(
    name, description, content=items, content_rowid=rowid
);

CREATE VIRTUAL TABLE containers_fts USING fts5(
    name, description, content=containers, content_rowid=rowid
);

-- Items FTS sync triggers
CREATE TRIGGER items_ai AFTER INSERT ON items BEGIN
    INSERT INTO items_fts(rowid, name, description)
    VALUES (new.rowid, new.name, new.description);
END;

CREATE TRIGGER items_ad AFTER DELETE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, name, description)
    VALUES ('delete', old.rowid, old.name, old.description);
END;

CREATE TRIGGER items_au AFTER UPDATE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, name, description)
    VALUES ('delete', old.rowid, old.name, old.description);
    INSERT INTO items_fts(rowid, name, description)
    VALUES (new.rowid, new.name, new.description);
END;

-- Containers FTS sync triggers
CREATE TRIGGER containers_ai AFTER INSERT ON containers BEGIN
    INSERT INTO containers_fts(rowid, name, description)
    VALUES (new.rowid, new.name, new.description);
END;

CREATE TRIGGER containers_ad AFTER DELETE ON containers BEGIN
    INSERT INTO containers_fts(containers_fts, rowid, name, description)
    VALUES ('delete', old.rowid, old.name, old.description);
END;

CREATE TRIGGER containers_au AFTER UPDATE ON containers BEGIN
    INSERT INTO containers_fts(containers_fts, rowid, name, description)
    VALUES ('delete', old.rowid, old.name, old.description);
    INSERT INTO containers_fts(rowid, name, description)
    VALUES (new.rowid, new.name, new.description);
END;
```

External content FTS — no data duplication. Queries join via implicit `rowid`:

```sql
SELECT i.* FROM items i
JOIN items_fts ON items_fts.rowid = i.rowid
WHERE items_fts MATCH ?
ORDER BY rank;
```

**Important**: Tables with TEXT PRIMARY KEY still have an implicit integer `rowid`. Do NOT use `WITHOUT ROWID` on `items` or `containers` — it would break the FTS join. Avoid `VACUUM` (which can renumber rowids and desync external-content FTS) — use `PRAGMA auto_vacuum = INCREMENTAL` instead.

Tags searched via simple `LIKE` — small dataset, FTS overkill.

## Auto-Migration JSON → SQLite

### Detection and flow

```
1. Check: dataDir/meta.json OR dataDir/data.json exists?
2. No  → fresh install, skip
3. Yes → SELECT COUNT(*) FROM containers
4. DB empty → import JSON data, backup files
5. DB non-empty → skip (already migrated)
```

### Import order (FK constraints)

1. `tags` — topological sort, parents before children
2. `containers` — topological sort, parents before children
3. `items` — after containers
4. `item_tags`, `container_tags` — after items/containers/tags (from TagIDs on models)
5. `printer_configs`, `templates`, `assets` — independent

### Legacy data.json support

If `data.json` found instead of partitioned files: run V0→V1→V2 migrations in memory (copy existing `migrateIfNeeded` logic), then import result into SQLite.

### Backup policy

JSON files renamed to `*.migrated` (e.g., `containers.json.migrated`). Not deleted. If `.migrated` already exists, skip rename (don't overwrite backup).

## Interface Changes

### Removed

```go
// DELETED
type Saveable interface {
    Save() error
}
```

### Modified signatures

```go
// Delete now returns parent reference — eliminates double-lookup in handlers
DeleteContainer(id string) (parentID string, err error)
DeleteItem(id string) (containerID string, err error)
DeleteTag(id string) (parentID string, err error)
```

### New methods

```go
// TagStore — replaces 3 copies of resolveTags
ResolveTagIDs(ids []string) []Tag

// TagStore — replaces inline SUM(quantity) in handler
TagItemStats(id string) (itemCount int, totalQty int, err error)

// PrinterStore — replaces linear scan in DebugHandler
GetPrinter(id string) *PrinterConfig

// TemplateStore — separates create from update, handler stops mutating domain objects
CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*Template, error)
UpdateTemplate(id string, name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*Template, error)
DeleteTemplate(id string) error  // now returns error (SQLite deletes can fail)
```

### TagIDs on models — computed field

`TagIDs []string` stays on `Container` and `Item` structs to preserve JSON API contract (`tag_ids` in responses). It is no longer stored — instead, it is populated at query time via a LEFT JOIN on the junction table. Store methods that return `Container` or `Item` always populate `TagIDs`. The field is ignored on write (tags are managed via `AddItemTag`/`RemoveItemTag`).

## Handler & API Cleanup

### Unified error responses

Single helper replaces 3 JSON error shapes:

```go
func WriteError(w http.ResponseWriter, status int, err error) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
```

Bulk ops keep `{"errors": [...]}` for partial failures — only exception.

### Status code fixes

| Operation | Before | After |
|-----------|--------|-------|
| DELETE success | 200 `{"ok":true}` | 204 No Content |
| POST /assets | 200 | 201 |
| POST /templates | 200 `{"ok":true}` | 201 + created object |

### `RenderPartial` on `Responder` interface

```go
type Responder interface {
    Respond(w, r, status, data, template, vmFn)
    RespondError(w, r, err)
    RenderPartial(w, r, template, partial string, data any) bool
}
```

- `HTMLResponder.RenderPartial` — renders fragment, returns `true`
- `JSONResponder.RenderPartial` — returns `false` (handler continues normal JSON path)

Eliminates `h.resp.(*HTMLResponder)` type-assert in 3 handlers.

### Endpoint removals

- **`GET /containers/{id}/items-json`** — removed. Client uses `Accept: application/json` on existing `/containers/{id}/items` endpoint.

### Logic moved to services

- **Template dispatch** (schema vs designer) → `TemplateService.ResolveTemplate(id)` — shared by `PrintHandler` and `AdhocHandler`
- **USB dedup** → `transport.ScanUSB()` returns deduplicated results
- **`resolveTags` closure** in `server.go` → `TagService.ResolveTagIDs()`

## Composition Root Changes

### `app/server.go`

```go
func NewServer(cfg Config) (*Server, error) {
    db, err := sqlite.New(cfg.DataDir)
    if err != nil { return nil, err }

    inventory := service.NewInventoryService(db)
    tags := service.NewTagService(db)
    // ... all services, no Saveable in interfaces ...

    return &Server{db: db, ...}, nil
}

func (s *Server) Shutdown() error {
    return s.db.Close()  // replaces s.store.Save()
}
```

### `cmd/qlx/main.go`

- `--data` flag still points to data dir (now contains `qlx.db` + `assets/`)
- `Save()` removed from graceful shutdown — replaced by `Close()`
- MIPS build (`-tags minimal`) — no store, no changes

### Data directory layout

```
Before:                     After:
data/                       data/
  meta.json                   qlx.db
  containers.json             assets/
  items.json                    {id}.bin
  tags.json                   containers.json.migrated
  printers.json               items.json.migrated
  templates.json               ...
  assets.json
  assets/
    {id}.bin
```

## Testing Strategy

### Store unit tests

Each `sqlite/*.go` gets a corresponding `_test.go`. Pattern:

```go
func testStore(t *testing.T) *SQLiteStore {
    db, err := New(t.TempDir())  // real file, WAL, FK constraints
    if err != nil { t.Fatal(err) }
    t.Cleanup(func() { db.Close() })
    return db
}
```

`t.TempDir()` for store tests (real I/O). `:memory:` for service tests (speed).

### Coverage areas

| Area | Tests |
|------|-------|
| CRUD | Create, Get, Update, Delete + returned parentID |
| Hierarchy | ContainerChildren, ContainerPath, TagDescendants, Move |
| Junction tables | AddItemTag, RemoveItemTag, cascade on DeleteTag |
| FTS5 | Search by name, description, prefix match, ranking |
| Bulk | BulkMove, BulkDelete in transaction, partial failure |
| JSON import | Load fixtures → migrate → verify data in DB |
| FK constraints | Delete container with items → error, delete tag → cascade |
| Edge cases | Empty strings, NULL parent_id, duplicate tags |

### JSON migration test fixtures

```
internal/store/sqlite/testdata/
  legacy_v0.json
  partitioned/
    meta.json
    containers.json
    items.json
    tags.json
    printers.json
    templates.json
    assets.json
```

### Service tests

Replace `NewMemoryStore()` with `sqlite.New(":memory:")`. Changes: remove `Save()` calls, update Delete signatures.

### E2E tests (Playwright)

No test changes. Fixture starts server with `--data` temp dir. Server creates `qlx.db` instead of JSONs. If E2E tests pass, migration is correct.

### No external test frameworks

Standard library only: table-driven tests, `httptest`, `t.Fatal`/`t.Errorf`.

## Dependencies

### New

- `github.com/ncruces/go-sqlite3` — pure Go SQLite (WASM/wazero, no CGO)
- `github.com/pressly/goose/v3` — SQL migrations with `embed.FS` support

### Removed

None — existing deps stay.

## SQLite Runtime Config

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;
PRAGMA cache_size = -4096;   -- 4MB (conservative for potential embedded use)
PRAGMA foreign_keys = ON;
PRAGMA temp_store = MEMORY;
PRAGMA auto_vacuum = INCREMENTAL;  -- safe rowid-preserving alternative to VACUUM
```

## Implementation Notes

### Asset storage atomicity

`SaveAsset` writes metadata to SQLite and binary to `{dataDir}/assets/{id}.bin`. Order: write file first, then INSERT metadata. On file write failure, no DB row is created. On DB insert failure after file write, the orphan file is cleaned up. `GetAsset` returns metadata; `AssetData` reads from disk.

### ExportData stays unchanged

`ExportData() (map[string]*Container, map[string]*Item)` loads everything into memory. Acceptable for current scale (single-user appliance). Streaming export is out of scope.

## Out of Scope

- Pagination / sorting — follow-up PR
- HTML error partials (replacing `http.Error` in HTMX flows) — follow-up PR
- `PrintContext` embedded struct for view models — nice-to-have
- Dual-backend abstraction — not needed (MIPS build has no store)
