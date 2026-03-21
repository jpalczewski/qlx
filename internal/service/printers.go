package service

import "github.com/erxyi/qlx/internal/store"

// PrinterService handles printer CRUD operations,
// calling Save() after each mutation.
type PrinterService struct {
	store interface {
		PrinterStore
		Saveable
	}
}

// NewPrinterService creates a new PrinterService backed by the given store.
func NewPrinterService(s interface {
	PrinterStore
	Saveable
}) *PrinterService {
	return &PrinterService{store: s}
}

// AllPrinters returns all configured printers.
func (s *PrinterService) AllPrinters() []store.PrinterConfig {
	return s.store.AllPrinters()
}

// AddPrinter adds a new printer configuration and persists.
func (s *PrinterService) AddPrinter(name, encoder, model, transport, address string) (*store.PrinterConfig, error) {
	p := s.store.AddPrinter(name, encoder, model, transport, address)
	if err := s.store.Save(); err != nil {
		return nil, err
	}
	return p, nil
}

// DeletePrinter deletes a printer configuration and persists.
func (s *PrinterService) DeletePrinter(id string) error {
	if err := s.store.DeletePrinter(id); err != nil {
		return err
	}
	return s.store.Save()
}
