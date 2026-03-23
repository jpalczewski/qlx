package sqlite

import (
	"os"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *SQLiteStore {
	t.Helper()
	db, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestNew_CreatesDBFile(t *testing.T) {
	dir := t.TempDir()
	db, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	dbPath := filepath.Join(dir, "qlx.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("expected %s to exist", dbPath)
	}
}

func TestNew_RunsMigrations(t *testing.T) {
	db := testStore(t)

	var count int
	err := db.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='containers'").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("containers table not created by migrations")
	}
}

func TestNew_SetsPragmas(t *testing.T) {
	db := testStore(t)

	var journalMode string
	if err := db.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("expected WAL journal mode, got %s", journalMode)
	}

	var fk int
	if err := db.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Fatal("expected foreign_keys ON")
	}
}
