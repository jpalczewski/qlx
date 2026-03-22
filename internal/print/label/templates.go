package label

// LabelTag represents a tag with its display info for label rendering.
type LabelTag struct {
	Name string   // tag display name
	Icon string   // Phosphor icon name (may be empty)
	Path []string // ancestor names root-first, e.g. ["elektronika", "arduino"]
}

// LabelChild represents a child container or item for label rendering.
type LabelChild struct {
	Name string // child display name
	Icon string // Phosphor icon name (may be empty)
}

// LabelData holds the data slots for label rendering.
type LabelData struct {
	Name        string       // → "title" slot
	Description string       // → "description" slot
	Location    string       // container path "Room → Shelf" → "location" slot
	QRContent   string       // URL for QR code
	BarcodeID   string       // ID for barcode
	Icon        string       // Phosphor icon name for title
	Tags        []LabelTag   // assigned tags
	Children    []LabelChild // sub-containers + items (container only)
}
