package api

import (
	"encoding/json"
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
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
		webutil.JSON(w, http.StatusOK, s.store.TagChildren(parentID))
		return
	}
	webutil.JSON(w, http.StatusOK, s.store.AllTags())
}

func (s *Server) HandleTagCreate(w http.ResponseWriter, r *http.Request) {
	req := upsertTagRequest{
		Name:     r.FormValue("name"),      //nolint:gosec // G120: internal tool, no untrusted input
		ParentID: r.FormValue("parent_id"), //nolint:gosec // G120: internal tool, no untrusted input
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	tag := s.store.CreateTag(req.ParentID, req.Name)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusCreated, tag)
}

func (s *Server) HandleTag(w http.ResponseWriter, r *http.Request) {
	tag := s.store.GetTag(r.PathValue("id"))
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

	tag, err := s.store.UpdateTag(r.PathValue("id"), req.Name)
	if err != nil {
		writeTagError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, tag)
}

func (s *Server) HandleTagDelete(w http.ResponseWriter, r *http.Request) {
	err := s.store.DeleteTag(r.PathValue("id"))
	if err != nil {
		writeTagError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
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
	if err := s.store.MoveTag(r.PathValue("id"), req.ParentID); err != nil {
		writeTagError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleTagDescendants(w http.ResponseWriter, r *http.Request) {
	webutil.JSON(w, http.StatusOK, s.store.TagDescendants(r.PathValue("id")))
}

func (s *Server) HandleItemTagAdd(w http.ResponseWriter, r *http.Request) {
	req := addTagRequest{
		TagID: r.FormValue("tag_id"), //nolint:gosec // G120: internal tool, no untrusted input
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if err := s.store.AddItemTag(r.PathValue("id"), req.TagID); err != nil {
		writeTagError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleItemTagRemove(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RemoveItemTag(r.PathValue("id"), r.PathValue("tag_id")); err != nil {
		writeTagError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
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

	if err := s.store.AddContainerTag(r.PathValue("id"), req.TagID); err != nil {
		writeTagError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleContainerTagRemove(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RemoveContainerTag(r.PathValue("id"), r.PathValue("tag_id")); err != nil {
		writeTagError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeTagError(w http.ResponseWriter, err error) {
	switch err {
	case store.ErrTagNotFound, store.ErrItemNotFound, store.ErrContainerNotFound:
		webutil.JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case store.ErrTagHasChildren:
		webutil.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	default:
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
}
