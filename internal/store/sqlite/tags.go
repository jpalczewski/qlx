package sqlite

import (
	"database/sql"
	"time"

	"github.com/erxyi/qlx/internal/store"
	"github.com/google/uuid"
)

// scanTag scans a single row into a store.Tag.
// It expects columns: id, parent_id (nullable), name, color, icon, created_at, updated_at.
func scanTag(row interface {
	Scan(dest ...any) error
}) (store.Tag, error) {
	var t store.Tag
	var parentID *string
	var updatedAt time.Time
	err := row.Scan(&t.ID, &parentID, &t.Name, &t.Color, &t.Icon, &t.CreatedAt, &updatedAt)
	if parentID != nil {
		t.ParentID = *parentID
	}
	return t, err
}

const tagSelectCols = `id, parent_id, name, color, icon, created_at, updated_at`

// GetTag returns the tag with the given ID, or nil if not found.
func (s *SQLiteStore) GetTag(id string) *store.Tag {
	row := s.db.QueryRow(
		`SELECT `+tagSelectCols+` FROM tags WHERE id = ?`, id)
	t, err := scanTag(row)
	if err != nil {
		return nil
	}
	return &t
}

// CreateTag inserts a new tag (optionally with a parent) and returns it, or nil on error.
func (s *SQLiteStore) CreateTag(parentID, name, color, icon string) *store.Tag {
	id := uuid.New().String()
	var pid *string
	if parentID != "" {
		pid = &parentID
	}
	_, err := s.db.Exec(
		`INSERT INTO tags (id, parent_id, name, color, icon) VALUES (?, ?, ?, ?, ?)`,
		id, pid, name, color, icon)
	if err != nil {
		return nil
	}
	return s.GetTag(id)
}

// UpdateTag updates the mutable fields of a tag and returns the updated record.
// Returns store.ErrTagNotFound if no row matched.
func (s *SQLiteStore) UpdateTag(id, name, color, icon string) (*store.Tag, error) {
	res, err := s.db.Exec(
		`UPDATE tags SET name=?, color=?, icon=?, updated_at=datetime('now') WHERE id=?`,
		name, color, icon, id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, store.ErrTagNotFound
	}
	return s.GetTag(id), nil
}

// DeleteTag removes a tag by ID and returns its parent ID (empty string for root tags).
// Returns store.ErrTagNotFound if no row matched.
// The ON DELETE CASCADE on item_tags and container_tags handles junction cleanup automatically.
func (s *SQLiteStore) DeleteTag(id string) (string, error) {
	t := s.GetTag(id)
	if t == nil {
		return "", store.ErrTagNotFound
	}
	parentID := t.ParentID
	_, err := s.db.Exec(`DELETE FROM tags WHERE id = ?`, id)
	if err != nil {
		return "", err
	}
	return parentID, nil
}

// MoveTag sets a new parent for the given tag.
// Pass an empty string for newParentID to move the tag to the root.
// Returns store.ErrTagNotFound if no row matched.
func (s *SQLiteStore) MoveTag(id, newParentID string) error {
	var pid *string
	if newParentID != "" {
		pid = &newParentID
	}
	res, err := s.db.Exec(
		`UPDATE tags SET parent_id=?, updated_at=datetime('now') WHERE id=?`, pid, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrTagNotFound
	}
	return nil
}

// AllTags returns every tag in the store, ordered by name.
func (s *SQLiteStore) AllTags() []store.Tag {
	rows, err := s.db.Query(
		`SELECT ` + tagSelectCols + ` FROM tags ORDER BY name`)
	if err != nil {
		return nil
	}
	return s.scanTags(rows)
}

// TagChildren returns the direct children of the tag with the given parentID.
// Pass an empty string to list root tags (those with no parent).
func (s *SQLiteStore) TagChildren(parentID string) []store.Tag {
	var (
		rows *sql.Rows
		err  error
	)
	if parentID == "" {
		rows, err = s.db.Query(
			`SELECT ` + tagSelectCols + ` FROM tags WHERE parent_id IS NULL ORDER BY name`)
	} else {
		rows, err = s.db.Query(
			`SELECT `+tagSelectCols+` FROM tags WHERE parent_id = ? ORDER BY name`, parentID)
	}
	if err != nil {
		return nil
	}
	return s.scanTags(rows)
}

// TagPath returns the path from the root tag down to (and including) the tag with the given ID.
func (s *SQLiteStore) TagPath(id string) []store.Tag {
	var path []store.Tag
	current := id
	for current != "" {
		t := s.GetTag(current)
		if t == nil {
			break
		}
		path = append([]store.Tag{*t}, path...)
		current = t.ParentID
	}
	return path
}

// TagDescendants returns all descendant tag IDs of the given tag using BFS.
func (s *SQLiteStore) TagDescendants(id string) []string {
	var result []string
	queue := []string{id}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		children := s.TagChildren(current)
		for _, c := range children {
			result = append(result, c.ID)
			queue = append(queue, c.ID)
		}
	}
	return result
}

// AddItemTag associates a tag with an item.
func (s *SQLiteStore) AddItemTag(itemID, tagID string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO item_tags (item_id, tag_id) VALUES (?, ?)`, itemID, tagID)
	return err
}

// RemoveItemTag removes the association between a tag and an item.
func (s *SQLiteStore) RemoveItemTag(itemID, tagID string) error {
	_, err := s.db.Exec(
		`DELETE FROM item_tags WHERE item_id = ? AND tag_id = ?`, itemID, tagID)
	return err
}

// AddContainerTag associates a tag with a container.
func (s *SQLiteStore) AddContainerTag(containerID, tagID string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO container_tags (container_id, tag_id) VALUES (?, ?)`, containerID, tagID)
	return err
}

// RemoveContainerTag removes the association between a tag and a container.
func (s *SQLiteStore) RemoveContainerTag(containerID, tagID string) error {
	_, err := s.db.Exec(
		`DELETE FROM container_tags WHERE container_id = ? AND tag_id = ?`, containerID, tagID)
	return err
}

// ItemsByTag returns all items associated with the given tag.
func (s *SQLiteStore) ItemsByTag(tagID string) []store.Item {
	rows, err := s.db.Query(`
		SELECT i.id, i.container_id, i.name, i.description, i.color, i.icon, i.quantity, i.created_at
		FROM items i
		JOIN item_tags it ON it.item_id = i.id
		WHERE it.tag_id = ?`, tagID)
	if err != nil {
		return nil
	}

	// First pass: drain rows before issuing secondary queries (single-connection pool).
	var items []store.Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	_ = rows.Close()

	// Second pass: populate tag IDs.
	for i := range items {
		items[i].TagIDs = s.itemTagIDs(items[i].ID)
	}
	return items
}

// ContainersByTag returns all containers associated with the given tag.
func (s *SQLiteStore) ContainersByTag(tagID string) []store.Container {
	rows, err := s.db.Query(`
		SELECT c.id, COALESCE(c.parent_id, ''), c.name, c.description, c.color, c.icon, c.created_at
		FROM containers c
		JOIN container_tags ct ON ct.container_id = c.id
		WHERE ct.tag_id = ?`, tagID)
	if err != nil {
		return nil
	}
	return s.scanContainers(rows)
}

// ResolveTagIDs returns the Tag records for the given IDs (skipping any not found).
// Returns nil for empty or nil input.
func (s *SQLiteStore) ResolveTagIDs(ids []string) []store.Tag {
	if len(ids) == 0 {
		return nil
	}
	var result []store.Tag
	for _, id := range ids {
		t := s.GetTag(id)
		if t != nil {
			result = append(result, *t)
		}
	}
	return result
}

// TagItemStats returns the count of distinct items and the total quantity for the given tag.
func (s *SQLiteStore) TagItemStats(id string) (int, int, error) {
	var count, qty int
	err := s.db.QueryRow(`
		SELECT COUNT(i.id), COALESCE(SUM(i.quantity), 0)
		FROM item_tags it
		JOIN items i ON i.id = it.item_id
		WHERE it.tag_id = ?`, id).Scan(&count, &qty)
	return count, qty, err
}

// scanTags iterates over rows and builds a slice of tags.
// Rows are fully drained and closed before returning.
func (s *SQLiteStore) scanTags(rows *sql.Rows) []store.Tag {
	var tags []store.Tag
	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			continue
		}
		tags = append(tags, t)
	}
	_ = rows.Close()
	return tags
}
