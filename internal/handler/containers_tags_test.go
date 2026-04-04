package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/erxyi/qlx/internal/service"
)

func newTestContainerHandlerWithTags(t *testing.T) (*ContainerHandler, *service.InventoryService, *service.TagService) {
	t.Helper()
	s := newHandlerTestStore(t)
	inv := service.NewInventoryService(s)
	tags := service.NewTagService(s)
	notes := service.NewNoteService(s)
	h := NewContainerHandler(inv, nil, nil, nil, notes, tags, &JSONResponder{})
	return h, inv, tags
}

func TestCreateContainerWithTags(t *testing.T) {
	h, inv, tags := newTestContainerHandlerWithTags(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	parent, err := inv.CreateContainer("", "Parent", "", "", "")
	if err != nil {
		t.Fatalf("create parent container: %v", err)
	}
	tag1, err := tags.CreateTag("", "electronics", "", "")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}

	form := url.Values{}
	form.Set("parent_id", parent.ID)
	form.Set("name", "Child Box")
	form.Add("tag_ids", tag1.ID)

	req := httptest.NewRequest("POST", "/containers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("expected 2xx, got %d: %s", w.Code, w.Body.String())
	}

	children := inv.ContainerChildren(parent.ID)
	if len(children) != 1 {
		t.Fatalf("expected 1 child container, got %d", len(children))
	}
	if len(children[0].TagIDs) != 1 {
		t.Errorf("expected 1 tag, got %d: %v", len(children[0].TagIDs), children[0].TagIDs)
	}
}

func TestCreateContainerWithInvalidTag(t *testing.T) {
	h, inv, _ := newTestContainerHandlerWithTags(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	parent, err := inv.CreateContainer("", "Parent", "", "", "")
	if err != nil {
		t.Fatalf("create parent container: %v", err)
	}

	form := url.Values{}
	form.Set("parent_id", parent.ID)
	form.Set("name", "Child Box")
	form.Add("tag_ids", "nonexistent-tag-id")

	req := httptest.NewRequest("POST", "/containers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// Container must NOT have been created
	children := inv.ContainerChildren(parent.ID)
	if len(children) != 0 {
		t.Errorf("expected 0 children (container should not be created), got %d", len(children))
	}
}
