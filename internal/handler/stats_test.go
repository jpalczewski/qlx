package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/store/sqlite"
)

func newTestStatsHandler(t *testing.T) *StatsHandler {
	t.Helper()
	db, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	inv := service.NewInventoryService(db)
	tags := service.NewTagService(db)
	return NewStatsHandler(inv, tags, &JSONResponder{})
}

func TestStatsHandler_Page_EmptyStore(t *testing.T) {
	h := newTestStatsHandler(t)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stats", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var vm StatsViewModel
	if err := json.NewDecoder(w.Body).Decode(&vm); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if vm.Containers != 0 {
		t.Errorf("expected 0 containers, got %d", vm.Containers)
	}
	if vm.Items != 0 {
		t.Errorf("expected 0 items, got %d", vm.Items)
	}
	if vm.TotalQty != 0 {
		t.Errorf("expected 0 total qty, got %d", vm.TotalQty)
	}
}

func TestStatsHandler_Page_WithData(t *testing.T) {
	db, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	inv := service.NewInventoryService(db)
	tags := service.NewTagService(db)
	h := NewStatsHandler(inv, tags, &JSONResponder{})

	// Create containers
	root1, err := inv.CreateContainer("", "Root1", "", "red", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = inv.CreateContainer(root1.ID, "Child1", "", "blue", "")
	if err != nil {
		t.Fatal(err)
	}

	// Create items
	_, err = inv.CreateItem(root1.ID, "Item1", "", 5, "red", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = inv.CreateItem(root1.ID, "Item2", "", 3, "blue", "")
	if err != nil {
		t.Fatal(err)
	}

	// Create a tag and assign it
	tag, err := tags.CreateTag("", "Electronics", "blue", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := tags.AddContainerTag(root1.ID, tag.ID); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stats", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var vm StatsViewModel
	if err := json.NewDecoder(w.Body).Decode(&vm); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if vm.Containers != 2 {
		t.Errorf("expected 2 containers, got %d", vm.Containers)
	}
	if vm.RootContainers != 1 {
		t.Errorf("expected 1 root container, got %d", vm.RootContainers)
	}
	if vm.Items != 2 {
		t.Errorf("expected 2 items, got %d", vm.Items)
	}
	if vm.TotalQty != 8 {
		t.Errorf("expected total qty 8, got %d", vm.TotalQty)
	}
	if len(vm.Tags) != 1 {
		t.Fatalf("expected 1 tag stat, got %d", len(vm.Tags))
	}
	if vm.Tags[0].Name != "Electronics" {
		t.Errorf("expected tag name Electronics, got %s", vm.Tags[0].Name)
	}
	if vm.Tags[0].ContainerCount != 1 {
		t.Errorf("expected container count 1, got %d", vm.Tags[0].ContainerCount)
	}
	if vm.Tags[0].TotalUses != 1 {
		t.Errorf("expected total uses 1, got %d", vm.Tags[0].TotalUses)
	}
}

func TestStatsHandler_Page_SortsByTotalUsesDesc(t *testing.T) {
	db, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	inv := service.NewInventoryService(db)
	tagSvc := service.NewTagService(db)
	h := NewStatsHandler(inv, tagSvc, &JSONResponder{})

	// Create two tags; tagB gets more uses than tagA
	tagA, _ := tagSvc.CreateTag("", "TagA", "", "")
	tagB, _ := tagSvc.CreateTag("", "TagB", "", "")

	c, _ := inv.CreateContainer("", "C1", "", "", "")

	// TagA: 1 container; TagB: 2 containers
	if err := tagSvc.AddContainerTag(c.ID, tagA.ID); err != nil {
		t.Fatal(err)
	}

	c2, err := inv.CreateContainer("", "C3", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	c3, err := inv.CreateContainer("", "C4", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := tagSvc.AddContainerTag(c2.ID, tagB.ID); err != nil {
		t.Fatal(err)
	}
	if err := tagSvc.AddContainerTag(c3.ID, tagB.ID); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stats", nil)
	mux.ServeHTTP(w, r)

	var vm StatsViewModel
	json.NewDecoder(w.Body).Decode(&vm)

	if len(vm.Tags) < 2 {
		t.Fatalf("expected at least 2 tag stats, got %d", len(vm.Tags))
	}
	if vm.Tags[0].Name != "TagB" {
		t.Errorf("expected TagB first (more uses), got %s", vm.Tags[0].Name)
	}
}
