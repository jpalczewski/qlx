package label

type LabelData struct {
	Name        string
	Description string
	Location    string // container path "Room → Shelf → Box"
	QRContent   string // URL for QR code
	BarcodeID   string // item ID for barcode
}

var templateNames = []string{"simple", "standard", "compact", "detailed"}
