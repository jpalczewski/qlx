package service

import "github.com/erxyi/qlx/internal/store"

// PrinterService handles printer CRUD operations.
type PrinterService struct {
	store store.PrinterStore
}

// NewPrinterService creates a new PrinterService backed by the given store.
func NewPrinterService(s store.PrinterStore) *PrinterService {
	return &PrinterService{store: s}
}

// AllPrinters returns all configured printers.
func (s *PrinterService) AllPrinters() []store.PrinterConfig {
	return s.store.AllPrinters()
}

// GetPrinter returns the printer with the given ID, or nil.
func (s *PrinterService) GetPrinter(id string) *store.PrinterConfig {
	return s.store.GetPrinter(id)
}

// AddPrinter adds a new printer configuration.
func (s *PrinterService) AddPrinter(name, encoder, model, transport, address string) (*store.PrinterConfig, error) {
	p := s.store.AddPrinter(name, encoder, model, transport, address)
	return p, nil
}

// DeletePrinter deletes a printer configuration.
func (s *PrinterService) DeletePrinter(id string) error {
	return s.store.DeletePrinter(id)
}

// UpdateOffset sets calibration offsets for a printer.
func (s *PrinterService) UpdateOffset(id string, offsetX, offsetY int) error {
	return s.store.UpdatePrinterOffset(id, offsetX, offsetY)
}
