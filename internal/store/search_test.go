package store

import "testing"

func TestSearch(t *testing.T) {
	s := NewMemoryStore()

	// Seed containers.
	s.CreateContainer("", "Electronics Box", "")
	s.CreateContainer("", "Hardware Tools", "")

	// Seed items.
	s.CreateItem("", "Arduino Nano", "", 1)
	s.CreateItem("", "Resistor Pack", "", 10)

	// Seed tags.
	s.CreateTag("", "electronic parts")
	s.CreateTag("", "tools")

	t.Run("search containers", func(t *testing.T) {
		got := s.SearchContainers("electro")
		if len(got) != 1 {
			t.Fatalf("SearchContainers(%q) returned %d results, want 1", "electro", len(got))
		}
		if got[0].Name != "Electronics Box" {
			t.Errorf("got %q, want %q", got[0].Name, "Electronics Box")
		}
	})

	t.Run("search items", func(t *testing.T) {
		got := s.SearchItems("ino")
		if len(got) != 1 {
			t.Fatalf("SearchItems(%q) returned %d results, want 1", "ino", len(got))
		}
		if got[0].Name != "Arduino Nano" {
			t.Errorf("got %q, want %q", got[0].Name, "Arduino Nano")
		}
	})

	t.Run("search tags", func(t *testing.T) {
		got := s.SearchTags("electronic")
		if len(got) != 1 {
			t.Fatalf("SearchTags(%q) returned %d results, want 1", "electronic", len(got))
		}
		if got[0].Name != "electronic parts" {
			t.Errorf("got %q, want %q", got[0].Name, "electronic parts")
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		got := s.SearchItems("ARDUINO")
		if len(got) != 1 {
			t.Fatalf("SearchItems(%q) returned %d results, want 1", "ARDUINO", len(got))
		}
		if got[0].Name != "Arduino Nano" {
			t.Errorf("got %q, want %q", got[0].Name, "Arduino Nano")
		}
	})

	t.Run("no results", func(t *testing.T) {
		got := s.SearchItems("nonexistent")
		if len(got) != 0 {
			t.Fatalf("SearchItems(%q) returned %d results, want 0", "nonexistent", len(got))
		}
	})
}
