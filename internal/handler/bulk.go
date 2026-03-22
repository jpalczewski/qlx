package handler

import (
	"encoding/json"
	"net/http"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/dto"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

// BulkHandler handles HTTP requests for bulk move, delete, and tag operations.
type BulkHandler struct {
	bulk *service.BulkService
}

// NewBulkHandler creates a new BulkHandler.
func NewBulkHandler(bulk *service.BulkService) *BulkHandler {
	return &BulkHandler{bulk: bulk}
}

// RegisterRoutes registers bulk routes on the given mux.
func (h *BulkHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /bulk/move", h.Move)
	mux.HandleFunc("POST /bulk/delete", h.Delete)
	mux.HandleFunc("POST /bulk/tags", h.AddTag)
}

// Move handles POST /bulk/move.
func (h *BulkHandler) Move(w http.ResponseWriter, r *http.Request) {
	var req dto.BulkMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	itemIDs, containerIDs := dto.SplitBulkIDs(req.IDs)
	errs, err := h.bulk.Move(itemIDs, containerIDs, req.TargetContainerID)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"ok":     len(errs) == 0,
		"errors": errs,
	})
}

// Delete handles POST /bulk/delete.
func (h *BulkHandler) Delete(w http.ResponseWriter, r *http.Request) {
	var req dto.BulkDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	itemIDs, containerIDs := dto.SplitBulkIDs(req.IDs)
	deleted, errs, err := h.bulk.Delete(itemIDs, containerIDs)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"ok":      len(errs) == 0,
		"deleted": deleted,
		"failed":  errs,
	})
}

// AddTag handles POST /bulk/tags.
func (h *BulkHandler) AddTag(w http.ResponseWriter, r *http.Request) {
	var req dto.BulkTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	itemIDs, containerIDs := dto.SplitBulkIDs(req.IDs)
	if err := h.bulk.AddTag(itemIDs, containerIDs, req.TagID); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
