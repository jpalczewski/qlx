package service

import (
	"strings"
	"testing"

	"github.com/erxyi/qlx/internal/store"
)

// mockInventoryStore provides a configurable mock for InventoryService tests.
type mockInventoryStore struct {
	getContainer      func(id string) *store.Container
	createContainer   func(parentID, name, desc, color, icon string) *store.Container
	updateContainer   func(id, name, desc, color, icon string) (*store.Container, error)
	deleteContainer   func(id string) (string, error)
	moveContainer     func(id, newParentID string) error
	containerChildren func(parentID string) []store.Container
	containerItems    func(containerID string) []store.Item
	containerPath     func(id string) []store.Container
	getItem           func(id string) *store.Item
	createItem        func(containerID, name, desc string, qty int, color, icon string) *store.Item
	updateItem        func(id, name, desc string, qty int, color, icon string) (*store.Item, error)
	deleteItem        func(id string) (string, error)
	moveItem          func(id, containerID string) error
}

func (m *mockInventoryStore) GetContainer(id string) *store.Container {
	if m.getContainer != nil {
		return m.getContainer(id)
	}
	return nil
}
func (m *mockInventoryStore) CreateContainer(parentID, name, desc, color, icon string) *store.Container {
	if m.createContainer != nil {
		return m.createContainer(parentID, name, desc, color, icon)
	}
	return &store.Container{ID: "c1", Name: name}
}
func (m *mockInventoryStore) UpdateContainer(id, name, desc, color, icon string) (*store.Container, error) {
	if m.updateContainer != nil {
		return m.updateContainer(id, name, desc, color, icon)
	}
	return &store.Container{ID: id, Name: name}, nil
}
func (m *mockInventoryStore) DeleteContainer(id string) (string, error) {
	if m.deleteContainer != nil {
		return m.deleteContainer(id)
	}
	return "", nil
}
func (m *mockInventoryStore) MoveContainer(id, newParentID string) error {
	if m.moveContainer != nil {
		return m.moveContainer(id, newParentID)
	}
	return nil
}
func (m *mockInventoryStore) ContainerChildren(parentID string) []store.Container {
	if m.containerChildren != nil {
		return m.containerChildren(parentID)
	}
	return nil
}
func (m *mockInventoryStore) ContainerItems(containerID string) []store.Item {
	if m.containerItems != nil {
		return m.containerItems(containerID)
	}
	return nil
}
func (m *mockInventoryStore) ContainerPath(id string) []store.Container {
	if m.containerPath != nil {
		return m.containerPath(id)
	}
	return nil
}
func (m *mockInventoryStore) AllContainers() []store.Container {
	return nil
}
func (m *mockInventoryStore) GetItem(id string) *store.Item {
	if m.getItem != nil {
		return m.getItem(id)
	}
	return nil
}
func (m *mockInventoryStore) CreateItem(containerID, name, desc string, qty int, color, icon string) *store.Item {
	if m.createItem != nil {
		return m.createItem(containerID, name, desc, qty, color, icon)
	}
	return &store.Item{ID: "i1", Name: name}
}
func (m *mockInventoryStore) UpdateItem(id, name, desc string, qty int, color, icon string) (*store.Item, error) {
	if m.updateItem != nil {
		return m.updateItem(id, name, desc, qty, color, icon)
	}
	return &store.Item{ID: id, Name: name, Quantity: qty}, nil
}
func (m *mockInventoryStore) DeleteItem(id string) (string, error) {
	if m.deleteItem != nil {
		return m.deleteItem(id)
	}
	return "", nil
}
func (m *mockInventoryStore) MoveItem(id, containerID string) error {
	if m.moveItem != nil {
		return m.moveItem(id, containerID)
	}
	return nil
}

func TestInventoryService_CreateContainer(t *testing.T) {
	tests := []struct {
		name     string
		mock     *mockInventoryStore
		wantErr  bool
		wantName string
	}{
		{
			name: "success",
			mock: &mockInventoryStore{
				createContainer: func(_, name, _, _, _ string) *store.Container {
					return &store.Container{ID: "c1", Name: name}
				},
			},
			wantName: "Box A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInventoryService(tt.mock)
			c, err := svc.CreateContainer("", "Box A", "desc", "", "")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.Name != tt.wantName {
				t.Errorf("got name %q, want %q", c.Name, tt.wantName)
			}
		})
	}
}

func TestInventoryService_PaletteValidation(t *testing.T) {
	svc := NewInventoryService(&mockInventoryStore{
		createContainer: func(_, name, _, color, icon string) *store.Container {
			return &store.Container{ID: "c1", Name: name, Color: color, Icon: icon}
		},
	})

	t.Run("create container valid color and icon", func(t *testing.T) {
		c, err := svc.CreateContainer("", "Box", "", "red", "wrench")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Color != "red" || c.Icon != "wrench" {
			t.Errorf("got color=%q icon=%q, want color=%q icon=%q", c.Color, c.Icon, "red", "wrench")
		}
	})

	t.Run("create container invalid color", func(t *testing.T) {
		_, err := svc.CreateContainer("", "Box", "", "notacolor", "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid color") {
			t.Errorf("error %q does not contain %q", err.Error(), "invalid color")
		}
	})

	t.Run("create container invalid icon", func(t *testing.T) {
		_, err := svc.CreateContainer("", "Box", "", "", "notanicon")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid icon") {
			t.Errorf("error %q does not contain %q", err.Error(), "invalid icon")
		}
	})

	t.Run("create container empty color and icon", func(t *testing.T) {
		_, err := svc.CreateContainer("", "Box", "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("create item invalid icon", func(t *testing.T) {
		_, err := svc.CreateItem("c1", "Item", "", 1, "", "notanicon")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid icon") {
			t.Errorf("error %q does not contain %q", err.Error(), "invalid icon")
		}
	})

	t.Run("update container valid color and icon", func(t *testing.T) {
		_, err := svc.UpdateContainer("c1", "Box", "", "blue", "wrench")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("update container empty color and icon", func(t *testing.T) {
		_, err := svc.UpdateContainer("c1", "Box", "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("update item valid color and icon", func(t *testing.T) {
		_, err := svc.UpdateItem("i1", "Item", "", 1, "green", "wrench")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("update item empty color and icon", func(t *testing.T) {
		_, err := svc.UpdateItem("i1", "Item", "", 1, "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestInventoryService_UpdateContainer(t *testing.T) {
	tests := []struct {
		name    string
		mock    *mockInventoryStore
		wantErr bool
	}{
		{
			name: "success",
			mock: &mockInventoryStore{},
		},
		{
			name: "not found",
			mock: &mockInventoryStore{
				updateContainer: func(_, _, _, _, _ string) (*store.Container, error) {
					return nil, store.ErrContainerNotFound
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInventoryService(tt.mock)
			_, err := svc.UpdateContainer("c1", "New Name", "desc", "", "")
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestInventoryService_DeleteContainer(t *testing.T) {
	tests := []struct {
		name    string
		mock    *mockInventoryStore
		wantErr bool
	}{
		{
			name: "success",
			mock: &mockInventoryStore{},
		},
		{
			name: "has children",
			mock: &mockInventoryStore{
				deleteContainer: func(_ string) (string, error) { return "", store.ErrContainerHasChildren },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInventoryService(tt.mock)
			_, err := svc.DeleteContainer("c1")
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestInventoryService_CreateItem(t *testing.T) {
	tests := []struct {
		name    string
		mock    *mockInventoryStore
		wantErr bool
	}{
		{
			name: "success",
			mock: &mockInventoryStore{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInventoryService(tt.mock)
			_, err := svc.CreateItem("c1", "Item", "desc", 1, "", "")
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestInventoryService_DeleteItem(t *testing.T) {
	tests := []struct {
		name    string
		mock    *mockInventoryStore
		wantErr bool
	}{
		{
			name: "success",
			mock: &mockInventoryStore{},
		},
		{
			name: "not found",
			mock: &mockInventoryStore{
				deleteItem: func(_ string) (string, error) { return "", store.ErrItemNotFound },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInventoryService(tt.mock)
			_, err := svc.DeleteItem("i1")
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestInventoryService_MoveItem(t *testing.T) {
	tests := []struct {
		name    string
		mock    *mockInventoryStore
		wantErr bool
	}{
		{
			name: "success",
			mock: &mockInventoryStore{},
		},
		{
			name: "invalid container",
			mock: &mockInventoryStore{
				moveItem: func(_, _ string) error { return store.ErrInvalidContainer },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInventoryService(tt.mock)
			err := svc.MoveItem("i1", "c2")
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestInventoryService_MoveContainer(t *testing.T) {
	tests := []struct {
		name    string
		mock    *mockInventoryStore
		wantErr bool
	}{
		{
			name: "success",
			mock: &mockInventoryStore{},
		},
		{
			name: "cycle detected",
			mock: &mockInventoryStore{
				moveContainer: func(_, _ string) error { return store.ErrCycleDetected },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInventoryService(tt.mock)
			err := svc.MoveContainer("c1", "c2")
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestInventoryService_Passthrough(t *testing.T) {
	mock := &mockInventoryStore{
		getContainer: func(id string) *store.Container {
			return &store.Container{ID: id, Name: "Test"}
		},
		containerChildren: func(_ string) []store.Container {
			return []store.Container{{ID: "c2"}}
		},
		containerItems: func(_ string) []store.Item {
			return []store.Item{{ID: "i1"}}
		},
		containerPath: func(_ string) []store.Container {
			return []store.Container{{ID: "root"}, {ID: "c1"}}
		},
		getItem: func(id string) *store.Item {
			return &store.Item{ID: id, Name: "Widget"}
		},
	}

	svc := NewInventoryService(mock)

	if c := svc.GetContainer("c1"); c == nil || c.ID != "c1" {
		t.Error("GetContainer passthrough failed")
	}
	if children := svc.ContainerChildren("c1"); len(children) != 1 {
		t.Error("ContainerChildren passthrough failed")
	}
	if items := svc.ContainerItems("c1"); len(items) != 1 {
		t.Error("ContainerItems passthrough failed")
	}
	if path := svc.ContainerPath("c1"); len(path) != 2 {
		t.Error("ContainerPath passthrough failed")
	}
	if item := svc.GetItem("i1"); item == nil || item.ID != "i1" {
		t.Error("GetItem passthrough failed")
	}
}
