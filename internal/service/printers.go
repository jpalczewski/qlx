package service

import (
	"errors"
	"fmt"

	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// ErrUnknownTransport is returned when a printer is added with a transport
// name that is not in the transport registry.
var ErrUnknownTransport = errors.New("unknown transport")

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
func (s *PrinterService) AddPrinter(name, encoder, model, transportName, address string) (*store.PrinterConfig, error) {
	info, ok := transport.Lookup(transportName)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTransport, transportName)
	}
	if !info.Available {
		// Stored anyway so configs survive switching between builds.
		webutil.LogError("printer %q uses transport %q which is not available in this build", name, transportName)
	}
	p := s.store.AddPrinter(name, encoder, model, transportName, address)
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
