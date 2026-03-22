package handler

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"strconv"

	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// DebugHandler handles debug tool pages.
type DebugHandler struct {
	pm       *print.PrinterManager
	printers *service.PrinterService
	resp     Responder
}

// NewDebugHandler creates a new DebugHandler.
func NewDebugHandler(pm *print.PrinterManager, prn *service.PrinterService, resp Responder) *DebugHandler {
	return &DebugHandler{pm: pm, printers: prn, resp: resp}
}

// RegisterRoutes registers debug tool routes.
func (h *DebugHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /debug/tools", h.Page)
	mux.HandleFunc("GET /debug/calibration.png", h.CalibrationImage)
	mux.HandleFunc("POST /debug/calibration/print", h.PrintCalibration)
	mux.HandleFunc("GET /debug/printer-info", h.PrinterInfo)
	mux.HandleFunc("POST /debug/calibration/offset", h.SetOffset)
}

// Page handles GET /debug/tools.
func (h *DebugHandler) Page(w http.ResponseWriter, r *http.Request) {
	vm := DebugToolsData{
		Printers: h.printers.AllPrinters(),
		Schemas:  label.SchemaNames(),
		Fonts:    label.FontNames(),
	}
	h.resp.Respond(w, r, http.StatusOK, vm, "debug-tools", func() any { return vm })
}

// PrinterInfo handles GET /debug/printer-info?id=... — returns printer status + model info.
func (h *DebugHandler) PrinterInfo(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "missing printer id"})
		return
	}

	// Find printer config from list
	var cfg *store.PrinterConfig
	for _, p := range h.printers.AllPrinters() {
		if p.ID == id {
			found := p
			cfg = &found
			break
		}
	}
	if cfg == nil {
		webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "printer not found"})
		return
	}

	status := h.pm.GetStatus(id)

	// Get model info for printhead width
	var printheadPx int
	var dpi int
	for _, enc := range h.pm.AvailableEncoders() {
		for _, m := range enc.Models() {
			if m.ID == cfg.Model {
				printheadPx = m.PrintWidthPx
				dpi = m.DPI
			}
		}
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"printer":         cfg,
		"status":          status,
		"printhead_px":    printheadPx,
		"dpi":             dpi,
		"label_width_mm":  status.LabelWidthMm,
		"label_height_mm": status.LabelHeightMm,
		"offset_x":        cfg.OffsetX,
		"offset_y":        cfg.OffsetY,
	})
}

// CalibrationImage handles GET /debug/calibration.png — returns a calibration grid PNG.
func (h *DebugHandler) CalibrationImage(w http.ResponseWriter, r *http.Request) {
	widthPx := queryInt(r, "w", 384)
	heightPx := queryInt(r, "h", 240)
	if widthPx < 50 || widthPx > 1000 {
		widthPx = 384
	}
	if heightPx < 50 || heightPx > 1000 {
		heightPx = 240
	}

	widthMm := queryInt(r, "wmm", 0)
	heightMm := queryInt(r, "hmm", 0)

	img := renderCalibrationGrid(widthPx, heightPx, widthMm, heightMm)

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	if err := png.Encode(w, img); err != nil {
		webutil.LogError("calibration png encode: %v", err)
	}
}

// PrintCalibration handles POST /debug/calibration/print — prints the calibration grid.
// Always produces a 384px-wide image (B1 printhead width) with the grid centered.
func (h *DebugHandler) PrintCalibration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PrinterID string `json:"printer_id"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		WidthMm   int    `json:"width_mm"`
		HeightMm  int    `json:"height_mm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if req.Width <= 0 {
		req.Width = 384
	}
	if req.Height <= 0 {
		req.Height = 160
	}

	// Get printhead width from printer model
	printheadPx := 384
	if req.PrinterID != "" {
		for _, p := range h.printers.AllPrinters() {
			if p.ID == req.PrinterID {
				for _, enc := range h.pm.AvailableEncoders() {
					for _, m := range enc.Models() {
						if m.ID == p.Model {
							printheadPx = m.PrintWidthPx
						}
					}
				}
			}
		}
	}

	// Render grid at requested size
	grid := renderCalibrationGrid(req.Width, req.Height, req.WidthMm, req.HeightMm)

	// Place grid on full-printhead-width canvas (no scaling, white padding)
	var img image.Image
	if req.Width < printheadPx {
		canvas := image.NewRGBA(image.Rect(0, 0, printheadPx, req.Height))
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		// Center the grid horizontally
		offsetX := (printheadPx - req.Width) / 2
		draw.Draw(canvas, image.Rect(offsetX, 0, offsetX+req.Width, req.Height), grid, image.Point{}, draw.Src)
		img = canvas
	} else {
		img = grid
	}

	if err := h.pm.PrintImage(req.PrinterID, img); err != nil {
		webutil.LogError("calibration print failed: %v", err)
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// SetOffset handles POST /debug/calibration/offset — saves calibration offsets for a printer.
func (h *DebugHandler) SetOffset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PrinterID string `json:"printer_id"`
		OffsetX   int    `json:"offset_x"`
		OffsetY   int    `json:"offset_y"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if err := h.printers.UpdateOffset(req.PrinterID, req.OffsetX, req.OffsetY); err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	webutil.LogInfo("calibration offset set for %s: (%+d, %+d)", req.PrinterID, req.OffsetX, req.OffsetY)
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// renderCalibrationGrid produces a calibration image with grid lines, border, and dimension labels.
func renderCalibrationGrid(widthPx, heightPx, widthMm, heightMm int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, widthPx, heightPx))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	black := color.RGBA{0, 0, 0, 255}
	gray := color.RGBA{0, 0, 0, 255}         // 100px lines: full black (thermal printers need high contrast)
	lightGray := color.RGBA{80, 80, 80, 255} // 20px lines: dark gray (visible on thermal)

	bw := 6 // border width (thick for thermal visibility)

	// Draw thick border
	fillRect(img, 0, 0, widthPx, bw, black)                 // top
	fillRect(img, 0, heightPx-bw, widthPx, heightPx, black) // bottom
	fillRect(img, 0, 0, bw, heightPx, black)                // left
	fillRect(img, widthPx-bw, 0, widthPx, heightPx, black)  // right

	// Grid lines: every 20px (2px wide), every 100px (4px wide, darker)
	for x := 20; x < widthPx-bw; x += 20 {
		thick := x%100 == 0
		c := lightGray
		lw := 2
		if thick {
			c = gray
			lw = 4
		}
		fillRect(img, x-lw/2, bw, x-lw/2+lw, heightPx-bw, c)
	}
	for y := 20; y < heightPx-bw; y += 20 {
		thick := y%100 == 0
		c := lightGray
		lw := 2
		if thick {
			c = gray
			lw = 4
		}
		fillRect(img, bw, y-lw/2, widthPx-bw, y-lw/2+lw, c)
	}

	// Crosshair at center (5px thick, 40px arms)
	cx, cy := widthPx/2, heightPx/2
	fillRect(img, cx-25, cy-2, cx+25, cy+3, black)
	fillRect(img, cx-2, cy-25, cx+3, cy+25, black)

	// Corner markers (thick L-shapes, 20px)
	for i := 0; i < 20; i++ {
		for t := 0; t < 3; t++ {
			// top-left
			img.Set(bw+i, bw+t, black)
			img.Set(bw+t, bw+i, black)
			// top-right
			img.Set(widthPx-bw-1-i, bw+t, black)
			img.Set(widthPx-bw-1-t, bw+i, black)
			// bottom-left
			img.Set(bw+i, heightPx-bw-1-t, black)
			img.Set(bw+t, heightPx-bw-1-i, black)
			// bottom-right
			img.Set(widthPx-bw-1-i, heightPx-bw-1-t, black)
			img.Set(widthPx-bw-1-t, heightPx-bw-1-i, black)
		}
	}

	// Dimension text
	face, err := label.LoadFace("basic", 13)
	if err == nil {
		dimText := fmt.Sprintf("%dx%d px", widthPx, heightPx)
		if widthMm > 0 && heightMm > 0 {
			dimText = fmt.Sprintf("%dx%d px (%dx%d mm)", widthPx, heightPx, widthMm, heightMm)
		}
		drawDebugText(img, bw+6, bw+18, dimText, black, face)
		drawDebugText(img, bw+6, heightPx-bw-6, "QLX calibration", black, face)
	}

	return img
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.Set(x, y, c)
		}
	}
}

func drawCornerMarker(img *image.RGBA, x, y, dx, dy int, c color.RGBA) {
	for i := 0; i < 10; i++ {
		img.Set(x+i*dx, y, c)
		img.Set(x, y+i*dy, c)
	}
}

func queryInt(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func drawDebugText(img *image.RGBA, x, y int, text string, col color.Color, face font.Face) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}
