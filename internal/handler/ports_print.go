package handler

import (
	"image"

	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/store"
)

type labelPrinter interface {
	Print(printerID string, data label.LabelData, templateName string, renderOpts label.RenderOpts, printOpts encoder.PrintOpts) error
	PrintImage(printerID string, img image.Image, printOpts encoder.PrintOpts) error
}

type printerCatalog interface {
	ConnectedPrinters() []store.PrinterConfig
	AvailableEncoders() map[string]encoder.Encoder
	Encoder(name string) encoder.Encoder
}

type printerStatusProvider interface {
	GetStatus(printerID string) print.PrinterStatus
	AllStatuses() map[string]print.PrinterStatus
}

type printerConnections interface {
	Add(cfg store.PrinterConfig) error
	Remove(printerID string) error
	Reconnect(printerID string) error
	StateInfo(printerID string) (print.ConnState, string)
}

type stateSubscriber interface {
	Subscribe() <-chan print.StateChange
	Unsubscribe(ch <-chan print.StateChange)
}

type printHandlerPrinterDeps interface {
	labelPrinter
	printerCatalog
	printerStatusProvider
}

type printHandlerConnectionDeps interface {
	printerConnections
	stateSubscriber
}

type connectedPrinterProvider interface {
	ConnectedPrinters() []store.PrinterConfig
}

type encoderCatalog interface {
	AvailableEncoders() map[string]encoder.Encoder
	Encoder(name string) encoder.Encoder
}

type debugPrinterDeps interface {
	labelPrinter
	encoderCatalog
	printerStatusProvider
}
