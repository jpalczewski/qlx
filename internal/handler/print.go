package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"strconv"
	"strings"

	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

// PrintHandler handles HTTP requests for printer management and print operations.
type PrintHandler struct {
	pm        *print.PrinterManager
	inventory *service.InventoryService
	printers  *service.PrinterService
	templates *service.TemplateService
	tags      *service.TagService
	resp      Responder
}

// NewPrintHandler creates a new PrintHandler.
func NewPrintHandler(pm *print.PrinterManager, inv *service.InventoryService,
	prn *service.PrinterService, tmpl *service.TemplateService, tags *service.TagService, resp Responder) *PrintHandler {
	return &PrintHandler{pm: pm, inventory: inv, printers: prn, templates: tmpl, tags: tags, resp: resp}
}

// resolveTags converts tag IDs to LabelTag structs with path information.
func (h *PrintHandler) resolveTags(tagIDs []string) []label.LabelTag {
	if h.tags == nil {
		return nil
	}
	var tags []label.LabelTag
	for _, tagID := range tagIDs {
		tagPath := h.tags.TagPath(tagID)
		if len(tagPath) > 0 {
			tag := tagPath[len(tagPath)-1]
			pathNames := make([]string, len(tagPath))
			for i, t := range tagPath {
				pathNames[i] = t.Name
			}
			tags = append(tags, label.LabelTag{Name: tag.Name, Icon: tag.Icon, Path: pathNames})
		}
	}
	return tags
}

// RegisterRoutes registers printer and print routes on the given mux.
func (h *PrintHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /printers", h.ListPrinters)
	mux.HandleFunc("POST /printers", h.CreatePrinter)
	mux.HandleFunc("DELETE /printers/{id}", h.DeletePrinter)
	mux.HandleFunc("GET /encoders", h.ListEncoders)
	mux.HandleFunc("POST /items/{id}/print", h.PrintItem)
	mux.HandleFunc("GET /items/{id}/preview", h.PreviewItem)
	mux.HandleFunc("POST /containers/{id}/print", h.PrintContainer)
	mux.HandleFunc("GET /containers/{id}/preview", h.PreviewContainer)
	mux.HandleFunc("POST /print-image", h.PrintImage)
	mux.HandleFunc("GET /printers/status", h.AllStatuses)
	mux.HandleFunc("GET /printers/{id}/status", h.Status)
	mux.HandleFunc("POST /printers/{id}/connect", h.Connect)
	mux.HandleFunc("POST /printers/{id}/disconnect", h.Disconnect)
	mux.HandleFunc("GET /printers/events", h.Events)
	mux.HandleFunc("GET /printers/scan/usb", h.ScanUSB)
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

// buildItemLabelData builds LabelData for the given item.
func (h *PrintHandler) buildItemLabelData(itemID string) (label.LabelData, bool) {
	item := h.inventory.GetItem(itemID)
	if item == nil {
		return label.LabelData{}, false
	}
	path := h.inventory.ContainerPath(item.ContainerID)
	return label.LabelData{
		Name:        item.Name,
		Description: item.Description,
		Location:    webutil.FormatContainerPath(path, " → "),
		QRContent:   "/items/" + item.ID,
		BarcodeID:   item.ID,
		Icon:        item.Icon,
		Tags:        h.resolveTags(item.TagIDs),
	}, true
}

// buildContainerLabelData builds LabelData for the given container.
func (h *PrintHandler) buildContainerLabelData(containerID string, showChildren bool) (label.LabelData, bool) {
	container := h.inventory.GetContainer(containerID)
	if container == nil {
		return label.LabelData{}, false
	}
	path := h.inventory.ContainerPath(container.ParentID)
	data := label.LabelData{
		Name:        container.Name,
		Description: container.Description,
		Location:    webutil.FormatContainerPath(path, " → "),
		QRContent:   "/containers/" + container.ID,
		BarcodeID:   container.ID,
		Icon:        container.Icon,
		Tags:        h.resolveTags(container.TagIDs),
	}
	if showChildren {
		for _, child := range h.inventory.ContainerChildren(container.ID) {
			data.Children = append(data.Children, label.LabelChild{Name: child.Name, Icon: child.Icon})
		}
		for _, item := range h.inventory.ContainerItems(container.ID) {
			data.Children = append(data.Children, label.LabelChild{Name: item.Name, Icon: item.Icon})
		}
	}
	return data, true
}

// printOrClientRender prints a built-in schema or returns JSON for client-side designer rendering.
func (h *PrintHandler) printOrClientRender(w http.ResponseWriter, data label.LabelData,
	printerID, templateName string, opts label.RenderOpts) {

	if _, ok := label.GetSchema(templateName); ok {
		if err := h.pm.Print(printerID, data, templateName, opts); err != nil {
			webutil.LogError("print failed: %v", err)
			webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	respondClientRender(w, h.templates, templateName, data)
}

// PrintItem handles POST /items/{id}/print.
func (h *PrintHandler) PrintItem(w http.ResponseWriter, r *http.Request) {
	var req PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	data, ok := h.buildItemLabelData(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	h.printOrClientRender(w, data, req.PrinterID, req.Template, label.RenderOpts{PrintDate: req.PrintDate})
}

// previewWidth extracts the width query parameter, defaulting to 384 (Niimbot B1).
func previewWidth(r *http.Request) int {
	if w, err := strconv.Atoi(r.URL.Query().Get("width")); err == nil && w > 0 {
		return w
	}
	return 384
}

// renderPreview renders a label preview and writes the response.
// For built-in schemas it returns a PNG image; for designer templates it returns JSON.
func renderPreview(w http.ResponseWriter, templates *service.TemplateService,
	data label.LabelData, templateName string, widthPx int, opts label.RenderOpts) {

	if _, ok := label.GetSchema(templateName); ok {
		img, err := label.Render(data, templateName, widthPx, 203, opts)
		if err != nil {
			webutil.LogError("preview render failed: %v", err)
			webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writePNG(w, img)
		return
	}

	respondClientRender(w, templates, templateName, data)
}

// writePNG encodes an image as PNG and writes it to the response.
func writePNG(w http.ResponseWriter, img image.Image) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": "png encode: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := w.Write(buf.Bytes()); err != nil {
		webutil.LogError("preview write failed: %v", err)
	}
}

// respondClientRender looks up a designer template and returns it as JSON for client-side rendering.
func respondClientRender(w http.ResponseWriter, templates *service.TemplateService,
	templateName string, data label.LabelData) {

	tmpl := templates.GetTemplate(templateName)
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

// PreviewItem handles GET /items/{id}/preview — returns a label preview image.
func (h *PrintHandler) PreviewItem(w http.ResponseWriter, r *http.Request) {
	data, ok := h.buildItemLabelData(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	templateName := r.URL.Query().Get("template")
	if templateName == "" {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "template parameter required"})
		return
	}

	opts := label.RenderOpts{PrintDate: r.URL.Query().Get("print_date") == "true"}
	renderPreview(w, h.templates, data, templateName, previewWidth(r), opts)
}

// PreviewContainer handles GET /containers/{id}/preview — returns a label preview image.
func (h *PrintHandler) PreviewContainer(w http.ResponseWriter, r *http.Request) {
	showChildren := r.URL.Query().Get("show_children") == "true"
	data, ok := h.buildContainerLabelData(r.PathValue("id"), showChildren)
	if !ok {
		http.NotFound(w, r)
		return
	}

	templateName := r.URL.Query().Get("template")
	if templateName == "" {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "template parameter required"})
		return
	}

	opts := label.RenderOpts{PrintDate: r.URL.Query().Get("print_date") == "true"}
	renderPreview(w, h.templates, data, templateName, previewWidth(r), opts)
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

// PrintContainer handles POST /containers/{id}/print.
func (h *PrintHandler) PrintContainer(w http.ResponseWriter, r *http.Request) {
	var req ContainerPrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	data, ok := h.buildContainerLabelData(r.PathValue("id"), req.ShowChildren)
	if !ok {
		http.NotFound(w, r)
		return
	}

	opts := label.RenderOpts{PrintDate: req.PrintDate}

	if len(req.Templates) == 0 {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "no templates selected"})
		return
	}

	for _, tmplName := range req.Templates {
		if _, ok := label.GetSchema(tmplName); ok {
			if err := h.pm.Print(req.PrinterID, data, tmplName, opts); err != nil {
				webutil.LogError("container print failed (schema %s): %v", tmplName, err)
				webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		} else {
			tmpl := h.templates.GetTemplate(tmplName)
			if tmpl == nil {
				webutil.JSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("template %q not found", tmplName)})
				return
			}
			// Designer template: return for client-side rendering (single template only)
			webutil.JSON(w, http.StatusOK, map[string]any{
				"ok":        true,
				"render":    "client",
				"template":  tmpl,
				"item_data": data,
			})
			return
		}
	}

	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ScanUSB handles GET /printers/scan/usb — enumerates connected Brother USB devices.
func (h *PrintHandler) ScanUSB(w http.ResponseWriter, r *http.Request) {
	webutil.LogInfo("starting USB scan...")
	results, err := transport.ScanUSB()
	if err != nil {
		webutil.LogError("USB scan error: %v", err)
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Deduplicate by serial number, keeping first occurrence.
	// Devices with empty serial are always kept (no dedup key).
	seen := make(map[string]bool)
	unique := make([]transport.USBScanResult, 0, len(results))
	for _, res := range results {
		if res.Serial != "" {
			if seen[res.Serial] {
				continue
			}
			seen[res.Serial] = true
		}
		unique = append(unique, res)
	}

	webutil.LogInfo("USB scan found %d device(s) (%d raw)", len(unique), len(results))
	webutil.JSON(w, http.StatusOK, unique)
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
