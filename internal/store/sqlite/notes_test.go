package sqlite

import (
	"testing"
)

func TestNoteStore_CRUD(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	item := db.CreateItem(c.ID, "Widget", "", 1, "", "")

	// Create note on container
	note := db.CreateNote(c.ID, "", "Fragile", "Handle with care", "red", "alert-triangle")
	if note == nil {
		t.Fatal("expected note, got nil")
	}
	if note.Title != "Fragile" {
		t.Errorf("got title %q, want %q", note.Title, "Fragile")
	}
	if note.ContainerID != c.ID {
		t.Errorf("got container_id %q, want %q", note.ContainerID, c.ID)
	}
	if note.ItemID != "" {
		t.Errorf("expected empty item_id, got %q", note.ItemID)
	}

	// Create note on item
	noteItem := db.CreateNote("", item.ID, "Review", "Check by 2026-04", "yellow", "clock")
	if noteItem == nil {
		t.Fatal("expected note, got nil")
	}
	if noteItem.ItemID != item.ID {
		t.Errorf("got item_id %q, want %q", noteItem.ItemID, item.ID)
	}

	// Get
	got := db.GetNote(note.ID)
	if got == nil {
		t.Fatal("expected note, got nil")
	}
	if got.Content != "Handle with care" {
		t.Errorf("got content %q, want %q", got.Content, "Handle with care")
	}

	// Update
	updated, err := db.UpdateNote(note.ID, "Very Fragile", "Handle with extreme care", "orange", "alert-triangle")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Very Fragile" {
		t.Errorf("got title %q, want %q", updated.Title, "Very Fragile")
	}

	// Delete
	if err := db.DeleteNote(note.ID); err != nil {
		t.Fatal(err)
	}
	if db.GetNote(note.ID) != nil {
		t.Error("expected nil after delete")
	}
}

func TestNoteStore_ContainerNotes(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	db.CreateNote(c.ID, "", "Note 1", "", "", "")
	db.CreateNote(c.ID, "", "Note 2", "", "", "")

	notes := db.ContainerNotes(c.ID)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	// Newest first (DESC)
	if notes[0].Title != "Note 2" {
		t.Errorf("expected newest first, got %q", notes[0].Title)
	}
}

func TestNoteStore_ItemNotes(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	item := db.CreateItem(c.ID, "Widget", "", 1, "", "")
	db.CreateNote("", item.ID, "Item Note", "", "", "")

	notes := db.ItemNotes(item.ID)
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
}

func TestNoteStore_CascadeDeleteContainer(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	note := db.CreateNote(c.ID, "", "Will vanish", "", "", "")

	_, err := db.DeleteContainer(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if db.GetNote(note.ID) != nil {
		t.Error("expected note to be cascade-deleted with container")
	}
}

func TestNoteStore_CascadeDeleteItem(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	item := db.CreateItem(c.ID, "Widget", "", 1, "", "")
	note := db.CreateNote("", item.ID, "Will vanish", "", "", "")

	_, err := db.DeleteItem(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if db.GetNote(note.ID) != nil {
		t.Error("expected note to be cascade-deleted with item")
	}
}

func TestNoteStore_DeleteNotFound(t *testing.T) {
	db := testStore(t)
	err := db.DeleteNote("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNoteStore_UpdateNotFound(t *testing.T) {
	db := testStore(t)
	_, err := db.UpdateNote("nonexistent", "X", "", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
}
