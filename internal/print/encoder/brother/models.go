package brother

import "github.com/erxyi/qlx/internal/print/encoder"

type qlModel struct {
	ID              string
	Name            string
	BytesPerRow     int
	MinLengthDots   int
	MaxLengthDots   int
	Compression     bool
	ModeSwitching   bool
	Cutting         bool
}

var ql700 = qlModel{
	ID: "QL-700", Name: "Brother QL-700",
	BytesPerRow: 90, MinLengthDots: 150, MaxLengthDots: 11811,
	Compression: false, ModeSwitching: false, Cutting: true,
}

var allModels = []qlModel{ql700}

func modelInfo(m qlModel) encoder.ModelInfo {
	return encoder.ModelInfo{
		ID: m.ID, Name: m.Name, DPI: 300,
		PrintWidthPx: m.BytesPerRow * 8,
		MediaTypes: []string{"endless", "die-cut"},
		DensityRange: [2]int{1, 1}, DensityDefault: 1,
	}
}
