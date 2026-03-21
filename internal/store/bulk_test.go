package store

import (
	"testing"
)

// helpers

func setupBulkStore(t *testing.T) (*Store, *Container, *Container, *Container, *Item, *Item) {
	t.Helper()
	s := NewMemoryStore()

	root := s.CreateContainer("", "Root", "")
	child := s.CreateContainer(root.ID, "Child", "")
	other := s.CreateContainer("", "Other", "")

	item1 := s.CreateItem(root.ID, "Item1", "", 1)
	item2 := s.CreateItem(root.ID, "Item2", "", 1)

	return s, root, child, other, item1, item2
}

func TestBulkMoveItems(t *testing.T) {
	s, root, _, other, item1, item2 := setupBulkStore(t)
	_ = root

	errs := s.MoveItems([]string{item1.ID, item2.ID}, other.ID)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	got1 := s.GetItem(item1.ID)
	got2 := s.GetItem(item2.ID)
	if got1.ContainerID != other.ID {
		t.Errorf("item1 container: want %s, got %s", other.ID, got1.ContainerID)
	}
	if got2.ContainerID != other.ID {
		t.Errorf("item2 container: want %s, got %s", other.ID, got2.ContainerID)
	}
}

func TestBulkMoveContainers(t *testing.T) {
	s := NewMemoryStore()
	parent := s.CreateContainer("", "Parent", "")
	a := s.CreateContainer("", "A", "")
	b := s.CreateContainer("", "B", "")

	errs := s.MoveContainers([]string{a.ID, b.ID}, parent.ID)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	gotA := s.GetContainer(a.ID)
	gotB := s.GetContainer(b.ID)
	if gotA.ParentID != parent.ID {
		t.Errorf("container A parent: want %s, got %s", parent.ID, gotA.ParentID)
	}
	if gotB.ParentID != parent.ID {
		t.Errorf("container B parent: want %s, got %s", parent.ID, gotB.ParentID)
	}
}

func TestBulkMoveContainersCycleDetection(t *testing.T) {
	s := NewMemoryStore()
	parent := s.CreateContainer("", "Parent", "")
	child := s.CreateContainer(parent.ID, "Child", "")

	// Moving parent into child would create a cycle.
	errs := s.MoveContainers([]string{parent.ID}, child.ID)
	if len(errs) == 0 {
		t.Fatal("expected cycle error, got none")
	}
	if errs[0].ID != parent.ID {
		t.Errorf("expected error for parent ID %s, got %s", parent.ID, errs[0].ID)
	}

	// Ensure parent was NOT moved.
	got := s.GetContainer(parent.ID)
	if got.ParentID != "" {
		t.Errorf("parent should still be root, got ParentID=%s", got.ParentID)
	}
}

func TestBulkMoveContainersIntraBatchAncestry(t *testing.T) {
	s := NewMemoryStore()
	ancestor := s.CreateContainer("", "Ancestor", "")
	descendant := s.CreateContainer(ancestor.ID, "Descendant", "")
	target := s.CreateContainer("", "Target", "")

	// Moving both ancestor and its descendant in the same batch should fail.
	errs := s.MoveContainers([]string{ancestor.ID, descendant.ID}, target.ID)
	if len(errs) == 0 {
		t.Fatal("expected intra-batch ancestry error, got none")
	}

	// Verify neither was moved.
	gotAncestor := s.GetContainer(ancestor.ID)
	gotDescendant := s.GetContainer(descendant.ID)
	if gotAncestor.ParentID != "" {
		t.Errorf("ancestor should still be root, got ParentID=%s", gotAncestor.ParentID)
	}
	if gotDescendant.ParentID != ancestor.ID {
		t.Errorf("descendant parent should still be ancestor, got ParentID=%s", gotDescendant.ParentID)
	}
}

func TestBulkDeleteItems(t *testing.T) {
	s, _, _, _, item1, item2 := setupBulkStore(t)

	deleted, errs := s.DeleteItems([]string{item1.ID, item2.ID})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(deleted) != 2 {
		t.Fatalf("expected 2 deleted, got %d", len(deleted))
	}

	if s.GetItem(item1.ID) != nil {
		t.Error("item1 should be deleted")
	}
	if s.GetItem(item2.ID) != nil {
		t.Error("item2 should be deleted")
	}
}

func TestBulkDeleteContainersRejectsNonEmpty(t *testing.T) {
	s, root, child, other, _, _ := setupBulkStore(t)

	// root has items (should fail), child is empty but has root as parent (should succeed),
	// other is empty (should succeed).
	// We also add an item to a grandchild to verify child-with-children is rejected.
	grandchild := s.CreateContainer(child.ID, "Grandchild", "")
	_ = s.CreateItem(grandchild.ID, "GItem", "", 1)

	// child has a child container (grandchild), so child should fail with ErrContainerHasChildren.
	deleted, errs := s.DeleteContainers([]string{root.ID, child.ID, other.ID})

	if len(errs) == 0 {
		t.Fatal("expected errors for non-empty containers")
	}

	errIDs := make(map[string]string, len(errs))
	for _, e := range errs {
		errIDs[e.ID] = e.Reason
	}

	if _, ok := errIDs[root.ID]; !ok {
		t.Error("expected error for root (has items)")
	}
	if _, ok := errIDs[child.ID]; !ok {
		t.Error("expected error for child (has child containers)")
	}

	// other should be deleted
	deletedSet := make(map[string]bool, len(deleted))
	for _, id := range deleted {
		deletedSet[id] = true
	}
	if !deletedSet[other.ID] {
		t.Error("expected other (empty container) to be deleted")
	}
}

func TestBulkMove(t *testing.T) {
	s := NewMemoryStore()
	target := s.CreateContainer("", "Target", "")
	c1 := s.CreateContainer("", "C1", "")
	c2 := s.CreateContainer("", "C2", "")
	item1 := s.CreateItem("", "I1", "", 1)
	item2 := s.CreateItem("", "I2", "", 1)

	errs := s.BulkMove([]string{item1.ID, item2.ID}, []string{c1.ID, c2.ID}, target.ID)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if s.GetContainer(c1.ID).ParentID != target.ID {
		t.Error("c1 not moved to target")
	}
	if s.GetContainer(c2.ID).ParentID != target.ID {
		t.Error("c2 not moved to target")
	}
	if s.GetItem(item1.ID).ContainerID != target.ID {
		t.Error("item1 not moved to target")
	}
	if s.GetItem(item2.ID).ContainerID != target.ID {
		t.Error("item2 not moved to target")
	}
}

func TestBulkDelete(t *testing.T) {
	s := NewMemoryStore()
	c1 := s.CreateContainer("", "C1", "")
	c2 := s.CreateContainer("", "C2", "")
	item1 := s.CreateItem(c1.ID, "I1", "", 1)
	item2 := s.CreateItem(c1.ID, "I2", "", 1)

	// Delete items first then empty containers.
	deleted, errs := s.BulkDelete([]string{item1.ID, item2.ID}, []string{c1.ID, c2.ID})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(deleted) != 4 {
		t.Fatalf("expected 4 deleted (2 items + 2 containers), got %d", len(deleted))
	}

	if s.GetItem(item1.ID) != nil {
		t.Error("item1 should be deleted")
	}
	if s.GetItem(item2.ID) != nil {
		t.Error("item2 should be deleted")
	}
	if s.GetContainer(c1.ID) != nil {
		t.Error("c1 should be deleted")
	}
	if s.GetContainer(c2.ID) != nil {
		t.Error("c2 should be deleted")
	}
}

func TestBulkAddTag(t *testing.T) {
	s := NewMemoryStore()
	tag := s.CreateTag("", "Bulk")

	c1 := s.CreateContainer("", "C1", "")
	c2 := s.CreateContainer("", "C2", "")
	item1 := s.CreateItem(c1.ID, "I1", "", 1)
	item2 := s.CreateItem(c1.ID, "I2", "", 1)

	// Include a missing ID to verify skipping.
	err := s.BulkAddTag(
		[]string{item1.ID, item2.ID, "nonexistent-item"},
		[]string{c1.ID, c2.ID, "nonexistent-container"},
		tag.ID,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, id := range []string{item1.ID, item2.ID} {
		got := s.GetItem(id)
		if !containsString(got.TagIDs, tag.ID) {
			t.Errorf("item %s missing tag", id)
		}
	}
	for _, id := range []string{c1.ID, c2.ID} {
		got := s.GetContainer(id)
		if !containsString(got.TagIDs, tag.ID) {
			t.Errorf("container %s missing tag", id)
		}
	}

	// Calling again should not duplicate.
	err = s.BulkAddTag([]string{item1.ID}, []string{c1.ID}, tag.ID)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	got := s.GetItem(item1.ID)
	count := 0
	for _, tid := range got.TagIDs {
		if tid == tag.ID {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected tag to appear once, got %d", count)
	}
}

func TestBulkAddTagNotFound(t *testing.T) {
	s := NewMemoryStore()
	err := s.BulkAddTag(nil, nil, "no-such-tag")
	if err != ErrTagNotFound {
		t.Errorf("expected ErrTagNotFound, got %v", err)
	}
}
