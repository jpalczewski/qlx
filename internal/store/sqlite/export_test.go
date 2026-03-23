package sqlite

import (
	"testing"
)

func TestExportStore_AllItems(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	db.CreateItem(c.ID, "A", "", 1, "", "")
	db.CreateItem(c.ID, "B", "", 1, "", "")

	all := db.AllItems()
	if len(all) != 2 {
		t.Errorf("got %d items, want 2", len(all))
	}
}

func TestExportStore_AllItems_OrderedByName(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	db.CreateItem(c.ID, "Zebra", "", 1, "", "")
	db.CreateItem(c.ID, "Alpha", "", 1, "", "")

	all := db.AllItems()
	if len(all) != 2 {
		t.Fatalf("got %d items, want 2", len(all))
	}
	if all[0].Name != "Alpha" {
		t.Errorf("expected first item to be %q, got %q", "Alpha", all[0].Name)
	}
}

func TestExportStore_ExportData(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	db.CreateItem(c.ID, "Widget", "", 1, "", "")

	containers, items := db.ExportData()
	if len(containers) != 1 {
		t.Errorf("got %d containers, want 1", len(containers))
	}
	if len(items) != 1 {
		t.Errorf("got %d items, want 1", len(items))
	}

	if _, ok := containers[c.ID]; !ok {
		t.Errorf("container %q not in export map", c.ID)
	}
}

func TestExportStore_ExportData_Empty(t *testing.T) {
	db := testStore(t)

	containers, items := db.ExportData()
	if len(containers) != 0 {
		t.Errorf("got %d containers, want 0", len(containers))
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}
