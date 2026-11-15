package print

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	imagedraw "image/draw"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
	"golang.org/x/image/draw"
)

// PrinterConfigStore provides read-only access to printer configuration.
type PrinterConfigStore interface {
	GetPrinter(id string) *store.PrinterConfig
	AllPrinters() []store.PrinterConfig
}

// PrinterManager manages persistent printer sessions with heartbeat.
type PrinterManager struct {
	store    PrinterConfigStore
	encoders map[string]encoder.Encoder
	cm       *ConnectionManager
	renderer DocumentRenderer
}

// NewPrinterManager creates a PrinterManager backed by the given store and ConnectionManager.
func NewPrinterManager(s PrinterConfigStore, cm *ConnectionManager) *PrinterManager {
	return &PrinterManager{
		store:    s,
		cm:       cm,
		encoders: make(map[string]encoder.Encoder),
		renderer: labelDocumentRenderer{},
	}
}

// DocumentRenderer renders a label document for execution. It returns one
// image per label of the document (two for the paired template).
type DocumentRenderer interface {
	Render(data label.LabelData, template string, media label.MediaInfo, opts label.RenderOpts) ([]image.Image, error)
}

type labelDocumentRenderer struct{}

func (labelDocumentRenderer) Render(data label.LabelData, template string, media label.MediaInfo, opts label.RenderOpts) ([]image.Image, error) {
	return label.RenderPages(data, template, media, opts)
}

// RenderError reports that a label could not be rendered for the printer's
// media (for example the paired template on unsupported media). Nothing was
// sent to the printer, so callers may retry with another template. Detect it
// with errors.As.
type RenderError struct{ Err error }

// Error implements error.
func (e *RenderError) Error() string { return "render: " + e.Err.Error() }

// Unwrap returns the underlying render error.
func (e *RenderError) Unwrap() error { return e.Err }

// DefaultTransportFactory returns a TransportFactoryFn that creates transports
// by name from the transport registry.
func DefaultTransportFactory() TransportFactoryFn {
	return transport.New
}

// FindModel returns the ModelInfo matching modelID from an encoder, or nil if not found.
func FindModel(enc encoder.Encoder, modelID string) *encoder.ModelInfo {
	for _, mi := range enc.Models() {
		if mi.ID == modelID {
			info := mi
			return &info
		}
	}
	return nil
}

// PxFromMm converts millimeters to pixels at the given DPI. Returns 0 if either is <= 0.
func PxFromMm(mm, dpi int) int {
	if mm <= 0 || dpi <= 0 {
		return 0
	}
	return int(float64(mm) * float64(dpi) / 25.4)
}

// MediaForStatus returns the render geometry for the media currently loaded in
// the printer. Encoders implementing encoder.MediaGeometry report the printable
// area of the loaded media; for all others the image is as wide as the printhead
// and as tall as the media length.
func MediaForStatus(enc encoder.Encoder, model *encoder.ModelInfo, st PrinterStatus) label.MediaInfo {
	media := label.MediaInfo{
		WidthPx:  model.PrintWidthPx,
		HeightPx: PxFromMm(st.LabelHeightMm, model.DPI),
		DPI:      model.DPI,
	}
	if g, ok := enc.(encoder.MediaGeometry); ok {
		if w, h, found := g.PrintableArea(st.LabelType, st.LabelWidthMm, st.LabelHeightMm); found {
			media.WidthPx, media.HeightPx = w, h
		}
	}
	return media
}

// validatePrintOpts clamps values to valid ranges and applies defaults.
func validatePrintOpts(opts encoder.PrintOpts, mi *encoder.ModelInfo) encoder.PrintOpts {
	if opts.Density == 0 {
		opts.Density = mi.DensityDefault
	}
	// Only clamp if range is set (non-zero bounds indicate density control)
	if mi.DensityRange[0] != 0 || mi.DensityRange[1] != 0 {
		if opts.Density < mi.DensityRange[0] {
			opts.Density = mi.DensityRange[0]
		}
		if opts.Density > mi.DensityRange[1] {
			opts.Density = mi.DensityRange[1]
		}
	}
	if opts.Copies < 1 {
		opts.Copies = 1
	}
	if opts.Copies > 100 {
		opts.Copies = 100
	}
	if opts.CutEvery < 0 {
		opts.CutEvery = 0
	}
	if opts.CutEvery > opts.Copies {
		opts.CutEvery = opts.Copies
	}
	return opts
}

// printContext holds resolved printer resources for a print operation.
type printContext struct {
	cfg       *store.PrinterConfig
	model     *encoder.ModelInfo
	session   *PrinterSession
	media     label.MediaInfo
	printOpts encoder.PrintOpts
}

// resolveForPrint looks up printer config, encoder, model, session, and validates print options.
func (m *PrinterManager) resolveForPrint(printerID string, opts encoder.PrintOpts) (*printContext, error) {
	cfg := m.store.GetPrinter(printerID)
	if cfg == nil {
		return nil, fmt.Errorf("printer not found: %s", printerID)
	}

	enc, ok := m.encoders[cfg.Encoder]
	if !ok {
		return nil, fmt.Errorf("encoder not found: %s", cfg.Encoder)
	}

	modelInfo := FindModel(enc, cfg.Model)
	if modelInfo == nil {
		return nil, fmt.Errorf("model not found: %s", cfg.Model)
	}

	if m.cm == nil {
		return nil, fmt.Errorf("printer %s not connected (no connection manager)", printerID)
	}
	if m.cm.State(printerID) != StateConnected {
		return nil, fmt.Errorf("printer %s not connected", printerID)
	}
	session := m.cm.Session(printerID)
	if session == nil {
		return nil, fmt.Errorf("printer %s: no active session", printerID)
	}

	validated := validatePrintOpts(opts, modelInfo)

	media := MediaForStatus(enc, modelInfo, session.Status())

	return &printContext{
		cfg:       cfg,
		model:     modelInfo,
		session:   session,
		media:     media,
		printOpts: validated,
	}, nil
}

// RegisterEncoder adds an encoder. Must be called before use.
func (m *PrinterManager) RegisterEncoder(enc encoder.Encoder) {
	m.encoders[enc.Name()] = enc
}

// SetConnectionManager wires the ConnectionManager after construction.
// Used when CM needs PM's encoder lookup (breaks the initialization cycle).
func (m *PrinterManager) SetConnectionManager(cm *ConnectionManager) {
	m.cm = cm
}

// Encoder returns a registered encoder by name, or nil if not found.
func (m *PrinterManager) Encoder(name string) encoder.Encoder {
	return m.encoders[name]
}

// ConnectedPrinters returns printers in StateConnected.
// Falls back to all configured printers when none are connected, so the
// print form is always rendered when printers are registered.
func (m *PrinterManager) ConnectedPrinters() []store.PrinterConfig {
	all := m.store.AllPrinters()
	if m.cm == nil {
		return all
	}
	var out []store.PrinterConfig
	for _, p := range all {
		if m.cm.State(p.ID) == StateConnected {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return all
	}
	return out
}

// AvailableEncoders returns a snapshot of registered encoders.
func (m *PrinterManager) AvailableEncoders() map[string]encoder.Encoder {
	cp := make(map[string]encoder.Encoder, len(m.encoders))
	for k, v := range m.encoders {
		cp[k] = v
	}
	return cp
}

// GetStatus returns status for a single printer.
func (m *PrinterManager) GetStatus(printerID string) PrinterStatus {
	if m.cm == nil {
		return PrinterStatus{Battery: -1, TotalLabels: -1, UsedLabels: -1}
	}
	session := m.cm.Session(printerID)
	if session == nil {
		return PrinterStatus{Battery: -1, TotalLabels: -1, UsedLabels: -1}
	}
	return session.Status()
}

// AllStatuses returns statuses for all managed printers.
func (m *PrinterManager) AllStatuses() map[string]PrinterStatus {
	result := make(map[string]PrinterStatus)
	if m.cm == nil {
		return result
	}
	for id, state := range m.cm.States() {
		if state == StateConnected {
			if session := m.cm.Session(id); session != nil {
				result[id] = session.Status()
				continue
			}
		}
		result[id] = PrinterStatus{
			Connected:   state == StateConnected,
			Battery:     -1,
			TotalLabels: -1,
			UsedLabels:  -1,
		}
	}
	return result
}

// Print renders a label and sends it to the printer.
func (m *PrinterManager) Print(printerID string, data label.LabelData, templateName string, renderOpts label.RenderOpts, printOpts encoder.PrintOpts) error {
	return m.withLinkRetry(printerID, func() (*PrinterSession, error) {
		return m.printOnce(printerID, data, templateName, renderOpts, printOpts)
	})
}

// reconnectWait bounds how long a print waits for the ConnectionManager to
// re-establish a session after the link failed. A var so tests can shorten it.
var reconnectWait = 15 * time.Second

// withLinkRetry runs attempt and, if it failed with a retryable *LinkError
// (the link broke before any page data reached the device) or with
// ErrPrinterDisconnected (a session that ended between lookup and use), waits
// for the reconnected session and runs attempt once more. Other errors, and
// link errors after page data may have been consumed, are returned as is: a
// repeat could print a duplicate label.
func (m *PrinterManager) withLinkRetry(printerID string, attempt func() (*PrinterSession, error)) error {
	used, err := attempt()
	var le *LinkError
	if err == nil {
		return err
	}
	if retryable := errors.As(err, &le) && le.Retryable; !retryable && !errors.Is(err, ErrPrinterDisconnected) {
		return err
	}
	webutil.LogError("printer %s: link failed before any page data was sent, retrying once on a new session: %v", printerID, err)
	if !m.awaitNewSession(printerID, used) {
		return err
	}
	_, err2 := attempt()
	if err2 != nil {
		return err2
	}
	return nil
}

// awaitNewSession waits until the printer has a connected session other than
// old, up to reconnectWait or until the ConnectionManager is stopping.
func (m *PrinterManager) awaitNewSession(printerID string, old *PrinterSession) bool {
	if m.cm == nil {
		return false
	}
	var done <-chan struct{}
	if m.cm.ctx != nil {
		done = m.cm.ctx.Done()
	}
	deadline := time.NewTimer(reconnectWait)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		if m.cm.State(printerID) == StateConnected {
			if sess := m.cm.Session(printerID); sess != nil && sess != old && !sess.closed.Load() {
				return true
			}
		}
		select {
		case <-deadline.C:
			return false
		case <-done:
			return false
		case <-tick.C:
		}
	}
}

// printOnce resolves the printer, renders and prints once. It returns the
// session it used (nil if it never got that far).
func (m *PrinterManager) printOnce(printerID string, data label.LabelData, templateName string, renderOpts label.RenderOpts, printOpts encoder.PrintOpts) (*PrinterSession, error) {
	ctx, err := m.resolveForPrint(printerID, printOpts)
	if err != nil {
		return nil, err
	}

	renderer := m.renderer
	if renderer == nil {
		renderer = labelDocumentRenderer{}
	}
	pages, err := renderer.Render(data, templateName, ctx.media, renderOpts)
	if err != nil {
		return ctx.session, &RenderError{Err: err}
	}
	for i, pg := range pages {
		pages[i] = applyCalibrationOffset(pg, ctx.cfg, ctx.media.WidthPx)
	}

	webutil.LogInfo("printing on %s (%s/%s): %d page(s) x %d", ctx.cfg.Name, ctx.cfg.Encoder, ctx.cfg.Model, len(pages), ctx.printOpts.Copies)
	if err := ctx.session.PrintPages(pages, ctx.cfg.Model, ctx.printOpts); err != nil {
		return ctx.session, fmt.Errorf("encode: %w", err)
	}

	webutil.LogInfo("print complete on %s", ctx.cfg.Name)
	return ctx.session, nil
}

// PrintImage sends a pre-rendered image directly to the printer, bypassing label rendering.
func (m *PrinterManager) PrintImage(printerID string, img image.Image, printOpts encoder.PrintOpts) error {
	return m.withLinkRetry(printerID, func() (*PrinterSession, error) {
		return m.printImageOnce(printerID, img, printOpts)
	})
}

func (m *PrinterManager) printImageOnce(printerID string, img image.Image, printOpts encoder.PrintOpts) (*PrinterSession, error) {
	ctx, err := m.resolveForPrint(printerID, printOpts)
	if err != nil {
		return nil, err
	}

	// Pre-rendered images are laid out for the full printhead row (the encoder
	// places them on the media), so scale to the printhead width, not to the
	// printable width of the loaded media.
	headPx := ctx.model.PrintWidthPx
	imgWidth := img.Bounds().Dx()
	if imgWidth != headPx && headPx > 0 {
		scale := float64(headPx) / float64(imgWidth)
		newH := int(float64(img.Bounds().Dy()) * scale)
		dst := image.NewRGBA(image.Rect(0, 0, headPx, newH))
		imagedraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, imagedraw.Src)
		draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
		img = dst
		webutil.LogInfo("scaled image %d→%dpx width for %s", imgWidth, headPx, ctx.cfg.Model)
	}

	img = applyCalibrationOffset(img, ctx.cfg, headPx)

	webutil.LogInfo("printing image on %s (%s/%s)", ctx.cfg.Name, ctx.cfg.Encoder, ctx.cfg.Model)
	if err := ctx.session.Print(img, ctx.cfg.Model, ctx.printOpts); err != nil {
		return ctx.session, fmt.Errorf("encode: %w", err)
	}

	webutil.LogInfo("print image complete on %s", ctx.cfg.Name)
	return ctx.session, nil
}

// applyCalibrationOffset shifts the image content to compensate for printhead misalignment.
// Positive X = content moves right on paper, negative X = content moves left.
// The output image keeps the same dimensions (printhead width × original height).
// Content that shifts outside the canvas is clipped; exposed areas are white.
func applyCalibrationOffset(img image.Image, cfg *store.PrinterConfig, printheadPx int) image.Image {
	if cfg.OffsetX == 0 && cfg.OffsetY == 0 {
		return img
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	outW := w
	if printheadPx > 0 {
		outW = printheadPx
	}
	outH := h

	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	imagedraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, imagedraw.Src)

	// Source point: where in the original image to start reading.
	// If offset is negative (shift left), we skip pixels from the left of the source.
	srcPt := bounds.Min
	dstX := cfg.OffsetX
	if dstX < 0 {
		srcPt.X -= dstX // skip -dstX pixels from source left
		dstX = 0
	}
	dstY := cfg.OffsetY
	if dstY < 0 {
		srcPt.Y -= dstY
		dstY = 0
	}

	dstRect := image.Rect(dstX, dstY, dstX+w, dstY+h)
	imagedraw.Draw(dst, dstRect, img, srcPt, imagedraw.Src)

	webutil.LogInfo("applied calibration offset (%+d, %+d) for %s", cfg.OffsetX, cfg.OffsetY, cfg.Name)
	return dst
}
