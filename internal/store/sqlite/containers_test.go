package sqlite

import (
	"testing"

	"github.com/erxyi/qlx/internal/store"
)

func TestContainerStore_CRUD(t *testing.T) {
	db := testStore(t)

	// Create
	c := db.CreateContainer("", "Box A", "A storage box", "red", "box")
	if c == nil {
		t.Fatal("expected container, got nil")
	}
	if c.Name != "Box A" {
		t.Errorf("got name %q, want %q", c.Name, "Box A")
	}
	if c.ID == "" {
		t.Error("expected non-empty ID")
	}

	// Get
	got := db.GetContainer(c.ID)
	if got == nil {
		t.Fatal("expected container, got nil")
	}
	if got.Name != "Box A" {
		t.Errorf("got %q, want %q", got.Name, "Box A")
	}

	// Update
	updated, err := db.UpdateContainer(c.ID, "Box B", "Updated", "blue", "cube")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Box B" {
		t.Errorf("got %q, want %q", updated.Name, "Box B")
	}

	// Delete
	parentID, err := db.DeleteContainer(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if parentID != "" {
		t.Errorf("expected empty parentID, got %q", parentID)
	}
	if db.GetContainer(c.ID) != nil {
		t.Error("expected nil after delete")
	}
}

func TestContainerStore_Hierarchy(t *testing.T) {
	db := testStore(t)

	parent := db.CreateContainer("", "Parent", "", "", "")
	child1 := db.CreateContainer(parent.ID, "Child1", "", "", "")
	child2 := db.CreateContainer(parent.ID, "Child2", "", "", "")
	db.CreateContainer(child1.ID, "GrandChild", "", "", "")

	_ = child2 // used implicitly via children count

	children := db.ContainerChildren(parent.ID)
	if len(children) != 2 {
		t.Errorf("got %d children, want 2", len(children))
	}

	path := db.ContainerPath(child1.ID)
	// path should be [parent, child1] — from root to node
	if len(path) != 2 {
		t.Errorf("got path length %d, want 2", len(path))
	}
	if path[0].ID != parent.ID {
		t.Errorf("path[0] should be parent")
	}
}

func TestContainerStore_DeleteNotFound(t *testing.T) {
	db := testStore(t)
	_, err := db.DeleteContainer("nonexistent")
	if err == nil {
		t.Fatal("expected error deleting nonexistent container")
	}
}

func TestContainerStore_UpdateNotFound(t *testing.T) {
	db := testStore(t)
	_, err := db.UpdateContainer("nonexistent", "X", "", "", "")
	if err == nil {
		t.Fatal("expected error updating nonexistent container")
	}
}

func TestContainerStore_AllContainers(t *testing.T) {
	db := testStore(t)

	db.CreateContainer("", "A", "", "", "")
	db.CreateContainer("", "B", "", "", "")

	all := db.AllContainers()
	if len(all) != 2 {
		t.Errorf("got %d containers, want 2", len(all))
	}
}

func TestContainerStore_MoveContainer(t *testing.T) {
	db := testStore(t)

	src := db.CreateContainer("", "Source", "", "", "")
	dst := db.CreateContainer("", "Dest", "", "", "")
	child := db.CreateContainer(src.ID, "Child", "", "", "")

	err := db.MoveContainer(child.ID, dst.ID)
	if err != nil {
		t.Fatal(err)
	}

	children := db.ContainerChildren(dst.ID)
	if len(children) != 1 || children[0].ID != child.ID {
		t.Error("child not moved to dst")
	}
}

func TestContainerStore_ContainerItems(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	if c == nil {
		t.Fatal("expected container")
	}

	// No items yet
	items := db.ContainerItems(c.ID)
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestContainerStore_ErrorSentinels(t *testing.T) {
	db := testStore(t)

	_, err := db.DeleteContainer("no-such-id")
	if err != store.ErrContainerNotFound {
		t.Errorf("expected ErrContainerNotFound, got %v", err)
	}

	_, err = db.UpdateContainer("no-such-id", "X", "", "", "")
	if err != store.ErrContainerNotFound {
		t.Errorf("expected ErrContainerNotFound, got %v", err)
	}

	err = db.MoveContainer("no-such-id", "")
	if err != store.ErrContainerNotFound {
		t.Errorf("expected ErrContainerNotFound, got %v", err)
	}
}
