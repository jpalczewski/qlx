package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"strings"

	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

// PrintHandler handles HTTP requests for printer management and print operations.
type PrintHandler struct {
	pm        *print.PrinterManager
	inventory *service.InventoryService
	printers  *service.PrinterService
	templates *service.TemplateService
	resp      Responder
}

// NewPrintHandler creates a new PrintHandler.
func NewPrintHandler(pm *print.PrinterManager, inv *service.InventoryService,
	prn *service.PrinterService, tmpl *service.TemplateService, resp Responder) *PrintHandler {
	return &PrintHandler{pm: pm, inventory: inv, printers: prn, templates: tmpl, resp: resp}
}

// RegisterRoutes registers printer and print routes on the given mux.
func (h *PrintHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /printers", h.ListPrinters)
	mux.HandleFunc("POST /printers", h.CreatePrinter)
	mux.HandleFunc("DELETE /printers/{id}", h.DeletePrinter)
	mux.HandleFunc("GET /encoders", h.ListEncoders)
	mux.HandleFunc("POST /items/{id}/print", h.PrintItem)
	mux.HandleFunc("POST /print-image", h.PrintImage)
	mux.HandleFunc("GET /printers/status", h.AllStatuses)
	mux.HandleFunc("GET /printers/{id}/status", h.Status)
	mux.HandleFunc("POST /printers/{id}/connect", h.Connect)
	mux.HandleFunc("POST /printers/{id}/disconnect", h.Disconnect)
	mux.HandleFunc("GET /printers/events", h.Events)
}

// ListPrinters handles GET /printers.
func (h *PrintHandler) ListPrinters(w http.ResponseWriter, r *http.Request) {
	printers := h.printers.AllPrinters()

	h.resp.Respond(w, r, http.StatusOK, printers, "printers", func() any {
		return h.printersVM()
	})
}

// CreatePrinter handles POST /printers.
func (h *PrintHandler) CreatePrinter(w http.ResponseWriter, r *http.Request) {
	var req AddPrinterRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	printer, err := h.printers.AddPrinter(req.Name, req.Encoder, req.Model, req.Transport, req.Address)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusCreated, printer, "printers", func() any {
		return h.printersVM()
	})
}

// DeletePrinter handles DELETE /printers/{id}.
func (h *PrintHandler) DeletePrinter(w http.ResponseWriter, r *http.Request) {
	if err := h.printers.DeletePrinter(r.PathValue("id")); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, map[string]bool{"deleted": true}, "printers", func() any {
		return h.printersVM()
	})
}

// ListEncoders handles GET /encoders.
func (h *PrintHandler) ListEncoders(w http.ResponseWriter, r *http.Request) {
	type encoderInfo struct {
		Name   string              `json:"name"`
		Models []map[string]string `json:"models"`
	}

	var result []encoderInfo
	for name, enc := range h.pm.AvailableEncoders() {
		info := encoderInfo{Name: name}
		for _, m := range enc.Models() {
			info.Models = append(info.Models, map[string]string{
				"id":   m.ID,
				"name": m.Name,
			})
		}
		result = append(result, info)
	}

	h.resp.Respond(w, r, http.StatusOK, result, "printers", func() any {
		return h.printersVM()
	})
}

// PrintItem handles POST /items/{id}/print.
func (h *PrintHandler) PrintItem(w http.ResponseWriter, r *http.Request) {
	var req PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	item := h.inventory.GetItem(r.PathValue("id"))
	if item == nil {
		http.NotFound(w, r)
		return
	}

	path := h.inventory.ContainerPath(item.ContainerID)
	data := label.LabelData{
		Name:        item.Name,
		Description: item.Description,
		Location:    webutil.FormatContainerPath(path, " \u2192 "),
		QRContent:   "/item/" + item.ID,
		BarcodeID:   item.ID,
	}

	// Check if this is a legacy template or designer template
	switch req.Template {
	case "simple", "standard", "compact", "detailed":
		if err := h.pm.Print(req.PrinterID, data, req.Template, label.RenderOpts{}); err != nil {
			webutil.LogError("print failed: %v", err)
			webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		tmpl := h.templates.GetTemplate(req.Template)
		if tmpl == nil {
			webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
			return
		}
		webutil.JSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"render":    "client",
			"template":  tmpl,
			"item_data": data,
		})
	}
}

// PrintImage handles POST /print-image.
func (h *PrintHandler) PrintImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PrinterID string `json:"printer_id"`
		PNG       string `json:"png"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Decode base64 PNG (format: "data:image/png;base64,XXXX")
	parts := strings.SplitN(req.PNG, ",", 2)
	if len(parts) != 2 {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid PNG data"})
		return
	}
	imgData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "base64 decode: " + err.Error()})
		return
	}

	img, err := png.Decode(bytes.NewReader(imgData))
	if err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "png decode: " + err.Error()})
		return
	}

	if err := h.pm.PrintImage(req.PrinterID, img); err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// AllStatuses handles GET /printers/status (JSON only).
func (h *PrintHandler) AllStatuses(w http.ResponseWriter, _ *http.Request) {
	webutil.JSON(w, http.StatusOK, h.pm.AllStatuses())
}

// Status handles GET /printers/{id}/status (JSON only).
func (h *PrintHandler) Status(w http.ResponseWriter, r *http.Request) {
	webutil.JSON(w, http.StatusOK, h.pm.GetStatus(r.PathValue("id")))
}

// Connect handles POST /printers/{id}/connect (JSON only).
func (h *PrintHandler) Connect(w http.ResponseWriter, r *http.Request) {
	if err := h.pm.ConnectPrinter(r.PathValue("id")); err != nil {
		webutil.LogError("connect printer: %v", err)
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Disconnect handles POST /printers/{id}/disconnect (JSON only).
func (h *PrintHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	h.pm.DisconnectPrinter(r.PathValue("id"))
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Events handles GET /printers/events (SSE stream, no content negotiation).
func (h *PrintHandler) Events(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := h.pm.SubscribeSSE()
	defer h.pm.UnsubscribeSSE(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case evt := <-ch:
			data, _ := json.Marshal(evt)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// printersVM builds the view model for the printers page.
func (h *PrintHandler) printersVM() PrintersData {
	printersList := h.printers.AllPrinters()
	var encoders []EncoderData
	for name, enc := range h.pm.AvailableEncoders() {
		encoders = append(encoders, EncoderData{
			Name:   name,
			Models: enc.Models(),
		})
	}
	return PrintersData{
		Printers: printersList,
		Encoders: encoders,
	}
}
