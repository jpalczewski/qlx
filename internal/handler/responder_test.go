package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
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
