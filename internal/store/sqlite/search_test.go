package sqlite

import (
	"testing"
)

func TestSearchStore_Items(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	db.CreateItem(c.ID, "Resistor 10k", "10kΩ resistor", 10, "", "")
	db.CreateItem(c.ID, "Capacitor", "ceramic cap", 5, "", "")

	results := db.SearchItems("Resistor")
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
	if len(results) > 0 && results[0].Name != "Resistor 10k" {
		t.Errorf("got name %q, want %q", results[0].Name, "Resistor 10k")
	}
}

func TestSearchStore_Items_Empty(t *testing.T) {
	db := testStore(t)

	c := db.CreateContainer("", "Box", "", "", "")
	db.CreateItem(c.ID, "Widget", "", 1, "", "")

	results := db.SearchItems("")
	if results != nil {
		t.Error("expected nil for empty query")
	}
}

func TestSearchStore_Containers(t *testing.T) {
	db := testStore(t)

	db.CreateContainer("", "Electronics Shelf", "", "", "")
	db.CreateContainer("", "Tools Cabinet", "", "", "")

	results := db.SearchContainers("Electronics")
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

func TestSearchStore_Containers_Empty(t *testing.T) {
	db := testStore(t)

	db.CreateContainer("", "Electronics Shelf", "", "", "")

	results := db.SearchContainers("")
	if results != nil {
		t.Error("expected nil for empty query")
	}
}

func TestSearchStore_Tags(t *testing.T) {
	db := testStore(t)

	db.CreateTag("", "Electronics", "", "")
	db.CreateTag("", "Mechanical", "", "")

	results := db.SearchTags("Elec")
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
	if len(results) > 0 && results[0].Name != "Electronics" {
		t.Errorf("got name %q, want %q", results[0].Name, "Electronics")
	}
}

func TestSearchStore_Tags_Empty(t *testing.T) {
	db := testStore(t)

	db.CreateTag("", "Electronics", "", "")

	results := db.SearchTags("")
	if results != nil {
		t.Error("expected nil for empty query")
	}
}
