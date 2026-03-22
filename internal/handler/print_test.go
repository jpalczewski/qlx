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

func newTestPrintHandler() (*PrintHandler, *service.InventoryService) {
	s := store.NewMemoryStore()
	pm := print.NewPrinterManager(s)
	inv := service.NewInventoryService(s)
	prn := service.NewPrinterService(s)
	tmpl := service.NewTemplateService(s)
	tags := service.NewTagService(s)
	h := NewPrintHandler(pm, inv, prn, tmpl, tags, &JSONResponder{})
	return h, inv
}

func TestPrintHandler_PrintContainer_NotFound(t *testing.T) {
	h, _ := newTestPrintHandler()

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
	h, _ := newTestPrintHandler()

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
	h, inv := newTestPrintHandler()

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
	h, _ := newTestPrintHandler()

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

func TestPrintHandler_ListPrinters_Empty(t *testing.T) {
	h, _ := newTestPrintHandler()

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
