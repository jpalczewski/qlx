package sqlite

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/driver"
	"github.com/pressly/goose/v3"
)

// SQLiteStore implements the store.Store interface using SQLite.
type SQLiteStore struct {
	db      *sql.DB
	dataDir string
}

// New opens (or creates) a SQLite database in dataDir, runs migrations, and returns the store.
// Pass ":memory:" as dataDir for in-memory testing.
func New(dataDir string) (*SQLiteStore, error) {
	var dsn string
	if dataDir == ":memory:" {
		dsn = ":memory:"
	} else {
		dsn = filepath.Join(dataDir, "qlx.db")
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Single connection for WAL mode consistency
	db.SetMaxOpenConns(1)

	if err := setPragmas(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set pragmas: %w", err)
	}

	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	s := &SQLiteStore{db: db, dataDir: dataDir}

	if err := s.migrateJSON(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate json: %w", err)
	}

	return s, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func setPragmas(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA cache_size = -4096",
		"PRAGMA foreign_keys = ON",
		"PRAGMA temp_store = MEMORY",
		"PRAGMA auto_vacuum = INCREMENTAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	return nil
}

func init() {
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		panic("goose sqlite3 dialect: " + err.Error())
	}
}

func runMigrations(db *sql.DB) error {
	return goose.Up(db, "migrations")
}
