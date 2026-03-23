package sqlite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateJSON_Partitioned(t *testing.T) {
	// Copy test fixtures to a temp dir
	dataDir := t.TempDir()
	fixtureDir := filepath.Join("testdata", "partitioned")

	for _, name := range []string{"containers.json", "items.json", "tags.json", "printers.json", "templates.json"} {
		src := filepath.Join(fixtureDir, name)
		dst := filepath.Join(dataDir, name)
		data, err := os.ReadFile(src) //nolint:gosec // G304: test fixture path
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		if err := os.WriteFile(dst, data, 0600); err != nil { //nolint:gosec // G306: test temp file
			t.Fatal(err)
		}
	}

	// Open the store — migration should run automatically
	db, err := New(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Verify data was imported
	containers := db.AllContainers()
	if len(containers) != 2 {
		t.Errorf("got %d containers, want 2", len(containers))
	}

	items := db.AllItems()
	if len(items) != 2 {
		t.Errorf("got %d items, want 2", len(items))
	}

	all := db.AllTags()
	if len(all) != 1 {
		t.Errorf("got %d tags, want 1", len(all))
	}

	printers := db.AllPrinters()
	if len(printers) != 1 {
		t.Errorf("got %d printers, want 1", len(printers))
	}

	templates := db.AllTemplates()
	if len(templates) != 1 {
		t.Errorf("got %d templates, want 1", len(templates))
	}

	// Verify junction tables (container c1 has tag t1)
	c1 := db.GetContainer("c1")
	if c1 == nil {
		t.Fatal("container c1 not found")
	}
	if len(c1.TagIDs) != 1 || c1.TagIDs[0] != "t1" {
		t.Errorf("c1 TagIDs = %v, want [t1]", c1.TagIDs)
	}

	// Verify backup files created
	if _, err := os.Stat(filepath.Join(dataDir, "containers.json.migrated")); os.IsNotExist(err) {
		t.Error("containers.json.migrated not created")
	}

	// Verify original JSON files removed (renamed to .migrated)
	if _, err := os.Stat(filepath.Join(dataDir, "containers.json")); !os.IsNotExist(err) {
		t.Error("containers.json should have been renamed to .migrated")
	}
}

func TestMigrateJSON_AlreadyMigrated(t *testing.T) {
	// If DB already has data, skip re-import
	dataDir := t.TempDir()
	fixtureDir := filepath.Join("testdata", "partitioned")

	for _, name := range []string{"containers.json", "items.json", "tags.json"} {
		src := filepath.Join(fixtureDir, name)
		dst := filepath.Join(dataDir, name)
		data, err := os.ReadFile(src) //nolint:gosec // G304: test fixture path
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		if err := os.WriteFile(dst, data, 0600); err != nil { //nolint:gosec // G306: test temp file
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	// Open once (imports data)
	db1, err := New(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	_ = db1.Close()

	// Open again — should NOT duplicate data
	db2, err := New(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	containers := db2.AllContainers()
	if len(containers) != 2 {
		t.Errorf("got %d containers, want 2 (no duplicates)", len(containers))
	}
}
