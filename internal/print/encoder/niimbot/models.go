package niimbot

import "github.com/erxyi/qlx/internal/print/encoder"

type niimbotModel struct {
	ID             string
	Name           string
	DPI            int
	PrintheadPx    int
	DensityMin     int
	DensityMax     int
	DensityDefault int
}

var b1 = niimbotModel{
	ID: "B1", Name: "Niimbot B1", DPI: 203,
	PrintheadPx: 384, DensityMin: 1, DensityMax: 5, DensityDefault: 3,
}

var allModels = []niimbotModel{b1}

func modelInfo(m niimbotModel) encoder.ModelInfo {
	return encoder.ModelInfo{
		ID: m.ID, Name: m.Name, DPI: m.DPI,
		PrintWidthPx: m.PrintheadPx,
		MediaTypes: []string{"with-gaps", "black", "transparent"},
		DensityRange: [2]int{m.DensityMin, m.DensityMax},
		DensityDefault: m.DensityDefault,
	}
}
