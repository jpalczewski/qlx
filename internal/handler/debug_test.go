package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/store"
)

func newTestDebugHandler() *DebugHandler {
	s := store.NewMemoryStore()
	pm := print.NewPrinterManager(s)
	prn := service.NewPrinterService(s)
	return NewDebugHandler(pm, prn, &JSONResponder{})
}

func TestDebugHandler_CalibrationImage_ReturnsPNG(t *testing.T) {
	h := newTestDebugHandler()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/calibration.png?w=100&h=80", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected Content-Type image/png, got %s", ct)
	}
	// PNG files start with the 8-byte PNG signature
	if w.Body.Len() < 8 {
		t.Fatal("response body too small to be a valid PNG")
	}
	sig := w.Body.Bytes()[:8]
	pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i := range pngSig {
		if sig[i] != pngSig[i] {
			t.Fatalf("invalid PNG signature at byte %d: got %x, want %x", i, sig[i], pngSig[i])
		}
	}
}

func TestDebugHandler_CalibrationImage_DefaultSize(t *testing.T) {
	h := newTestDebugHandler()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/calibration.png", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDebugHandler_CalibrationImage_InvalidSizeFallback(t *testing.T) {
	h := newTestDebugHandler()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Width too large should fallback to default
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/calibration.png?w=9999&h=9999", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDebugHandler_PrinterInfo_MissingID(t *testing.T) {
	h := newTestDebugHandler()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/printer-info", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDebugHandler_PrinterInfo_NotFound(t *testing.T) {
	h := newTestDebugHandler()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/printer-info?id=nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
