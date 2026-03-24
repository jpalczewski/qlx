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

func TestExportStore_ExportContainerTree(t *testing.T) {
	db := testStore(t)

	root := db.CreateContainer("", "Root", "", "", "")
	child := db.CreateContainer(root.ID, "Child", "", "", "")
	grandchild := db.CreateContainer(child.ID, "Grandchild", "", "", "")
	_ = db.CreateContainer("", "Unrelated", "", "", "")

	tree, err := db.ExportContainerTree(root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 3 {
		t.Fatalf("got %d containers, want 3 (root+child+grandchild)", len(tree))
	}

	ids := make(map[string]bool)
	for _, c := range tree {
		ids[c.ID] = true
	}
	if !ids[root.ID] || !ids[child.ID] || !ids[grandchild.ID] {
		t.Error("tree missing expected containers")
	}
}

func TestExportStore_ExportItems_SingleContainer(t *testing.T) {
	db := testStore(t)

	tag := db.CreateTag("", "Electronics", "", "")
	c := db.CreateContainer("", "Box", "", "", "")
	item := db.CreateItem(c.ID, "Widget", "A small widget", 3, "", "")
	db.AddItemTag(item.ID, tag.ID)

	items, err := db.ExportItems(c.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Name != "Widget" {
		t.Errorf("name = %q, want Widget", items[0].Name)
	}
	if items[0].Quantity != 3 {
		t.Errorf("quantity = %d, want 3", items[0].Quantity)
	}
	if len(items[0].TagNames) != 1 || items[0].TagNames[0] != "Electronics" {
		t.Errorf("tags = %v, want [Electronics]", items[0].TagNames)
	}
}

func TestExportStore_ExportItems_Recursive(t *testing.T) {
	db := testStore(t)

	root := db.CreateContainer("", "Root", "", "", "")
	child := db.CreateContainer(root.ID, "Child", "", "", "")
	db.CreateItem(root.ID, "RootItem", "", 1, "", "")
	db.CreateItem(child.ID, "ChildItem", "", 1, "", "")
	unrelated := db.CreateContainer("", "Unrelated", "", "", "")
	db.CreateItem(unrelated.ID, "UnrelatedItem", "", 1, "", "")

	items, err := db.ExportItems(root.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	names := map[string]bool{}
	for _, i := range items {
		names[i.Name] = true
	}
	if !names["RootItem"] || !names["ChildItem"] {
		t.Errorf("got names %v, want RootItem and ChildItem", names)
	}
}

func TestExportStore_ExportItems_NoTags(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	db.CreateItem(c.ID, "Plain", "", 1, "", "")

	items, err := db.ExportItems(c.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].TagNames != nil {
		t.Errorf("tags = %v, want nil", items[0].TagNames)
	}
}
