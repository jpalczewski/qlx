package sqlite

import (
	"github.com/erxyi/qlx/internal/store"
	"github.com/google/uuid"
)

// scanItem scans a single row into a store.Item.
// It expects columns: id, container_id, name, description, color, icon, quantity, created_at.
func scanItem(row interface {
	Scan(dest ...any) error
}) (store.Item, error) {
	var item store.Item
	err := row.Scan(&item.ID, &item.ContainerID, &item.Name, &item.Description,
		&item.Color, &item.Icon, &item.Quantity, &item.CreatedAt)
	return item, err
}

// GetItem returns the item with the given ID, or nil if not found.
func (s *SQLiteStore) GetItem(id string) *store.Item {
	row := s.db.QueryRow(
		`SELECT id, container_id, name, description, color, icon, quantity, created_at FROM items WHERE id = ?`, id)
	item, err := scanItem(row)
	if err != nil {
		return nil
	}
	item.TagIDs = s.itemTagIDs(item.ID)
	return &item
}

// CreateItem inserts a new item into the given container and returns it, or nil on error.
func (s *SQLiteStore) CreateItem(containerID, name, desc string, qty int, color, icon string) *store.Item {
	id := uuid.New().String()
	_, err := s.db.Exec(
		`INSERT INTO items (id, container_id, name, description, quantity, color, icon) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, containerID, name, desc, qty, color, icon)
	if err != nil {
		return nil
	}
	return s.GetItem(id)
}

// UpdateItem updates the mutable fields of an item and returns the updated record.
// Returns store.ErrItemNotFound if no row matched.
func (s *SQLiteStore) UpdateItem(id, name, desc string, qty int, color, icon string) (*store.Item, error) {
	res, err := s.db.Exec(
		`UPDATE items SET name=?, description=?, quantity=?, color=?, icon=?, updated_at=datetime('now') WHERE id=?`,
		name, desc, qty, color, icon, id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, store.ErrItemNotFound
	}
	return s.GetItem(id), nil
}

// DeleteItem removes an item by ID and returns its container ID.
// Returns store.ErrItemNotFound if no row matched.
func (s *SQLiteStore) DeleteItem(id string) (string, error) {
	item := s.GetItem(id)
	if item == nil {
		return "", store.ErrItemNotFound
	}
	containerID := item.ContainerID
	_, err := s.db.Exec(`DELETE FROM items WHERE id = ?`, id)
	if err != nil {
		return "", err
	}
	return containerID, nil
}

// MoveItem sets a new container for the given item.
// Returns store.ErrItemNotFound if no row matched.
// Returns store.ErrInvalidContainer if the target container does not exist.
func (s *SQLiteStore) MoveItem(id, containerID string) error {
	if s.GetContainer(containerID) == nil {
		return store.ErrInvalidContainer
	}
	res, err := s.db.Exec(
		`UPDATE items SET container_id=?, updated_at=datetime('now') WHERE id=?`, containerID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrItemNotFound
	}
	return nil
}
