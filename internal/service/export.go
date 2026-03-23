package service

import "github.com/erxyi/qlx/internal/store"

// ExportService handles data export operations.
type ExportService struct {
	store store.ExportStore
}

// NewExportService creates a new ExportService.
func NewExportService(s store.ExportStore) *ExportService {
	return &ExportService{store: s}
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
