package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

type listRowEnv struct {
	mux       *http.ServeMux
	inv       *service.InventoryService
	templates *templateSet
}

type templateSet struct {
	render func(page, define string, data any) string
}

func newListRowEnv(t *testing.T) *listRowEnv {
	t.Helper()
	s := newHandlerTestStore(t)
	inv := service.NewInventoryService(s)
	notes := service.NewNoteService(s)

	tmpls := LoadTemplates(TemplateFuncs{
		ResolveTags: func([]string) []store.Tag { return nil },
		GetTag:      func(string) *store.Tag { return nil },
	})
	tr := webutil.NewTranslations()
	resp := NewHTMLResponder(tmpls, tr)

	mux := http.NewServeMux()
	NewContainerHandler(inv, nil, nil, nil, notes, nil, resp).RegisterRoutes(mux)
	NewItemHandler(inv, nil, nil, nil, notes, nil, resp).RegisterRoutes(mux)

	return &listRowEnv{
		mux: mux,
		inv: inv,
		templates: &templateSet{render: func(page, define string, data any) string {
			var buf bytes.Buffer
			if err := tmpls[page].ExecuteTemplate(&buf, define, PageData{Lang: "en", translator: tr, Data: data}); err != nil {
				t.Fatalf("execute %s: %v", define, err)
			}
			return buf.String()
		}},
	}
}

func (e *listRowEnv) do(t *testing.T, method, target string, form url.Values, hdr map[string]string) string {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	r := httptest.NewRequest(method, target, body)
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	r.Header.Set("HX-Request", "true")
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	if w.Code >= 400 {
		t.Fatalf("%s %s: status %d: %s", method, target, w.Code, w.Body.String())
	}
	return w.Body.String()
}

var (
	spaceRe = regexp.MustCompile(`\s+`)
	flashRe = regexp.MustCompile(` class="flash"`)
)

// rowFor extracts the <li> row whose data-id equals id, normalised for comparison.
func rowFor(t *testing.T, page, id string, stripFlash bool) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<li[^>]*data-id="` + regexp.QuoteMeta(id) + `"[^>]*>.*?</li>`)
	m := re.FindString(page)
	if m == "" {
		t.Fatalf("row for %s not found in:\n%s", id, page)
	}
	if stripFlash {
		m = flashRe.ReplaceAllString(m, "")
	}
	return strings.TrimSpace(spaceRe.ReplaceAllString(m, " "))
}

func TestListRows_InlineAndPartialMatch(t *testing.T) {
	e := newListRowEnv(t)

	box, err := e.inv.CreateContainer("", "Box", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	sub, err := e.inv.CreateContainer(box.ID, "Sub", "Sub description", "", "")
	if err != nil {
		t.Fatal(err)
	}
	item, err := e.inv.CreateItem(box.ID, "Widget", "Widget description", 3, "", "")
	if err != nil {
		t.Fatal(err)
	}

	pageHTML := e.do(t, "GET", "/containers/"+box.ID, nil, nil)

	tests := []struct {
		name    string
		id      string
		partial string
		data    any
		want    []string
	}{
		{"item", item.ID, "item-list-item", item, []string{"Widget", "Widget description", `class="badge">x3<`, "bulk-select", `data-type="item"`, `draggable="true"`}},
		{"container", sub.ID, "container-list-item", sub, []string{"Sub", "Sub description", "bulk-select", `data-type="container"`, `data-drop-id="` + sub.ID + `"`, `data-drop-type="container"`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inline := rowFor(t, pageHTML, tc.id, false)
			if strings.Contains(inline, "flash") {
				t.Errorf("initial render must not carry the flash class: %s", inline)
			}
			partial := rowFor(t, e.templates.render("containers", tc.partial, tc.data), tc.id, false)
			if !strings.Contains(partial, `class="flash"`) {
				t.Errorf("appended row must carry the flash class: %s", partial)
			}
			if got := flashRe.ReplaceAllString(partial, ""); got != inline {
				t.Errorf("partial differs from inline row\n partial: %s\n inline:  %s", got, inline)
			}
			for _, w := range tc.want {
				if !strings.Contains(inline, w) {
					t.Errorf("row missing %q: %s", w, inline)
				}
			}
		})
	}
}

func TestListRows_QuickEntryResponseMatchesReload(t *testing.T) {
	e := newListRowEnv(t)
	box, err := e.inv.CreateContainer("", "Box", "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	resp := e.do(t, "POST", "/items", url.Values{
		"container_id": {box.ID}, "name": {"Bolt"}, "description": {"M4 bolt"}, "quantity": {"5"},
	}, map[string]string{"HX-Target": "item-list"})

	items := e.inv.ContainerItems(box.ID)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	quick := rowFor(t, resp, items[0].ID, true)
	reload := rowFor(t, e.do(t, "GET", "/containers/"+box.ID, nil, nil), items[0].ID, false)
	if quick != reload {
		t.Errorf("quick-entry row differs from reloaded row\n quick:  %s\n reload: %s", quick, reload)
	}
	if !strings.Contains(resp, `class="flash"`) {
		t.Error("quick-entry row should be flagged flash")
	}
}

// Deleting an item with HTMX re-renders the container page; it must be the
// complete page (container header, quick entry prefill), not the root listing.
func TestItemDelete_RendersFullContainerPage(t *testing.T) {
	e := newListRowEnv(t)
	box, err := e.inv.CreateContainer("", "Box", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	item, err := e.inv.CreateItem(box.ID, "Gone", "", 1, "", "")
	if err != nil {
		t.Fatal(err)
	}

	body := e.do(t, "DELETE", "/items/"+item.ID, nil, nil)
	for _, w := range []string{`class="container-detail"`, `data-prefill-container-id="` + box.ID + `"`} {
		if !strings.Contains(body, w) {
			t.Errorf("response missing %q", w)
		}
	}
}
