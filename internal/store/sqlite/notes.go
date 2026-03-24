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
		`INSERT INTO notes (id, container_id, item_id, title, content, color, icon, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, strftime('%Y-%m-%d %H:%M:%f', 'now'))`,
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
		`SELECT `+noteSelectCols+` FROM notes WHERE container_id = ? ORDER BY created_at DESC, rowid DESC`, containerID)
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
		`SELECT `+noteSelectCols+` FROM notes WHERE item_id = ? ORDER BY created_at DESC, rowid DESC`, itemID)
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
