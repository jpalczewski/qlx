package ui

import (
	"encoding/json"
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

type bulkIDEntry struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type bulkMoveRequest struct {
	IDs               []bulkIDEntry `json:"ids"`
	TargetContainerID string        `json:"target_container_id"`
}

type bulkDeleteRequest struct {
	IDs []bulkIDEntry `json:"ids"`
}

type bulkTagsRequest struct {
	IDs   []bulkIDEntry `json:"ids"`
	TagID string        `json:"tag_id"`
}

func splitBulkIDs(entries []bulkIDEntry) (itemIDs, containerIDs []string) {
	for _, e := range entries {
		switch e.Type {
		case "item":
			itemIDs = append(itemIDs, e.ID)
		case "container":
			containerIDs = append(containerIDs, e.ID)
		}
	}
	return
}

// HandleBulkMove handles POST /ui/actions/bulk/move.
func (s *Server) HandleBulkMove(w http.ResponseWriter, r *http.Request) {
	var req bulkMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	itemIDs, containerIDs := splitBulkIDs(req.IDs)
	errs, err := s.bulk.Move(itemIDs, containerIDs, req.TargetContainerID)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"ok":     len(errs) == 0,
		"errors": errs,
	})
}

// HandleBulkDelete handles POST /ui/actions/bulk/delete.
func (s *Server) HandleBulkDelete(w http.ResponseWriter, r *http.Request) {
	var req bulkDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	itemIDs, containerIDs := splitBulkIDs(req.IDs)
	deleted, errs, err := s.bulk.Delete(itemIDs, containerIDs)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"deleted": deleted,
		"failed":  errs,
	})
}

// HandleBulkTags handles POST /ui/actions/bulk/tags.
func (s *Server) HandleBulkTags(w http.ResponseWriter, r *http.Request) {
	var req bulkTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	itemIDs, containerIDs := splitBulkIDs(req.IDs)
	if err := s.bulk.AddTag(itemIDs, containerIDs, req.TagID); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
