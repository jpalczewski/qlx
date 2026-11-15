package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erxyi/qlx/internal/service"
)

func TestSearchHandler_SearchItems(t *testing.T) {
	s := newHandlerTestStore(t)
	inv := service.NewInventoryService(s)
	shelf, _ := inv.CreateContainer("", "Shelf", "", "", "")
	box, _ := inv.CreateContainer(shelf.ID, "Blue Box", "", "", "")
	for _, n := range []string{"Resistor 10k", "Resistor 22k", "Capacitor"} {
		if _, err := inv.CreateItem(box.ID, n, "", 3, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	NewSearchHandler(service.NewSearchService(s), &JSONResponder{}).RegisterRoutes(mux)

	tests := []struct {
		name, query string
		want        int
	}{
		{"match", "?q=resistor", 2},
		{"limit", "?q=resistor&limit=1", 1},
		{"no match", "?q=zzz", 0},
		{"empty query", "?q=", 0},
		{"no query", "", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/search/items"+tc.query, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status %d", w.Code)
			}
			var got []itemSearchJSON
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Fatalf("got %d hits, want %d", len(got), tc.want)
			}
			for _, h := range got {
				if h.ContainerID != box.ID || h.ContainerName != "Blue Box" || h.Path != "Shelf / Blue Box" || h.Quantity != 3 {
					t.Errorf("bad hit %+v", h)
				}
			}
		})
	}
}
