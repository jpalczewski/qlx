package service

import "github.com/erxyi/qlx/internal/store"

// BulkService handles bulk move, delete, and tag operations,
// calling Save() after each mutation.
type BulkService struct {
	store interface {
		BulkStore
		Saveable
	}
}

// NewBulkService creates a new BulkService backed by the given store.
func NewBulkService(s interface {
	BulkStore
	Saveable
}) *BulkService {
	return &BulkService{store: s}
}

// Move moves items and containers to a target container and persists.
func (s *BulkService) Move(itemIDs, containerIDs []string, targetID string) ([]store.BulkError, error) {
	errs := s.store.BulkMove(itemIDs, containerIDs, targetID)
	if err := s.store.Save(); err != nil {
		return nil, err
	}
	return errs, nil
}

// Delete deletes items and containers, returning deleted IDs and per-entity errors, then persists.
func (s *BulkService) Delete(itemIDs, containerIDs []string) ([]string, []store.BulkError, error) {
	deleted, errs := s.store.BulkDelete(itemIDs, containerIDs)
	if err := s.store.Save(); err != nil {
		return nil, nil, err
	}
	return deleted, errs, nil
}

// AddTag adds a tag to multiple items and containers, then persists.
func (s *BulkService) AddTag(itemIDs, containerIDs []string, tagID string) error {
	if err := s.store.BulkAddTag(itemIDs, containerIDs, tagID); err != nil {
		return err
	}
	return s.store.Save()
}
