package store

import "slices"

// BulkError records a failure for a single ID within a bulk operation.
type BulkError struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// MoveItems moves each item in ids to targetContainerID, collecting per-item errors.
func (s *Store) MoveItems(ids []string, targetContainerID string) []BulkError {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.moveItemsLocked(ids, targetContainerID)
}

// moveItemsLocked is the lock-free implementation of MoveItems.
// Must be called with s.mu held for writing.
func (s *Store) moveItemsLocked(ids []string, targetContainerID string) []BulkError {
	if targetContainerID != "" {
		if _, ok := s.containers[targetContainerID]; !ok {
			errs := make([]BulkError, 0, len(ids))
			for _, id := range ids {
				errs = append(errs, BulkError{ID: id, Reason: ErrInvalidContainer.Error()})
			}
			return errs
		}
	}

	var errs []BulkError
	for _, id := range ids {
		item, ok := s.items[id]
		if !ok {
			errs = append(errs, BulkError{ID: id, Reason: ErrItemNotFound.Error()})
			continue
		}
		item.ContainerID = targetContainerID
	}
	s.dirty |= dirtyItems
	return errs
}

// MoveContainers moves each container in ids to targetParentID atomically.
// All containers are validated before any move is committed; if any validation
// fails the entire batch is rejected.
func (s *Store) MoveContainers(ids []string, targetParentID string) []BulkError {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.moveContainersLocked(ids, targetParentID)
}

// moveContainersLocked is the lock-free implementation of MoveContainers.
// Must be called with s.mu held for writing.
func (s *Store) moveContainersLocked(ids []string, targetParentID string) []BulkError {
	// Validate target exists (empty string = root, always valid).
	if targetParentID != "" {
		if _, ok := s.containers[targetParentID]; !ok {
			errs := make([]BulkError, 0, len(ids))
			for _, id := range ids {
				errs = append(errs, BulkError{ID: id, Reason: ErrInvalidParent.Error()})
			}
			return errs
		}
	}

	// Build a set of IDs being moved for quick lookup.
	movingSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		movingSet[id] = struct{}{}
	}

	// First pass: per-container cycle check.
	var errs []BulkError
	for _, id := range ids {
		if reason := s.validateContainerMove(id, targetParentID); reason != "" {
			errs = append(errs, BulkError{ID: id, Reason: reason})
		}
	}

	// Second pass: intra-batch ancestry check.
	// Build a set of already-failed IDs to skip.
	failed := make(map[string]struct{}, len(errs))
	for _, e := range errs {
		failed[e.ID] = struct{}{}
	}
	for _, id := range ids {
		if _, skip := failed[id]; skip {
			continue
		}
		if reason := s.validateIntraBatchAncestry(id, movingSet); reason != "" {
			errs = append(errs, BulkError{ID: id, Reason: reason})
		}
	}

	// If any validation failed, abort the entire batch.
	if len(errs) > 0 {
		return errs
	}

	// Commit all moves.
	for _, id := range ids {
		s.containers[id].ParentID = targetParentID
	}
	s.dirty |= dirtyContainers
	return nil
}

// validateContainerMove checks a single container for existence, self-move, and ancestor cycles.
// Returns a non-empty reason string if validation fails.
func (s *Store) validateContainerMove(id, targetParentID string) string {
	if _, ok := s.containers[id]; !ok {
		return ErrContainerNotFound.Error()
	}
	if id == targetParentID {
		return ErrCycleDetected.Error()
	}
	ancestor := s.containers[targetParentID]
	for ancestor != nil {
		if ancestor.ID == id {
			return ErrCycleDetected.Error()
		}
		ancestor = s.containers[ancestor.ParentID]
	}
	return ""
}

// validateIntraBatchAncestry checks whether any ancestor of id is also in movingSet.
// Returns a non-empty reason string if validation fails.
func (s *Store) validateIntraBatchAncestry(id string, movingSet map[string]struct{}) string {
	c, ok := s.containers[id]
	if !ok {
		return ""
	}
	ancestor := s.containers[c.ParentID]
	for ancestor != nil {
		if _, inBatch := movingSet[ancestor.ID]; inBatch {
			return "ancestor is also being moved"
		}
		ancestor = s.containers[ancestor.ParentID]
	}
	return ""
}

// DeleteItems deletes all items whose IDs are in ids. Returns the IDs that were
// successfully deleted and a slice of per-item errors for those that were not found.
func (s *Store) DeleteItems(ids []string) ([]string, []BulkError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteItemsLocked(ids)
}

// deleteItemsLocked is the lock-free implementation of DeleteItems.
// Must be called with s.mu held for writing.
func (s *Store) deleteItemsLocked(ids []string) ([]string, []BulkError) {
	var deleted []string
	var errs []BulkError
	for _, id := range ids {
		if _, ok := s.items[id]; !ok {
			errs = append(errs, BulkError{ID: id, Reason: ErrItemNotFound.Error()})
			continue
		}
		delete(s.items, id)
		deleted = append(deleted, id)
	}
	if len(deleted) > 0 {
		s.dirty |= dirtyItems
	}
	return deleted, errs
}

// DeleteContainers deletes all containers whose IDs are in ids. Only empty
// containers (no child containers, no items) are deleted. Returns successfully
// deleted IDs and per-container errors for those that could not be deleted.
func (s *Store) DeleteContainers(ids []string) ([]string, []BulkError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteContainersLocked(ids)
}

// deleteContainersLocked is the lock-free implementation of DeleteContainers.
// Must be called with s.mu held for writing.
func (s *Store) deleteContainersLocked(ids []string) ([]string, []BulkError) {
	var deleted []string
	var errs []BulkError

	for _, id := range ids {
		if _, ok := s.containers[id]; !ok {
			errs = append(errs, BulkError{ID: id, Reason: ErrContainerNotFound.Error()})
			continue
		}

		// Check for child containers.
		hasChildren := false
		for _, c := range s.containers {
			if c.ParentID == id {
				hasChildren = true
				break
			}
		}
		if hasChildren {
			errs = append(errs, BulkError{ID: id, Reason: ErrContainerHasChildren.Error()})
			continue
		}

		// Check for items in the container.
		hasItems := false
		for _, item := range s.items {
			if item.ContainerID == id {
				hasItems = true
				break
			}
		}
		if hasItems {
			errs = append(errs, BulkError{ID: id, Reason: ErrContainerHasItems.Error()})
			continue
		}

		delete(s.containers, id)
		deleted = append(deleted, id)
	}
	if len(deleted) > 0 {
		s.dirty |= dirtyContainers
	}
	return deleted, errs
}

// BulkMove moves itemIDs and containerIDs to targetID in a single atomic lock acquisition.
func (s *Store) BulkMove(itemIDs, containerIDs []string, targetID string) []BulkError {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []BulkError
	if len(containerIDs) > 0 {
		errs = append(errs, s.moveContainersLocked(containerIDs, targetID)...)
	}
	if len(itemIDs) > 0 {
		errs = append(errs, s.moveItemsLocked(itemIDs, targetID)...)
	}
	return errs
}

// BulkDelete deletes itemIDs and containerIDs in a single atomic lock acquisition.
// Returns all successfully deleted IDs (items first, then containers) and all failures.
func (s *Store) BulkDelete(itemIDs, containerIDs []string) ([]string, []BulkError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var allDeleted []string
	var allErrs []BulkError

	if len(itemIDs) > 0 {
		deleted, errs := s.deleteItemsLocked(itemIDs)
		allDeleted = append(allDeleted, deleted...)
		allErrs = append(allErrs, errs...)
	}
	if len(containerIDs) > 0 {
		deleted, errs := s.deleteContainersLocked(containerIDs)
		allDeleted = append(allDeleted, deleted...)
		allErrs = append(allErrs, errs...)
	}
	return allDeleted, allErrs
}

// BulkAddTag adds tagID to all specified items and containers. Missing entities
// are skipped silently. Duplicate tag assignments are ignored.
// Returns ErrTagNotFound if the tag does not exist.
func (s *Store) BulkAddTag(itemIDs, containerIDs []string, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tags[tagID]; !ok {
		return ErrTagNotFound
	}

	for _, id := range itemIDs {
		item, ok := s.items[id]
		if !ok {
			continue
		}
		if !slices.Contains(item.TagIDs, tagID) {
			item.TagIDs = append(item.TagIDs, tagID)
			s.dirty |= dirtyItems
		}
	}

	for _, id := range containerIDs {
		c, ok := s.containers[id]
		if !ok {
			continue
		}
		if !slices.Contains(c.TagIDs, tagID) {
			c.TagIDs = append(c.TagIDs, tagID)
			s.dirty |= dirtyContainers
		}
	}

	return nil
}
