package service

import (
	"fmt"

	"github.com/erxyi/qlx/internal/shared/palette"
	"github.com/erxyi/qlx/internal/shared/validate"
	"github.com/erxyi/qlx/internal/store"
)

// TagService handles tag CRUD and tag assignment operations.
type TagService struct {
	store interface {
		store.TagStore
		store.ItemStore
		store.ContainerStore
	}
}

// NewTagService creates a new TagService backed by the given store.
func NewTagService(s interface {
	store.TagStore
	store.ItemStore
	store.ContainerStore
}) *TagService {
	return &TagService{store: s}
}

// --- Read methods (passthrough) ---

// GetTag returns the tag with the given ID, or nil.
func (s *TagService) GetTag(id string) *store.Tag {
	return s.store.GetTag(id)
}

// AllTags returns all tags.
func (s *TagService) AllTags() []store.Tag {
	return s.store.AllTags()
}

// TagChildren returns the direct children of the given tag.
func (s *TagService) TagChildren(parentID string) []store.Tag {
	return s.store.TagChildren(parentID)
}

// TagPath returns the path from root to the given tag.
func (s *TagService) TagPath(id string) []store.Tag {
	return s.store.TagPath(id)
}

// TagDescendants returns all descendant tag IDs.
func (s *TagService) TagDescendants(id string) []string {
	return s.store.TagDescendants(id)
}

// ItemsByTag returns all items tagged with the given tag ID.
func (s *TagService) ItemsByTag(tagID string) []store.Item {
	return s.store.ItemsByTag(tagID)
}

// ContainersByTag returns all containers tagged with the given tag ID.
func (s *TagService) ContainersByTag(tagID string) []store.Container {
	return s.store.ContainersByTag(tagID)
}

// ResolveTagIDs returns tag objects for the given IDs.
func (s *TagService) ResolveTagIDs(ids []string) []store.Tag {
	return s.store.ResolveTagIDs(ids)
}

// TagItemStats returns the number of items and containers tagged with the given tag.
func (s *TagService) TagItemStats(id string) (int, int, error) {
	return s.store.TagItemStats(id)
}

// --- Mutation methods ---

// CreateTag creates a new tag and persists.
func (s *TagService) CreateTag(parentID, name, color, icon string) (*store.Tag, error) {
	if err := validate.Name(name, validate.MaxTagNameLength); err != nil {
		return nil, err
	}
	if color != "" && !palette.ValidColor(color) {
		return nil, fmt.Errorf("invalid color: %s", color)
	}
	if icon != "" && !palette.ValidIcon(icon) {
		return nil, fmt.Errorf("invalid icon: %s", icon)
	}
	t := s.store.CreateTag(parentID, name, color, icon)
	return t, nil
}

// UpdateTag updates a tag's name and persists.
func (s *TagService) UpdateTag(id, name, color, icon string) (*store.Tag, error) {
	if err := validate.Name(name, validate.MaxTagNameLength); err != nil {
		return nil, err
	}
	if color != "" && !palette.ValidColor(color) {
		return nil, fmt.Errorf("invalid color: %s", color)
	}
	if icon != "" && !palette.ValidIcon(icon) {
		return nil, fmt.Errorf("invalid icon: %s", icon)
	}
	return s.store.UpdateTag(id, name, color, icon)
}

// DeleteTag deletes a tag and returns the parent ID.
func (s *TagService) DeleteTag(id string) (string, error) {
	return s.store.DeleteTag(id)
}

// MoveTag moves a tag to a new parent.
func (s *TagService) MoveTag(id, newParentID string) error {
	return s.store.MoveTag(id, newParentID)
}

// AddItemTag adds a tag to an item.
func (s *TagService) AddItemTag(itemID, tagID string) error {
	return s.store.AddItemTag(itemID, tagID)
}

// RemoveItemTag removes a tag from an item.
func (s *TagService) RemoveItemTag(itemID, tagID string) error {
	return s.store.RemoveItemTag(itemID, tagID)
}

// AddContainerTag adds a tag to a container.
func (s *TagService) AddContainerTag(containerID, tagID string) error {
	return s.store.AddContainerTag(containerID, tagID)
}

// RemoveContainerTag removes a tag from a container.
func (s *TagService) RemoveContainerTag(containerID, tagID string) error {
	return s.store.RemoveContainerTag(containerID, tagID)
}
