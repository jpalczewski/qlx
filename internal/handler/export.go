package handler

import (
	"encoding/csv"
	"net/http"
	"time"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

// ExportHandler handles HTTP requests for data export operations.
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
	mux.HandleFunc("GET /export/json", h.ExportJSON)
	mux.HandleFunc("GET /export/csv", h.ExportCSV)
}

// ExportJSON handles GET /export/json.
func (h *ExportHandler) ExportJSON(w http.ResponseWriter, _ *http.Request) {
	containers, items := h.export.ExportJSON()
	webutil.JSON(w, http.StatusOK, map[string]any{
		"containers": containers,
		"items":      items,
	})
}

// ExportCSV handles GET /export/csv.
func (h *ExportHandler) ExportCSV(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")

	cw := csv.NewWriter(w)
	defer cw.Flush()

	_ = cw.Write([]string{"item_id", "item_name", "item_description", "container_path", "created_at"})

	items := h.export.AllItems()
	for _, item := range items {
		path := h.inventory.ContainerPath(item.ContainerID)
		pathStr := webutil.FormatContainerPath(path, " -> ")

		_ = cw.Write([]string{
			item.ID,
			item.Name,
			item.Description,
			pathStr,
			item.CreatedAt.Format(time.RFC3339),
		})
	}
}
