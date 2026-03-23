package sqlite

import (
	"testing"
)

func TestItemStore_CRUD(t *testing.T) {
	db := testStore(t)

	// Need a container first
	c := db.CreateContainer("", "Box", "", "", "")

	item := db.CreateItem(c.ID, "Resistor", "10kΩ", 50, "yellow", "component")
	if item == nil {
		t.Fatal("expected item, got nil")
	}
	if item.Name != "Resistor" {
		t.Errorf("got %q, want %q", item.Name, "Resistor")
	}
	if item.Quantity != 50 {
		t.Errorf("got qty %d, want 50", item.Quantity)
	}

	got := db.GetItem(item.ID)
	if got == nil {
		t.Fatal("expected item, got nil")
	}
	if got.ContainerID != c.ID {
		t.Errorf("wrong container_id")
	}

	updated, err := db.UpdateItem(item.ID, "Resistor 10k", "Updated", 100, "orange", "component")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Quantity != 100 {
		t.Errorf("got qty %d, want 100", updated.Quantity)
	}

	containerID, err := db.DeleteItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if containerID != c.ID {
		t.Errorf("returned containerID %q, want %q", containerID, c.ID)
	}
	if db.GetItem(item.ID) != nil {
		t.Error("expected nil after delete")
	}
}

func TestItemStore_MoveItem(t *testing.T) {
	db := testStore(t)

	src := db.CreateContainer("", "Src", "", "", "")
	dst := db.CreateContainer("", "Dst", "", "", "")
	item := db.CreateItem(src.ID, "Widget", "", 1, "", "")

	if err := db.MoveItem(item.ID, dst.ID); err != nil {
		t.Fatal(err)
	}

	moved := db.GetItem(item.ID)
	if moved == nil || moved.ContainerID != dst.ID {
		t.Error("item not moved")
	}
}

func TestItemStore_DeleteNotFound(t *testing.T) {
	db := testStore(t)
	_, err := db.DeleteItem("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestItemStore_UpdateNotFound(t *testing.T) {
	db := testStore(t)
	_, err := db.UpdateItem("nonexistent", "X", "", 1, "", "")
	if err == nil {
		t.Fatal("expected error")
	}
}
