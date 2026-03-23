package sqlite

import (
	"testing"
)

func TestTemplateStore_CRUD(t *testing.T) {
	db := testStore(t)

	tmpl, err := db.CreateTemplate("Label A", []string{"tag1"}, "universal", 62.0, 29.0, 696, 271, `[]`)
	if err != nil {
		t.Fatal(err)
	}
	if tmpl == nil {
		t.Fatal("expected template, got nil")
	}
	if tmpl.Name != "Label A" {
		t.Errorf("got %q", tmpl.Name)
	}

	got := db.GetTemplate(tmpl.ID)
	if got == nil || got.Name != "Label A" {
		t.Fatal("GetTemplate failed")
	}

	updated, err := db.UpdateTemplate(tmpl.ID, "Label B", []string{}, "item", 50.0, 25.0, 500, 250, `[{"type":"text"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Label B" {
		t.Errorf("got %q", updated.Name)
	}

	all := db.AllTemplates()
	if len(all) != 1 {
		t.Errorf("got %d templates, want 1", len(all))
	}

	if err := db.DeleteTemplate(tmpl.ID); err != nil {
		t.Fatal(err)
	}
	if db.GetTemplate(tmpl.ID) != nil {
		t.Error("expected nil after delete")
	}
}

func TestTemplateStore_UpdateNotFound(t *testing.T) {
	db := testStore(t)
	_, err := db.UpdateTemplate("nonexistent", "X", nil, "", 0, 0, 0, 0, "[]")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTemplateStore_DeleteNotFound(t *testing.T) {
	db := testStore(t)
	err := db.DeleteTemplate("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}
