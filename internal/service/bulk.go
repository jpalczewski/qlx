package service

import "github.com/erxyi/qlx/internal/store"

// BulkService handles bulk move, delete, and tag operations.
type BulkService struct {
	store store.BulkStore
}

// NewBulkService creates a new BulkService backed by the given store.
func NewBulkService(s store.BulkStore) *BulkService {
	return &BulkService{store: s}
}

// Move moves items and containers to a target container.
func (s *BulkService) Move(itemIDs, containerIDs []string, targetID string) ([]store.BulkError, error) {
	errs := s.store.BulkMove(itemIDs, containerIDs, targetID)
	return errs, nil
}

// Delete deletes items and containers, returning deleted IDs and per-entity errors.
func (s *BulkService) Delete(itemIDs, containerIDs []string) ([]string, []store.BulkError, error) {
	deleted, errs := s.store.BulkDelete(itemIDs, containerIDs)
	return deleted, errs, nil
}

// AddTag adds a tag to multiple items and containers.
func (s *BulkService) AddTag(itemIDs, containerIDs []string, tagID string) error {
	return s.store.BulkAddTag(itemIDs, containerIDs, tagID)
}
