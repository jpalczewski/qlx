package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

type Server struct {
	store *store.Store
}

func NewServer(s *store.Store) *Server {
	return &Server{store: s}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/containers", s.HandleContainers)
	mux.HandleFunc("POST /api/containers", s.HandleContainerCreate)
	mux.HandleFunc("GET /api/containers/{id}", s.HandleContainer)
	mux.HandleFunc("PUT /api/containers/{id}", s.HandleContainerUpdate)
	mux.HandleFunc("DELETE /api/containers/{id}", s.HandleContainerDelete)
	mux.HandleFunc("GET /api/containers/{id}/items", s.HandleContainerItems)

	mux.HandleFunc("GET /api/items/{id}", s.HandleItem)
	mux.HandleFunc("POST /api/items", s.HandleItemCreate)
	mux.HandleFunc("PUT /api/items/{id}", s.HandleItemUpdate)
	mux.HandleFunc("DELETE /api/items/{id}", s.HandleItemDelete)

	mux.HandleFunc("PATCH /api/containers/{id}/move", s.HandleContainerMove)
	mux.HandleFunc("PATCH /api/items/{id}/move", s.HandleItemMove)

	mux.HandleFunc("GET /api/export/json", s.HandleExportJSON)
	mux.HandleFunc("GET /api/export/csv", s.HandleExportCSV)
}

type upsertContainerRequest struct {
	ParentID    string `json:"parent_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type upsertItemRequest struct {
	ContainerID string `json:"container_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) HandleContainers(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent_id")
	if parentID != "" || r.URL.Query().Has("parent_id") {
		webutil.JSON(w, http.StatusOK, s.store.ContainerChildren(parentID))
		return
	}
	webutil.JSON(w, http.StatusOK, s.store.AllContainers())
}

func (s *Server) HandleContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	container := s.store.GetContainer(id)
	if container == nil {
		http.NotFound(w, r)
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"container": container,
		"children":  s.store.ContainerChildren(id),
		"path":      s.store.ContainerPath(id),
	})
}

func (s *Server) HandleContainerItems(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	container := s.store.GetContainer(id)
	if container == nil {
		http.NotFound(w, r)
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"container": container,
		"items":     s.store.ContainerItems(id),
		"path":      s.store.ContainerPath(id),
	})
}

func (s *Server) HandleContainerCreate(w http.ResponseWriter, r *http.Request) {
	req := upsertContainerRequest{
		ParentID:    r.FormValue("parent_id"),
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	container := s.store.CreateContainer(req.ParentID, req.Name, req.Description)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusCreated, container)
}

func (s *Server) HandleContainerUpdate(w http.ResponseWriter, r *http.Request) {
	req := upsertContainerRequest{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	container, err := s.store.UpdateContainer(r.PathValue("id"), req.Name, req.Description)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, container)
}

func (s *Server) HandleContainerDelete(w http.ResponseWriter, r *http.Request) {
	err := s.store.DeleteContainer(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) HandleItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item := s.store.GetItem(id)
	if item == nil {
		http.NotFound(w, r)
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"item": item,
		"path": s.store.ContainerPath(item.ContainerID),
	})
}

func (s *Server) HandleItemCreate(w http.ResponseWriter, r *http.Request) {
	req := upsertItemRequest{
		ContainerID: r.FormValue("container_id"),
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	item := s.store.CreateItem(req.ContainerID, req.Name, req.Description)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusCreated, item)
}

func (s *Server) HandleItemUpdate(w http.ResponseWriter, r *http.Request) {
	req := upsertItemRequest{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	item, err := s.store.UpdateItem(r.PathValue("id"), req.Name, req.Description)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, item)
}

func (s *Server) HandleItemDelete(w http.ResponseWriter, r *http.Request) {
	err := s.store.DeleteItem(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

type moveContainerRequest struct {
	ParentID string `json:"parent_id"`
}

type moveItemRequest struct {
	ContainerID string `json:"container_id"`
}

func (s *Server) HandleContainerMove(w http.ResponseWriter, r *http.Request) {
	var req moveContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if err := s.store.MoveContainer(r.PathValue("id"), req.ParentID); err != nil {
		writeStoreError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleItemMove(w http.ResponseWriter, r *http.Request) {
	var req moveItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if err := s.store.MoveItem(r.PathValue("id"), req.ContainerID); err != nil {
		writeStoreError(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleExportJSON(w http.ResponseWriter, r *http.Request) {
	containers, items := s.store.ExportData()
	webutil.JSON(w, http.StatusOK, map[string]any{
		"containers": containers,
		"items":      items,
	})
}

func (s *Server) HandleExportCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")

	cw := csv.NewWriter(w)
	defer cw.Flush()

	_ = cw.Write([]string{"item_id", "item_name", "item_description", "container_path", "created_at"})

	items := s.store.AllItems()
	for _, item := range items {
		path := s.store.ContainerPath(item.ContainerID)
		var pathStrs []string
		for _, c := range path {
			pathStrs = append(pathStrs, c.Name)
		}
		pathStr := strings.Join(pathStrs, " -> ")

		_ = cw.Write([]string{
			item.ID,
			item.Name,
			item.Description,
			pathStr,
			item.CreatedAt.Format(time.RFC3339),
		})
	}
}

func isJSONBody(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Content-Type"), "application/json")
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch err {
	case store.ErrContainerNotFound, store.ErrItemNotFound:
		webutil.JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case store.ErrContainerHasChildren, store.ErrContainerHasItems:
		webutil.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	default:
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
}
