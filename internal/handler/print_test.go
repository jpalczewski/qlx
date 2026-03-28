package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/store"
)

func newTestPrintHandler(t *testing.T) (*PrintHandler, *service.InventoryService) {
	t.Helper()
	s := newHandlerTestStore(t)
	pm := print.NewPrinterManager(s, nil)
	inv := service.NewInventoryService(s)
	prn := service.NewPrinterService(s)
	tmpl := service.NewTemplateService(s)
	tags := service.NewTagService(s)
	notes := service.NewNoteService(s)
	h := NewPrintHandler(pm, nil, inv, prn, tmpl, tags, notes, &JSONResponder{})
	return h, inv
}

func TestPrintHandler_PrintContainer_NotFound(t *testing.T) {
	h, _ := newTestPrintHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]any{
		"printer_id": "some-printer",
		"templates":  []string{"simple"},
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/containers/nonexistent-id/print", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent container, got %d", w.Code)
	}
}

func TestPrintHandler_PrintContainer_InvalidJSON(t *testing.T) {
	h, _ := newTestPrintHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/containers/x/print", bytes.NewReader([]byte("bad")))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPrintHandler_PrintContainer_NoTemplates(t *testing.T) {
	h, inv := newTestPrintHandler(t)

	c, err := inv.CreateContainer("", "Box", "desc", "", "")
	if err != nil {
		t.Fatalf("create container: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]any{
		"printer_id": "some-printer",
		"templates":  []string{},
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/containers/"+c.ID+"/print", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for no templates, got %d", w.Code)
	}
}

func TestPrintHandler_PrintItem_NotFound(t *testing.T) {
	h, _ := newTestPrintHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]any{
		"printer_id": "some-printer",
		"template":   "simple",
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/items/nonexistent-id/print", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPrintHandler_PreviewItem_ReturnsPNG(t *testing.T) {
	h, inv := newTestPrintHandler(t)

	c, err := inv.CreateContainer("", "Box", "", "", "")
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	item, err := inv.CreateItem(c.ID, "Widget", "A fine widget", 1, "", "")
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/items/"+item.ID+"/preview?template=simple", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if ct != "image/png" {
		t.Fatalf("expected Content-Type image/png, got %s", ct)
	}
	// Verify PNG magic bytes
	body := w.Body.Bytes()
	if len(body) < 4 || body[0] != 0x89 || body[1] != 0x50 || body[2] != 0x4e || body[3] != 0x47 {
		t.Fatalf("response is not a valid PNG (first 4 bytes: %x)", body[:4])
	}
}

func TestPrintHandler_PreviewItem_MissingTemplate(t *testing.T) {
	h, inv := newTestPrintHandler(t)

	c, _ := inv.CreateContainer("", "Box", "", "", "")
	item, _ := inv.CreateItem(c.ID, "Widget", "", 1, "", "")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/items/"+item.ID+"/preview", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing template, got %d", w.Code)
	}
}

func TestPrintHandler_PreviewItem_NotFound(t *testing.T) {
	h, _ := newTestPrintHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/items/nonexistent/preview?template=simple", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPrintHandler_PreviewContainer_ReturnsPNG(t *testing.T) {
	h, inv := newTestPrintHandler(t)

	c, err := inv.CreateContainer("", "Storage Box", "A container", "", "")
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	// Add child item for show_children
	if _, err := inv.CreateItem(c.ID, "Child Item", "", 1, "", ""); err != nil {
		t.Fatalf("create item: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/containers/"+c.ID+"/preview?template=simple&show_children=true", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected image/png, got %s", ct)
	}
}

func TestPrintHandler_PreviewContainer_NotFound(t *testing.T) {
	h, _ := newTestPrintHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/containers/nonexistent/preview?template=simple", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPrintHandler_ListPrinters_Empty(t *testing.T) {
	h, _ := newTestPrintHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/printers", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var printers []store.PrinterConfig
	if err := json.NewDecoder(w.Body).Decode(&printers); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(printers) != 0 {
		t.Fatalf("expected 0 printers, got %d", len(printers))
	}
}
