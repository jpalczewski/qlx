package sqlite

import "github.com/erxyi/qlx/internal/store"

// AllItems returns every item in the store, ordered by name.
func (s *SQLiteStore) AllItems() []store.Item {
	rows, err := s.db.Query(
		`SELECT id, container_id, name, description, color, icon, quantity, created_at FROM items ORDER BY name`)
	if err != nil {
		return nil
	}

	// First pass: drain the cursor before issuing secondary queries (single-connection pool).
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

// ExportData returns maps of all containers and items keyed by their IDs.
// It satisfies the store.ExportStore interface.
// AllContainers is already implemented in containers.go.
func (s *SQLiteStore) ExportData() (map[string]*store.Container, map[string]*store.Item) {
	containers := s.AllContainers()
	items := s.AllItems()

	cMap := make(map[string]*store.Container, len(containers))
	for i := range containers {
		c := containers[i]
		cMap[c.ID] = &c
	}

	iMap := make(map[string]*store.Item, len(items))
	for i := range items {
		item := items[i]
		iMap[item.ID] = &item
	}

	return cMap, iMap
}

// ExportItems returns denormalized items for a container, optionally including
// items from descendant containers. This is a stub — full implementation in Task 3.
func (s *SQLiteStore) ExportItems(containerID string, recursive bool) ([]store.ExportItem, error) {
	return nil, nil
}

// ExportContainerTree returns the container and all its descendants in
// breadth-first order. This is a stub — full implementation in Task 3.
func (s *SQLiteStore) ExportContainerTree(containerID string) ([]store.Container, error) {
	return nil, nil
}
