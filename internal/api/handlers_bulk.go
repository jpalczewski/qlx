package api

import (
	"encoding/json"
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

type bulkIDEntry struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "container" or "item"
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

func (s *Server) HandleBulkMove(w http.ResponseWriter, r *http.Request) {
	var req bulkMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	itemIDs, containerIDs := splitBulkIDs(req.IDs)
	errs := s.store.BulkMove(itemIDs, containerIDs, req.TargetContainerID)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]any{"errors": errs})
}

func (s *Server) HandleBulkDelete(w http.ResponseWriter, r *http.Request) {
	var req bulkDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	itemIDs, containerIDs := splitBulkIDs(req.IDs)
	deleted, failed := s.store.BulkDelete(itemIDs, containerIDs)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]any{
		"deleted": deleted,
		"failed":  failed,
	})
}

func (s *Server) HandleBulkTags(w http.ResponseWriter, r *http.Request) {
	var req bulkTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	itemIDs, containerIDs := splitBulkIDs(req.IDs)
	if err := s.store.BulkAddTag(itemIDs, containerIDs, req.TagID); err != nil {
		writeTagError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
