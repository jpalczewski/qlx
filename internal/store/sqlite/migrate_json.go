package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// migrateJSON checks for legacy JSON files and imports them into SQLite if the DB is empty.
// Called from New() after running schema migrations.
func (s *SQLiteStore) migrateJSON() error {
	if s.dataDir == ":memory:" {
		return nil
	}

	// Check if any legacy JSON files exist
	legacyFiles := []string{"meta.json", "containers.json", "data.json"}
	hasLegacy := false
	for _, f := range legacyFiles {
		if _, err := os.Stat(filepath.Join(s.dataDir, f)); err == nil {
			hasLegacy = true
			break
		}
	}
	if !hasLegacy {
		return nil
	}

	// Check if DB already has data
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM containers").Scan(&count); err != nil {
		return fmt.Errorf("check containers: %w", err)
	}
	if count > 0 {
		return nil // already migrated
	}

	// Import data
	if err := s.importJSON(); err != nil {
		return fmt.Errorf("import JSON: %w", err)
	}

	// Backup JSON files
	s.backupJSONFiles()
	return nil
}

func (s *SQLiteStore) importJSON() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if err := importTags(tx, s.dataDir); err != nil {
		return fmt.Errorf("import tags: %w", err)
	}
	if err := importContainers(tx, s.dataDir); err != nil {
		return fmt.Errorf("import containers: %w", err)
	}
	if err := importItems(tx, s.dataDir); err != nil {
		return fmt.Errorf("import items: %w", err)
	}
	if err := importPrinters(tx, s.dataDir); err != nil {
		return fmt.Errorf("import printers: %w", err)
	}
	if err := importTemplates(tx, s.dataDir); err != nil {
		return fmt.Errorf("import templates: %w", err)
	}

	return tx.Commit()
}

// legacyContainer is the JSON representation from the legacy partitioned file format.
type legacyContainer struct {
	ID          string   `json:"id"`
	ParentID    string   `json:"parent_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Color       string   `json:"color"`
	Icon        string   `json:"icon"`
	TagIDs      []string `json:"tag_ids"`
}

// legacyItem is the JSON representation from the legacy partitioned file format.
type legacyItem struct {
	ID          string   `json:"id"`
	ContainerID string   `json:"container_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Quantity    int      `json:"quantity"`
	Color       string   `json:"color"`
	Icon        string   `json:"icon"`
	TagIDs      []string `json:"tag_ids"`
}

// legacyTag is the JSON representation from the legacy partitioned file format.
type legacyTag struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Icon     string `json:"icon"`
}

// readJSONFile reads a JSON array from path into a slice of T.
// Returns nil, nil if the file does not exist.
func readJSONFile[T any](path string) ([]T, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil // file not present is OK
	}
	if err != nil {
		return nil, err
	}
	var items []T
	return items, json.Unmarshal(data, &items)
}

func importTags(tx *sql.Tx, dataDir string) error {
	tags, err := readJSONFile[legacyTag](filepath.Join(dataDir, "tags.json"))
	if err != nil {
		return err
	}
	for _, t := range tags {
		var pid *string
		if t.ParentID != "" {
			pid = &t.ParentID
		}
		_, err := tx.Exec(`INSERT OR IGNORE INTO tags (id, parent_id, name, color, icon) VALUES (?, ?, ?, ?, ?)`,
			t.ID, pid, t.Name, t.Color, t.Icon)
		if err != nil {
			return err
		}
	}
	return nil
}

func importContainers(tx *sql.Tx, dataDir string) error {
	containers, err := readJSONFile[legacyContainer](filepath.Join(dataDir, "containers.json"))
	if err != nil {
		return err
	}
	// Insert in topological order (parents before children).
	// Simple approach: multiple passes until all are inserted.
	remaining := make([]legacyContainer, len(containers))
	copy(remaining, containers)
	maxPasses := len(remaining) + 1
	for pass := 0; pass < maxPasses && len(remaining) > 0; pass++ {
		var nextRemaining []legacyContainer
		for _, c := range remaining {
			var pid *string
			if c.ParentID != "" {
				pid = &c.ParentID
			}
			_, insertErr := tx.Exec(`INSERT OR IGNORE INTO containers (id, parent_id, name, description, color, icon) VALUES (?, ?, ?, ?, ?, ?)`,
				c.ID, pid, c.Name, c.Description, c.Color, c.Icon)
			if insertErr != nil {
				nextRemaining = append(nextRemaining, c)
			}
		}
		remaining = nextRemaining
	}
	// Insert junction table entries for container_tags.
	for _, c := range containers {
		for _, tagID := range c.TagIDs {
			_, _ = tx.Exec(`INSERT OR IGNORE INTO container_tags (container_id, tag_id) VALUES (?, ?)`, c.ID, tagID)
		}
	}
	return nil
}

func importItems(tx *sql.Tx, dataDir string) error {
	items, err := readJSONFile[legacyItem](filepath.Join(dataDir, "items.json"))
	if err != nil {
		return err
	}
	for _, item := range items {
		qty := item.Quantity
		if qty == 0 {
			qty = 1
		}
		_, err := tx.Exec(`INSERT OR IGNORE INTO items (id, container_id, name, description, quantity, color, icon) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.ContainerID, item.Name, item.Description, qty, item.Color, item.Icon)
		if err != nil {
			return err
		}
		for _, tagID := range item.TagIDs {
			_, _ = tx.Exec(`INSERT OR IGNORE INTO item_tags (item_id, tag_id) VALUES (?, ?)`, item.ID, tagID)
		}
	}
	return nil
}

func importPrinters(tx *sql.Tx, dataDir string) error {
	type legacyPrinter struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Encoder   string `json:"encoder"`
		Model     string `json:"model"`
		Transport string `json:"transport"`
		Address   string `json:"address"`
		OffsetX   int    `json:"offset_x"`
		OffsetY   int    `json:"offset_y"`
	}
	printers, err := readJSONFile[legacyPrinter](filepath.Join(dataDir, "printers.json"))
	if err != nil {
		return err
	}
	for _, p := range printers {
		_, err := tx.Exec(`INSERT OR IGNORE INTO printer_configs (id, name, encoder, model, transport, address, offset_x, offset_y) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			p.ID, p.Name, p.Encoder, p.Model, p.Transport, p.Address, p.OffsetX, p.OffsetY)
		if err != nil {
			return err
		}
	}
	return nil
}

func importTemplates(tx *sql.Tx, dataDir string) error {
	type legacyTemplate struct {
		ID       string      `json:"id"`
		Name     string      `json:"name"`
		Tags     []string    `json:"tags"`
		Target   string      `json:"target"`
		WidthMM  float64     `json:"width_mm"`
		HeightMM float64     `json:"height_mm"`
		WidthPx  int         `json:"width_px"`
		HeightPx int         `json:"height_px"`
		Elements interface{} `json:"elements"` // may be a JSON array or a pre-serialised string
	}
	templates, err := readJSONFile[legacyTemplate](filepath.Join(dataDir, "templates.json"))
	if err != nil {
		return err
	}
	for _, tmpl := range templates {
		tagsJSON, _ := json.Marshal(tmpl.Tags)
		// Elements may already be a string ("[]") or an actual JSON array.
		// Normalise to a JSON string for storage.
		var elemStr string
		switch v := tmpl.Elements.(type) {
		case string:
			elemStr = v
		default:
			b, _ := json.Marshal(tmpl.Elements)
			elemStr = string(b)
		}
		_, err := tx.Exec(`INSERT OR IGNORE INTO templates (id, name, tags, target, width_mm, height_mm, width_px, height_px, elements) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			tmpl.ID, tmpl.Name, string(tagsJSON), tmpl.Target, tmpl.WidthMM, tmpl.HeightMM, tmpl.WidthPx, tmpl.HeightPx, elemStr)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) backupJSONFiles() {
	files := []string{"meta.json", "containers.json", "items.json", "tags.json", "printers.json", "templates.json", "assets.json", "data.json"}
	for _, f := range files {
		src := filepath.Join(s.dataDir, f)
		dst := src + ".migrated"
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		if _, err := os.Stat(dst); err == nil {
			continue // backup already exists, don't overwrite
		}
		_ = os.Rename(src, dst)
	}
}
