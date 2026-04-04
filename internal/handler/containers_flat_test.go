package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFlatList(t *testing.T) {
	h, inv := newTestContainerHandler(t)

	root, _ := inv.CreateContainer("", "Warsztat", "", "", "")
	shelf, _ := inv.CreateContainer(root.ID, "Półka 1", "", "", "")
	drawer, _ := inv.CreateContainer(shelf.ID, "Szuflada A", "", "", "archive-box")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/containers/flat", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []FlatContainer
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 containers, got %d", len(result))
	}

	// Find drawer and check path
	var found bool
	for _, fc := range result {
		if fc.ID == drawer.ID {
			found = true
			if fc.Path != "Warsztat / Półka 1" {
				t.Errorf("expected path 'Warsztat / Półka 1', got %q", fc.Path)
			}
			if fc.Icon != "archive-box" {
				t.Errorf("expected icon 'archive-box', got %q", fc.Icon)
			}
		}
	}
	if !found {
		t.Error("drawer not found in flat list")
	}
}
