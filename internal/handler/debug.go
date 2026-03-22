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
		req.Height = 240
	}

	img := renderCalibrationGrid(req.Width, req.Height, req.WidthMm, req.HeightMm)

	if err := h.pm.PrintImage(req.PrinterID, img); err != nil {
		webutil.LogError("calibration print failed: %v", err)
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// renderCalibrationGrid produces a calibration image with grid lines, border, and dimension labels.
func renderCalibrationGrid(widthPx, heightPx, widthMm, heightMm int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, widthPx, heightPx))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	black := color.RGBA{0, 0, 0, 255}
	gray := color.RGBA{180, 180, 180, 255}
	lightGray := color.RGBA{220, 220, 220, 255}

	// Draw border (2px)
	for x := 0; x < widthPx; x++ {
		img.Set(x, 0, black)
		img.Set(x, 1, black)
		img.Set(x, heightPx-1, black)
		img.Set(x, heightPx-2, black)
	}
	for y := 0; y < heightPx; y++ {
		img.Set(0, y, black)
		img.Set(1, y, black)
		img.Set(widthPx-1, y, black)
		img.Set(widthPx-2, y, black)
	}

	// Draw grid lines every 10px (light) and every 50px (darker)
	for x := 10; x < widthPx; x += 10 {
		c := lightGray
		if x%50 == 0 {
			c = gray
		}
		for y := 2; y < heightPx-2; y++ {
			img.Set(x, y, c)
		}
	}
	for y := 10; y < heightPx; y += 10 {
		c := lightGray
		if y%50 == 0 {
			c = gray
		}
		for x := 2; x < widthPx-2; x++ {
			img.Set(x, y, c)
		}
	}

	// Draw crosshair at center
	cx, cy := widthPx/2, heightPx/2
	for i := -15; i <= 15; i++ {
		if cx+i >= 0 && cx+i < widthPx {
			img.Set(cx+i, cy, black)
		}
		if cy+i >= 0 && cy+i < heightPx {
			img.Set(cx, cy+i, black)
		}
	}

	// Draw corner markers (10px L-shapes)
	drawCornerMarker(img, 2, 2, 1, 1, black)
	drawCornerMarker(img, widthPx-3, 2, -1, 1, black)
	drawCornerMarker(img, 2, heightPx-3, 1, -1, black)
	drawCornerMarker(img, widthPx-3, heightPx-3, -1, -1, black)

	// Draw dimension text
	face, err := label.LoadFace("basic", 13)
	if err == nil {
		dimText := fmt.Sprintf("%dx%d px", widthPx, heightPx)
		if widthMm > 0 && heightMm > 0 {
			dimText = fmt.Sprintf("%dx%d px (%dx%d mm)", widthPx, heightPx, widthMm, heightMm)
		}
		drawDebugText(img, 8, 20, dimText, black, face)
		drawDebugText(img, 8, heightPx-8, "QLX calibration", black, face)
	}

	return img
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
