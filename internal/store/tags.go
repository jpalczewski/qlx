package store

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTagNotFound    = errors.New("tag not found")
	ErrTagHasChildren = errors.New("tag has children")
)

// CreateTag creates a new tag with the given parent and name. parentID may be empty for a root tag.
func (s *Store) CreateTag(parentID, name string) *Tag {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := &Tag{
		ID:        uuid.New().String(),
		ParentID:  parentID,
		Name:      name,
		CreatedAt: time.Now(),
	}
	s.tags[t.ID] = t
	return t
}

// GetTag returns a copy of the tag with the given id, or nil if not found.
func (s *Store) GetTag(id string) *Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tags[id]
	if !ok {
		return nil
	}
	copy := *t
	return &copy
}

// UpdateTag updates the name of the tag with the given id.
func (s *Store) UpdateTag(id, name string) (*Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tags[id]
	if !ok {
		return nil, ErrTagNotFound
	}

	t.Name = name
	copy := *t
	return &copy, nil
}

// DeleteTag removes the tag and cascades: removes the tag ID from all items and containers.
// Returns ErrTagHasChildren if any tags have this tag as their parent.
func (s *Store) DeleteTag(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tags[id]; !ok {
		return ErrTagNotFound
	}

	for _, t := range s.tags {
		if t.ParentID == id {
			return ErrTagHasChildren
		}
	}

	delete(s.tags, id)

	for _, item := range s.items {
		item.TagIDs = removeFromSlice(item.TagIDs, id)
	}

	for _, c := range s.containers {
		c.TagIDs = removeFromSlice(c.TagIDs, id)
	}

	return nil
}

// AllTags returns all tags in the store.
func (s *Store) AllTags() []Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags := make([]Tag, 0, len(s.tags))
	for _, t := range s.tags {
		tags = append(tags, *t)
	}
	return tags
}

// TagChildren returns the direct children of the tag with the given id.
func (s *Store) TagChildren(id string) []Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var children []Tag
	for _, t := range s.tags {
		if t.ParentID == id {
			children = append(children, *t)
		}
	}
	return children
}

// TagPath returns the path from the root to the tag with the given id.
func (s *Store) TagPath(id string) []Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var path []Tag
	current := s.tags[id]
	for current != nil {
		path = append([]Tag{*current}, path...)
		if current.ParentID == "" {
			break
		}
		current = s.tags[current.ParentID]
	}
	return path
}

// TagDescendants returns all descendant tag IDs of the tag with the given id using O(N) BFS.
func (s *Store) TagDescendants(id string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.tagDescendantsLocked(id)
}

// tagDescendantsLocked returns all descendant tag IDs using O(N) BFS. Must be called with at least a read lock.
func (s *Store) tagDescendantsLocked(id string) []string {
	// Build parent→children map in one pass.
	children := make(map[string][]string, len(s.tags))
	for _, t := range s.tags {
		children[t.ParentID] = append(children[t.ParentID], t.ID)
	}

	// BFS from id.
	var result []string
	queue := children[id]
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		result = append(result, cur)
		queue = append(queue, children[cur]...)
	}
	return result
}

// MoveTag moves tagID to a new parent. Returns ErrCycleDetected if the move would create a cycle.
func (s *Store) MoveTag(tagID, newParentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tag, ok := s.tags[tagID]
	if !ok {
		return ErrTagNotFound
	}

	if newParentID != "" {
		if _, ok := s.tags[newParentID]; !ok {
			return ErrTagNotFound
		}
	}

	if newParentID == tagID {
		return ErrCycleDetected
	}

	// Walk up from newParentID to check for tagID in ancestors.
	ancestor := s.tags[newParentID]
	for ancestor != nil {
		if ancestor.ID == tagID {
			return ErrCycleDetected
		}
		ancestor = s.tags[ancestor.ParentID]
	}

	tag.ParentID = newParentID
	return nil
}

// AddItemTag adds tagID to item's TagIDs. No-op if already present.
func (s *Store) AddItemTag(itemID, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[itemID]
	if !ok {
		return ErrItemNotFound
	}

	if _, ok := s.tags[tagID]; !ok {
		return ErrTagNotFound
	}

	if containsString(item.TagIDs, tagID) {
		return nil
	}

	item.TagIDs = append(item.TagIDs, tagID)
	return nil
}

// RemoveItemTag removes tagID from item's TagIDs.
func (s *Store) RemoveItemTag(itemID, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[itemID]
	if !ok {
		return ErrItemNotFound
	}

	item.TagIDs = removeFromSlice(item.TagIDs, tagID)
	return nil
}

// AddContainerTag adds tagID to container's TagIDs. No-op if already present.
func (s *Store) AddContainerTag(containerID, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.containers[containerID]
	if !ok {
		return ErrContainerNotFound
	}

	if _, ok := s.tags[tagID]; !ok {
		return ErrTagNotFound
	}

	if containsString(c.TagIDs, tagID) {
		return nil
	}

	c.TagIDs = append(c.TagIDs, tagID)
	return nil
}

// RemoveContainerTag removes tagID from container's TagIDs.
func (s *Store) RemoveContainerTag(containerID, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.containers[containerID]
	if !ok {
		return ErrContainerNotFound
	}

	c.TagIDs = removeFromSlice(c.TagIDs, tagID)
	return nil
}

// ItemsByTag returns all items whose TagIDs include tagID or any of its descendants.
func (s *Store) ItemsByTag(tagID string) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build the set of relevant tag IDs: {tagID} ∪ descendants.
	relevant := make(map[string]struct{})
	relevant[tagID] = struct{}{}
	for _, id := range s.tagDescendantsLocked(tagID) {
		relevant[id] = struct{}{}
	}

	var result []Item
	for _, item := range s.items {
		for _, tid := range item.TagIDs {
			if _, ok := relevant[tid]; ok {
				result = append(result, *item)
				break
			}
		}
	}
	return result
}

// removeFromSlice returns a new slice with val removed (first occurrence only).
func removeFromSlice(slice []string, val string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != val {
			result = append(result, s)
		}
	}
	return result
}

// containsString reports whether slice contains val.
func containsString(sl []string, val string) bool {
	return slices.Contains(sl, val)
}
