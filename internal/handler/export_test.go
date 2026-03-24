package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/erxyi/qlx/internal/service"
)

func newTestExportHandler(t *testing.T) (*ExportHandler, *service.InventoryService) {
	t.Helper()
	s := newHandlerTestStore(t)
	exp := service.NewExportService(s)
	inv := service.NewInventoryService(s)
	h := NewExportHandler(exp, inv)
	return h, inv
}

func TestExportHandler_ValidFormats(t *testing.T) {
	tests := []struct {
		format      string
		contentType string
	}{
		{"csv", "text/csv; charset=utf-8"},
		{"json", "application/json"},
		{"md", "text/markdown; charset=utf-8"},
	}

	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			h, inv := newTestExportHandler(t)

			c, err := inv.CreateContainer("", "TestBox", "A box", "", "")
			if err != nil {
				t.Fatalf("create container: %v", err)
			}
			_, err = inv.CreateItem(c.ID, "Widget", "A widget", 5, "", "")
			if err != nil {
				t.Fatalf("create item: %v", err)
			}

			mux := http.NewServeMux()
			h.RegisterRoutes(mux)

			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/export?format="+tc.format, nil)
			mux.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("format %s: expected 200, got %d; body: %s", tc.format, w.Code, w.Body.String())
			}
			ct := w.Header().Get("Content-Type")
			if ct != tc.contentType {
				t.Errorf("format %s: expected Content-Type %q, got %q", tc.format, tc.contentType, ct)
			}
		})
	}
}

func TestExportHandler_MissingFormat(t *testing.T) {
	h, _ := newTestExportHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/export", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestExportHandler_InvalidFormat(t *testing.T) {
	h, _ := newTestExportHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/export?format=xml", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestExportHandler_InvalidMDStyle(t *testing.T) {
	h, _ := newTestExportHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/export?format=md&md_style=fancy", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestExportHandler_ContainerNotFound(t *testing.T) {
	h, _ := newTestExportHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/export?format=csv&container=nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestExportHandler_PerContainer(t *testing.T) {
	h, inv := newTestExportHandler(t)

	c, err := inv.CreateContainer("", "StorageBox", "", "", "")
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	_, err = inv.CreateItem(c.ID, "Bolt", "A small bolt", 10, "", "")
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	// Create another container with its own item — should not appear in scoped export
	other, err := inv.CreateContainer("", "OtherBox", "", "", "")
	if err != nil {
		t.Fatalf("create other container: %v", err)
	}
	_, err = inv.CreateItem(other.ID, "Nut", "A nut", 3, "", "")
	if err != nil {
		t.Fatalf("create other item: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/export?format=csv&container="+c.ID, nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "Bolt") {
		t.Errorf("expected body to contain 'Bolt', got: %s", body)
	}
	if strings.Contains(body, "Nut") {
		t.Errorf("expected body NOT to contain 'Nut' from other container, got: %s", body)
	}
}

func TestExportHandler_Download(t *testing.T) {
	h, _ := newTestExportHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/export?format=csv&download=true", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("expected Content-Disposition to contain 'attachment', got: %q", cd)
	}
}

func TestExportHandler_Download_NoHeader_WhenFalse(t *testing.T) {
	h, _ := newTestExportHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/export?format=csv", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if cd := w.Header().Get("Content-Disposition"); cd != "" {
		t.Errorf("expected no Content-Disposition header without download=true, got: %q", cd)
	}
}

func TestExportHandler_RecursiveExport(t *testing.T) {
	h, inv := newTestExportHandler(t)

	parent, err := inv.CreateContainer("", "Parent", "", "", "")
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	_, err = inv.CreateItem(parent.ID, "ParentItem", "", 1, "", "")
	if err != nil {
		t.Fatalf("create parent item: %v", err)
	}

	child, err := inv.CreateContainer(parent.ID, "Child", "", "", "")
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	_, err = inv.CreateItem(child.ID, "ChildItem", "", 2, "", "")
	if err != nil {
		t.Fatalf("create child item: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/export?format=csv&container="+parent.ID+"&recursive=true", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "ParentItem") {
		t.Errorf("expected body to contain 'ParentItem', got: %s", body)
	}
	if !strings.Contains(body, "ChildItem") {
		t.Errorf("expected body to contain 'ChildItem', got: %s", body)
	}
}
