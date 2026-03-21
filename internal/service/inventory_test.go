package service

import (
	"errors"
	"testing"

	"github.com/erxyi/qlx/internal/store"
)

// mockInventoryStore provides a configurable mock for InventoryService tests.
type mockInventoryStore struct {
	getContainer      func(id string) *store.Container
	createContainer   func(parentID, name, desc string) *store.Container
	updateContainer   func(id, name, desc string) (*store.Container, error)
	deleteContainer   func(id string) error
	moveContainer     func(id, newParentID string) error
	containerChildren func(parentID string) []store.Container
	containerItems    func(containerID string) []store.Item
	containerPath     func(id string) []store.Container
	getItem           func(id string) *store.Item
	createItem        func(containerID, name, desc string, qty int) *store.Item
	updateItem        func(id, name, desc string, qty int) (*store.Item, error)
	deleteItem        func(id string) error
	moveItem          func(id, containerID string) error
	save              func() error
}

func (m *mockInventoryStore) GetContainer(id string) *store.Container {
	if m.getContainer != nil {
		return m.getContainer(id)
	}
	return nil
}
func (m *mockInventoryStore) CreateContainer(parentID, name, desc string) *store.Container {
	if m.createContainer != nil {
		return m.createContainer(parentID, name, desc)
	}
	return &store.Container{ID: "c1", Name: name}
}
func (m *mockInventoryStore) UpdateContainer(id, name, desc string) (*store.Container, error) {
	if m.updateContainer != nil {
		return m.updateContainer(id, name, desc)
	}
	return &store.Container{ID: id, Name: name}, nil
}
func (m *mockInventoryStore) DeleteContainer(id string) error {
	if m.deleteContainer != nil {
		return m.deleteContainer(id)
	}
	return nil
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
func (m *mockInventoryStore) GetItem(id string) *store.Item {
	if m.getItem != nil {
		return m.getItem(id)
	}
	return nil
}
func (m *mockInventoryStore) CreateItem(containerID, name, desc string, qty int) *store.Item {
	if m.createItem != nil {
		return m.createItem(containerID, name, desc, qty)
	}
	return &store.Item{ID: "i1", Name: name}
}
func (m *mockInventoryStore) UpdateItem(id, name, desc string, qty int) (*store.Item, error) {
	if m.updateItem != nil {
		return m.updateItem(id, name, desc, qty)
	}
	return &store.Item{ID: id, Name: name, Quantity: qty}, nil
}
func (m *mockInventoryStore) DeleteItem(id string) error {
	if m.deleteItem != nil {
		return m.deleteItem(id)
	}
	return nil
}
func (m *mockInventoryStore) MoveItem(id, containerID string) error {
	if m.moveItem != nil {
		return m.moveItem(id, containerID)
	}
	return nil
}
func (m *mockInventoryStore) Save() error {
	if m.save != nil {
		return m.save()
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
				createContainer: func(_, name, _ string) *store.Container {
					return &store.Container{ID: "c1", Name: name}
				},
			},
			wantName: "Box A",
		},
		{
			name: "save error",
			mock: &mockInventoryStore{
				save: func() error { return errors.New("disk full") },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInventoryService(tt.mock)
			c, err := svc.CreateContainer("", "Box A", "desc")
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
				updateContainer: func(_, _, _ string) (*store.Container, error) {
					return nil, store.ErrContainerNotFound
				},
			},
			wantErr: true,
		},
		{
			name: "save error",
			mock: &mockInventoryStore{
				save: func() error { return errors.New("disk full") },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInventoryService(tt.mock)
			_, err := svc.UpdateContainer("c1", "New Name", "desc")
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
				deleteContainer: func(_ string) error { return store.ErrContainerHasChildren },
			},
			wantErr: true,
		},
		{
			name: "save error",
			mock: &mockInventoryStore{
				save: func() error { return errors.New("disk full") },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInventoryService(tt.mock)
			err := svc.DeleteContainer("c1")
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
		{
			name: "save error",
			mock: &mockInventoryStore{
				save: func() error { return errors.New("disk full") },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInventoryService(tt.mock)
			_, err := svc.CreateItem("c1", "Item", "desc", 1)
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
				deleteItem: func(_ string) error { return store.ErrItemNotFound },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInventoryService(tt.mock)
			err := svc.DeleteItem("i1")
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
		{
			name: "save error",
			mock: &mockInventoryStore{
				save: func() error { return errors.New("disk full") },
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
