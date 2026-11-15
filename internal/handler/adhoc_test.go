package handler

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/print/encoder/brother"
	"github.com/erxyi/qlx/internal/print/encoder/niimbot"
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

// previewPNGSize requests /adhoc/preview with the given query and returns the PNG size.
func previewPNGSize(t *testing.T, h *AdhocHandler, query string) image.Point {
	t.Helper()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/adhoc/preview?"+query, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	img, err := png.Decode(w.Body)
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	return img.Bounds().Size()
}

func TestAdhocHandler_Preview_UsesPrinterMedia(t *testing.T) {
	s := newHandlerTestStore(t)
	pm := print.NewPrinterManager(s, nil)
	pm.RegisterEncoder(&niimbot.NiimbotEncoder{})
	brotherEnc := &brother.BrotherEncoder{}
	pm.RegisterEncoder(brotherEnc)
	prn := service.NewPrinterService(s)
	h := NewAdhocHandler(pm, prn, service.NewTemplateService(s), &JSONResponder{})

	b1, err := prn.AddPrinter("Test B1", "niimbot", "B1", "mock", "")
	if err != nil {
		t.Fatalf("AddPrinter: %v", err)
	}
	ql, err := prn.AddPrinter("Test QL", brotherEnc.Name(), brotherEnc.Models()[0].ID, "mock", "")
	if err != nil {
		t.Fatalf("AddPrinter: %v", err)
	}

	tests := []struct {
		name  string
		query string
		wantW int
	}{
		{"no printer uses width param", "template=name&text=Kabel&width=500", 500},
		{"niimbot width wins over width param", "template=name&text=Kabel&width=500&printer_id=" + b1.ID, 384},
		{"brother printhead width without media status", "template=name&text=Kabel&printer_id=" + ql.ID, 720},
		{"unknown printer falls back", "template=name&text=Kabel&width=500&printer_id=nope", 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := previewPNGSize(t, h, tt.query).X; got != tt.wantW {
				t.Errorf("preview width = %d, want %d", got, tt.wantW)
			}
		})
	}
}
