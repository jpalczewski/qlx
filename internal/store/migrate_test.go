package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateDetectsVersion(t *testing.T) {
	t.Run("version 0 from file without version field", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "data.json")

		// Write a v0 store file (no "version" field)
		raw := `{"containers":{},"items":{}}`
		if err := os.WriteFile(path, []byte(raw), 0644); err != nil { //nolint:gosec // G306: test setup
			t.Fatalf("setup: %v", err)
		}

		_, version, err := loadRaw(path)
		if err != nil {
			t.Fatalf("loadRaw() error = %v", err)
		}
		if version != 0 {
			t.Errorf("version = %d, want 0", version)
		}
	})

	t.Run("version detected from version field", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "data.json")

		raw := `{"version":1,"containers":{},"items":{}}`
		if err := os.WriteFile(path, []byte(raw), 0644); err != nil { //nolint:gosec // G306: test setup
			t.Fatalf("setup: %v", err)
		}

		_, version, err := loadRaw(path)
		if err != nil {
			t.Fatalf("loadRaw() error = %v", err)
		}
		if version != 1 {
			t.Errorf("version = %d, want 1", version)
		}
	})
}

func runV0ToV1Setup(t *testing.T) (path string, result map[string]any, newVersion int) {
	t.Helper()

	tmpDir := t.TempDir()
	path = filepath.Join(tmpDir, "data.json")

	// A v0 store with one item and one container, no quantity/tag_ids/tags.
	v0 := `{"containers":{"c1":{"id":"c1","name":"Box"}},"items":{"i1":{"id":"i1","name":"Widget"}}}`
	if err := os.WriteFile(path, []byte(v0), 0644); err != nil { //nolint:gosec // G306: test setup
		t.Fatalf("setup: %v", err)
	}

	raw, version, err := loadRaw(path)
	if err != nil {
		t.Fatalf("loadRaw: %v", err)
	}
	if version != 0 {
		t.Fatalf("expected version 0, got %d", version)
	}

	migrated, newVersion, err := runMigrations(path, raw, version)
	if err != nil {
		t.Fatalf("runMigrations: %v", err)
	}

	if err := json.Unmarshal(migrated, &result); err != nil {
		t.Fatalf("unmarshal migrated: %v", err)
	}

	return path, result, newVersion
}

func TestMigrateV0ToV1(t *testing.T) {
	t.Run("new version is current", func(t *testing.T) {
		_, _, newVersion := runV0ToV1Setup(t)
		if newVersion != currentVersion {
			t.Errorf("newVersion = %d, want %d", newVersion, currentVersion)
		}
	})

	t.Run("backup created", func(t *testing.T) {
		path, _, _ := runV0ToV1Setup(t)
		backupPath := path + ".v0.bak"
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Errorf("backup file not found at %s", backupPath)
		}
	})

	t.Run("version field set to current", func(t *testing.T) {
		_, result, _ := runV0ToV1Setup(t)
		if v, ok := result["version"].(float64); !ok || int(v) != currentVersion {
			t.Errorf("version in migrated = %v, want %d", result["version"], currentVersion)
		}
	})

	t.Run("tags collection added", func(t *testing.T) {
		_, result, _ := runV0ToV1Setup(t)
		if _, ok := result["tags"]; !ok {
			t.Error("tags collection missing from migrated data")
		}
	})

	t.Run("item gets quantity 1", func(t *testing.T) {
		_, result, _ := runV0ToV1Setup(t)
		items, _ := result["items"].(map[string]any)
		item, _ := items["i1"].(map[string]any)
		if qty, ok := item["quantity"].(float64); !ok || int(qty) != 1 {
			t.Errorf("item quantity = %v, want 1", item["quantity"])
		}
	})

	t.Run("item gets empty tag_ids", func(t *testing.T) {
		_, result, _ := runV0ToV1Setup(t)
		items, _ := result["items"].(map[string]any)
		item, _ := items["i1"].(map[string]any)
		if tagIDs, ok := item["tag_ids"].([]any); !ok || len(tagIDs) != 0 {
			t.Errorf("item tag_ids = %v, want empty array", item["tag_ids"])
		}
	})

	t.Run("container gets empty tag_ids", func(t *testing.T) {
		_, result, _ := runV0ToV1Setup(t)
		containers, _ := result["containers"].(map[string]any)
		container, _ := containers["c1"].(map[string]any)
		if tagIDs, ok := container["tag_ids"].([]any); !ok || len(tagIDs) != 0 {
			t.Errorf("container tag_ids = %v, want empty array", container["tag_ids"])
		}
	})
}

func TestNewStoreRunsMigration(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.json")

	// Write a v0 store with an item
	v0 := `{"containers":{},"items":{"i1":{"id":"i1","container_id":"","name":"Widget","description":"","created_at":"2025-01-01T00:00:00Z"}}}`
	if err := os.WriteFile(path, []byte(v0), 0644); err != nil { //nolint:gosec // G306: test setup
		t.Fatalf("setup: %v", err)
	}

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	item := s.GetItem("i1")
	if item == nil {
		t.Fatal("item i1 not found after migration")
	}
	if item.Quantity != 1 {
		t.Errorf("item Quantity = %d, want 1", item.Quantity)
	}
	if item.TagIDs == nil {
		t.Error("item TagIDs should be non-nil after migration")
	}
	if len(item.TagIDs) != 0 {
		t.Errorf("item TagIDs = %v, want empty slice", item.TagIDs)
	}
}
