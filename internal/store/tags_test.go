package store

import (
	"errors"
	"testing"
)

func TestTagCreate(t *testing.T) {
	s := NewMemoryStore()

	tag := s.CreateTag("", "Electronics", "", "")
	if tag.ID == "" {
		t.Error("CreateTag should set ID")
	}
	if tag.Name != "Electronics" {
		t.Errorf("Name = %q, want %q", tag.Name, "Electronics")
	}
	if tag.ParentID != "" {
		t.Errorf("ParentID = %q, want empty", tag.ParentID)
	}
	if tag.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	child := s.CreateTag(tag.ID, "Cables", "", "")
	if child.ParentID != tag.ID {
		t.Errorf("child ParentID = %q, want %q", child.ParentID, tag.ID)
	}
}

func TestTagGet(t *testing.T) {
	s := NewMemoryStore()

	tag := s.CreateTag("", "Electronics", "", "")
	got := s.GetTag(tag.ID)

	if got == nil {
		t.Fatal("GetTag returned nil")
	}
	if got.Name != "Electronics" {
		t.Errorf("Name = %q, want %q", got.Name, "Electronics")
	}

	// Returns copy, not pointer to internal state.
	got.Name = "Modified"
	internal := s.GetTag(tag.ID)
	if internal.Name != "Electronics" {
		t.Error("GetTag should return a copy, not a reference to internal state")
	}

	if s.GetTag("nonexistent") != nil {
		t.Error("GetTag should return nil for nonexistent ID")
	}
}

func TestTagUpdate(t *testing.T) {
	s := NewMemoryStore()

	tag := s.CreateTag("", "Electronics", "", "")
	updated, err := s.UpdateTag(tag.ID, "Gadgets", "", "")
	if err != nil {
		t.Fatalf("UpdateTag error = %v", err)
	}
	if updated.Name != "Gadgets" {
		t.Errorf("Name = %q, want %q", updated.Name, "Gadgets")
	}

	got := s.GetTag(tag.ID)
	if got.Name != "Gadgets" {
		t.Errorf("stored Name = %q, want %q", got.Name, "Gadgets")
	}

	_, err = s.UpdateTag("nonexistent", "Name", "", "")
	if !errors.Is(err, ErrTagNotFound) {
		t.Errorf("UpdateTag error = %v, want ErrTagNotFound", err)
	}
}

func TestTagDeleteLeaf(t *testing.T) {
	s := NewMemoryStore()

	tag := s.CreateTag("", "Electronics", "", "")
	if err := s.DeleteTag(tag.ID); err != nil {
		t.Fatalf("DeleteTag error = %v", err)
	}
	if s.GetTag(tag.ID) != nil {
		t.Error("DeleteTag should remove tag")
	}

	err := s.DeleteTag("nonexistent")
	if !errors.Is(err, ErrTagNotFound) {
		t.Errorf("DeleteTag error = %v, want ErrTagNotFound", err)
	}
}

func TestTagDeleteWithChildrenFails(t *testing.T) {
	s := NewMemoryStore()

	parent := s.CreateTag("", "Electronics", "", "")
	_ = s.CreateTag(parent.ID, "Cables", "", "")

	err := s.DeleteTag(parent.ID)
	if !errors.Is(err, ErrTagHasChildren) {
		t.Errorf("DeleteTag error = %v, want ErrTagHasChildren", err)
	}
}

func TestTagDeleteRemovesFromItems(t *testing.T) {
	s := NewMemoryStore()

	tag := s.CreateTag("", "Electronics", "", "")
	container := s.CreateContainer("", "Box", "", "", "")
	item := s.CreateItem(container.ID, "Cable", "", 1, "", "")

	if err := s.AddItemTag(item.ID, tag.ID); err != nil {
		t.Fatalf("AddItemTag error = %v", err)
	}
	if err := s.AddContainerTag(container.ID, tag.ID); err != nil {
		t.Fatalf("AddContainerTag error = %v", err)
	}

	if err := s.DeleteTag(tag.ID); err != nil {
		t.Fatalf("DeleteTag error = %v", err)
	}

	updatedItem := s.GetItem(item.ID)
	for _, tid := range updatedItem.TagIDs {
		if tid == tag.ID {
			t.Error("deleted tag ID should be removed from item TagIDs")
		}
	}

	updatedContainer := s.GetContainer(container.ID)
	for _, tid := range updatedContainer.TagIDs {
		if tid == tag.ID {
			t.Error("deleted tag ID should be removed from container TagIDs")
		}
	}
}

func TestTagCRUD(t *testing.T) {
	t.Run("create root tag", TestTagCreate)
	t.Run("get tag", TestTagGet)
	t.Run("update tag", TestTagUpdate)
	t.Run("delete leaf tag", TestTagDeleteLeaf)
	t.Run("delete tag with children fails", TestTagDeleteWithChildrenFails)
	t.Run("delete tag removes from items", TestTagDeleteRemovesFromItems)
}

func TestTagDescendants(t *testing.T) {
	s := NewMemoryStore()

	// Build hierarchy: root -> child1, child2; child1 -> grandchild1, grandchild2
	root := s.CreateTag("", "Root", "", "")
	child1 := s.CreateTag(root.ID, "Child1", "", "")
	child2 := s.CreateTag(root.ID, "Child2", "", "")
	grandchild1 := s.CreateTag(child1.ID, "Grandchild1", "", "")
	grandchild2 := s.CreateTag(child1.ID, "Grandchild2", "", "")

	descendants := s.TagDescendants(root.ID)
	if len(descendants) != 4 {
		t.Fatalf("TagDescendants count = %d, want 4", len(descendants))
	}

	descSet := make(map[string]bool)
	for _, id := range descendants {
		descSet[id] = true
	}
	for _, id := range []string{child1.ID, child2.ID, grandchild1.ID, grandchild2.ID} {
		if !descSet[id] {
			t.Errorf("TagDescendants missing %q", id)
		}
	}
	if descSet[root.ID] {
		t.Error("TagDescendants should not include the root itself")
	}

	// Child1 descendants should be grandchild1 and grandchild2 only.
	child1Desc := s.TagDescendants(child1.ID)
	if len(child1Desc) != 2 {
		t.Errorf("TagDescendants(child1) count = %d, want 2", len(child1Desc))
	}

	// Leaf has no descendants.
	leafDesc := s.TagDescendants(grandchild1.ID)
	if len(leafDesc) != 0 {
		t.Errorf("TagDescendants(leaf) count = %d, want 0", len(leafDesc))
	}
}

func TestItemsByTag(t *testing.T) {
	s := NewMemoryStore()

	electronics := s.CreateTag("", "Electronics", "", "")
	cables := s.CreateTag(electronics.ID, "Cables", "", "")

	container := s.CreateContainer("", "Box", "", "", "")
	hdmi := s.CreateItem(container.ID, "HDMI Cable", "", 1, "", "")
	laptop := s.CreateItem(container.ID, "Laptop", "", 1, "", "")
	book := s.CreateItem(container.ID, "Book", "", 1, "", "") // no tags
	_ = book

	if err := s.AddItemTag(hdmi.ID, cables.ID); err != nil {
		t.Fatalf("AddItemTag error = %v", err)
	}
	if err := s.AddItemTag(laptop.ID, electronics.ID); err != nil {
		t.Fatalf("AddItemTag error = %v", err)
	}

	// Query by parent tag should return items tagged with itself and descendants.
	items := s.ItemsByTag(electronics.ID)
	if len(items) != 2 {
		t.Fatalf("ItemsByTag(electronics) count = %d, want 2", len(items))
	}

	names := make(map[string]bool)
	for _, item := range items {
		names[item.Name] = true
	}
	if !names["HDMI Cable"] || !names["Laptop"] {
		t.Errorf("ItemsByTag names = %v, want HDMI Cable and Laptop", names)
	}

	// Query by child tag should return only directly tagged items.
	cableItems := s.ItemsByTag(cables.ID)
	if len(cableItems) != 1 {
		t.Fatalf("ItemsByTag(cables) count = %d, want 1", len(cableItems))
	}
	if cableItems[0].Name != "HDMI Cable" {
		t.Errorf("ItemsByTag(cables) item = %q, want HDMI Cable", cableItems[0].Name)
	}

	// Query by nonexistent tag should return empty.
	none := s.ItemsByTag("nonexistent")
	if len(none) != 0 {
		t.Errorf("ItemsByTag(nonexistent) count = %d, want 0", len(none))
	}
}

func TestMoveTag(t *testing.T) {
	t.Run("valid move", func(t *testing.T) {
		s := NewMemoryStore()

		root1 := s.CreateTag("", "Root1", "", "")
		root2 := s.CreateTag("", "Root2", "", "")
		child := s.CreateTag(root1.ID, "Child", "", "")

		if err := s.MoveTag(child.ID, root2.ID); err != nil {
			t.Fatalf("MoveTag error = %v", err)
		}

		got := s.GetTag(child.ID)
		if got.ParentID != root2.ID {
			t.Errorf("ParentID = %q, want %q", got.ParentID, root2.ID)
		}
	})

	t.Run("move to root (empty parent)", func(t *testing.T) {
		s := NewMemoryStore()

		root := s.CreateTag("", "Root", "", "")
		child := s.CreateTag(root.ID, "Child", "", "")

		if err := s.MoveTag(child.ID, ""); err != nil {
			t.Fatalf("MoveTag to root error = %v", err)
		}

		got := s.GetTag(child.ID)
		if got.ParentID != "" {
			t.Errorf("ParentID = %q, want empty", got.ParentID)
		}
	})

	t.Run("cycle detection - direct", func(t *testing.T) {
		s := NewMemoryStore()

		a := s.CreateTag("", "A", "", "")
		b := s.CreateTag(a.ID, "B", "", "")

		err := s.MoveTag(a.ID, b.ID)
		if !errors.Is(err, ErrCycleDetected) {
			t.Errorf("MoveTag A->B error = %v, want ErrCycleDetected", err)
		}
	})

	t.Run("cycle detection - indirect", func(t *testing.T) {
		s := NewMemoryStore()

		a := s.CreateTag("", "A", "", "")
		b := s.CreateTag(a.ID, "B", "", "")
		c := s.CreateTag(b.ID, "C", "", "")

		err := s.MoveTag(a.ID, c.ID)
		if !errors.Is(err, ErrCycleDetected) {
			t.Errorf("MoveTag A->C error = %v, want ErrCycleDetected", err)
		}
	})

	t.Run("cycle detection - self", func(t *testing.T) {
		s := NewMemoryStore()

		a := s.CreateTag("", "A", "", "")

		err := s.MoveTag(a.ID, a.ID)
		if !errors.Is(err, ErrCycleDetected) {
			t.Errorf("MoveTag A->A error = %v, want ErrCycleDetected", err)
		}
	})

	t.Run("nonexistent tag", func(t *testing.T) {
		s := NewMemoryStore()

		err := s.MoveTag("nonexistent", "")
		if !errors.Is(err, ErrTagNotFound) {
			t.Errorf("MoveTag error = %v, want ErrTagNotFound", err)
		}
	})

	t.Run("nonexistent new parent", func(t *testing.T) {
		s := NewMemoryStore()

		tag := s.CreateTag("", "Tag", "", "")
		err := s.MoveTag(tag.ID, "nonexistent")
		if !errors.Is(err, ErrTagNotFound) {
			t.Errorf("MoveTag error = %v, want ErrTagNotFound", err)
		}
	})
}

func TestTagAssignmentItems(t *testing.T) {
	s := NewMemoryStore()

	tag := s.CreateTag("", "Electronics", "", "")
	container := s.CreateContainer("", "Box", "", "", "")
	item := s.CreateItem(container.ID, "Cable", "", 1, "", "")

	if err := s.AddItemTag(item.ID, tag.ID); err != nil {
		t.Fatalf("AddItemTag error = %v", err)
	}

	got := s.GetItem(item.ID)
	if !containsString(got.TagIDs, tag.ID) {
		t.Error("AddItemTag should add tag to item")
	}

	// No duplicates.
	if err := s.AddItemTag(item.ID, tag.ID); err != nil {
		t.Fatalf("AddItemTag (duplicate) error = %v", err)
	}
	got = s.GetItem(item.ID)
	count := 0
	for _, tid := range got.TagIDs {
		if tid == tag.ID {
			count++
		}
	}
	if count != 1 {
		t.Errorf("tag should appear exactly once, got %d", count)
	}

	if err := s.RemoveItemTag(item.ID, tag.ID); err != nil {
		t.Fatalf("RemoveItemTag error = %v", err)
	}
	got = s.GetItem(item.ID)
	if containsString(got.TagIDs, tag.ID) {
		t.Error("RemoveItemTag should remove tag from item")
	}
}

func TestTagAssignmentContainers(t *testing.T) {
	s := NewMemoryStore()

	tag := s.CreateTag("", "Storage", "", "")
	container := s.CreateContainer("", "Box", "", "", "")

	if err := s.AddContainerTag(container.ID, tag.ID); err != nil {
		t.Fatalf("AddContainerTag error = %v", err)
	}

	got := s.GetContainer(container.ID)
	if !containsString(got.TagIDs, tag.ID) {
		t.Error("AddContainerTag should add tag to container")
	}

	// No duplicates.
	if err := s.AddContainerTag(container.ID, tag.ID); err != nil {
		t.Fatalf("AddContainerTag (duplicate) error = %v", err)
	}
	got = s.GetContainer(container.ID)
	count := 0
	for _, tid := range got.TagIDs {
		if tid == tag.ID {
			count++
		}
	}
	if count != 1 {
		t.Errorf("tag should appear exactly once, got %d", count)
	}

	if err := s.RemoveContainerTag(container.ID, tag.ID); err != nil {
		t.Fatalf("RemoveContainerTag error = %v", err)
	}
	got = s.GetContainer(container.ID)
	if containsString(got.TagIDs, tag.ID) {
		t.Error("RemoveContainerTag should remove tag from container")
	}
}

func TestTagAssignmentErrors(t *testing.T) {
	s := NewMemoryStore()

	tag := s.CreateTag("", "Tag", "", "")
	container := s.CreateContainer("", "Box", "", "", "")
	item := s.CreateItem(container.ID, "Item", "", 1, "", "")

	err := s.AddItemTag("nonexistent", tag.ID)
	if !errors.Is(err, ErrItemNotFound) {
		t.Errorf("AddItemTag error = %v, want ErrItemNotFound", err)
	}

	err = s.AddItemTag(item.ID, "nonexistent")
	if !errors.Is(err, ErrTagNotFound) {
		t.Errorf("AddItemTag error = %v, want ErrTagNotFound", err)
	}

	err = s.RemoveItemTag("nonexistent", tag.ID)
	if !errors.Is(err, ErrItemNotFound) {
		t.Errorf("RemoveItemTag error = %v, want ErrItemNotFound", err)
	}

	err = s.AddContainerTag("nonexistent", tag.ID)
	if !errors.Is(err, ErrContainerNotFound) {
		t.Errorf("AddContainerTag error = %v, want ErrContainerNotFound", err)
	}

	err = s.AddContainerTag(container.ID, "nonexistent")
	if !errors.Is(err, ErrTagNotFound) {
		t.Errorf("AddContainerTag error = %v, want ErrTagNotFound", err)
	}

	err = s.RemoveContainerTag("nonexistent", tag.ID)
	if !errors.Is(err, ErrContainerNotFound) {
		t.Errorf("RemoveContainerTag error = %v, want ErrContainerNotFound", err)
	}
}

func TestTagAssignment(t *testing.T) {
	t.Run("add and remove item tags", TestTagAssignmentItems)
	t.Run("add and remove container tags", TestTagAssignmentContainers)
	t.Run("error cases", TestTagAssignmentErrors)
}
