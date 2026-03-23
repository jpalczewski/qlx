package sqlite

import "github.com/erxyi/qlx/internal/store"

// BulkMove moves items and containers to a new target container/parent in a single transaction.
// It collects errors per ID rather than aborting on the first failure.
// For containers, cycle detection is performed before the move.
func (s *SQLiteStore) BulkMove(itemIDs, containerIDs []string, targetID string) []store.BulkError {
	var errs []store.BulkError

	// Build set of containers being moved for intra-batch checks.
	moveSet := make(map[string]bool, len(containerIDs))
	for _, id := range containerIDs {
		moveSet[id] = true
	}

	// Cycle detection for containers before starting the transaction.
	for _, id := range containerIDs {
		if targetID == id {
			errs = append(errs, store.BulkError{ID: id, Reason: store.ErrCycleDetected.Error()})
			continue
		}
		descendants := s.containerDescendants(id)
		for _, d := range descendants {
			if d == targetID {
				errs = append(errs, store.BulkError{ID: id, Reason: store.ErrCycleDetected.Error()})
				break
			}
			// Intra-batch ancestor conflict: if a descendant of this container
			// is also being moved, flag it as a conflict.
			if moveSet[d] {
				errs = append(errs, store.BulkError{ID: id, Reason: "intra-batch ancestor conflict"})
				break
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}

	tx, err := s.db.Begin()
	if err != nil {
		return []store.BulkError{{ID: "tx", Reason: err.Error()}}
	}
	defer func() { _ = tx.Rollback() }()

	for _, id := range itemIDs {
		_, err := tx.Exec(
			`UPDATE items SET container_id=?, updated_at=datetime('now') WHERE id=?`, targetID, id)
		if err != nil {
			errs = append(errs, store.BulkError{ID: id, Reason: err.Error()})
		}
	}
	for _, id := range containerIDs {
		_, err := tx.Exec(
			`UPDATE containers SET parent_id=?, updated_at=datetime('now') WHERE id=?`, targetID, id)
		if err != nil {
			errs = append(errs, store.BulkError{ID: id, Reason: err.Error()})
		}
	}

	if err := tx.Commit(); err != nil {
		return append(errs, store.BulkError{ID: "commit", Reason: err.Error()})
	}
	return errs
}

// BulkDelete deletes the specified items and containers in a single transaction.
// It returns the IDs of successfully deleted entities and any per-ID errors.
func (s *SQLiteStore) BulkDelete(itemIDs, containerIDs []string) ([]string, []store.BulkError) {
	var (
		deleted []string
		errs    []store.BulkError
	)

	tx, err := s.db.Begin()
	if err != nil {
		return nil, []store.BulkError{{ID: "tx", Reason: err.Error()}}
	}
	defer func() { _ = tx.Rollback() }()

	for _, id := range itemIDs {
		res, err := tx.Exec(`DELETE FROM items WHERE id=?`, id)
		if err != nil {
			errs = append(errs, store.BulkError{ID: id, Reason: err.Error()})
			continue
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			deleted = append(deleted, id)
		}
	}
	for _, id := range containerIDs {
		res, err := tx.Exec(`DELETE FROM containers WHERE id=?`, id)
		if err != nil {
			errs = append(errs, store.BulkError{ID: id, Reason: err.Error()})
			continue
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			deleted = append(deleted, id)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, append(errs, store.BulkError{ID: "commit", Reason: err.Error()})
	}
	return deleted, errs
}

// BulkAddTag associates the given tag with all specified items and containers.
// Uses INSERT OR IGNORE to silently skip already-existing associations.
// Returns store.ErrTagNotFound if the tag does not exist.
func (s *SQLiteStore) BulkAddTag(itemIDs, containerIDs []string, tagID string) error {
	if s.GetTag(tagID) == nil {
		return store.ErrTagNotFound
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, id := range itemIDs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO item_tags (item_id, tag_id) VALUES (?, ?)`, id, tagID); err != nil {
			return err
		}
	}
	for _, id := range containerIDs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO container_tags (container_id, tag_id) VALUES (?, ?)`, id, tagID); err != nil {
			return err
		}
	}

	return tx.Commit()
}
