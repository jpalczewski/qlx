package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/store"
	"github.com/erxyi/qlx/internal/store/sqlite"
)

func newTestContainerHandler(t *testing.T) (*ContainerHandler, *service.InventoryService) {
	t.Helper()
	s := newHandlerTestStore(t)
	inv := service.NewInventoryService(s)
	notes := service.NewNoteService(s)
	h := NewContainerHandler(inv, nil, nil, nil, notes, nil, &JSONResponder{})
	return h, inv
}

func TestContainerHandler_List_JSON(t *testing.T) {
	h, inv := newTestContainerHandler(t)

	c, err := inv.CreateContainer("", "TestBox", "A box", "", "")
	if err != nil {
		t.Fatalf("create container: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/containers", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var containers []store.Container
	if err := json.NewDecoder(w.Body).Decode(&containers); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].ID != c.ID {
		t.Errorf("expected container ID %s, got %s", c.ID, containers[0].ID)
	}
	if containers[0].Name != "TestBox" {
		t.Errorf("expected name TestBox, got %s", containers[0].Name)
	}
}

func TestContainerHandler_List_WithParentID(t *testing.T) {
	h, inv := newTestContainerHandler(t)

	parent, err := inv.CreateContainer("", "Parent", "", "", "")
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	child, err := inv.CreateContainer(parent.ID, "Child", "", "", "")
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/containers?parent_id="+parent.ID, nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var containers []store.Container
	if err := json.NewDecoder(w.Body).Decode(&containers); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(containers) != 1 {
		t.Fatalf("expected 1 child, got %d", len(containers))
	}
	if containers[0].ID != child.ID {
		t.Errorf("expected child ID %s, got %s", child.ID, containers[0].ID)
	}
}

func TestContainerHandler_Create_JSON(t *testing.T) {
	h, _ := newTestContainerHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := map[string]string{
		"name":        "NewContainer",
		"description": "Created via API",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/containers", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var container store.Container
	if err := json.NewDecoder(w.Body).Decode(&container); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if container.Name != "NewContainer" {
		t.Errorf("expected name NewContainer, got %s", container.Name)
	}
	if container.Description != "Created via API" {
		t.Errorf("expected description 'Created via API', got %s", container.Description)
	}
	if container.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestContainerHandler_Delete_JSON(t *testing.T) {
	h, inv := newTestContainerHandler(t)

	c, err := inv.CreateContainer("", "ToDelete", "", "", "")
	if err != nil {
		t.Fatalf("create container: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/containers/"+c.ID, nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d; body: %s", w.Code, w.Body.String())
	}

	// Verify container is actually deleted
	if inv.GetContainer(c.ID) != nil {
		t.Error("container should have been deleted")
	}
}

func TestContainerHandler_Detail_NotFound(t *testing.T) {
	h, _ := newTestContainerHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/containers/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// newHandlerTestStore creates an in-memory SQLite store for handler tests.
func newHandlerTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	db, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
