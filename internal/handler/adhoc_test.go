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

func newTestAdhocHandler() *AdhocHandler {
	s := store.NewMemoryStore()
	pm := print.NewPrinterManager(s)
	prn := service.NewPrinterService(s)
	tmpl := service.NewTemplateService(s)
	return NewAdhocHandler(pm, prn, tmpl, &JSONResponder{})
}

func TestAdhocHandler_Print_EmptyText(t *testing.T) {
	h := newTestAdhocHandler()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]string{"text": "", "printer_id": "x", "template": "simple"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/adhoc/print", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty text, got %d", w.Code)
	}
}

func TestAdhocHandler_Print_InvalidJSON(t *testing.T) {
	h := newTestAdhocHandler()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/adhoc/print", bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestAdhocHandler_Page(t *testing.T) {
	h := newTestAdhocHandler()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/quick-print", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
