package encoder

import (
	"context"
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
	Density  int  `json:"density"`   // 0 = model default; Niimbot 1-5, Brother ignored
	Copies   int  `json:"copies"`    // 0/1 = single; >1 = multi-copy
	CutEvery int  `json:"cut_every"` // 0 = no cut; 1 = every copy; N = every N copies
	HighRes  bool `json:"high_res"`  // Brother: 600 DPI vertical; others: ignored
}

// StatusQuerier is optionally implemented by encoders that support status queries.
type StatusQuerier interface {
	// Connect sends initial handshake.
	Connect(ctx context.Context, tr transport.Transport) error
	// Heartbeat reads current printer status (battery, lid, paper).
	Heartbeat(tr transport.Transport) (HeartbeatResult, error)
	// RfidInfo reads tape/label RFID data.
	RfidInfo(ctx context.Context, tr transport.Transport) (RfidResult, error)
}

// HeartbeatResult contains data from a heartbeat response.
type HeartbeatResult struct {
	Battery     int
	LidClosed   bool
	PaperLoaded bool
}

// RfidResult contains tape/label info from RFID tag.
type RfidResult struct {
	LabelType     string
	TotalLabels   int
	UsedLabels    int
	Barcode       string // EAN barcode from RFID (for cloud size lookup)
	LabelWidthMm  int    // 0 if unknown
	LabelHeightMm int    // 0 if unknown
}
