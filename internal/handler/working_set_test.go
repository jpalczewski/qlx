package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/erxyi/qlx/internal/embedded"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

type wsEnv struct {
	mux    *http.ServeMux
	ws     *service.WorkingSetService
	shelf  *store.Container
	box    *store.Container
	drawer *store.Container
}

func newWSEnv(t *testing.T, resp Responder) *wsEnv {
	t.Helper()
	s := newHandlerTestStore(t)
	inv := service.NewInventoryService(s)
	ws := service.NewWorkingSetService(s)

	shelf, err := inv.CreateContainer("", "Shelf", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	box, err := inv.CreateContainer(shelf.ID, "Blue Box", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	drawer, err := inv.CreateContainer("", "Drawer", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inv.CreateItem(box.ID, "Resistor", "", 1, "", ""); err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		resp = &JSONResponder{}
	}
	mux := http.NewServeMux()
	NewWorkingSetHandler(ws, resp).RegisterRoutes(mux)
	return &wsEnv{mux: mux, ws: ws, shelf: shelf, box: box, drawer: drawer}
}

func (e *wsEnv) do(method, target, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	return w
}

func TestWorkingSet_PinUnpin(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        func(e *wsEnv) string
		body        string
		wantStatus  int
		wantTrigger bool
	}{
		{"pin ok", "POST", func(e *wsEnv) string { return "/api/working-set/pins/" + e.box.ID }, "", 204, true},
		{"pin unknown", "POST", func(*wsEnv) string { return "/api/working-set/pins/nope" }, "", 404, false},
		{"unpin ok", "DELETE", func(e *wsEnv) string { return "/api/working-set/pins/" + e.box.ID }, "", 204, true},
		{"unpin not pinned is idempotent", "DELETE", func(e *wsEnv) string { return "/api/working-set/pins/" + e.drawer.ID }, "", 204, true},
		{"reorder ok", "PUT", func(*wsEnv) string { return "/api/working-set/pins" }, `{"ids":[]}`, 204, true},
		{"reorder bad json", "PUT", func(*wsEnv) string { return "/api/working-set/pins" }, `{`, 400, false},
		{"reorder missing ids", "PUT", func(*wsEnv) string { return "/api/working-set/pins" }, `{}`, 400, false},
		{"reorder null ids", "PUT", func(*wsEnv) string { return "/api/working-set/pins" }, `{"ids":null}`, 400, false},
		{"reorder trailing data", "PUT", func(*wsEnv) string { return "/api/working-set/pins" }, `{"ids":[]} {"x":1}`, 400, false},
		{"reorder oversized", "PUT", func(*wsEnv) string { return "/api/working-set/pins" }, `{"ids":["` + strings.Repeat("a", maxReorderBody) + `"]}`, 413, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newWSEnv(t, nil)
			w := e.do(tc.method, tc.path(e), tc.body)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", w.Code, tc.wantStatus, w.Body.String())
			}
			got := w.Header().Get("HX-Trigger")
			if tc.wantTrigger && got != "workingSetChanged" {
				t.Errorf("HX-Trigger = %q, want workingSetChanged", got)
			}
			if !tc.wantTrigger && got != "" {
				t.Errorf("HX-Trigger = %q, want empty", got)
			}
		})
	}
}

func TestWorkingSet_ReorderErrorIsJSON(t *testing.T) {
	e := newWSEnv(t, nil)
	w := e.do("PUT", "/api/working-set/pins", `{`)
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["error"] == "" {
		t.Fatalf("want JSON error body, got %q (%v)", w.Body.String(), err)
	}
}

func TestWorkingSet_PinReorderEffects(t *testing.T) {
	e := newWSEnv(t, nil)
	for _, id := range []string{e.box.ID, e.drawer.ID} {
		if w := e.do("POST", "/api/working-set/pins/"+id, ""); w.Code != 204 {
			t.Fatalf("pin %s: %d", id, w.Code)
		}
	}
	if !e.ws.IsPinned(e.box.ID) {
		t.Fatal("box should be pinned")
	}
	if w := e.do("PUT", "/api/working-set/pins", `{"ids":["`+e.drawer.ID+`","`+e.box.ID+`"]}`); w.Code != 204 {
		t.Fatalf("reorder: %d", w.Code)
	}
	snap := decodeSnapshot(t, e.do("GET", "/api/working-set", ""))
	if len(snap.Pins) != 2 || snap.Pins[0].ID != e.drawer.ID || snap.Pins[1].ID != e.box.ID {
		t.Fatalf("unexpected pin order: %+v", snap.Pins)
	}
	e.do("DELETE", "/api/working-set/pins/"+e.box.ID, "")
	if e.ws.IsPinned(e.box.ID) {
		t.Error("box should be unpinned")
	}
}

func decodeSnapshot(t *testing.T, w *httptest.ResponseRecorder) workingSetJSON {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var snap workingSetJSON
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return snap
}

func TestWorkingSet_RecordOpen(t *testing.T) {
	tests := []struct {
		name   string
		id     func(e *wsEnv) string
		status int
	}{
		{"known", func(e *wsEnv) string { return e.box.ID }, 204},
		{"unknown", func(*wsEnv) string { return "nope" }, 404},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newWSEnv(t, nil)
			w := e.do("POST", "/api/working-set/recents/"+tc.id(e), "")
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			if w.Header().Get("HX-Trigger") != "" {
				t.Error("recents must not set HX-Trigger")
			}
		})
	}
}

func TestWorkingSet_Snapshot(t *testing.T) {
	e := newWSEnv(t, nil)

	empty := e.do("GET", "/api/working-set", "")
	if !strings.Contains(empty.Body.String(), `"pins":[]`) || !strings.Contains(empty.Body.String(), `"recents":[]`) {
		t.Fatalf("empty snapshot must use [] not null: %s", empty.Body.String())
	}

	e.do("POST", "/api/working-set/pins/"+e.drawer.ID, "")
	e.do("POST", "/api/working-set/recents/"+e.box.ID, "")
	e.do("POST", "/api/working-set/recents/"+e.shelf.ID, "")

	snap := decodeSnapshot(t, e.do("GET", "/api/working-set", ""))
	if len(snap.Pins) != 1 || snap.Pins[0].ID != e.drawer.ID || !snap.Pins[0].Pinned {
		t.Fatalf("pins = %+v", snap.Pins)
	}
	if snap.Pins[0].LastOpenedAt != nil {
		t.Error("never-opened pin must have null last_opened_at")
	}
	if len(snap.Recents) != 2 || snap.Recents[0].ID != e.shelf.ID || snap.Recents[1].ID != e.box.ID {
		t.Fatalf("recents = %+v", snap.Recents)
	}
	box := snap.Recents[1]
	if box.ItemCount != 1 || box.Path != "Shelf / Blue Box" || box.OpenCount != 1 || box.LastOpenedAt == nil || box.ParentID != e.shelf.ID {
		t.Errorf("box entry = %+v", box)
	}

}

func TestWorkingSet_SnapshotRecentLimit(t *testing.T) {
	e := newWSEnv(t, nil)
	e.do("POST", "/api/working-set/recents/"+e.box.ID, "")
	e.do("POST", "/api/working-set/recents/"+e.shelf.ID, "")

	limited := decodeSnapshot(t, e.do("GET", "/api/working-set?recent=1", ""))
	if len(limited.Recents) != 1 || limited.Recents[0].ID != e.shelf.ID {
		t.Errorf("recent=1 recents = %+v", limited.Recents)
	}
	bad := decodeSnapshot(t, e.do("GET", "/api/working-set?recent=abc", ""))
	if len(bad.Recents) != 2 {
		t.Errorf("invalid recent should fall back to default, got %d", len(bad.Recents))
	}
}

func TestWorkingSet_QuickSwitch(t *testing.T) {
	e := newWSEnv(t, nil)
	e.do("POST", "/api/working-set/pins/"+e.drawer.ID, "")

	tests := []struct {
		name    string
		query   string
		wantIDs []string
	}{
		{"name prefix", "?q=blue", []string{e.box.ID}},
		{"path match", "?q=shelf", []string{e.shelf.ID, e.box.ID}},
		{"no match", "?q=zzz", nil},
		{"empty query lists pins", "", []string{e.drawer.ID}},
		{"limit", "?q=shelf&limit=1", []string{e.shelf.ID}},
		{"bad limit uses default", "?q=shelf&limit=-3", []string{e.shelf.ID, e.box.ID}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := e.do("GET", "/api/quick-switch"+tc.query, "")
			if w.Code != 200 {
				t.Fatalf("status = %d", w.Code)
			}
			var got []quickSwitchJSON
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("results must be [] not null")
			}
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("got %d results %+v, want %d", len(got), got, len(tc.wantIDs))
			}
			for i, id := range tc.wantIDs {
				if got[i].ID != id {
					t.Errorf("result %d = %s, want %s", i, got[i].ID, id)
				}
			}
		})
	}

	var one []quickSwitchJSON
	_ = json.Unmarshal(e.do("GET", "/api/quick-switch?q=blue", "").Body.Bytes(), &one)
	if one[0].Name != "Blue Box" || one[0].Path != "Shelf / Blue Box" || one[0].Score <= 0 {
		t.Errorf("shape = %+v", one[0])
	}
}

func newRailEnv(t *testing.T) *wsEnv {
	t.Helper()
	tr := webutil.NewTranslations()
	if err := tr.LoadFromFS(embedded.Static, "static/i18n"); err != nil {
		t.Fatal(err)
	}
	var e *wsEnv
	tmpls := LoadTemplates(TemplateFuncs{IsPinned: func(id string) bool { return e.ws.IsPinned(id) }})
	e = newWSEnv(t, NewHTMLResponder(tmpls, tr))
	return e
}

func TestWorkingSet_RailFragment(t *testing.T) {
	e := newRailEnv(t)
	e.do("POST", "/api/working-set/pins/"+e.drawer.ID, "")
	e.do("POST", "/api/working-set/recents/"+e.box.ID, "")

	r := httptest.NewRequest("GET", "/partials/rail", nil)
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type = %q", ct)
	}
	body := w.Body.String()
	for _, want := range []string{
		"<aside", `role="heading" aria-level="3"`,
		`href="/containers/` + e.drawer.ID + `"`, "Drawer",
		`href="/containers/` + e.box.ID + `"`, "Blue Box",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment missing %q:\n%s", want, body)
		}
	}
	for _, bad := range []string{"<h1", "<h2", "<nav"} {
		if strings.Contains(body, bad) {
			t.Errorf("fragment must not contain %q", bad)
		}
	}
}

func TestWorkingSet_RailEmpty(t *testing.T) {
	e := newRailEnv(t)
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, httptest.NewRequest("GET", "/partials/rail", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "rail-empty") {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestTemplateFuncs_IsPinned(t *testing.T) {
	e := newRailEnv(t)
	e.do("POST", "/api/working-set/pins/"+e.box.ID, "")
	tmpls := LoadTemplates(TemplateFuncs{IsPinned: e.ws.IsPinned})
	tpl, err := tmpls["rail"].Clone()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tpl.New("probe").Parse(`{{ IsPinned .A }}/{{ IsPinned .B }}`); err != nil {
		t.Fatalf("IsPinned not registered: %v", err)
	}
	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "probe", map[string]string{"A": e.box.ID, "B": e.drawer.ID}); err != nil {
		t.Fatal(err)
	}
	if sb.String() != "true/false" {
		t.Errorf("got %q", sb.String())
	}
}

func TestWorkingSet_SnapshotRecentAboveTen(t *testing.T) {
	s := newHandlerTestStore(t)
	inv := service.NewInventoryService(s)
	mux := http.NewServeMux()
	NewWorkingSetHandler(service.NewWorkingSetService(s), &JSONResponder{}).RegisterRoutes(mux)
	for i := 0; i < 30; i++ {
		c, err := inv.CreateContainer("", "C", "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.RecordContainerOpen(c.ID, time.UnixMilli(1_700_000_000_000+int64(i))); err != nil {
			t.Fatal(err)
		}
	}
	for q, want := range map[string]int{"": 8, "?recent=25": 25, "?recent=500": 30} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/working-set"+q, nil))
		if got := len(decodeSnapshot(t, w).Recents); got != want {
			t.Errorf("%q: recents = %d, want %d", q, got, want)
		}
	}
}

func TestWorkingSet_RailJSONFallback(t *testing.T) {
	e := newRailEnv(t)
	e.do("POST", "/api/working-set/pins/"+e.drawer.ID, "")
	r := httptest.NewRequest("GET", "/partials/rail", nil)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	snap := decodeSnapshot(t, w)
	if len(snap.Pins) != 1 || snap.Pins[0].ID != e.drawer.ID {
		t.Errorf("pins = %+v", snap.Pins)
	}
}

func TestWorkingSet_RailShowsPath(t *testing.T) {
	e := newRailEnv(t)
	e.do("POST", "/api/working-set/pins/"+e.box.ID, "")
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, httptest.NewRequest("GET", "/partials/rail", nil))
	if !strings.Contains(w.Body.String(), "Shelf / Blue Box") {
		t.Errorf("rail should show container path:\n%s", w.Body.String())
	}
}
