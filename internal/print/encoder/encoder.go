package encoder

import (
	"context"
	"image"
	"strings"

	"github.com/erxyi/qlx/internal/print/transport"
)

type Encoder interface {
	Name() string
	Models() []ModelInfo
	Encode(img image.Image, model string, opts PrintOpts, tr transport.Transport) error
}

type ModelInfo struct {
	ID               string
	Name             string
	DPI              int
	PrintWidthPx     int
	MediaTypes       []string
	DensityRange     [2]int
	DensityDefault   int
	CutSupported     bool // true if encoder supports cut_every
	HighResSupported bool // true if encoder supports high_res mode
}

type PrintOpts struct {
	Density  int  `json:"density"`   // 0 = model default; Niimbot 1-5, Brother ignored
	Copies   int  `json:"copies"`    // 0/1 = single; >1 = multi-copy
	CutEvery int  `json:"cut_every"` // 0 = no cut; 1 = every copy; N = every N copies
	HighRes  bool `json:"high_res"`  // Brother: 600 DPI vertical; others: ignored
}

// MediaGeometry is optionally implemented by encoders whose printable area
// depends on the media loaded in the printer (for example Brother QL labels,
// where the printable area is smaller than the label itself).
type MediaGeometry interface {
	// PrintableArea returns the printable area in pixels for the loaded media.
	// mediaType is the status label type (e.g. "die-cut", "continuous").
	// heightPx is 0 for continuous media. ok is false when the media is unknown,
	// in which case callers fall back to the printhead width and the media length.
	PrintableArea(mediaType string, widthMm, lengthMm int) (widthPx, heightPx int, ok bool)
}

// StatusError reports a printer condition such as an open cover, missing media
// or a media mismatch. It is not a transport or protocol failure: the printer
// answered and the connection is healthy, so callers should keep the session
// alive, surface the condition and refuse to print until it clears. Detect it
// with errors.As.
type StatusError struct {
	// Source names the encoder that produced the error (e.g. "brother").
	Source string
	// Conditions lists the active conditions in human-readable form.
	Conditions []string
}

// Error implements error, e.g. "brother: cover open, no media".
func (e *StatusError) Error() string {
	list := strings.Join(e.Conditions, ", ")
	if list == "" {
		list = "printer status error"
	}
	if e.Source == "" {
		return list
	}
	return e.Source + ": " + list
}

// Reinitializer is optionally implemented by status-capable encoders that can
// reset the printer's error state (for example Brother ESC @). Sticky error
// bits such as a media mismatch are cleared only that way, so the session
// re-initialises the printer before refusing to print on a StatusError.
type Reinitializer interface {
	// Reinitialize resets the printer to its idle state without a full
	// connect handshake.
	Reinitialize(tr transport.Transport) error
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
