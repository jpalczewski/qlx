package sqlite

import (
	"encoding/json"
	"fmt"

	"github.com/erxyi/qlx/internal/store"
	"github.com/google/uuid"
)

// scanTemplate scans a single row into a store.Template.
// It expects columns: id, name, tags, target, width_mm, height_mm, width_px, height_px, elements, created_at, updated_at.
func scanTemplate(row interface {
	Scan(dest ...any) error
}) (store.Template, error) {
	var t store.Template
	var tagsJSON string
	err := row.Scan(&t.ID, &t.Name, &tagsJSON, &t.Target, &t.WidthMM, &t.HeightMM, &t.WidthPx, &t.HeightPx, &t.Elements, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	if err := json.Unmarshal([]byte(tagsJSON), &t.Tags); err != nil {
		t.Tags = nil
	}
	return t, nil
}

// AllTemplates returns all templates ordered by name.
func (s *SQLiteStore) AllTemplates() []store.Template {
	rows, err := s.db.Query(`SELECT id, name, tags, target, width_mm, height_mm, width_px, height_px, elements, created_at, updated_at FROM templates ORDER BY name`)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()
	var result []store.Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err == nil {
			result = append(result, t)
		}
	}
	return result
}

// GetTemplate returns the template with the given ID, or nil if not found.
func (s *SQLiteStore) GetTemplate(id string) *store.Template {
	row := s.db.QueryRow(`SELECT id, name, tags, target, width_mm, height_mm, width_px, height_px, elements, created_at, updated_at FROM templates WHERE id = ?`, id)
	t, err := scanTemplate(row)
	if err != nil {
		return nil
	}
	return &t
}

// CreateTemplate inserts a new template and returns it.
func (s *SQLiteStore) CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*store.Template, error) {
	id := uuid.New().String()
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return nil, fmt.Errorf("marshal tags: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO templates (id, name, tags, target, width_mm, height_mm, width_px, height_px, elements) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, name, string(tagsJSON), target, widthMM, heightMM, widthPx, heightPx, elements)
	if err != nil {
		return nil, err
	}
	return s.GetTemplate(id), nil
}

// UpdateTemplate updates the mutable fields of a template and returns the updated record.
// Returns store.ErrTemplateNotFound if no row matched.
func (s *SQLiteStore) UpdateTemplate(id, name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*store.Template, error) {
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return nil, fmt.Errorf("marshal tags: %w", err)
	}
	res, err := s.db.Exec(
		`UPDATE templates SET name=?, tags=?, target=?, width_mm=?, height_mm=?, width_px=?, height_px=?, elements=?, updated_at=datetime('now') WHERE id=?`,
		name, string(tagsJSON), target, widthMM, heightMM, widthPx, heightPx, elements, id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, store.ErrTemplateNotFound
	}
	return s.GetTemplate(id), nil
}

// DeleteTemplate removes a template by ID.
// Returns store.ErrTemplateNotFound if no row matched.
func (s *SQLiteStore) DeleteTemplate(id string) error {
	res, err := s.db.Exec(`DELETE FROM templates WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrTemplateNotFound
	}
	return nil
}
