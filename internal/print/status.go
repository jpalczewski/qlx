package print

import "time"

// PrinterStatus represents the current state of a connected printer.
type PrinterStatus struct {
	Connected    bool      `json:"connected"`
	Battery      int       `json:"battery"`        // 0-100, -1 if unknown
	LidClosed    bool      `json:"lid_closed"`
	PaperLoaded  bool      `json:"paper_loaded"`
	LabelType    string    `json:"label_type"`     // "with-gaps", "transparent", etc.
	TotalLabels  int       `json:"total_labels"`   // from RFID, -1 if unknown
	UsedLabels   int       `json:"used_labels"`    // from RFID, -1 if unknown
	PrintWidthMm int       `json:"print_width_mm"` // calculated from model DPI + printhead pixels
	DPI          int       `json:"dpi"`
	LastError    string    `json:"last_error"`
	LastUpdated  time.Time `json:"last_updated"`
}
