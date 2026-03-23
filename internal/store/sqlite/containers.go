package sqlite

import (
	"database/sql"

	"github.com/erxyi/qlx/internal/store"
	"github.com/google/uuid"
)

// scanContainer scans a single row into a store.Container.
// It expects columns: id, parent_id (COALESCE to ”), name, description, color, icon, created_at.
func scanContainer(row interface {
	Scan(dest ...any) error
}) (store.Container, error) {
	var c store.Container
	err := row.Scan(&c.ID, &c.ParentID, &c.Name, &c.Description, &c.Color, &c.Icon, &c.CreatedAt)
	return c, err
}

const containerSelectCols = `id, COALESCE(parent_id, ''), name, description, color, icon, created_at`

// GetContainer returns the container with the given ID, or nil if not found.
func (s *SQLiteStore) GetContainer(id string) *store.Container {
	row := s.db.QueryRow(
		`SELECT `+containerSelectCols+` FROM containers WHERE id = ?`, id)
	c, err := scanContainer(row)
	if err != nil {
		return nil
	}
	c.TagIDs = s.containerTagIDs(c.ID)
	return &c
}

// CreateContainer inserts a new container and returns it, or nil on error.
func (s *SQLiteStore) CreateContainer(parentID, name, desc, color, icon string) *store.Container {
	id := uuid.New().String()
	var pid *string
	if parentID != "" {
		pid = &parentID
	}
	_, err := s.db.Exec(
		`INSERT INTO containers (id, parent_id, name, description, color, icon) VALUES (?, ?, ?, ?, ?, ?)`,
		id, pid, name, desc, color, icon)
	if err != nil {
		return nil
	}
	return s.GetContainer(id)
}

// UpdateContainer updates the mutable fields of a container and returns the updated record.
// Returns store.ErrContainerNotFound if no row matched.
func (s *SQLiteStore) UpdateContainer(id, name, desc, color, icon string) (*store.Container, error) {
	res, err := s.db.Exec(
		`UPDATE containers SET name=?, description=?, color=?, icon=?, updated_at=datetime('now') WHERE id=?`,
		name, desc, color, icon, id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, store.ErrContainerNotFound
	}
	return s.GetContainer(id), nil
}

// DeleteContainer removes a container by ID and returns its parent ID (empty string for root).
// Returns store.ErrContainerNotFound if no row matched.
// Returns store.ErrContainerHasChildren if the container has child containers.
// Returns store.ErrContainerHasItems if the container has items.
func (s *SQLiteStore) DeleteContainer(id string) (string, error) {
	c := s.GetContainer(id)
	if c == nil {
		return "", store.ErrContainerNotFound
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM containers WHERE parent_id = ?`, id).Scan(&count); err != nil {
		return "", err
	}
	if count > 0 {
		return "", store.ErrContainerHasChildren
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM items WHERE container_id = ?`, id).Scan(&count); err != nil {
		return "", err
	}
	if count > 0 {
		return "", store.ErrContainerHasItems
	}

	parentID := c.ParentID
	_, err := s.db.Exec(`DELETE FROM containers WHERE id = ?`, id)
	if err != nil {
		return "", err
	}
	return parentID, nil
}

// MoveContainer sets a new parent for the given container.
// Pass an empty string for newParentID to move the container to root.
// Returns store.ErrContainerNotFound if no row matched.
// Returns store.ErrCycleDetected if the move would create a cycle.
func (s *SQLiteStore) MoveContainer(id, newParentID string) error {
	if newParentID != "" {
		if newParentID == id {
			return store.ErrCycleDetected
		}
		descendants := s.containerDescendants(id)
		for _, d := range descendants {
			if d == newParentID {
				return store.ErrCycleDetected
			}
		}
	}

	var pid *string
	if newParentID != "" {
		pid = &newParentID
	}
	res, err := s.db.Exec(
		`UPDATE containers SET parent_id=?, updated_at=datetime('now') WHERE id=?`, pid, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrContainerNotFound
	}
	return nil
}

// containerDescendants returns all descendant container IDs of the given container using BFS.
func (s *SQLiteStore) containerDescendants(id string) []string {
	var result []string
	queue := []string{id}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		children := s.ContainerChildren(current)
		for _, c := range children {
			result = append(result, c.ID)
			queue = append(queue, c.ID)
		}
	}
	return result
}

// ContainerChildren returns the direct children of the container with the given parentID.
// Pass an empty string to list root containers (those with no parent).
func (s *SQLiteStore) ContainerChildren(parentID string) []store.Container {
	var (
		rows *sql.Rows
		err  error
	)
	if parentID == "" {
		rows, err = s.db.Query(
			`SELECT ` + containerSelectCols + ` FROM containers WHERE parent_id IS NULL ORDER BY name`)
	} else {
		rows, err = s.db.Query(
			`SELECT `+containerSelectCols+` FROM containers WHERE parent_id = ? ORDER BY name`, parentID)
	}
	if err != nil {
		return nil
	}
	return s.scanContainers(rows)
}

// ContainerItems returns all items that belong to the given container.
func (s *SQLiteStore) ContainerItems(containerID string) []store.Item {
	rows, err := s.db.Query(
		`SELECT id, container_id, name, description, color, icon, quantity, created_at FROM items WHERE container_id = ? ORDER BY name`,
		containerID)
	if err != nil {
		return nil
	}

	// First pass: drain the cursor before issuing secondary queries.
	var items []store.Item
	for rows.Next() {
		var item store.Item
		if err := rows.Scan(&item.ID, &item.ContainerID, &item.Name, &item.Description,
			&item.Color, &item.Icon, &item.Quantity, &item.CreatedAt); err != nil {
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

// ContainerPath returns the path from the root container down to (and including) the container
// with the given ID.
func (s *SQLiteStore) ContainerPath(id string) []store.Container {
	var path []store.Container
	current := id
	for current != "" {
		c := s.GetContainer(current)
		if c == nil {
			break
		}
		path = append([]store.Container{*c}, path...)
		current = c.ParentID
	}
	return path
}

// AllContainers returns every container in the store, ordered by name.
func (s *SQLiteStore) AllContainers() []store.Container {
	rows, err := s.db.Query(
		`SELECT ` + containerSelectCols + ` FROM containers ORDER BY name`)
	if err != nil {
		return nil
	}
	return s.scanContainers(rows)
}

// scanContainers iterates over rows and builds a slice of containers with their tag IDs.
// It fully drains and closes rows before issuing any secondary queries to avoid deadlock
// on the single-connection SQLite pool.
func (s *SQLiteStore) scanContainers(rows *sql.Rows) []store.Container {
	// First pass: scan all containers without tag IDs so the rows cursor is closed.
	var containers []store.Container
	for rows.Next() {
		c, err := scanContainer(rows)
		if err != nil {
			continue
		}
		containers = append(containers, c)
	}
	// Close rows before issuing secondary queries.
	_ = rows.Close()

	// Second pass: populate tag IDs now that the rows are closed.
	for i := range containers {
		containers[i].TagIDs = s.containerTagIDs(containers[i].ID)
	}
	return containers
}

// containerTagIDs returns the tag IDs associated with the given container.
func (s *SQLiteStore) containerTagIDs(containerID string) []string {
	rows, err := s.db.Query(`SELECT tag_id FROM container_tags WHERE container_id = ?`, containerID)
	if err != nil {
		return []string{}
	}
	defer func() { _ = rows.Close() }()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// itemTagIDs returns the tag IDs associated with the given item.
func (s *SQLiteStore) itemTagIDs(itemID string) []string {
	rows, err := s.db.Query(`SELECT tag_id FROM item_tags WHERE item_id = ?`, itemID)
	if err != nil {
		return []string{}
	}
	defer func() { _ = rows.Close() }()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
