package handler

import (
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

func TestJSONResponder_Respond(t *testing.T) {
	resp := &JSONResponder{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	data := map[string]string{"name": "test"}
	resp.Respond(w, r, http.StatusCreated, data, "", nil)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestJSONResponder_RespondError(t *testing.T) {
	resp := &JSONResponder{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	resp.RespondError(w, r, errors.New("not found"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestJSONResponder_Redirect(t *testing.T) {
	resp := &JSONResponder{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)

	resp.Redirect(w, r, "/containers", map[string]bool{"ok": true})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHTMLResponder_JSON_WhenAcceptJSON(t *testing.T) {
	resp := newTestHTMLResponder(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept", "application/json")

	data := map[string]string{"name": "test"}
	resp.Respond(w, r, http.StatusOK, data, "containers", func() any { return nil })

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected JSON content type, got %s", ct)
	}
}

func TestHTMLResponder_Partial_WhenHTMX(t *testing.T) {
	resp := newTestHTMLResponder(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("HX-Request", "true")

	vmCalled := false
	resp.Respond(w, r, http.StatusOK, nil, "containers", func() any {
		vmCalled = true
		return "test data"
	})

	if !vmCalled {
		t.Fatal("expected vmFn to be called for HTMX request")
	}
}

func TestHTMLResponder_Redirect_HTMX(t *testing.T) {
	resp := newTestHTMLResponder(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("HX-Request", "true")

	resp.Redirect(w, r, "/containers", nil)

	if w.Header().Get("HX-Redirect") != "/containers" {
		t.Fatalf("expected HX-Redirect header, got %q", w.Header().Get("HX-Redirect"))
	}
}

func TestHTMLResponder_Redirect_Browser(t *testing.T) {
	resp := newTestHTMLResponder(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)

	resp.Redirect(w, r, "/containers", nil)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
}

func TestHTMLResponder_Redirect_JSON(t *testing.T) {
	resp := newTestHTMLResponder(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Accept", "application/json")

	resp.Redirect(w, r, "/containers", map[string]bool{"ok": true})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHTMLResponder_VaryHeader(t *testing.T) {
	resp := newTestHTMLResponder(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept", "application/json")

	resp.Respond(w, r, http.StatusOK, nil, "", nil)

	if v := w.Header().Get("Vary"); v != "HX-Request" {
		t.Fatalf("expected Vary: HX-Request, got %q", v)
	}
}

func newTestHTMLResponder(t *testing.T) *HTMLResponder {
	t.Helper()
	tmpl := template.Must(template.New("containers").Parse(
		`{{define "containers"}}content:{{.Data}}{{end}}{{define "layout"}}layout:{{.Data}}{{end}}`))
	return NewHTMLResponder(
		map[string]*template.Template{"containers": tmpl},
		webutil.NewTranslations(),
	)
}
