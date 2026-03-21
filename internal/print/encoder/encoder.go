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
