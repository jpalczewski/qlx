package webutil

import (
	"net/http/httptest"
	"testing"

	"github.com/erxyi/qlx/internal/store"
)

func TestStoreHTTPStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{store.ErrContainerNotFound, 404},
		{store.ErrItemNotFound, 404},
		{store.ErrTagNotFound, 404},
		{store.ErrPrinterNotFound, 404},
		{store.ErrContainerHasChildren, 409},
		{store.ErrContainerHasItems, 409},
		{store.ErrTagHasChildren, 409},
		{store.ErrCycleDetected, 400},
		{store.ErrInvalidParent, 400},
		{store.ErrInvalidContainer, 400},
		{store.ErrNoteNotFound, 404},
	}
	for _, tt := range tests {
		if got := StoreHTTPStatus(tt.err); got != tt.status {
			t.Errorf("StoreHTTPStatus(%v) = %d, want %d", tt.err, got, tt.status)
		}
	}
}

func TestWriteStoreErrorJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteStoreErrorJSON(w, store.ErrItemNotFound)
	if w.Code != 404 {
		t.Errorf("got status %d, want 404", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("got Content-Type %q, want application/json", ct)
	}
}
