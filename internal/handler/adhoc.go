package handler

import (
	"encoding/json"
	"net/http"

	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

// AdhocHandler handles HTTP requests for ad-hoc label printing.
type AdhocHandler struct {
	pm        *print.PrinterManager
	printers  *service.PrinterService
	templates *service.TemplateService
	resp      Responder
}

// NewAdhocHandler creates a new AdhocHandler.
func NewAdhocHandler(pm *print.PrinterManager, prn *service.PrinterService,
	tmpl *service.TemplateService, resp Responder) *AdhocHandler {
	return &AdhocHandler{pm: pm, printers: prn, templates: tmpl, resp: resp}
}

// RegisterRoutes registers ad-hoc print routes on the given mux.
func (h *AdhocHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /quick-print", h.Page)
	mux.HandleFunc("POST /adhoc/print", h.Print)
	mux.HandleFunc("GET /adhoc/preview", h.Preview)
}

// Page renders the quick print page with printers and schema names.
func (h *AdhocHandler) Page(w http.ResponseWriter, r *http.Request) {
	vm := QuickPrintData{
		Printers: h.printers.AllPrinters(),
		Schemas:  label.SchemaNames(),
	}

	h.resp.Respond(w, r, http.StatusOK, vm, "quick-print", func() any {
		return vm
	})
}

// Print handles POST /adhoc/print — prints an ad-hoc text label.
func (h *AdhocHandler) Print(w http.ResponseWriter, r *http.Request) {
	var req AdhocPrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if req.Text == "" {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "text is required"})
		return
	}

	data := label.LabelData{Name: req.Text}
	opts := label.RenderOpts{PrintDate: req.PrintDate}

	if _, ok := label.GetSchema(req.Template); ok {
		if err := h.pm.Print(req.PrinterID, data, req.Template, opts); err != nil {
			webutil.LogError("adhoc print failed: %v", err)
			webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	respondClientRender(w, h.templates, req.Template, data)
}

// Preview handles GET /adhoc/preview — returns a label preview image for ad-hoc text.
func (h *AdhocHandler) Preview(w http.ResponseWriter, r *http.Request) {
	text := r.URL.Query().Get("text")
	if text == "" {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "text parameter required"})
		return
	}

	templateName := r.URL.Query().Get("template")
	if templateName == "" {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "template parameter required"})
		return
	}

	data := label.LabelData{Name: text}
	opts := label.RenderOpts{PrintDate: r.URL.Query().Get("print_date") == "true"}
	renderPreview(w, h.templates, data, templateName, previewWidth(r), opts)
}
