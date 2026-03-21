package service

import (
	"fmt"

	"github.com/erxyi/qlx/internal/shared/palette"
	"github.com/erxyi/qlx/internal/shared/validate"
	"github.com/erxyi/qlx/internal/store"
)

// TagService handles tag CRUD and tag assignment operations,
// calling Save() after each mutation.
type TagService struct {
	store interface {
		TagStore
		ItemStore
		ContainerStore
		Saveable
	}
}

// NewTagService creates a new TagService backed by the given store.
func NewTagService(s interface {
	TagStore
	ItemStore
	ContainerStore
	Saveable
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
	if err := s.store.Save(); err != nil {
		return nil, err
	}
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
	t, err := s.store.UpdateTag(id, name, color, icon)
	if err != nil {
		return nil, err
	}
	if err := s.store.Save(); err != nil {
		return nil, err
	}
	return t, nil
}

// DeleteTag deletes a tag and persists.
func (s *TagService) DeleteTag(id string) error {
	if err := s.store.DeleteTag(id); err != nil {
		return err
	}
	return s.store.Save()
}

// MoveTag moves a tag to a new parent and persists.
func (s *TagService) MoveTag(id, newParentID string) error {
	if err := s.store.MoveTag(id, newParentID); err != nil {
		return err
	}
	return s.store.Save()
}

// AddItemTag adds a tag to an item and persists.
func (s *TagService) AddItemTag(itemID, tagID string) error {
	if err := s.store.AddItemTag(itemID, tagID); err != nil {
		return err
	}
	return s.store.Save()
}

// RemoveItemTag removes a tag from an item and persists.
func (s *TagService) RemoveItemTag(itemID, tagID string) error {
	if err := s.store.RemoveItemTag(itemID, tagID); err != nil {
		return err
	}
	return s.store.Save()
}

// AddContainerTag adds a tag to a container and persists.
func (s *TagService) AddContainerTag(containerID, tagID string) error {
	if err := s.store.AddContainerTag(containerID, tagID); err != nil {
		return err
	}
	return s.store.Save()
}

// RemoveContainerTag removes a tag from a container and persists.
func (s *TagService) RemoveContainerTag(containerID, tagID string) error {
	if err := s.store.RemoveContainerTag(containerID, tagID); err != nil {
		return err
	}
	return s.store.Save()
}
