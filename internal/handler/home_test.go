package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/erxyi/qlx/internal/embedded"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

type homeEnv struct {
	mux   *http.ServeMux
	inv   *service.InventoryService
	ws    *service.WorkingSetService
	shelf *store.Container
	box   *store.Container
	other *store.Container
}

func newHomeEnv(t *testing.T) *homeEnv {
	t.Helper()
	s := newHandlerTestStore(t)
	inv := service.NewInventoryService(s)
	ws := service.NewWorkingSetService(s)
	notes := service.NewNoteService(s)

	tr := webutil.NewTranslations()
	if err := tr.LoadFromFS(embedded.Static, "static/i18n"); err != nil {
		t.Fatal(err)
	}
	tags := service.NewTagService(s)
	tmpls := LoadTemplates(TemplateFuncs{ResolveTags: tags.ResolveTagIDs, GetTag: tags.GetTag, IsPinned: ws.IsPinned})
	resp := NewHTMLResponder(tmpls, tr)

	shelf, err := inv.CreateContainer("", "Shelf", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	box, err := inv.CreateContainer(shelf.ID, "Blue Box", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := inv.CreateContainer("", "Drawer", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inv.CreateItem(box.ID, "Resistor", "", 1, "", ""); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	NewHomeHandler(inv, ws, resp).RegisterRoutes(mux)
	NewContainerHandler(inv, nil, nil, nil, notes, nil, resp).RegisterRoutes(mux)
	return &homeEnv{mux: mux, inv: inv, ws: ws, shelf: shelf, box: box, other: other}
}

func (e *homeEnv) get(target string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", target, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	return w
}

func TestHome_HTMLEmptyState(t *testing.T) {
	e := newHomeEnv(t)
	w := e.get("/", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"<!DOCTYPE html>", "W użyciu", `data-home="empty"`, "Przypnij segregator", `href="/containers"`, "Wszystkie segregatory"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
	if strings.Contains(body, `data-home="pins"`) || strings.Contains(body, `data-home="recents"`) {
		t.Error("empty state must not render pin/recent sections")
	}
	// The wide search names an items group, but only in search-result
	// labels: the page itself has no item list.
	if strings.Contains(body, `id="item-list"`) {
		t.Error("start screen must not contain an items view")
	}
}

func TestHome_HTMLPinnedAndRecent(t *testing.T) {
	e := newHomeEnv(t)
	if err := e.ws.Pin(e.other.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.ws.RecordOpen(e.box.ID); err != nil {
		t.Fatal(err)
	}

	w := e.get("/", map[string]string{"HX-Request": "true"})
	body := w.Body.String()
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("HTMX request must get a fragment")
	}
	for _, want := range []string{
		`data-home="pins"`, `data-home="recents"`,
		`href="/containers/` + e.other.ID + `"`, `href="/containers/` + e.box.ID + `"`,
		"Drawer", "Blue Box", "Shelf", `aria-pressed="true"`, `aria-pressed="false"`,
		`href="/containers/` + e.other.ID + `?print=1"`, `aria-label="Dodaj przedmiot do: Drawer"`,
		`class="home-card-add"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
	if strings.Contains(body, `data-home="empty"`) {
		t.Error("empty state rendered despite entries")
	}
}

func TestHome_JSONContractUnchanged(t *testing.T) {
	e := newHomeEnv(t)
	w := e.get("/", map[string]string{"Accept": "application/json"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var got []store.Container
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := e.inv.AllContainers()
	if len(got) != len(want) || len(got) != 3 {
		t.Fatalf("got %d containers, want %d (3)", len(got), len(want))
	}
}

func TestContainersRoot_HasNoItemsSection(t *testing.T) {
	e := newHomeEnv(t)
	body := e.get("/containers", nil).Body.String()
	if !strings.Contains(body, `id="container-list"`) {
		t.Fatal("root must list child containers")
	}
	for _, bad := range []string{`id="item-list"`, `class="items"`, "Brak przedmiotów", "qe-type-toggle", "Nowy item"} {
		if strings.Contains(body, bad) {
			t.Errorf("root view must not contain %q", bad)
		}
	}
	if !strings.Contains(body, `data-container-only="true"`) {
		t.Error("root quick entry must be container-only")
	}
}

func TestContainerView_KeepsItemsSection(t *testing.T) {
	e := newHomeEnv(t)
	body := e.get("/containers/"+e.box.ID, nil).Body.String()
	for _, want := range []string{`id="item-list"`, "qe-type-toggle", "Nowy item", "Resistor"} {
		if !strings.Contains(body, want) {
			t.Errorf("container view missing %q", want)
		}
	}
	if strings.Contains(body, "data-container-only") {
		t.Error("container view must not be container-only")
	}
}
