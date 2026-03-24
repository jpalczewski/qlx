package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

// ExportHandler handles HTTP requests for data export.
type ExportHandler struct {
	export    *service.ExportService
	inventory *service.InventoryService
}

// NewExportHandler creates a new ExportHandler.
func NewExportHandler(export *service.ExportService, inv *service.InventoryService) *ExportHandler {
	return &ExportHandler{export: export, inventory: inv}
}

// RegisterRoutes registers export routes on the given mux.
func (h *ExportHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /export", h.Export)
}

// validExportFormat returns true if format is one of csv, json, md.
func validExportFormat(format string) bool {
	return format == "csv" || format == "json" || format == "md"
}

// validMDStyle returns true if style is one of table, document, both.
func validMDStyle(style string) bool {
	return style == "table" || style == "document" || style == "both"
}

// exportContentType returns the MIME type for the given export format.
func exportContentType(format string) string {
	switch format {
	case "csv":
		return "text/csv; charset=utf-8"
	case "json":
		return "application/json"
	default: // md
		return "text/markdown; charset=utf-8"
	}
}

// Export handles GET /export with query params: format, container, recursive, md_style, download.
func (h *ExportHandler) Export(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	format := q.Get("format")
	containerID := q.Get("container")
	recursive := q.Get("recursive") == "true"
	mdStyle := q.Get("md_style")
	download := q.Get("download") == "true"

	if !validExportFormat(format) {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or missing format parameter (csv, json, md)"})
		return
	}

	if mdStyle == "" {
		mdStyle = "table"
	}
	if !validMDStyle(mdStyle) {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid md_style parameter (table, document, both)"})
		return
	}

	containerName, ok := h.resolveContainer(w, containerID)
	if !ok {
		return
	}

	filename := buildExportFilename(format, containerID, containerName)

	w.Header().Set("Content-Type", exportContentType(format))
	if download {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	}

	if err := h.writeExport(w, format, containerID, recursive, mdStyle); err != nil {
		webutil.LogError("export %s: %v", format, err)
	}
}

// resolveContainer validates containerID and returns its name. Returns ("", true) when containerID is empty.
func (h *ExportHandler) resolveContainer(w http.ResponseWriter, containerID string) (string, bool) {
	if containerID == "" {
		return "", true
	}
	c := h.inventory.GetContainer(containerID)
	if c == nil {
		webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "container not found"})
		return "", false
	}
	return c.Name, true
}

// buildExportFilename constructs the suggested download filename.
func buildExportFilename(format, containerID, containerName string) string {
	if containerID != "" {
		return "qlx-" + service.SanitizeFilename(containerName) + "-export." + format
	}
	return "qlx-export." + format
}

// writeExport dispatches to the appropriate ExportService method.
func (h *ExportHandler) writeExport(w http.ResponseWriter, format, containerID string, recursive bool, mdStyle string) error {
	switch format {
	case "csv":
		return h.export.ExportCSV(w, containerID, recursive)
	case "json":
		return h.export.ExportJSON(w, containerID, recursive)
	default: // md
		return h.export.ExportMarkdown(w, containerID, recursive, mdStyle)
	}
}
