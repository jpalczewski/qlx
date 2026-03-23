package service

import (
	"fmt"

	"github.com/erxyi/qlx/internal/shared/palette"
	"github.com/erxyi/qlx/internal/shared/validate"
	"github.com/erxyi/qlx/internal/store"
)

// InventoryService handles container and item CRUD operations.
type InventoryService struct {
	store interface {
		store.ContainerStore
		store.ItemStore
	}
}

// NewInventoryService creates a new InventoryService backed by the given store.
func NewInventoryService(s interface {
	store.ContainerStore
	store.ItemStore
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
	return s.store.UpdateContainer(id, name, desc, color, icon)
}

// DeleteContainer deletes a container and returns the parent ID.
func (s *InventoryService) DeleteContainer(id string) (string, error) {
	return s.store.DeleteContainer(id)
}

// MoveContainer moves a container to a new parent.
func (s *InventoryService) MoveContainer(id, newParentID string) error {
	return s.store.MoveContainer(id, newParentID)
}

// AllContainers returns all containers without filtering.
func (s *InventoryService) AllContainers() []store.Container {
	return s.store.AllContainers()
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
	return s.store.UpdateItem(id, name, desc, qty, color, icon)
}

// DeleteItem deletes an item and returns the container ID.
func (s *InventoryService) DeleteItem(id string) (string, error) {
	return s.store.DeleteItem(id)
}

// MoveItem moves an item to a new container.
func (s *InventoryService) MoveItem(id, containerID string) error {
	return s.store.MoveItem(id, containerID)
}
