package api

import (
	"encoding/json"
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

type upsertTagRequest struct {
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
}

type moveTagRequest struct {
	ParentID string `json:"parent_id"`
}

type addTagRequest struct {
	TagID string `json:"tag_id"`
}

func (s *Server) HandleTags(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent_id")
	if parentID != "" || r.URL.Query().Has("parent_id") {
		webutil.JSON(w, http.StatusOK, s.tags.TagChildren(parentID))
		return
	}
	webutil.JSON(w, http.StatusOK, s.tags.AllTags())
}

func (s *Server) HandleTagCreate(w http.ResponseWriter, r *http.Request) {
	req := upsertTagRequest{
		Name:     r.FormValue("name"),      //nolint:gosec // G120: internal tool, no untrusted input
		ParentID: r.FormValue("parent_id"), //nolint:gosec // G120: internal tool, no untrusted input
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	tag, err := s.tags.CreateTag(req.ParentID, req.Name)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	webutil.JSON(w, http.StatusCreated, tag)
}

func (s *Server) HandleTag(w http.ResponseWriter, r *http.Request) {
	tag := s.tags.GetTag(r.PathValue("id"))
	if tag == nil {
		http.NotFound(w, r)
		return
	}
	webutil.JSON(w, http.StatusOK, tag)
}

func (s *Server) HandleTagUpdate(w http.ResponseWriter, r *http.Request) {
	req := upsertTagRequest{
		Name: r.FormValue("name"), //nolint:gosec // G120: internal tool, no untrusted input
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	tag, err := s.tags.UpdateTag(r.PathValue("id"), req.Name)
	if err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, tag)
}

func (s *Server) HandleTagDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.tags.DeleteTag(r.PathValue("id")); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) HandleTagMove(w http.ResponseWriter, r *http.Request) {
	var req moveTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if err := s.tags.MoveTag(r.PathValue("id"), req.ParentID); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleTagDescendants(w http.ResponseWriter, r *http.Request) {
	webutil.JSON(w, http.StatusOK, s.tags.TagDescendants(r.PathValue("id")))
}

func (s *Server) HandleItemTagAdd(w http.ResponseWriter, r *http.Request) {
	req := addTagRequest{
		TagID: r.FormValue("tag_id"), //nolint:gosec // G120: internal tool, no untrusted input
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if err := s.tags.AddItemTag(r.PathValue("id"), req.TagID); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleItemTagRemove(w http.ResponseWriter, r *http.Request) {
	if err := s.tags.RemoveItemTag(r.PathValue("id"), r.PathValue("tag_id")); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleContainerTagAdd(w http.ResponseWriter, r *http.Request) {
	req := addTagRequest{
		TagID: r.FormValue("tag_id"), //nolint:gosec // G120: internal tool, no untrusted input
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if err := s.tags.AddContainerTag(r.PathValue("id"), req.TagID); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleContainerTagRemove(w http.ResponseWriter, r *http.Request) {
	if err := s.tags.RemoveContainerTag(r.PathValue("id"), r.PathValue("tag_id")); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
