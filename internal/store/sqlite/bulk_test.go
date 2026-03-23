package sqlite

import (
	"testing"
)

func TestBulkStore_Move(t *testing.T) {
	db := testStore(t)

	src := db.CreateContainer("", "Src", "", "", "")
	dst := db.CreateContainer("", "Dst", "", "", "")
	item := db.CreateItem(src.ID, "Widget", "", 1, "", "")

	errs := db.BulkMove([]string{item.ID}, nil, dst.ID)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	moved := db.GetItem(item.ID)
	if moved == nil {
		t.Fatal("item not found after move")
	}
	if moved.ContainerID != dst.ID {
		t.Errorf("got container %q, want %q", moved.ContainerID, dst.ID)
	}
}

func TestBulkStore_MoveContainer(t *testing.T) {
	db := testStore(t)

	parent := db.CreateContainer("", "Parent", "", "", "")
	child := db.CreateContainer("", "Child", "", "", "")
	newParent := db.CreateContainer("", "NewParent", "", "", "")

	// Child is currently a root container; move it under newParent.
	errs := db.BulkMove(nil, []string{child.ID}, newParent.ID)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	_ = parent // avoid unused warning

	moved := db.GetContainer(child.ID)
	if moved == nil {
		t.Fatal("container not found after move")
	}
	if moved.ParentID != newParent.ID {
		t.Errorf("got parent %q, want %q", moved.ParentID, newParent.ID)
	}
}

func TestBulkStore_Delete(t *testing.T) {
	db := testStore(t)

	c1 := db.CreateContainer("", "A", "", "", "")
	c2 := db.CreateContainer("", "B", "", "", "")
	item := db.CreateItem(c1.ID, "Widget", "", 1, "", "")

	deleted, errs := db.BulkDelete([]string{item.ID}, []string{c2.ID})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(deleted) != 2 {
		t.Errorf("got %d deleted IDs, want 2", len(deleted))
	}

	if db.GetItem(item.ID) != nil {
		t.Error("item still exists after bulk delete")
	}
	if db.GetContainer(c2.ID) != nil {
		t.Error("container still exists after bulk delete")
	}
}

func TestBulkStore_AddTag(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	item := db.CreateItem(c.ID, "Widget", "", 1, "", "")
	tag := db.CreateTag("", "Tag", "", "")

	if err := db.BulkAddTag([]string{item.ID}, []string{c.ID}, tag.ID); err != nil {
		t.Fatal(err)
	}

	items := db.ItemsByTag(tag.ID)
	if len(items) != 1 {
		t.Errorf("got %d items with tag, want 1", len(items))
	}
	containers := db.ContainersByTag(tag.ID)
	if len(containers) != 1 {
		t.Errorf("got %d containers with tag, want 1", len(containers))
	}
}

func TestBulkStore_AddTag_Idempotent(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	item := db.CreateItem(c.ID, "Widget", "", 1, "", "")
	tag := db.CreateTag("", "Tag", "", "")

	// Add twice — should not error (INSERT OR IGNORE).
	if err := db.BulkAddTag([]string{item.ID}, []string{c.ID}, tag.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.BulkAddTag([]string{item.ID}, []string{c.ID}, tag.ID); err != nil {
		t.Fatalf("second BulkAddTag should be idempotent, got: %v", err)
	}
}
