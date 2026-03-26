# Notes (Sticky Annotations) — Design Spec

**Date:** 2026-03-24
**Issue:** #11
**Scope:** Backend only (API + store + service)

## Overview

Independent note entities attached to containers or items — the digital equivalent of a physical sticky note. Each note has a title, content, color, and icon. Notes are searchable via FTS5 and printable as standalone labels.

## Decisions

| Decision | Choice |
|---|---|
| Fields | title + content + color + icon |
| Parent | container XOR item (two FK columns, CHECK constraint) |
| Tags | none |
| Ordering | created_at DESC |
| Parent deletion | cascade (FK ON DELETE CASCADE) |
| Move/reassign | not supported |
| Search | FTS5, integrated into existing /search endpoint |
| Print | POST /api/notes/{id}/print |

## Data Model

### SQL Schema

```sql
CREATE TABLE notes (
    id TEXT PRIMARY KEY,
    container_id TEXT REFERENCES containers(id) ON DELETE CASCADE,
    item_id TEXT REFERENCES items(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    color TEXT NOT NULL DEFAULT '',
    icon TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((container_id IS NOT NULL) != (item_id IS NOT NULL))
);

CREATE VIRTUAL TABLE notes_fts USING fts5(
    title, content,
    content=notes, content_rowid=rowid
);

-- FTS sync triggers (INSERT/UPDATE/DELETE) following existing pattern from containers_fts/items_fts
```

### Go Struct

```go
type Note struct {
    ID          string `json:"id"`
    ContainerID string `json:"container_id,omitempty"`
    ItemID      string `json:"item_id,omitempty"`
    Title       string `json:"title"`
    Content     string `json:"content"`
    Color       string `json:"color"`
    Icon        string `json:"icon"`
    CreatedAt   string `json:"created_at"`
}
```

## Store Interface

```go
type NoteStore interface {
    GetNote(id string) *Note
    CreateNote(containerID, itemID, title, content, color, icon string) *Note
    UpdateNote(id, title, content, color, icon string) (*Note, error)
    DeleteNote(id string) error
    ContainerNotes(containerID string) []Note
    ItemNotes(itemID string) []Note
}
```

`SearchNotes(query string) []Note` added to existing `SearchStore` interface.

## API Contract

### Endpoints

```
GET    /api/notes/{id}              → get note
POST   /api/notes                   → create note
PUT    /api/notes/{id}              → update note
DELETE /api/notes/{id}              → delete note
GET    /api/containers/{id}/notes   → list container notes (DESC)
GET    /api/items/{id}/notes        → list item notes (DESC)
POST   /api/notes/{id}/print        → print note as label
GET    /search?q=...                → extended with note results
```

### POST /api/notes — Create

**Request:**
```json
{
  "container_id": "uuid",
  "title": "Fragile",
  "content": "Handle with care",
  "color": "red",
  "icon": "alert-triangle"
}
```
Or with `"item_id"` instead of `"container_id"`. Exactly one must be provided.

**Response (201):**
```json
{
  "id": "uuid",
  "container_id": "uuid",
  "title": "Fragile",
  "content": "Handle with care",
  "color": "red",
  "icon": "alert-triangle",
  "created_at": "2026-03-24T12:00:00Z"
}
```

### GET /api/notes/{id} — Get

**Response (200):**
```json
{
  "id": "uuid",
  "container_id": "uuid",
  "title": "Fragile",
  "content": "Handle with care",
  "color": "red",
  "icon": "alert-triangle",
  "created_at": "2026-03-24T12:00:00Z"
}
```

**Response (404):**
```json
{ "error": "not found" }
```

### PUT /api/notes/{id} — Update

**Request:**
```json
{
  "title": "Very Fragile",
  "content": "Handle with extreme care",
  "color": "red",
  "icon": "alert-triangle"
}
```

**Response (200):** updated note object

**Response (404):**
```json
{ "error": "not found" }
```

### DELETE /api/notes/{id} — Delete

**Response (200):**
```json
{ "id": "uuid" }
```

**Response (404):**
```json
{ "error": "not found" }
```

### GET /api/containers/{id}/notes — List Container Notes

**Response (200):**
```json
[
  { "id": "uuid", "container_id": "uuid", "title": "...", "content": "...", "color": "...", "icon": "...", "created_at": "..." },
  ...
]
```

Returns empty array `[]` if no notes.

### GET /api/items/{id}/notes — List Item Notes

Same shape as container notes, with `item_id` instead of `container_id`.

### POST /api/notes/{id}/print — Print

**Request:**
```json
{
  "template": "template-id"
}
```

**Response:** follows existing print endpoint pattern (SSE events via PrinterManager).

### GET /search?q=... — Extended Search

Response gains a `notes` field alongside existing `containers`, `items`, `tags`:

```json
{
  "containers": [...],
  "items": [...],
  "tags": [...],
  "notes": [
    { "id": "uuid", "container_id": "uuid", "title": "...", "content": "...", "color": "...", "icon": "...", "created_at": "..." }
  ]
}
```

## Service Layer

```go
type NoteService struct {
    store interface {
        NoteStore
    }
}
```

**Validation (Create/Update):**
- `title` — required, `validate.Name(title, MaxNameLength)`
- `content` — optional, `validate.OptionalText(content, MaxDescriptionLength)`
- `color` — `palette.ValidColor(color)`
- `icon` — `palette.ValidIcon(icon)`
- Create: exactly one of `container_id`/`item_id` must be non-empty

**Print:** handler calls `NoteService.GetNote()` then delegates to `PrinterService` (existing pattern from items).

## Handler

```go
type NoteHandler struct {
    notes     *service.NoteService
    inventory *service.InventoryService
    printers  *service.PrinterService
    resp      Responder
}
```

Implements `RouteRegistrar`. Request structs:

```go
type CreateNoteRequest struct {
    ContainerID string `json:"container_id" form:"container_id"`
    ItemID      string `json:"item_id" form:"item_id"`
    Title       string `json:"title" form:"title"`
    Content     string `json:"content" form:"content"`
    Color       string `json:"color" form:"color"`
    Icon        string `json:"icon" form:"icon"`
}

type UpdateNoteRequest struct {
    Title   string `json:"title" form:"title"`
    Content string `json:"content" form:"content"`
    Color   string `json:"color" form:"color"`
    Icon    string `json:"icon" form:"icon"`
}
```

## Migration

Goose migration file following existing naming convention. Includes:
- `notes` table with FK constraints and CHECK
- `notes_fts` virtual table
- FTS sync triggers (INSERT/UPDATE/DELETE)
- Down migration drops both tables
