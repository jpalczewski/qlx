package ui

import (
	"encoding/json"
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

type bulkMoveRequest struct {
	ItemIDs      []string `json:"item_ids"`
	ContainerIDs []string `json:"container_ids"`
	TargetID     string   `json:"target_id"`
}

type bulkDeleteRequest struct {
	ItemIDs      []string `json:"item_ids"`
	ContainerIDs []string `json:"container_ids"`
}

type bulkTagsRequest struct {
	ItemIDs      []string `json:"item_ids"`
	ContainerIDs []string `json:"container_ids"`
	TagID        string   `json:"tag_id"`
}

// HandleBulkMove handles POST /ui/actions/bulk/move.
// Moves items and containers to a target container.
func (s *Server) HandleBulkMove(w http.ResponseWriter, r *http.Request) {
	var req bulkMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	errs := s.store.BulkMove(req.ItemIDs, req.ContainerIDs, req.TargetID)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"ok":     len(errs) == 0,
		"errors": errs,
	})
}

// HandleBulkDelete handles POST /ui/actions/bulk/delete.
// Deletes items and containers identified by the request body IDs.
func (s *Server) HandleBulkDelete(w http.ResponseWriter, r *http.Request) {
	var req bulkDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	deleted, errs := s.store.BulkDelete(req.ItemIDs, req.ContainerIDs)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"deleted": deleted,
		"failed":  errs,
	})
}

// HandleBulkTags handles POST /ui/actions/bulk/tags.
// Adds a tag to multiple items and containers.
func (s *Server) HandleBulkTags(w http.ResponseWriter, r *http.Request) {
	var req bulkTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if err := s.store.BulkAddTag(req.ItemIDs, req.ContainerIDs, req.TagID); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
