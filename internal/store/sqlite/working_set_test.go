package sqlite

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/erxyi/qlx/internal/store"
)

func TestWorkingSet_PinUnpinIdempotent(t *testing.T) {
	db := testStore(t)
	a := db.CreateContainer("", "A", "", "", "")
	b := db.CreateContainer("", "B", "", "", "")

	tests := []struct {
		name string
		do   func() error
		want []string
	}{
		{"pin a", func() error { return db.PinContainer(a.ID) }, []string{a.ID}},
		{"pin a again", func() error { return db.PinContainer(a.ID) }, []string{a.ID}},
		{"pin b", func() error { return db.PinContainer(b.ID) }, []string{a.ID, b.ID}},
		{"unpin a", func() error { return db.UnpinContainer(a.ID) }, []string{b.ID}},
		{"unpin a again", func() error { return db.UnpinContainer(a.ID) }, []string{b.ID}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.do(); err != nil {
				t.Fatal(err)
			}
			if got := db.Pins(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Pins() = %v, want %v", got, tt.want)
			}
		})
	}

	if err := db.PinContainer("missing"); !errors.Is(err, store.ErrContainerNotFound) {
		t.Errorf("pin missing: got %v, want ErrContainerNotFound", err)
	}
}

func TestWorkingSet_ReorderPins(t *testing.T) {
	db := testStore(t)
	var ids []string
	for _, n := range []string{"A", "B", "C"} {
		c := db.CreateContainer("", n, "", "", "")
		ids = append(ids, c.ID)
		if err := db.PinContainer(c.ID); err != nil {
			t.Fatal(err)
		}
	}
	a, b, c := ids[0], ids[1], ids[2]

	tests := []struct {
		name  string
		order []string
		want  []string
	}{
		{"reverse", []string{c, b, a}, []string{c, b, a}},
		{"partial keeps rest after", []string{a}, []string{a, c, b}},
		{"unknown and duplicates ignored", []string{"nope", b, b, c}, []string{b, c, a}},
		{"empty keeps order", nil, []string{b, c, a}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := db.ReorderPins(tt.order); err != nil {
				t.Fatal(err)
			}
			if got := db.Pins(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Pins() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkingSet_RecentsOrderAndCount(t *testing.T) {
	db := testStore(t)
	a := db.CreateContainer("", "A", "", "", "")
	b := db.CreateContainer("", "B", "", "", "")
	base := time.UnixMilli(1_700_000_000_000)

	// Millisecond differences must order correctly.
	steps := []struct {
		id string
		at time.Time
	}{
		{a.ID, base},
		{b.ID, base.Add(1 * time.Millisecond)},
		{a.ID, base.Add(2 * time.Millisecond)},
	}
	for _, s := range steps {
		if err := db.RecordContainerOpen(s.id, s.at); err != nil {
			t.Fatal(err)
		}
	}
	got := db.RecentContainers(10)
	if len(got) != 2 || got[0].ContainerID != a.ID || got[1].ContainerID != b.ID {
		t.Fatalf("unexpected order: %+v", got)
	}
	if got[0].OpenCount != 2 || got[1].OpenCount != 1 {
		t.Errorf("open counts = %d,%d want 2,1", got[0].OpenCount, got[1].OpenCount)
	}
	if !got[0].LastOpenedAt.Equal(base.Add(2 * time.Millisecond)) {
		t.Errorf("LastOpenedAt = %v", got[0].LastOpenedAt)
	}
	if n := len(db.RecentContainers(1)); n != 1 {
		t.Errorf("limit 1 returned %d", n)
	}
	if err := db.RecordContainerOpen("missing", base); !errors.Is(err, store.ErrContainerNotFound) {
		t.Errorf("got %v, want ErrContainerNotFound", err)
	}
}

func TestWorkingSet_RecentsTrimmedAt50(t *testing.T) {
	db := testStore(t)
	base := time.UnixMilli(1_700_000_000_000)
	var ids []string
	for i := 0; i < store.MaxRecentContainers+5; i++ {
		c := db.CreateContainer("", fmt.Sprintf("C%d", i), "", "", "")
		ids = append(ids, c.ID)
		if err := db.RecordContainerOpen(c.ID, base.Add(time.Duration(i)*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
	}
	got := db.RecentContainers(1000)
	if len(got) != store.MaxRecentContainers {
		t.Fatalf("len = %d, want %d", len(got), store.MaxRecentContainers)
	}
	if got[0].ContainerID != ids[len(ids)-1] {
		t.Errorf("newest should be first")
	}
	if got[len(got)-1].ContainerID != ids[5] {
		t.Errorf("oldest kept should be index 5, got %s", got[len(got)-1].ContainerID)
	}
}

func TestWorkingSet_CascadeOnContainerDelete(t *testing.T) {
	db := testStore(t)
	c := db.CreateContainer("", "Gone", "", "", "")
	keep := db.CreateContainer("", "Keep", "", "", "")
	for _, id := range []string{c.ID, keep.ID} {
		if err := db.PinContainer(id); err != nil {
			t.Fatal(err)
		}
		if err := db.RecordContainerOpen(id, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.DeleteContainer(c.ID); err != nil {
		t.Fatal(err)
	}
	if got := db.Pins(); !reflect.DeepEqual(got, []string{keep.ID}) {
		t.Errorf("Pins() = %v", got)
	}
	r := db.RecentContainers(10)
	if len(r) != 1 || r[0].ContainerID != keep.ID {
		t.Errorf("recents = %+v", r)
	}
}

func TestWorkingSet_ContainerItemCounts(t *testing.T) {
	db := testStore(t)
	a := db.CreateContainer("", "A", "", "", "")
	b := db.CreateContainer("", "B", "", "", "")
	db.CreateItem(a.ID, "i1", "", 1, "", "")
	db.CreateItem(a.ID, "i2", "", 1, "", "")

	got := db.ContainerItemCounts([]string{a.ID, b.ID})
	if got[a.ID] != 2 || got[b.ID] != 0 {
		t.Errorf("counts = %v", got)
	}
	if got := db.ContainerItemCounts(nil); len(got) != 0 {
		t.Errorf("nil ids: %v", got)
	}
}

func TestWorkingSet_SameMillisecondOpensKeepCallOrder(t *testing.T) {
	db := testStore(t)
	at := time.UnixMilli(1_700_000_000_000)
	var ids []string
	for i := 0; i < 5; i++ {
		c := db.CreateContainer("", fmt.Sprintf("C%d", i), "", "", "")
		ids = append(ids, c.ID)
		if err := db.RecordContainerOpen(c.ID, at); err != nil {
			t.Fatal(err)
		}
	}
	// Re-open the first one in the same millisecond: it becomes the newest.
	if err := db.RecordContainerOpen(ids[0], at); err != nil {
		t.Fatal(err)
	}
	want := []string{ids[0], ids[4], ids[3], ids[2], ids[1]}
	var got []string
	for _, r := range db.RecentContainers(10) {
		got = append(got, r.ContainerID)
		if r.ContainerID == ids[0] && r.OpenCount != 2 {
			t.Errorf("open_count = %d, want 2", r.OpenCount)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}
