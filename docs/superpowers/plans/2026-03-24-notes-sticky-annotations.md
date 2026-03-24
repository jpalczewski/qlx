# Notes (Sticky Annotations) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add note entities (sticky annotations) attached to containers/items with full CRUD, FTS5 search, and print support.

**Architecture:** New `Note` model with two nullable FK columns (`container_id`, `item_id`) and a CHECK constraint ensuring exactly one is set. Full vertical slice: migration → store → service → handler. Search integrated into existing `/search` endpoint. Print delegated to existing `PrinterManager`.

**Tech Stack:** Go, SQLite (FTS5), goose migrations, httptest

---

### Task 1: Database Migration

**Files:**
- Create: `internal/store/sqlite/migrations/003_notes.sql`

- [ ] **Step 1: Write the migration file**

```sql
-- +goose Up

CREATE TABLE notes (
    id           TEXT PRIMARY KEY,
    container_id TEXT REFERENCES containers(id) ON DELETE CASCADE,
    item_id      TEXT REFERENCES items(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    content      TEXT NOT NULL DEFAULT '',
    color        TEXT NOT NULL DEFAULT '',
    icon         TEXT NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    CHECK ((container_id IS NOT NULL) != (item_id IS NOT NULL))
);

CREATE INDEX idx_notes_container ON notes(container_id);
CREATE INDEX idx_notes_item ON notes(item_id);

CREATE VIRTUAL TABLE notes_fts USING fts5(
    title, content, content=notes, content_rowid=rowid
);

-- +goose StatementBegin
CREATE TRIGGER notes_ai AFTER INSERT ON notes BEGIN
    INSERT INTO notes_fts(rowid, title, content)
    VALUES (new.rowid, new.title, new.content);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER notes_ad AFTER DELETE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, title, content)
    VALUES ('delete', old.rowid, old.title, old.content);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER notes_au AFTER UPDATE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, title, content)
    VALUES ('delete', old.rowid, old.title, old.content);
    INSERT INTO notes_fts(rowid, title, content)
    VALUES (new.rowid, new.title, new.content);
END;
-- +goose StatementEnd

-- +goose Down

DROP TRIGGER IF EXISTS notes_au;
DROP TRIGGER IF EXISTS notes_ad;
DROP TRIGGER IF EXISTS notes_ai;
DROP TABLE IF EXISTS notes_fts;
DROP INDEX IF EXISTS idx_notes_item;
DROP INDEX IF EXISTS idx_notes_container;
DROP TABLE IF EXISTS notes;
```

- [ ] **Step 2: Verify migration loads**

Run: `go test ./internal/store/sqlite/ -run TestNew_RunsMigrations -v`
Expected: PASS (goose picks up new migration automatically)

- [ ] **Step 3: Commit**

```bash
git add internal/store/sqlite/migrations/003_notes.sql
git commit -m "feat(migrations): add notes table with FTS5 indexes"
```

---

### Task 2: Note Model, Store Interface, and Error Mapping

**Files:**
- Modify: `internal/store/models.go` (add Note struct after Item)
- Modify: `internal/store/errors.go` (add ErrNoteNotFound)
- Modify: `internal/store/interfaces.go` (add NoteStore interface, extend SearchStore)
- Modify: `internal/store/store.go` (add NoteStore to Store)
- Modify: `internal/shared/webutil/errors.go` (add ErrNoteNotFound → 404 mapping)
- Modify: `internal/shared/webutil/errors_test.go` (add test case)

- [ ] **Step 1: Add Note struct to models.go**

After `Item` struct (line 36), add:

```go
// Note represents a sticky annotation attached to a container or item.
type Note struct {
	ID          string    `json:"id"`
	ContainerID string    `json:"container_id,omitempty"`
	ItemID      string    `json:"item_id,omitempty"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Color       string    `json:"color"`
	Icon        string    `json:"icon"`
	CreatedAt   time.Time `json:"created_at"`
}
```

- [ ] **Step 2: Add error to errors.go**

Add after `ErrTemplateNotFound`:

```go
ErrNoteNotFound = errors.New("note not found")
```

- [ ] **Step 3: Add NoteStore interface to interfaces.go**

After `ExportStore` interface, add:

```go
// NoteStore defines note-related store operations.
type NoteStore interface {
	GetNote(id string) *Note
	CreateNote(containerID, itemID, title, content, color, icon string) *Note
	UpdateNote(id, title, content, color, icon string) (*Note, error)
	DeleteNote(id string) error
	ContainerNotes(containerID string) []Note
	ItemNotes(itemID string) []Note
}
```

- [ ] **Step 4: Add SearchNotes to SearchStore**

In `SearchStore` interface, add:

```go
SearchNotes(query string) []Note
```

- [ ] **Step 5: Add NoteStore to Store composite**

In `internal/store/store.go`, add `NoteStore` before `Close() error`:

```go
type Store interface {
	ContainerStore
	ItemStore
	TagStore
	BulkStore
	SearchStore
	PrinterStore
	TemplateStore
	ExportStore
	NoteStore
	Close() error
}
```

- [ ] **Step 6: Add ErrNoteNotFound to error mapping**

In `internal/shared/webutil/errors.go`, add to `statusMap`:

```go
store.ErrNoteNotFound:         http.StatusNotFound,
```

In `internal/shared/webutil/errors_test.go`, add test case to the table:

```go
{store.ErrNoteNotFound, 404},
```

- [ ] **Step 7: Verify the webutil tests pass**

Run: `go test ./internal/shared/webutil/ -v`
Expected: PASS

- [ ] **Step 8: Verify store package compiles**

Run: `go build ./internal/store/...`
Expected: FAIL — SQLiteStore does not implement NoteStore yet (expected, confirms interface is wired)

- [ ] **Step 9: Commit**

```bash
git add internal/store/models.go internal/store/errors.go internal/store/interfaces.go internal/store/store.go internal/shared/webutil/errors.go internal/shared/webutil/errors_test.go
git commit -m "feat(store): add Note model, NoteStore interface, and error mapping"
```

---

### Task 3: SQLite Store Implementation — Tests First

**Files:**
- Create: `internal/store/sqlite/notes_test.go`
- Create: `internal/store/sqlite/notes.go`

- [ ] **Step 1: Write store tests**

```go
package sqlite

import (
	"testing"
)

func TestNoteStore_CRUD(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	item := db.CreateItem(c.ID, "Widget", "", 1, "", "")

	// Create note on container
	note := db.CreateNote(c.ID, "", "Fragile", "Handle with care", "red", "alert-triangle")
	if note == nil {
		t.Fatal("expected note, got nil")
	}
	if note.Title != "Fragile" {
		t.Errorf("got title %q, want %q", note.Title, "Fragile")
	}
	if note.ContainerID != c.ID {
		t.Errorf("got container_id %q, want %q", note.ContainerID, c.ID)
	}
	if note.ItemID != "" {
		t.Errorf("expected empty item_id, got %q", note.ItemID)
	}

	// Create note on item
	noteItem := db.CreateNote("", item.ID, "Review", "Check by 2026-04", "yellow", "clock")
	if noteItem == nil {
		t.Fatal("expected note, got nil")
	}
	if noteItem.ItemID != item.ID {
		t.Errorf("got item_id %q, want %q", noteItem.ItemID, item.ID)
	}

	// Get
	got := db.GetNote(note.ID)
	if got == nil {
		t.Fatal("expected note, got nil")
	}
	if got.Content != "Handle with care" {
		t.Errorf("got content %q, want %q", got.Content, "Handle with care")
	}

	// Update
	updated, err := db.UpdateNote(note.ID, "Very Fragile", "Handle with extreme care", "orange", "alert-triangle")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Very Fragile" {
		t.Errorf("got title %q, want %q", updated.Title, "Very Fragile")
	}

	// Delete
	if err := db.DeleteNote(note.ID); err != nil {
		t.Fatal(err)
	}
	if db.GetNote(note.ID) != nil {
		t.Error("expected nil after delete")
	}
}

func TestNoteStore_ContainerNotes(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	db.CreateNote(c.ID, "", "Note 1", "", "", "")
	db.CreateNote(c.ID, "", "Note 2", "", "", "")

	notes := db.ContainerNotes(c.ID)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	// Newest first (DESC)
	if notes[0].Title != "Note 2" {
		t.Errorf("expected newest first, got %q", notes[0].Title)
	}
}

func TestNoteStore_ItemNotes(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	item := db.CreateItem(c.ID, "Widget", "", 1, "", "")
	db.CreateNote("", item.ID, "Item Note", "", "", "")

	notes := db.ItemNotes(item.ID)
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
}

func TestNoteStore_CascadeDeleteContainer(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	note := db.CreateNote(c.ID, "", "Will vanish", "", "", "")

	// Delete container — note should cascade
	_, err := db.DeleteContainer(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if db.GetNote(note.ID) != nil {
		t.Error("expected note to be cascade-deleted with container")
	}
}

func TestNoteStore_CascadeDeleteItem(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	item := db.CreateItem(c.ID, "Widget", "", 1, "", "")
	note := db.CreateNote("", item.ID, "Will vanish", "", "", "")

	_, err := db.DeleteItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if db.GetNote(note.ID) != nil {
		t.Error("expected note to be cascade-deleted with item")
	}
}

func TestNoteStore_DeleteNotFound(t *testing.T) {
	db := testStore(t)
	err := db.DeleteNote("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNoteStore_UpdateNotFound(t *testing.T) {
	db := testStore(t)
	_, err := db.UpdateNote("nonexistent", "X", "", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/sqlite/ -run TestNoteStore -v`
Expected: FAIL — compilation error, methods not yet implemented

- [ ] **Step 3: Implement notes.go**

```go
package sqlite

import (
	"github.com/erxyi/qlx/internal/store"
	"github.com/google/uuid"
)

// scanNote scans a single row into a store.Note.
func scanNote(row interface {
	Scan(dest ...any) error
}) (store.Note, error) {
	var note store.Note
	var containerID, itemID *string
	err := row.Scan(&note.ID, &containerID, &itemID, &note.Title, &note.Content,
		&note.Color, &note.Icon, &note.CreatedAt)
	if containerID != nil {
		note.ContainerID = *containerID
	}
	if itemID != nil {
		note.ItemID = *itemID
	}
	return note, err
}

const noteSelectCols = `id, container_id, item_id, title, content, color, icon, created_at`

// GetNote returns the note with the given ID, or nil if not found.
func (s *SQLiteStore) GetNote(id string) *store.Note {
	row := s.db.QueryRow(
		`SELECT `+noteSelectCols+` FROM notes WHERE id = ?`, id)
	note, err := scanNote(row)
	if err != nil {
		return nil
	}
	return &note
}

// CreateNote inserts a new note and returns it, or nil on error.
func (s *SQLiteStore) CreateNote(containerID, itemID, title, content, color, icon string) *store.Note {
	id := uuid.New().String()
	var cID, iID *string
	if containerID != "" {
		cID = &containerID
	}
	if itemID != "" {
		iID = &itemID
	}
	_, err := s.db.Exec(
		`INSERT INTO notes (id, container_id, item_id, title, content, color, icon) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, cID, iID, title, content, color, icon)
	if err != nil {
		return nil
	}
	return s.GetNote(id)
}

// UpdateNote updates a note's mutable fields and returns the updated record.
func (s *SQLiteStore) UpdateNote(id, title, content, color, icon string) (*store.Note, error) {
	res, err := s.db.Exec(
		`UPDATE notes SET title=?, content=?, color=?, icon=?, updated_at=datetime('now') WHERE id=?`,
		title, content, color, icon, id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, store.ErrNoteNotFound
	}
	return s.GetNote(id), nil
}

// DeleteNote removes a note by ID.
func (s *SQLiteStore) DeleteNote(id string) error {
	res, err := s.db.Exec(`DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNoteNotFound
	}
	return nil
}

// ContainerNotes returns all notes for a container, newest first.
func (s *SQLiteStore) ContainerNotes(containerID string) []store.Note {
	rows, err := s.db.Query(
		`SELECT `+noteSelectCols+` FROM notes WHERE container_id = ? ORDER BY created_at DESC`, containerID)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var notes []store.Note
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			continue
		}
		notes = append(notes, note)
	}
	return notes
}

// ItemNotes returns all notes for an item, newest first.
func (s *SQLiteStore) ItemNotes(itemID string) []store.Note {
	rows, err := s.db.Query(
		`SELECT `+noteSelectCols+` FROM notes WHERE item_id = ? ORDER BY created_at DESC`, itemID)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var notes []store.Note
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			continue
		}
		notes = append(notes, note)
	}
	return notes
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/sqlite/ -run TestNoteStore -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite/notes.go internal/store/sqlite/notes_test.go
git commit -m "feat(store): implement SQLite notes store with tests"
```

---

### Task 4: Search Notes — Tests First

**Files:**
- Modify: `internal/store/sqlite/search.go` (add SearchNotes)
- Modify: `internal/store/sqlite/search_test.go` (add SearchNotes tests)

- [ ] **Step 1: Write search test**

Add to `internal/store/sqlite/search_test.go`:

```go
func TestSearchStore_Notes(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	db.CreateNote(c.ID, "", "Fragile", "Handle with care", "", "")
	db.CreateNote(c.ID, "", "Review", "Check date", "", "")

	results := db.SearchNotes("Fragile")
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
	if len(results) > 0 && results[0].Title != "Fragile" {
		t.Errorf("got title %q, want %q", results[0].Title, "Fragile")
	}
}

func TestSearchStore_Notes_Content(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	db.CreateNote(c.ID, "", "Note", "ceramic capacitor inside", "", "")

	results := db.SearchNotes("capacitor")
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

func TestSearchStore_Notes_Empty(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	db.CreateNote(c.ID, "", "Note", "content", "", "")

	results := db.SearchNotes("")
	if results != nil {
		t.Error("expected nil for empty query")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/sqlite/ -run TestSearchStore_Notes -v`
Expected: FAIL — SearchNotes not implemented

- [ ] **Step 3: Add SearchNotes to search.go**

Add at the end of `internal/store/sqlite/search.go`:

```go
// SearchNotes performs a full-text search over notes using the FTS5 index.
func (s *SQLiteStore) SearchNotes(query string) []store.Note {
	fq := fts5Query(query)
	if fq == "" {
		return nil
	}
	rows, err := s.db.Query(`
		SELECT n.id, n.container_id, n.item_id, n.title, n.content, n.color, n.icon, n.created_at
		FROM notes n
		JOIN notes_fts ON notes_fts.rowid = n.rowid
		WHERE notes_fts MATCH ?
		ORDER BY rank`, fq)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var notes []store.Note
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			continue
		}
		notes = append(notes, note)
	}
	return notes
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/sqlite/ -run TestSearchStore_Notes -v`
Expected: PASS

- [ ] **Step 5: Run all store tests**

Run: `go test ./internal/store/sqlite/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store/sqlite/search.go internal/store/sqlite/search_test.go
git commit -m "feat(store): add FTS5 search for notes"
```

---

### Task 5: Note Service

**Files:**
- Create: `internal/service/notes.go`

- [ ] **Step 1: Implement NoteService**

```go
package service

import (
	"fmt"

	"github.com/erxyi/qlx/internal/shared/palette"
	"github.com/erxyi/qlx/internal/shared/validate"
	"github.com/erxyi/qlx/internal/store"
)

// NoteService handles note CRUD operations.
type NoteService struct {
	store interface {
		store.NoteStore
	}
}

// NewNoteService creates a new NoteService backed by the given store.
func NewNoteService(s interface {
	store.NoteStore
}) *NoteService {
	return &NoteService{store: s}
}

// GetNote returns the note with the given ID, or nil.
func (s *NoteService) GetNote(id string) *store.Note {
	return s.store.GetNote(id)
}

// CreateNote validates and creates a new note.
func (s *NoteService) CreateNote(containerID, itemID, title, content, color, icon string) (*store.Note, error) {
	if err := validate.Name(title, validate.MaxNameLength); err != nil {
		return nil, err
	}
	if err := validate.OptionalText(content, validate.MaxDescriptionLength); err != nil {
		return nil, err
	}
	if color != "" && !palette.ValidColor(color) {
		return nil, fmt.Errorf("invalid color: %s", color)
	}
	if icon != "" && !palette.ValidIcon(icon) {
		return nil, fmt.Errorf("invalid icon: %s", icon)
	}
	if (containerID == "") == (itemID == "") {
		return nil, fmt.Errorf("exactly one of container_id or item_id must be set")
	}
	note := s.store.CreateNote(containerID, itemID, title, content, color, icon)
	return note, nil
}

// UpdateNote validates and updates a note.
func (s *NoteService) UpdateNote(id, title, content, color, icon string) (*store.Note, error) {
	if err := validate.Name(title, validate.MaxNameLength); err != nil {
		return nil, err
	}
	if err := validate.OptionalText(content, validate.MaxDescriptionLength); err != nil {
		return nil, err
	}
	if color != "" && !palette.ValidColor(color) {
		return nil, fmt.Errorf("invalid color: %s", color)
	}
	if icon != "" && !palette.ValidIcon(icon) {
		return nil, fmt.Errorf("invalid icon: %s", icon)
	}
	return s.store.UpdateNote(id, title, content, color, icon)
}

// DeleteNote deletes a note.
func (s *NoteService) DeleteNote(id string) error {
	return s.store.DeleteNote(id)
}

// ContainerNotes returns all notes for a container.
func (s *NoteService) ContainerNotes(containerID string) []store.Note {
	return s.store.ContainerNotes(containerID)
}

// ItemNotes returns all notes for an item.
func (s *NoteService) ItemNotes(itemID string) []store.Note {
	return s.store.ItemNotes(itemID)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/service/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/service/notes.go
git commit -m "feat(service): add NoteService with validation"
```

---

### Task 6: Note Handler — Tests First

**Files:**
- Modify: `internal/handler/request.go` (add request types)
- Modify: `internal/handler/viewmodels.go` (add SearchResultsData.Notes field)
- Create: `internal/handler/notes.go`
- Create: `internal/handler/notes_test.go`

- [ ] **Step 1: Add request types to request.go**

Add after `AdhocPrintRequest` (line 141):

```go
// CreateNoteRequest is the input for note creation.
type CreateNoteRequest struct {
	ContainerID string `json:"container_id" form:"container_id"`
	ItemID      string `json:"item_id" form:"item_id"`
	Title       string `json:"title" form:"title"`
	Content     string `json:"content" form:"content"`
	Color       string `json:"color" form:"color"`
	Icon        string `json:"icon" form:"icon"`
}

// UpdateNoteRequest is the input for note updates.
type UpdateNoteRequest struct {
	Title   string `json:"title" form:"title"`
	Content string `json:"content" form:"content"`
	Color   string `json:"color" form:"color"`
	Icon    string `json:"icon" form:"icon"`
}
```

- [ ] **Step 2: Add Notes field to SearchResultsData**

In `internal/handler/viewmodels.go`, add `Notes` to `SearchResultsData`:

```go
type SearchResultsData struct {
	Query      string
	Containers []store.Container
	Items      []store.Item
	Tags       []store.Tag
	Notes      []store.Note
}
```

- [ ] **Step 3: Write handler tests**

```go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/store"
)

func newTestNoteHandler(t *testing.T) (*NoteHandler, *service.NoteService, *service.InventoryService) {
	t.Helper()
	s := newHandlerTestStore(t)
	inv := service.NewInventoryService(s)
	notes := service.NewNoteService(s)
	h := NewNoteHandler(notes, inv, &JSONResponder{})
	return h, notes, inv
}

func TestNoteHandler_Create_JSON(t *testing.T) {
	h, _, inv := newTestNoteHandler(t)

	container, err := inv.CreateContainer("", "TestBox", "", "", "")
	if err != nil {
		t.Fatalf("create container: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := map[string]any{
		"container_id": container.ID,
		"title":        "Fragile",
		"content":      "Handle with care",
		"color":        "red",
		"icon":         "alert-triangle",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/notes", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var note store.Note
	if err := json.NewDecoder(w.Body).Decode(&note); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if note.Title != "Fragile" {
		t.Errorf("expected title Fragile, got %s", note.Title)
	}
	if note.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestNoteHandler_Detail_NotFound(t *testing.T) {
	h, _, _ := newTestNoteHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/notes/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestNoteHandler_Update_JSON(t *testing.T) {
	h, notesSvc, inv := newTestNoteHandler(t)

	container, _ := inv.CreateContainer("", "Box", "", "", "")
	note, _ := notesSvc.CreateNote(container.ID, "", "Old", "old content", "", "")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := map[string]any{
		"title":   "New Title",
		"content": "new content",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/notes/"+note.ID, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var updated store.Note
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Title != "New Title" {
		t.Errorf("expected title 'New Title', got %s", updated.Title)
	}
}

func TestNoteHandler_Delete_JSON(t *testing.T) {
	h, notesSvc, inv := newTestNoteHandler(t)

	container, _ := inv.CreateContainer("", "Box", "", "", "")
	note, _ := notesSvc.CreateNote(container.ID, "", "ToDelete", "", "", "")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/notes/"+note.ID, nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestNoteHandler_ContainerNotes(t *testing.T) {
	h, notesSvc, inv := newTestNoteHandler(t)

	container, _ := inv.CreateContainer("", "Box", "", "", "")
	notesSvc.CreateNote(container.ID, "", "Note1", "", "", "")
	notesSvc.CreateNote(container.ID, "", "Note2", "", "", "")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/containers/"+container.ID+"/notes", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var notes []store.Note
	json.NewDecoder(w.Body).Decode(&notes)
	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/handler/ -run TestNoteHandler -v`
Expected: FAIL — NoteHandler not implemented

- [ ] **Step 5: Implement notes.go handler**

```go
package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/store"
)

// NoteHandler handles HTTP requests for note CRUD operations.
type NoteHandler struct {
	notes     *service.NoteService
	inventory *service.InventoryService
	resp      Responder
}

// NewNoteHandler creates a new NoteHandler.
func NewNoteHandler(notes *service.NoteService, inv *service.InventoryService, resp Responder) *NoteHandler {
	return &NoteHandler{notes: notes, inventory: inv, resp: resp}
}

// RegisterRoutes registers note routes on the given mux.
func (h *NoteHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /notes/{id}", h.Detail)
	mux.HandleFunc("POST /notes", h.Create)
	mux.HandleFunc("PUT /notes/{id}", h.Update)
	mux.HandleFunc("DELETE /notes/{id}", h.Delete)
	mux.HandleFunc("GET /containers/{id}/notes", h.ContainerNotes)
	mux.HandleFunc("GET /items/{id}/notes", h.ItemNotes)
}

// Detail handles GET /notes/{id}.
func (h *NoteHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	note := h.notes.GetNote(id)
	if note == nil {
		h.resp.RespondError(w, r, store.ErrNoteNotFound)
		return
	}
	h.resp.Respond(w, r, http.StatusOK, note, "note", nil)
}

// Create handles POST /notes.
func (h *NoteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateNoteRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	note, err := h.notes.CreateNote(req.ContainerID, req.ItemID, req.Title, req.Content, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusCreated, note, "note", nil)
}

// Update handles PUT /notes/{id}.
func (h *NoteHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req UpdateNoteRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	note, err := h.notes.UpdateNote(id, req.Title, req.Content, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, note, "note", nil)
}

// Delete handles DELETE /notes/{id}.
func (h *NoteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.notes.DeleteNote(id); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, map[string]string{"id": id}, "", nil)
}

// ContainerNotes handles GET /containers/{id}/notes.
func (h *NoteHandler) ContainerNotes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.inventory.GetContainer(id) == nil {
		h.resp.RespondError(w, r, store.ErrContainerNotFound)
		return
	}

	notes := h.notes.ContainerNotes(id)
	h.resp.Respond(w, r, http.StatusOK, notes, "notes", nil)
}

// ItemNotes handles GET /items/{id}/notes.
func (h *NoteHandler) ItemNotes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.inventory.GetItem(id) == nil {
		h.resp.RespondError(w, r, store.ErrItemNotFound)
		return
	}

	notes := h.notes.ItemNotes(id)
	h.resp.Respond(w, r, http.StatusOK, notes, "notes", nil)
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/handler/ -run TestNoteHandler -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/handler/request.go internal/handler/viewmodels.go internal/handler/notes.go internal/handler/notes_test.go
git commit -m "feat(handler): add note CRUD handlers with tests"
```

---

### Task 7: Integrate Search and Wire Composition Root

**Files:**
- Modify: `internal/service/search.go` (add SearchNotes)
- Modify: `internal/handler/search.go` (add notes to search results)
- Modify: `internal/app/server.go` (wire NoteService + NoteHandler)

- [ ] **Step 1: Add SearchNotes to SearchService**

In `internal/service/search.go`, update the store interface and add method:

Change `store store.SearchStore` — this is already correct since we extended `SearchStore`.

Add method:

```go
// SearchNotes searches notes by title and content.
func (s *SearchService) SearchNotes(query string) []store.Note {
	return s.store.SearchNotes(query)
}
```

- [ ] **Step 2: Add notes to search handler**

In `internal/handler/search.go`, update the `Search` method.

Empty case — add `"notes": []any{}` to the data map.

Non-empty case — add:
```go
notes := h.search.SearchNotes(q)
```

Add `"notes": notes` to the data map and `Notes: notes` to `SearchResultsData`.

- [ ] **Step 3: Wire in server.go**

In `internal/app/server.go`:

After `export := service.NewExportService(s)` (line 37), add:
```go
notes := service.NewNoteService(s)
```

In the `registrars` slice, after `handler.NewExportHandler(export, inventory)` (line 54), add:
```go
handler.NewNoteHandler(notes, inventory, resp),
```

- [ ] **Step 4: Verify it compiles and all tests pass**

Run: `go build ./... && go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/search.go internal/handler/search.go internal/app/server.go
git commit -m "feat(app): integrate notes into search and wire composition root"
```

---

### Task 8: Print Endpoint

**Files:**
- Modify: `internal/handler/notes.go` (add Print method and route)

- [ ] **Step 1: Check existing print handler pattern**

Read `internal/handler/print.go` to understand how `POST /items/{id}/print` works — it gets the item, resolves template, calls `PrinterManager.Print()`. Replicate this pattern for notes.

- [ ] **Step 2: Add print route to RegisterRoutes**

Add to `NoteHandler.RegisterRoutes`:
```go
mux.HandleFunc("POST /notes/{id}/print", h.Print)
```

- [ ] **Step 3: Add PrinterService and TemplateService to NoteHandler**

Update `NoteHandler` struct to include printer and template services (follow `ItemHandler` pattern). Update `NewNoteHandler` constructor signature accordingly. Update `server.go` wiring and test helpers.

- [ ] **Step 4: Implement Print handler**

Follow existing print handler pattern — get note, resolve printer/template, call `PrinterManager.Print()`.

- [ ] **Step 5: Verify it compiles**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/handler/notes.go internal/app/server.go
git commit -m "feat(handler): add note print endpoint"
```

---

### Task 9: Lint and Full Test Suite

- [ ] **Step 1: Run linter**

Run: `make lint`
Expected: PASS (fix any issues)

- [ ] **Step 2: Run full test suite**

Run: `make test`
Expected: ALL PASS

- [ ] **Step 3: Run E2E tests**

Run: `make test-e2e`
Expected: ALL PASS (existing tests should not break)

- [ ] **Step 4: Fix any issues and commit**

If lint or tests fail, fix and commit with appropriate message.
