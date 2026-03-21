package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	t.Run("empty path creates empty store", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "data.json")

		s, err := NewStore(path)
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}
		if len(s.AllContainers()) != 0 || len(s.AllItems()) != 0 {
			t.Error("new store should be empty")
		}
	})

	t.Run("load existing data", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "data.json")

		existing := `{"containers":{"c1":{"id":"c1","parent_id":"","name":"Root","description":"","created_at":"2025-01-01T00:00:00Z"}},"items":{"i1":{"id":"i1","container_id":"c1","name":"Item","description":"","created_at":"2025-01-01T00:00:00Z"}}}`
		if err := os.WriteFile(path, []byte(existing), 0644); err != nil { //nolint:gosec // G306: test setup, intentional permissions
			t.Fatalf("setup: %v", err)
		}

		s, err := NewStore(path)
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}

		if len(s.AllContainers()) != 1 {
			t.Errorf("Containers count = %d, want 1", len(s.AllContainers()))
		}
		c := s.GetContainer("c1")
		if c == nil || c.Name != "Root" {
			t.Errorf("Container c1 name = %v, want Root", c)
		}
		if len(s.AllItems()) != 1 {
			t.Errorf("Items count = %d, want 1", len(s.AllItems()))
		}
		i := s.GetItem("i1")
		if i == nil || i.Name != "Item" {
			t.Errorf("Item i1 name = %v, want Item", i)
		}
	})
}

func TestContainerCRUD(t *testing.T) {
	s := NewMemoryStore()

	c := s.CreateContainer("", "Room", "A room", "", "")
	if c.ID == "" {
		t.Error("CreateContainer should set ID")
	}
	if c.Name != "Room" {
		t.Errorf("Name = %q, want %q", c.Name, "Room")
	}

	got := s.GetContainer(c.ID)
	if got == nil {
		t.Fatal("GetContainer returned nil")
	}
	if got.Name != "Room" {
		t.Errorf("GetContainer Name = %q, want %q", got.Name, "Room")
	}

	updated, err := s.UpdateContainer(c.ID, "Bedroom", "A bedroom", "", "")
	if err != nil {
		t.Fatalf("UpdateContainer error = %v", err)
	}
	if updated.Name != "Bedroom" {
		t.Errorf("UpdateContainer Name = %q, want %q", updated.Name, "Bedroom")
	}

	if err := s.DeleteContainer(c.ID); err != nil {
		t.Fatalf("DeleteContainer error = %v", err)
	}
	if s.GetContainer(c.ID) != nil {
		t.Error("DeleteContainer should remove container")
	}

	_, err = s.UpdateContainer("nonexistent", "Name", "", "", "")
	if !errors.Is(err, ErrContainerNotFound) {
		t.Errorf("UpdateContainer error = %v, want ErrContainerNotFound", err)
	}

	err = s.DeleteContainer("nonexistent")
	if !errors.Is(err, ErrContainerNotFound) {
		t.Errorf("DeleteContainer error = %v, want ErrContainerNotFound", err)
	}
}

func TestItemCRUD(t *testing.T) {
	s := NewMemoryStore()
	container := s.CreateContainer("", "Box", "", "", "")

	item := s.CreateItem(container.ID, "Cable", "HDMI cable", 1, "", "")
	if item.ID == "" {
		t.Error("CreateItem should set ID")
	}
	if item.ContainerID != container.ID {
		t.Errorf("ContainerID = %q, want %q", item.ContainerID, container.ID)
	}

	got := s.GetItem(item.ID)
	if got == nil {
		t.Fatal("GetItem returned nil")
	}
	if got.Name != "Cable" {
		t.Errorf("GetItem Name = %q, want %q", got.Name, "Cable")
	}

	updated, err := s.UpdateItem(item.ID, "HDMI Cable", "2m HDMI cable", 5, "", "")
	if err != nil {
		t.Fatalf("UpdateItem error = %v", err)
	}
	if updated.Name != "HDMI Cable" {
		t.Errorf("UpdateItem Name = %q, want %q", updated.Name, "HDMI Cable")
	}
	if updated.Quantity != 5 {
		t.Errorf("UpdateItem Quantity = %d, want 5", updated.Quantity)
	}

	// Quantity 0 should preserve existing value.
	preserved, err := s.UpdateItem(item.ID, "HDMI Cable", "2m HDMI cable", 0, "", "")
	if err != nil {
		t.Fatalf("UpdateItem preserve qty error = %v", err)
	}
	if preserved.Quantity != 5 {
		t.Errorf("UpdateItem preserved Quantity = %d, want 5", preserved.Quantity)
	}

	if err := s.DeleteItem(item.ID); err != nil {
		t.Fatalf("DeleteItem error = %v", err)
	}
	if s.GetItem(item.ID) != nil {
		t.Error("DeleteItem should remove item")
	}

	_, err = s.UpdateItem("nonexistent", "Name", "", 1, "", "")
	if !errors.Is(err, ErrItemNotFound) {
		t.Errorf("UpdateItem error = %v, want ErrItemNotFound", err)
	}

	err = s.DeleteItem("nonexistent")
	if !errors.Is(err, ErrItemNotFound) {
		t.Errorf("DeleteItem error = %v, want ErrItemNotFound", err)
	}
}

func TestContainerPath(t *testing.T) {
	s := NewMemoryStore()

	root := s.CreateContainer("", "Room", "", "", "")
	shelf := s.CreateContainer(root.ID, "Shelf", "", "", "")
	box := s.CreateContainer(shelf.ID, "Box", "", "", "")

	path := s.ContainerPath(box.ID)
	if len(path) != 3 {
		t.Fatalf("ContainerPath length = %d, want 3", len(path))
	}
	if path[0].Name != "Room" {
		t.Errorf("path[0].Name = %q, want %q", path[0].Name, "Room")
	}
	if path[1].Name != "Shelf" {
		t.Errorf("path[1].Name = %q, want %q", path[1].Name, "Shelf")
	}
	if path[2].Name != "Box" {
		t.Errorf("path[2].Name = %q, want %q", path[2].Name, "Box")
	}

	rootPath := s.ContainerPath(root.ID)
	if len(rootPath) != 1 {
		t.Fatalf("ContainerPath(root) length = %d, want 1", len(rootPath))
	}

	emptyPath := s.ContainerPath("nonexistent")
	if len(emptyPath) != 0 {
		t.Errorf("ContainerPath(nonexistent) length = %d, want 0", len(emptyPath))
	}
}

func TestContainerChildren(t *testing.T) {
	s := NewMemoryStore()

	root := s.CreateContainer("", "Room", "", "", "")
	child1 := s.CreateContainer(root.ID, "Shelf 1", "", "", "")
	_ = s.CreateContainer(root.ID, "Shelf 2", "", "", "")
	grandchild := s.CreateContainer(child1.ID, "Box", "", "", "")

	children := s.ContainerChildren(root.ID)
	if len(children) != 2 {
		t.Fatalf("ContainerChildren count = %d, want 2", len(children))
	}

	names := make(map[string]bool)
	for _, c := range children {
		names[c.Name] = true
	}
	if !names["Shelf 1"] || !names["Shelf 2"] {
		t.Errorf("ContainerChildren names = %v, want Shelf 1 and Shelf 2", names)
	}

	grandchildren := s.ContainerChildren(child1.ID)
	if len(grandchildren) != 1 {
		t.Fatalf("ContainerChildren(grandchild) count = %d, want 1", len(grandchildren))
	}
	if grandchildren[0].Name != "Box" {
		t.Errorf("grandchild name = %q, want %q", grandchildren[0].Name, "Box")
	}

	none := s.ContainerChildren(grandchild.ID)
	if len(none) != 0 {
		t.Errorf("ContainerChildren(leaf) count = %d, want 0", len(none))
	}
}

func TestContainerItems(t *testing.T) {
	s := NewMemoryStore()

	container := s.CreateContainer("", "Box", "", "", "")
	other := s.CreateContainer("", "Other", "", "", "")

	item1 := s.CreateItem(container.ID, "Item 1", "", 1, "", "")
	item2 := s.CreateItem(container.ID, "Item 2", "", 1, "", "")
	_ = s.CreateItem(other.ID, "Item 3", "", 1, "", "")

	items := s.ContainerItems(container.ID)
	if len(items) != 2 {
		t.Fatalf("ContainerItems count = %d, want 2", len(items))
	}

	names := make(map[string]bool)
	for _, item := range items {
		names[item.Name] = true
	}
	if !names[item1.Name] || !names[item2.Name] {
		t.Errorf("ContainerItems names = %v, want Item 1 and Item 2", names)
	}

	empty := s.ContainerItems("nonexistent")
	if len(empty) != 0 {
		t.Errorf("ContainerItems(nonexistent) count = %d, want 0", len(empty))
	}
}

func TestMoveItem(t *testing.T) {
	s := NewMemoryStore()

	container1 := s.CreateContainer("", "Box 1", "", "", "")
	container2 := s.CreateContainer("", "Box 2", "", "", "")
	item := s.CreateItem(container1.ID, "Item", "", 1, "", "")

	if err := s.MoveItem(item.ID, container2.ID); err != nil {
		t.Fatalf("MoveItem error = %v", err)
	}

	moved := s.GetItem(item.ID)
	if moved.ContainerID != container2.ID {
		t.Errorf("ContainerID = %q, want %q", moved.ContainerID, container2.ID)
	}

	if err := s.MoveItem(item.ID, ""); err != nil {
		t.Fatalf("MoveItem to root error = %v", err)
	}
	moved = s.GetItem(item.ID)
	if moved.ContainerID != "" {
		t.Errorf("ContainerID = %q, want empty", moved.ContainerID)
	}

	err := s.MoveItem("nonexistent", container1.ID)
	if !errors.Is(err, ErrItemNotFound) {
		t.Errorf("MoveItem error = %v, want ErrItemNotFound", err)
	}

	err = s.MoveItem(item.ID, "nonexistent")
	if !errors.Is(err, ErrInvalidContainer) {
		t.Errorf("MoveItem error = %v, want ErrInvalidContainer", err)
	}
}

func TestMoveContainer_PreventsCycle(t *testing.T) {
	s := NewMemoryStore()

	a := s.CreateContainer("", "A", "", "", "")
	b := s.CreateContainer(a.ID, "B", "", "", "")
	c := s.CreateContainer(b.ID, "C", "", "", "")

	err := s.MoveContainer(a.ID, b.ID)
	if !errors.Is(err, ErrCycleDetected) {
		t.Errorf("MoveContainer A->B error = %v, want ErrCycleDetected", err)
	}

	err = s.MoveContainer(a.ID, c.ID)
	if !errors.Is(err, ErrCycleDetected) {
		t.Errorf("MoveContainer A->C error = %v, want ErrCycleDetected", err)
	}

	err = s.MoveContainer(a.ID, a.ID)
	if !errors.Is(err, ErrCycleDetected) {
		t.Errorf("MoveContainer A->A error = %v, want ErrCycleDetected", err)
	}

	d := s.CreateContainer("", "D", "", "", "")
	if err := s.MoveContainer(c.ID, d.ID); err != nil {
		t.Fatalf("MoveContainer C->D error = %v", err)
	}
	moved := s.GetContainer(c.ID)
	if moved.ParentID != d.ID {
		t.Errorf("ParentID = %q, want %q", moved.ParentID, d.ID)
	}

	err = s.MoveContainer("nonexistent", a.ID)
	if !errors.Is(err, ErrContainerNotFound) {
		t.Errorf("MoveContainer error = %v, want ErrContainerNotFound", err)
	}

	err = s.MoveContainer(a.ID, "nonexistent")
	if !errors.Is(err, ErrInvalidParent) {
		t.Errorf("MoveContainer error = %v, want ErrInvalidParent", err)
	}
}

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.json")

	s1, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	container := s1.CreateContainer("", "Room", "A room", "", "")
	item := s1.CreateItem(container.ID, "Cable", "HDMI", 1, "", "")

	if err := s1.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	s2, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() second load error = %v", err)
	}

	if len(s2.AllContainers()) != 1 {
		t.Errorf("Containers count = %d, want 1", len(s2.AllContainers()))
	}
	if len(s2.AllItems()) != 1 {
		t.Errorf("Items count = %d, want 1", len(s2.AllItems()))
	}

	loadedContainer := s2.GetContainer(container.ID)
	if loadedContainer == nil {
		t.Fatal("Container not found after reload")
	}
	if loadedContainer.Name != "Room" {
		t.Errorf("Container Name = %q, want %q", loadedContainer.Name, "Room")
	}

	loadedItem := s2.GetItem(item.ID)
	if loadedItem == nil {
		t.Fatal("Item not found after reload")
	}
	if loadedItem.Name != "Cable" {
		t.Errorf("Item Name = %q, want %q", loadedItem.Name, "Cable")
	}
}

func TestPrinterCRUD(t *testing.T) {
	s := NewMemoryStore()

	p := s.AddPrinter("Brother kuchnia", "brother-ql", "QL-700", "usb", "/dev/usb/lp0")
	if p.ID == "" {
		t.Error("AddPrinter should set ID")
	}
	if p.Name != "Brother kuchnia" {
		t.Errorf("Name = %q, want %q", p.Name, "Brother kuchnia")
	}

	got := s.GetPrinter(p.ID)
	if got == nil || got.Name != "Brother kuchnia" {
		t.Error("GetPrinter failed")
	}

	all := s.AllPrinters()
	if len(all) != 1 {
		t.Errorf("AllPrinters count = %d, want 1", len(all))
	}

	err := s.DeletePrinter(p.ID)
	if err != nil {
		t.Fatalf("DeletePrinter error: %v", err)
	}
	if s.GetPrinter(p.ID) != nil {
		t.Error("printer should be deleted")
	}

	err = s.DeletePrinter("nonexistent")
	if err == nil {
		t.Error("expected error deleting nonexistent printer")
	}
}

func TestPrinterPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.json")

	s1, _ := NewStore(path)
	p := s1.AddPrinter("Test", "niimbot", "B1", "serial", "/dev/tty.BT")
	if err := s1.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	s2, _ := NewStore(path)
	got := s2.GetPrinter(p.ID)
	if got == nil || got.Name != "Test" {
		t.Error("printer not persisted")
	}
}

func TestDeleteContainer_Constraints(t *testing.T) {
	s := NewMemoryStore()

	parent := s.CreateContainer("", "Parent", "", "", "")
	child := s.CreateContainer(parent.ID, "Child", "", "", "")
	item := s.CreateItem(parent.ID, "Item", "", 1, "", "")

	err := s.DeleteContainer(parent.ID)
	if !errors.Is(err, ErrContainerHasChildren) {
		t.Errorf("DeleteContainer with children error = %v, want ErrContainerHasChildren", err)
	}

	if err := s.DeleteItem(item.ID); err != nil {
		t.Fatalf("DeleteItem error = %v", err)
	}

	err = s.DeleteContainer(parent.ID)
	if !errors.Is(err, ErrContainerHasChildren) {
		t.Errorf("DeleteContainer still has child error = %v, want ErrContainerHasChildren", err)
	}

	if err := s.DeleteContainer(child.ID); err != nil {
		t.Fatalf("DeleteContainer child error = %v", err)
	}

	if err := s.DeleteContainer(parent.ID); err != nil {
		t.Fatalf("DeleteContainer parent error = %v", err)
	}
}

func TestTemplateCRUD(t *testing.T) {
	s := NewMemoryStore()

	// Create
	tmpl := s.CreateTemplate("Address Label", []string{"shipping"}, "universal", 62, 29, 0, 0, `[{"type":"text","value":"Hello"}]`)
	if tmpl.ID == "" {
		t.Error("CreateTemplate should set ID")
	}
	if tmpl.Name != "Address Label" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "Address Label")
	}
	if tmpl.Target != "universal" {
		t.Errorf("Target = %q, want %q", tmpl.Target, "universal")
	}
	if tmpl.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	// Get
	got := s.GetTemplate(tmpl.ID)
	if got == nil {
		t.Fatal("GetTemplate returned nil")
	}
	if got.Name != "Address Label" {
		t.Errorf("GetTemplate Name = %q, want %q", got.Name, "Address Label")
	}

	// Get nonexistent
	if s.GetTemplate("nonexistent") != nil {
		t.Error("GetTemplate should return nil for nonexistent ID")
	}

	// List
	s.CreateTemplate("QR Label", []string{"inventory"}, "printer:B1", 0, 0, 384, 240, `[]`)
	all := s.AllTemplates()
	if len(all) != 2 {
		t.Errorf("AllTemplates count = %d, want 2", len(all))
	}

	// Update (SaveTemplate)
	tmpl.Name = "Updated Label"
	tmpl.Elements = `[{"type":"text","value":"Updated"}]`
	s.SaveTemplate(*tmpl)

	updated := s.GetTemplate(tmpl.ID)
	if updated.Name != "Updated Label" {
		t.Errorf("SaveTemplate Name = %q, want %q", updated.Name, "Updated Label")
	}
	if updated.UpdatedAt.Before(tmpl.CreatedAt) {
		t.Error("UpdatedAt should be after CreatedAt")
	}

	// Delete
	s.DeleteTemplate(tmpl.ID)
	if s.GetTemplate(tmpl.ID) != nil {
		t.Error("DeleteTemplate should remove template")
	}
	if len(s.AllTemplates()) != 1 {
		t.Errorf("AllTemplates after delete = %d, want 1", len(s.AllTemplates()))
	}

	// Delete nonexistent (should not panic)
	s.DeleteTemplate("nonexistent")
}

func TestAssetCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	assetsDir := filepath.Join(tmpDir, "assets")
	path := filepath.Join(tmpDir, "data.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	imgData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
	var assetID string

	t.Run("Save", func(t *testing.T) {
		asset, err := s.SaveAsset("logo.png", "image/png", imgData)
		if err != nil {
			t.Fatalf("SaveAsset error = %v", err)
		}
		if asset.ID == "" {
			t.Error("should set ID")
		}
		if asset.Name != "logo.png" {
			t.Errorf("Name = %q, want %q", asset.Name, "logo.png")
		}
		assetID = asset.ID
	})

	t.Run("Get", func(t *testing.T) {
		got := s.GetAsset(assetID)
		if got == nil {
			t.Fatal("returned nil")
		}
		if got.Name != "logo.png" {
			t.Errorf("Name = %q, want %q", got.Name, "logo.png")
		}
		if s.GetAsset("nonexistent") != nil {
			t.Error("should return nil for nonexistent")
		}
	})

	t.Run("ReadData", func(t *testing.T) {
		data, err := s.AssetData(assetID)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if len(data) != len(imgData) {
			t.Errorf("length = %d, want %d", len(data), len(imgData))
		}
		if _, err := s.AssetData("nonexistent"); err == nil {
			t.Error("should error for nonexistent")
		}
	})

	t.Run("List", func(t *testing.T) {
		if _, err := s.SaveAsset("icon.jpg", "image/jpeg", []byte{0xFF, 0xD8}); err != nil {
			t.Fatal(err)
		}
		if len(s.AllAssets()) != 2 {
			t.Errorf("count = %d, want 2", len(s.AllAssets()))
		}
	})

	t.Run("Delete", func(t *testing.T) {
		s.DeleteAsset(assetID)
		if s.GetAsset(assetID) != nil {
			t.Error("should remove asset")
		}
		if _, err := os.Stat(filepath.Join(assetsDir, assetID+".bin")); !os.IsNotExist(err) {
			t.Error("should remove file from disk")
		}
		s.DeleteAsset("nonexistent") // should not panic
	})
}
