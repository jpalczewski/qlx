package app

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	qlprint "github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/store/sqlite"
)

func newAppTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	db, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRoot_RenderModes(t *testing.T) {
	mem := newAppTestStore(t)
	srv := NewServer(mem, qlprint.NewPrinterManager(mem, nil), nil)

	fullReq := httptest.NewRequest("GET", "/", nil)
	fullRes := httptest.NewRecorder()
	srv.ServeHTTP(fullRes, fullReq)

	if fullRes.Code != 200 {
		t.Fatalf("full page status = %d, want 200", fullRes.Code)
	}
	if !strings.Contains(fullRes.Body.String(), "<!DOCTYPE html>") {
		t.Fatal("expected full layout for non-HTMX / request")
	}

	fragReq := httptest.NewRequest("GET", "/", nil)
	fragReq.Header.Set("HX-Request", "true")
	fragRes := httptest.NewRecorder()
	srv.ServeHTTP(fragRes, fragReq)

	if fragRes.Code != 200 {
		t.Fatalf("fragment status = %d, want 200", fragRes.Code)
	}
	if strings.Contains(fragRes.Body.String(), "<!DOCTYPE html>") {
		t.Fatal("expected HTML fragment for HTMX / request")
	}
}

func TestWorkingSet_Wiring(t *testing.T) {
	mem := newAppTestStore(t)
	srv := NewServer(mem, qlprint.NewPrinterManager(mem, nil), nil)
	c := mem.CreateContainer("", "Wiring Box", "", "", "")

	do := func(method, target string, hdr map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, nil)
		for k, v := range hdr {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
		return w
	}

	if w := do("POST", "/api/working-set/pins/"+c.ID, map[string]string{"HX-Request": "true"}); w.Code != 204 || w.Header().Get("HX-Trigger") != "workingSetChanged" {
		t.Fatalf("pin: status %d trigger %q", w.Code, w.Header().Get("HX-Trigger"))
	}
	if w := do("GET", "/partials/rail", map[string]string{"HX-Request": "true"}); w.Code != 200 || !strings.Contains(w.Body.String(), "Wiring Box") {
		t.Fatalf("rail: status %d body %s", w.Code, w.Body.String())
	}

	// GET / JSON keeps returning the plain container list.
	w := do("GET", "/", map[string]string{"Accept": "application/json"})
	var containers []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &containers); err != nil {
		t.Fatalf("GET / JSON must be a container array: %v (%s)", err, w.Body.String())
	}
	if len(containers) != 1 || containers[0]["id"] != c.ID {
		t.Fatalf("unexpected containers: %v", containers)
	}
}

func TestAddItemInContainer(t *testing.T) {
	mem := newAppTestStore(t)
	container := mem.CreateContainer("", "Box", "", "", "")
	srv := NewServer(mem, qlprint.NewPrinterManager(mem, nil), nil)

	form := url.Values{}
	form.Set("container_id", container.ID)
	form.Set("name", "Cable")
	form.Set("description", "HDMI")

	req := httptest.NewRequest("POST", "/items", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	res := httptest.NewRecorder()

	srv.ServeHTTP(res, req)

	if res.Code != 200 && res.Code != 201 {
		t.Fatalf("status = %d, want 200 or 201", res.Code)
	}
	if !strings.Contains(res.Body.String(), "Cable") {
		t.Fatal("expected created item to be rendered in response")
	}
}
