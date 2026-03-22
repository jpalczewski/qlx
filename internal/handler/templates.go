package handler

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// previewSampleData is the sample item data used for template preview rendering.
var previewSampleData = map[string]string{
	"name":        "Sample Item",
	"description": "A sample item for preview",
	"location":    "Warehouse > Shelf A",
	"qr_content":  "/items/preview",
	"barcode_id":  "PREVIEW001",
}

// TemplateHandler handles HTTP requests for label template CRUD operations.
type TemplateHandler struct {
	templates *service.TemplateService
	pm        *print.PrinterManager
	resp      Responder
}

// NewTemplateHandler creates a new TemplateHandler.
func NewTemplateHandler(templates *service.TemplateService, pm *print.PrinterManager, resp Responder) *TemplateHandler {
	return &TemplateHandler{templates: templates, pm: pm, resp: resp}
}

// RegisterRoutes registers template routes on the given mux.
func (h *TemplateHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /templates", h.List)
	mux.HandleFunc("GET /templates/new", h.New)
	mux.HandleFunc("GET /templates/{id}/edit", h.Edit)
	mux.HandleFunc("POST /templates", h.Save)
	mux.HandleFunc("PUT /templates/{id}", h.Save)
	mux.HandleFunc("DELETE /templates/{id}", h.Delete)
}

// List handles GET /templates with optional ?tag= filter.
func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	activeTag := r.URL.Query().Get("tag")
	all := h.templates.AllTemplates()

	// Collect unique tags across all templates.
	tagSet := make(map[string]bool)
	for _, t := range all {
		for _, tag := range t.Tags {
			tagSet[tag] = true
		}
	}
	templateTags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		templateTags = append(templateTags, tag)
	}
	sort.Strings(templateTags)

	// Filter templates by active tag.
	filtered := filterTemplatesByTag(all, activeTag)

	h.resp.Respond(w, r, http.StatusOK, filtered, "templates", func() any {
		return TemplateListData{
			Templates: filtered,
			Tags:      templateTags,
			ActiveTag: activeTag,
		}
	})
}

// filterTemplatesByTag returns templates matching the given tag, or all if tag is empty.
func filterTemplatesByTag(all []store.Template, tag string) []store.Template {
	if tag == "" {
		return all
	}
	var filtered []store.Template
	for _, t := range all {
		for _, tt := range t.Tags {
			if tt == tag {
				filtered = append(filtered, t)
				break
			}
		}
	}
	return filtered
}

// New handles GET /templates/new — renders the designer page for a new template.
func (h *TemplateHandler) New(w http.ResponseWriter, r *http.Request) {
	models := h.collectPrinterModels()
	modelsJSON, _ := json.Marshal(models)
	previewJSON, _ := json.Marshal(previewSampleData)

	h.resp.Respond(w, r, http.StatusOK, nil, "template-designer", func() any {
		return DesignerData{
			Target:            "universal",
			Width:             62,
			Height:            29,
			TemplateJSON:      "[]",
			PrinterModels:     models,
			PrinterModelsJSON: string(modelsJSON),
			PreviewDataJSON:   string(previewJSON),
		}
	})
}

// Edit handles GET /templates/{id}/edit — renders the designer page for an existing template.
func (h *TemplateHandler) Edit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tmpl := h.templates.GetTemplate(id)
	if tmpl == nil {
		http.NotFound(w, r)
		return
	}

	models := h.collectPrinterModels()
	modelsJSON, _ := json.Marshal(models)
	previewJSON, _ := json.Marshal(previewSampleData)

	width := tmpl.WidthMM
	height := tmpl.HeightMM
	if strings.HasPrefix(tmpl.Target, "printer:") {
		width = float64(tmpl.WidthPx)
		height = float64(tmpl.HeightPx)
	}

	h.resp.Respond(w, r, http.StatusOK, tmpl, "template-designer", func() any {
		return DesignerData{
			TemplateID:        tmpl.ID,
			TemplateName:      tmpl.Name,
			TemplateTags:      strings.Join(tmpl.Tags, ", "),
			Target:            tmpl.Target,
			Width:             width,
			Height:            height,
			TemplateJSON:      tmpl.Elements,
			PrinterModels:     models,
			PrinterModelsJSON: string(modelsJSON),
			PreviewDataJSON:   string(previewJSON),
		}
	})
}

// Save handles POST /templates and PUT /templates/{id} — saves template data as JSON.
func (h *TemplateHandler) Save(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string   `json:"name"`
		Tags     []string `json:"tags"`
		Target   string   `json:"target"`
		WidthMM  float64  `json:"width_mm"`
		HeightMM float64  `json:"height_mm"`
		WidthPx  int      `json:"width_px"`
		HeightPx int      `json:"height_px"`
		Elements string   `json:"elements"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	id := r.PathValue("id")

	if id != "" {
		// Update existing
		tmpl := h.templates.GetTemplate(id)
		if tmpl == nil {
			webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
			return
		}
		tmpl.Name = req.Name
		tmpl.Tags = req.Tags
		tmpl.Target = req.Target
		if strings.HasPrefix(req.Target, "printer:") {
			tmpl.WidthPx = req.WidthPx
			tmpl.HeightPx = req.HeightPx
			tmpl.WidthMM = 0
			tmpl.HeightMM = 0
		} else {
			tmpl.WidthMM = req.WidthMM
			tmpl.HeightMM = req.HeightMM
			tmpl.WidthPx = 0
			tmpl.HeightPx = 0
		}
		tmpl.Elements = req.Elements
		tmpl.UpdatedAt = time.Now()
		if err := h.templates.SaveTemplate(*tmpl); err != nil {
			webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	} else {
		// Create new
		if _, err := h.templates.CreateTemplate(req.Name, req.Tags, req.Target, req.WidthMM, req.HeightMM, req.WidthPx, req.HeightPx, req.Elements); err != nil {
			webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Delete handles DELETE /templates/{id}.
func (h *TemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.templates.DeleteTemplate(id); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	all := h.templates.AllTemplates()
	h.resp.Respond(w, r, http.StatusOK, map[string]any{"ok": true}, "templates", func() any {
		return TemplateListData{
			Templates: all,
		}
	})
}

// collectPrinterModels gathers model info from all available encoders.
func (h *TemplateHandler) collectPrinterModels() []encoder.ModelInfo {
	var models []encoder.ModelInfo
	for _, enc := range h.pm.AvailableEncoders() {
		models = append(models, enc.Models()...)
	}
	return models
}
