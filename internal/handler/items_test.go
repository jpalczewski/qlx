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

func newTestItemHandler(t *testing.T) (*ItemHandler, *service.InventoryService) {
	t.Helper()
	s := newHandlerTestStore(t)
	inv := service.NewInventoryService(s)
	notes := service.NewNoteService(s)
	h := NewItemHandler(inv, nil, nil, nil, notes, nil, &JSONResponder{})
	return h, inv
}

func TestItemHandler_Create_JSON(t *testing.T) {
	h, inv := newTestItemHandler(t)

	// Create a container first (items need a container)
	container, err := inv.CreateContainer("", "TestBox", "", "", "")
	if err != nil {
		t.Fatalf("create container: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := map[string]any{
		"container_id": container.ID,
		"name":         "TestItem",
		"description":  "A test item",
		"quantity":     3,
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/items", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var item store.Item
	if err := json.NewDecoder(w.Body).Decode(&item); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if item.Name != "TestItem" {
		t.Errorf("expected name TestItem, got %s", item.Name)
	}
	if item.Description != "A test item" {
		t.Errorf("expected description 'A test item', got %s", item.Description)
	}
	if item.Quantity != 3 {
		t.Errorf("expected quantity 3, got %d", item.Quantity)
	}
	if item.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestItemHandler_Detail_NotFound(t *testing.T) {
	h, _ := newTestItemHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/items/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
