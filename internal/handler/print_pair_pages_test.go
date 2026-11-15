package handler

import (
	"net/http"
	"testing"

	"github.com/erxyi/qlx/internal/print/label"
)

func TestPrint_PairPagesFirstReachesRenderOpts(t *testing.T) {
	fake := &fakePrintDeps{}
	_, inv, mux := newFakePrintHandler(t, fake)
	c, _ := inv.CreateContainer("", "Box", "", "", "")
	item, _ := inv.CreateItem(c.ID, "Widget", "", 1, "", "")

	cases := []struct {
		name      string
		path      string
		body      map[string]any
		wantFirst bool
	}{
		{"item first", "/items/" + item.ID + "/print", map[string]any{"template": "pair", "pair_pages": "first", "copies": 2, "cut_every": 1}, true},
		{"item both", "/items/" + item.ID + "/print", map[string]any{"template": "pair", "pair_pages": "both"}, false},
		{"item default", "/items/" + item.ID + "/print", map[string]any{"template": "pair"}, false},
		{"container first", "/containers/" + c.ID + "/print", map[string]any{"templates": []string{"pair"}, "pair_pages": "first"}, true},
		{"container default", "/containers/" + c.ID + "/print", map[string]any{"templates": []string{"pair"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake.calls = nil
			w := postJSON(mux, tc.path, tc.body)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
			}
			if len(fake.calls) != 1 || fake.calls[0].template != label.PairTemplate {
				t.Fatalf("calls = %+v, want one pair print", fake.calls)
			}
			if got := fake.calls[0].opts.FirstPageOnly; got != tc.wantFirst {
				t.Errorf("FirstPageOnly = %v, want %v", got, tc.wantFirst)
			}
		})
	}

	fake.calls = nil
	postJSON(mux, "/items/"+item.ID+"/print", map[string]any{"template": "pair", "pair_pages": "first", "copies": 2, "cut_every": 1})
	if po := fake.calls[0].printOpt; po.Copies != 2 || po.CutEvery != 1 {
		t.Errorf("print opts = %+v, want copies and cut_every passed through unchanged", po)
	}
}

func TestPrint_InvalidPairPagesIsRejected(t *testing.T) {
	fake := &fakePrintDeps{}
	_, inv, mux := newFakePrintHandler(t, fake)
	c, _ := inv.CreateContainer("", "Box", "", "", "")
	item, _ := inv.CreateItem(c.ID, "Widget", "", 1, "", "")

	for _, path := range []string{"/items/" + item.ID + "/print", "/containers/" + c.ID + "/print"} {
		w := postJSON(mux, path, map[string]any{"template": "pair", "templates": []string{"pair"}, "pair_pages": "second"})
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", path, w.Code)
		}
	}
	if len(fake.calls) != 0 {
		t.Errorf("calls = %d, want nothing printed", len(fake.calls))
	}
}
