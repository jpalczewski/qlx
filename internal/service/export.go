package service

import "github.com/erxyi/qlx/internal/store"

// ExportService handles data export operations.
type ExportService struct {
	store     ExportStore
	inventory *InventoryService
}

// NewExportService creates a new ExportService.
func NewExportService(s ExportStore, inventory *InventoryService) *ExportService {
	return &ExportService{store: s, inventory: inventory}
}

func (s *ExportService) ExportJSON() (map[string]*store.Container, map[string]*store.Item) {
	return s.store.ExportData()
}

func (s *ExportService) AllItems() []store.Item {
	return s.store.AllItems()
}

func (s *ExportService) AllContainers() []store.Container {
	return s.store.AllContainers()
}
