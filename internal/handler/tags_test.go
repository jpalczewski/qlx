package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/store"
)

func newTestTagHandler() (*TagHandler, *service.TagService, *service.InventoryService) {
	s := store.NewMemoryStore()
	tags := service.NewTagService(s)
	inv := service.NewInventoryService(s)
	h := NewTagHandler(tags, inv, &JSONResponder{})
	return h, tags, inv
}

func TestTagHandler_Create_JSON(t *testing.T) {
	h, _, _ := newTestTagHandler()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	form := url.Values{
		"name":  {"Electronics"},
		"color": {"blue"},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/tags", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var tag store.Tag
	if err := json.NewDecoder(w.Body).Decode(&tag); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if tag.Name != "Electronics" {
		t.Errorf("expected name Electronics, got %s", tag.Name)
	}
	if tag.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestTagHandler_AddItemTag_JSON(t *testing.T) {
	h, tags, inv := newTestTagHandler()

	// Create a container, item, and tag
	container, err := inv.CreateContainer("", "Box", "", "", "")
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	item, err := inv.CreateItem(container.ID, "Widget", "", 1, "", "")
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	tag, err := tags.CreateTag("", "Fragile", "", "")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := map[string]string{"tag_id": tag.ID}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/items/"+item.ID+"/tags", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var chips TagChipsData
	if err := json.NewDecoder(w.Body).Decode(&chips); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if chips.ObjectID != item.ID {
		t.Errorf("expected object_id %s, got %s", item.ID, chips.ObjectID)
	}
	if chips.ObjectType != "item" {
		t.Errorf("expected object_type item, got %s", chips.ObjectType)
	}
	if len(chips.Tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(chips.Tags))
	}
	if chips.Tags[0].ID != tag.ID {
		t.Errorf("expected tag ID %s, got %s", tag.ID, chips.Tags[0].ID)
	}
}
