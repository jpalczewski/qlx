package service

import (
	"errors"
	"testing"

	"github.com/erxyi/qlx/internal/store"
)

type mockTagStore struct {
	getTag             func(id string) *store.Tag
	createTag          func(parentID, name string) *store.Tag
	updateTag          func(id, name string) (*store.Tag, error)
	deleteTag          func(id string) error
	moveTag            func(id, newParentID string) error
	allTags            func() []store.Tag
	tagChildren        func(parentID string) []store.Tag
	tagPath            func(id string) []store.Tag
	tagDescendants     func(id string) []string
	addItemTag         func(itemID, tagID string) error
	removeItemTag      func(itemID, tagID string) error
	addContainerTag    func(containerID, tagID string) error
	removeContainerTag func(containerID, tagID string) error

	// ItemStore + ContainerStore (needed by TagService's store interface)
	getItem           func(id string) *store.Item
	createItem        func(containerID, name, desc string, qty int) *store.Item
	updateItem        func(id, name, desc string) (*store.Item, error)
	deleteItem        func(id string) error
	moveItem          func(id, containerID string) error
	getContainer      func(id string) *store.Container
	createContainer   func(parentID, name, desc string) *store.Container
	updateContainer   func(id, name, desc string) (*store.Container, error)
	deleteContainer   func(id string) error
	moveContainer     func(id, newParentID string) error
	containerChildren func(parentID string) []store.Container
	containerItems    func(containerID string) []store.Item
	containerPath     func(id string) []store.Container

	save func() error
}

func (m *mockTagStore) GetTag(id string) *store.Tag {
	if m.getTag != nil {
		return m.getTag(id)
	}
	return nil
}
func (m *mockTagStore) CreateTag(parentID, name string) *store.Tag {
	if m.createTag != nil {
		return m.createTag(parentID, name)
	}
	return &store.Tag{ID: "t1", Name: name, ParentID: parentID}
}
func (m *mockTagStore) UpdateTag(id, name string) (*store.Tag, error) {
	if m.updateTag != nil {
		return m.updateTag(id, name)
	}
	return &store.Tag{ID: id, Name: name}, nil
}
func (m *mockTagStore) DeleteTag(id string) error {
	if m.deleteTag != nil {
		return m.deleteTag(id)
	}
	return nil
}
func (m *mockTagStore) MoveTag(id, newParentID string) error {
	if m.moveTag != nil {
		return m.moveTag(id, newParentID)
	}
	return nil
}
func (m *mockTagStore) AllTags() []store.Tag {
	if m.allTags != nil {
		return m.allTags()
	}
	return nil
}
func (m *mockTagStore) TagChildren(parentID string) []store.Tag {
	if m.tagChildren != nil {
		return m.tagChildren(parentID)
	}
	return nil
}
func (m *mockTagStore) TagPath(id string) []store.Tag {
	if m.tagPath != nil {
		return m.tagPath(id)
	}
	return nil
}
func (m *mockTagStore) TagDescendants(id string) []string {
	if m.tagDescendants != nil {
		return m.tagDescendants(id)
	}
	return nil
}
func (m *mockTagStore) AddItemTag(itemID, tagID string) error {
	if m.addItemTag != nil {
		return m.addItemTag(itemID, tagID)
	}
	return nil
}
func (m *mockTagStore) RemoveItemTag(itemID, tagID string) error {
	if m.removeItemTag != nil {
		return m.removeItemTag(itemID, tagID)
	}
	return nil
}
func (m *mockTagStore) AddContainerTag(containerID, tagID string) error {
	if m.addContainerTag != nil {
		return m.addContainerTag(containerID, tagID)
	}
	return nil
}
func (m *mockTagStore) RemoveContainerTag(containerID, tagID string) error {
	if m.removeContainerTag != nil {
		return m.removeContainerTag(containerID, tagID)
	}
	return nil
}
func (m *mockTagStore) GetItem(id string) *store.Item {
	if m.getItem != nil {
		return m.getItem(id)
	}
	return nil
}
func (m *mockTagStore) CreateItem(containerID, name, desc string, qty int) *store.Item {
	if m.createItem != nil {
		return m.createItem(containerID, name, desc, qty)
	}
	return &store.Item{ID: "i1", Name: name}
}
func (m *mockTagStore) UpdateItem(id, name, desc string) (*store.Item, error) {
	if m.updateItem != nil {
		return m.updateItem(id, name, desc)
	}
	return &store.Item{ID: id, Name: name}, nil
}
func (m *mockTagStore) DeleteItem(id string) error {
	if m.deleteItem != nil {
		return m.deleteItem(id)
	}
	return nil
}
func (m *mockTagStore) MoveItem(id, containerID string) error {
	if m.moveItem != nil {
		return m.moveItem(id, containerID)
	}
	return nil
}
func (m *mockTagStore) GetContainer(id string) *store.Container {
	if m.getContainer != nil {
		return m.getContainer(id)
	}
	return nil
}
func (m *mockTagStore) CreateContainer(parentID, name, desc string) *store.Container {
	if m.createContainer != nil {
		return m.createContainer(parentID, name, desc)
	}
	return &store.Container{ID: "c1", Name: name}
}
func (m *mockTagStore) UpdateContainer(id, name, desc string) (*store.Container, error) {
	if m.updateContainer != nil {
		return m.updateContainer(id, name, desc)
	}
	return &store.Container{ID: id, Name: name}, nil
}
func (m *mockTagStore) DeleteContainer(id string) error {
	if m.deleteContainer != nil {
		return m.deleteContainer(id)
	}
	return nil
}
func (m *mockTagStore) MoveContainer(id, newParentID string) error {
	if m.moveContainer != nil {
		return m.moveContainer(id, newParentID)
	}
	return nil
}
func (m *mockTagStore) ContainerChildren(parentID string) []store.Container {
	if m.containerChildren != nil {
		return m.containerChildren(parentID)
	}
	return nil
}
func (m *mockTagStore) ContainerItems(containerID string) []store.Item {
	if m.containerItems != nil {
		return m.containerItems(containerID)
	}
	return nil
}
func (m *mockTagStore) ContainerPath(id string) []store.Container {
	if m.containerPath != nil {
		return m.containerPath(id)
	}
	return nil
}
func (m *mockTagStore) Save() error {
	if m.save != nil {
		return m.save()
	}
	return nil
}

func TestTagService_CreateTag(t *testing.T) {
	tests := []struct {
		name    string
		mock    *mockTagStore
		wantErr bool
	}{
		{
			name: "success",
			mock: &mockTagStore{},
		},
		{
			name: "save error",
			mock: &mockTagStore{
				save: func() error { return errors.New("disk full") },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewTagService(tt.mock)
			tag, err := svc.CreateTag("", "Electronics")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tag.Name != "Electronics" {
				t.Errorf("got name %q, want %q", tag.Name, "Electronics")
			}
		})
	}
}

func TestTagService_UpdateTag(t *testing.T) {
	tests := []struct {
		name    string
		mock    *mockTagStore
		wantErr bool
	}{
		{
			name: "success",
			mock: &mockTagStore{},
		},
		{
			name: "not found",
			mock: &mockTagStore{
				updateTag: func(_, _ string) (*store.Tag, error) {
					return nil, store.ErrTagNotFound
				},
			},
			wantErr: true,
		},
		{
			name: "save error",
			mock: &mockTagStore{
				save: func() error { return errors.New("disk full") },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewTagService(tt.mock)
			_, err := svc.UpdateTag("t1", "Updated")
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTagService_DeleteTag(t *testing.T) {
	tests := []struct {
		name    string
		mock    *mockTagStore
		wantErr bool
	}{
		{
			name: "success",
			mock: &mockTagStore{},
		},
		{
			name: "has children",
			mock: &mockTagStore{
				deleteTag: func(_ string) error { return store.ErrTagHasChildren },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewTagService(tt.mock)
			err := svc.DeleteTag("t1")
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTagService_MoveTag(t *testing.T) {
	tests := []struct {
		name    string
		mock    *mockTagStore
		wantErr bool
	}{
		{
			name: "success",
			mock: &mockTagStore{},
		},
		{
			name: "cycle detected",
			mock: &mockTagStore{
				moveTag: func(_, _ string) error { return store.ErrCycleDetected },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewTagService(tt.mock)
			err := svc.MoveTag("t1", "t2")
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTagService_AddItemTag(t *testing.T) {
	tests := []struct {
		name    string
		mock    *mockTagStore
		wantErr bool
	}{
		{
			name: "success",
			mock: &mockTagStore{},
		},
		{
			name: "item not found",
			mock: &mockTagStore{
				addItemTag: func(_, _ string) error { return store.ErrItemNotFound },
			},
			wantErr: true,
		},
		{
			name: "save error",
			mock: &mockTagStore{
				save: func() error { return errors.New("disk full") },
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewTagService(tt.mock)
			err := svc.AddItemTag("i1", "t1")
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTagService_AddContainerTag(t *testing.T) {
	svc := NewTagService(&mockTagStore{})
	if err := svc.AddContainerTag("c1", "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTagService_RemoveItemTag(t *testing.T) {
	svc := NewTagService(&mockTagStore{})
	if err := svc.RemoveItemTag("i1", "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTagService_RemoveContainerTag(t *testing.T) {
	svc := NewTagService(&mockTagStore{})
	if err := svc.RemoveContainerTag("c1", "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTagService_Passthrough(t *testing.T) {
	mock := &mockTagStore{
		getTag: func(id string) *store.Tag {
			return &store.Tag{ID: id, Name: "Test"}
		},
		allTags: func() []store.Tag {
			return []store.Tag{{ID: "t1"}, {ID: "t2"}}
		},
		tagChildren: func(_ string) []store.Tag {
			return []store.Tag{{ID: "t3"}}
		},
		tagPath: func(_ string) []store.Tag {
			return []store.Tag{{ID: "t1"}, {ID: "t2"}}
		},
		tagDescendants: func(_ string) []string {
			return []string{"t2", "t3"}
		},
	}

	svc := NewTagService(mock)

	if tag := svc.GetTag("t1"); tag == nil || tag.ID != "t1" {
		t.Error("GetTag passthrough failed")
	}
	if tags := svc.AllTags(); len(tags) != 2 {
		t.Error("AllTags passthrough failed")
	}
	if children := svc.TagChildren("t1"); len(children) != 1 {
		t.Error("TagChildren passthrough failed")
	}
	if path := svc.TagPath("t1"); len(path) != 2 {
		t.Error("TagPath passthrough failed")
	}
	if desc := svc.TagDescendants("t1"); len(desc) != 2 {
		t.Error("TagDescendants passthrough failed")
	}
}
