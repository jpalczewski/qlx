package encoder

import (
	"image"
	"github.com/erxyi/qlx/internal/print/transport"
)

type Encoder interface {
	Name() string
	Models() []ModelInfo
	Encode(img image.Image, model string, opts PrintOpts, tr transport.Transport) error
}

type ModelInfo struct {
	ID             string
	Name           string
	DPI            int
	PrintWidthPx   int
	MediaTypes     []string
	DensityRange   [2]int
	DensityDefault int
}

type PrintOpts struct {
	Density  int
	AutoCut  bool
	Quantity int
}

// StatusQuerier is optionally implemented by encoders that support status queries.
type StatusQuerier interface {
	// Connect sends initial handshake.
	Connect(tr transport.Transport) error
	// Heartbeat reads current printer status (battery, lid, paper).
	Heartbeat(tr transport.Transport) (HeartbeatResult, error)
	// RfidInfo reads tape/label RFID data.
	RfidInfo(tr transport.Transport) (RfidResult, error)
}

// HeartbeatResult contains data from a heartbeat response.
type HeartbeatResult struct {
	Battery     int
	LidClosed   bool
	PaperLoaded bool
}

// RfidResult contains tape/label info from RFID tag.
type RfidResult struct {
	LabelType   string
	TotalLabels int
	UsedLabels  int
}
