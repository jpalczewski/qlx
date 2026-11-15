package service

import (
	"reflect"
	"testing"
	"time"

	"github.com/erxyi/qlx/internal/store/sqlite"
)

func newWorkingSetFixture(t *testing.T) (*WorkingSetService, *sqlite.SQLiteStore) {
	t.Helper()
	db, err := sqlite.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewWorkingSetService(db), db
}

func TestWorkingSetService_Snapshot(t *testing.T) {
	svc, db := newWorkingSetFixture(t)
	pinned := db.CreateContainer("", "Pinned", "", "", "")
	r1 := db.CreateContainer("", "Recent1", "", "", "")
	r2 := db.CreateContainer("", "Recent2", "", "", "")
	db.CreateItem(pinned.ID, "x", "", 1, "", "")
	db.CreateItem(r1.ID, "y", "", 1, "", "")
	db.CreateItem(r1.ID, "z", "", 1, "", "")

	if err := svc.Pin(pinned.ID); err != nil {
		t.Fatal(err)
	}
	base := time.UnixMilli(1_700_000_000_000)
	for i, id := range []string{pinned.ID, r1.ID, r2.ID} {
		if err := db.RecordContainerOpen(id, base.Add(time.Duration(i)*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
	}

	snap := svc.Snapshot()
	if len(snap.Pins) != 1 || snap.Pins[0].Container.ID != pinned.ID || snap.Pins[0].ItemCount != 1 || !snap.Pins[0].Pinned {
		t.Errorf("pins = %+v", snap.Pins)
	}
	if len(snap.Recents) != 2 || snap.Recents[0].Container.ID != r2.ID || snap.Recents[1].Container.ID != r1.ID {
		t.Fatalf("recents = %+v", snap.Recents)
	}
	if snap.Recents[1].ItemCount != 2 || snap.Recents[1].OpenCount != 1 {
		t.Errorf("recent meta = %+v", snap.Recents[1])
	}
}

func TestWorkingSetService_IsPinnedCache(t *testing.T) {
	svc, db := newWorkingSetFixture(t)
	c := db.CreateContainer("", "C", "", "", "")

	if svc.IsPinned(c.ID) {
		t.Fatal("should not be pinned")
	}
	if err := svc.Pin(c.ID); err != nil {
		t.Fatal(err)
	}
	if !svc.IsPinned(c.ID) {
		t.Error("should be pinned after Pin")
	}
	if err := svc.Unpin(c.ID); err != nil {
		t.Fatal(err)
	}
	if svc.IsPinned(c.ID) {
		t.Error("should not be pinned after Unpin")
	}
}

func TestNormalizeSearch(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Żółty", "zolty"},
		{"  ZAŻÓŁĆ Gęślą Jaźń ", "zazolc gesla jazn"},
		{"Café", "cafe"},
		{"plain", "plain"},
	}
	for _, tt := range tests {
		if got := normalizeSearch(tt.in); got != tt.want {
			t.Errorf("normalizeSearch(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func quickNames(rs []QuickSwitchResult) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Container.Name)
	}
	return out
}

func TestWorkingSetService_QuickSwitchRanking(t *testing.T) {
	svc, db := newWorkingSetFixture(t)
	garage := db.CreateContainer("", "Garaż", "", "", "")
	shelf := db.CreateContainer(garage.ID, "Półka żółta", "", "", "")
	db.CreateContainer("", "Żółty", "", "", "")
	db.CreateContainer("", "Żółty karton", "", "", "")
	db.CreateContainer("", "Karton z żółtym", "", "", "")
	db.CreateContainer(shelf.ID, "Śruby", "", "", "")

	tests := []struct {
		name  string
		q     string
		limit int
		want  []string
	}{
		{"exact beats prefix beats substring", "zolty", 10, []string{"Żółty", "Żółty karton", "Karton z żółtym"}},
		{"polish input matches", "ŻÓŁTY", 10, []string{"Żółty", "Żółty karton", "Karton z żółtym"}},
		{"word prefix beats substring", "kart", 10, []string{"Karton z żółtym", "Żółty karton"}},
		{"path match ranks after name matches", "garaz", 10, []string{"Garaż", "Śruby", "Półka żółta"}},
		{"name beats path-only", "polka", 10, []string{"Półka żółta", "Śruby"}},
		{"limit", "zolty", 1, []string{"Żółty"}},
		{"no match", "qqq", 10, nil},
		{"empty query without working set", "", 10, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quickNames(svc.QuickSwitch(tt.q, tt.limit)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("QuickSwitch(%q) = %v, want %v", tt.q, got, tt.want)
			}
		})
	}
}

func TestWorkingSetService_QuickSwitchBoosts(t *testing.T) {
	svc, db := newWorkingSetFixture(t)
	prefix := db.CreateContainer("", "Żółty", "", "", "")
	substr := db.CreateContainer("", "Karton z żółtym", "", "", "")

	if got := quickNames(svc.QuickSwitch("zolt", 10)); got[0] != "Żółty" {
		t.Fatalf("baseline = %v", got)
	}

	// A pin lifts the substring match above the prefix match.
	if err := svc.Pin(substr.ID); err != nil {
		t.Fatal(err)
	}
	if got := quickNames(svc.QuickSwitch("zolt", 10)); got[0] != "Karton z żółtym" {
		t.Errorf("pinned should rank first, got %v", got)
	}

	// Empty query lists the working set: pins first, then recents.
	if err := db.RecordContainerOpen(prefix.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := quickNames(svc.QuickSwitch("", 10)); !reflect.DeepEqual(got, []string{"Karton z żółtym", "Żółty"}) {
		t.Errorf("empty query = %v", got)
	}

	// Recents are boosted too, but less than pins.
	if err := svc.Unpin(substr.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordContainerOpen(substr.ID, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	got := svc.QuickSwitch("zolt", 10)
	if len(got) != 2 || got[0].Container.Name != "Żółty" {
		t.Errorf("recent boost should not beat a prefix match: %v", quickNames(got))
	}
}
