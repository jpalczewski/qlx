package sqlite

import (
	"testing"

	"github.com/erxyi/qlx/internal/store"
)

func TestTagStore_CRUD(t *testing.T) {
	db := testStore(t)

	tag := db.CreateTag("", "Electronics", "blue", "chip")
	if tag == nil {
		t.Fatal("expected tag, got nil")
	}
	if tag.Name != "Electronics" {
		t.Errorf("got %q, want %q", tag.Name, "Electronics")
	}

	got := db.GetTag(tag.ID)
	if got == nil || got.Name != "Electronics" {
		t.Fatal("GetTag failed")
	}

	updated, err := db.UpdateTag(tag.ID, "Electronics & Tech", "green", "circuit")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Electronics & Tech" {
		t.Errorf("got %q", updated.Name)
	}

	parentID, err := db.DeleteTag(tag.ID)
	if err != nil {
		t.Fatal(err)
	}
	if parentID != "" {
		t.Errorf("expected empty parentID, got %q", parentID)
	}
	if db.GetTag(tag.ID) != nil {
		t.Error("expected nil after delete")
	}
}

func TestTagStore_Hierarchy(t *testing.T) {
	db := testStore(t)

	parent := db.CreateTag("", "Electronics", "", "")
	child := db.CreateTag(parent.ID, "Resistors", "", "")
	db.CreateTag(parent.ID, "Capacitors", "", "")

	children := db.TagChildren(parent.ID)
	if len(children) != 2 {
		t.Errorf("got %d children, want 2", len(children))
	}

	path := db.TagPath(child.ID)
	if len(path) != 2 {
		t.Errorf("path length %d, want 2", len(path))
	}

	desc := db.TagDescendants(parent.ID)
	if len(desc) < 2 {
		t.Errorf("got %d descendants, want >= 2", len(desc))
	}
}

func TestTagStore_ItemTagJunction(t *testing.T) {
	db := testStore(t)

	container := db.CreateContainer("", "Box", "", "", "")
	item := db.CreateItem(container.ID, "Resistor", "", 1, "", "")
	tag := db.CreateTag("", "Electronics", "", "")

	if err := db.AddItemTag(item.ID, tag.ID); err != nil {
		t.Fatal(err)
	}

	items := db.ItemsByTag(tag.ID)
	if len(items) != 1 || items[0].ID != item.ID {
		t.Errorf("ItemsByTag returned %v", items)
	}

	// TagIDs on item should be populated
	got := db.GetItem(item.ID)
	if len(got.TagIDs) != 1 || got.TagIDs[0] != tag.ID {
		t.Errorf("item.TagIDs = %v, want [%s]", got.TagIDs, tag.ID)
	}

	if err := db.RemoveItemTag(item.ID, tag.ID); err != nil {
		t.Fatal(err)
	}
	if len(db.ItemsByTag(tag.ID)) != 0 {
		t.Error("tag still has items after remove")
	}
}

func TestTagStore_ContainerTagJunction(t *testing.T) {
	db := testStore(t)

	container := db.CreateContainer("", "Drawer", "", "", "")
	tag := db.CreateTag("", "Storage", "", "")

	if err := db.AddContainerTag(container.ID, tag.ID); err != nil {
		t.Fatal(err)
	}

	containers := db.ContainersByTag(tag.ID)
	if len(containers) != 1 || containers[0].ID != container.ID {
		t.Errorf("ContainersByTag returned %v", containers)
	}

	if err := db.RemoveContainerTag(container.ID, tag.ID); err != nil {
		t.Fatal(err)
	}
	if len(db.ContainersByTag(tag.ID)) != 0 {
		t.Error("tag still has containers after remove")
	}
}

func TestTagStore_DeleteCascades(t *testing.T) {
	db := testStore(t)

	container := db.CreateContainer("", "Box", "", "", "")
	item := db.CreateItem(container.ID, "Widget", "", 1, "", "")
	tag := db.CreateTag("", "Tag", "", "")
	db.AddItemTag(item.ID, tag.ID)

	_, err := db.DeleteTag(tag.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Item should still exist
	if db.GetItem(item.ID) == nil {
		t.Error("item should not be deleted when tag is deleted")
	}
	// But item should have no tags
	got := db.GetItem(item.ID)
	if len(got.TagIDs) != 0 {
		t.Errorf("item should have no tags after tag deletion, got %v", got.TagIDs)
	}
}

func TestTagStore_ResolveTagIDs(t *testing.T) {
	db := testStore(t)

	t1 := db.CreateTag("", "T1", "", "")
	t2 := db.CreateTag("", "T2", "", "")

	resolved := db.ResolveTagIDs([]string{t1.ID, t2.ID})
	if len(resolved) != 2 {
		t.Errorf("got %d resolved tags, want 2", len(resolved))
	}
}

func TestTagStore_TagItemStats(t *testing.T) {
	db := testStore(t)

	container := db.CreateContainer("", "Box", "", "", "")
	tag := db.CreateTag("", "Tag", "", "")

	item1 := db.CreateItem(container.ID, "Widget1", "", 5, "", "")
	item2 := db.CreateItem(container.ID, "Widget2", "", 3, "", "")
	db.AddItemTag(item1.ID, tag.ID)
	db.AddItemTag(item2.ID, tag.ID)

	count, qty, err := db.TagItemStats(tag.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("got count %d, want 2", count)
	}
	if qty != 8 {
		t.Errorf("got qty %d, want 8", qty)
	}
}

func TestTagStore_AllTags(t *testing.T) {
	db := testStore(t)
	db.CreateTag("", "A", "", "")
	db.CreateTag("", "B", "", "")
	all := db.AllTags()
	if len(all) != 2 {
		t.Errorf("got %d tags, want 2", len(all))
	}
}

func TestTagStore_DeleteNotFound(t *testing.T) {
	db := testStore(t)
	_, err := db.DeleteTag("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTagStore_ErrorSentinels(t *testing.T) {
	db := testStore(t)
	_, err := db.UpdateTag("nonexistent", "X", "", "")
	if err != store.ErrTagNotFound {
		t.Errorf("expected ErrTagNotFound, got %v", err)
	}
}
