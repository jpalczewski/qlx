package brother

type labelDef struct { //nolint:unused // label dimension table, reserved for future media detection
	ID           string
	TapeWidthMm  int
	TapeLengthMm int
	DotsPrintW   int
	DotsPrintL   int
	OffsetR      int
	FeedMargin   int
	MediaType    byte
}

const (
	mediaContinuous byte = 0x0A
	mediaDieCut     byte = 0x0B
)

var allLabels = []labelDef{ //nolint:unused // label dimension table, reserved for future media detection
	{"62", 62, 0, 696, 0, 12, 35, mediaContinuous},
	{"29", 29, 0, 306, 0, 6, 35, mediaContinuous},
	{"29x90", 29, 90, 306, 991, 6, 0, mediaDieCut},
	{"62x29", 62, 29, 696, 271, 12, 0, mediaDieCut},
	{"62x100", 62, 100, 696, 1109, 12, 0, mediaDieCut},
}
