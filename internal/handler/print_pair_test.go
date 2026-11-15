package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/store"
)

// printCall is one recorded Print invocation.
type printCall struct {
	template string
	data     label.LabelData
	opts     label.RenderOpts
	printOpt encoder.PrintOpts
}

// fakePrintDeps implements printHandlerPrinterDeps and records Print calls.
type fakePrintDeps struct {
	mu    sync.Mutex
	calls []printCall
	// errFor returns the error Print gives for a template (nil = success).
	errFor func(template string) error
}

func (f *fakePrintDeps) Print(_ string, data label.LabelData, tmpl string, ro label.RenderOpts, po encoder.PrintOpts) error {
	f.mu.Lock()
	f.calls = append(f.calls, printCall{template: tmpl, data: data, opts: ro, printOpt: po})
	f.mu.Unlock()
	if f.errFor != nil {
		return f.errFor(tmpl)
	}
	return nil
}
func (f *fakePrintDeps) PrintImage(string, image.Image, encoder.PrintOpts) error { return nil }
func (f *fakePrintDeps) ConnectedPrinters() []store.PrinterConfig                { return nil }
func (f *fakePrintDeps) AvailableEncoders() map[string]encoder.Encoder           { return nil }
func (f *fakePrintDeps) Encoder(string) encoder.Encoder                          { return nil }
func (f *fakePrintDeps) GetStatus(string) print.PrinterStatus                    { return print.PrinterStatus{} }
func (f *fakePrintDeps) AllStatuses() map[string]print.PrinterStatus             { return nil }

func newFakePrintHandler(t *testing.T, fake *fakePrintDeps) (*PrintHandler, *service.InventoryService, *http.ServeMux) {
	t.Helper()
	s := newHandlerTestStore(t)
	inv := service.NewInventoryService(s)
	h := NewPrintHandler(fake, nil, inv, service.NewPrinterService(s), service.NewTemplateService(s),
		service.NewTagService(s), service.NewNoteService(s), &JSONResponder{})
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return h, inv, mux
}

func postJSON(mux *http.ServeMux, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return out
}

func TestPrintContainer_PairTemplateIsPrintedOnServer(t *testing.T) {
	fake := &fakePrintDeps{}
	_, inv, mux := newFakePrintHandler(t, fake)
	parent, _ := inv.CreateContainer("", "Room", "", "", "")
	c, _ := inv.CreateContainer(parent.ID, "Shelf", "synthetic shelf", "", "")
	sub, _ := inv.CreateContainer(c.ID, "Drawer", "", "", "")
	if _, err := inv.CreateItem(c.ID, "Widget", "", 1, "", ""); err != nil {
		t.Fatal(err)
	}

	w := postJSON(mux, "/containers/"+c.ID+"/print", map[string]any{
		"printer_id": "p1", "templates": []string{"pair"}, "show_children": true,
		"print_description": true, "print_date": true, "copies": 3, "cut_every": 1,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	if body := decodeBody(t, w); body["ok"] != true || body["warning"] != nil {
		t.Errorf("body = %v, want ok without warning", body)
	}
	if len(fake.calls) != 1 || fake.calls[0].template != label.PairTemplate {
		t.Fatalf("calls = %+v, want one pair print (a pair is not a designer template)", fake.calls)
	}
	call := fake.calls[0]
	if !call.opts.PrintDescription || !call.opts.PrintDate {
		t.Errorf("render opts = %+v, want description and date", call.opts)
	}
	if call.printOpt.Copies != 3 || call.printOpt.CutEvery != 1 {
		t.Errorf("print opts = %+v, want 3 pairs cut after each", call.printOpt)
	}
	if got := call.data.LocationPath; len(got) != 1 || got[0] != "Room" {
		t.Errorf("LocationPath = %v, want [Room]", got)
	}
	if call.data.Location != "Room" {
		t.Errorf("Location = %q", call.data.Location)
	}

	assertChildKinds(t, call.data.Children, sub.Name, "Widget")
}

// assertChildKinds checks that the container child is marked IsContainer and
// the item child is not.
func assertChildKinds(t *testing.T, children []label.LabelChild, container, item string) {
	t.Helper()
	var gotContainer, gotItem bool
	for _, ch := range children {
		switch ch.Name {
		case container:
			gotContainer = ch.IsContainer
		case item:
			gotItem = !ch.IsContainer
		}
	}
	if !gotContainer || !gotItem {
		t.Errorf("children = %+v, want %q as container and %q as item", children, container, item)
	}
}

func TestPrintItem_LocationUsesSharedSeparatorAndPath(t *testing.T) {
	fake := &fakePrintDeps{}
	_, inv, mux := newFakePrintHandler(t, fake)
	a, _ := inv.CreateContainer("", "Room", "", "", "")
	b, _ := inv.CreateContainer(a.ID, "Shelf", "", "", "")
	item, _ := inv.CreateItem(b.ID, "Widget", "synthetic", 1, "", "")

	w := postJSON(mux, "/items/"+item.ID+"/print", map[string]any{"printer_id": "p1", "template": "pair"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	d := fake.calls[0].data
	if d.Location != "Room"+label.LocationSeparator+"Shelf" {
		t.Errorf("Location = %q", d.Location)
	}
	if len(d.LocationPath) != 2 || d.LocationPath[0] != "Room" || d.LocationPath[1] != "Shelf" {
		t.Errorf("LocationPath = %v", d.LocationPath)
	}
	if fake.calls[0].opts.PrintDescription {
		t.Error("description must be off unless requested")
	}
}

func TestPrintItem_PairFallsBackToSingleWithWarning(t *testing.T) {
	fake := &fakePrintDeps{errFor: func(tmpl string) error {
		if tmpl == label.PairTemplate {
			return &print.RenderError{Err: errors.New("unsupported media")}
		}
		return nil
	}}
	_, inv, mux := newFakePrintHandler(t, fake)
	c, _ := inv.CreateContainer("", "Box", "", "", "")
	item, _ := inv.CreateItem(c.ID, "Widget", "", 1, "", "")

	w := postJSON(mux, "/items/"+item.ID+"/print", map[string]any{"printer_id": "p1", "template": "pair", "copies": 2})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (never a 500 for an unsupported pair), body %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["ok"] != true || body["warning"] != "pair_fallback" || body["message"] == "" {
		t.Errorf("body = %v, want ok with pair_fallback warning", body)
	}
	if len(fake.calls) != 2 || fake.calls[1].template != "simple" || fake.calls[1].printOpt.Copies != 2 {
		t.Errorf("calls = %+v, want pair attempt then single with the same options", fake.calls)
	}
}

func TestPrintItem_PrinterErrorIsNotRetriedAsSingle(t *testing.T) {
	fake := &fakePrintDeps{errFor: func(string) error { return errors.New("printer offline") }}
	_, inv, mux := newFakePrintHandler(t, fake)
	c, _ := inv.CreateContainer("", "Box", "", "", "")
	item, _ := inv.CreateItem(c.ID, "Widget", "", 1, "", "")

	w := postJSON(mux, "/items/"+item.ID+"/print", map[string]any{"printer_id": "p1", "template": "pair"})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 for a real print failure", w.Code)
	}
	if len(fake.calls) != 1 {
		t.Errorf("calls = %d, want 1: a transport failure must not print a second label", len(fake.calls))
	}
}

func TestPrintItem_SingleTemplateUnchanged(t *testing.T) {
	fake := &fakePrintDeps{}
	_, inv, mux := newFakePrintHandler(t, fake)
	c, _ := inv.CreateContainer("", "Box", "", "", "")
	item, _ := inv.CreateItem(c.ID, "Widget", "", 1, "", "")

	w := postJSON(mux, "/items/"+item.ID+"/print", map[string]any{"printer_id": "p1", "template": "simple", "copies": 2})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if len(fake.calls) != 1 || fake.calls[0].template != "simple" || fake.calls[0].printOpt.Copies != 2 {
		t.Errorf("calls = %+v", fake.calls)
	}
}

// previewSize fetches a preview and returns its PNG dimensions and headers.
func previewSize(t *testing.T, mux *http.ServeMux, url string) (image.Point, http.Header) {
	t.Helper()
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d, body %s", url, w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("GET %s: content type %q", url, ct)
	}
	img, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("GET %s: decode: %v", url, err)
	}
	return img.Bounds().Size(), w.Header()
}

func TestPreview_PairPages(t *testing.T) {
	_, inv, mux := newFakePrintHandler(t, &fakePrintDeps{})
	c, _ := inv.CreateContainer("", "Box", "", "", "")
	item, _ := inv.CreateItem(c.ID, "Widget", "synthetic", 1, "", "")
	base := "/items/" + item.ID + "/preview?template=pair&width=696"

	p1, h1 := previewSize(t, mux, base+"&page=1")
	p2, _ := previewSize(t, mux, base+"&page=2")
	all, hAll := previewSize(t, mux, base+"&page=all")
	def, _ := previewSize(t, mux, base)

	if p1 != p2 || p1.X != 696 {
		t.Errorf("page sizes = %v and %v, want equal and 696 wide", p1, p2)
	}
	if all.X != p1.X || all.Y != p1.Y+p2.Y+pairPreviewGap {
		t.Errorf("all = %v, want both pages stacked: %dx%d", all, p1.X, p1.Y+p2.Y+pairPreviewGap)
	}
	if def != all {
		t.Errorf("default page = %v, want all (%v)", def, all)
	}
	if h1.Get("X-Qlx-Pages") != "2" || hAll.Get("X-Qlx-Warning") != "" {
		t.Errorf("headers = %v", h1)
	}
}

func TestPreview_PairDescriptionOption(t *testing.T) {
	_, inv, mux := newFakePrintHandler(t, &fakePrintDeps{})
	c, _ := inv.CreateContainer("", "Box", "", "", "")
	item, _ := inv.CreateItem(c.ID, "Widget", "a fairly long synthetic description of this widget", 1, "", "")
	url := "/items/" + item.ID + "/preview?template=pair&width=696&page=1"

	fetch := func(u string) []byte {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", u, nil))
		return w.Body.Bytes()
	}
	if bytes.Equal(fetch(url), fetch(url+"&print_description=true")) {
		t.Error("print_description=true must change the identity page")
	}
}

func TestPreview_PairContainerWithChildren(t *testing.T) {
	_, inv, mux := newFakePrintHandler(t, &fakePrintDeps{})
	c, _ := inv.CreateContainer("", "Box", "", "", "")
	_, _ = inv.CreateContainer(c.ID, "Inner", "", "", "")
	_, _ = inv.CreateItem(c.ID, "Widget", "", 1, "", "")

	if sz, _ := previewSize(t, mux, "/containers/"+c.ID+"/preview?template=pair&show_children=true&page=2"); sz.X == 0 {
		t.Error("empty preview")
	}
}

func TestPreview_InvalidPage(t *testing.T) {
	_, inv, mux := newFakePrintHandler(t, &fakePrintDeps{})
	c, _ := inv.CreateContainer("", "Box", "", "", "")
	item, _ := inv.CreateItem(c.ID, "Widget", "", 1, "", "")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/items/"+item.ID+"/preview?template=pair&page=3", nil))
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestPreview_SingleTemplateIgnoresPage(t *testing.T) {
	_, inv, mux := newFakePrintHandler(t, &fakePrintDeps{})
	c, _ := inv.CreateContainer("", "Box", "", "", "")
	item, _ := inv.CreateItem(c.ID, "Widget", "", 1, "", "")

	a, _ := previewSize(t, mux, "/items/"+item.ID+"/preview?template=simple&width=696")
	b, h := previewSize(t, mux, "/items/"+item.ID+"/preview?template=simple&width=696&page=2")
	if a != b || h.Get("X-Qlx-Pages") != "1" {
		t.Errorf("single template: %v vs %v, pages header %q", a, b, h.Get("X-Qlx-Pages"))
	}
}

func TestPreview_PairFallsBackWithWarning(t *testing.T) {
	_, inv, mux := newFakePrintHandler(t, &fakePrintDeps{})
	c, _ := inv.CreateContainer("", "Box", "", "", "")
	item, _ := inv.CreateItem(c.ID, "Widget", "", 1, "", "")

	// A canvas this narrow cannot hold the pair layout.
	_, h := previewSize(t, mux, "/items/"+item.ID+"/preview?template=pair&width=8")
	if h.Get("X-Qlx-Warning") != "pair_fallback" || h.Get("X-Qlx-Pages") != "1" {
		t.Errorf("headers = %v, want pair_fallback and a single page", h)
	}
}
