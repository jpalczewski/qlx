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

func newTestStatsDB(t *testing.T) (*sqlite.SQLiteStore, *service.InventoryService, *service.TagService, *StatsHandler) {
	t.Helper()
	db, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	inv := service.NewInventoryService(db)
	tags := service.NewTagService(db)
	return db, inv, tags, NewStatsHandler(inv, tags, &JSONResponder{})
}

// seedStatsData creates 2 containers, 2 items (qty 5+3), and 1 tag assigned to root1.
func seedStatsData(t *testing.T, inv *service.InventoryService, tags *service.TagService) {
	t.Helper()
	root1, err := inv.CreateContainer("", "Root1", "", "red", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = inv.CreateContainer(root1.ID, "Child1", "", "blue", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = inv.CreateItem(root1.ID, "Item1", "", 5, "red", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = inv.CreateItem(root1.ID, "Item2", "", 3, "blue", ""); err != nil {
		t.Fatal(err)
	}
	tag, err := tags.CreateTag("", "Electronics", "blue", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := tags.AddContainerTag(root1.ID, tag.ID); err != nil {
		t.Fatal(err)
	}
}

func getStatsVM(t *testing.T, h *StatsHandler) StatsViewModel {
	t.Helper()
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
	return vm
}

func TestStatsHandler_Page_EmptyStore(t *testing.T) {
	h := newTestStatsHandler(t)
	vm := getStatsVM(t, h)

	if vm.Containers != 0 {
		t.Errorf("expected 0 containers, got %d", vm.Containers)
	}
	if vm.Items != 0 {
		t.Errorf("expected 0 items, got %d", vm.Items)
	}
	if vm.TotalQty != 0 {
		t.Errorf("expected 0 total qty, got %d", vm.TotalQty)
	}
	if vm.RootContainers != 0 {
		t.Errorf("expected 0 root containers, got %d", vm.RootContainers)
	}
	if len(vm.Tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(vm.Tags))
	}
}

func TestStatsHandler_Page_WithData(t *testing.T) {
	_, inv, tags, h := newTestStatsDB(t)
	seedStatsData(t, inv, tags)
	vm := getStatsVM(t, h)

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
	_, inv, tagSvc, h := newTestStatsDB(t)

	// Create two tags; tagB gets more uses than tagA
	tagA, err := tagSvc.CreateTag("", "TagA", "", "")
	if err != nil {
		t.Fatal(err)
	}
	tagB, err := tagSvc.CreateTag("", "TagB", "", "")
	if err != nil {
		t.Fatal(err)
	}

	c, err := inv.CreateContainer("", "C1", "", "", "")
	if err != nil {
		t.Fatal(err)
	}

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

	vm := getStatsVM(t, h)

	if len(vm.Tags) < 2 {
		t.Fatalf("expected at least 2 tag stats, got %d", len(vm.Tags))
	}
	if vm.Tags[0].Name != "TagB" {
		t.Errorf("expected TagB first (more uses), got %s", vm.Tags[0].Name)
	}
}
