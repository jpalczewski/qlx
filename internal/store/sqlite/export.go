package sqlite

import (
	"strings"

	"github.com/erxyi/qlx/internal/store"
)

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

	var containers []store.Container
	for rows.Next() {
		var c store.Container
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.Description, &c.Color, &c.Icon, &c.CreatedAt); err != nil {
			continue
		}
		containers = append(containers, c)
	}
	rowsErr := rows.Err()
	_ = rows.Close()
	if rowsErr != nil {
		return nil, rowsErr
	}

	for i := range containers {
		containers[i].TagIDs = s.containerTagIDs(containers[i].ID)
	}
	return containers, nil
}

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
	rowsErr := rows.Err()
	_ = rows.Close()
	if rowsErr != nil {
		return nil, rowsErr
	}
	return items, nil
}
