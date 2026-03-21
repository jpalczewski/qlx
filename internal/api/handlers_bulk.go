package api

import (
	"encoding/json"
	"net/http"

	"github.com/erxyi/qlx/internal/shared/dto"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

func (s *Server) HandleBulkMove(w http.ResponseWriter, r *http.Request) {
	var req dto.BulkMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	itemIDs, containerIDs := dto.SplitBulkIDs(req.IDs)
	errs, err := s.bulk.Move(itemIDs, containerIDs, req.TargetContainerID)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]any{"errors": errs})
}

func (s *Server) HandleBulkDelete(w http.ResponseWriter, r *http.Request) {
	var req dto.BulkDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	itemIDs, containerIDs := dto.SplitBulkIDs(req.IDs)
	deleted, failed, err := s.bulk.Delete(itemIDs, containerIDs)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]any{
		"deleted": deleted,
		"failed":  failed,
	})
}

func (s *Server) HandleBulkTags(w http.ResponseWriter, r *http.Request) {
	var req dto.BulkTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	itemIDs, containerIDs := dto.SplitBulkIDs(req.IDs)
	if err := s.bulk.AddTag(itemIDs, containerIDs, req.TagID); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
