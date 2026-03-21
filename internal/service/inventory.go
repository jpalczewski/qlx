package service

import (
	"fmt"

	"github.com/erxyi/qlx/internal/shared/palette"
	"github.com/erxyi/qlx/internal/shared/validate"
	"github.com/erxyi/qlx/internal/store"
)

// InventoryService handles container and item CRUD operations,
// calling Save() after each mutation.
type InventoryService struct {
	store interface {
		ContainerStore
		ItemStore
		Saveable
	}
}

// NewInventoryService creates a new InventoryService backed by the given store.
func NewInventoryService(s interface {
	ContainerStore
	ItemStore
	Saveable
}) *InventoryService {
	return &InventoryService{store: s}
}

// --- Container read methods (passthrough) ---

// GetContainer returns the container with the given ID, or nil.
func (s *InventoryService) GetContainer(id string) *store.Container {
	return s.store.GetContainer(id)
}

// ContainerChildren returns the direct child containers of parentID.
func (s *InventoryService) ContainerChildren(parentID string) []store.Container {
	return s.store.ContainerChildren(parentID)
}

// ContainerItems returns the items in the given container.
func (s *InventoryService) ContainerItems(containerID string) []store.Item {
	return s.store.ContainerItems(containerID)
}

// ContainerPath returns the path from root to the given container.
func (s *InventoryService) ContainerPath(id string) []store.Container {
	return s.store.ContainerPath(id)
}

// --- Container mutation methods ---

// CreateContainer creates a new container and persists the change.
func (s *InventoryService) CreateContainer(parentID, name, desc, color, icon string) (*store.Container, error) {
	if err := validate.Name(name, validate.MaxNameLength); err != nil {
		return nil, err
	}
	if err := validate.OptionalText(desc, validate.MaxDescriptionLength); err != nil {
		return nil, err
	}
	if color != "" && !palette.ValidColor(color) {
		return nil, fmt.Errorf("invalid color: %s", color)
	}
	if icon != "" && !palette.ValidIcon(icon) {
		return nil, fmt.Errorf("invalid icon: %s", icon)
	}
	c := s.store.CreateContainer(parentID, name, desc, color, icon)
	if err := s.store.Save(); err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateContainer updates a container's name and description, then persists.
func (s *InventoryService) UpdateContainer(id, name, desc, color, icon string) (*store.Container, error) {
	if err := validate.Name(name, validate.MaxNameLength); err != nil {
		return nil, err
	}
	if err := validate.OptionalText(desc, validate.MaxDescriptionLength); err != nil {
		return nil, err
	}
	if color != "" && !palette.ValidColor(color) {
		return nil, fmt.Errorf("invalid color: %s", color)
	}
	if icon != "" && !palette.ValidIcon(icon) {
		return nil, fmt.Errorf("invalid icon: %s", icon)
	}
	c, err := s.store.UpdateContainer(id, name, desc, color, icon)
	if err != nil {
		return nil, err
	}
	if err := s.store.Save(); err != nil {
		return nil, err
	}
	return c, nil
}

// DeleteContainer deletes a container and persists the change.
func (s *InventoryService) DeleteContainer(id string) error {
	if err := s.store.DeleteContainer(id); err != nil {
		return err
	}
	return s.store.Save()
}

// MoveContainer moves a container to a new parent and persists.
func (s *InventoryService) MoveContainer(id, newParentID string) error {
	if err := s.store.MoveContainer(id, newParentID); err != nil {
		return err
	}
	return s.store.Save()
}

// --- Item read methods (passthrough) ---

// GetItem returns the item with the given ID, or nil.
func (s *InventoryService) GetItem(id string) *store.Item {
	return s.store.GetItem(id)
}

// --- Item mutation methods ---

// CreateItem creates a new item and persists the change.
func (s *InventoryService) CreateItem(containerID, name, desc string, qty int, color, icon string) (*store.Item, error) {
	if err := validate.Name(name, validate.MaxNameLength); err != nil {
		return nil, err
	}
	if err := validate.OptionalText(desc, validate.MaxDescriptionLength); err != nil {
		return nil, err
	}
	if color != "" && !palette.ValidColor(color) {
		return nil, fmt.Errorf("invalid color: %s", color)
	}
	if icon != "" && !palette.ValidIcon(icon) {
		return nil, fmt.Errorf("invalid icon: %s", icon)
	}
	item := s.store.CreateItem(containerID, name, desc, qty, color, icon)
	if err := s.store.Save(); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateItem updates an item's name, description, and quantity, then persists.
// If qty is less than 1, the existing quantity is preserved.
func (s *InventoryService) UpdateItem(id, name, desc string, qty int, color, icon string) (*store.Item, error) {
	if err := validate.Name(name, validate.MaxNameLength); err != nil {
		return nil, err
	}
	if err := validate.OptionalText(desc, validate.MaxDescriptionLength); err != nil {
		return nil, err
	}
	if color != "" && !palette.ValidColor(color) {
		return nil, fmt.Errorf("invalid color: %s", color)
	}
	if icon != "" && !palette.ValidIcon(icon) {
		return nil, fmt.Errorf("invalid icon: %s", icon)
	}
	item, err := s.store.UpdateItem(id, name, desc, qty, color, icon)
	if err != nil {
		return nil, err
	}
	if err := s.store.Save(); err != nil {
		return nil, err
	}
	return item, nil
}

// DeleteItem deletes an item and persists the change.
func (s *InventoryService) DeleteItem(id string) error {
	if err := s.store.DeleteItem(id); err != nil {
		return err
	}
	return s.store.Save()
}

// MoveItem moves an item to a new container and persists.
func (s *InventoryService) MoveItem(id, containerID string) error {
	if err := s.store.MoveItem(id, containerID); err != nil {
		return err
	}
	return s.store.Save()
}
