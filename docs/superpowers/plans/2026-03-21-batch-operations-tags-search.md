# Batch Operations, Tags & Search — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add batch adding (quick entry), multi-select with bulk move/delete/tag, hierarchical tags with inheritance, and global search to the QLX inventory UI.

**Architecture:** Hybrid HTMX + vanilla JS. Store stays JSON file-based with a new migration system. Tags are a new entity with hierarchy (ParentID). Bulk operations use JSON bodies in UI handlers (documented exception). Tree picker is a reusable HTMX `<dialog>` component.

**Tech Stack:** Go 1.22+ stdlib, HTMX 1.9+, vanilla JS, JSON file store

**Spec:** `docs/superpowers/specs/2026-03-21-batch-operations-design.md`

---

## File Map

### New Files
- `internal/store/migrate.go` — migration system (version check, backup, sequential migrations)
- `internal/store/migrate_test.go` — migration tests
- `internal/store/tags.go` — Tag CRUD, TagChildren, TagPath, TagDescendants, AddTag, RemoveTag, ItemsByTag
- `internal/store/tags_test.go` — tag store tests
- `internal/store/bulk.go` — MoveItems, MoveContainers, DeleteItems, DeleteContainers, bulk tag
- `internal/store/bulk_test.go` — bulk store tests
- `internal/store/search.go` — SearchContainers, SearchItems, SearchTags
- `internal/store/search_test.go` — search store tests
- `internal/ui/handlers_tags.go` — UI tag handlers
- `internal/ui/handlers_bulk.go` — UI bulk operation handlers
- `internal/ui/handlers_search.go` — UI search handler
- `internal/ui/handlers_partials.go` — tree picker and tag-tree picker partials
- `internal/api/handlers_tags.go` — API tag endpoints
- `internal/api/handlers_bulk.go` — API bulk endpoints
- `internal/api/handlers_search.go` — API search endpoint
- `internal/embedded/templates/tags.html` — tag tree page
- `internal/embedded/templates/search.html` — search results page
- `internal/embedded/templates/partials/container_list_item.html` — single container `<li>`
- `internal/embedded/templates/partials/item_list_item.html` — single item `<li>`
- `internal/embedded/templates/partials/tag_list_item.html` — single tag `<li>`
- `internal/embedded/templates/partials/tree_picker.html` — reusable tree picker dialog
- `internal/embedded/templates/partials/tag_chips.html` — tag badge/chip list partial

### Modified Files
- `internal/store/models.go` — add Quantity/TagIDs to Item, TagIDs to Container, Tag struct
- `internal/store/store.go` — add `tags` to storeData, call migrate on startup, add Version field
- `internal/ui/server.go` — register new templates, routes, view models
- `internal/ui/handlers.go` — modify HandleContainerCreate/HandleItemCreate for quick-entry HTMX response
- `internal/api/server.go` — register new API routes
- `internal/embedded/templates/containers.html` — add `id` attrs to `<ul>`, quick entry forms, checkboxes, action bar, tag chips, filter bar, search input in layout
- `internal/embedded/templates/item.html` — add quantity display, tag chips
- `internal/embedded/templates/layout.html` — add search input in nav, add Tags link
- `internal/embedded/static/ui-lite.js` — selection module, action bar, multi-drag, move/tag picker, search
- `internal/embedded/static/style.css` — flash animation, action bar, checkbox, dialog, tag chips, search

---

## Task 1: Store Migration System

**Files:**
- Create: `internal/store/migrate.go`
- Create: `internal/store/migrate_test.go`
- Modify: `internal/store/store.go` (storeData struct, NewStore)

- [ ] **Step 1: Write failing test for migration version detection**

```go
// internal/store/migrate_test.go
package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateDetectsVersion(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.json")

	// v0 data: no version field
	data := `{"containers":{},"items":{}}`
	os.WriteFile(path, []byte(data), 0644)

	raw, version, err := loadRaw(path)
	if err != nil {
		t.Fatalf("loadRaw error: %v", err)
	}
	if version != 0 {
		t.Errorf("version = %d, want 0", version)
	}
	if raw == nil {
		t.Fatal("raw data is nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -run TestMigrateDetectsVersion -v`
Expected: FAIL — `loadRaw` undefined

- [ ] **Step 3: Implement loadRaw and migration runner**

```go
// internal/store/migrate.go
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Migration is a function that transforms raw store data from one version to the next.
type Migration func(data map[string]any) error

// migrations is the ordered list of migrations. Index 0 = v0→v1, index 1 = v1→v2, etc.
var migrations = []Migration{
	migrateV0ToV1,
}

// loadRaw reads the store file and returns the parsed JSON map and the current version.
func loadRaw(path string) (map[string]any, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, 0, fmt.Errorf("unmarshal store: %w", err)
	}

	version := 0
	if v, ok := raw["version"].(float64); ok {
		version = int(v)
	}

	return raw, version, nil
}

// runMigrations applies all pending migrations, creating backups before each.
// Returns the final migrated JSON bytes and the new version number.
func runMigrations(path string, raw map[string]any, currentVersion int) ([]byte, int, error) {
	for i := currentVersion; i < len(migrations); i++ {
		// Backup before migration
		if err := backupStore(path, i); err != nil {
			return nil, i, fmt.Errorf("backup before v%d→v%d: %w", i, i+1, err)
		}

		if err := migrations[i](raw); err != nil {
			return nil, i, fmt.Errorf("migration v%d→v%d: %w", i, i+1, err)
		}

		raw["version"] = float64(i + 1)
	}

	result, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return nil, currentVersion, err
	}

	return result, len(migrations), nil
}

// backupStore creates an atomic backup of the store file.
func backupStore(path string, version int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to backup
		}
		return err
	}

	dir := filepath.Dir(path)
	backupPath := filepath.Join(dir, fmt.Sprintf("backup-v%d.json", version))
	tmpPath := backupPath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, backupPath)
}

// migrateV0ToV1 adds quantity to items, tag_ids to items and containers, and tags collection.
func migrateV0ToV1(data map[string]any) error {
	// Add quantity to items
	if items, ok := data["items"].(map[string]any); ok {
		for _, v := range items {
			if item, ok := v.(map[string]any); ok {
				if _, exists := item["quantity"]; !exists {
					item["quantity"] = float64(1)
				}
				if _, exists := item["tag_ids"]; !exists {
					item["tag_ids"] = []any{}
				}
			}
		}
	}

	// Add tag_ids to containers
	if containers, ok := data["containers"].(map[string]any); ok {
		for _, v := range containers {
			if container, ok := v.(map[string]any); ok {
				if _, exists := container["tag_ids"]; !exists {
					container["tag_ids"] = []any{}
				}
			}
		}
	}

	// Add tags collection
	if _, exists := data["tags"]; !exists {
		data["tags"] = map[string]any{}
	}

	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -run TestMigrateDetectsVersion -v`
Expected: PASS

- [ ] **Step 5: Write test for v0→v1 migration**

```go
func TestMigrateV0ToV1(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.json")

	v0 := `{"containers":{"c1":{"id":"c1","parent_id":"","name":"Box","description":"","created_at":"2025-01-01T00:00:00Z"}},"items":{"i1":{"id":"i1","container_id":"c1","name":"Widget","description":"","created_at":"2025-01-01T00:00:00Z"}}}`
	os.WriteFile(path, []byte(v0), 0644)

	raw, version, _ := loadRaw(path)
	if version != 0 {
		t.Fatalf("expected v0, got v%d", version)
	}

	result, newVersion, err := runMigrations(path, raw, version)
	if err != nil {
		t.Fatalf("runMigrations error: %v", err)
	}
	if newVersion != 1 {
		t.Errorf("newVersion = %d, want 1", newVersion)
	}

	// Parse result and verify
	var migrated map[string]any
	json.Unmarshal(result, &migrated)

	items := migrated["items"].(map[string]any)
	item := items["i1"].(map[string]any)
	if item["quantity"] != float64(1) {
		t.Errorf("item quantity = %v, want 1", item["quantity"])
	}
	if item["tag_ids"] == nil {
		t.Error("item tag_ids should exist")
	}

	containers := migrated["containers"].(map[string]any)
	container := containers["c1"].(map[string]any)
	if container["tag_ids"] == nil {
		t.Error("container tag_ids should exist")
	}

	if migrated["tags"] == nil {
		t.Error("tags collection should exist")
	}

	// Verify backup was created
	backupPath := filepath.Join(tmpDir, "backup-v0.json")
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("backup-v0.json should exist")
	}
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -run TestMigrateV0ToV1 -v`
Expected: PASS

- [ ] **Step 7: Integrate migration into NewStore**

Modify `internal/store/store.go`:
- Add `Version int` and `Tags map[string]*Tag` to `storeData`
- In `NewStore()`: before unmarshalling, call `loadRaw` + `runMigrations` if version < current, write migrated data atomically, then proceed with normal unmarshal
- In `Save()`: update the marshalled `storeData` literal to include `Tags: s.tags` and `Version: currentVersion`
- In `NewStore()`: after unmarshal, add nil guard `if s.tags == nil { s.tags = make(map[string]*Tag) }`
- In `NewMemoryStore()`: add `tags: make(map[string]*Tag)` to the initialization

- [ ] **Step 8: Write test for NewStore with migration**

```go
func TestNewStoreRunsMigration(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.json")

	v0 := `{"containers":{},"items":{"i1":{"id":"i1","container_id":"","name":"A","description":"","created_at":"2025-01-01T00:00:00Z"}}}`
	os.WriteFile(path, []byte(v0), 0644)

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	item := s.GetItem("i1")
	if item == nil {
		t.Fatal("item i1 not found")
	}
	if item.Quantity != 1 {
		t.Errorf("item.Quantity = %d, want 1", item.Quantity)
	}
}
```

- [ ] **Step 9: Run all store tests**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -v`
Expected: ALL PASS

- [ ] **Step 10: Commit**

```bash
git add internal/store/migrate.go internal/store/migrate_test.go internal/store/store.go
git commit -m "feat(store): add migration system with v0→v1 (quantity, tag_ids, tags)"
```

---

## Task 2: Data Model Changes

**Files:**
- Modify: `internal/store/models.go`

- [ ] **Step 1: Add new fields and Tag struct**

In `internal/store/models.go`:
- Add `Quantity int` and `TagIDs []string` to `Item`
- Add `TagIDs []string` to `Container`
- Add `Tag` struct

```go
type Container struct {
	ID          string    `json:"id"`
	ParentID    string    `json:"parent_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	TagIDs      []string  `json:"tag_ids"`
	CreatedAt   time.Time `json:"created_at"`
}

type Item struct {
	ID          string    `json:"id"`
	ContainerID string    `json:"container_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Quantity    int       `json:"quantity"`
	TagIDs      []string  `json:"tag_ids"`
	CreatedAt   time.Time `json:"created_at"`
}

type Tag struct {
	ID        string    `json:"id"`
	ParentID  string    `json:"parent_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 2: Update CreateItem signature and set default Quantity**

In `internal/store/store.go`, update `CreateItem` signature to `CreateItem(containerID, name, description string, quantity int) *Item`. If `quantity < 1`, default to `1`. Also add `TagIDs: []string{}`. Update all existing callers of `CreateItem` (in `ui/handlers.go` and `api/server.go`) to pass `1` as the quantity argument.

- [ ] **Step 3: Update CreateContainer to init TagIDs**

In `internal/store/store.go`, `CreateContainer()`: set `TagIDs: []string{}`.

- [ ] **Step 4: Run all existing tests to verify no regression**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/models.go internal/store/store.go
git commit -m "feat(store): add Quantity and TagIDs to Item/Container, add Tag model"
```

---

## Task 3: Tag Store CRUD

**Files:**
- Create: `internal/store/tags.go`
- Create: `internal/store/tags_test.go`

- [ ] **Step 1: Write failing tests for tag CRUD**

```go
// internal/store/tags_test.go
package store

import "testing"

func TestTagCRUD(t *testing.T) {
	s := NewMemoryStore()

	t.Run("create root tag", func(t *testing.T) {
		tag := s.CreateTag("", "Electronics")
		if tag.ID == "" {
			t.Error("CreateTag should set ID")
		}
		if tag.Name != "Electronics" {
			t.Errorf("Name = %q, want %q", tag.Name, "Electronics")
		}
		if tag.ParentID != "" {
			t.Errorf("ParentID = %q, want empty", tag.ParentID)
		}
	})

	t.Run("create child tag", func(t *testing.T) {
		parent := s.CreateTag("", "Materials")
		child := s.CreateTag(parent.ID, "Filament")
		if child.ParentID != parent.ID {
			t.Errorf("ParentID = %q, want %q", child.ParentID, parent.ID)
		}
	})

	t.Run("get tag", func(t *testing.T) {
		tag := s.CreateTag("", "Tools")
		got := s.GetTag(tag.ID)
		if got == nil || got.Name != "Tools" {
			t.Errorf("GetTag returned %v, want Tools", got)
		}
	})

	t.Run("update tag", func(t *testing.T) {
		tag := s.CreateTag("", "Old")
		updated, err := s.UpdateTag(tag.ID, "New")
		if err != nil {
			t.Fatalf("UpdateTag error: %v", err)
		}
		if updated.Name != "New" {
			t.Errorf("Name = %q, want %q", updated.Name, "New")
		}
	})

	t.Run("delete leaf tag", func(t *testing.T) {
		tag := s.CreateTag("", "Temporary")
		if err := s.DeleteTag(tag.ID); err != nil {
			t.Fatalf("DeleteTag error: %v", err)
		}
		if s.GetTag(tag.ID) != nil {
			t.Error("tag should be deleted")
		}
	})

	t.Run("delete tag with children fails", func(t *testing.T) {
		parent := s.CreateTag("", "Parent")
		s.CreateTag(parent.ID, "Child")
		err := s.DeleteTag(parent.ID)
		if err == nil {
			t.Error("should fail to delete tag with children")
		}
	})

	t.Run("delete tag removes from items", func(t *testing.T) {
		tag := s.CreateTag("", "ToRemove")
		c := s.CreateContainer("", "Box", "")
		item := s.CreateItem(c.ID, "Thing", "")
		s.AddItemTag(item.ID, tag.ID)

		if err := s.DeleteTag(tag.ID); err != nil {
			t.Fatalf("DeleteTag error: %v", err)
		}
		updated := s.GetItem(item.ID)
		for _, tid := range updated.TagIDs {
			if tid == tag.ID {
				t.Error("tag should be removed from item")
			}
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -run TestTagCRUD -v`
Expected: FAIL — methods undefined

- [ ] **Step 3: Implement tag CRUD**

```go
// internal/store/tags.go
package store

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTagNotFound    = errors.New("tag not found")
	ErrTagHasChildren = errors.New("tag has children")
)

// CreateTag creates a new tag with the given parentID and name.
func (s *Store) CreateTag(parentID, name string) *Tag {
	s.mu.Lock()
	defer s.mu.Unlock()

	tag := &Tag{
		ID:        uuid.New().String(),
		ParentID:  parentID,
		Name:      name,
		CreatedAt: time.Now(),
	}
	s.tags[tag.ID] = tag
	return tag
}

// GetTag returns the tag with the given ID, or nil if not found.
func (s *Store) GetTag(id string) *Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t := s.tags[id]
	if t == nil {
		return nil
	}
	copy := *t
	return &copy
}

// UpdateTag updates the name of the tag with the given ID.
func (s *Store) UpdateTag(id, name string) (*Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := s.tags[id]
	if t == nil {
		return nil, ErrTagNotFound
	}
	t.Name = name
	copy := *t
	return &copy, nil
}

// DeleteTag deletes a leaf tag and removes it from all items and containers.
func (s *Store) DeleteTag(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tags[id] == nil {
		return ErrTagNotFound
	}

	// Check for children
	for _, t := range s.tags {
		if t.ParentID == id {
			return ErrTagHasChildren
		}
	}

	// Remove tag from all items
	for _, item := range s.items {
		item.TagIDs = removeFromSlice(item.TagIDs, id)
	}

	// Remove tag from all containers
	for _, container := range s.containers {
		container.TagIDs = removeFromSlice(container.TagIDs, id)
	}

	delete(s.tags, id)
	return nil
}

// AllTags returns all tags.
func (s *Store) AllTags() []Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Tag, 0, len(s.tags))
	for _, t := range s.tags {
		result = append(result, *t)
	}
	return result
}

// TagChildren returns all direct children of the given tag.
func (s *Store) TagChildren(id string) []Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Tag
	for _, t := range s.tags {
		if t.ParentID == id {
			result = append(result, *t)
		}
	}
	return result
}

// TagPath returns the path from root to the given tag (inclusive).
func (s *Store) TagPath(id string) []Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var path []Tag
	current := id
	for current != "" {
		t := s.tags[current]
		if t == nil {
			break
		}
		path = append([]Tag{*t}, path...)
		current = t.ParentID
	}
	return path
}

// TagDescendants returns all descendant tag IDs (single-pass O(N) algorithm).
func (s *Store) TagDescendants(id string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.tagDescendantsLocked(id)
}

func (s *Store) tagDescendantsLocked(id string) []string {
	// Build parent→children map in one pass
	childrenOf := make(map[string][]string)
	for _, t := range s.tags {
		childrenOf[t.ParentID] = append(childrenOf[t.ParentID], t.ID)
	}

	// BFS from id
	var result []string
	queue := childrenOf[id]
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		queue = append(queue, childrenOf[current]...)
	}
	return result
}

// MoveTag moves a tag to a new parent. Rejects cycles.
func (s *Store) MoveTag(tagID, newParentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := s.tags[tagID]
	if t == nil {
		return ErrTagNotFound
	}

	if newParentID != "" {
		if s.tags[newParentID] == nil {
			return ErrTagNotFound
		}
		// Cycle detection: walk up from newParentID
		current := newParentID
		for current != "" {
			if current == tagID {
				return errors.New("move would create a cycle")
			}
			parent := s.tags[current]
			if parent == nil {
				break
			}
			current = parent.ParentID
		}
	}

	t.ParentID = newParentID
	return nil
}

// AddItemTag adds a tag to an item.
func (s *Store) AddItemTag(itemID, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := s.items[itemID]
	if item == nil {
		return ErrItemNotFound
	}
	if s.tags[tagID] == nil {
		return ErrTagNotFound
	}
	// Avoid duplicates
	for _, tid := range item.TagIDs {
		if tid == tagID {
			return nil
		}
	}
	item.TagIDs = append(item.TagIDs, tagID)
	return nil
}

// RemoveItemTag removes a tag from an item.
func (s *Store) RemoveItemTag(itemID, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := s.items[itemID]
	if item == nil {
		return ErrItemNotFound
	}
	item.TagIDs = removeFromSlice(item.TagIDs, tagID)
	return nil
}

// AddContainerTag adds a tag to a container.
func (s *Store) AddContainerTag(containerID, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	container := s.containers[containerID]
	if container == nil {
		return ErrContainerNotFound
	}
	if s.tags[tagID] == nil {
		return ErrTagNotFound
	}
	for _, tid := range container.TagIDs {
		if tid == tagID {
			return nil
		}
	}
	container.TagIDs = append(container.TagIDs, tagID)
	return nil
}

// RemoveContainerTag removes a tag from a container.
func (s *Store) RemoveContainerTag(containerID, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	container := s.containers[containerID]
	if container == nil {
		return ErrContainerNotFound
	}
	container.TagIDs = removeFromSlice(container.TagIDs, tagID)
	return nil
}

// ItemsByTag returns items that have the given tag or any descendant tag.
func (s *Store) ItemsByTag(tagID string) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build set of matching tag IDs
	matchIDs := make(map[string]bool)
	matchIDs[tagID] = true
	for _, d := range s.tagDescendantsLocked(tagID) {
		matchIDs[d] = true
	}

	var result []Item
	for _, item := range s.items {
		for _, tid := range item.TagIDs {
			if matchIDs[tid] {
				result = append(result, *item)
				break
			}
		}
	}
	return result
}

func removeFromSlice(slice []string, val string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != val {
			result = append(result, s)
		}
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -run TestTagCRUD -v`
Expected: ALL PASS

- [ ] **Step 5: Write tests for TagDescendants and ItemsByTag**

```go
func TestTagDescendants(t *testing.T) {
	s := NewMemoryStore()

	root := s.CreateTag("", "Electronics")
	sensors := s.CreateTag(root.ID, "Sensors")
	temp := s.CreateTag(sensors.ID, "Temperature")
	s.CreateTag(root.ID, "Modules")

	descendants := s.TagDescendants(root.ID)
	if len(descendants) != 3 {
		t.Errorf("descendants count = %d, want 3", len(descendants))
	}

	// Sensors subtree
	subDescendants := s.TagDescendants(sensors.ID)
	if len(subDescendants) != 1 || subDescendants[0] != temp.ID {
		t.Errorf("sensors descendants = %v, want [%s]", subDescendants, temp.ID)
	}
}

func TestItemsByTag(t *testing.T) {
	s := NewMemoryStore()

	elec := s.CreateTag("", "Electronics")
	sensors := s.CreateTag(elec.ID, "Sensors")

	c := s.CreateContainer("", "Box", "")
	item1 := s.CreateItem(c.ID, "Thermometer", "")
	item2 := s.CreateItem(c.ID, "Resistor", "")

	s.AddItemTag(item1.ID, sensors.ID)
	s.AddItemTag(item2.ID, elec.ID)

	// Query by parent tag should return both
	results := s.ItemsByTag(elec.ID)
	if len(results) != 2 {
		t.Errorf("ItemsByTag(Electronics) count = %d, want 2", len(results))
	}

	// Query by child tag should return only item1
	results = s.ItemsByTag(sensors.ID)
	if len(results) != 1 {
		t.Errorf("ItemsByTag(Sensors) count = %d, want 1", len(results))
	}
}
```

- [ ] **Step 6: Run tests**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -run "TestTagDescendants|TestItemsByTag" -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/store/tags.go internal/store/tags_test.go
git commit -m "feat(store): add tag CRUD, hierarchy, descendants, and tag assignment"
```

---

## Task 4: Bulk Store Operations

**Files:**
- Create: `internal/store/bulk.go`
- Create: `internal/store/bulk_test.go`

- [ ] **Step 1: Write failing tests for bulk move and delete**

```go
// internal/store/bulk_test.go
package store

import "testing"

func TestBulkMoveItems(t *testing.T) {
	s := NewMemoryStore()
	c1 := s.CreateContainer("", "Source", "")
	c2 := s.CreateContainer("", "Target", "")
	i1 := s.CreateItem(c1.ID, "A", "")
	i2 := s.CreateItem(c1.ID, "B", "")

	errs := s.MoveItems([]string{i1.ID, i2.ID}, c2.ID)
	if len(errs) != 0 {
		t.Fatalf("MoveItems errors: %v", errs)
	}

	if s.GetItem(i1.ID).ContainerID != c2.ID {
		t.Error("item1 should be in Target")
	}
	if s.GetItem(i2.ID).ContainerID != c2.ID {
		t.Error("item2 should be in Target")
	}
}

func TestBulkMoveContainers(t *testing.T) {
	s := NewMemoryStore()
	parent := s.CreateContainer("", "Parent", "")
	c1 := s.CreateContainer("", "A", "")
	c2 := s.CreateContainer("", "B", "")

	errs := s.MoveContainers([]string{c1.ID, c2.ID}, parent.ID)
	if len(errs) != 0 {
		t.Fatalf("MoveContainers errors: %v", errs)
	}
	if s.GetContainer(c1.ID).ParentID != parent.ID {
		t.Error("c1 should be under Parent")
	}
}

func TestBulkMoveContainersCycleDetection(t *testing.T) {
	s := NewMemoryStore()
	parent := s.CreateContainer("", "Parent", "")
	child := s.CreateContainer(parent.ID, "Child", "")

	// Try to move parent into child — should fail
	errs := s.MoveContainers([]string{parent.ID}, child.ID)
	if len(errs) == 0 {
		t.Error("should detect cycle")
	}
}

func TestBulkDeleteItems(t *testing.T) {
	s := NewMemoryStore()
	c := s.CreateContainer("", "Box", "")
	i1 := s.CreateItem(c.ID, "A", "")
	i2 := s.CreateItem(c.ID, "B", "")

	deleted, failed := s.DeleteItems([]string{i1.ID, i2.ID})
	if len(deleted) != 2 || len(failed) != 0 {
		t.Errorf("deleted=%d failed=%d, want 2/0", len(deleted), len(failed))
	}
}

func TestBulkDeleteContainersRejectsNonEmpty(t *testing.T) {
	s := NewMemoryStore()
	c := s.CreateContainer("", "Box", "")
	s.CreateItem(c.ID, "Thing", "")

	deleted, failed := s.DeleteContainers([]string{c.ID})
	if len(deleted) != 0 || len(failed) != 1 {
		t.Errorf("deleted=%d failed=%d, want 0/1", len(deleted), len(failed))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -run "TestBulk" -v`
Expected: FAIL — methods undefined

- [ ] **Step 3: Implement bulk operations**

```go
// internal/store/bulk.go
package store

import "fmt"

// BulkError represents a single failure in a bulk operation.
type BulkError struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// MoveItems moves multiple items to a target container. Returns errors for items that failed.
func (s *Store) MoveItems(ids []string, targetContainerID string) []BulkError {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.moveItemsLocked(ids, targetContainerID)
}

func (s *Store) moveItemsLocked(ids []string, targetContainerID string) []BulkError {
	if targetContainerID != "" && s.containers[targetContainerID] == nil {
		return []BulkError{{ID: "target", Reason: "target container not found"}}
	}

	var errs []BulkError
	for _, id := range ids {
		item := s.items[id]
		if item == nil {
			errs = append(errs, BulkError{ID: id, Reason: "item not found"})
			continue
		}
		item.ContainerID = targetContainerID
	}
	return errs
}

// MoveContainers moves multiple containers to a target parent.
// Pre-validates all moves for cycles before committing any.
func (s *Store) MoveContainers(ids []string, targetParentID string) []BulkError {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.moveContainersLocked(ids, targetParentID)
}

func (s *Store) moveContainersLocked(ids []string, targetParentID string) []BulkError {
	// Validate target
	if targetParentID != "" && s.containers[targetParentID] == nil {
		return []BulkError{{ID: "target", Reason: "target container not found"}}
	}

	// Pre-validate all: check cycles for each container
	movingSet := make(map[string]bool)
	for _, id := range ids {
		movingSet[id] = true
	}

	var errs []BulkError
	for _, id := range ids {
		if s.containers[id] == nil {
			errs = append(errs, BulkError{ID: id, Reason: "container not found"})
			continue
		}
		if id == targetParentID {
			errs = append(errs, BulkError{ID: id, Reason: "cannot move into self"})
			continue
		}
		// Walk up from target to check cycle — skip containers in the movingSet
		// (they will also be reparented, so their current ancestry doesn't count)
		current := targetParentID
		for current != "" {
			if current == id {
				errs = append(errs, BulkError{ID: id, Reason: "move would create a cycle"})
				break
			}
			parent := s.containers[current]
			if parent == nil {
				break
			}
			current = parent.ParentID
		}
	}

	// Second pass: check intra-batch ancestry conflicts.
	// If container A is an ancestor of container B, and both are in the batch,
	// moving both to the same target would orphan B's subtree.
	// Detect: for each pair, walk up from one to see if it hits another batch member.
	for _, id := range ids {
		current := s.containers[id].ParentID
		for current != "" {
			if movingSet[current] {
				errs = append(errs, BulkError{ID: id, Reason: fmt.Sprintf("ancestor %s is also being moved", current)})
				break
			}
			parent := s.containers[current]
			if parent == nil {
				break
			}
			current = parent.ParentID
		}
	}

	// If any validation failed, abort entire batch
	if len(errs) > 0 {
		return errs
	}

	// Commit all moves
	for _, id := range ids {
		s.containers[id].ParentID = targetParentID
	}
	return nil
}

// DeleteItems deletes multiple items. Returns lists of deleted and failed IDs.
func (s *Store) DeleteItems(ids []string) (deleted []string, failed []BulkError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteItemsLocked(ids)
}

func (s *Store) deleteItemsLocked(ids []string) (deleted []string, failed []BulkError) {
	for _, id := range ids {
		if s.items[id] == nil {
			failed = append(failed, BulkError{ID: id, Reason: "item not found"})
			continue
		}
		delete(s.items, id)
		deleted = append(deleted, id)
	}
	return
}

// DeleteContainers deletes multiple containers. Only deletes empty containers.
func (s *Store) DeleteContainers(ids []string) (deleted []string, failed []BulkError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteContainersLocked(ids)
}

func (s *Store) deleteContainersLocked(ids []string) (deleted []string, failed []BulkError) {
	for _, id := range ids {
		c := s.containers[id]
		if c == nil {
			failed = append(failed, BulkError{ID: id, Reason: "container not found"})
			continue
		}

		// Check for children
		hasChildren := false
		for _, other := range s.containers {
			if other.ParentID == id {
				hasChildren = true
				break
			}
		}
		if hasChildren {
			failed = append(failed, BulkError{ID: id, Reason: "container has children"})
			continue
		}

		// Check for items
		hasItems := false
		for _, item := range s.items {
			if item.ContainerID == id {
				hasItems = true
				break
			}
		}
		if hasItems {
			failed = append(failed, BulkError{ID: id, Reason: "container has items"})
			continue
		}

		delete(s.containers, id)
		deleted = append(deleted, id)
	}
	return
}

// BulkAddTag adds a tag to multiple items and containers.
func (s *Store) BulkAddTag(itemIDs, containerIDs []string, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tags[tagID] == nil {
		return ErrTagNotFound
	}

	for _, id := range itemIDs {
		item := s.items[id]
		if item == nil {
			continue
		}
		if !containsString(item.TagIDs, tagID) {
			item.TagIDs = append(item.TagIDs, tagID)
		}
	}

	for _, id := range containerIDs {
		container := s.containers[id]
		if container == nil {
			continue
		}
		if !containsString(container.TagIDs, tagID) {
			container.TagIDs = append(container.TagIDs, tagID)
		}
	}

	return nil
}

func containsString(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// BulkDelete handles mixed deletion of items and containers atomically under a single lock.
func (s *Store) BulkDelete(itemIDs, containerIDs []string) (deleted []string, failed []BulkError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delItems, failItems := s.deleteItemsLocked(itemIDs)
	delContainers, failContainers := s.deleteContainersLocked(containerIDs)

	deleted = append(delItems, delContainers...)
	failed = append(failItems, failContainers...)
	return
}

// BulkMove handles mixed movement of items and containers atomically under a single lock.
func (s *Store) BulkMove(itemIDs, containerIDs []string, targetID string) []BulkError {
	s.mu.Lock()
	defer s.mu.Unlock()

	var allErrs []BulkError

	if len(containerIDs) > 0 {
		errs := s.moveContainersLocked(containerIDs, targetID)
		allErrs = append(allErrs, errs...)
	}

	if len(itemIDs) > 0 {
		errs := s.moveItemsLocked(itemIDs, targetID)
		allErrs = append(allErrs, errs...)
	}

	return allErrs
}
```

**IMPORTANT:** `BulkDelete`, `BulkMove`, `MoveItems`, `MoveContainers`, `DeleteItems`, `DeleteContainers` must all use unexported `*Locked` helper functions (no lock acquisition). The public methods acquire the lock once and call the locked helpers. `BulkDelete`/`BulkMove` acquire the lock at the top level and call the locked helpers directly — ensuring atomicity (single write lock for entire batch, as required by spec).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -run "TestBulk" -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/bulk.go internal/store/bulk_test.go
git commit -m "feat(store): add bulk move, delete, and tag operations"
```

---

## Task 5: Search Store Methods

**Files:**
- Create: `internal/store/search.go`
- Create: `internal/store/search_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/store/search_test.go
package store

import "testing"

func TestSearch(t *testing.T) {
	s := NewMemoryStore()
	c := s.CreateContainer("", "Electronics Box", "")
	s.CreateItem(c.ID, "Arduino Nano", "")
	s.CreateItem(c.ID, "Raspberry Pi", "")
	s.CreateTag("", "electronic parts")

	t.Run("search containers", func(t *testing.T) {
		results := s.SearchContainers("electro")
		if len(results) != 1 {
			t.Errorf("count = %d, want 1", len(results))
		}
	})

	t.Run("search items", func(t *testing.T) {
		results := s.SearchItems("ino")
		if len(results) != 1 || results[0].Name != "Arduino Nano" {
			t.Errorf("results = %v, want Arduino Nano", results)
		}
	})

	t.Run("search tags", func(t *testing.T) {
		results := s.SearchTags("electronic")
		if len(results) != 1 {
			t.Errorf("count = %d, want 1", len(results))
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		results := s.SearchItems("ARDUINO")
		if len(results) != 1 {
			t.Errorf("count = %d, want 1", len(results))
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -run TestSearch -v`
Expected: FAIL

- [ ] **Step 3: Implement search**

```go
// internal/store/search.go
package store

import "strings"

// SearchContainers returns containers whose name contains the query (case-insensitive).
func (s *Store) SearchContainers(q string) []Container {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q = strings.ToLower(q)
	var results []Container
	for _, c := range s.containers {
		if strings.Contains(strings.ToLower(c.Name), q) {
			results = append(results, *c)
		}
	}
	return results
}

// SearchItems returns items whose name contains the query (case-insensitive).
func (s *Store) SearchItems(q string) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q = strings.ToLower(q)
	var results []Item
	for _, item := range s.items {
		if strings.Contains(strings.ToLower(item.Name), q) {
			results = append(results, *item)
		}
	}
	return results
}

// SearchTags returns tags whose name contains the query (case-insensitive).
func (s *Store) SearchTags(q string) []Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q = strings.ToLower(q)
	var results []Tag
	for _, t := range s.tags {
		if strings.Contains(strings.ToLower(t.Name), q) {
			results = append(results, *t)
		}
	}
	return results
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -run TestSearch -v`
Expected: PASS

- [ ] **Step 5: Run all store tests**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store/search.go internal/store/search_test.go
git commit -m "feat(store): add search methods for containers, items, and tags"
```

---

## Task 6: API Endpoints — Tags, Bulk, Search

**Files:**
- Create: `internal/api/handlers_tags.go`
- Create: `internal/api/handlers_bulk.go`
- Create: `internal/api/handlers_search.go`
- Modify: `internal/api/server.go` (route registration)

- [ ] **Step 1: Implement tag API handlers**

Create `internal/api/handlers_tags.go` with handlers following the existing pattern in `server.go`:
- `HandleTags` — GET with optional `?parent_id=` filter
- `HandleTagCreate` — POST, reads `name` and `parent_id` from form/JSON
- `HandleTag` — GET single tag by `{id}`
- `HandleTagUpdate` — PUT, reads `name`
- `HandleTagDelete` — DELETE
- `HandleTagMove` — PATCH, reads JSON `{"parent_id": "..."}`
- `HandleTagDescendants` — GET returns descendant IDs
- `HandleItemTagAdd` — POST on `/api/items/{id}/tags`
- `HandleItemTagRemove` — DELETE on `/api/items/{id}/tags/{tag_id}`
- `HandleContainerTagAdd` — POST on `/api/containers/{id}/tags`
- `HandleContainerTagRemove` — DELETE on `/api/containers/{id}/tags/{tag_id}`

All follow the `webutil.SaveOrFail(w, s.store.Save)` + `webutil.JSON` pattern.

- [ ] **Step 2: Implement bulk API handlers**

Create `internal/api/handlers_bulk.go`:
- `HandleBulkMove` — POST, decodes JSON `{"ids": [{"id","type"}], "target_container_id": "..."}`
- `HandleBulkDelete` — POST, decodes JSON `{"ids": [{"id","type"}]}`
- `HandleBulkTags` — POST, decodes JSON `{"ids": [{"id","type"}], "tag_id": "..."}`

Each splits IDs by type into `itemIDs` and `containerIDs`, calls the corresponding store bulk method.

- [ ] **Step 3: Implement search API handler**

Create `internal/api/handlers_search.go`:
- `HandleSearch` — GET `?q=`, returns `{"containers": [...], "items": [...], "tags": [...]}`

- [ ] **Step 4: Register routes in api/server.go**

Add to `RegisterRoutes`:
```go
// Tags
mux.HandleFunc("GET /api/tags", s.HandleTags)
mux.HandleFunc("POST /api/tags", s.HandleTagCreate)
mux.HandleFunc("GET /api/tags/{id}", s.HandleTag)
mux.HandleFunc("PUT /api/tags/{id}", s.HandleTagUpdate)
mux.HandleFunc("DELETE /api/tags/{id}", s.HandleTagDelete)
mux.HandleFunc("PATCH /api/tags/{id}/move", s.HandleTagMove)
mux.HandleFunc("GET /api/tags/{id}/descendants", s.HandleTagDescendants)

// Tag assignment
mux.HandleFunc("POST /api/items/{id}/tags", s.HandleItemTagAdd)
mux.HandleFunc("DELETE /api/items/{id}/tags/{tag_id}", s.HandleItemTagRemove)
mux.HandleFunc("POST /api/containers/{id}/tags", s.HandleContainerTagAdd)
mux.HandleFunc("DELETE /api/containers/{id}/tags/{tag_id}", s.HandleContainerTagRemove)

// Bulk
mux.HandleFunc("POST /api/bulk/move", s.HandleBulkMove)
mux.HandleFunc("POST /api/bulk/delete", s.HandleBulkDelete)
mux.HandleFunc("POST /api/bulk/tags", s.HandleBulkTags)

// Search
mux.HandleFunc("GET /api/search", s.HandleSearch)
```

- [ ] **Step 5: Run lint and build**

Run: `cd /Users/erxyi/Projekty/qlx && make lint && make build-mac`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api/handlers_tags.go internal/api/handlers_bulk.go internal/api/handlers_search.go internal/api/server.go
git commit -m "feat(api): add tag, bulk, and search API endpoints"
```

---

## Task 7: UI Templates — Partials & New Pages

**Files:**
- Create: `internal/embedded/templates/partials/container_list_item.html`
- Create: `internal/embedded/templates/partials/item_list_item.html`
- Create: `internal/embedded/templates/partials/tag_list_item.html`
- Create: `internal/embedded/templates/partials/tree_picker.html`
- Create: `internal/embedded/templates/partials/tag_chips.html`
- Create: `internal/embedded/templates/tags.html`
- Create: `internal/embedded/templates/search.html`
- Modify: `internal/embedded/templates/layout.html` — add search input and Tags nav link
- Modify: `internal/embedded/templates/containers.html` — add `id` attrs, quick entry forms, checkboxes, tag chips, filter bar

- [ ] **Step 1: Create container_list_item.html partial**

Single `<li>` for a container, matching the existing markup in `containers.html`. Used for quick-entry HTMX response.

```html
{{ define "container-list-item" }}
<li draggable="true" data-id="{{ .ID }}" data-type="container" class="flash">
    <input type="checkbox" class="bulk-select" data-id="{{ .ID }}" data-type="container">
    <a href="/ui/containers/{{ .ID }}" hx-get="/ui/containers/{{ .ID }}" hx-target="#content">{{ .Name }}</a>
</li>
{{ end }}
```

- [ ] **Step 2: Create item_list_item.html partial**

```html
{{ define "item-list-item" }}
<li draggable="true" data-id="{{ .ID }}" data-type="item" class="flash">
    <input type="checkbox" class="bulk-select" data-id="{{ .ID }}" data-type="item">
    <a href="/ui/items/{{ .ID }}" hx-get="/ui/items/{{ .ID }}" hx-target="#content">{{ .Name }}</a>
    {{ if gt .Quantity 1 }}<span class="badge">{{ .Quantity }}</span>{{ end }}
</li>
{{ end }}
```

- [ ] **Step 3: Create tag_list_item.html partial**

```html
{{ define "tag-list-item" }}
<li draggable="true" data-id="{{ .ID }}" data-type="tag" class="flash">
    <a href="/ui/tags?parent={{ .ID }}" hx-get="/ui/tags?parent={{ .ID }}" hx-target="#content">{{ .Name }}</a>
</li>
{{ end }}
```

- [ ] **Step 4: Create tag_chips.html partial**

```html
{{ define "tag-chips" }}
<div class="tag-chips" id="tag-chips-{{ .ObjectID }}">
    {{ range .Tags }}
    <span class="tag-chip">
        {{ .Name }}
        <button class="tag-remove" data-object-id="{{ $.ObjectID }}" data-object-type="{{ $.ObjectType }}" data-tag-id="{{ .ID }}"
                hx-delete="/ui/actions/{{ $.ObjectType }}s/{{ $.ObjectID }}/tags/{{ .ID }}"
                hx-target="#tag-chips-{{ $.ObjectID }}"
                hx-swap="outerHTML">&times;</button>
    </span>
    {{ end }}
    <button class="tag-add" data-object-id="{{ .ObjectID }}" data-object-type="{{ .ObjectType }}">+</button>
</div>
{{ end }}
```

- [ ] **Step 5: Create tree_picker.html partial**

A `<dialog>` with search input and lazy-loaded tree. Used for both move picker and tag picker.

```html
{{ define "tree-picker" }}
<dialog id="{{ .PickerID }}" class="tree-picker-dialog">
    <div class="tree-picker">
        <div class="tree-picker-header">
            <h3>{{ .Title }}</h3>
            <button class="dialog-close" onclick="this.closest('dialog').close()">&times;</button>
        </div>
        <input type="search" class="tree-search" placeholder="Szukaj..."
               hx-get="/ui/partials/{{ .TreeEndpoint }}/search"
               hx-trigger="input changed delay:300ms"
               hx-target="#{{ .PickerID }}-results"
               hx-swap="innerHTML"
               name="q">
        <div id="{{ .PickerID }}-results"
             hx-get="/ui/partials/{{ .TreeEndpoint }}"
             hx-trigger="load"
             hx-vals='{"parent_id": ""}'
             hx-swap="innerHTML">
        </div>
        <div class="tree-picker-footer">
            <button class="btn btn-primary tree-picker-confirm" disabled>Wybierz</button>
            <button class="btn btn-secondary" onclick="this.closest('dialog').close()">Anuluj</button>
        </div>
    </div>
</dialog>
{{ end }}
```

- [ ] **Step 6: Create tags.html page**

Full-page template for tag tree, analogous to `containers.html`. Note: Go's `html/template` package provides `not`, `and`, `or` as built-in functions — no FuncMap entry needed. The existing templates already use these (e.g. `containers.html` line 143).

- [ ] **Step 7: Create search.html page**

Search results page with grouped sections (Containers / Items / Tags).

- [ ] **Step 8: Update layout.html**

Add search input in nav and Tags link:
```html
<nav>
    <a href="/ui" hx-get="/ui" hx-target="#content">QLX</a>
    <a href="/ui/printers" hx-get="/ui/printers" hx-target="#content">Drukarki</a>
    <a href="/ui/templates" hx-get="/ui/templates" hx-target="#content">Szablony</a>
    <a href="/ui/tags" hx-get="/ui/tags" hx-target="#content">Tagi</a>
    <input type="search" id="global-search" placeholder="Szukaj..."
           hx-get="/ui/search"
           hx-trigger="input changed delay:300ms"
           hx-target="#content"
           name="q">
    <span id="printer-status"></span>
</nav>
```

- [ ] **Step 9: Update containers.html**

- Add `id="container-list"` and `id="item-list"` to `<ul>` elements
- Always render `<ul>` (even when empty, with `<li class="empty-state">` inside)
- Add quick entry forms at bottom of each list
- Add checkbox to each `<li>`
- Add tag chips section
- Add tag filter bar

- [ ] **Step 10: Update item.html**

- Add quantity display
- Add tag chips section

- [ ] **Step 11: Commit**

```bash
git add internal/embedded/templates/
git commit -m "feat(ui): add templates for tags, search, quick entry, tree picker, and tag chips"
```

---

## Task 8: UI Server — Register Templates & Routes

**Files:**
- Modify: `internal/ui/server.go`

- [ ] **Step 1: Add new view models**

```go
type TagTreeData struct {
	Tags    []store.Tag
	Parent  *store.Tag
	Path    []store.Tag
}

type SearchResultsData struct {
	Query      string
	Containers []store.Container
	Items      []store.Item
	Tags       []store.Tag
}

type TreePickerData struct {
	PickerID     string
	Title        string
	TreeEndpoint string
}

type TagChipsData struct {
	ObjectID   string
	ObjectType string // "item" or "container"
	Tags       []store.Tag
}
```

- [ ] **Step 2: Register new templates in NewServer**

Add to `templateFiles`:
```go
"tags":    "templates/tags.html",
"search":  "templates/search.html",
```

Add to `sharedFiles` (alongside breadcrumb.html):
```go
"templates/partials/container_list_item.html",
"templates/partials/item_list_item.html",
"templates/partials/tag_list_item.html",
"templates/partials/tree_picker.html",
"templates/partials/tag_chips.html",
```

- [ ] **Step 3: Register new routes**

Add to `RegisterRoutes`:
```go
// Tags UI
mux.HandleFunc("GET /ui/tags", s.HandleTags)
mux.HandleFunc("POST /ui/actions/tags", s.HandleTagCreate)
mux.HandleFunc("PUT /ui/actions/tags/{id}", s.HandleTagUpdate)
mux.HandleFunc("DELETE /ui/actions/tags/{id}", s.HandleTagDelete)
mux.HandleFunc("POST /ui/actions/tags/{id}/move", s.HandleTagMove)

// Tag assignment UI
mux.HandleFunc("POST /ui/actions/items/{id}/tags", s.HandleItemTagAdd)
mux.HandleFunc("DELETE /ui/actions/items/{id}/tags/{tag_id}", s.HandleItemTagRemove)
mux.HandleFunc("POST /ui/actions/containers/{id}/tags", s.HandleContainerTagAdd)
mux.HandleFunc("DELETE /ui/actions/containers/{id}/tags/{tag_id}", s.HandleContainerTagRemove)

// Partials
mux.HandleFunc("GET /ui/partials/tree", s.HandleTreePartial)
mux.HandleFunc("GET /ui/partials/tree/search", s.HandleTreeSearchPartial)
mux.HandleFunc("GET /ui/partials/tag-tree", s.HandleTagTreePartial)
mux.HandleFunc("GET /ui/partials/tag-tree/search", s.HandleTagTreeSearchPartial)

// Bulk operations UI
mux.HandleFunc("POST /ui/actions/bulk/move", s.HandleBulkMove)
mux.HandleFunc("POST /ui/actions/bulk/delete", s.HandleBulkDelete)
mux.HandleFunc("POST /ui/actions/bulk/tags", s.HandleBulkTags)

// Search UI
mux.HandleFunc("GET /ui/search", s.HandleSearch)
```

- [ ] **Step 4: Update ContainerListData to include tag filter**

Add `ActiveTagFilter string` and `AllTags []store.Tag` fields.

- [ ] **Step 5: Build to verify compilation**

Run: `cd /Users/erxyi/Projekty/qlx && make build-mac`
Expected: Build will fail until handlers are implemented (Task 9). That's OK — commit the server.go changes.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/server.go
git commit -m "feat(ui): register tag, bulk, search routes and templates"
```

---

## Task 9: UI Handlers — Tags, Quick Entry, Bulk, Search, Partials

**Files:**
- Create: `internal/ui/handlers_tags.go`
- Create: `internal/ui/handlers_bulk.go`
- Create: `internal/ui/handlers_search.go`
- Create: `internal/ui/handlers_partials.go`
- Modify: `internal/ui/handlers.go` (quick entry response in existing create handlers)

- [ ] **Step 1: Implement tag UI handlers**

`internal/ui/handlers_tags.go`:
- `HandleTags` — renders tag tree page with `TagTreeData`
- `HandleTagCreate` — creates tag. If `HX-Request`: returns `tag-list-item` partial. Else: redirects.
- `HandleTagUpdate` — updates tag name
- `HandleTagDelete` — deletes tag
- `HandleTagMove` — moves tag in hierarchy
- `HandleItemTagAdd` — adds tag to item, returns updated `tag-chips` partial via OOB
- `HandleItemTagRemove` — removes tag from item, returns updated `tag-chips` partial
- `HandleContainerTagAdd` / `HandleContainerTagRemove` — same for containers

All follow `webutil.SaveOrFail(w, s.store.Save)` pattern.

- [ ] **Step 2: Implement bulk UI handlers**

`internal/ui/handlers_bulk.go`:
- `HandleBulkMove` — decodes JSON body `{"ids": [...], "target_container_id": "..."}`, splits by type, calls `s.store.BulkMove`, returns JSON `{"ok": true}` or error
- `HandleBulkDelete` — decodes JSON, calls `s.store.BulkDelete`, returns `{"deleted": [...], "failed": [...]}`
- `HandleBulkTags` — decodes JSON, calls `s.store.BulkAddTag`

- [ ] **Step 3: Implement search UI handler**

`internal/ui/handlers_search.go`:
- `HandleSearch` — reads `q` param, calls store search methods, renders `search` template with `SearchResultsData`. For each result, populates the breadcrumb path.

- [ ] **Step 4: Implement partial handlers**

`internal/ui/handlers_partials.go`:
- `HandleTreePartial` — reads `parent_id`, returns `<ul>` with container children as `<li>` elements (for move picker)
- `HandleTreeSearchPartial` — reads `q`, returns flat list of matching containers with breadcrumb path
- `HandleTagTreePartial` — same pattern for tags
- `HandleTagTreeSearchPartial` — same for tag search

- [ ] **Step 5: Modify existing create handlers for quick entry**

In `internal/ui/handlers.go`:

`HandleContainerCreate`: after creating container and saving, check if request has `HX-Request` header. If so, render `container-list-item` partial template with the new container and return. Otherwise, redirect as before.

`HandleItemCreate`: same pattern — read `quantity` from form (parse as int, default 1), pass to `CreateItem`. If HTMX request, render `item-list-item` partial.

- [ ] **Step 5b: Update HandleContainer/HandleRoot for tag filtering**

In `internal/ui/handlers.go`, update the container view handlers to read `r.URL.Query().Get("tag")`. When a tag filter is active:
- Call `s.store.ItemsByTag(tagID)` and intersect with items in the current container
- For containers, filter those whose `TagIDs` intersect with `{tagID} ∪ TagDescendants(tagID)`
- Pass `ActiveTagFilter` and `AllTags` to the view model so the template can render the filter bar

- [ ] **Step 6: Build and lint**

Run: `cd /Users/erxyi/Projekty/qlx && make lint && make build-mac`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ui/handlers_tags.go internal/ui/handlers_bulk.go internal/ui/handlers_search.go internal/ui/handlers_partials.go internal/ui/handlers.go
git commit -m "feat(ui): add tag, bulk, search, partial handlers and quick entry mode"
```

---

## Task 10: Frontend JS — Selection, Action Bar, Multi-Drag, Pickers

**Files:**
- Modify: `internal/embedded/static/ui-lite.js`

- [ ] **Step 1: Add selection module**

Add to `ui-lite.js`:
- `selection` Set for tracking `{id, type}` selections
- `initBulkSelect()` — attaches change events to `.bulk-select` checkboxes
- `toggleSelectionMode()` — shows/hides checkboxes
- `updateActionBar()` — creates/shows/hides sticky action bar based on selection size

- [ ] **Step 2: Add action bar with Move/Tag/Delete buttons**

Dynamically create action bar element on first selection. Buttons:
- "Przenieś do..." → opens move picker dialog
- "Taguj..." → opens tag picker dialog
- "Usuń zaznaczone" → opens confirmation dialog
- "Odznacz" → clears selection

- [ ] **Step 3: Add move picker interaction**

- `openMovePicker()` — calls `document.getElementById('move-picker').showModal()`
- Tree navigation: click on expand arrow loads children via HTMX (already handled by `hx-get` on the partial)
- Selection of target: click on container row highlights it, enables "Wybierz" button
- Confirm: sends `POST /ui/actions/bulk/move` with JSON body, reloads current view

- [ ] **Step 4: Add tag picker interaction**

Same pattern as move picker but for tags.

- [ ] **Step 5: Add multi-drag support**

Extend existing `initDragDrop()`:
- On `dragstart`: check if dragged element's ID is in `selection`. If yes, set drag data to include all selected IDs. Create a composite drag image with badge showing count.
- On `drop`: if multiple IDs in drag data, send `POST /ui/actions/bulk/move` instead of single move.

- [ ] **Step 6: Add bulk delete confirmation**

- `openDeleteConfirm()` — creates/opens `<dialog>` with count and confirm/cancel buttons
- On confirm: sends `POST /ui/actions/bulk/delete` with JSON, reads response, removes deleted elements from DOM, shows toast for failures

- [ ] **Step 7: Re-initialize after HTMX swaps**

Ensure `initBulkSelect()` and `initDragDrop()` are called on `htmx:afterSwap`.

- [ ] **Step 8: Commit**

```bash
git add internal/embedded/static/ui-lite.js
git commit -m "feat(ui): add selection, action bar, multi-drag, move/tag pickers in JS"
```

---

## Task 11: CSS — Action Bar, Flash, Checkboxes, Dialogs, Tags, Search

**Files:**
- Modify: `internal/embedded/static/style.css`

- [ ] **Step 1: Add flash animation**

```css
@keyframes flash-highlight {
    0% { background-color: var(--accent); }
    100% { background-color: transparent; }
}
.flash {
    animation: flash-highlight 1s ease-out;
}
```

- [ ] **Step 2: Add checkbox and action bar styles**

```css
.bulk-select {
    display: none;
    margin-right: 0.5rem;
}
.selection-mode .bulk-select {
    display: inline-block;
}

.action-bar {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    background: var(--bg-card);
    border-top: 1px solid var(--border);
    padding: 0.75rem 1rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    z-index: 100;
}
.action-bar .action-count {
    margin-right: auto;
    color: var(--text-muted);
}
```

- [ ] **Step 3: Add dialog/picker styles**

```css
.tree-picker-dialog {
    max-width: 500px;
    width: 90vw;
    max-height: 70vh;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--bg-card);
    padding: 0;
}
.tree-picker-dialog::backdrop {
    background: rgba(0, 0, 0, 0.5);
}
.tree-picker {
    display: flex;
    flex-direction: column;
    height: 100%;
}
.tree-picker-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem;
    border-bottom: 1px solid var(--border);
}
.tree-search {
    margin: 0.75rem 1rem;
}
.tree-picker-footer {
    padding: 1rem;
    border-top: 1px solid var(--border);
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
}
```

- [ ] **Step 4: Add tag chip styles**

```css
.tag-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
    align-items: center;
}
.tag-chip {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.15rem 0.5rem;
    background: var(--bg-alt);
    border-radius: 12px;
    font-size: 0.85rem;
}
.tag-chip .tag-remove {
    background: none;
    border: none;
    cursor: pointer;
    color: var(--text-muted);
    padding: 0;
    font-size: 1rem;
    line-height: 1;
}
.tag-add {
    background: none;
    border: 1px dashed var(--border);
    border-radius: 12px;
    cursor: pointer;
    padding: 0.15rem 0.5rem;
    color: var(--text-muted);
    font-size: 0.85rem;
}
```

- [ ] **Step 5: Add search and quick entry styles**

```css
#global-search {
    flex: 0 1 200px;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    border: 1px solid var(--border);
    background: var(--bg-alt);
    color: var(--text);
}

.quick-entry {
    display: flex;
    gap: 0.5rem;
    padding: 0.5rem 0;
}
.quick-entry input[type="text"] {
    flex: 1;
}
.quick-entry input[type="number"] {
    width: 4rem;
}

.empty-state {
    color: var(--text-muted);
    font-style: italic;
    list-style: none;
}
```

- [ ] **Step 6: Add tag filter bar styles**

```css
.tag-filter-bar {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
    padding: 0.5rem 0;
    align-items: center;
}
.tag-filter-bar .tag-chip {
    cursor: pointer;
}
.tag-filter-bar .tag-chip.active {
    background: var(--accent);
    color: white;
}
```

- [ ] **Step 7: Commit**

```bash
git add internal/embedded/static/style.css
git commit -m "feat(ui): add CSS for action bar, flash animation, dialogs, tag chips, search"
```

---

## Task 12: Integration Testing & Final Verification

**Files:**
- All modified files

- [ ] **Step 1: Run all store tests**

Run: `cd /Users/erxyi/Projekty/qlx && go test ./internal/store/ -v`
Expected: ALL PASS

- [ ] **Step 2: Run full test suite**

Run: `cd /Users/erxyi/Projekty/qlx && make test`
Expected: ALL PASS

- [ ] **Step 3: Run linter**

Run: `cd /Users/erxyi/Projekty/qlx && make lint`
Expected: No errors

- [ ] **Step 4: Build for Mac**

Run: `cd /Users/erxyi/Projekty/qlx && make build-mac`
Expected: Binary builds successfully

- [ ] **Step 5: Build for MIPS (cross-compile check)**

Run: `cd /Users/erxyi/Projekty/qlx && make build-mips`
Expected: Binary builds successfully (no CGO deps in new code)

- [ ] **Step 6: Manual smoke test**

Run: `cd /Users/erxyi/Projekty/qlx && make run`

Verify in browser at `http://localhost:8080/ui`:
1. Quick entry: add multiple containers by typing name + Enter repeatedly
2. Quick entry: add multiple items with quantity
3. Navigate to Tags page, create tag hierarchy
4. Assign tags to items/containers
5. Use tag filter on container view
6. Multi-select items, move to another container
7. Multi-select items, bulk delete
8. Global search works
9. Drag-and-drop still works for single items

- [ ] **Step 7: Commit any fixes from smoke testing**

```bash
git add -u
git commit -m "fix: address issues found during integration testing"
```
