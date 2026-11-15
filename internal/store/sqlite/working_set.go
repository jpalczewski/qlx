package sqlite

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/erxyi/qlx/internal/store"
)

func (s *SQLiteStore) containerExists(id string) bool {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM containers WHERE id = ?`, id).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// PinContainer pins a container at the end of the pin list. Pinning an already
// pinned container is a no-op. Returns store.ErrContainerNotFound for unknown IDs.
func (s *SQLiteStore) PinContainer(id string) error {
	if !s.containerExists(id) {
		return store.ErrContainerNotFound
	}
	_, err := s.db.Exec(`
		INSERT INTO container_pins (container_id, position, created_at)
		VALUES (?, (SELECT COALESCE(MAX(position), -1) + 1 FROM container_pins), ?)
		ON CONFLICT(container_id) DO NOTHING`,
		id, time.Now().UnixMilli())
	return err
}

// UnpinContainer removes a pin. Unpinning a container that is not pinned is a no-op.
func (s *SQLiteStore) UnpinContainer(id string) error {
	_, err := s.db.Exec(`DELETE FROM container_pins WHERE container_id = ?`, id)
	return err
}

// ReorderPins assigns positions following the order of ids. Ids that are not
// pinned are ignored; pinned containers not listed keep their relative order
// after the listed ones.
func (s *SQLiteStore) ReorderPins(ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.Query(`SELECT container_id FROM container_pins ORDER BY position, created_at, container_id`)
	if err != nil {
		return err
	}
	var current []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		current = append(current, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()

	pinned := make(map[string]bool, len(current))
	for _, id := range current {
		pinned[id] = true
	}
	ordered := make([]string, 0, len(current))
	seen := make(map[string]bool, len(current))
	for _, id := range ids {
		if pinned[id] && !seen[id] {
			seen[id] = true
			ordered = append(ordered, id)
		}
	}
	for _, id := range current {
		if !seen[id] {
			ordered = append(ordered, id)
		}
	}

	for i, id := range ordered {
		if _, err := tx.Exec(`UPDATE container_pins SET position = ? WHERE container_id = ?`, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Pins returns pinned container IDs in display order.
func (s *SQLiteStore) Pins() []string {
	rows, err := s.db.Query(`SELECT container_id FROM container_pins ORDER BY position, created_at, container_id`)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// RecordContainerOpen upserts a recent entry, incrementing open_count, and
// trims the table to the newest store.MaxRecentContainers entries.
// Returns store.ErrContainerNotFound for unknown IDs.
func (s *SQLiteStore) RecordContainerOpen(id string, at time.Time) error {
	if !s.containerExists(id) {
		return store.ErrContainerNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Delete + re-insert (instead of upsert) so the row gets a fresh, strictly
	// increasing rowid. Ordering by rowid breaks ties between opens that land
	// in the same millisecond, keeping call order.
	count := 0
	err = tx.QueryRow(`SELECT open_count FROM container_recents WHERE container_id = ?`, id).Scan(&count)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM container_recents WHERE container_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO container_recents (container_id, last_opened_at, open_count)
		VALUES (?, ?, ?)`, id, at.UnixMilli(), count+1); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		DELETE FROM container_recents WHERE container_id NOT IN (
			SELECT container_id FROM container_recents
			ORDER BY last_opened_at DESC, rowid DESC
			LIMIT ?)`, store.MaxRecentContainers); err != nil {
		return err
	}
	return tx.Commit()
}

// RecentContainers returns up to limit recent entries, newest first.
func (s *SQLiteStore) RecentContainers(limit int) []store.RecentContainer {
	if limit <= 0 {
		return nil
	}
	rows, err := s.db.Query(`
		SELECT container_id, last_opened_at, open_count FROM container_recents
		ORDER BY last_opened_at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()
	var out []store.RecentContainer
	for rows.Next() {
		var r store.RecentContainer
		var ms int64
		if err := rows.Scan(&r.ContainerID, &ms, &r.OpenCount); err != nil {
			continue
		}
		r.LastOpenedAt = time.UnixMilli(ms)
		out = append(out, r)
	}
	return out
}

// ContainerItemCounts returns the number of items directly inside each of the
// given containers. Containers without items map to 0.
func (s *SQLiteStore) ContainerItemCounts(ids []string) map[string]int {
	counts := make(map[string]int, len(ids))
	if len(ids) == 0 {
		return counts
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
		counts[id] = 0
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	var rows *sql.Rows
	rows, err := s.db.Query(
		`SELECT container_id, COUNT(*) FROM items WHERE container_id IN (`+placeholders+`) GROUP BY container_id`, args...) //nolint:gosec // placeholders only
	if err != nil {
		return counts
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			continue
		}
		counts[id] = n
	}
	return counts
}
