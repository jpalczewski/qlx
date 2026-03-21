package app

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	qlprint "github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/store"
)

func TestUIRoot_RenderModes(t *testing.T) {
	mem := store.NewMemoryStore()
	srv := NewServer(mem, qlprint.NewPrintService(mem))

	fullReq := httptest.NewRequest("GET", "/ui", nil)
	fullRes := httptest.NewRecorder()
	srv.ServeHTTP(fullRes, fullReq)

	if fullRes.Code != 200 {
		t.Fatalf("full page status = %d, want 200", fullRes.Code)
	}
	if !strings.Contains(fullRes.Body.String(), "<!DOCTYPE html>") {
		t.Fatal("expected full layout for non-HTMX /ui request")
	}

	fragReq := httptest.NewRequest("GET", "/ui", nil)
	fragReq.Header.Set("HX-Request", "true")
	fragRes := httptest.NewRecorder()
	srv.ServeHTTP(fragRes, fragReq)

	if fragRes.Code != 200 {
		t.Fatalf("fragment status = %d, want 200", fragRes.Code)
	}
	if strings.Contains(fragRes.Body.String(), "<!DOCTYPE html>") {
		t.Fatal("expected HTML fragment for HTMX /ui request")
	}
}

func TestRootRedirectsToUI(t *testing.T) {
	mem := store.NewMemoryStore()
	srv := NewServer(mem, qlprint.NewPrintService(mem))
	req := httptest.NewRequest("GET", "/", nil)
	res := httptest.NewRecorder()

	srv.ServeHTTP(res, req)

	if res.Code != 200 {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if !strings.Contains(res.Body.String(), "<!DOCTYPE html>") {
		t.Fatal("expected full layout on root")
	}
}

func TestUIAddItemInContainer(t *testing.T) {
	mem := store.NewMemoryStore()
	container := mem.CreateContainer("", "Box", "")
	srv := NewServer(mem, qlprint.NewPrintService(mem))

	form := url.Values{}
	form.Set("container_id", container.ID)
	form.Set("name", "Cable")
	form.Set("description", "HDMI")

	req := httptest.NewRequest("POST", "/ui/actions/items", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	res := httptest.NewRecorder()

	srv.ServeHTTP(res, req)

	if res.Code != 200 {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if !strings.Contains(res.Body.String(), "Cable") {
		t.Fatal("expected created item to be rendered in response")
	}
}
