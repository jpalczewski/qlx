package sqlite

import (
	"github.com/erxyi/qlx/internal/store"
	"github.com/google/uuid"
)

// AllPrinters returns all printer configurations ordered by name.
func (s *SQLiteStore) AllPrinters() []store.PrinterConfig {
	rows, err := s.db.Query(`SELECT id, name, encoder, model, transport, address, offset_x, offset_y FROM printer_configs ORDER BY name`)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()
	var result []store.PrinterConfig
	for rows.Next() {
		var p store.PrinterConfig
		if err := rows.Scan(&p.ID, &p.Name, &p.Encoder, &p.Model, &p.Transport, &p.Address, &p.OffsetX, &p.OffsetY); err == nil {
			result = append(result, p)
		}
	}
	return result
}

// GetPrinter returns the printer with the given ID, or nil if not found.
func (s *SQLiteStore) GetPrinter(id string) *store.PrinterConfig {
	var p store.PrinterConfig
	err := s.db.QueryRow(`SELECT id, name, encoder, model, transport, address, offset_x, offset_y FROM printer_configs WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.Encoder, &p.Model, &p.Transport, &p.Address, &p.OffsetX, &p.OffsetY)
	if err != nil {
		return nil
	}
	return &p
}

// AddPrinter inserts a new printer configuration and returns it, or nil on error.
func (s *SQLiteStore) AddPrinter(name, encoder, model, transport, address string) *store.PrinterConfig {
	id := uuid.New().String()
	_, err := s.db.Exec(`INSERT INTO printer_configs (id, name, encoder, model, transport, address) VALUES (?, ?, ?, ?, ?, ?)`,
		id, name, encoder, model, transport, address)
	if err != nil {
		return nil
	}
	return s.GetPrinter(id)
}

// DeletePrinter removes a printer by ID.
// Returns store.ErrPrinterNotFound if no row matched.
func (s *SQLiteStore) DeletePrinter(id string) error {
	res, err := s.db.Exec(`DELETE FROM printer_configs WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrPrinterNotFound
	}
	return nil
}

// UpdatePrinterOffset sets the calibration offsets for the given printer.
// Returns store.ErrPrinterNotFound if no row matched.
func (s *SQLiteStore) UpdatePrinterOffset(id string, offsetX, offsetY int) error {
	res, err := s.db.Exec(`UPDATE printer_configs SET offset_x=?, offset_y=? WHERE id=?`, offsetX, offsetY, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrPrinterNotFound
	}
	return nil
}
