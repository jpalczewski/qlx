package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/service"
)

func newTestAdhocHandler(t *testing.T) *AdhocHandler {
	t.Helper()
	s := newHandlerTestStore(t)
	pm := print.NewPrinterManager(s, nil)
	prn := service.NewPrinterService(s)
	tmpl := service.NewTemplateService(s)
	return NewAdhocHandler(pm, prn, tmpl, &JSONResponder{})
}

func TestAdhocHandler_Print_EmptyText(t *testing.T) {
	h := newTestAdhocHandler(t)

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
	h := newTestAdhocHandler(t)

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

func TestAdhocHandler_Preview_ReturnsPNG(t *testing.T) {
	h := newTestAdhocHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/adhoc/preview?template=simple&text=Hello+World", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if ct != "image/png" {
		t.Fatalf("expected Content-Type image/png, got %s", ct)
	}
	body := w.Body.Bytes()
	if len(body) < 4 || body[0] != 0x89 || body[1] != 0x50 {
		t.Fatalf("response is not a valid PNG")
	}
}

func TestAdhocHandler_Preview_MissingText(t *testing.T) {
	h := newTestAdhocHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/adhoc/preview?template=simple", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAdhocHandler_Preview_MissingTemplate(t *testing.T) {
	h := newTestAdhocHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/adhoc/preview?text=Hello", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAdhocHandler_Page(t *testing.T) {
	h := newTestAdhocHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/quick-print", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
