package sqlite

import (
	"testing"
)

func TestPrinterStore_CRUD(t *testing.T) {
	db := testStore(t)

	p := db.AddPrinter("Brother QL-700", "brother", "ql700", "usb", "")
	if p == nil {
		t.Fatal("expected printer, got nil")
	}
	if p.Name != "Brother QL-700" {
		t.Errorf("got %q", p.Name)
	}

	got := db.GetPrinter(p.ID)
	if got == nil || got.Name != "Brother QL-700" {
		t.Fatal("GetPrinter failed")
	}

	all := db.AllPrinters()
	if len(all) != 1 {
		t.Errorf("got %d printers, want 1", len(all))
	}

	if err := db.UpdatePrinterOffset(p.ID, 5, -3); err != nil {
		t.Fatal(err)
	}
	updated := db.GetPrinter(p.ID)
	if updated.OffsetX != 5 || updated.OffsetY != -3 {
		t.Errorf("offsets not updated: x=%d y=%d", updated.OffsetX, updated.OffsetY)
	}

	if err := db.DeletePrinter(p.ID); err != nil {
		t.Fatal(err)
	}
	if db.GetPrinter(p.ID) != nil {
		t.Error("expected nil after delete")
	}
}

func TestPrinterStore_DeleteNotFound(t *testing.T) {
	db := testStore(t)
	err := db.DeletePrinter("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}
