package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Migration is a function that mutates the raw store map in place.
type Migration func(data map[string]any) error

// migrations is the ordered list of schema upgrade functions.
// migrations[i] upgrades from version i to version i+1.
var migrations = []Migration{
	migrateV0ToV1,
}

// currentVersion is the schema version produced by the latest migration.
var currentVersion = len(migrations)

// loadRaw reads the store JSON file and returns the parsed map together with
// the schema version stored in the "version" field (0 when the field is absent).
func loadRaw(path string) (map[string]any, int, error) {
	//nolint:gosec // G304: path from trusted CLI input
	fileData, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	if len(fileData) == 0 {
		return map[string]any{}, 0, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(fileData, &raw); err != nil {
		return nil, 0, err
	}

	version := 0
	if v, ok := raw["version"]; ok {
		if vf, ok := v.(float64); ok {
			version = int(vf)
		}
	}

	return raw, version, nil
}

// backupStore writes an atomic backup of path to "<path>.v<version>.bak".
func backupStore(path string, version int) error {
	//nolint:gosec // G304: path from trusted CLI input
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("backup read: %w", err)
	}

	backupPath := fmt.Sprintf("%s.v%d.bak", path, version)
	tmpPath := backupPath + ".tmp"

	//nolint:gosec // G306: intentional permissions for backup file
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("backup write tmp: %w", err)
	}

	if err := os.Rename(tmpPath, backupPath); err != nil {
		return fmt.Errorf("backup rename: %w", err)
	}

	return nil
}

// runMigrations applies all pending migrations starting from currentVersion
// and returns the migrated JSON bytes and the new version number.
func runMigrations(path string, raw map[string]any, fromVersion int) ([]byte, int, error) {
	for i := fromVersion; i < len(migrations); i++ {
		// Back up before each migration step.
		if err := backupStore(path, i); err != nil {
			return nil, fromVersion, fmt.Errorf("migration %d backup: %w", i, err)
		}

		if err := migrations[i](raw); err != nil {
			return nil, i, fmt.Errorf("migration %d: %w", i, err)
		}
	}

	raw["version"] = len(migrations)

	out, err := json.Marshal(raw)
	if err != nil {
		return nil, len(migrations), fmt.Errorf("marshal after migrations: %w", err)
	}

	return out, len(migrations), nil
}

// migrateV0ToV1 upgrades store data from version 0 to version 1:
//   - items: adds quantity=1 and tag_ids=[]
//   - containers: adds tag_ids=[]
//   - adds an empty tags collection
func migrateV0ToV1(data map[string]any) error {
	if items, ok := data["items"].(map[string]any); ok {
		for _, v := range items {
			item, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if _, exists := item["quantity"]; !exists {
				item["quantity"] = float64(1)
			}
			if _, exists := item["tag_ids"]; !exists {
				item["tag_ids"] = []any{}
			}
		}
	}

	if containers, ok := data["containers"].(map[string]any); ok {
		for _, v := range containers {
			container, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if _, exists := container["tag_ids"]; !exists {
				container["tag_ids"] = []any{}
			}
		}
	}

	if _, exists := data["tags"]; !exists {
		data["tags"] = map[string]any{}
	}

	return nil
}

// writeAtomic writes data to path using a temp file + rename pattern.
func writeAtomic(path string, data []byte) error {
	//nolint:gosec // G301: intentional permissions for data directory
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	//nolint:gosec // G306: intentional permissions for data file
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
