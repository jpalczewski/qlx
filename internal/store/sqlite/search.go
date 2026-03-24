package sqlite

import (
	"strings"

	"github.com/erxyi/qlx/internal/store"
)

// fts5Query prepares a query string for FTS5 MATCH.
// It trims whitespace and appends '*' for prefix matching.
// If the query contains FTS5 special characters, it falls back to quoting.
func fts5Query(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	// If the query contains special FTS5 operators, wrap in double quotes for literal match.
	if strings.ContainsAny(q, `"*^(){}[]+-:~<>&|!'\/`) {
		return `"` + strings.ReplaceAll(q, `"`, `""`) + `"`
	}
	return q + "*"
}

// SearchItems performs a full-text search over items using the FTS5 index.
// Returns nil for an empty query.
func (s *SQLiteStore) SearchItems(query string) []store.Item {
	fq := fts5Query(query)
	if fq == "" {
		return nil
	}
	rows, err := s.db.Query(`
		SELECT i.id, i.container_id, i.name, i.description, i.color, i.icon, i.quantity, i.created_at
		FROM items i
		JOIN items_fts ON items_fts.rowid = i.rowid
		WHERE items_fts MATCH ?
		ORDER BY rank`, fq)
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

// SearchContainers performs a full-text search over containers using the FTS5 index.
// Returns nil for an empty query.
func (s *SQLiteStore) SearchContainers(query string) []store.Container {
	fq := fts5Query(query)
	if fq == "" {
		return nil
	}
	rows, err := s.db.Query(`
		SELECT c.id, COALESCE(c.parent_id, ''), c.name, c.description, c.color, c.icon, c.created_at
		FROM containers c
		JOIN containers_fts ON containers_fts.rowid = c.rowid
		WHERE containers_fts MATCH ?
		ORDER BY rank`, fq)
	if err != nil {
		return nil
	}
	return s.scanContainers(rows)
}

// SearchNotes performs a full-text search over notes using the FTS5 index.
// Returns nil for an empty query.
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

	notes := make([]store.Note, 0)
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			continue
		}
		notes = append(notes, note)
	}
	return notes
}

// SearchTags searches tag names using a LIKE pattern match.
// Returns nil for an empty query.
func (s *SQLiteStore) SearchTags(query string) []store.Tag {
	if query == "" {
		return nil
	}
	rows, err := s.db.Query(`
		SELECT `+tagSelectCols+`
		FROM tags
		WHERE name LIKE ?
		ORDER BY name`, "%"+query+"%")
	if err != nil {
		return nil
	}
	return s.scanTags(rows)
}
