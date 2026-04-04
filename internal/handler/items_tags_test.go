package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/erxyi/qlx/internal/service"
)

func newTestItemHandlerWithTags(t *testing.T) (*ItemHandler, *service.InventoryService, *service.TagService) {
	t.Helper()
	s := newHandlerTestStore(t)
	inv := service.NewInventoryService(s)
	tags := service.NewTagService(s)
	notes := service.NewNoteService(s)
	h := NewItemHandler(inv, nil, nil, nil, notes, tags, &JSONResponder{})
	return h, inv, tags
}

func TestCreateItemWithInvalidTag(t *testing.T) {
	h, inv, _ := newTestItemHandlerWithTags(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	container, err := inv.CreateContainer("", "Box", "", "", "")
	if err != nil {
		t.Fatalf("create container: %v", err)
	}

	form := url.Values{}
	form.Set("container_id", container.ID)
	form.Set("name", "Bolt")
	form.Add("tag_ids", "nonexistent-id")

	req := httptest.NewRequest("POST", "/items", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// Item must NOT have been created
	items := inv.ContainerItems(container.ID)
	if len(items) != 0 {
		t.Errorf("expected 0 items (item should not be created), got %d", len(items))
	}
}

func TestCreateItemWithTags(t *testing.T) {
	h, inv, tags := newTestItemHandlerWithTags(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	container, err := inv.CreateContainer("", "Box", "", "", "")
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	tag1, err := tags.CreateTag("", "metal", "", "")
	if err != nil {
		t.Fatalf("create tag1: %v", err)
	}
	tag2, err := tags.CreateTag("", "round", "", "")
	if err != nil {
		t.Fatalf("create tag2: %v", err)
	}

	form := url.Values{}
	form.Set("container_id", container.ID)
	form.Set("name", "Bolt")
	form.Set("quantity", "5")
	form.Add("tag_ids", tag1.ID)
	form.Add("tag_ids", tag2.ID)

	req := httptest.NewRequest("POST", "/items", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("expected 2xx, got %d: %s", w.Code, w.Body.String())
	}

	items := inv.ContainerItems(container.ID)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(items[0].TagIDs) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(items[0].TagIDs), items[0].TagIDs)
	}
}
