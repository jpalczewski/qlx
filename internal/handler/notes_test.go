package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/store"
)

func newTestNoteHandler(t *testing.T) (*NoteHandler, *service.NoteService, *service.InventoryService) {
	t.Helper()
	s := newHandlerTestStore(t)
	inv := service.NewInventoryService(s)
	notes := service.NewNoteService(s)
	h := NewNoteHandler(notes, inv, &JSONResponder{})
	return h, notes, inv
}

func TestNoteHandler_Create_JSON(t *testing.T) {
	h, _, inv := newTestNoteHandler(t)

	container, err := inv.CreateContainer("", "TestBox", "", "", "")
	if err != nil {
		t.Fatalf("create container: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := map[string]any{
		"container_id": container.ID,
		"title":        "Fragile",
		"content":      "Handle with care",
		"color":        "red",
		"icon":         "warning",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/notes", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var note store.Note
	if err := json.NewDecoder(w.Body).Decode(&note); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if note.Title != "Fragile" {
		t.Errorf("expected title Fragile, got %s", note.Title)
	}
	if note.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestNoteHandler_Detail_NotFound(t *testing.T) {
	h, _, _ := newTestNoteHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/notes/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestNoteHandler_Update_JSON(t *testing.T) {
	h, notesSvc, inv := newTestNoteHandler(t)

	container, _ := inv.CreateContainer("", "Box", "", "", "")
	note, _ := notesSvc.CreateNote(container.ID, "", "Old", "old content", "", "")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := map[string]any{
		"title":   "New Title",
		"content": "new content",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/notes/"+note.ID, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var updated store.Note
	json.NewDecoder(w.Body).Decode(&updated) //nolint:errcheck
	if updated.Title != "New Title" {
		t.Errorf("expected title 'New Title', got %s", updated.Title)
	}
}

func TestNoteHandler_Delete_JSON(t *testing.T) {
	h, notesSvc, inv := newTestNoteHandler(t)

	container, _ := inv.CreateContainer("", "Box", "", "", "")
	note, _ := notesSvc.CreateNote(container.ID, "", "ToDelete", "", "", "")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/notes/"+note.ID, nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestNoteHandler_ContainerNotes(t *testing.T) {
	h, notesSvc, inv := newTestNoteHandler(t)

	container, _ := inv.CreateContainer("", "Box", "", "", "")
	notesSvc.CreateNote(container.ID, "", "Note1", "", "", "") //nolint:errcheck
	notesSvc.CreateNote(container.ID, "", "Note2", "", "", "") //nolint:errcheck

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/containers/"+container.ID+"/notes", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var notes []store.Note
	json.NewDecoder(w.Body).Decode(&notes) //nolint:errcheck
	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}
}
